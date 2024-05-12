package ipc

import "time"

func (sm *ServerManager) getServers() []*Server {
	sm.mutex.Lock()
	servers := sm.Servers
	sm.mutex.Unlock()
	return servers
}

func (sm *ServerManager) MapExec(callback func(*Server), from string) {
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

func (sm *ServerManager) Close() {
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
