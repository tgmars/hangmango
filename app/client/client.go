package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// Client ... maintains the client's state and communications channels
type Client struct {
	socket net.Conn
	data   chan []byte
	guid   string
}

// receive() ... Reads data off the clients socket into a 4096 byte array and prints,
// formats them and prints to the user. This function is called
// as a goroutine from main()
func (client *Client) receive() {
	for {
		message := make([]byte, 4096)
		length, err := client.socket.Read(message)
		if err != nil {
			fmt.Println(("ERROR: Reading from socket"))
			client.socket.Close()
			break
		}
		if length > 0 {
			n := bytes.IndexByte(message, 0)
			// Multiple packets sent in rapid succession can fill the message buffer and get parsed
			// in a single iteration of the go rapidroutine. Thus, we catch this case by
			// splitting out the messages on newlines.
			sMessages := strings.Split(strings.TrimRight(string(message[:n]), "\n"), "\n")
			for _, msg := range sMessages {
				fmt.Printf("RECEIVED: %s\n", msg)
				if msg == "GAME OVER" {
					os.Exit(0)
				}
			}
		}
	}
}

// send() ... Handles the case where data is put onto
// the client.data channel. The data is read off the channel
// and sent out the socket to the server, logging to stdout.
func (client *Client) send() {
	defer client.socket.Close()
	for {
		select {
		case message, ok := <-client.data:
			if !ok {
				return
			}
			_, err := client.socket.Write(message)
			if err != nil {
				fmt.Printf("ERROR: %s\n", err)
				break
				// return
			}
			// fmt.Printf("TO - %s - %v\n", client.socket.RemoteAddr().String(), string(message))
		}
	}
}

// main ... initiate connection to the server, make a new client struct,
// start the send and receive goroutines, send the client the START GAME
// message and wait for user input.
func main() {
	conn, err := net.Dial("tcp", "localhost:4444")
	if err != nil {
		// handle error
	}
	client := &Client{socket: conn, data: make(chan []byte), guid: fmt.Sprintf("%d", time.Now().Unix())}

	go client.send()
	go client.receive()
	client.data <- []byte("START GAME\n")

	for {
		reader := bufio.NewReader(os.Stdin)
		message, _ := reader.ReadString('\n')
		client.data <- []byte(message)
	}
}
