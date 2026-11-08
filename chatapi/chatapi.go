package chatapi

import (
	"encoding/json"
	"io"
	"log"
	"sync"

	"github.com/niumao/nnnchat/history"
)

type ChatAPI struct {
	rooms   map[string]*Room
	history history.Store
	*sync.RWMutex
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
		rooms:   make(map[string]*Room),
		history: store,
		RWMutex: new(sync.RWMutex),
	}
}

// Close shuts down every chat room managed by this API. Callers are
// responsible for deciding when that should happen (e.g. on process shutdown).
func (cAPI *ChatAPI) Close() {
	cAPI.RLock()
	defer cAPI.RUnlock()
	for name, r := range cAPI.rooms {
		log.Printf("Closing room %s \n", name)
		close(r.Quit)
	}
}

// AddClient adds a new client to the chat server. Expects a JSON file
func (cAPI *ChatAPI) AddClient(c io.ReadWriteCloser) {
	cinfo := new(clientInfo)
	if err := json.NewDecoder(c).Decode(cinfo); err != nil {
		log.Printf("Could not create chat room, invalid chat room name, error: %s \n", err)
	} else {
		cAPI.handleClient(cinfo, c)
	}
}

func (cAPI *ChatAPI) handleClient(cinfo *clientInfo, c io.ReadWriteCloser) {
	cAPI.Lock()
	defer cAPI.Unlock()
	r, ok := cAPI.rooms[cinfo.Room]
	if !ok {
		r = CreateRoom(cinfo.Room, cAPI.history)
	}
	r.AddClient(c, cinfo.Name)
	cAPI.rooms[cinfo.Room] = r
}
