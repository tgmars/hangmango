package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"time"
)

// generateGameHash ... populates the gamehash field for the provided client object
// the hash is a SHA256 hash generated from the current UTC time in minutes, game answer,
// and client and server IP:port. For example:
// 2020-05-23T04:24:00Z/hangmananswer/127.0.0.1:39214/127.0.0.1:4444
// if this code is running on a client, the RemoteAddr and LocalAddr should be swapped.
func (client *client) generateGameHash() {
	// Get the current minute
	m := time.Minute
	t := time.Now().UTC().Truncate(m)
	tbytes, err := t.MarshalText()
	if err != nil {
		log.Printf("- GAMEHASH - error marshalling current time to bytes - %s", err)
	}

	// Get the initially selected word by the server
	w := client.state.answer

	// Get the clients network details
	c := client.socket.RemoteAddr()
	// Get the servers network details
	s := client.socket.LocalAddr()

	// Concat the strings in an injective fashion using delims to ensure the hash is not trivially broken.
	formatted := []byte(fmt.Sprintf("%s/%s/%s/%s", tbytes, w, c, s))

	hash := sha256.New()
	hash.Write(formatted)
	hashed := hash.Sum(nil)

	client.gameHash = hashed
}

func (client *client) validateGameHash() bool {
	_ = client.gameHash
	return false
}
