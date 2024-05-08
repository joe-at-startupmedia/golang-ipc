package ipc

import (
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"
)

var RetriesEnabledClientConfig = &ClientConfig{
	Timeout:    time.Duration(time.Second * 10),
	RetryTimer: time.Duration(time.Second * 1),
}

func serverConfig(name string) *ServerConfig {
	return &ServerConfig{Name: name}
}

func clientConfig(name string) *ClientConfig {
	return &ClientConfig{Name: name}
}

func TestStartUp_Name(t *testing.T) {

	_, err := StartServer(serverConfig(""))
	if err.Error() != "ipcName cannot be an empty string" {
		t.Error("server - should have an error becuse the ipc name is empty")
	}

	/*
		This is no longer possible to test because the client must first connect
		to the manager which also requires a name
		_, err2 := StartClient(clientConfig(""))

		t.Error(err2)
		if err2.Error() != "ipcName cannot be an empty string" {
			t.Error("client - should have an error becuse the ipc name is empty")
		}
	*/
}

func TestStartUp_Configs(t *testing.T) {

	_, err := StartServer(serverConfig("test"))
	if err != nil {
		t.Error(err)
	}

	_, err2 := StartClient(clientConfig("test"))
	if err2 != nil {
		t.Error(err)
	}

	scon := serverConfig("test")

	ccon := clientConfig("test")

	_, err3 := StartServer(scon)
	if err3 != nil {
		t.Error(err2)
	}

	_, err4 := StartClient(ccon)
	if err4 != nil {
		t.Error(err)
	}

	scon.MaxMsgSize = -1

	_, err5 := StartServer(scon)
	if err5 != nil {
		t.Error(err2)
	}

	ccon.Timeout = -1
	ccon.RetryTimer = -1

	_, err6 := StartClient(ccon)
	if err6 != nil {
		t.Error(err)
	}

	scon.MaxMsgSize = 1025
	ccon.RetryTimer = 1

	_, err7 := StartServer(scon)
	if err7 != nil {
		t.Error(err2)
	}

	_, err8 := StartClient(ccon)
	if err8 != nil {
		t.Error(err)
	}

	t.Run("Unmask Server Socket Permissions", func(t *testing.T) {
		scon.Name = "test_perm"
		scon.UnmaskPermissions = true

		srv, err := StartServer(scon)
		if err != nil {
			t.Error(err)
		}

		// test would not work in windows
		// can check test_perm.sock in /tmp after running tests to see perms

		time.Sleep(time.Second / 4)

		info, err := os.Stat(srv.listener.Addr().String())
		if err != nil {
			t.Error(err)
		}
		got := fmt.Sprintf("%04o", info.Mode().Perm())
		want := "0777"

		if got != want {
			t.Errorf("Got %q, Wanted %q", got, want)
		}

		scon.UnmaskPermissions = false
	})
}

/*
func TestStartUp_Timeout(t *testing.T) {

	scon := &ServerConfig{
		//Timeout: 1,
	}

	sc, _ := StartServer("test_dummy", scon)

	for {
		_, err1 := sc.Read()

		if err1 != nil {
			if err1.Error() != "timed out waiting for client to connect" {
				t.Error("should of got server timeout")
			}
			break
		}

	}

	ccon := &ClientConfig{
		Timeout:    2,
		RetryTimer: 1,
	}

	cc, _ := StartClient("test2", ccon)

	for {
		_, err := cc.Read()
		if err != nil {
			if err.Error() != "timed out trying to connect" {
				t.Error("should of got timeout as client was trying to connect")
			}

			break

		}
	}
}
*/

func TestWrite(t *testing.T) {

	sc, err := StartServer(serverConfig("test10"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test10"))
	if err2 != nil {
		t.Error(err)
	}

	connected := make(chan bool, 1)

	go func() {

		for {

			m, err := cc.Read()
			if err != nil {
				//t.Error(fmt.Sprintf("Got read error: %s", err))
			} else if m.Status == "Connected" {
				connected <- true
			}
		}
	}()

	go func() {

		for {
			sc.Read()
		}

	}()

	<-connected

	buf := make([]byte, 1)

	err3 := sc.Write(0, buf)

	if err3.Error() != "message type 0 is reserved" {
		t.Error("0 is not allowed as a message type")
	}

	buf = make([]byte, sc.maxMsgSize+5)
	err4 := sc.Write(2, buf)

	if err4.Error() != "message exceeds maximum message length" {
		t.Errorf("There should be an error as the data we're attempting to write is bigger than the MAX_MSG_SIZE, instead we got: %s", err4)
	}

	sc.status = NotConnected

	buf2 := make([]byte, 5)
	err5 := sc.Write(2, buf2)
	if err5.Error() != "cannot write under current status: Not Connected" {
		t.Errorf("we should have an error becuse there is no connection but instead we got: %s", err5)
	}

	sc.status = Connected

	buf = make([]byte, 1)

	err = cc.Write(0, buf)
	if err == nil {
		t.Error("0 is not allowwed as a message try")
	}

	buf = make([]byte, MAX_MSG_SIZE+5)
	err = cc.Write(2, buf)
	if err == nil {
		t.Error("There should be an error is the data we're attempting to write is bigger than the MAX_MSG_SIZE")
	}

	cc.status = NotConnected

	buf = make([]byte, 5)
	err = cc.Write(2, buf)
	if err.Error() == "cannot write under current status: Not Connected" {

	} else {
		t.Error("we should have an error becuse there is no connection")
	}
}

func TestRead(t *testing.T) {

	sIPC := &Server{Actor: Actor{
		name:     "Test",
		status:   NotConnected,
		received: make(chan *Message),
	}}

	sIPC.status = Connected

	serverFinished := make(chan bool, 1)

	go func(s *Server) {

		_, err := sIPC.Read()
		if err != nil {
			t.Error("err should be nill as tbe read function should read the 1st message added to received")
		}
		_, err2 := sIPC.Read()
		if err2 != nil {
			t.Error("err should be nill as tbe read function should read the 1st message added to received")
		}

		_, err3 := sIPC.Read()
		if err3 == nil {
			t.Error("we should get an error as the messages have been read and the channel closed")

		} else {
			serverFinished <- true
		}

	}(sIPC)

	sIPC.received <- &Message{MsgType: 1, Data: []byte("message 1")}
	sIPC.received <- &Message{MsgType: 1, Data: []byte("message 2")}
	close(sIPC.received) // close channel

	<-serverFinished

	// Client - read tests

	// 3 x client side tests
	cIPC := &Client{
		Actor:      Actor{name: "test", status: NotConnected, received: make(chan *Message)},
		timeout:    2 * time.Second,
		retryTimer: 1,
	}

	cIPC.status = Connected

	clientFinished := make(chan bool, 1)

	go func() {

		_, err4 := cIPC.Read()
		if err4 != nil {
			t.Error("err should be nill as tbe read function should read the 1st message added to received")
		}
		_, err5 := cIPC.Read()
		if err5 != nil {
			t.Error("err should be nill as tbe read function should read the 1st message added to received")
		}

		_, err6 := cIPC.Read()
		if err6 == nil {
			t.Error("we should get an error as the messages have been read and the channel closed")
		} else {
			clientFinished <- true
		}

	}()

	cIPC.received <- &Message{MsgType: 1, Data: []byte("message 1")}
	cIPC.received <- &Message{MsgType: 1, Data: []byte("message 1")}
	close(cIPC.received) // close received channel

	<-clientFinished
}

func TestStatus(t *testing.T) {

	sc := &Server{Actor: Actor{
		status: NotConnected,
	}}

	s := sc.getStatus()

	if s.String() != "Not Connected" {
		t.Error("status string should have returned Not Connected")
	}

	sc.status = Listening

	s1 := sc.getStatus()

	if s1.String() != "Listening" {
		t.Error("status string should have returned Listening")
	}

	sc.status = Connecting

	s1 = sc.getStatus()

	if s1.String() != "Connecting" {
		t.Error("status string should have returned Connecting")
	}

	sc.status = Connected

	s2 := sc.getStatus()

	if s2.String() != "Connected" {
		t.Error("status string should have returned Connected")
	}

	sc.status = ReConnecting

	s3 := sc.getStatus()

	if s3.String() != "Reconnecting" {
		t.Error("status string should have returned Reconnecting")
	}

	sc.status = Closed

	s4 := sc.getStatus()

	if s4.String() != "Closed" {
		t.Error("status string should have returned Closed")
	}

	sc.status = Error

	s5 := sc.getStatus()

	if s5.String() != "Error" {
		t.Error("status string should have returned Error")
	}

	sc.status = Closing

	s6 := sc.getStatus()

	if s6.String() != "Closing" {
		t.Error("status string should have returned Error")
	}

	if s6.String() != "Closing" {
		t.Error("status string should have returned Error")
	}

	cc := &Client{Actor: Actor{
		status: NotConnected,
	}}

	cc.getStatus()
	cc.Status()

	cc2 := &Client{Actor: Actor{
		status: 9,
	}}

	cc2.getStatus()
	cc2.Status()
}

func TestGetConnected(t *testing.T) {

	sc, err := StartServer(serverConfig("test22"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test22"))
	if err2 != nil {
		t.Error(err)
	}

	for {
		cc.Read()
		m, _ := sc.Read()

		if m.Status == "Connected" {
			break
		}
	}
}

func TestServerWrongMessageType(t *testing.T) {

	sc, err := StartServer(serverConfig("test333"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test333"))
	if err2 != nil {
		t.Error(err)
	}

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {

		ready := false

		for {
			m, _ := sc.Read()
			if m.Status == "Connected" {
				connected <- true
				ready = true
				continue
			}

			if ready == true {
				if m.MsgType != 5 {
					// received wrong message type

				} else {
					t.Error("should have got wrong message type")
				}
				complete <- true
				break
			}
		}

	}()

	go func() {
		for {
			m, _ := cc.Read()

			if m.Status == "Connected" {
				connected2 <- true
			}
		}
	}()

	<-connected
	<-connected2

	// test wrong message type
	cc.Write(2, []byte("hello server 1"))

	<-complete
}
func TestClientWrongMessageType(t *testing.T) {

	sc, err := StartServer(serverConfig("test3"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test3"))
	if err2 != nil {
		t.Error(err)
	}

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {
		for {
			m, _ := sc.Read()
			if m.Status == "Connected" {
				connected2 <- true
				continue

			}

		}
	}()

	go func() {

		ready := false

		for {

			m, err45 := cc.Read()

			if m.Status == "Connected" {
				connected <- true
				ready = true
				continue

			}

			if ready == true {

				if err45 == nil {
					if m.MsgType != 5 {
						// received wrong message type
					} else {
						t.Error("should have got wrong message type")
					}
					complete <- true
					break

				} else {
					t.Error(err45)
					break
				}
			}

		}
	}()

	<-connected
	<-connected2
	sc.Write(2, []byte(""))

	<-complete
}
func TestServerCorrectMessageType(t *testing.T) {

	sc, err := StartServer(serverConfig("test358"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test358"))
	if err2 != nil {
		t.Error(err)
	}

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {
		for {
			m, err := sc.Read()
			if err == nil && m.Status == "Connected" {
				connected2 <- true
			}
		}
	}()

	go func() {

		ready := false

		for {
			m, err23 := cc.Read()
			if err23 == nil && m.Status == "Connected" {
				ready = true
				connected <- true
				continue
			}
			if ready == true {
				if err23 == nil {
					if m.MsgType == 5 {
						// received correct message type
					} else {
						t.Error("should have got correct message type")
					}

					complete <- true

				} else {
					t.Error(err23)
					break
				}
			}

		}
	}()

	<-connected
	<-connected2

	sc.Write(5, []byte(""))

	<-complete
}

func TestClientCorrectMessageType(t *testing.T) {

	sc, err := StartServer(serverConfig("test355"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test355"))
	if err2 != nil {
		t.Error(err)
	}

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {

		for {
			m, _ := cc.Read()

			if m.Status == "Connected" {
				connected2 <- true
			}
		}

	}()

	go func() {

		ready := false

		for {

			m, err34 := sc.Read()

			if m.Status == "Connected" {
				ready = true
				connected <- true
				continue
			}

			if ready == true {
				if err34 == nil {
					if m.MsgType == 5 {
						// received correct message type
					} else {
						t.Error("should have got correct message type")
					}

					complete <- true

				} else {
					t.Error(err34)
					break
				}
			}
		}
	}()

	<-connected2
	<-connected

	cc.Write(5, []byte(""))
	<-complete
}
func TestServerSendMessage(t *testing.T) {

	sc, err := StartServer(serverConfig("test377"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test377"))
	if err2 != nil {
		t.Error(err)
	}

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {

		for {

			m, err := sc.Read()
			if err != nil {
				t.Error(fmt.Sprintf("Got read error: %s", err))
			} else if m.Status == "Connected" {
				connected <- true
			}
		}
	}()

	go func() {

		ready := false

		for {

			m, err56 := cc.Read()

			if m.Status == "Connected" {
				ready = true
				connected2 <- true
				continue
			}

			if ready == true {
				if err56 == nil {
					if m.MsgType == 5 {
						if string(m.Data) == "Here is a test message sent from the server to the client... -/and some more test data to pad it out a bit" {
							// correct msg has been received
						} else {
							t.Error("Message recreceivedieved is wrong")
						}
					} else {
						t.Error("should have got correct message type")
					}

					complete <- true
					break

				} else {
					t.Error(err56)
					complete <- true
					break
				}

			}
		}

	}()

	<-connected2
	<-connected

	sc.Write(5, []byte("Here is a test message sent from the server to the client... -/and some more test data to pad it out a bit"))

	<-complete
}
func TestClientSendMessage(t *testing.T) {

	sc, err := StartServer(serverConfig("test3661"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test3661"))
	if err2 != nil {
		t.Error(err)
	}

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {

		for {

			m, _ := cc.Read()
			if m.Status == "Connected" {
				connected <- true
			}

		}
	}()

	go func() {

		ready := false

		for {

			m, _ := sc.Read()

			if m.Status == "Connected" {
				ready = true
				connected2 <- true
				continue
			}

			if ready == true {
				if err == nil {
					if m.MsgType == 5 {

						if string(m.Data) == "Here is a test message sent from the client to the server... -/and some more test data to pad it out a bit" {
							// correct msg has been received
						} else {
							t.Error("Message recreceivedieved is wrong")
						}

					} else {
						t.Error("should have got correct message type")
					}
					complete <- true
					break

				} else {
					t.Error(err)
					complete <- true
					break
				}
			}

		}
	}()

	<-connected
	<-connected2

	cc.Write(5, []byte("Here is a test message sent from the client to the server... -/and some more test data to pad it out a bit"))

	<-complete
}

func TestClientClose(t *testing.T) {

	sc, err := StartServer(serverConfig("test10A"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test10A"))
	if err2 != nil {
		t.Error(err)
	}

	holdIt := make(chan bool, 1)

	go func() {

		for {

			m, _ := sc.Read()

			if m.Status == "Disconnected" {
				holdIt <- false
				break
			}

		}

	}()

	for {

		mm, err := cc.Read()

		if err == nil {
			if mm.Status == "Connected" {
				cc.Close()
			}

			if mm.Status == "Closed" {
				break
			}
		}

	}

	<-holdIt
}

func TestServerClose(t *testing.T) {

	sc, err := StartServer(serverConfig("test1010"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test1010"))
	if err2 != nil {
		t.Error(err)
	}

	holdIt := make(chan bool, 1)

	go func() {

		for {

			m, _ := cc.Read()

			if m.Status == "Reconnecting" {
				holdIt <- false
				break
			}
		}

	}()

	for {

		mm, err2 := sc.Read()

		if err2 == nil {
			if mm.Status == "Connected" {
				sc.Close()
			}

			if mm.Status == "Closed" {
				break
			}
		}

	}

	<-holdIt
}

func TestClientReconnect(t *testing.T) {

	sc, err := StartServer(serverConfig("test127"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	ccon := clientConfig("test127")

	cc, err2 := StartClient(ccon)
	if err2 != nil {
		t.Error(err)
	}
	connected := make(chan bool, 1)
	clientConfirm := make(chan bool, 1)
	clientConnected := make(chan bool, 1)

	go func() {

		for {

			m, _ := sc.Read()
			if m.Status == "Connected" {
				connected <- true
				break
			}

		}
	}()

	go func() {

		reconnectCheck := 0

		for {

			m, _ := cc.Read()
			if m.Status == "Connected" {
				clientConnected <- true
			}

			if m.Status == "Reconnecting" {
				reconnectCheck = 2
			}

			if m.Status == "Connected" && reconnectCheck == 2 {
				clientConfirm <- true
				break
			}
		}
	}()

	<-connected
	<-clientConnected

	sc.Close()

	sc2, err := StartServer(serverConfig("test127"))
	if err != nil {
		t.Error(err)
	}

	for {

		m, _ := sc2.Read()
		if m.Status == "Connected" {
			<-clientConfirm
			break
		}
	}
}

func TestClientReconnectTimeout(t *testing.T) {

	server, err := StartServer(serverConfig("test7"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	config := &ClientConfig{
		Name:       "test7",
		Timeout:    2 * time.Second,
		RetryTimer: 1,
	}

	cc, err2 := StartClient(config)
	if err2 != nil {
		t.Error(err)
	}

	go func() {

		for {

			m, _ := server.Read()
			if m.Status == "Connected" {
				server.Close()
				break
			}

		}
	}()

	connect := false
	reconnect := false

	for {

		mm, err5 := cc.Read()

		if err5 == nil {
			if mm.Status == "Connected" {
				connect = true
			}

			if mm.Status == "Reconnecting" {
				reconnect = true
			}

			if mm.Status == "Timeout" && reconnect == true && connect == true {
				return
			}
		}

		if err5 != nil {
			if err5.Error() != "timed out trying to re-connect" {
				t.Fatal("should have got the timed out error")
			}

			break

		}
	}
}

func TestServerReconnect(t *testing.T) {

	sc, err := StartServer(serverConfig("test1277"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	ccon := clientConfig("test1277")

	cc, err2 := StartClient(ccon)
	if err2 != nil {
		t.Error(err2)
	}
	connected := make(chan bool, 1)
	clientConfirm := make(chan bool, 1)
	clientConnected := make(chan bool, 1)

	go func() {

		for {
			//cc.logger.Debug("x")
			m, _ := cc.Read()
			if m.Status == "Connected" {
				//cc.logger.Debug("a")
				<-clientConnected
				//cc.logger.Debug("b")
				connected <- true
				break
			}

		}
	}()

	go func() {

		reconnectCheck := 0

		for {
			//We used ReadTimed in order to scan client buffer on the next loop
			sc.ServerManager.ReadTimed(2*time.Second, TimeoutMessage, func(_ *Server, m *Message, err error) {
				//sc.ServerManager.Read(func(_ *Server, m *Message, err error) {
				//sc.logger.Debugf("z %s %d", m.Status, reconnectCheck)

				if err != nil {
					//sc.logger.Debugf("TestServerReconnect sever read loop err: %s", err)
					return
				}

				if m.Status == "Connected" {
					//sc.logger.Debug("c")
					clientConnected <- true
				}

				if m.Status == "Disconnected" {
					//sc.logger.Debug("d")
					reconnectCheck = 1
				}

				if m.Status == "Connected" && reconnectCheck == 1 {
					//sc.logger.Debug("e")
					clientConfirm <- true
				}
			})
		}
	}()

	//cc.logger.Debug("f")
	<-connected
	cc.Close()

	//cc.logger.Debug("h")
	c2, err := StartClient(clientConfig("test1277"))
	if err != nil {
		t.Error(err)
	}

	for {
		//cc.logger.Debug("i")
		m, _ := c2.Read()
		if m.Status == "Connected" {
			//cc.logger.Debug("j")
			break
		}
	}

	//cc.logger.Debug("k")
	<-clientConfirm

	sc.Close()
	cc.Close()
}

func TestServerReconnect2(t *testing.T) {

	sc, err := StartServer(serverConfig("test337"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	cc, err2 := StartClient(clientConfig("test337"))
	if err2 != nil {
		t.Error(err)
	}

	hasConnected := make(chan bool, 1)
	hasDisconnected := make(chan bool, 1)
	hasReconnected := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-hasReconnected:
				return
			default:
				m, err := cc.Read()
				//cc.logger.Warnf("Z %s", m.Status)
				if m.Status == "Connected" {

					<-hasConnected

					cc.Close()

					<-hasDisconnected

					c2, err2 := StartClient(clientConfig("test337"))
					if err2 != nil {
						t.Error(err)
					}

					for {
						n, _ := c2.Read()
						//cc.logger.Warnf("Y  %s", m.Status)
						if n.Status == "Connected" {
							c2.Close()
							break
						}
					}
					//cc.logger.Warnf("Y 2")
					return
				}
			}
		}
	}()

	connect := false
	disconnect := false

	for {
		select {
		case <-hasReconnected:
			//sc.logger.Warnf("X 4")
			return
		default:
			sc.ServerManager.ReadTimed(2*time.Second, TimeoutMessage, func(_ *Server, m *Message, err error) {
				//sc.logger.Warnf("X %s %t %t", m.Status, connect, disconnect)
				if m.Status == "Connected" && connect == false {
					//sc.logger.Warnf("X 1")
					hasConnected <- true
					connect = true
				}

				if m.Status == "Disconnected" {
					//sc.logger.Warnf("X 2")
					hasDisconnected <- true
					disconnect = true
				}

				if m.Status == "Connected" && connect == true && disconnect == true {
					//sc.logger.Warnf("X 3")
					hasReconnected <- true
				}
			})
		}
	}
	sc.Close()
	cc.Close()
}

func TestClientReadClose(t *testing.T) {

	sc, err := StartServer(serverConfig("test7R"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(DEFAULT_CLIENT_CONNECT_WAIT)

	config := &ClientConfig{
		Timeout:    2 * time.Second,
		RetryTimer: 1,
		Name:       "test7R",
	}

	cc, err2 := StartClient(config)
	if err2 != nil {
		t.Error(err)
	}
	connected := make(chan bool, 1)
	clientTimout := make(chan bool, 1)
	clientConnected := make(chan bool, 1)
	clientError := make(chan bool, 1)

	go func() {

		for {

			m, _ := sc.Read()
			if m.Status == "Connected" {
				connected <- true
				break
			}

		}
	}()

	go func() {

		reconnect := false

		for {

			m, err3 := cc.Read()

			if err3 != nil {
				log.Printf("err: %s", err3)
				if err3.Error() == "the received channel has been closed" {
					clientError <- true // after the connection times out the received channel is closed, so we're now testing that the close error is returned.
					// This is the only error the received function returns.
					break
				}
			}

			if err3 == nil {
				if m.Status == "Connected" {
					clientConnected <- true
				}

				if m.Status == "Reconnecting" {
					reconnect = true
				}

				if m.Status == "Timeout" && reconnect == true {
					clientTimout <- true
				}
			}

		}
	}()

	<-connected
	<-clientConnected

	sc.Close()
	<-clientTimout
	<-clientError
}

func TestServerReceiveWrongVersionNumber(t *testing.T) {

	sc, err := StartServer(serverConfig("test5"))
	if err != nil {
		t.Error(err)
	}

	go func() {

		cc, err2 := NewClient("test5", nil)
		if err2 != nil {
			t.Error(err2)
		}

		time.Sleep(3 * time.Second)
		cc.clientId = 1
		conn, err := net.Dial("unix", cc.getSocketName())
		if err != nil {
			t.Error(err)
		}
		cc.conn = conn

		recv := make([]byte, 2)
		_, err2 = cc.conn.Read(recv)
		if err2 != nil {
			return
		}

		if recv[0] != 4 {
			cc.handshakeSendReply(cc.conn, 1)
			return
		}

	}()

	for {

		m, err := sc.Read()
		if err != nil {
			fmt.Println(err)
			return
		}

		if m.Err != nil {
			if m.Err.Error() != "client has a different VERSION number" {
				t.Error("should have error because server sent the client the wrong VERSION number 1")
			}
		}
	}
}
