package ipc

import (
	"errors"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"strconv"
)

// checks the name passed into the start function to ensure it's ok/will work.
func checkIpcName(ipcName string) error {

	if len(ipcName) == 0 {
		return errors.New("ipcName cannot be an empty string")
	}

	return nil
}

func getLogrusLevel(logLevel string) logrus.Level {
	if os.Getenv("IPC_DEBUG") == "true" {
		return logrus.DebugLevel
	} else {
		switch logLevel {
		case "debug":
			return logrus.DebugLevel
		case "info":
			return logrus.InfoLevel
		case "warn":
			return logrus.WarnLevel
		case "error":
			return logrus.ErrorLevel
		}
		log.Println("log_level configuration is empty or invalid. Possible values include: debug, info, warn and error. using default level: info")
		return logrus.InfoLevel
	}
}

func GetDefaultClientConnectWait() int {
	envVar := os.Getenv("CLIENT_WAIT_TIMEOUT")
	if len(envVar) > 0 {
		valInt, err := strconv.Atoi(envVar)
		if err == nil {
			return valInt
		}
	}
	return defaultClientConnectWait
}
