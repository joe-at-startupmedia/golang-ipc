package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

func NewActor(ac *ActorConfig) Actor {

	logger := logrus.New()
	var logLevel logrus.Level
	if ac.IsServer && ac.ServerConfig != nil {
		logLevel = getLogrusLevel(ac.ServerConfig.LogLevel)
	} else if !ac.IsServer && ac.ClientConfig != nil {
		logLevel = getLogrusLevel(ac.ClientConfig.LogLevel)
	} else {
		logLevel = getLogrusLevel("")
	}
	if logLevel > logrus.WarnLevel {
		logger.SetReportCaller(true)
	}
	logger.SetLevel(logLevel)
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	return Actor{
		name:       ac.Name,
		status:     NotConnected,
		received:   make(chan *Message),
		toWrite:    make(chan *Message),
		maxMsgSize: ac.MaxMsgSize,
		isServer:   ac.IsServer,
		logger:     logger,
	}
}

// Read - blocking function, reads each message recieved
// if MsgType is a negative number its an internal message
func (a *Actor) Read() (*Message, error) {

	m, ok := <-a.received
	if !ok {
		err := errors.New("the received channel has been closed")
		//a.logger.Errorf("Actor.Read err: %e", err)
		return nil, err
	}

	if m.Err != nil {
		a.logger.Errorf("Actor.Read err: %e", m.Err)
		if !a.isServer {
			close(a.received)
			close(a.toWrite)
		}
		return nil, m.Err
	}

	return m, nil
}

// Write - writes a  message to the ipc connection.
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (a *Actor) Write(msgType int, message []byte) error {

	if msgType == 0 {
		err := errors.New("message type 0 is reserved")
		a.logger.Errorf("Actor.Write err: %s", err)
		return err
	}

	if a.isServer && a.status == Listening {
		time.Sleep(time.Millisecond * 2)
		a.logger.Infoln("Server is still listening so lets use recursion")
		//it's possible the client hasn't connected yet so retry it
		return a.Write(msgType, message)
	}
	if a.status != Connected {
		err := errors.New(fmt.Sprintf("cannot write under current status: %s", a.status.String()))
		a.logger.Errorf("Actor.Write err: %s", err)
		return err
	}

	mlen := len(message)
	if mlen > a.maxMsgSize {
		err := errors.New("message exceeds maximum message length")
		a.logger.Errorf("Actor.Write err: %s", err)
		return err
	}

	a.toWrite <- &Message{MsgType: msgType, Data: message}

	return nil
}

func (a *Actor) write() {

	for {

		m, ok := <-a.toWrite

		if !ok {
			break
		}

		toSend := append(intToBytes(m.MsgType), m.Data...)
		writer := bufio.NewWriter(a.conn)
		//first send the message size
		writer.Write(intToBytes(len(toSend)))
		//last send the message
		writer.Write(toSend)

		err := writer.Flush()
		if err != nil {
			a.logger.Errorf("error flushing data: %s", err)
			continue
		}

		time.Sleep(2 * time.Millisecond)

	}
}

// getStatus - get the current status of the connection
func (a *Actor) getStatus() Status {
	return a.status
}

// StatusCode - returns the current connection status
func (a *Actor) StatusCode() Status {
	return a.status
}

// Status - returns the current connection status as a string
func (a *Actor) Status() string {

	return a.status.String()
}

// Close - closes the connection
func (a *Actor) Close() {

	a.status = Closing

	if a.conn != nil {
		a.conn.Close()
	}
}
