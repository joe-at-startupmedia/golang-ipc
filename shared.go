package ipc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// checks the name passed into the start function to ensure it's ok/will work.
func checkIpcName(ipcName string) error {

	if len(ipcName) == 0 {
		return errors.New("ipcName cannot be an empty string")
	}

	return nil
}

func intToBytes(mLen int) []byte {

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(mLen))

	return b
}

func bytesToInt(b []byte) int {

	var mlen uint32

	binary.Read(bytes.NewReader(b[:]), binary.BigEndian, &mlen) // message length

	return int(mlen)
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
	}
	return DEFAULT_LOG_LEVEL
}

func GetDefaultClientConnectWait() int {
	envVar := os.Getenv("IPC_WAIT")
	if len(envVar) > 0 {
		valInt, err := strconv.Atoi(envVar)
		if err == nil {
			return valInt
		}
	}
	return DEFAULT_WAIT
}

// Sleep change the sleep time by using IPC_WAIT env variable (seconds)
func Sleep() {
	wait := GetDefaultClientConnectWait()
	if wait > 5 {
		time.Sleep(time.Duration(wait) * time.Millisecond)
	} else {
		time.Sleep(time.Duration(wait) * time.Second)
	}
}
