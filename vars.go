package ipc

import "github.com/sirupsen/logrus"

const (
	VERSION                     = 2       // ipc package VERSION
	MAX_MSG_SIZE                = 3145728 // 3Mb  - Maximum bytes allowed for each message
	DEFAULT_CLIENT_CONNECT_WAIT = 2
	DEFAULT_LOG_LEVEL           = logrus.ErrorLevel //
)
