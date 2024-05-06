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
		cc.retryTimer = 0

	} else {
		if config.Timeout < 0 {
			cc.timeout = 0
		} else {
			cc.timeout = config.Timeout
		}

		if config.RetryTimer < 0 {
			cc.retryTimer = 0
		} else {
			cc.retryTimer = config.RetryTimer
		}
	}

	return start(cc)
}

func start(c *Client) (*Client, error) {

	go c.dispatchStatus(Connecting)

	if c.timeout != 0 {

		dialFinished := make(chan bool, 1)
		dialErrorChan := make(chan error, 1)

		go func() {
			startTime := time.Now()
			timer := time.NewTicker(time.Millisecond * 1000)
			for {
				<-timer.C
				select {
				case <-dialFinished:
					return
				default:
					if time.Since(startTime).Seconds() > 2 {
						c.logger.Debugf("Start loop since: %f", time.Since(startTime).Seconds())
					}
					if time.Since(startTime).Seconds() > c.timeout.Seconds() {
						dialErrorChan <- errors.New("timed out trying to connect")
						return
					}
				}
			}
		}()

		go func() {
			err := c.dial()
			dialFinished <- true
			dialErrorChan <- err
			if err != nil {
				c.dispatchError(err)
			}
		}()

		err := <-dialErrorChan

		//TODO if Retry is allowed
		if err != nil {
			return start(c)
		}
	} else {
		err := c.dial()
		if err != nil {
			c.dispatchError(err)
			return c, err
		}
	}

	go c.read()
	go c.write()

	go c.dispatchStatus(Connected)

	return c, nil
}

// Client connect to the unix socket created by the server -  for unix and linux
func (c *Client) dial() error {

	base := "/tmp/"
	sock := ".sock"

	startTime := time.Now()

	for {

		if c.timeout != 0 {
			if time.Since(startTime).Seconds() > 2 {
				c.logger.Debugf("Seconds since: %f, timeout seconds: %f", time.Since(startTime).Seconds(), c.timeout.Seconds())
			}
			if time.Since(startTime).Seconds() > c.timeout.Seconds() {
				c.status = Closed
				return errors.New("timed out trying to connect")
			}
		}

		conn, err := net.Dial("unix", base+c.name+sock)
		if err != nil {
			c.logger.Debugf("Client.dial err: %s", err)
			//connect: no such file or directory happens a lot when the client connection closes under normal circumstances
			if !strings.Contains(err.Error(), "connect: no such file or directory") &&
				!strings.Contains(err.Error(), "connect: connection refused") {
				c.dispatchError(err)
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

		time.Sleep(c.retryTimer)
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
		c.logger.Debugf("Client.readData err: %s", err)
		if c.status == Closing {
			c.dispatchStatus(Closed)
			c.dispatchErrorStr("client has closed the connection")
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
	c.dispatchStatus(ReConnecting)

	err := c.dial() // connect to the pipe
	if err != nil {
		c.logger.Errorf("Client.reconnect -> dial err: %s", err)
		if err.Error() == "timed out trying to connect" {
			c.dispatchStatus(Timeout)
			c.dispatchErrorStr("timed out trying to re-connect")
		}

		return
	}

	c.dispatchStatus(Connected)

	go c.read()
}
