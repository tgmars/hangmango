# Hangmango
Hangmango is a hangman game that operates over tcp sockets. It features a robust server that accepts multiple concurrent connections and a client that implements a basic command line interface.

## Build
Building a working Hangmango client and server requires execution of `go build` for both the client and server directories.

With the GOPATH variable set as `/home/yourusername/go`, there are two options to fetch the source prior to building:
1. `go get -v -u github.com/tgmars/hangmango` **NOTE:** this will require setting `HTTPS_PROXY` proxy envar if behind a corporate proxy. 
  
    **OR**
2. Extract the .zip into your $GOPATH, this will create the required folders and extract files in the correct structure.

Following either of the options above, the following structure should be present within your $GOPATH.
```
user@host:~/go/src/github.com/tgmars/hangmango$ 
.                                                                                                                                                                                      
├── app
│   ├── client
│   │   └── client.go
│   ├── server
│   │   ├── hangman.go
│   │   └── server.go
│   └── wordlist.txt
├── readme.md
├── startClient.sh
└── startServer.sh
```
From `~/go/src/github.com/tgmars/hangmango`, execute the following commands to build hangmanclient and hangmanserver into the intended locations. `startClient.sh` and `startServer.sh` require these locations to execute.
```
cd app/client/;go build -o ../hangmanclient;cd ../..;
cd app/server/;go build -o ../hangmanserver;cd ../..;
```
You are now ready to run **Hangmango!**

---
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
--- 
## Features and Design Considerations
The following section describes the wordlist feature and considerations applied in regards to security and architecture of the client-server model.
### Wordlists
A hardcoded list of default words to be selected from for a game of Hangmango includes `apple hello laminate sorcerer willow`, to expand this list the contents of the included `wordlist.txt` should be edited. It must contain newline separated words. The default contents of `wordlist.txt` is `here these are extra words for hangman tangible tarantula fantastic`. 

### Security
Whilst there is no protection against MiTM attacks until encryption is implemented, data validation has been considered in the development of both the client and server. Messages must match regex identifiers, messages greater than specified buffers (at the server) result in errors that are handled gracefully, and information of server operation is logged and verbosely presented to STDOUT. Encryption is a work in progress and is documented under the Encryption header below.

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

### Encrypted Communications
Prior to operating the layer 7 hangman protocol, we establish an encrypted session betweent the client and server.
1. Client retrieves the public key from the specified Hangmango server, by sending a `PUBKEYREQ` message to the server. The server responds with it's RSA public key.
2. Client stores the public key of the server in memory.
3. Client generates a new keypair and sends a `PUBKEYRESP` that includes the clients public key to the Hangmango server.
4. Server only accepts this incoming data if its from a known host.  
5. Server stores the public key of the client in a NoSQL database that maintains all data associated with a client.
6. Client and server progress to play game over the hangman protocol.

- Following a PUBKEYREQ message while the client is already running, the client should make their serverPubKey = (rsa.PublicKey{})

**NOTE:** Some client and server side validation on data received over sockets will need modifying to account for increased data sizes due to encryption and transmission of public keys.  

### Mitigating Cheating
**Encryption** - Encryption will increase the cost for an attacker for conduct a MitM attack on Hangmango communicates. Public key encryption has been chosen as it scales well in terms of cost of implementation and security. Without a verification of the public key by a CA, and checks that valid certificates are used, the server could be impersonated and the key exchange intercepted, allowing for an attacker masquerade as a valid server. 

**Hashing** - A hash calculated using details of the source, destination and contents of the message is calculated and sent along with the message. If the message has been modified in transit, or a client or servers address has been manipulated, the computed hash at the destination of the message will not match and be discarded. A failed hash message requires a modification to the protocol to pass a retry message back to the sender. Hashing will assure the integrity of the message. 

**Signing** - The encrypted messages must be signed to ensure their authenticity. 

**Sequencing** - A message sequence could be implemented to ensure that clients only receive the message currently intended for them. 

