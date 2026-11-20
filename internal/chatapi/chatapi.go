package chatapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"

	"github.com/niumao/nnnchat/internal/auth"
	"github.com/niumao/nnnchat/internal/history"
)

// ChatAPI is the single entry point transports submit client connections
// through. It owns the directory of rooms; all other chat logic lives in
// each room's own goroutine (see room.go), so this type only ever needs to
// guard room lookup/creation, not message handling.
type ChatAPI struct {
	rooms     map[string]*Room
	history   history.Store
	jwtSecret []byte // nil disables auth
	mu        sync.Mutex
}

type clientInfo struct {
	Room  string `json:"room"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

// New start a new instance of the new chat api
func New() *ChatAPI {
	return NewWithHistory(nil)
}

// NewWithHistory starts a new instance of the chat api that persists and
// replays room history through the given store. A nil store disables history.
func NewWithHistory(store history.Store) *ChatAPI {
	return &ChatAPI{
		rooms:   make(map[string]*Room),
		history: store,
	}
}

// NewWithAuth starts a new instance of the chat api that requires a valid
// JWT (signed with jwtSecret) on every client join, in addition to optional
// history. A nil jwtSecret disables auth, same as a nil store disables
// history.
func NewWithAuth(store history.Store, jwtSecret []byte) *ChatAPI {
	return &ChatAPI{
		rooms:     make(map[string]*Room),
		history:   store,
		jwtSecret: jwtSecret,
	}
}

// Close shuts down every chat room managed by this API. Callers are
// responsible for deciding when that should happen (e.g. on process shutdown).
func (cAPI *ChatAPI) Close() {
	cAPI.mu.Lock()
	rooms := cAPI.rooms
	cAPI.rooms = make(map[string]*Room)
	cAPI.mu.Unlock()

	for name, r := range rooms {
		slog.Info("closing room", "room", name)
		close(r.Quit)
	}
}

// AddClient adds a new client to the chat server. Expects a JSON file
func (cAPI *ChatAPI) AddClient(c io.ReadWriteCloser) {
	cinfo := new(clientInfo)
	if err := json.NewDecoder(c).Decode(cinfo); err != nil {
		slog.Error("could not create chat room, invalid chat room name", "err", err)
		c.Close()
		return
	}
	if cAPI.jwtSecret != nil {
		if err := cAPI.authorize(cinfo); err != nil {
			slog.Warn("rejecting client", "client", cinfo.Name, "room", cinfo.Room, "err", err)
			c.Close()
			return
		}
	}
	room := cAPI.roomFor(cinfo.Room)
	startClientConn(cinfo.Name, room, c)
}

// authorize verifies cinfo.Token against cAPI.jwtSecret and checks that its
// claims actually grant this client the name and room it's asking for.
// Only called when auth is enabled (jwtSecret != nil).
func (cAPI *ChatAPI) authorize(cinfo *clientInfo) error {
	claims, err := auth.Verify(cAPI.jwtSecret, cinfo.Token)
	if err != nil {
		return err
	}
	if claims.Subject != cinfo.Name {
		return fmt.Errorf("token subject %q does not match client name %q", claims.Subject, cinfo.Name)
	}
	if !slices.Contains(claims.Rooms, cinfo.Room) {
		return fmt.Errorf("token does not grant access to room %q", cinfo.Room)
	}
	return nil
}

// roomFor returns the named room, creating it if it doesn't exist yet. This
// is the only place ChatAPI's own state is touched, so the lock's critical
// section is just a map lookup/insert -- it never waits on a room's history
// replay or any other per-client work, so one room's traffic can't stall
// another room's joins.
func (cAPI *ChatAPI) roomFor(name string) *Room {
	cAPI.mu.Lock()
	defer cAPI.mu.Unlock()
	r, ok := cAPI.rooms[name]
	if !ok {
		r = CreateRoom(name, cAPI.history)
		cAPI.rooms[name] = r
	}
	return r
}
