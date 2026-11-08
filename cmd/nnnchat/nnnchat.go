package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/niumao/nnnchat"
	"github.com/niumao/nnnchat/chatapi"
	"github.com/niumao/nnnchat/history"

	"github.com/redis/go-redis/v9"
)

// historySize caps how many past messages are replayed per room; keep in
// sync with chatapi's own historySize used for the Recent() call.
const historySize = 50

func main() {
	tcpAddr := flag.String("tcp", "localhost:8989", "Address for the TCP chat server to listen on")
	wsAddr := flag.String("ws", "localhost:8099", "Address for the websocket chat server to listen on")
	redisAddr := flag.String("redis", "", "Address of a Redis server used to persist chat history (empty disables history)")
	flag.Parse()
	store := newHistoryStore(*redisAddr)
	api := chatapi.NewWithHistory(store)

	fmt.Printf("NNNChat server starting - tcp=%s ws=%s history=%t\n", *tcpAddr, *wsAddr, store != nil)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Println("Shutting down chat server")
		api.Close()
		os.Exit(0)
	}()

	go func() {
		nnnchat.RunWSChatWithExistingAPI("/mychat", *wsAddr, api)
	}()
	if err := nnnchat.RunTCPWithExistingAPI(*tcpAddr, api); err != nil {
		log.Fatalf("Could not listen on %s, error %s \n", *tcpAddr, err)
	}
}

// newHistoryStore returns a Redis-backed history.Store for addr, or nil if
// addr is empty or Redis is not reachable (chat still works, just without history).
func newHistoryStore(addr string) history.Store {
	if addr == "" {
		return nil
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis at %s not reachable, chat history disabled: %s \n", addr, err)
		return nil
	}
	return history.NewRedisStore(rdb, historySize)
}
