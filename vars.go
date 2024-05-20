package ipc

import "github.com/sirupsen/logrus"

const (
	VERSION                = 2       // ipc package VERSION
	MAX_MSG_SIZE           = 3145728 // 3Mb  - Maximum bytes allowed for each message
	DEFAULT_WAIT           = 10
	DEFAULT_LOG_LEVEL      = logrus.ErrorLevel //
	SOCKET_NAME_BASE       = "/tmp/"
	SOCKET_NAME_EXT        = ".sock"
	CLIENT_CONNECT_MSGTYPE = 12
	ENCRYPT_BY_DEFAULT     = true
	DEFAULT_NETWORK_TYPE   = "tcp"
	DEFAULT_NETWORK_HOST   = "127.0.0.1"
	DEFAULT_NETWORK_PORT   = 8100
)
