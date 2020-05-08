package main

// receivelogic contains methods called from the receiveData() function in server.go

import (
	"encoding/json"
	"log"
	"regexp"
)

// Valid regex for servers receipt of client data
var regexpHangman = regexp.MustCompile(`^[a-zA-Z]+\s?(?:[a-zA-Z]+)?$`)

func receiverLogic(client *client, message []byte, length int) {
	// If the message is valid in length, format, etc, then we can parse it.
	if length > 0 {
		// Decrypt the message using our private key
		// Parse the message in a separate function or go file
		message = message[:length]
		// this is really bad atm, because it decrypts the PUBKEYREQ message from the client.
		if client.encrypted {
			message = decrypt(message, serverPrivKey)
		}

		// sMessage := strings.TrimRight(string(message), "\n")
		sMessage := string(message)

		// Validate message is within the regex set.
		// match := regexpHangman.Match([]byte(sMessage))
		match := true
		if !match {
			log.Printf("- FROM - %s - Invalid message received - %s\n", client.socket.RemoteAddr().String(), sMessage)
		} else {
			// If the message is valid; we can determine if a new client needs to be created, or to handle encryption
			// establishment.
			log.Printf("- FROM - %s - %s\n", client.socket.RemoteAddr().String(), sMessage)
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
	log.Printf("- HANGMAN - New game created for this connection: %v\n", client.state)
	client.data <- []byte(client.state.hint)
}
