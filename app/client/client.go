package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

// Client ... maintains the client's state and communications channels
type Client struct {
	socket net.Conn
	data   chan []byte
	guid   string
}

// Regex pattern for basic client side validation of string prior to sending to server.
var regexpHangman = regexp.MustCompile("[a-zA-Z]+")

// Regex pattern for basic client side validation of a string received from the server.
var regexpValidServerMessage = regexp.MustCompile("^[a-zA-Z_0-9 ]{1,100}$")

// receive() ... Reads data off the clients socket into a 4096 byte array and prints,
// formats them and prints to the user. This function is called
// as a goroutine from main()
func (client *Client) receive() {
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
			n := bytes.IndexByte(message, 0)
			// Multiple packets sent in rapid succession can fill the message buffer and get parsed
			// in a single iteration of the go rapidroutine. Thus, we catch this case by
			// splitting out the messages on newlines.
			sMessages := strings.Split(strings.TrimRight(string(message[:n]), "\n"), "\n")
			for _, msg := range sMessages {
				match := regexpValidServerMessage.Match([]byte(msg))
				if match {
					fmt.Printf("RECEIVED - %s\n", msg)
					if msg == "GAME OVER" {
						os.Exit(0)
					}
				} else {
					fmt.Println("ERROR - Invalid message received from server.")
					fmt.Println("CLIENT - Exiting hangmango client")
					os.Exit(1)
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
	client := &Client{socket: conn, data: make(chan []byte), guid: fmt.Sprintf("%d", time.Now().Unix())}

	go client.send()
	go client.receive()
	client.data <- []byte("START GAME\n")

	// Wait for user input and send anything that matches simple client side validation to the server.
	for {
		reader := bufio.NewReader(os.Stdin)
		message, _ := reader.ReadString('\n')
		message = strings.TrimRight(message, "\n")
		// Validate message is within the regex set.
		match := regexpHangman.Match([]byte(message))
		// Validate message is in the regex set & hasn't completely filled the buffer from ReadString (4096 bytes)
		if match && (len([]byte(message)) <= 4095) {
			client.data <- []byte(message)
		} else if match == false {
			fmt.Println("Input must be an upper or lowercase character in the english alphabet (a-z or A-Z).")
		} else if len([]byte(message)) >= 4096 {
			fmt.Println("Length of input must be less than 4096 bytes.")
		}
	}
}
