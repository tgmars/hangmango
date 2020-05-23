package main

import (
	"bufio"
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

//  clientManager ... struct that maintains
// connected clients and channels for connection and disconnnection
type clientManager struct {
	clients    map[*client]bool
	register   chan *client
	unregister chan *client
}

// client ... struct that represents a client socket, the data channel
// to send and receive information on, and currently unused state & guid
type client struct {
	socket       net.Conn
	data         chan []byte
	state        HangmanState
	guid         string
	pubkey       rsa.PublicKey
	encrypted    bool
	message      message
	encmsg       encryptedMessage
	symmetricKey []byte
	gameHash     []byte
}

type message struct {
	Mtype     string `json:",omitempty"`
	Content   []byte `json:",omitempty"`
	Hash      []byte `json:",omitempty"`
	Signature []byte `json:",omitempty"`
}

// encryptedMessage ... Maintains two fields, A is the encrypted message and the other
// is the MAC validation/signature field.
type encryptedMessage struct {
	A []byte `json:"A,omitempty"`
	B []byte `json:"B,omitempty"`
}

// serverPrivKey and serverPubKey are RSA 2048 byte length keys
var serverPrivKey, serverPubKey, serverPubKeyJSON = initialiseEncryption()
var serverSignPrivKey = initialiseSigning()

// start ... handle connection and disconnection of clients
// from the clientManager.
func (manager *clientManager) start() {
	// Keep this gorouting running for the life of execution.
	for {
		select {
		case connection := <-manager.register:
			manager.clients[connection] = true
			log.Printf("- Client connected from %v", connection.socket.RemoteAddr())
			// TODO: timeout the connection

		case connection := <-manager.unregister:
			if _, ok := manager.clients[connection]; ok {
				close(connection.data)
				delete(manager.clients, connection)
			}
			manager.clients[connection] = false
			log.Printf("- Client disconnected from %v", connection.socket.RemoteAddr())
		}
	}
}

// sendData ... send data to specified client within the context of the
// current clientManager. Flag characters are appended to the end of the
// message array prior to sending to the client.
func (manager *clientManager) sendData(client *client) {
	defer client.socket.Close()
	for {
		select {
		case message, ok := <-client.data:
			// If the data channel is not OK, return, handle an error first.
			if !ok {
				return
			}
			// apply some error handling around this;
			// message = append(message, '\n')
			_, err := client.socket.Write(message)
			// length, err := client.socket.Write(message)
			if err != nil {
				log.Println(err)
				return
			}
			// if length > 0 {
			// }
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
func (manager *clientManager) receiveData(client *client) {
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
		receiverLogic(client, message, length)
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

	log.Println("- Parsing wordlist...")
	parseWordlist(*flagWordlist)

	log.Println("- Generating keypair...")

	log.Println("- Starting server...")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *flagLPort))
	if err != nil {
		log.Printf("- ERROR - %s\n", err)
		log.Println("- SERVER - Exiting.")
		os.Exit(1)

	}
	log.Printf("- Started server on %d\n", *flagLPort)
	manager := clientManager{
		clients:    make(map[*client]bool),
		register:   make(chan *client),
		unregister: make(chan *client),
	}
	go manager.start()
	for {
		connection, err := listener.Accept()
		if err != nil {
			log.Printf("- ERROR - Error accepting connection from client: %s\n", connection.RemoteAddr().String())
			log.Println(err)
		}

		client := &client{socket: connection, data: make(chan []byte), guid: fmt.Sprintf("%d", time.Now().Unix())}
		manager.register <- client
		go manager.receiveData(client)
		go manager.sendData(client)
	}
}
