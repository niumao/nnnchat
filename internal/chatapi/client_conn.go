package chatapi

import (
	"bufio"
	"io"
	"log/slog"
)

// startClientConn wires up a client's connection: it joins the room via
// mailbox, then starts a reader goroutine (turns incoming lines into Events
// submitted to mailbox) and a writer goroutine (flushes whatever the room
// actor sends back to this client's outbox).
func startClientConn(name string, roomName string, mailbox chan<- Event, cn io.ReadWriteCloser) {
	outbox := make(chan string, wcBufferSize)
	mailbox <- Event{Type: EventJoin, Client: name, Outbox: outbox}

	go writeLoop(cn, outbox)
	go readLoop(cn, mailbox, name, roomName)
}

// readLoop reads newline-delimited messages from cn and submits them to the
// room's mailbox, until the connection is closed or errors.
func readLoop(cn io.ReadWriteCloser, mailbox chan<- Event, name, roomName string) {
	scanner := bufio.NewScanner(bufio.NewReader(cn))
	for scanner.Scan() {
		text := scanner.Text()
		slog.Info("chat message", "room", roomName, "client", name, "text", text)
		mailbox <- Event{Type: EventMessage, Client: name, Text: text}
	}
	mailbox <- Event{Type: EventLeave, Client: name}
	cn.Close()
}

// writeLoop flushes every message the room actor sends to outbox out to cn,
// until outbox is closed (the room removed this client or shut down).
func writeLoop(cn io.ReadWriteCloser, outbox <-chan string) {
	w := bufio.NewWriter(cn)
	for s := range outbox {
		w.WriteString(s)
		w.Flush()
	}
}
