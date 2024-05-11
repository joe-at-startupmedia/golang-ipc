package main

import (
	"fmt"
	ipc "github.com/joe-at-startupmedia/golang-ipc"
	"log"
	"time"
)

// prevents a race condition where the client attempts to connect before the server begins listening
var serverErrorChan = make(chan error, 1)
var serverChan = make(chan *ipc.Server, 1)

func main() {

	go server()
	err := <-serverErrorChan
	srv := <-serverChan
	if err != nil {
		log.Printf("server error %s:", err)
		main()
	}

	// change the sleep time by using IPC_WAIT env variable (seconds)
	ipc.Sleep()

	clientConfig := &ipc.ClientConfig{Name: "example-simple", Encryption: ipc.ENCRYPT_BY_DEFAULT}

	c1, err := ipc.StartClient(clientConfig)
	if err != nil {
		log.Printf("client error %s:", err)
		main()
	}

	ipc.Sleep()

	serverPonger(c1)

	ipc.Sleep()

	c1.Close()
	srv.Close()

	ipc.Sleep()
}

func serverPonger(c *ipc.Client) {

	pongMessage := fmt.Sprintf("Message from client(%d) - PONG", c.ClientId)

	for {

		message, err := c.ReadTimed(time.Second*5, ipc.TimeoutMessage)

		if err == nil && c.StatusCode() != ipc.Connecting {

			//log.Printf("Client(%d) received: %s - Message type: %d, Message Status %s", c.ClientId, string(message.Data), message.MsgType, message.Status)

			if message.MsgType == -1 {

				log.Println("client status", c.Status())

				if message.Status == "Reconnecting" {
					c.Close()
					return
				} else if message.Status == "Connected" {
					c.Write(5, []byte(pongMessage))
				}

			} else {

				log.Printf("Client(%d) received: %s - Message type: %d", c.ClientId, string(message.Data), message.MsgType)
				break
			}

		} else if err != nil {
			log.Println("Read err: ", err)
			//this happens in rare edge cases when the client attempts to connect too fast after server is listening
			if err.Error() == "Client.Read timed out" {
				main()
				break
			}
			//break
		} else {
			log.Println("client status", c.Status())
		}
		ipc.Sleep()
	}

}

func server() {

	srv, err := ipc.StartServer(&ipc.ServerConfig{Name: "example-simple", Encryption: ipc.ENCRYPT_BY_DEFAULT})
	serverErrorChan <- err

	if err != nil {
		log.Println("server error", err)
		return
	}

	serverChan <- srv
	//log.Println("server status", srv.Status())

	for {
		msg, err := srv.ReadTimed(time.Second*5, ipc.TimeoutMessage)
		server_onRead(srv, msg, err)
	}
}

func server_onRead(s *ipc.Server, message *ipc.Message, err error) {
	if err == nil {

		if message.MsgType == -1 {

			if message.Status == "Connected" {

				log.Println("server sending ping: status", s.Status())
				s.Write(1, []byte("server - PING"))

			}

		} else {

			log.Println("Server received: "+string(message.Data)+" - Message type: ", message.MsgType)
			s.Write(1, []byte("server - PING"))
		}

	} else {
		log.Println("Read err: ", err)
	}
}
