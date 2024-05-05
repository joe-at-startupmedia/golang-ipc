package ipc

import (
	"errors"
	"io"
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

	cc := &Client{Actor: NewActor(&ActorConfig{
		Name:         ipcName,
		ClientConfig: config,
	})}

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
			cc.retryTimer = config.RetryTimer
		}

	}

	go start(cc)

	return cc, nil
}

func start(c *Client) {

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
			c.logger.Debugf("Seconds since: %f, timeout seconds: %f", time.Since(startTime).Seconds(), c.timeout.Seconds())
			if time.Since(startTime).Seconds() > c.timeout.Seconds() {
				c.status = Closed
				return errors.New("timed out trying to connect")
			}
		}

		conn, err := net.Dial("unix", base+c.name+sock)
		if err != nil {
			//connect: no such file or directory happens a lot when the client connection closes under normal circumstances
			if strings.Contains(err.Error(), "connect: no such file or directory") ||
				strings.Contains(err.Error(), "connect: connection refused") {
				c.logger.Debugf("Client.dial err: %s", err)
			} else {
				c.logger.Errorf("Client.dial err: %s", err)
				c.received <- &Message{Err: err, MsgType: -1}
			}

		} else {

			c.conn = conn

			err = c.handshake()
			if err != nil {
				c.logger.Errorf("Client.dial handshake err: %s", err)
				return err
			}

			return nil
		}

		time.Sleep(c.retryTimer * time.Second)

	}

}

func (a *Client) read() {
	bLen := make([]byte, 4)

	for {
		res := a.readData(bLen)
		if !res {
			break
		}

		mLen := bytesToInt(bLen)

		msgRecvd := make([]byte, mLen)

		res = a.readData(msgRecvd)
		if !res {
			break
		}

		if bytesToInt(msgRecvd[:4]) == 0 {
			//  type 0 = control message
			a.logger.Debugf("Client.read - control message encountered")
		} else {
			a.received <- &Message{Data: msgRecvd[4:], MsgType: bytesToInt(msgRecvd[:4])}
		}
	}
}

func (c *Client) readData(buff []byte) bool {

	_, err := io.ReadFull(c.conn, buff)
	if err != nil {
		c.logger.Errorf("Client.readData err: %s", err)
		if c.status == Closing {
			c.status = Closed
			c.received <- &Message{Status: c.status.String(), MsgType: -1}
			c.received <- &Message{Err: errors.New("client has closed the connection"), MsgType: -1}
			return false
		}

		if err == io.EOF { // the connection has been closed by the client.
			c.conn.Close()

			if c.status != Closing {
				go c.reconnect()
			}
			return false
		}

		// other read error
		return false

	}

	return true

}

func (c *Client) reconnect() {

	c.logger.Warn("Client.reconnect called")
	c.status = ReConnecting
	c.received <- &Message{Status: c.status.String(), MsgType: -1}

	err := c.dial() // connect to the pipe
	if err != nil {
		c.logger.Errorf("Client.reconnect -> dial err: %s", err)
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
