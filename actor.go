package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"os"
	"sync"
	"time"
)

var TimeoutMessage = &Message{MsgType: 2, Err: errors.New("timed_out")}

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
		status:   NotConnected,
		received: make(chan *Message),
		toWrite:  make(chan *Message),
		logger:   logger,
		config:   ac,
		mutex:    &sync.Mutex{},
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
		a.logger.Errorf("%s.Read err: %s", a, m.Err)
		if !a.config.IsServer {
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
		select {
		case <-time.After(duration):
			readMsgChan <- onTimeoutMessage
			readErrChan <- nil

			//requeue the message when the Read task does finally finish
			<-readFinished
			msg := <-readMsgChan
			if msg != nil && a.getStatus() <= Connected {
				a.logger.Debugf("%s.ReadTimed recycling timed-out message %s", a, msg.Data)
				a.received <- msg
			}
			return
		case <-readFinished:
			return
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
		a.logger.Errorf("%s.Write err: %s", a, err)
		return err
	}

	status := a.getStatus()

	if a.config.IsServer && status == Listening {
		time.Sleep(time.Millisecond * 2)
		a.logger.Infoln("Server is still listening so lets use recursion")
		//it's possible the client hasn't connected yet so retry it
		return a.Write(msgType, message)
	} else if !a.config.IsServer && status == Connecting {
		a.logger.Infoln("Client is still connecting so lets use recursion")
		time.Sleep(time.Millisecond * 100)
		return a.Write(msgType, message)
	} else if status != Connected {
		err := errors.New(fmt.Sprintf("cannot write under current status: %s", a.Status()))
		a.logger.Errorf("%s.Write err: %s", a, err)
		return err
	}

	mlen := len(message)
	if a.config.IsServer {
		if mlen > a.config.ServerConfig.MaxMsgSize {
			err := errors.New("message exceeds maximum message length")
			a.logger.Errorf("%s.Write err: %s", a, err)
			return err
		}
	} else if mlen > a.clientRef.maxMsgSize {
		err := errors.New("message exceeds maximum message length")
		a.logger.Errorf("%s.Write err: %s", a, err)
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

		if a.shouldUseEncryption() {
			var err error
			msgRecvd, err = decrypt(*a.cipher, msgRecvd)
			if err != nil {
				a.dispatchError(err)
				continue
			}
		}

		msgType := bytesToInt(msgRecvd[:4])
		msgData := msgRecvd[4:]

		if msgType == 0 {
			//  type 0 = control message
			a.logger.Debugf("%s.read - control message encountered", a)
		} else {
			a.received <- &Message{Data: msgData, MsgType: msgType}
		}
	}
}

func (a *Actor) write() {

	for {

		m, ok := <-a.toWrite

		if !ok {
			break
		}

		toSend := append(intToBytes(m.MsgType), m.Data...)
		writer := bufio.NewWriter(a.getConn())

		if a.shouldUseEncryption() {
			var err error
			toSend, err = encrypt(*a.cipher, toSend)
			if err != nil {
				a.dispatchError(err)
				continue
			}
		}

		//first send the message size
		_, err := writer.Write(intToBytes(len(toSend)))
		if err != nil {
			a.logger.Errorf("%s error writing message size: %s", a, err)
		}
		//last send the message
		_, err = writer.Write(toSend)
		if err != nil {
			a.logger.Errorf("%s error writing message: %s", a, err)
		}

		if a.getStatus() <= 4 {
			err = writer.Flush()
			if err != nil {
				a.logger.Errorf("%s error flushing data: %s", a, err)
				continue
			}
		}
	}
}

func (a *Actor) dispatchStatusBlocking(status Status) {
	a.logger.Debugf("Actor.dispacthStatus(%s): %s", a, a.Status())
	a.setStatus(status)
	a.received <- &Message{Status: status.String(), MsgType: -1}
}

func (a *Actor) dispatchErrorStrBlocking(err string) {
	a.dispatchError(errors.New(err))
}

func (a *Actor) dispatchErrorBlocking(err error) {
	a.logger.Debugf("Actor.dispacthError(%s): %s", a, err)
	a.received <- &Message{Err: err, MsgType: -1}
}

func (a *Actor) dispatchStatus(status Status) {
	go a.dispatchStatusBlocking(status)
}

func (a *Actor) dispatchErrorStr(err string) {
	go a.dispatchErrorStrBlocking(err)
}

func (a *Actor) dispatchError(err error) {
	go a.dispatchErrorBlocking(err)
}

// getStatus - get the current status of the connection
func (a *Actor) getStatus() Status {
	a.mutex.Lock()
	status := a.status
	a.mutex.Unlock()
	return status
}

func (a *Actor) setStatus(status Status) {
	a.mutex.Lock()
	a.status = status
	a.mutex.Unlock()
}

func (a *Actor) getConn() net.Conn {
	a.mutex.Lock()
	conn := a.conn
	a.mutex.Unlock()
	return conn
}

func (a *Actor) setConn(conn net.Conn) {
	a.mutex.Lock()
	a.conn = conn
	a.mutex.Unlock()
}

// StatusCode - returns the current connection status
func (a *Actor) StatusCode() Status {
	return a.getStatus()
}

// Status - returns the current connection status as a string
func (a *Actor) Status() string {

	return a.getStatus().String()
}

// Close - closes the connection
func (a *Actor) Close() {

	//omits errors resulting from connections being closed
	a.logger.SetLevel(logrus.FatalLevel)

	a.setStatus(Closing)

	if a.conn != nil {
		a.getConn().Close()
	}
}

func (a *Actor) String() string {
	if a.config.IsServer {
		return fmt.Sprintf("Server(%s)", a.getStatus())
	} else {
		return a.clientRef.String()
	}
}
