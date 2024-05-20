package ipc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// StartClient - start the ipc client.
// ipcName = is the name of the unix socket or named pipe that the client will try and connect to.
func StartClient(config *ClientConfig) (*Client, error) {
	if config.MultiClient {
		return StartClientPool(config)
	} else {
		cc, err := NewClient(config.Name, config)
		if err != nil {
			return nil, err
		}
		cc.ClientId = 0
		return start(cc)
	}
}

func NewClient(name string, config *ClientConfig) (*Client, error) {
	err := checkIpcName(name)
	if err != nil {
		return nil, err

	}

	if config == nil {
		config = &ClientConfig{
			Name:       name,
			Encryption: ENCRYPT_BY_DEFAULT,
		}
	}

	cc := &Client{Actor: NewActor(&ActorConfig{
		ClientConfig: config,
	})}
	cc.clientRef = cc

	config.Name = name

	if config.Timeout < 0 {
		cc.timeout = 0
	} else {
		cc.timeout = config.Timeout
	}

	if config.RetryTimer <= 0 {
		cc.retryTimer = 1 * time.Second
	} else {
		cc.retryTimer = config.RetryTimer
	}

	return cc, err
}

func start(c *Client) (*Client, error) {
	c.dispatchStatus(Connecting)

	err := c.dial()
	if err != nil {
		c.dispatchError(err)
		return c, err
	}

	go c.read(c.ByteReader)
	go c.write()
	c.dispatchStatus(Connected)

	return c, nil
}

// Client connect to the unix socket created by the server -  for unix and linux
func (c *Client) dial() error {

	errChan := make(chan error, 1)

	go func() {
		startTime := time.Now()
		for {
			if c.timeout != 0 {
				if time.Since(startTime) > c.timeout {
					return
				}
			}
			conn, err := c.connect()
			if err != nil {
				c.logger.Debugf("Client.dial err: %s", err)
				//connect: no such file or directory happens a lot when the client connection closes under normal circumstances
				if !strings.Contains(err.Error(), "connect: no such file or directory") &&
					!strings.Contains(err.Error(), "connect: connection refused") {
					c.dispatchError(err)
				}
			} else {
				c.setConn(conn)
				err = c.handshake()
				if err != nil {
					c.logger.Errorf("%s.dial handshake err: %s", c, err)
				}

				errChan <- err
				return
			}

			time.Sleep(c.retryTimer)
		}
	}()

	if c.timeout != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
		select {
		case <-ctx.Done():
			return errors.New("timed out trying to connect")
		case err := <-errChan:
			return err
		}
	} else {
		return <-errChan
	}
}

func (c *Client) ByteReader(a *Actor, buff []byte) bool {

	_, err := io.ReadFull(a.getConn(), buff)
	if err != nil {
		a.logger.Debugf("%s.readData err: %s", c, err)
		if c.getStatus() == Closing {
			a.dispatchStatusBlocking(Closed)
			a.dispatchErrorStrBlocking("client has closed the connection")
			return false
		}

		if err == io.EOF { // the connection has been closed by the client.
			a.getConn().Close()

			if a.getStatus() != Closing {
				go reconnect(c)
			}
			return false
		}

		// other read error
		return false
	}

	return true
}

func reconnect(c *Client) {

	c.logger.Warn("Client.reconnect called")
	c.dispatchStatus(ReConnecting)

	// IMPORTANT removing this line will allow a dial before the new connection
	// is ready resulting in a dial hang when a timeout is not specified
	time.Sleep(c.retryTimer)
	err := c.dial()
	if err != nil {
		c.logger.Errorf("Client.reconnect -> dial err: %s", err)
		if err.Error() == "timed out trying to connect" {
			c.dispatchStatusBlocking(Timeout)
			c.dispatchErrorStrBlocking("timed out trying to re-connect")
		}

		return
	}

	c.dispatchStatus(Connected)

	go c.read(c.ByteReader)
}

// getStatus - get the current status of the connection
func (c *Client) String() string {
	return fmt.Sprintf("Client(%d)(%s)", c.ClientId, c.getStatus())
}
