package chatapi

import (
	"context"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/niumao/nnnchat/internal/history"
)

// historySize caps how many past messages are retained and replayed per room.
const historySize = 50

// historyTimeout bounds how long a single history read/write may block the
// room, so a slow (but not fully down) Redis can't stall live chat.
const historyTimeout = 500 * time.Millisecond

// Room type represents a chat room
type Room struct {
	name    string
	Msgch   chan string
	clients map[string]chan<- string
	history history.Store
	//signals the quitting of the chat room
	Quit chan struct{}
	*sync.RWMutex
}

// CreateRoom starts a new chat room with name rname. A nil store disables history.
func CreateRoom(rname string, store history.Store) *Room {
	r := &Room{
		name:    rname,
		Msgch:   make(chan string),
		RWMutex: new(sync.RWMutex),
		clients: make(map[string]chan<- string),
		history: store,
		Quit:    make(chan struct{}),
	}
	r.Run()
	return r
}

// AddClient adds a new client to the chat room
func (r *Room) AddClient(c io.ReadWriteCloser, clientname string) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.clients[clientname]; ok {
		log.Printf("Client %s already exist in chat room %s, existing...", clientname, r.name)
		return
	}
	log.Printf("Adding client %s \n", clientname)
	wc, done := StartClient(clientname, r.Msgch, c, r.name)
	r.replayHistory(wc)
	r.clients[clientname] = wc

	//remove client when done is signalled
	go func() {
		<-done
		r.RemoveClientSync(clientname)
	}()
}

// replayHistory sends the room's recent history to a newly joined client,
// before it starts receiving live broadcasts. No-op if history is disabled.
func (r *Room) replayHistory(wc chan<- string) {
	if r.history == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), historyTimeout)
	defer cancel()
	msgs, err := r.history.Recent(ctx, r.name, historySize)
	if err != nil {
		log.Printf("Could not load chat history for room %s: %s \n", r.name, err)
		return
	}
	for _, m := range msgs {
		wc <- m.Sender + ":" + m.Text + "\n"
	}
}

// ClCount returns the number of clients in a chat room
func (r *Room) ClCount() int {
	return len(r.clients)
}

// RemoveClientSync removes a client from the chat room. This is a blocking call
func (r *Room) RemoveClientSync(name string) {
	r.Lock()
	defer r.Unlock()
	wc, ok := r.clients[name]
	if !ok {
		// Already removed (e.g. the room closed concurrently), nothing to do.
		return
	}
	log.Printf("Removing client %s \n", name)
	delete(r.clients, name)
	// Closing wc stops that client's writeMonitor goroutine (its `range c.wc`
	// loop ends). Without this, that goroutine leaked forever: the client
	// disconnected but nothing ever told its writer to stop waiting on wc.
	close(wc)
}

// Run runs a chat room
func (r *Room) Run() {
	log.Println("Starting chat room", r.name)
	//handle the chat room main message channel
	go func() {
		for msg := range r.Msgch {
			r.broadcastMsg(msg)
		}
	}()

	//handle when the quit channel is triggered
	go func() {
		<-r.Quit
		r.CloseChatRoomSync()
	}()
}

// CloseChatRoomSync closes a chat room. This is a blocking call
func (r *Room) CloseChatRoomSync() {
	r.Lock()
	defer r.Unlock()
	close(r.Msgch)
	// Same reasoning as RemoveClientSync: closing each client's wc stops its
	// writeMonitor goroutine. Previously these were only removed from the
	// map, never closed, so every client still connected when the room shut
	// down left its writer goroutine blocked on wc forever (goroutine leak).
	for name, wc := range r.clients {
		delete(r.clients, name)
		close(wc)
	}
}

// fan out is used to distribute the chat message.
//
// This used to spawn a goroutine per client per message to do the send. That's
// unbounded: a single slow client leaves goroutines (and the messages queued
// in their closures) piling up with no limit, since nothing ever caps how many
// pending sends accumulate. Now each client has a small buffered channel
// (wcBufferSize) and we send to it directly with a non-blocking select: a
// client that's keeping up gets the message, a client that's falling behind
// just drops it instead of costing the broadcaster a new goroutine.
func (r *Room) broadcastMsg(msg string) {
	r.appendHistory(msg)
	r.RLock()
	defer r.RUnlock()
	for name, wc := range r.clients {
		select {
		case wc <- msg:
		default:
			log.Printf("Client %s in room %s is not keeping up, dropping message \n", name, r.name)
		}
	}
}

// appendHistory persists msg (formatted as "sender:text\n") for later replay.
// No-op if history is disabled.
func (r *Room) appendHistory(msg string) {
	if r.history == nil {
		return
	}
	sender, text, ok := strings.Cut(strings.TrimSuffix(msg, "\n"), ":")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), historyTimeout)
	defer cancel()
	err := r.history.Append(ctx, r.name, history.Message{
		Sender: sender,
		Text:   text,
		Ts:     time.Now().Unix(),
	})
	if err != nil {
		log.Printf("Could not persist chat history for room %s: %s \n", r.name, err)
	}
}
