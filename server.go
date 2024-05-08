package ipc

import (
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

// StartServer - starts the ipc server.
//
// ipcName - is the name of the unix socket or named pipe that will be created, the client needs to use the same name
func StartServer(config *ServerConfig) (*Server, error) {

	cms, err := NewServer(config.Name+"_manager", config)
	if err != nil {
		return nil, err
	}
	cms, err = cms.run(0)
	if err != nil {
		return nil, err
	}

	s, err := NewServer(config.Name, config)
	if err != nil {
		return nil, err
	}
	s.ServerManager = &ServerManager{
		Servers:      []*Server{cms, s},
		ServerConfig: config,
		Logger:       s.logger,
	}

	ClientCount := 0

	go func() {
		for {

			message, err := cms.Read()
			msgType := message.MsgType
			msgData := string(message.Data)

			if err == nil && msgType == CLIENT_CONNECT_MSGTYPE && msgData == "client_id_request" {
				cms.logger.Infof("recieved a request to create a new client server %d", ClientCount+1)
				ns, err := NewServer(config.Name, config)
				if err == nil {
					ClientCount++
					cms.Write(CLIENT_CONNECT_MSGTYPE, intToBytes(ClientCount))
					if ClientCount == 1 {
						//we already pre-provisioned the first client
						continue
					}
					go ns.run(ClientCount)
					s.ServerManager = &ServerManager{
						Servers:      append(s.ServerManager.Servers, ns),
						ServerConfig: config,
						Logger:       s.logger,
					}
				} else {
					cms.logger.Errorf("encountered an error attempting to create a client server %d %s", ClientCount+1, err)
				}
			}
		}
	}()

	return s.run(1)
}

func (sm *ServerManager) MapExec(callback func(*Server), from string) {
	serverLen := len(sm.Servers)
	serverOp := make(chan bool, serverLen)
	for i, server := range sm.Servers {
		//skip the first serverManager instance
		if i == 0 {
			continue
		}
		go func(s *Server) {
			callback(s)
			serverOp <- true
		}(server)
	}
	n := 0
	for n < serverLen-1 {
		<-serverOp
		n++
		sm.Logger.Debugf("sm.%sfinished for server(%d)", from, n)
	}
}

func (sm *ServerManager) Read(callback func(*Server, *Message, error)) {
	sm.MapExec(func(s *Server) {
		message, err := s.Read()
		callback(s, message, err)
	}, "Read")
}

func (sm *ServerManager) ReadTimed(duration time.Duration, timeoutMessage *Message, callback func(*Server, *Message, error)) {
	sm.MapExec(func(s *Server) {
		message, err := s.ReadTimed(duration, timeoutMessage)
		callback(s, message, err)
	}, "ReadTimed")
}

func NewServer(name string, config *ServerConfig) (*Server, error) {
	err := checkIpcName(name)
	if err != nil {
		return nil, err
	}

	s := &Server{Actor: NewActor(&ActorConfig{
		Name:         name,
		IsServer:     true,
		ServerConfig: config,
	})}

	if config == nil {
		s.maxMsgSize = MAX_MSG_SIZE
		s.unMask = false
	} else {

		if config.MaxMsgSize < 1024 {
			s.maxMsgSize = MAX_MSG_SIZE
		} else {
			s.maxMsgSize = config.MaxMsgSize
		}

		s.unMask = config.UnmaskPermissions
	}
	return s, err
}

func (s *Server) run(clientId int) (*Server, error) {

	var socketName string

	if clientId > 0 {
		socketName = fmt.Sprintf("%s%s%d%s", SOCKET_NAME_BASE, s.name, clientId, SOCKET_NAME_EXT)
	} else {
		socketName = fmt.Sprintf("%s%s%s", SOCKET_NAME_BASE, s.name, SOCKET_NAME_EXT)
	}

	if err := os.RemoveAll(socketName); err != nil {
		return s, err
	}

	var oldUmask int
	if s.unMask {
		oldUmask = syscall.Umask(0)
	}

	listener, err := net.Listen("unix", socketName)

	s.listener = listener

	if s.unMask {
		syscall.Umask(oldUmask)
	}

	if err != nil {
		s.logger.Errorf("Server.run err: %s", err)
		return s, err
	}

	go s.acceptLoop(clientId)
	s.status = Listening

	return s, nil
}

func (s *Server) acceptLoop(clientId int) {

	i := 0
	for {

		conn, err := s.listener.Accept()
		if err != nil {
			s.logger.Debugf("Server.acceptLoop -> listen.Accept err: %s", err)
			return
		}

		if s.status == Listening || s.status == Disconnected {

			s.conn = conn

			err2 := s.handshake(&conn, clientId)
			if err2 != nil {
				s.logger.Errorf("Server.acceptLoop handshake err: %s", err2)
				s.dispatchError(err2)
				s.status = Error
				s.listener.Close()
				conn.Close()

			} else {
				go s.read(s.ByteReader)
				go s.write()

				s.dispatchStatus(Connected)
			}
		}
		i++
	}
}

func (s *Server) ByteReader(a *Actor, buff []byte) bool {

	_, err := io.ReadFull(a.conn, buff)
	if err != nil {

		if a.status == Closing {

			a.dispatchStatus(Closed)
			a.dispatchErrorStr("server has closed the connection")
			return false
		}

		if err == io.EOF {
			a.dispatchStatus(Disconnected)
			return false
		}
	}

	return true
}

// Close - closes the connection
func (s *Server) Close() {

	s.Actor.Close()

	for _, srv := range s.ServerManager.Servers {
		if srv.listener != nil {
			srv.listener.Close()
		}
	}
}
