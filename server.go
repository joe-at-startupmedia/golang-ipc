package ipc

import (
	"io"
)

// StartServer - starts the ipc server.
func StartServer(config *ServerConfig) (*Server, error) {

	if config.MultiClient {
		return StartServerPool(config)
	} else {
		s, err := NewServer(config.Name, config)
		if err != nil {
			return nil, err
		}
		return s.run(0)
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

func (s *Server) run(clientId int) (*Server, error) {

	err := s.listen(clientId)
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

			s.setConn(conn)

			s.logger.Debug("acceptLoop 1")
			err2 := s.handshake()
			s.logger.Debug("acceptLoop 2")
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
		s.Connections.Close()
	} else {
		s.close()
	}
}
