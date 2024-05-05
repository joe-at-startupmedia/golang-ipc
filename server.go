package ipc

import (
	"errors"
	"io"
	"net"
	"os"
	"syscall"
)

// StartServer - starts the ipc server.
//
// ipcName - is the name of the unix socket or named pipe that will be created, the client needs to use the same name
func StartServer(ipcName string, config *ServerConfig) (*Server, error) {

	err := checkIpcName(ipcName)
	if err != nil {
		return nil, err
	}

	s := &Server{Actor: NewActor(&ActorConfig{
		Name:         ipcName,
		IsServer:     true,
		ServerConfig: config,
	})}

	if config == nil {
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
		s.logger.Errorf("Server.run err: %s", err)
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
			s.logger.Errorf("Server.acceptLoop -> listen.Accept err: %s", err)
			break
		}

		if s.status == Listening || s.status == Disconnected {

			s.conn = conn

			err2 := s.handshake()
			if err2 != nil {
				s.logger.Errorf("Server.acceptLoop handshake err: %s", err2)
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

func (a *Server) read() {
	bLen := make([]byte, 4)

	for {
		res := a.readData(bLen)
		if !res {
			break
		}

		mLen := bytesToInt(bLen)

		msgRecvd := make([]byte, mLen)

		res = a.readData(msgRecvd)
		if !res {
			break
		}

		if bytesToInt(msgRecvd[:4]) == 0 {
			//  type 0 = control message
			a.logger.Debugf("Server.read - control message encountered")
		} else {
			a.received <- &Message{Data: msgRecvd[4:], MsgType: bytesToInt(msgRecvd[:4])}
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
