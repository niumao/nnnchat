package history

import "context"

// Message is a single chat line persisted for room history.
type Message struct {
	Sender string `json:"sender"`
	Text   string `json:"text"`
	Ts     int64  `json:"ts"`
}

// Store persists chat messages per room and replays recent history to new clients.
type Store interface {
	Append(ctx context.Context, room string, msg Message) error
	Recent(ctx context.Context, room string, n int) ([]Message, error)
}
