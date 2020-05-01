# Hangmango
Hangmango is a hangman game that operates over tcp sockets. It features a robust server that accepts multiple concurrent connections and a client that implements a basic command line interface.

## Usage
Hangmango has two binaries for execution, `hangmanserver` and `hangmanclient`, these will parse command line arguments with help text. For simple usage, `startClient.sh` and `startServer.sh` are both scripts that will execute the binaries, taking an IP address and port number as positional arguments.
#### Primary usage - Bash scripts
`startServer.sh int_port_to_listen_on`

`startClient.sh server_ip_address server_port_to_connect_to`
#### Secondary usage - Binary executions
```
Usage of ../hangmanserver:
  -lport int
        Port to listen for incoming connections on. (default 4444)
  -wordlist string
        Path to a newline separated list of words to use as a valid set of answers in a hangman game. (optional)
```
```
Usage of ../hangmanclient:
  -dhost string
        Hangmango server IPv4 address to connect to. (default "127.0.0.1")
  -dport int
        Port that the target Hangmango server is listening on. (default 4444)
```

### Wordlists
A hardcoded list of default words to be selected from for a game of Hangmango includes `apple hello laminate sorcerer willow`, to expand this list the contents of the included `wordlist.txt` should be edited. It must contain newline separated words. The default contents of `wordlist.txt` is `here these are extra words for hangman tangible tarantula fantastic`. 

### Security
Whilst there is no protection against MiTM attacks until encryption is implemented, data validation has been considered in the development of both the client and server. Messages must match regex identifiers, messages greater than specified buffers (at the server) result in errors that are handled gracefully, and information of server operation is logged and verbosely presented to STDOUT.

### Architecture
Hangmango uses a ClientManager struct and Golang's concept of channels to 'register' and 'unregister' clients from the server. A channel allows us to manipulate data in a concurrency safe manner within Goroutines. Upon receipt of a valid `START GAME` message, a new client object is created. Within that client object, a hangman game state is created and associated with the current connection. 

Both client and server initate their send() and receive() functions as Goroutines. Within each of these Goroutines, data that is transferred over sockets is directed to the data channel for each client. Data that conforms to the required length is then read off of the data channel for further processing per the hangman protocol. Running these functions as Goroutines enables us to scale out for concurrent client connections with ease.

The code responsible for implementing the rules of the hangman game are stored in `hangman.go`. A new hangman game is created for each valid incoming connection. New games select words from the list, seeded with the current time.   

### Protocol
The hangmango protocol is per the specifications of UNE's COSC540 assessment 2. Namely:

Each message in the protocol consists of a string of ASCII characters followed by the line feed character (ASCII code 10). Other than the `START GAME` message, all messages from the client should be case insensitive. Other than the `GAME OVER` message, all messages from the server should be lowercase.

The game begins with the client sending a `START GAME` message to the server.

The server responds to the `START GAME` message with the first hint. From this point on, any single character message consisting of a letter from A-Z or a-z from the client is treated as a letter guess, and any message consisting of more than one letter from A-Z or a-z is treated as a word guess.

After a letter guess, the server responds with a hint if there are still letters in the secret word that have not yet been guessed.

After a word guess, the server responds with a hint if the guess was different from the secret word.

Once the client has guessed the secret word (by either sending a correct word guess or guessing each letter in the secret word), the server sends the client's score followed by a `GAME OVER` message and then ends the connection.

Any other message sent to or from the client is considered an error, and should result in the receiving party dropping the connection. In particular, any client guess that includes characters outside the range of A-Z or a-z must be considered an error by the server.