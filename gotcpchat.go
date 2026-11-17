package nnnchat

import (
	"log/slog"
	"net"

	"github.com/niumao/nnnchat/internal/chatapi"
)

// RunTCP will start chat tcp server on the provided connection string
func RunTCP(connection string) error {
	return RunTCPWithExistingAPI(connection, chatapi.New())
}

// RunTCPWithExistingAPI will start chat tcp server on the provided connection string using an existing chat api session
func RunTCPWithExistingAPI(connection string, chat *chatapi.ChatAPI) error {
	l, err := net.Listen("tcp", connection)
	if err != nil {
		slog.Error("error listening for chat clients", "err", err)
		return err
	}
	defer l.Close()
	//chat := chatapi.New()
	for {
		conn, err := l.Accept()
		if err != nil {
			break
		}
		go func(c net.Conn) {
			chat.AddClient(c)
		}(conn)
	}

	return err
}
