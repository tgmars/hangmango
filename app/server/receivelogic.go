package main

// receivelogic contains methods called from the receiveData() function in server.go

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
)

// Valid regex for servers receipt of client data
var regexpHangman = regexp.MustCompile(`^[a-zA-Z]+\s?(?:[a-zA-Z]+)?$`)

func receiverLogic(client *client, message []byte, length int) {
	// If the message is valid in length, format, etc, then we can parse it.
	if length > 0 {

		message = message[:length]

		// Our encryption establishment messages are of the form MSG{json-serialsed-rsa.PublicKey}
		// Our application messages are of the form, message\n so we need to handle them a bit differently.
		// Unless we move to serialising everything and working with a message struct
		// Thus, we handle two cases; where the message is encrypted and we parse out the underlying hangman protocol
		// or it's not encrypted yet because it's still a handshake and we parse it as is.
		if client.encrypted {
			message = decrypt(message, serverPrivKey)
			unMarshalMessage(message, client)
		} else {
			unMarshalMessage(message, client)
		}

		log.Printf("- DEBUG - parsed message struct - %v", client.message)

		// Validate message is within the regex set.
		// match := regexpHangman.Match([]byte(sMessage))
		match := true
		if !match {
			log.Printf("- FROM - %s - Invalid message received - EL:%d - %s", client.socket.RemoteAddr().String(), length, fmt.Sprintf("%s", client.message))
		} else {
			// If the message is valid; we can determine if a new client needs to be created, or to handle encryption
			// establishment.
			log.Printf("- FROM - %s - EL:%d - %s", client.socket.RemoteAddr().String(), length, fmt.Sprintf("%s", client.message))
			// also need to check if client.mesage.Content is valid within the character set here.
			if client.state.valid && client.message.Mtype == "" && len(client.message.Content) > 0 {
				// Pass the plaintext message off to hangman to process it
				messageJSON := generateHangmanJSONMessage([]byte(client.state.process(string(client.message.Content))))
				encryptJSONAddToChannel(client, messageJSON)
				// If the last call to state.process set valid to false, we know the game is over and can
				// send a followup message to the client indicating so.
				if !client.state.valid {
					handleGameOver(client)
				}
			} else {
				// Handle a PUBKEYREQ message
				if client.message.Mtype == "PUBKEYREQ" {
					handlePubKeyReq(client)
				}
				// Make a new game for the client
				if client.message.Mtype == "" && bytes.Equal(client.message.Content, []byte("START GAME")) {
					log.Printf("- DEBUG - handling START GAME ")
					handleStartGameReq(client)
				}
			}
		}
	}
}

// Handler functions for various messages

// handlePubKeyReq ... executes the logic required of the server
// when a client sent a REQPUBKEY message. The result is sent on the
// data channel as a slice of bytes to the client passed to the function
func handlePubKeyReq(client *client) {
	err := json.Unmarshal(client.message.Content, &client.pubkey)
	fmt.Println(string(client.message.Content))
	if err != nil {
		log.Printf("- ERROR - Deserialisation error - %s\n", err)
	} else {
		client.encrypted = true
		// provide our public key
		kObj, err := json.Marshal(serverPubKey)
		if err != nil {
			log.Printf("- ENCODING - %s", err)
		}
		msg := message{Mtype: "PUBKEYRESP", Content: kObj}
		bmsg, err := json.Marshal(msg)
		if err != nil {
			log.Printf("- ENCODING - %s", err)
		}
		client.data <- bmsg
		// Clear the message to nil
		client.message = message{}
	}
}

// handleStartGameReq ... executes the logic required of the server
// when a client sent a START GAME message. The result is sent on the
// data channel as a slice of bytes to the client passed to the function
func handleStartGameReq(client *client) {
	client.state = HangmanState{
		turn:        false,
		answer:      "",
		guesses:     make([]string, 0),
		wordguesses: make([]string, 0),
		hint:        "",
		valid:       true,
	}
	client.state.NewGame()
	log.Printf("- HANGMAN - New game created for this connection: %v", client.state)
	messageJSON := generateHangmanJSONMessage([]byte(client.state.hint))
	encryptJSONAddToChannel(client, messageJSON)
}

// handleGameOver ... partially implemented, does not send score result yet
func handleGameOver(client *client) {
	messageJSON := generateHangmanJSONMessage([]byte("GAME OVER"))
	encryptJSONAddToChannel(client, messageJSON)
}

func unMarshalMessage(message []byte, client *client) {
	err := json.Unmarshal(message, &client.message)
	if err != nil {
		log.Printf("- ERROR - Deserialisation error occured for incoming message - %s\n", err)
	}
}

func encryptJSONAddToChannel(client *client, plaintextMessageJSON []byte) {
	encrypted := encrypt(plaintextMessageJSON, client.pubkey)
	client.data <- encrypted
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
