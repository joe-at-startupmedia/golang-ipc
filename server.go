package ipc

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
)

// StartServer - starts the ipc server.
//
// ipcName - is the name of the unix socket or named pipe that will be created, the client needs to use the same name
func StartServer(config *ServerConfig) (*Server, error) {

	if config.MultiClient {
		return StartMultiServer(config)
	} else {
		return StartOnlyServer(config)
	}
}

func NewServer(name string, config *ServerConfig) (*Server, error) {
	err := checkIpcName(name)
	if err != nil {
		return nil, err
	}
	config.Name = name
	s := &Server{Actor: NewActor(&ActorConfig{
		IsServer:     true,
		ServerConfig: config,
	})}

	if config == nil {
		serverConfig := &ServerConfig{
			MaxMsgSize: MAX_MSG_SIZE,
			Encryption: ENCRYPT_BY_DEFAULT,
		}
		s.config.ServerConfig = serverConfig
	} else {

		if config.MaxMsgSize < 1024 {
			s.config.ServerConfig.MaxMsgSize = MAX_MSG_SIZE
		}
	}
	return s, err
}

func StartOnlyServer(config *ServerConfig) (*Server, error) {

	s, err := NewServer(config.Name, config)
	if err != nil {
		return nil, err
	}
	s.ServerManager = &ServerManager{
		Servers:      []*Server{{}, s}, //we add an empty server in case we need to MapExec
		ServerConfig: config,
		Logger:       s.logger,
		mutex:        &sync.Mutex{},
	}

	return s.run(0)
}

func StartMultiServer(config *ServerConfig) (*Server, error) {

	//well be modifying the config.Name property by reference
	configName := config.Name

	cms, err := NewServer(configName+"_manager", config)
	if err != nil {
		return nil, err
	}
	cms, err = cms.run(0)
	if err != nil {
		return nil, err
	}

	s, err := NewServer(configName, config)
	if err != nil {
		return nil, err
	}
	s.ServerManager = &ServerManager{
		Servers:      []*Server{cms, s},
		ServerConfig: config,
		Logger:       s.logger,
		mutex:        &sync.Mutex{},
	}

	ClientCount := 0

	go func() {
		for {

			message, err := cms.Read()
			if err != nil {
				s.logger.Errorf("ServerManager.read err: %s", err)
				s.dispatchError(err)
				continue
			}
			msgType := message.MsgType
			msgData := string(message.Data)

			if err == nil && msgType == CLIENT_CONNECT_MSGTYPE && msgData == "client_id_request" {
				cms.logger.Infof("recieved a request to create a new client server %d", ClientCount+1)
				ns, err := NewServer(configName, config)
				if err == nil {
					ClientCount++
					cms.Write(CLIENT_CONNECT_MSGTYPE, intToBytes(ClientCount))
					if ClientCount == 1 {
						//we already pre-provisioned the first client
						continue
					}
					go ns.run(ClientCount)
					s.ServerManager.mutex.Lock()
					s.ServerManager.Servers = append(s.ServerManager.Servers, ns)
					s.ServerManager.mutex.Unlock()
				} else {
					cms.logger.Errorf("encountered an error attempting to create a client server %d %s", ClientCount+1, err)
				}
			}
		}
	}()

	return s.run(1)
}

func (s *Server) run(clientId int) (*Server, error) {

	var socketName string

	if clientId > 0 {
		socketName = fmt.Sprintf("%s%s%d%s", SOCKET_NAME_BASE, s.config.ServerConfig.Name, clientId, SOCKET_NAME_EXT)
	} else {
		socketName = fmt.Sprintf("%s%s%s", SOCKET_NAME_BASE, s.config.ServerConfig.Name, SOCKET_NAME_EXT)
	}

	if err := os.RemoveAll(socketName); err != nil {
		return s, err
	}

	var oldUmask int
	if s.config.ServerConfig.UnmaskPermissions {
		oldUmask = syscall.Umask(0)
	}

	listener, err := net.Listen("unix", socketName)

	s.listener = listener

	if s.config.ServerConfig.UnmaskPermissions {
		syscall.Umask(oldUmask)
	}

	if err != nil {
		s.logger.Errorf("Server.run err: %s", err)
		return s, err
	}

	go s.acceptLoop()
	s.setStatus(Listening)

	return s, nil
}

func (s *Server) acceptLoop() {

	for {

		conn, err := s.listener.Accept()
		if err != nil {
			s.logger.Debugf("Server.acceptLoop -> listen.Accept err: %s", err)
			return
		}

		status := s.getStatus()

		if status == Listening || status == Disconnected {

			s.conn = conn

			err2 := s.handshake()
			if err2 != nil {
				s.logger.Errorf("Server.acceptLoop handshake err: %s", err2)
				s.dispatchError(err2)
				s.setStatus(Error)
				s.listener.Close()
				conn.Close()

			} else {
				go s.read(s.ByteReader)
				go s.write()

				s.dispatchStatus(Connected)
			}
		}
	}
}

func (s *Server) ByteReader(a *Actor, buff []byte) bool {

	_, err := io.ReadFull(a.conn, buff)
	if err != nil {

		if a.getStatus() == Closing {
			a.dispatchStatusBlocking(Closed)
			a.dispatchErrorStrBlocking("server has closed the connection")
			return false
		}

		if err == io.EOF {
			a.dispatchStatus(Disconnected)
			return false
		}
	}

	return true
}

func (s *Server) close() {

	s.Actor.Close()

	if s.listener != nil {
		s.listener.Close()
	}
}

// Close - closes the connection
func (s *Server) Close() {

	if s.config.ServerConfig.MultiClient {
		s.ServerManager.Close()
	} else {
		s.close()
	}
}
