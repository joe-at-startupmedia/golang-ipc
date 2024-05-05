package ipc

import (
	"github.com/sirupsen/logrus"
	"net"
	"time"
)

type Actor struct {
	name       string
	conn       net.Conn
	status     Status
	received   chan (*Message)
	toWrite    chan (*Message)
	maxMsgSize int
	isServer   bool
	logger     *logrus.Logger
}

// Server - holds the details of the server connection & config.
type Server struct {
	Actor
	listen net.Listener
	unMask bool
}

// Client - holds the details of the client connection and config.
type Client struct {
	Actor
	timeout    time.Duration //
	retryTimer time.Duration // number of seconds before trying to connect again
}

// Message - contains the received message
type Message struct {
	Err     error  // details of any error
	MsgType int    // 0 = reserved , -1 is an internal message (disconnection or error etc), all messages recieved will be > 0
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

type ActorConfig struct {
	Name         string
	MaxMsgSize   int
	IsServer     bool
	ServerConfig *ServerConfig
	ClientConfig *ClientConfig
}

// ServerConfig - used to pass configuation overrides to ServerStart()
type ServerConfig struct {
	MaxMsgSize        int
	UnmaskPermissions bool
	LogLevel          string
}

// ClientConfig - used to pass configuation overrides to ClientStart()
type ClientConfig struct {
	Timeout    time.Duration
	RetryTimer time.Duration
	LogLevel   string
}
