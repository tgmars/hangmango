package main

import (
	"bufio"
	"crypto/rsa"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

// client ... maintains the client's state and communications channels
type client struct {
	socket    net.Conn
	data      chan []byte
	guid      string
	encrypted bool
	message   message
}

type message struct {
	Mtype     string `json:",omitempty"`
	Content   []byte `json:",omitempty"`
	Hash      []byte `json:",omitempty"`
	Signature []byte `json:",omitempty"`
}

// Regex pattern for basic client side validation of string prior to sending to server.
var regexpHangman = regexp.MustCompile("[a-zA-Z]+")

// Regex pattern for basic client side validation of a string received from the server.
var regexpValidServerMessage = regexp.MustCompile("^[a-zA-Z_0-9 ]{1,100}$")

// Public key of the server, contains a RSA 2048 byte key once a PUBKEYRESP from the server is parsed.
var serverPubKey rsa.PublicKey

// serverPrivKey and serverPubKey are RSA 2048 byte length keys
var clientPrivKey, clientPubKey = initialiseEncryption()

// receive() ... Reads data off the clients socket into a 4096 byte array and prints,
// formats them and prints to the user. This function is called
// as a goroutine from main()
func (client *client) receive() {
	for {
		message := make([]byte, 4096)
		length, err := client.socket.Read(message)
		if err != nil {
			fmt.Printf("ERROR - Reading from socket - %s\n", err)
			client.socket.Close()
			fmt.Println("CLIENT - Exiting hangmango client")
			os.Exit(1)

		}
		if length > 0 {
			receiveLogic(message, length, client)
		}
	}
}

// send() ... Handles the case where data is put onto
// the client.data channel. The data is read off the channel
// and sent out the socket to the server, logging to stdout.
func (client *client) send() {
	defer client.socket.Close()
	for {
		select {
		case message, ok := <-client.data:
			if !ok {
				return
			}
			_, err := client.socket.Write(message)
			if err != nil {
				fmt.Printf("ERROR - %s\n", err)
				fmt.Println("CLIENT - Exiting hangmango client")
				os.Exit(1)
			}
			// fmt.Printf("TO - %s - %v\n", client.socket.RemoteAddr().String(), string(message))
		}
	}
}

// main ... initiate connection to the server, make a new client struct,
// start the send and receive goroutines, send the client the START GAME
// message and wait for user input.
func main() {
	flagDAddress := flag.String("dhost", "127.0.0.1", "Hangmango server IPv4 address to connect to.")
	flagDPort := flag.Int("dport", 4444, "Port that the target Hangmango server is listening on.")
	flag.Parse()

	fmt.Println(`STARTUP - Welcome to hangmango! You will be presented with hints to guess a word selected by the server. 
	  You can enter guesses as individual english alphabet characters or an entire word. 
	  Incorrect guesses will deduct from your score per the following forumla: 
	  10 * (number of letters in secret word) - 2 * (number of characters guessed) - (number of words guessed)`)

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", *flagDAddress, *flagDPort))
	if err != nil {
		exitString := `ERROR - Unable to connect to specified hangmango server, likely that it's not 
		  running or a network device is preventing the connection. The raw error is below.`
		fmt.Printf(exitString+"\nERROR - %s\n", err)
		fmt.Println("CLIENT - Exiting hangmango client")
		os.Exit(1)
	}

	// Initialise the client struct that represents this client
	client := &client{socket: conn, data: make(chan []byte), guid: fmt.Sprintf("%d", time.Now().Unix())}

	go client.send()
	go client.receive()
	initPubKeyReq(client)

	// Wait for user input and send anything that matches simple client side validation to the server.
	for {
		// Block until
		reader := bufio.NewReader(os.Stdin)
		message, _ := reader.ReadString('\n')
		message = strings.TrimRight(message, "\n")
		// Validate message is within the regex set.
		match := regexpHangman.Match([]byte(message))
		// Validate message is in the regex set & hasn't completely filled the buffer from ReadString (4096 bytes)
		if match && (len([]byte(message)) <= 4095) {
			messageJSON := generateHangmanJSONMessage([]byte(message))
			encryptJSONAddToChannel(client, messageJSON)
		} else if match == false {
			fmt.Println("Input must be an upper or lowercase character in the english alphabet (a-z or A-Z).")
		} else if len([]byte(message)) >= 4096 {
			fmt.Println("Length of input must be less than 4096 bytes.")
		}
	}
}

func initPubKeyReq(client *client) {
	// provide our public key
	clientPubKeyBytes, err := json.Marshal(clientPubKey)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	// fmt.Println(string(clientPubKeyBytes))
	msg := message{Mtype: "PUBKEYREQ", Content: clientPubKeyBytes}
	bmsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	// fmt.Printf("%v\n", msg)
	// fmt.Printf("%s\n", string(bmsg))
	client.data <- bmsg
	// Clear the message to nil
	client.message = message{}

}

func receiveLogic(input []byte, length int, client *client) {
	// Multiple packets sent in rapid succession can fill the message buffer and get parsed
	// in a single iteration of the go rapidroutine. Thus, we catch this case by
	// splitting out the messages on newlines.
	input = input[:length]
	// Only parse PUBKEYRESP messages if we don't have a server key currently stored.
	// This implies that the message we'll receive won't be encrypted and can be treated as such.

	if client.encrypted == true {
		log.Println("Expecting encrypted data - need some way to verify this.")
		input = decrypt(input, clientPrivKey)
		unMarshalMessage(input, client)
	} else {
		unMarshalMessage(input, client)
	}

	// now we can access client.message.fields to parse out the different cases
	if client.message.Mtype == "PUBKEYRESP" {
		handlePubKeyResp(client)
	}
	// only hangmango application messages should meet this criteria.
	if (client.message.Mtype == "") && (len(client.message.Content) > 0) {
		fmt.Printf("%s\n", client.message.Content)
	}
}

// Handle the message containing a servers public key and initiate
// the game with them.
func handlePubKeyResp(client *client) {
	err := json.Unmarshal(client.message.Content, &serverPubKey)
	if err != nil {
		log.Printf("- ERROR - Deserialisation error - %s\n", err)
	} else {
		// log.Printf("Received pubkey size: %d", serverPubKey.Size())
		// fmt.Println(serverPubKey)
		client.encrypted = true
		msg := message{Content: []byte("START GAME")}
		bmsg, err := json.Marshal(msg)
		if err != nil {
			log.Printf("- ENCODING - %s", err)
		}
		encrypted := encrypt(bmsg, serverPubKey)
		client.data <- encrypted
		// Clear the message to nil
		client.message = message{}
	}
}

func unMarshalMessage(message []byte, client *client) {
	err := json.Unmarshal(message, &client.message)
	if err != nil {
		log.Printf("- ERROR - Deserialisation error occured for incoming message - %s\n", err)
	}
}

func encryptJSONAddToChannel(client *client, plaintextMessageJSON []byte) {
	encrypted := encrypt(plaintextMessageJSON, serverPubKey)
	client.data <- encrypted
	log.Printf("- TO - %s - %s\n", client.socket.RemoteAddr().String(), plaintextMessageJSON)
	client.message = message{}
}

func generateHangmanJSONMessage(msg []byte) []byte {
	messageStruct := message{Content: msg}
	messageBytes, err := json.Marshal(messageStruct)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	return messageBytes
}
