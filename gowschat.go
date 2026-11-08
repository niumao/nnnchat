package nnnchat

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/niumao/nnnchat/internal/chatapi"

	"github.com/coder/websocket"
)

type websocketConnWrapper struct {
	*websocket.Conn
	msgType websocket.MessageType
}

func (wsWrapper *websocketConnWrapper) Read(b []byte) (int, error) {
	t, r, err := wsWrapper.Conn.Reader(context.Background())
	wsWrapper.msgType = t
	if err != nil {
		log.Println("Websocket err:", err)
		return 0, err
	}
	return r.Read(b)
}

func (wsWrapper *websocketConnWrapper) Write(p []byte) (int, error) {
	w, err := wsWrapper.Conn.Writer(context.Background(), wsWrapper.msgType)
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

type wsChatHandler struct {
	AcceptOptions *websocket.AcceptOptions
	*chatapi.ChatAPI
}

// RunWSChatWithExistingAPI starts a websocket chat server with an existing chat api session
func RunWSChatWithExistingAPI(url, address string, chat *chatapi.ChatAPI) error {
	wsHandler := &wsChatHandler{
		AcceptOptions: &websocket.AcceptOptions{},
		ChatAPI:       chat,
	}
	http.HandleFunc(url, wsHandler.wshandler)
	return http.ListenAndServe(address, nil)
}

// RunWSChat starts a websocket chat server
func RunWSChat(url, address string) error {
	return RunWSChatWithExistingAPI(url, address, chatapi.New())
}

func (wh *wsChatHandler) wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, wh.AcceptOptions)
	if err != nil {
		log.Println("Error occured while trying to upgrade websocket", err)
		fmt.Fprintf(w, "Error occured while trying to upgrade websocket: %v", err)
		return
	}

	wsWrapper := &websocketConnWrapper{Conn: conn}
	wh.AddClient(wsWrapper)
}
