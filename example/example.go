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
	time.Sleep(time.Second * 4)

	_, err = ipc.StartClient("example1", nil)
	if err != nil {
		log.Printf("client error %s:", err)
		main()
	}

	time.Sleep(time.Second * 4)

	c, err := ipc.StartClient("example1", nil)
	if err != nil {
		log.Printf("client error %s:", err)
		main()
	}

	time.Sleep(time.Duration(ipc.GetDefaultClientConnectWait()) * time.Second)

	for {

		message, err := c.Read()

		if err == nil && c.StatusCode() != ipc.Connecting {

			if message.MsgType == -1 {

				log.Println("client status", c.Status())

				if message.Status == "Reconnecting" {
					c.Close()
					return
				}

			} else {

				log.Println("Client received: "+string(message.Data)+" - Message type: ", message.MsgType)
				c.Write(5, []byte("Message from client - PONG"))

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
						s.Write(1, []byte("server - PING"))

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
						s.Write(1, []byte("server - PING"))

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
