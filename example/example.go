package main

import (
	"log"
	"time"

	ipc "github.com/joe-at-startupmedia/golang-ipc"
)

// prevents a race condition where the client attempts to connect before the server begins listening
var serverErrorChan = make(chan error, 1)

func main() {

	go serverReadTimed()
	err := <-serverErrorChan
	if err != nil {
		log.Printf("server error %s:", err)
		main()
	}

	//wait for server to connect
	time.Sleep(time.Duration(ipc.GetDefaultClientConnectWait()) * time.Second)

	clientConfig := &ipc.ClientConfig{Name: "example1"}

	c1, err := ipc.StartClient(clientConfig)
	if err != nil {
		log.Printf("client error %s:", err)
		main()
	}

	time.Sleep(time.Duration(ipc.GetDefaultClientConnectWait()) * time.Second)

	c2, err := ipc.StartClient(clientConfig)
	if err != nil {
		log.Printf("client error %s:", err)
		main()
	}

	serverPinger(c2)

	time.Sleep(time.Second * 10)

	serverPinger(c1)

	time.Sleep(time.Second * 5)
}

func serverPinger(c *ipc.Client) {
	for {

		message, err := c.ReadTimed(5*time.Second, ipc.TimeoutMessage)

		if err == nil && c.StatusCode() != ipc.Connecting {

			if message.MsgType == -1 {

				log.Println("client status", c.Status())

				if message.Status == "Reconnecting" {
					c.Close()
					return
				}

			} else {

				log.Println("Client received: "+string(message.Data)+" - Message type: ", message.MsgType)
				c.Write(5, []byte("Message from client - PING"))
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
		time.Sleep(time.Duration(ipc.GetDefaultClientConnectWait()) * time.Second)
	}

}

func serverRead() {

	srv, err := ipc.StartServer(&ipc.ServerConfig{Name: "example1"})
	serverErrorChan <- err
	if err != nil {
		log.Println("server error", err)
		return
	}

	//log.Println("server status", srv.Status())

	for {
		log.Println("server status loop", srv.Status())
		srv.ServerManager.Read(func(s *ipc.Server, message *ipc.Message, err error) {
			if err == nil {

				if message.MsgType == -1 {

					if message.Status == "Connected" {

						log.Println("server status", s.Status())
						s.Write(1, []byte("server - PONG"))

					}

				} else {

					log.Println("Server received: "+string(message.Data)+" - Message type: ", message.MsgType)
					log.Printf("ServerManager server length: %d", len(srv.ServerManager.Servers))
					//s.Close()
					//return
				}

			} else {
				log.Println("Read err: ", err)
			}
		})
	}
}

func serverReadTimed() {

	srv, err := ipc.StartServer(&ipc.ServerConfig{Name: "example1"})
	serverErrorChan <- err
	if err != nil {
		log.Println("server error", err)
		return
	}

	//log.Println("server status", srv.Status())

	for {
		log.Println("server status loop", srv.Status())
		srv.ServerManager.ReadTimed(5*time.Second, ipc.TimeoutMessage, func(s *ipc.Server, message *ipc.Message, err error) {
			if err == nil {

				if message.MsgType == -1 {

					if message.Status == "Connected" {

						log.Println("server status", s.Status())
						s.Write(1, []byte("server - PONG"))

					}

				} else if message != ipc.TimeoutMessage {

					log.Println("Server received: "+string(message.Data)+" - Message type: ", message.MsgType)
				}

			} else {
				log.Println("Read err: ", err)
			}
		})
	}
}
