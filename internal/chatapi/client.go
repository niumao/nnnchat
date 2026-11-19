package chatapi

import (
	"bufio"
	"io"
	"log/slog"
)

// startClientConn wires up a client's connection: it joins the room, then
// starts a reader goroutine (turns incoming lines into requests on the
// room's channels) and a writer goroutine (flushes whatever the room sends
// back to this client's outbox).
func startClientConn(name string, room *Room, cn io.ReadWriteCloser) {
	outbox := make(chan string, wcBufferSize)
	room.joins <- joinReq{name: name, outbox: outbox}

	go writeLoop(cn, outbox)
	go readLoop(cn, room, name)
}

// readLoop reads newline-delimited messages from cn and submits them to the
// room, until the connection is closed or errors.
func readLoop(cn io.ReadWriteCloser, room *Room, name string) {
	scanner := bufio.NewScanner(bufio.NewReader(cn))
	for scanner.Scan() {
		text := scanner.Text()
		slog.Info("chat message", "room", room.name, "client", name, "text", text)
		room.messages <- msgReq{name: name, text: text}
	}
	room.leaves <- name
	cn.Close()
}

// writeLoop flushes every message the room sends to outbox out to cn, until
// outbox is closed (the room removed this client or shut down).
func writeLoop(cn io.ReadWriteCloser, outbox <-chan string) {
	w := bufio.NewWriter(cn)
	for s := range outbox {
		w.WriteString(s)
		w.Flush()
	}
}
