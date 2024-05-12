package main

import (
	"fmt"
	"log"
	"time"

	ipc "github.com/joe-at-startupmedia/golang-ipc"
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

	clientConfig := &ipc.ClientConfig{Name: "example-multi", MultiClient: true, Encryption: ipc.ENCRYPT_BY_DEFAULT}

	c1, err := ipc.StartClient(clientConfig)
	if err != nil {
		log.Printf("client error %s:", err)
		main()
	}

	ipc.Sleep()

	c2, err := ipc.StartClient(clientConfig)
	if err != nil {
		log.Printf("client error %s:", err)
		main()
	}

	serverPonger(c2, false)

	ipc.Sleep()

	serverPonger(c1, false)

	ipc.Sleep()

	serverPonger(c2, true)

	ipc.Sleep()

	srv.Close()
	c1.Close()
	c2.Close()

	ipc.Sleep()
}

func serverPonger(c *ipc.Client, autosend bool) {

	pongMessage := fmt.Sprintf("Message from client(%d) - PONG", c.ClientId)

	if autosend {
		c.Write(5, []byte(pongMessage))
	}

	for {

		message, err := c.ReadTimed(5*time.Second, ipc.TimeoutMessage)

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

			} else if message != ipc.TimeoutMessage {

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

	srv, err := ipc.StartServer(&ipc.ServerConfig{Name: "example-multi", MultiClient: true, Encryption: ipc.ENCRYPT_BY_DEFAULT})
	serverErrorChan <- err

	if err != nil {
		log.Println("server error", err)
		return
	}

	serverChan <- srv
	//log.Println("server status", srv.Status())

	for {
		//log.Println("server status loop", srv.Status())
		// we need to use the ReadTimed in order to poll all new clients
		srv.ServerManager.ReadTimed(5*time.Second, ipc.TimeoutMessage, func(s *ipc.Server, message *ipc.Message, err error) {
			if err == nil {

				if message.MsgType == -1 {

					if message.Status == "Connected" {

						log.Println("server sending ping: status", s.Status())
						s.Write(1, []byte("server - PING"))

					}

				} else if message != ipc.TimeoutMessage {

					log.Println("Server received: "+string(message.Data)+" - Message type: ", message.MsgType)
					s.Write(1, []byte("server - PING"))
				}

			} else {
				log.Println("Read err: ", err)
			}
		})
	}
}
