package main

import (
	"fmt"
	ipc "github.com/joe-at-startupmedia/golang-ipc"
	"log"
	"time"
)

const CONN_NAME = "example_timed"

func main() {

	s := server()
	defer s.Close()

	// change the sleep time by using IPC_WAIT env variable (seconds)
	ipc.Sleep()

	clientConfig := &ipc.ClientConfig{Name: CONN_NAME, Encryption: ipc.ENCRYPT_BY_DEFAULT}
	c1, err := ipc.StartClient(clientConfig)
	if err != nil {
		panic(err)
	}
	defer c1.Close()

	serverPonger(c1)

	ipc.Sleep()
}

func serverPonger(c *ipc.Client) {

	pongMessage := fmt.Sprintf("Message from client(%d) - PONG", c.ClientId)

	for {

		message, err := c.ReadTimed(time.Second * 5)

		if message == ipc.TimeoutMessage {
			continue
		} else if err != nil {
			log.Println("Read err: ", err)
			//this happens in rare edge cases when the client attempts to connect too fast after server is listening
			if err.Error() == "Client.Read timed out" {
				panic(err)
			}
			continue
		}

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

		ipc.Sleep()
	}

}

func server() *ipc.Server {

	s, err := ipc.StartServer(&ipc.ServerConfig{Name: CONN_NAME, Encryption: ipc.ENCRYPT_BY_DEFAULT})
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			msg, err2 := s.ReadTimed(time.Second * 5)

			if msg == ipc.TimeoutMessage {
				continue
			} else if err2 != nil {
				log.Println("Server Read err: ", err)
				continue
			}

			//internal message
			if msg.MsgType == -1 {

				log.Printf("Server status: %s", s.Status())

				if msg.Status == "Connected" {

					log.Println("server sending ping: status", s.Status())
					s.Write(1, []byte("server - PING"))
				} else if msg.Status == "Closed" {
					return
				}

				//user message
			} else {

				log.Println("Server received: "+string(msg.Data)+" - Message type: ", msg.MsgType)
				s.Write(1, []byte("server - PING"))
			}
		}
	}()

	return s
}
