package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// StartClient - start the ipc client.
// ipcName = is the name of the unix socket or named pipe that the client will try and connect to.
func StartClient(config *ClientConfig) (*Client, error) {

	cm, err := NewClient(config.Name+"_manager", config)
	if err != nil {
		return nil, err
	}

	cm, err = start(cm)

	if err != nil {
		return nil, err
	}

	err = cm.Write(CLIENT_CONNECT_MSGTYPE, []byte("client_id_request"))

	if err != nil {
		return nil, err
	}

	for {
		message, err := cm.ReadTimed(5*time.Second, TimeoutMessage)

		msgType := message.MsgType
		msgData := bytesToInt(message.Data)

		if err == nil && msgType == CLIENT_CONNECT_MSGTYPE && msgData > 0 {

			cm.logger.Infof("Attempting to create a new Client %d, %s", msgData, message.Data)

			cc, err := NewClient(config.Name, config)
			if err != nil {
				return nil, err
			}
			cc.ClientId = msgData
			cm.Close()
			return start(cc)
		} else {
			cm.logger.Debugf("err: %s, msgType: %d, msgData: %d", err, msgType, msgData)
		}
	}
}

func NewClient(name string, config *ClientConfig) (*Client, error) {
	err := checkIpcName(name)
	if err != nil {
		return nil, err

	}
	actor := NewActor(&ActorConfig{
		Name:         name,
		ClientConfig: config,
	})

	cc := &Client{
		Actor: actor,
	}

	actor.ClientRef = cc

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

	return cc, err
}

func (c *Client) getSocketName() string {
	if c.ClientId > 0 {
		return fmt.Sprintf("%s%s%d%s", SOCKET_NAME_BASE, c.name, c.ClientId, SOCKET_NAME_EXT)
	} else {
		return fmt.Sprintf("%s%s%s", SOCKET_NAME_BASE, c.name, SOCKET_NAME_EXT)
	}
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

	go c.read(c.ByteReader)
	go c.write()
	go c.dispatchStatus(Connected)

	return c, nil
}

func (a *Client) write() {

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

// Client connect to the unix socket created by the server -  for unix and linux
func (c *Client) dial() error {

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
		conn, err := net.Dial("unix", c.getSocketName())
		if err != nil {
			c.logger.Debugf("Client.dial err: %s", err)
			//connect: no such file or directory happens a lot when the client connection closes under normal circumstances
			if !strings.Contains(err.Error(), "connect: no such file or directory") &&
				!strings.Contains(err.Error(), "connect: connection refused") {
				c.dispatchError(err)

			} else {
				time.Sleep(time.Second * 1)
			}

		} else {

			c.conn = conn

			err = c.handshake(&conn)
			if err != nil {
				c.logger.Errorf("Client.dial handshake err: %s", err)
				return err
			}

			return nil
		}

		time.Sleep(c.retryTimer)
	}
}

func (c *Client) ByteReader(a *Actor, buff []byte) bool {

	_, err := io.ReadFull(a.conn, buff)
	if err != nil {
		a.logger.Debugf("Client.readData err: %s", err)
		if c.status == Closing {
			a.dispatchStatus(Closed)
			a.dispatchErrorStr("client has closed the connection")
			return false
		}

		if err == io.EOF { // the connection has been closed by the client.
			a.conn.Close()

			if a.status != Closing {
				go a.ClientRef.reconnect(c)
			}
			return false
		}

		// other read error
		return false
	}

	return true
}

func (b *Client) reconnect(c *Client) {

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

	go c.read(c.ByteReader)
}

// Close - closes the connection
func (a *Client) Close() {

	a.status = Closing

	if a.conn != nil {
		a.conn.Close()
	}
}

// getStatus - get the current status of the connection
func (c *Client) String() string {
	return fmt.Sprintf("Client(%d)", c.ClientId)
}

func (a *Client) dispatchStatus(status Status) {
	a.logger.Debugf("Actor.dispacthStatus(%s): %s", a.String(), a.Status())
	a.status = status
	a.received <- &Message{Status: a.Status(), MsgType: -1}
}

func (a *Client) dispatchError(err error) {
	a.logger.Debugf("Actor.dispacthError(%s): %s", a.String(), err)
	a.received <- &Message{Err: err, MsgType: -1}
}
