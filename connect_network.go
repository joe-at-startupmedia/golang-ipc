//go:build network

package ipc

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

func GetDefaultPort() int {
	envVar := os.Getenv("IPC_NETWORK_PORT")
	if len(envVar) > 0 {
		valInt, err := strconv.Atoi(envVar)
		if err == nil {
			return valInt
		}
	}
	return DEFAULT_NETWORK_PORT
}

func GetDefaultHost() string {
	envVar := os.Getenv("IPC_NETWORK_HOST")
	if len(envVar) > 0 {
		return envVar
	}
	return DEFAULT_NETWORK_HOST
}

func GetDefaultNetworkType() string {
	envVar := os.Getenv("IPC_NETWORK_TYPE")
	if envVar == "udp" {
		return envVar
	}
	return DEFAULT_NETWORK_TYPE
}

func getHostAddr(clientId int) string {
	return fmt.Sprintf("%s:%d", GetDefaultHost(), GetDefaultPort()+clientId)
}

func (c *Client) connect() (net.Conn, error) {

	conn, err := net.Dial(GetDefaultNetworkType(), getHostAddr(c.ClientId))
	if err != nil {
		c.dispatchError(err)
	}

	return conn, err
}

func (s *Server) listen(clientId int) error {

	listener, err := net.Listen(GetDefaultNetworkType(), getHostAddr(clientId))
	if err != nil {
		return err
	}

	s.listener = listener

	return nil
}
