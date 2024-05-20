//go:build !windows && !network

package ipc

import (
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
)

func getSocketName(clientId int, name string) string {
	if clientId > 0 {
		return fmt.Sprintf("%s%s%d%s", SOCKET_NAME_BASE, name, clientId, SOCKET_NAME_EXT)
	} else {
		return fmt.Sprintf("%s%s%s", SOCKET_NAME_BASE, name, SOCKET_NAME_EXT)
	}
}

func (c *Client) connect() (net.Conn, error) {

	conn, err := net.Dial("unix", getSocketName(c.ClientId, c.config.ClientConfig.Name))
	//connect: no such file or directory happens a lot when the client connection closes under normal circumstances
	if err != nil && !strings.Contains(err.Error(), "connect: no such file or directory") &&
		!strings.Contains(err.Error(), "connect: connection refused") {
		c.dispatchError(err)
	}

	return conn, err
}

func (s *Server) listen(clientId int) error {

	socketName := getSocketName(clientId, s.config.ServerConfig.Name)

	if err := os.RemoveAll(socketName); err != nil {
		return err
	}

	var oldUmask int
	if s.config.ServerConfig.UnmaskPermissions {
		oldUmask = syscall.Umask(0)
	}

	listener, err := net.Listen("unix", socketName)
	if err != nil {
		return err
	}

	s.listener = listener

	if s.config.ServerConfig.UnmaskPermissions {
		syscall.Umask(oldUmask)
	}

	return nil
}
