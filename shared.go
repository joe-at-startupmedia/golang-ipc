package ipc

import "errors"

func (status Status) String() string {
	return [...]string{
		"Not Connected",
		"Connecting",
		"Connected",
		"Listening",
		"Closing",
		"Reconnecting",
		"Timeout",
		"Closed",
		"Error",
		"Disconnected",
	}[status-1]
}

// checks the name passed into the start function to ensure it's ok/will work.
func checkIpcName(ipcName string) error {

	if len(ipcName) == 0 {
		return errors.New("ipcName cannot be an empty string")
	}

	return nil
}
