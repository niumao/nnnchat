package nnnchat

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/niumao/nnnchat/internal/chatapi"

	"github.com/coder/websocket"
)

type websocketConnWrapper struct {
	*websocket.Conn
}

func (wsWrapper *websocketConnWrapper) Read(b []byte) (int, error) {
	_, r, err := wsWrapper.Conn.Reader(context.Background())
	if err != nil {
		slog.Error("websocket read error", "err", err)
		return 0, err
	}
	return r.Read(b)
}

func (wsWrapper *websocketConnWrapper) Write(p []byte) (int, error) {
	w, err := wsWrapper.Conn.Writer(context.Background(), websocket.MessageText)
	if err != nil {
		return 0, err
	}
	n, err := w.Write(p)
	if err != nil {
		w.Close()
		return n, err
	}
	return n, w.Close()
}

func (wsWrapper *websocketConnWrapper) Close() error {
	return wsWrapper.Conn.Close(websocket.StatusNormalClosure, "")
}

// RunWSChatWithExistingAPI starts a websocket chat server with an existing chat api session
func RunWSChatWithExistingAPI(url, address string, chat *chatapi.ChatAPI) error {
	acceptOptions := &websocket.AcceptOptions{}
	http.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, acceptOptions)
		if err != nil {
			slog.Error("error occurred while trying to upgrade websocket", "err", err)
			fmt.Fprintf(w, "Error occured while trying to upgrade websocket: %v", err)
			return
		}

		wsWrapper := &websocketConnWrapper{Conn: conn}
		chat.AddClient(wsWrapper)
	})
	return http.ListenAndServe(address, nil)
}

// RunWSChat starts a websocket chat server
func RunWSChat(url, address string) error {
	return RunWSChatWithExistingAPI(url, address, chatapi.New())
}
