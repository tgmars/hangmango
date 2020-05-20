package main

import (
	"bufio"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
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
	encmsg    encryptedMessage
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

// Regex pattern for basic client side validation of string prior to sending to server.
var regexpHangman = regexp.MustCompile("[a-zA-Z]+")

// Regex pattern for basic client side validation of a string received from the server.
var regexpValidServerMessage = regexp.MustCompile("^[a-zA-Z_0-9 ]{1,100}$")

// Public key of the server, contains a RSA 2048 byte key once a PUBKEYRESP from the server is parsed.
var serverPubKey rsa.PublicKey

var serverCertificate, serverCertificateBytes, serverCertificatePubkey = initialiseSigning()

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
			// Multiple packets sent in rapid succession can fill the message buffer and get parsed
			// in a single iteration of the go routine. Thus, we catch this case by
			// splitting out the messages on newlines.
			// sMessages := strings.Split(strings.TrimRight(string(message[:n]), "\n"), "\n")
			// messages := bytes.Split(message, []byte("\n"))
			// for _, msg := range messages {
			receiveLogic(message, length, client)
			// }
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
			// message = append(message, '\n')
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
	encmsg := generateEncryptedMessage(bmsg)
	// fmt.Printf("%s\n", string(bmsg))
	client.data <- encmsg
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

	log.Printf("- DEBUG - FROM - Client received: %s", input)
	unMarshalMessage(input, client)

	// now we can access client.message.fields to parse out the different cases
	if client.message.Mtype == "PUBKEYRESP" {
		handlePubKeyResp(client)
	}
	if client.message.Mtype == "GAME OVER" {
		fmt.Printf("Game over! You scored: %s\n", client.message.Content)
		os.Exit(0)
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
		encryptJSONAddToChannel(client, bmsg)
	}
}

// unMarshalMessage is responsible for parsing encryptedMessage structs,
// verifying hashes to ensure sender identity and unmarshalling data back
// into message structs.
func unMarshalMessage(message []byte, client *client) {
	// Unmarshal our message into an encmsg struct
	err := json.Unmarshal(message, &client.encmsg)
	if err != nil {
		log.Printf("- ERROR - Deserialisation error occured for incoming encryptedMessage - %s\n", err)
	}
	// Verify the signature of the message
	hash := sha256.New()
	hash.Write(client.encmsg.A)
	encmsgAHashed := hash.Sum(nil)
	err = rsa.VerifyPSS(serverCertificatePubkey, crypto.SHA256, encmsgAHashed, client.encmsg.B, nil)
	if err != nil {
		log.Printf("- ERROR - Failed to verify message signature - %s\n", err)
	}

	var plaintext []byte
	if client.encrypted == true {
		plaintext = decrypt(client.encmsg.A, clientPrivKey)
	} else {
		plaintext = client.encmsg.A
	}

	err = json.Unmarshal(plaintext, &client.message)
	if err != nil {
		log.Printf("- ERROR - Deserialisation error occured for incoming message - %s\n", err)
	}
}

func encryptJSONAddToChannel(client *client, plaintextMessageJSON []byte) {
	encrypted := encrypt(plaintextMessageJSON, serverPubKey)
	encryptedAndValidated := generateEncryptedMessage(encrypted)
	client.data <- encryptedAndValidated
	log.Printf("- TO - %s - PT:%s\n", client.socket.RemoteAddr().String(), plaintextMessageJSON)
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

// Parse a byte representation of a message struct into a encryptedMessage struct and
// attach a message authentication token (signed copy of the message) to the encrypted messageStruct
func generateEncryptedMessage(messageBytes []byte) []byte {
	// we don't sign the byte encoded representation of the message struct for comms to the server
	// because we don't have certificates for the clients, and i need to trust the server, but don't care
	// if the clients go rogue.
	enc := encryptedMessage{A: messageBytes}
	// enc.Sign(&privkey)

	benc, err := json.Marshal(enc)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	return benc
}
