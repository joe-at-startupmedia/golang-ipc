package ipc

import (
	"crypto/cipher"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

type Actor struct {
	status    Status
	conn      net.Conn
	received  chan (*Message)
	toWrite   chan (*Message)
	logger    *logrus.Logger
	config    *ActorConfig
	cipher    *cipher.AEAD
	clientRef *Client
	mutex     *sync.Mutex
}

// Server - holds the details of the server connection & config.
type Server struct {
	Actor
	listener      net.Listener
	ServerManager *ServerManager
}

// Client - holds the details of the client connection and config.
type Client struct {
	Actor
	timeout    time.Duration //
	retryTimer time.Duration // number of seconds before trying to connect again
	ClientId   int
	maxMsgSize int //set in the handshake process dictated by the ServerConfig.MaxMsgSize value
}

type ServerManager struct {
	Servers      []*Server
	ServerConfig *ServerConfig
	Logger       *logrus.Logger
	mutex        *sync.Mutex
}

type ActorConfig struct {
	IsServer     bool
	ServerConfig *ServerConfig
	ClientConfig *ClientConfig
}

// ServerConfig - used to pass configuration overrides to ServerStart()
type ServerConfig struct {
	Name              string
	MaxMsgSize        int
	UnmaskPermissions bool
	LogLevel          string
	MultiClient       bool
	Encryption        bool
}

// ClientConfig - used to pass configuration overrides to ClientStart()
type ClientConfig struct {
	Name        string
	Timeout     time.Duration
	RetryTimer  time.Duration
	LogLevel    string
	MultiClient bool
	Encryption  bool
}

// Message - contains the received message
type Message struct {
	Err     error  // details of any error
	MsgType int    // 0 = reserved , -1 is an internal message (disconnection or error etc), all messages received will be > 0
	Data    []byte // message data received
	Status  string // the status of the connection
}

// Status - Status of the connection
type Status int

const (
	// NotConnected - 0
	NotConnected Status = iota
	// Listening - 1
	Listening
	// Connecting - 2
	Connecting
	// Connected - 3
	Connected
	// ReConnecting - 4
	ReConnecting
	// Closed - 5
	Closed
	// Closing - 6
	Closing
	// Error - 7
	Error
	// Timeout - 8
	Timeout
	// Disconnected - 9
	Disconnected
)

func (status Status) String() string {
	return [...]string{
		"Not Connected",
		"Listening",
		"Connecting",
		"Connected",
		"Reconnecting",
		"Closed",
		"Closing",
		"Error",
		"Timeout",
		"Disconnected",
	}[status]
}
