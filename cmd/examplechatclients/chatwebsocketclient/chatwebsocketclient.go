package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"

	"github.com/coder/websocket"
)

func main() {
	name := fmt.Sprintf("User%d", rand.IntN(400))
	clientName := flag.String("client", name, "name of the client to connect")
	url := flag.String("address", "ws://localhost:8099/mychat", "address of the chat server")
	roomName := flag.String("room", "lounge", "name of the chat room")
	token := flag.String("token", "", "JWT to present when the server requires auth")
	flag.Parse()

	ctx := context.Background()
	ws, _, err := websocket.Dial(ctx, *url, nil)
	if err != nil {
		slog.Error("websocket dial error", "err", err)
		os.Exit(1)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")

	go func() {
		for {
			_, msg, err := ws.Read(ctx)
			if err != nil {
				// connection is closed or there is an error, exit
				slog.Info("connection seems to be closed or error occurred", "err", err)
				os.Exit(0)
			}
			fmt.Println(string(msg))
		}
	}()

	clientInfo := struct {
		Name  string `json:"name"`
		Room  string `json:"room"`
		Token string `json:"token"`
	}{*clientName, *roomName, *token}
	data, err := json.Marshal(clientInfo)
	if err != nil {
		slog.Error("could not encode client info", "err", err)
		os.Exit(1)
	}
	if err := ws.Write(ctx, websocket.MessageText, data); err != nil {
		slog.Error("websocket send error", "err", err)
		os.Exit(1)
	}
	fmt.Println("Start typing your messages")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()
		if err := ws.Write(ctx, websocket.MessageText, []byte(msg+"\n")); err != nil {
			slog.Error("websocket send error", "err", err)
			os.Exit(1)
		}
	}
}
