package main

// receivelogic contains methods called from the receiveData() function in server.go

import (
	"bytes"
	"crypto/rsa"
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
		unMarshalMessage(message, client)
		// log.Printf("- DEBUG - parsed message struct - %v", client.message)

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
				// Check if a hash was sent in the message, if it was, compare it against the servers known.
				// If it doesn't something has gone wrong and we kill? the game.
				if len(client.message.Hash) > 0 && bytes.Equal(client.message.Hash, client.gameHash) {
					log.Printf("- GAMEHASH - Gamehash sent from the client matched the server, we're proceeding - %v - %v", client.message.Hash, client.gameHash)
				} else if len(client.message.Hash) > 0 && !bytes.Equal(client.message.Hash, client.gameHash) {
					log.Printf("- GAMEHASH - Gamehash sent from the client is wrong, something went awry, killing the game.. - %v - %v", client.message.Hash, client.gameHash)
					client.socket.Close()
				}
				// Pass the plaintext message off to hangman to process it
				hangmanResponse := client.state.process(string(client.message.Content))
				// If the last call to state.process set valid to false, we know the game is over and can
				// send a followup message to the client indicating so. Otherwise keep playing the game.
				if !client.state.valid {
					handleGameOver(client, hangmanResponse)
				} else {
					messageJSON := generateHangmanJSONMessage([]byte(hangmanResponse))
					encryptJSONAddToChannel(client, messageJSON)
				}
			} else {
				// Handle a PUBKEYREQ message
				if client.message.Mtype == "PUBKEYREQ" {
					handlePubKeyReq(client)
				}
				if client.message.Mtype == "SYMKEYREQ" {
					handleSymKeyReq(client)
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

		benc := generateEncryptedMessageAndSign(bmsg, serverSignPrivKey)

		client.data <- benc
		// Clear the message to nil
		client.message = message{}
		client.encmsg = encryptedMessage{}
	}
}

// handleSymKeyReq ... generates a key for AEAD GCM encryption
// and shares it back to the client with a message that's encrypted using said key.
func handleSymKeyReq(client *client) {

	AEADKey, err := generateSymmetricKeyBytes(32)
	if err != nil {
		log.Printf("- CRYPTO - %s", err)
	}

	msg := message{Mtype: "SYMKEYRESP", Content: AEADKey}
	bmsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	encryptJSONAddToChannel(client, bmsg)
	// Now that the sym key has been sent off to client, we set the struct's
	// symmetric key so that future decryption occurs using it.
	client.symmetricKey = AEADKey

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
	client.generateGameHash()
	log.Printf("- HANGMAN - New game created for this connection: %v", client.state)
	// we need a customer messageJSON (not using generateHangmanJSONMessage because we overload the Hash field)
	messageStruct := message{Content: []byte(client.state.hint), Hash: client.gameHash}
	messageBytes, err := json.Marshal(messageStruct)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	encryptJSONAddToChannel(client, messageBytes)
}

// handleGameOver ... Generate a message with Mtype=GAME OVER and Content=score, encrypt and add to channel.
func handleGameOver(client *client, score string) {
	messageStruct := message{Mtype: "GAME OVER", Content: []byte(score)}
	messageBytes, err := json.Marshal(messageStruct)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	encryptJSONAddToChannel(client, messageBytes)
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

	var plaintext []byte
	if client.encrypted == true && len(client.symmetricKey) == 0 {
		plaintext = decrypt(client.encmsg.A, serverPrivKey)
	} else if client.encrypted == true && len(client.symmetricKey) > 0 {
		plaintext = decryptAEADGCM(client.symmetricKey, client.encmsg.A, client.encmsg.B)
	} else {
		plaintext = client.encmsg.A
	}

	err = json.Unmarshal(plaintext, &client.message)
	if err != nil {
		log.Printf("- ERROR - Deserialisation error occured for incoming message - %s\n", err)
	}
}

func encryptJSONAddToChannel(client *client, plaintextMessageJSON []byte) {
	var encrypted []byte
	var nonce []byte
	var encryptedAndValidated []byte

	if len(client.symmetricKey) > 0 {
		encrypted, nonce = encryptAEADGCM(client.symmetricKey, plaintextMessageJSON)
		encryptedAndValidated = generateEncryptedMessage(encrypted, nonce)
	} else {
		encrypted = encrypt(plaintextMessageJSON, client.pubkey)
		encryptedAndValidated = generateEncryptedMessageAndSign(encrypted, serverSignPrivKey)
	}
	client.data <- encryptedAndValidated
	log.Printf("- TO - %s - PT:%s\n", client.socket.RemoteAddr().String(), plaintextMessageJSON)
	client.message = message{}
	client.encmsg = encryptedMessage{}
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
func generateEncryptedMessageAndSign(messageBytes []byte, certificatePrivkey rsa.PrivateKey) []byte {
	// sign the byte encoded representation of the message struct
	enc := encryptedMessage{A: messageBytes}
	enc.Sign(&certificatePrivkey)

	benc, err := json.Marshal(enc)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	return benc
}

// Parse a byte representation of a message struct into a encryptedMessage struct
// using the encrypted output for the A field, and the nonce in the B field.
func generateEncryptedMessage(messageBytes []byte, nonce []byte) []byte {
	enc := encryptedMessage{A: messageBytes, B: nonce}
	benc, err := json.Marshal(enc)
	if err != nil {
		log.Printf("- ENCODING - %s", err)
	}
	return benc
}
