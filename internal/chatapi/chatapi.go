package chatapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"sync"

	"github.com/niumao/nnnchat/internal/history"
)

// ChatAPI is the single entry point transports submit client connections
// through. It owns the directory of rooms; all other chat logic lives in
// each room's own actor goroutine (see room_actor.go), so this type only
// ever needs to guard room lookup/creation, not message handling.
type ChatAPI struct {
	rooms   map[string]*roomActor
	history history.Store
	mu      sync.Mutex
}

type clientInfo struct {
	Room string `json:"room"`
	Name string `json:"name"`
}

// New start a new instance of the new chat api
func New() *ChatAPI {
	return NewWithHistory(nil)
}

// NewWithHistory starts a new instance of the chat api that persists and
// replays room history through the given store. A nil store disables history.
func NewWithHistory(store history.Store) *ChatAPI {
	return &ChatAPI{
		rooms:   make(map[string]*roomActor),
		history: store,
	}
}

// Close shuts down every chat room managed by this API. Callers are
// responsible for deciding when that should happen (e.g. on process shutdown).
//
// Each room is told to stop via an EventShutdown sent into its own mailbox,
// rather than closing the mailbox from here. That keeps the room actor as
// the only goroutine that ever closes its own channels, so a client's reader
// goroutine concurrently sending to the same mailbox can never race a close.
func (cAPI *ChatAPI) Close() {
	cAPI.mu.Lock()
	rooms := cAPI.rooms
	cAPI.rooms = make(map[string]*roomActor)
	cAPI.mu.Unlock()

	for name, r := range rooms {
		slog.Info("closing room", "room", name)
		r.mailbox <- Event{Type: EventShutdown}
	}
}

// AddClient adds a new client to the chat server. Expects a JSON file
func (cAPI *ChatAPI) AddClient(c io.ReadWriteCloser) {
	cinfo := new(clientInfo)
	if err := json.NewDecoder(c).Decode(cinfo); err != nil {
		slog.Error("could not create chat room, invalid chat room name", "err", err)
		return
	}
	room := cAPI.roomFor(cinfo.Room)
	startClientConn(cinfo.Name, cinfo.Room, room.mailbox, c)
}

// roomFor returns the named room's actor, creating it if it doesn't exist
// yet. This is the only place ChatAPI's state is touched, so the lock's
// critical section stays tiny regardless of how much chat traffic a room has.
func (cAPI *ChatAPI) roomFor(name string) *roomActor {
	cAPI.mu.Lock()
	defer cAPI.mu.Unlock()
	r, ok := cAPI.rooms[name]
	if !ok {
		r = newRoomActor(name, cAPI.history)
		cAPI.rooms[name] = r
	}
	return r
}
