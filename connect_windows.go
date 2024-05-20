//go:build windows && !network

package ipc

import (
	"fmt"
	"github.com/Microsoft/go-winio"
	"net"
	"strings"
)

func getSocketName(clientId int, name string) string {
	if clientId > 0 {
		return fmt.Sprintf("%s%s%d", `\\.\pipe\`, name, clientId)
	} else {
		return fmt.Sprintf("%s%s", `\\.\pipe\`, name)
	}
}

func (c *Client) connect() (net.Conn, error) {

	conn, err := winio.DialPipe(getSocketName(c.ClientId, c.config.ClientConfig.Name), nil)

	if err != nil && !strings.Contains(err.Error(), "the system cannot find the file specified.") {
		c.dispatchError(err)
	}

	return conn, err
}

func (s *Server) listen(clientId int) error {

	socketName := getSocketName(clientId, s.config.ServerConfig.Name)

	var config *winio.PipeConfig
	if s.config.ServerConfig.UnmaskPermissions {
		config = &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;AU)"}
	}

	listener, err := winio.ListenPipe(socketName, config)
	if err != nil {
		return err
	}

	s.listener = listener

	return nil
}
