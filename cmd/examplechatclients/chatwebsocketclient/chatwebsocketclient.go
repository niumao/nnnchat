package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"

	"github.com/coder/websocket"
)

func main() {
	name := fmt.Sprintf("User%d", rand.IntN(400))
	clientName := flag.String("client", name, "name of the client to connect")
	url := flag.String("address", "ws://localhost:8099/mychat", "address of the chat server")
	roomName := flag.String("room", "lounge", "name of the chat room")
	flag.Parse()

	ctx := context.Background()
	ws, _, err := websocket.Dial(ctx, *url, nil)
	if err != nil {
		log.Fatal("websocket dial error ", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")

	go func() {
		for {
			_, msg, err := ws.Read(ctx)
			if err != nil {
				// connection is closed or there is an error, exit
				log.Println("Connection seem to be closed or error occured", err)
				os.Exit(0)
			}
			fmt.Println(string(msg))
		}
	}()

	clientInfo := struct {
		Name string `json:"name"`
		Room string `json:"room"`
	}{*clientName, *roomName}
	data, err := json.Marshal(clientInfo)
	if err != nil {
		log.Fatal("Could not encode client info", err)
	}
	if err := ws.Write(ctx, websocket.MessageText, data); err != nil {
		log.Fatal("Websocket send error", err)
	}
	fmt.Println("Start typing your messages")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()
		if err := ws.Write(ctx, websocket.MessageText, []byte(msg+"\n")); err != nil {
			log.Fatal("Websocket send error", err)
		}
	}
}
