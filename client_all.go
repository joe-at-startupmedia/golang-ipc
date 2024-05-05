package ipc

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

// StartClient - start the ipc client.
// ipcName = is the name of the unix socket or named pipe that the client will try and connect to.
func StartClient(ipcName string, config *ClientConfig) (*Client, error) {

	err := checkIpcName(ipcName)
	if err != nil {
		return nil, err

	}

	cc := &Client{
		Name:     ipcName,
		status:   NotConnected,
		received: make(chan *Message),
		toWrite:  make(chan *Message),
	}

	if config == nil {

		cc.timeout = 0
		cc.retryTimer = time.Duration(20)

	} else {

		if config.Timeout < 0 {
			cc.timeout = 0
		} else {
			cc.timeout = config.Timeout
		}

		if config.RetryTimer < 1 {
			cc.retryTimer = time.Duration(1)
		} else {
			cc.retryTimer = time.Duration(config.RetryTimer)
		}

	}

	go startClient(cc)

	return cc, nil
}

func startClient(c *Client) {

	c.status = Connecting
	c.received <- &Message{Status: c.status.String(), MsgType: -1}

	err := c.dial()
	if err != nil {
		c.received <- &Message{Err: err, MsgType: -1}
		return
	}

	go c.read()
	go c.write()

	c.status = Connected
	c.received <- &Message{Status: c.status.String(), MsgType: -1}
}

// Client connect to the unix socket created by the server -  for unix and linux
func (c *Client) dial() error {

	base := "/tmp/"
	sock := ".sock"

	startTime := time.Now()

	for {

		if c.timeout != 0 {

			if time.Since(startTime).Seconds() > c.timeout {
				c.status = Closed
				return errors.New("timed out trying to connect")
			}
		}

		conn, err := net.Dial("unix", base+c.Name+sock)
		if err != nil {

			if strings.Contains(err.Error(), "connect: no such file or directory") {

			} else if strings.Contains(err.Error(), "connect: connection refused") {

			} else {
				c.received <- &Message{Err: err, MsgType: -1}
			}

		} else {

			c.conn = conn

			err = c.handshake()
			if err != nil {
				return err
			}

			return nil
		}

		time.Sleep(c.retryTimer * time.Second)

	}

}

func (c *Client) read() {
	bLen := make([]byte, 4)

	for {

		res := c.readData(bLen)
		if !res {
			break
		}

		mLen := bytesToInt(bLen)

		msgRecvd := make([]byte, mLen)

		res = c.readData(msgRecvd)
		if !res {
			break
		}

		if bytesToInt(msgRecvd[:4]) == 0 {
			//  type 0 = control message
		} else {
			c.received <- &Message{Data: msgRecvd[4:], MsgType: bytesToInt(msgRecvd[:4])}
		}
	}
}

func (c *Client) readData(buff []byte) bool {

	_, err := io.ReadFull(c.conn, buff)
	if err != nil {
		if strings.Contains(err.Error(), "EOF") { // the connection has been closed by the client.
			c.conn.Close()

			if c.status != Closing || c.status == Closed {
				go c.reconnect()
			}
			return false
		}

		if c.status == Closing {
			c.status = Closed
			c.received <- &Message{Status: c.status.String(), MsgType: -1}
			c.received <- &Message{Err: errors.New("client has closed the connection"), MsgType: -2}
			return false
		}

		// other read error
		return false

	}

	return true

}

func (c *Client) reconnect() {

	c.status = ReConnecting
	c.received <- &Message{Status: c.status.String(), MsgType: -1}

	err := c.dial() // connect to the pipe
	if err != nil {
		if err.Error() == "timed out trying to connect" {
			c.status = Timeout
			c.received <- &Message{Status: c.status.String(), MsgType: -1}
			c.received <- &Message{Err: errors.New("timed out trying to re-connect"), MsgType: -1}
		}

		return
	}

	c.status = Connected
	c.received <- &Message{Status: c.status.String(), MsgType: -1}

	go c.read()
}

// Read - blocking function that receices messages
// if MsgType is a negative number its an internal message
func (c *Client) Read() (*Message, error) {

	m, ok := (<-c.received)
	if !ok {
		return nil, errors.New("the received channel has been closed")
	}

	if m.Err != nil {
		close(c.received)
		close(c.toWrite)
		return nil, m.Err
	}

	return m, nil
}

// Write - writes a  message to the ipc connection.
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (c *Client) Write(msgType int, message []byte) error {

	if msgType == 0 {
		return errors.New("Message type 0 is reserved")
	}

	if c.status != Connected {
		return errors.New(c.status.String())
	}

	mlen := len(message)
	if mlen > c.maxMsgSize {
		return errors.New("Message exceeds maximum message length")
	}

	c.toWrite <- &Message{MsgType: msgType, Data: message}

	return nil
}

func (c *Client) write() {

	for {

		m, ok := <-c.toWrite

		if !ok {
			break
		}

		toSend := append(intToBytes(m.MsgType), m.Data...)
		writer := bufio.NewWriter(c.conn)
		writer.Write(intToBytes(len(toSend)))
		writer.Write(toSend)

		err := writer.Flush()
		if err != nil {
			log.Println("error flushing data", err)
			continue
		}

		time.Sleep(2 * time.Millisecond)
	}
}

// getStatus - get the current status of the connection
func (c *Client) getStatus() Status {

	return c.status
}

// StatusCode - returns the current connection status
func (c *Client) StatusCode() Status {
	return c.status
}

// Status - returns the current connection status as a string
func (c *Client) Status() string {

	return c.status.String()
}

// Close - closes the connection
func (c *Client) Close() {

	c.status = Closing

	if c.conn != nil {
		c.conn.Close()
	}
}
