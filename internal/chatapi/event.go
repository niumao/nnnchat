package chatapi

// EventType identifies what a room actor should do with an Event.
type EventType int

const (
	EventJoin EventType = iota
	EventLeave
	EventMessage
	EventShutdown
)

// Event is the single unit of work passed from a client connection (or the
// Dispatcher) into a room actor's mailbox. Only the fields relevant to Type
// are populated.
type Event struct {
	Type   EventType
	Client string
	Text   string        // EventMessage only
	Outbox chan<- string // EventJoin only: where the room sends messages back to this client
}
