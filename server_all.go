package ipc

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

// StartServer - starts the ipc server.
//
// ipcName - is the name of the unix socket or named pipe that will be created, the client needs to use the same name
func StartServer(ipcName string, config *ServerConfig) (*Server, error) {

	err := checkIpcName(ipcName)
	if err != nil {
		return nil, err
	}

	s := &Server{
		name:     ipcName,
		status:   NotConnected,
		received: make(chan *Message),
		toWrite:  make(chan *Message),
	}

	if config == nil {
		s.timeout = 0
		s.maxMsgSize = maxMsgSize
		s.unMask = false

	} else {

		if config.MaxMsgSize < 1024 {
			s.maxMsgSize = maxMsgSize
		} else {
			s.maxMsgSize = config.MaxMsgSize
		}

		s.unMask = config.UnmaskPermissions
	}

	err = s.run()

	return s, err
}

func (s *Server) run() error {

	base := "/tmp/"
	sock := ".sock"

	if err := os.RemoveAll(base + s.name + sock); err != nil {
		return err
	}

	var oldUmask int
	if s.unMask {
		oldUmask = syscall.Umask(0)
	}

	listen, err := net.Listen("unix", base+s.name+sock)

	if s.unMask {
		syscall.Umask(oldUmask)
	}

	if err != nil {
		return err
	}

	s.listen = listen

	go s.acceptLoop()

	s.status = Listening

	return nil

}

func (s *Server) acceptLoop() {

	for {
		conn, err := s.listen.Accept()
		if err != nil {
			break
		}

		if s.status == Listening || s.status == Disconnected {

			s.conn = conn

			err2 := s.handshake()
			if err2 != nil {
				s.received <- &Message{Err: err2, MsgType: -1}
				s.status = Error
				s.listen.Close()
				s.conn.Close()

			} else {

				go s.read()
				go s.write()

				s.status = Connected
				s.received <- &Message{Status: s.status.String(), MsgType: -1}
			}

		}

	}

}

func (s *Server) read() {

	bLen := make([]byte, 4)

	for {

		res := s.readData(bLen)
		if !res {
			s.conn.Close()

			break
		}

		mLen := bytesToInt(bLen)

		msgRecvd := make([]byte, mLen)

		res = s.readData(msgRecvd)
		if !res {
			s.conn.Close()
			break
		}

		//this wierd edgecase happens when requests come too quicklu
		/*
			if cap(msgRecvd) == 0 {
				//log.Panicln("msgRecvd capacity is zero, requests coming too quickly?")
				s.status = Disconnected
				s.received <- &Message{Status: s.status.String(), MsgType: -1}
			} else

		*/
		if bytesToInt(msgRecvd[:4]) == 0 {
			//  type 0 = control message
		} else {
			s.received <- &Message{Data: msgRecvd[4:], MsgType: bytesToInt(msgRecvd[:4])}
		}

	}

}

func (s *Server) readData(buff []byte) bool {

	_, err := io.ReadFull(s.conn, buff)
	if err != nil {

		if s.status == Closing {

			s.status = Closed
			s.received <- &Message{Status: s.status.String(), MsgType: -1}
			s.received <- &Message{Err: errors.New("server has closed the connection"), MsgType: -1}
			return false
		}

		if err == io.EOF {

			s.status = Disconnected
			s.received <- &Message{Status: s.status.String(), MsgType: -1}
			return false
		}

	}

	return true
}

// Read - blocking function, reads each message recieved
// if MsgType is a negative number its an internal message
func (s *Server) Read() (*Message, error) {

	m, ok := <-s.received
	if !ok {
		return nil, errors.New("the received channel has been closed")
	}

	if m.Err != nil {
		//close(s.received)
		//close(s.toWrite)
		return nil, m.Err
	}

	return m, nil
}

// Write - writes a message to the ipc connection
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (s *Server) Write(msgType int, message []byte) error {

	if msgType == 0 {
		return errors.New("message type 0 is reserved")
	}

	mlen := len(message)

	if mlen > s.maxMsgSize {
		return errors.New("message exceeds maximum message length")
	}

	if s.status == Connected {

		s.toWrite <- &Message{MsgType: msgType, Data: message}

	} else {
		return errors.New(s.status.String())
	}

	return nil
}

func (s *Server) write() {

	for {

		m, ok := <-s.toWrite

		if !ok {
			break
		}

		toSend := append(intToBytes(m.MsgType), m.Data...)
		writer := bufio.NewWriter(s.conn)
		//first send the message size
		writer.Write(intToBytes(len(toSend)))
		//last send the message
		writer.Write(toSend)

		err := writer.Flush()
		if err != nil {
			log.Println("error flushing data", err)
			continue
		}

		time.Sleep(2 * time.Millisecond)

	}
}

// getStatus - get the current status of the connection
func (s *Server) getStatus() Status {
	return s.status
}

// StatusCode - returns the current connection status
func (s *Server) StatusCode() Status {
	return s.status
}

// Status - returns the current connection status as a string
func (s *Server) Status() string {

	return s.status.String()
}

// Close - closes the connection
func (s *Server) Close() {

	s.status = Closing

	if s.listen != nil {
		s.listen.Close()
	}

	if s.conn != nil {
		s.conn.Close()
	}
}
