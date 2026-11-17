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
// make the room actor wait, instead of blocking its single goroutine on one
// slow reader.
const wcBufferSize = 16

// roomActor owns all state for a single room. Every field below is only
// ever touched from run(), so no lock is needed: the actor is its own
// synchronization boundary.
type roomActor struct {
	name    string
	mailbox chan Event
	clients map[string]chan<- string
	history history.Store
}

// newRoomActor creates a room actor and starts its goroutine. A nil store
// disables history for this room.
func newRoomActor(name string, store history.Store) *roomActor {
	r := &roomActor{
		name:    name,
		mailbox: make(chan Event),
		clients: make(map[string]chan<- string),
		history: store,
	}
	go r.run()
	return r
}

// run is the actor's event loop: it owns r.clients exclusively, so every
// case below can read/write it without a lock.
func (r *roomActor) run() {
	slog.Info("starting chat room", "room", r.name)
	for e := range r.mailbox {
		switch e.Type {
		case EventJoin:
			r.handleJoin(e)
		case EventLeave:
			r.handleLeave(e)
		case EventMessage:
			r.handleMessage(e)
		case EventShutdown:
			r.handleShutdown()
			return
		}
	}
}

func (r *roomActor) handleJoin(e Event) {
	if _, ok := r.clients[e.Client]; ok {
		slog.Warn("client already exists in room, ignoring join", "client", e.Client, "room", r.name)
		return
	}
	slog.Info("adding client", "client", e.Client, "room", r.name)
	r.replayHistory(e.Outbox)
	r.clients[e.Client] = e.Outbox
}

func (r *roomActor) handleLeave(e Event) {
	wc, ok := r.clients[e.Client]
	if !ok {
		// Already removed (e.g. a duplicate Leave), nothing to do.
		return
	}
	slog.Info("removing client", "client", e.Client, "room", r.name)
	delete(r.clients, e.Client)
	// Closing wc stops that client's writeLoop (its `range outbox` loop ends).
	close(wc)
}

// handleMessage is the fan-out step: it used to spawn a goroutine per client
// per message to do the send, which is unbounded (a single slow client leaves
// goroutines piling up with no limit). Instead, each client has a small
// buffered channel and we send with a non-blocking select: a client that's
// keeping up gets the message, a client that's falling behind just drops it.
func (r *roomActor) handleMessage(e Event) {
	r.appendHistory(e.Client, e.Text)
	msg := e.Client + ":" + e.Text + "\n"
	for name, wc := range r.clients {
		select {
		case wc <- msg:
		default:
			slog.Warn("client not keeping up, dropping message", "client", name, "room", r.name)
		}
	}
}

// handleShutdown closes every client's outbound channel. Unlike closing the
// mailbox from outside, this runs on the actor's own goroutine, so there's
// no race with a client's reader goroutine concurrently sending to r.mailbox.
func (r *roomActor) handleShutdown() {
	for name, wc := range r.clients {
		delete(r.clients, name)
		close(wc)
	}
}

// replayHistory sends the room's recent history to a newly joined client,
// before it's registered to receive live broadcasts. No-op if history is
// disabled.
func (r *roomActor) replayHistory(wc chan<- string) {
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
func (r *roomActor) appendHistory(sender, text string) {
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
