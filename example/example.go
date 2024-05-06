package main

import (
	"log"
	"time"

	ipc "github.com/joe-at-startupmedia/golang-ipc"
)

// prevents a race condition where the client attempts to connect before the server begins listening
var serverErrorChan = make(chan error, 1)

func main() {

	go server()
	err := <-serverErrorChan

	if err != nil {
		main()
	}
	c, err := ipc.StartClient("example1", nil)
	if err != nil {
		log.Printf("client error %s:", err)
		main()
	}

	time.Sleep(time.Duration(ipc.GetDefaultClientConnectWait()) * time.Second)

	for {

		message, err := c.ReadTimed(time.Second * 5)

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

func server() {

	s, err := ipc.StartServer("example1", nil)
	serverErrorChan <- err
	if err != nil {
		log.Println("server error", err)
		return
	}

	log.Println("server status", s.Status())

	for {

		message, err := s.Read()

		if err == nil {

			if message.MsgType == -1 {

				if message.Status == "Connected" {

					log.Println("server status", s.Status())
					s.Write(1, []byte("server - PING"))

				}

			} else {

				log.Println("Server received: "+string(message.Data)+" - Message type: ", message.MsgType)
				s.Close()
				return
			}

		} else {
			break
		}
	}
}
