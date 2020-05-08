package main

// receivelogic contains methods called from the receiveData() function in server.go

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
)

// Valid regex for servers receipt of client data
var regexpHangman = regexp.MustCompile(`^[a-zA-Z]+\s?(?:[a-zA-Z]+)?$`)

func receiverLogic(client *client, message []byte, length int) {
	// If the message is valid in length, format, etc, then we can parse it.
	if length > 0 {

		message = message[:length]
		var sMessage string

		// Our encryption establishment messages are of the form MSG{json-serialsed-rsa.PublicKey}
		// Our application messages are of the form, message\n so we need to handle them a bit differently.
		// Unless we move to serialising everything and working with a message struct
		// Thus, we handle two cases; where the message is encrypted and we parse out the underlying hangman protocol
		// or it's not encrypted yet because it's still a handshake and we parse it as is.
		if client.encrypted {
			message = decrypt(message, serverPrivKey)
			sMessage = strings.TrimRight(string(message), "\n")
		} else {
			sMessage = string(message)
		}

		// Validate message is within the regex set.
		// match := regexpHangman.Match([]byte(sMessage))
		match := true
		if !match {
			log.Printf("- FROM - %s - Invalid message received - EL:%d - %s", client.socket.RemoteAddr().String(), length, sMessage)
		} else {
			// If the message is valid; we can determine if a new client needs to be created, or to handle encryption
			// establishment.
			log.Printf("- FROM - %s - EL:%d - %s", client.socket.RemoteAddr().String(), length, sMessage)
			if client.state.valid {
				client.data <- []byte(client.state.process(sMessage))
				// If the last call to state.process set valid to false, we know the game is over and can
				// send a followup message to the client indicating so.
				if !client.state.valid {
					client.data <- []byte("GAME OVER")
				}
			} else {
				// Handle a PUBKEYREQ message
				if sMessage[:9] == "PUBKEYREQ" {
					handlePubKeyReq(client, message, length)
				}
				// Make a new game for the client
				if sMessage == "START GAME" {
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
func handlePubKeyReq(client *client, message []byte, length int) {
	err := json.Unmarshal(message[9:length], &client.pubkey)
	if err != nil {
		log.Printf("- ERROR - Deserialisation error - %s\n", err)
	} else {
		client.encrypted = true
		// provide our public key
		kObj, err := json.Marshal(serverPubKey)
		if err != nil {
			log.Printf("- ENCODING - %s", err)
		}
		header := []byte("PUBKEYRESP")
		msg := append(header, kObj...)
		client.data <- msg

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
	client.data <- []byte(client.state.hint)
}
