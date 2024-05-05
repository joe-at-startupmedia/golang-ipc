package ipc

const version = 2 // ipc package version

const maxMsgSize = 3145728 // 3Mb  - Maximum bytes allowed for each message

// @TODO in race conditions where the client connects after the server start the connection will hang
const defaultClientConnectWait = 2 //the amount of time to wait in seconds after starting a server before the client connects
