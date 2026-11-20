package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
)

func main() {
	name := fmt.Sprintf("User%d", rand.IntN(400))
	clientName := flag.String("client", name, "name of the client to connect")
	addr := flag.String("address", "localhost:8989", "address of the chat server")
	roomName := flag.String("room", "lounge", "name of the chat room")
	token := flag.String("token", "", "JWT to present when the server requires auth")
	flag.Parse()
	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		slog.Error("could not connect to the chat server", "err", err)
		os.Exit(1)
	}
	slog.Info("connected to the tcp chat system")

	defer conn.Close()
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
		//connection is closed or there is an error, exit
		slog.Info("connection seems to be closed or error occurred", "err", scanner.Err())
		os.Exit(0)
	}()
	clientInfo := struct {
		Name  string `json:"name"`
		Room  string `json:"room"`
		Token string `json:"token"`
	}{*clientName, *roomName, *token}
	data, _ := json.Marshal(clientInfo)
	conn.Write(data)
	fmt.Println("Start typing your messages")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() && err == nil {
		msg := scanner.Text()
		_, err = fmt.Fprintf(conn, msg+"\n")
	}
}
