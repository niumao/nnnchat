package chatapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/niumao/nnnchat/internal/history"
)

// historySize caps how many past messages are retained and replayed per room.
const historySize = 50

// historyTimeout bounds how long a single history read/write may block the
// room, so a slow (but not fully down) Redis can't stall live chat.
const historyTimeout = 500 * time.Millisecond

// wcBufferSize is the per-client outbound buffer. Combined with the
// non-blocking send in handleMessage, this bounds how much a slow client can
// make the room's goroutine wait, instead of blocking it on one slow reader.
const wcBufferSize = 16

// joinReq is sent on Room.joins when a client connects.
type joinReq struct {
	name   string
	outbox chan<- string
}

// msgReq is sent on Room.messages when a client sends a line.
type msgReq struct {
	name string
	text string
}

// countReq is sent on Room.counts to query the number of connected clients;
// reply is where run() sends the answer back.
type countReq struct {
	reply chan<- int
}

// Room is a chat room. Every field below is only ever touched from run, so
// no lock is needed: the goroutine running run is this room's exclusive
// owner of its state. Every other goroutine interacts with a Room only by
// sending on joins/leaves/messages/counts -- never by touching clients
// directly -- which is what keeps this pure CSP rather than lock-based.
type Room struct {
	name     string
	joins    chan joinReq
	leaves   chan string
	messages chan msgReq
	counts   chan countReq
	Quit     chan struct{}
	history  history.Store

	clients map[string]chan<- string
}

// CreateRoom starts a new chat room with name rname. A nil store disables history.
func CreateRoom(rname string, store history.Store) *Room {
	r := &Room{
		name:     rname,
		joins:    make(chan joinReq),
		leaves:   make(chan string),
		messages: make(chan msgReq),
		counts:   make(chan countReq),
		Quit:     make(chan struct{}),
		history:  store,
		clients:  make(map[string]chan<- string),
	}
	go r.run()
	return r
}

// run is the room's event loop: it owns r.clients exclusively, so every
// case below can read/write it without a lock.
func (r *Room) run() {
	slog.Info("starting chat room", "room", r.name)
	for {
		select {
		case j := <-r.joins:
			r.handleJoin(j)
		case name := <-r.leaves:
			r.handleLeave(name)
		case m := <-r.messages:
			r.handleMessage(m)
		case c := <-r.counts:
			c.reply <- len(r.clients)
		case <-r.Quit:
			r.handleShutdown()
			return
		}
	}
}

func (r *Room) handleJoin(j joinReq) {
	if _, ok := r.clients[j.name]; ok {
		slog.Warn("client already exists in room, ignoring join", "client", j.name, "room", r.name)
		return
	}
	slog.Info("adding client", "client", j.name, "room", r.name)
	r.replayHistory(j.outbox)
	r.clients[j.name] = j.outbox
}

func (r *Room) handleLeave(name string) {
	wc, ok := r.clients[name]
	if !ok {
		// Already removed (e.g. a duplicate leave), nothing to do.
		return
	}
	slog.Info("removing client", "client", name, "room", r.name)
	delete(r.clients, name)
	// Closing wc stops that client's writeLoop goroutine (its `range outbox`
	// loop ends), so it doesn't leak after the client disconnects.
	close(wc)
}

// handleMessage is the fan-out step: it used to spawn a goroutine per client
// per message to do the send, which is unbounded (a single slow client
// leaves goroutines piling up with no limit). Instead, each client has a
// small buffered channel and we send with a non-blocking select: a client
// that's keeping up gets the message, a client that's falling behind just
// drops it.
func (r *Room) handleMessage(m msgReq) {
	r.appendHistory(m.name, m.text)
	msg := m.name + ":" + m.text + "\n"
	for name, wc := range r.clients {
		select {
		case wc <- msg:
		default:
			slog.Warn("client not keeping up, dropping message", "client", name, "room", r.name)
		}
	}
}

// handleShutdown closes every client's outbound channel. Because it runs on
// the room's own goroutine (triggered by closing Quit), there's no race
// with a client's reader goroutine concurrently sending on joins/messages.
func (r *Room) handleShutdown() {
	for name, wc := range r.clients {
		delete(r.clients, name)
		close(wc)
	}
}

// ClCount returns the number of clients currently in the room. Since the
// room's goroutine owns clients exclusively, this works like any other room
// interaction: send a request and wait for run() to reply, instead of
// reading the map directly under a lock.
func (r *Room) ClCount() int {
	reply := make(chan int)
	r.counts <- countReq{reply: reply}
	return <-reply
}

// replayHistory sends the room's recent history to a newly joined client,
// before it's registered to receive live broadcasts. No-op if history is
// disabled.
func (r *Room) replayHistory(wc chan<- string) {
	if r.history == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), historyTimeout)
	defer cancel()
	msgs, err := r.history.Recent(ctx, r.name, historySize)
	if err != nil {
		slog.Warn("could not load chat history", "room", r.name, "err", err)
		return
	}
	for _, m := range msgs {
		wc <- m.Sender + ":" + m.Text + "\n"
	}
}

// appendHistory persists a message for later replay. No-op if history is
// disabled.
func (r *Room) appendHistory(sender, text string) {
	if r.history == nil {
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
		slog.Warn("could not persist chat history", "room", r.name, "err", err)
	}
}
