package main

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"strings"
	"time"
)

//  ClientManager ... manager that maintains
// connected clients and channels for connection and disconnnection
type ClientManager struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
}

type Client struct {
	socket net.Conn
	data   chan []byte
	state  HangmanState
	guid   string
}

// start ... handle connection and disconnection of clients
// from the ClientManager.
func (manager *ClientManager) start() {
	// Keep this gorouting running for the life of execution.
	for {
		select {
		case connection := <-manager.register:
			manager.clients[connection] = true
			fmt.Printf("Client connected from %v\n", connection.socket.RemoteAddr())
			// TODO: timeout the connection

		case connection := <-manager.unregister:
			// TODO - Cleanup the game state for this connection
			if _, ok := manager.clients[connection]; ok {
				close(connection.data)
				delete(manager.clients, connection)
			}
			manager.clients[connection] = false
			fmt.Printf("Client disconnected from %v\n", connection.socket.RemoteAddr())
		}
	}
}

// sendData ... send data to specified Client within the context of the
// current ClientManager. Flag characters are appended to the end of the
// message array prior to sending to the client.
func (manager *ClientManager) sendData(client *Client) {
	defer client.socket.Close()
	for {
		select {
		case message, ok := <-client.data:
			// If the data channel is not OK, return, handle an error first.
			if !ok {
				return
			}
			// Append a newline character to all of the messages being sent out from the server.
			message = append(message, []byte("\n")...)
			length, err := client.socket.Write(message)
			if err != nil {
				fmt.Println(err)
				return
			}
			if length > 0 {
				fmt.Printf("TO - %s - %v\n", client.socket.RemoteAddr().String(), message)
			}
		}
	}
}

// receiveData ... listen for incoming connections per client
// and process data depending on whether the client.state is valid
// or not. If state is invalid, it creates a new game for the client and responds
// with the first hint. If state is valid, the received data is passed to hangman
// to process the game.
func (manager *ClientManager) receiveData(client *Client) {
	for {
		// Create a byte slice limited in length to 4096.
		message := make([]byte, 4096)
		length, err := client.socket.Read(message)
		if err != nil {
			manager.unregister <- client
			client.socket.Close()
			break
		}
		// If the message is valid in length, format, etc, then we can parse it.
		if length > 0 {
			n := bytes.IndexByte(message, 0)
			sMessage := strings.TrimRight(string(message[:n]), "\n")
			fmt.Printf("FROM - %s - %s\n", client.socket.RemoteAddr().String(), sMessage)
			if client.state.valid {
				client.data <- []byte(client.state.process(sMessage))
				// If the last call to state.process set valid to false, we know the game is over and can
				// send a followup message to the client indicating so.
				if !client.state.valid {
					client.data <- []byte("GAME OVER")
				}
			} else {
				// If message is in list of commands: do what command says
				// Move this to a function within hangman.go
				if sMessage == "START GAME" {
					client.state = HangmanState{
						turn:    false,
						answer:  "",
						guesses: make([]string, 0),
						hint:    "",
						valid:   true,
					}
					client.state.NewGame()
					fmt.Printf("HANGMAN - New game created for this connection: %v\n", client.state)
					client.data <- []byte(client.state.hint)
				}
			}

		}
	}
}

func main() {
	// Parse flags
	flagLPort := flag.Int("LPORT", 4444, "Port to listen for incoming connections on.")
	flag.Parse()

	fmt.Println("Starting server...")
	listener, error := net.Listen("tcp", fmt.Sprintf(":%d", *flagLPort))
	if error != nil {
		fmt.Println(error)
	}
	fmt.Printf("Started server on %d\n", *flagLPort)
	manager := ClientManager{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	go manager.start()
	for {
		connection, _ := listener.Accept()
		if error != nil {
			fmt.Println(error)
		}

		client := &Client{socket: connection, data: make(chan []byte), guid: fmt.Sprintf("%d", time.Now().Unix())}
		manager.register <- client
		go manager.receiveData(client)
		go manager.sendData(client)
	}

}
