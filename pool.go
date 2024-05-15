package ipc

import (
	"sync"
	"time"
)

var clientIdRequest = &Message{
	Data:    []byte("client_id_request"),
	MsgType: CLIENT_CONNECT_MSGTYPE,
}

func isClientIdRequest(msg *Message) bool {
	return msg.MsgType == CLIENT_CONNECT_MSGTYPE && string(msg.Data) == string(clientIdRequest.Data)
}

func StartServerPool(config *ServerConfig) (*Server, error) {

	//copy to prevent modification of the reference
	configName := config.Name

	//creates a server exclusively for listening for new connections
	cms, err := NewServer(configName+"_manager", config)
	if err != nil {
		return nil, err
	}
	cms, err = cms.run(0)
	if err != nil {
		return nil, err
	}

	//create another server for the user to interface
	s, err := NewServer(configName, config)
	if err != nil {
		return nil, err
	}
	s.Connections = &ConnectionPool{
		Servers:      []*Server{cms, s},
		ServerConfig: config,
		Logger:       s.logger,
		mutex:        &sync.Mutex{},
	}

	go connectionListener(cms, s)

	return s.run(1)
}

func connectionListener(cms *Server, s *Server) {
	clientCount := 1

	for {

		msg, err := cms.Read()
		if err != nil {
			s.logger.Errorf("ConnectionPool.read err: %s", err)
			s.dispatchError(err)
			continue
		}

		if isClientIdRequest(msg) {
			err = cms.Write(CLIENT_CONNECT_MSGTYPE, intToBytes(clientCount))
			if err != nil {
				continue
			}
			if clientCount == 1 {
				//we already pre-provisioned the first client
				clientCount++
				continue
			}
			cms.logger.Infof("received a request to create a new client server %d", clientCount)
			//ns, err2 := NewServer(fmt.Sprintf("%s%d", s.config.ServerConfig.Name, clientCount), s.config.ServerConfig)
			ns, err2 := NewServer(s.config.ServerConfig.Name, s.config.ServerConfig)
			if err2 != nil {
				cms.logger.Errorf("encountered an error attempting to create a client server %d %s", clientCount, err2)
				continue
			}

			go ns.run(clientCount)
			clientCount++
			s.Connections.mutex.Lock()
			s.Connections.Servers = append(s.Connections.Servers, ns)
			s.Connections.mutex.Unlock()
		}
	}
}

func StartClientPool(config *ClientConfig) (*Client, error) {

	//copy to prevent modification of the reference
	configName := config.Name

	cm, err := NewClient(configName+"_manager", config)
	if err != nil {
		return nil, err
	}
	defer cm.Close()

	cm, err = start(cm)
	if err != nil {
		return nil, err
	}

	err = cm.WriteMessage(clientIdRequest)
	if err != nil {
		return nil, err
	}

	for {
		message, err2 := cm.ReadTimed(5 * time.Second)

		if message == TimeoutMessage {
			continue
		} else if err2 != nil {
			cm.logger.Debugf("StartClientPool err: %s", err)
			continue
		} else if message.MsgType != CLIENT_CONNECT_MSGTYPE {
			continue
		}

		clientId := bytesToInt(message.Data)

		if clientId > 0 {

			cm.logger.Infof("Attempting to create a new Client %d", clientId)

			cc, err3 := NewClient(configName, config)
			if err3 != nil {
				return nil, err3
			}
			cc.ClientId = clientId
			return start(cc)
		}
	}
}

func (sm *ConnectionPool) getServers() []*Server {
	sm.mutex.Lock()
	servers := sm.Servers
	sm.mutex.Unlock()
	return servers
}

func (sm *ConnectionPool) MapExec(callback func(*Server), from string) {
	servers := sm.getServers()
	serverLen := len(servers)
	serverOp := make(chan bool, serverLen)
	for i, server := range servers {
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

func (sm *ConnectionPool) Read(callback func(*Server, *Message, error)) {
	sm.MapExec(func(s *Server) {
		message, err := s.Read()
		callback(s, message, err)
	}, "Read")
}

// ReadTimed will call ReadTimed on all connections waiting for the slowest one to finish
func (sm *ConnectionPool) ReadTimed(duration time.Duration, callback func(*Server, *Message, error)) {
	sm.MapExec(func(s *Server) {
		message, err := s.ReadTimed(duration)
		callback(s, message, err)
	}, "ReadTimed")
}

// ReadTimed will call ReadTimed on all connections waiting for the fastest one to finish
func (sm *ConnectionPool) ReadTimedFastest(duration time.Duration, callback func(*Server, *Message, error)) {
	wg := make(chan bool, len(sm.getServers()))
	go func() {
		sm.MapExec(func(s *Server) {
			message, err := s.ReadTimed(duration)
			callback(s, message, err)
			wg <- true
		}, "ReadTimedFastest")
	}()
	<-wg
}

func (sm *ConnectionPool) Close() {
	servers := sm.getServers()
	serverLen := len(servers)
	serverOp := make(chan bool, serverLen)
	var primary *Server
	for i, server := range servers {
		//the second server is the one we want to close last
		if i == 1 {
			primary = server
			continue
		}
		go func(s *Server) {
			s.close()
			serverOp <- true
		}(server)
	}
	n := 0
	for n < serverLen-1 {
		<-serverOp
		n++
		sm.Logger.Debugf("sm.Close finished for server(%d)", n)
	}
	primary.close()
}
