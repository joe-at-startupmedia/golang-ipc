package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

var TimeoutMessage = &Message{MsgType: 2, Err: errors.New("timed_out")}

type ActorInterface interface {
	String() string
}

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
		a.logger.Errorf("Actor.Read err: %s", m.Err)
		if !a.isServer {
			close(a.received)
			close(a.toWrite)
		}
		return nil, m.Err
	}

	return m, nil
}

func (a *Actor) ReadTimed(duration time.Duration, onTimeoutMessage *Message) (*Message, error) {

	readFinished := make(chan bool, 1)
	readMsgChan := make(chan *Message, 1)
	readErrChan := make(chan error, 1)

	go func() {
		startTime := time.Now()
		timer := time.NewTicker(time.Second * 1)
		for {
			<-timer.C
			select {
			case <-readFinished:
				return
			default:
				if time.Since(startTime).Seconds() > (duration).Seconds() {
					readMsgChan <- onTimeoutMessage
					readErrChan <- nil

					//requeue the message when the Read task does finally finish
					<-readFinished
					msg := <-readMsgChan
					a.logger.Debugf("Actor.ReadTimed recycling timed-out message %s", msg.Data)
					a.received <- msg
					return
				}
			}
		}
	}()

	go func() {
		m, err := a.Read()
		readFinished <- true
		readMsgChan <- m
		readErrChan <- err
	}()

	err := <-readErrChan
	if err != nil {
		return nil, err
	}

	msg := <-readMsgChan
	return msg, nil
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
	} else if !a.isServer && a.status == Connecting {
		a.logger.Infoln("Client is still connecting so lets use recursion")
		time.Sleep(time.Millisecond * 100)
		return a.Write(msgType, message)
	} else if a.status != Connected {
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

func (a *Actor) read(readBytesCb func(*Actor, []byte) bool) {
	bLen := make([]byte, 4)

	for {
		res := readBytesCb(a, bLen)
		if !res {
			break
		}

		mLen := bytesToInt(bLen)

		msgRecvd := make([]byte, mLen)

		res = readBytesCb(a, msgRecvd)
		if !res {
			break
		}

		msgType := bytesToInt(msgRecvd[:4])
		msgData := msgRecvd[4:]

		if msgType == 0 {
			//  type 0 = control message
			a.logger.Debugf("Server.read - control message encountered")
		} else {
			a.received <- &Message{Data: msgData, MsgType: msgType}
		}
	}
}

func (a *Server) write() {

	for {

		m, ok := <-a.toWrite

		if !ok {
			break
		}

		toSend := append(intToBytes(m.MsgType), m.Data...)
		writer := bufio.NewWriter(a.conn)
		//first send the message size
		_, err := writer.Write(intToBytes(len(toSend)))
		if err != nil {
			a.logger.Errorf("error writing message size: %s", err)
		}
		//last send the message
		_, err = writer.Write(toSend)
		if err != nil {
			a.logger.Errorf("error writing message: %s", err)
		}

		err = writer.Flush()
		if err != nil {
			a.logger.Errorf("error flushing data: %s", err)
			continue
		}

		time.Sleep(2 * time.Millisecond)

	}
}

func (a *Actor) dispatchStatus(status Status) {
	a.logger.Debugf("Actor.dispacthStatus(%s): %s", a.getRole(), a.Status())
	a.status = status
	a.received <- &Message{Status: a.Status(), MsgType: -1}
}

func (a *Actor) dispatchErrorStr(err string) {
	a.dispatchError(errors.New(err))
}

func (a *Actor) dispatchError(err error) {
	a.logger.Debugf("Actor.dispacthError(%s): %s", a.getRole(), err)
	a.received <- &Message{Err: err, MsgType: -1}
}

func (a *Actor) getRole() string {
	if a.isServer {
		return "Server"
	} else {
		return "Client"
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
