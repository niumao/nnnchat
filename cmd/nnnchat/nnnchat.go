package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/niumao/nnnchat"
	"github.com/niumao/nnnchat/internal/chatapi"
	"github.com/niumao/nnnchat/internal/history"
)

// historySize caps how many past messages are replayed per room; keep in
// sync with chatapi's own historySize used for the Recent() call.
const historySize = 50

func main() {
	tcpAddr := flag.String("tcp", "localhost:8989", "Address for the TCP chat server to listen on")
	wsAddr := flag.String("ws", "localhost:8099", "Address for the websocket chat server to listen on")
	redisAddr := flag.String("redis", "", "Address of a Redis server used to persist chat history (empty disables history)")
	flag.Parse()

	var store history.Store
	if *redisAddr != "" {
		s, err := history.New(history.Config{Driver: history.DriverRedis, Addr: *redisAddr, Cap: historySize})
		if err != nil {
			slog.Warn("history disabled", "err", err)
		} else {
			store = s
		}
	}
	api := chatapi.NewWithHistory(store)

	slog.Info("NNNChat server starting", "tcp", *tcpAddr, "ws", *wsAddr, "history", store != nil)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		slog.Info("shutting down chat server")
		api.Close()
		os.Exit(0)
	}()

	go func() {
		nnnchat.RunWSChatWithExistingAPI("/mychat", *wsAddr, api)
	}()
	if err := nnnchat.RunTCPWithExistingAPI(*tcpAddr, api); err != nil {
		slog.Error("could not listen", "addr", *tcpAddr, "err", err)
		os.Exit(1)
	}
}
