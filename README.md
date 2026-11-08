***NNNChat***

----------

An open source chat server implemented in pure Go. The chat server listens on a TCP port and/or a websocket. It supports the following concepts:

 - Chat clients: A chat client represents the person who is sending and receiving messages
 - Chat rooms: A chat room is a place where multiple chat clients can be added. All clients connected to the same chat room will be able to see their messages
 - Chat history: Chat rooms can optionally persist recent messages to Redis and replay them to clients as they join

This is a fork of [minaandrawos/machat](https://github.com/minaandrawos/machat) with the following changes:

 - Replaced both the server's `gorilla/websocket` and the example client's `golang.org/x/net/websocket` with [`github.com/coder/websocket`](https://github.com/coder/websocket)
 - Removed the unused SQL/SQLite `dblayer` package
 - Added an optional Redis-backed chat history store (`history/`) so new clients can be replayed recent messages when joining a room
 - Bounded per-client message delivery with a small buffered channel and non-blocking sends, so one slow client can no longer pile up unbounded goroutines on the broadcaster
 - Fixed a goroutine leak where a disconnected client's writer goroutine was never signalled to stop
 - Moved SIGINT/SIGTERM shutdown handling out of the chat API and into the server binary (`cmd/nnnchat`), via an explicit `ChatAPI.Close()`
 - Renamed the Go module/package from `github.com/minaandrawos/machat` to `github.com/niumao/nnnchat` to match this fork

Pull requests are more than welcome

----------


**How it works?**

 - The NNNChat server listens on a TCP server as well as a websocket server specified by you for
   incoming messages
 - A NNNChat client starts the connection by:
	 -  Establishing a TCP or a websocket connection to the NNNChat server port
	 -  Sending a JSON string message to indicate that chat client name as well as the name of the chat room that the client would like to join. The JSON format looks like this:
	 `{"name": "Mina", "room":"devroom"}`
     - Chat rooms can be shared between TCP clients and websocket clients based on how the chat API is initialized

Once the name and chat room is sent to the chat server, the client can then proceed to send string messages via TCP or websockets,  the server will route the messages to the appropriate chat room and share them with other clients in the chat room. 

If chat history is enabled (see below), a newly joined client is first replayed the room's recent messages from Redis before it starts receiving live broadcasts.


----------


**How to run the server?**

The NNNChat server supports three user flags:
 - `-tcp` which is used to indicate the TCP server listening address. The default value is "localhost:8989"
 - `-ws` which is used to indicate the websocket server listening address. The default value is "localhost:8099"
 - `-redis` which is used to indicate the address of a Redis server used to persist chat history. Leaving this empty (the default) disables history

Example command:

    <executablename> -tcp localhost:8989 -ws localhost:8099 -redis localhost:6379

'executablename' here is the name of the executable you build for the chat server. If you would like to run the code directly from the go code:

    #Run the chat server with the default ports and history disabled
    go run cmd/nnnchat/nnnchat.go

  There is an example TCP chat client application code provided which supports three user flags:
  

 - `-client` : for client name
 - `-room`: for chat room name
 - `-address`: for the TCP address of the chat server,  defaulted to localhost:8989

To run the example client application with client name 'Mina' and chat room 'dev':

    go run cmd/examplechatclients/chattcpclient/chattcpclient.go -client Mina -room dev

  There is also an example for a websocket client application which supports three user flags:
 - `-client` : for client name
 - `-room`: for chat room name
 - `-address`: for the websocket address of the chat server, defaulted to ws://localhost:8099/mychat


----------


**What's checked in?**

 - *./gotcpchat.go*: entry point to the chat TCP server code
 - *./gowschat.go*: entry point to the chat websocket server code
 - *./chatapi/chatapi.go*: top level chat API, wires up chat rooms and the optional history store
 - *./chatapi/clients.go*: code to handle the chat clients, the code is exposed as an API to be used by gotcpchat.go
 - *./chatapi/rooms.go*: code to handle chat rooms, the code is exposed as an API to be used by gotcpchat.go
 - *./history/history.go*: the `Store` interface used to persist and replay chat room history
 - *./history/redis.go*: a Redis-backed implementation of `Store`
 - *./cmd/nnnchat/nnnchat.go*: main package for the NNNChat server, you need to build this file to get the executable for the chat server
 - *./cmd/examplechatclients/chattcpclient/chattcpclient.go*: main package for an example TCP client that can connect to the chat server
 - *./cmd/examplechatclients/chatwebsocketclient/chatwebsocketclient.go*: main package for an example websocket client that can connect to the chat server

 ----------
 
 For other articles and tutorials, check the original author's website at www.minaandrawos.com
