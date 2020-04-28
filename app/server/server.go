package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
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

var regexpHangman = regexp.MustCompile(`^[a-zA-Z]+\s?(?:[a-zA-Z]+)?$`)

// start ... handle connection and disconnection of clients
// from the ClientManager.
func (manager *ClientManager) start() {
	// Keep this gorouting running for the life of execution.
	for {
		select {
		case connection := <-manager.register:
			manager.clients[connection] = true
			log.Printf("- Client connected from %v\n", connection.socket.RemoteAddr())
			// TODO: timeout the connection

		case connection := <-manager.unregister:
			// TODO - Cleanup the game state for this connection
			if _, ok := manager.clients[connection]; ok {
				close(connection.data)
				delete(manager.clients, connection)
			}
			manager.clients[connection] = false
			log.Printf("- Client disconnected from %v\n", connection.socket.RemoteAddr())
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
				log.Println(err)
				return
			}
			if length > 0 {
				log.Printf("- TO - %s - %v\n", client.socket.RemoteAddr().String(), message)
			}
		}
	}
}

// receiveData ... listen for incoming connections per client
// and process data depending on whether the client.state is valid
// or not. If state is invalid, it creates a new game for the client and responds
// with the first hint. If state is valid, the received data is passed to hangman
// to process the game.error
// Implements some basic security checks & anticheat through ensuring data larger than
// the buffers aren't parsed and catches the errors.
func (manager *ClientManager) receiveData(client *Client) {
	for {
		// defer func() ... If we hit a panic because of slice index out of range, cleanly close the goroutine.
		// Normaly in Go we'd try to parse the length of the message instead of using the recover()
		// function, but it's an appropriate use for recover() because there's no way to check the
		// length of the data at the server before it's been stored somewhere in memory.
		defer func() {
			if err := recover(); err != nil {
				log.Println("- ERROR - Goroutine panicked, attempted to store too much data in message, connection closed:", err)
				manager.unregister <- client
				client.socket.Close()
			}
		}()
		// Create a byte slice limited in length to 4096 to hold the incoming data, anything over 4096 bytes
		// will cause the goroutine to panic, unwrapping back up the stack until we hit the defer function
		// and the call to recover()
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

			// Validate message is within the regex set.
			match := regexpHangman.Match([]byte(sMessage))
			if !match {
				log.Printf("- FROM - %s - Invalid message received - %s\n", client.socket.RemoteAddr().String(), sMessage)
				break
			}
			log.Printf("- FROM - %s - %s\n", client.socket.RemoteAddr().String(), sMessage)
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
						turn:        false,
						answer:      "",
						guesses:     make([]string, 0),
						wordguesses: make([]string, 0),
						hint:        "",
						valid:       true,
					}
					client.state.NewGame()
					log.Printf("- HANGMAN - New game created for this connection: %v\n", client.state)
					client.data <- []byte(client.state.hint)
				}
			}

		}
	}
}

// parseWordlist ... Append a list of newline separated values from
// a file specified by the path argument to the answerPool variable
// defined in package main -> hangman.go
func parseWordlist(path string) {
	if path != "" {
		file, err := os.Open(path)
		if err != nil {
			log.Println(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			answerPool = append(answerPool, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			log.Println(err)
		}
	}
}

func main() {
	// Parse flags
	flagLPort := flag.Int("lport", 4444, "Port to listen for incoming connections on.")
	flagWordlist := flag.String("wordlist", "", "Path to a newline separated list of words to use as a valid set of answers in a hangman game. (optional)")
	flag.Parse()

	parseWordlist(*flagWordlist)

	log.Println("- Starting server...")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *flagLPort))
	if err != nil {
		log.Println(err)
		log.Println("- SERVER - Exiting.")
		os.Exit(1)

	}
	log.Printf("- Started server on %d\n", *flagLPort)
	manager := ClientManager{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	go manager.start()
	for {
		connection, err := listener.Accept()
		if err != nil {
			log.Printf(" - ERROR - Error accepting connection from client: %s\n", connection.RemoteAddr().String())
			log.Println(err)
		}

		client := &Client{socket: connection, data: make(chan []byte), guid: fmt.Sprintf("%d", time.Now().Unix())}
		manager.register <- client
		go manager.receiveData(client)
		go manager.sendData(client)
	}
}
