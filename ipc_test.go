package ipc

import (
	"fmt"
	"log"
	"strings"
	"testing"
	"time"
)

var RetriesEnabledClientConfig = &ClientConfig{
	Timeout:    time.Second * 3,
	RetryTimer: time.Second * 1,
	Encryption: ENCRYPT_BY_DEFAULT,
}

func serverConfig(name string) *ServerConfig {
	return &ServerConfig{Name: name, Encryption: ENCRYPT_BY_DEFAULT}
}

func clientConfig(name string) *ClientConfig {
	return &ClientConfig{Name: name, Encryption: ENCRYPT_BY_DEFAULT}
}

func TestStartUp_Name(t *testing.T) {

	_, err := StartServer(serverConfig(""))
	if err.Error() != "ipcName cannot be an empty string" {
		t.Error("server - should have an error becuse the ipc name is empty")
	}

	_, err2 := StartClient(clientConfig(""))
	if err2.Error() != "ipcName cannot be an empty string" {
		t.Error("client - should have an error becuse the ipc name is empty")
	}
}

func TestStartUp_Configs(t *testing.T) {

	scon := serverConfig("test_config")
	ccon := clientConfig("test_config")

	sc, err3 := StartServer(scon)
	if err3 != nil {
		t.Error(err3)
	}
	defer sc.Close()

	Sleep()

	cc, err4 := StartClient(ccon)
	if err4 != nil {
		t.Error(err4)
	}
	defer cc.Close()
}

func TestStartUp_Configs2(t *testing.T) {

	scon := serverConfig("test_config2")
	ccon := clientConfig("test_config2")

	scon.MaxMsgSize = -1

	sc, err5 := StartServer(scon)
	if err5 != nil {
		t.Error(err5)
	}
	defer sc.Close()

	Sleep()

	//testing junk values that will default to 0
	ccon.Timeout = -1
	ccon.RetryTimer = -1

	cc, err6 := StartClient(ccon)
	if err6 != nil {
		t.Error(err6)
	}
	defer cc.Close()
}

func TestStartUp_Configs3(t *testing.T) {

	scon := serverConfig("test_config3")
	ccon := clientConfig("test_config3")

	scon.MaxMsgSize = 1025

	sc, err7 := StartServer(scon)
	if err7 != nil {
		t.Error(err7)
	}
	defer sc.Close()

	Sleep()

	cc, err8 := StartClient(ccon)
	if err8 != nil {
		t.Error(err8)
	}
	defer cc.Close()
}

func TestTimeoutNoServer(t *testing.T) {

	ccon := clientConfig("test_timeout")
	ccon.Timeout = 2 * time.Second

	cc, err := StartClient(ccon)
	defer cc.Close()

	if !strings.Contains(err.Error(), "timed out trying to connect") {
		t.Error(err)
	}
}

func TestTimeoutNoServerRetry(t *testing.T) {

	ccon := clientConfig("test_timeout_retryloop")
	ccon.Timeout = 2000 * time.Millisecond //extremely impractical low value for testing purposes only
	ccon.RetryTimer = 1000 * time.Millisecond

	dialFinished := make(chan bool, 1)

	go func() {
		time.Sleep(time.Second * 3)
		dialFinished <- true
	}()

	go func() {
		//this should retry every second and never return
		cc, err := StartClient(ccon)
		defer cc.Close()
		if err != nil && !strings.Contains(err.Error(), "timed out trying to connect") {
			t.Error(err)
		}
	}()

	<-dialFinished
}

func TestTimeoutServerDisconnected(t *testing.T) {

	scon := serverConfig("test_timeout_server_disconnect")
	sc, err := StartServer(scon)
	if err != nil {
		t.Error(err)
	}

	ccon := clientConfig("test_timeout_server_disconnect")
	ccon.Timeout = 2 * time.Second

	Sleep()

	cc, err2 := StartClient(ccon)
	if err2 != nil {
		t.Error(err2)
	}
	defer cc.Close()

	go func() {
		time.Sleep(time.Second * 1)
		sc.Close()
	}()

	for {
		_, err := cc.Read() //Timed(time.Second*2, TimeoutMessage)
		if err != nil {
			//this error will only be reached if Timeout is specified, otherwise
			//the reconnect dial loop will loop perpetually
			if err.Error() == "the received channel has been closed" {
				break
				//}
				//this will be the first error captured before received channel closure
			} else if err.Error() == "timed out trying to re-connect" {
				break
			}
		}
	}
}

func TestWrite(t *testing.T) {

	sc, err := StartServer(serverConfig("test_write"))
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	Sleep()

	cc, err2 := StartClient(clientConfig("test_write"))
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

	connected := make(chan bool, 1)

	go func() {

		for {

			m, err := cc.Read()
			if err != nil {
				//t.Error(fmt.Sprintf("Got read error: %s", err))
			} else if m.Status == "Connected" {
				connected <- true
				return
			}
		}
	}()

	<-connected

	buf := make([]byte, 1)

	err3 := sc.Write(0, buf)

	if err3.Error() != "message type 0 is reserved" {
		t.Error("0 is not allowed as a message type")
	}

	buf = make([]byte, sc.config.ServerConfig.MaxMsgSize+5)
	err4 := sc.Write(2, buf)

	if err4.Error() != "message exceeds maximum message length" {
		t.Errorf("There should be an error as the data we're attempting to write is bigger than the MAX_MSG_SIZE, instead we got: %s", err4)
	}

	sc.setStatus(NotConnected)

	buf2 := make([]byte, 5)
	err5 := sc.Write(2, buf2)
	if err5.Error() != "cannot write under current status: Not Connected" {
		t.Errorf("we should have an error becuse there is no connection but instead we got: %s", err5)
	}

	sc.setStatus(Connected)

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

	cc.setStatus(NotConnected)

	buf = make([]byte, 5)
	err = cc.Write(2, buf)
	if err.Error() == "cannot write under current status: Not Connected" {

	} else {
		t.Error("we should have an error becuse there is no connection")
	}
}

func TestRead(t *testing.T) {

	sIPC := &Server{Actor: Actor{
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
		Actor:      Actor{status: NotConnected, received: make(chan *Message)},
		timeout:    2 * time.Second,
		retryTimer: 1 * time.Second,
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

	sc, err := StartServer(serverConfig("test_status"))
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	sc.setStatus(NotConnected)

	if sc.Status() != "Not Connected" {
		t.Error("status string should have returned Not Connected")
	}

	sc.setStatus(Listening)

	if sc.Status() != "Listening" {
		t.Error("status string should have returned Listening")
	}

	sc.setStatus(Connecting)

	if sc.Status() != "Connecting" {
		t.Error("status string should have returned Connecting")
	}

	sc.setStatus(Connected)

	if sc.Status() != "Connected" {
		t.Error("status string should have returned Connected")
	}

	sc.setStatus(ReConnecting)

	if sc.Status() != "Reconnecting" {
		t.Error("status string should have returned Reconnecting")
	}

	sc.setStatus(Closed)

	if sc.Status() != "Closed" {
		t.Error("status string should have returned Closed")
	}

	sc.setStatus(Error)

	if sc.Status() != "Error" {
		t.Error("status string should have returned Error")
	}

	sc.setStatus(Closing)

	if sc.Status() != "Closing" {
		t.Error("status string should have returned Error")
	}
}

func TestGetConnected(t *testing.T) {

	sc, err := StartServer(serverConfig("test22"))
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	Sleep()

	cc, err2 := StartClient(clientConfig("test22"))
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

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
	defer sc.Close()

	Sleep()

	cc, err2 := StartClient(clientConfig("test333"))
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

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
				return
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
	defer sc.Close()

	Sleep()

	cc, err2 := StartClient(clientConfig("test3"))
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {
		for {
			m, _ := sc.Read()
			if m.Status == "Connected" {
				connected2 <- true
				return
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
	defer sc.Close()

	Sleep()

	cc, err2 := StartClient(clientConfig("test358"))
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {
		for {
			m, err := sc.Read()
			if err == nil && m.Status == "Connected" {
				connected2 <- true
				return
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
					return
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
	defer sc.Close()

	Sleep()

	cc, err2 := StartClient(clientConfig("test355"))
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {

		for {
			m, _ := cc.Read()

			if m.Status == "Connected" {
				connected2 <- true
				return
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
					return
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
	defer sc.Close()

	Sleep()

	cc, err2 := StartClient(clientConfig("test377"))
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

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
				return
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
	defer sc.Close()

	Sleep()

	cc, err2 := StartClient(clientConfig("test3661"))
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

	connected := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	complete := make(chan bool, 1)

	go func() {

		for {

			m, _ := cc.Read()
			if m.Status == "Connected" {
				connected <- true
				return
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
	defer sc.Close()

	Sleep()

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

func TestClientReconnect(t *testing.T) {

	scon := serverConfig("test127")
	sc, err := StartServer(scon)
	if err != nil {
		t.Error(err)
	}

	Sleep()

	ccon := clientConfig("test127")

	cc, err2 := StartClient(ccon)
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()
	connected := make(chan bool, 1)
	clientConfirm := make(chan bool, 1)
	clientConnected := make(chan bool, 1)

	go func() {
		for {
			m, _ := sc.Read()
			if m.Status == "Connected" {
				connected <- true
				return
			}
		}
	}()

	go func() {

		reconnectCheck := false

		for {
			m, _ := cc.Read()
			if m == TimeoutMessage {
				continue
			} else if m != nil {
				if m.Status == "Connected" {
					if !reconnectCheck {
						clientConnected <- true
					} else {
						clientConfirm <- true
						return
					}
				} else if m.Status == "Reconnecting" {
					reconnectCheck = true
				}
			}
		}
	}()

	//wait for both the client and server to connect
	<-connected
	<-clientConnected

	//disconnect from the server
	sc.Close()

	//start a new server
	sc2, err := StartServer(scon)
	if err != nil {
		t.Error(err)
	}
	defer sc2.Close()

	for {

		m, _ := sc2.Read()
		if m.Status == "Connected" {
			<-clientConfirm
			break
		}
	}
}

func TestClientReconnectTimeout(t *testing.T) {

	sc, err := StartServer(serverConfig("test7"))
	if err != nil {
		t.Error(err)
	}

	Sleep()

	config := &ClientConfig{
		Name:       "test7",
		Timeout:    3 * time.Second,
		Encryption: ENCRYPT_BY_DEFAULT,
	}

	cc, err2 := StartClient(config)
	if err2 != nil {
		t.Error(err)
	}
	defer cc.Close()

	go func() {

		for {

			m, _ := sc.Read()
			if m.Status == "Connected" {
				sc.Close()
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
			} else if mm.Status == "Reconnecting" {
				reconnect = true
			} else if mm.Status == "Timeout" && reconnect == true && connect == true {
				return
			}
		} else {
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
	defer sc.Close()

	Sleep()

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
			m, _ := cc.Read()
			if m != nil && m.Status == "Connected" {
				<-clientConnected
				connected <- true
			}
		}
	}()

	go func() {

		reconnectCheck := 0
		connectCnt := 0
		for {
			m, err := sc.Read()

			if err != nil {
				//sc.logger.Debugf("TestServerReconnect sever read loop err: %s", err)
				//t.Error(err)
				return
			}

			if m.Status == "Connected" {
				if reconnectCheck == 1 && connectCnt > 0 {
					clientConfirm <- true
				} else {
					clientConnected <- true
					connectCnt++
				}
			}

			//dispatched on EOF
			if m.Status == "Disconnected" {
				reconnectCheck = 1
			}

		}
	}()

	<-connected
	cc.Close()

	//THIS IS IMPORTANT
	// this allows time for the server to realize the client disconnected
	// before adding the new client. If this is absent, tests will continue
	// working most of the time except when the rare race conditions are met
	time.Sleep(time.Second * 1)

	ccon = RetriesEnabledClientConfig
	ccon.Name = "test1277"
	ccon.Timeout = 2 * time.Second
	cc2, err := StartClient(ccon)
	if err != nil {
		t.Error(err)
	}
	defer cc2.Close()

	<-clientConfirm
}

func TestServerReconnect2(t *testing.T) {

	sc, err := StartServer(serverConfig("test337"))
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	Sleep()

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
						if n.Status == "Connected" {
							c2.Close()
							break
						}
					}
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
			return
		default:
			m, _ := sc.Read()
			if m.Status == "Connected" && connect == false {
				hasConnected <- true
				connect = true
			}

			if m.Status == "Disconnected" {
				hasDisconnected <- true
				disconnect = true
			}

			if m.Status == "Connected" && connect == true && disconnect == true {
				hasReconnected <- true
			}
		}
	}
}

func TestServerReconnectMulti(t *testing.T) {

	scon := serverConfig("test1277_multi")
	scon.MultiClient = true
	sc, err := StartServer(scon)
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	Sleep()

	ccon := clientConfig("test1277_multi")
	ccon.MultiClient = true
	cc, err2 := StartClient(ccon)
	if err2 != nil {
		t.Error(err)
	}

	connected := make(chan bool, 1)
	clientConfirm := make(chan bool, 1)
	clientConnected := make(chan bool, 1)

	go func() {

		for {
			m, _ := cc.Read()
			if m.Status == "Connected" {
				<-clientConnected
				connected <- true
				break
			}

		}
	}()

	go func() {

		reconnectCheck := 0

		for {
			//We used ReadTimed in order to scan client buffer on the next loop
			sc.Connections.ReadTimed(2*time.Second, func(_ *Server, m *Message, err error) {

				if err != nil {
					return
				}

				if m.Status == "Connected" {
					clientConnected <- true
				}

				if m.Status == "Disconnected" {
					reconnectCheck = 1
				}

				if m.Status == "Connected" && reconnectCheck == 1 {
					clientConfirm <- true
				}
			})
		}
	}()

	<-connected
	cc.Close()

	time.Sleep(1 * time.Second) //wait for connection to close before reconnecting
	ccon = clientConfig("test1277_multi")
	ccon.MultiClient = true
	c2, err := StartClient(ccon)
	if err != nil {
		t.Error(err)
	}
	defer c2.Close()

	for {
		m, _ := c2.Read()
		if m.Status == "Connected" {
			break
		}
	}

	<-clientConfirm
}

func TestServerReconnect2Multi(t *testing.T) {
	scon := serverConfig("test337_multi")
	scon.MultiClient = true
	sc, err := StartServer(scon)
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	Sleep()
	ccon := clientConfig("test337_multi")
	ccon.MultiClient = true
	cc, err2 := StartClient(ccon)
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
				if m.Status == "Connected" {

					<-hasConnected

					cc.Close()

					<-hasDisconnected

					ccon2 := clientConfig("test337_multi")
					ccon2.MultiClient = true
					c2, err2 := StartClient(ccon2)
					if err2 != nil {
						t.Error(err)
					}

					for {
						n, _ := c2.Read()
						if n.Status == "Connected" {
							c2.Close()
							break
						}
					}
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
			return
		default:
			sc.Connections.ReadTimed(2*time.Second, func(_ *Server, m *Message, err error) {
				if m.Status == "Connected" && connect == false {
					hasConnected <- true
					connect = true
				}

				if m.Status == "Disconnected" {
					hasDisconnected <- true
					disconnect = true
				}

				if m.Status == "Connected" && connect == true && disconnect == true {
					hasReconnected <- true
				}
			})
		}
	}
}

func TestClientReadClose(t *testing.T) {

	sc, err := StartServer(serverConfig("test_clientReadClose"))
	if err != nil {
		t.Error(err)
	}

	Sleep()

	config := &ClientConfig{
		Timeout:    3 * time.Second,
		RetryTimer: 1 * time.Second,
		Name:       "test_clientReadClose",
		Encryption: ENCRYPT_BY_DEFAULT,
	}

	cc, err2 := StartClient(config)
	if err2 != nil {
		t.Error(err2)
	}
	defer cc.Close()

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
				} else if m.Status == "Reconnecting" {
					reconnect = true
				} else if m.Status == "Timeout" && reconnect == true {
					clientTimout <- true
				}
			}
		}
	}()

	<-connected
	<-clientConnected
	//IMPORTANT Close was not placed here by mistake
	sc.Close()
	<-clientTimout
	<-clientError
}

func TestServerReceiveWrongVersionNumber(t *testing.T) {

	sc, err := StartServer(serverConfig("test5"))
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	go func() {

		cc, err2 := NewClient("test5", nil)
		if err2 != nil {
			t.Error(err2)
		}
		defer cc.Close()

		Sleep()
		//cc.ClientId = 1
		conn, err := cc.connect()
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
			cc.handshakeSendReply(1)
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

func TestServerReceiveWrongVersionNumberMulti(t *testing.T) {

	config := serverConfig("test5")
	config.MultiClient = true
	sc, err := StartServer(config)
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	go func() {

		cc, err2 := NewClient("test5", &ClientConfig{MultiClient: true})
		if err2 != nil {
			t.Error(err2)
		}
		defer cc.Close()

		Sleep()
		cc.ClientId = 1
		conn, err := cc.connect()
		if err != nil {
			t.Error(err)
		}
		cc.conn = conn
		Sleep()
		recv := make([]byte, 2)
		_, err2 = conn.Read(recv)
		if err2 != nil {
			return
		}

		if recv[0] != 4 {
			cc.handshakeSendReply(1)
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

func TestServerWrongEncryption(t *testing.T) {

	scon := serverConfig("testl337_enc")
	scon.Encryption = false
	sc, err := StartServer(scon)
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	Sleep()
	ccon := clientConfig("testl337_enc")
	ccon.Encryption = true
	cc, err2 := StartClient(ccon)
	defer cc.Close()

	if err2 != nil {
		if err2.Error() != "server tried to connect without encryption" {
			t.Error(err2)
		}
	}

	go func() {
		for {
			m, err := cc.Read()
			cc.logger.Debugf("Message: %v, err %s", m, err)
			if err != nil {
				if err.Error() != "server tried to connect without encryption" && m.MsgType != -2 {
					t.Error(err)
				}
				break
			} else if m.Status == "Closed" {
				break
			}
		}
	}()

	for {
		mm, err2 := sc.Read()
		sc.logger.Debugf("Message: %v, err %s", mm, err)
		if err2 != nil {
			if err2.Error() != "client is enforcing encryption" && mm.MsgType != -2 {
				t.Error(err2)
			}
			break
		}
	}
}

func TestServerWrongEncryption2(t *testing.T) {

	scon := serverConfig("testl338_enc")
	scon.Encryption = true
	sc, err := StartServer(scon)
	if err != nil {
		t.Error(err)
	}
	defer sc.Close()

	Sleep()
	ccon := clientConfig("testl338_enc")
	ccon.Encryption = false
	cc, err2 := StartClient(ccon)
	defer cc.Close()
	if err2 != nil {
		if err2.Error() != "server tried to connect without encryption" {
			t.Error(err2)
		}
	}

	go func() {
		for {
			m, err := cc.Read()
			cc.logger.Debugf("Message: %v, err %s", m, err)
			if err != nil {
				if err.Error() != "server tried to connect without encryption" {
					if m != nil && m.MsgType != -2 {
						t.Error(err)
					}
				}
				break
			} else if m.Status == "Closed" {
				break
			}
		}
	}()

	for {
		mm, err2 := sc.Read()
		sc.logger.Debugf("Message: %v, err %s", mm, err2)
		if err2 != nil {
			if err2.Error() != "public key received isn't valid length 97, got: 1" {
				t.Error(err2)
			}
			break
		}
	}
}
