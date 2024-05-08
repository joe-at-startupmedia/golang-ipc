package ipc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
)

// 1st message sent from the server
// byte 0 = protocal VERSION no.
func (sc *Server) handshake(conn *net.Conn) error {

	err := sc.one(*conn)
	if err != nil {
		return err
	}

	err = sc.msgLength(*conn)
	if err != nil {
		return err
	}

	return nil
}

func (sc *Server) one(conn net.Conn) error {

	buff := make([]byte, 1)

	buff[0] = byte(VERSION)
	_, err := conn.Write(buff)
	if err != nil {
		return errors.New("unable to send handshake ")
	}

	recv := make([]byte, 1)
	_, err = conn.Read(recv)
	if err != nil {
		return errors.New("failed to received handshake reply")
	}

	switch result := recv[0]; result {
	case 0:
		return nil
	case 1:
		return errors.New("client has a different VERSION number")
	}

	return errors.New("other error - handshake failed")
}

func (sc *Server) msgLength(conn net.Conn) error {

	buff := make([]byte, 4)
	binary.BigEndian.PutUint32(buff, uint32(sc.maxMsgSize))

	toSend := make([]byte, 4)
	binary.BigEndian.PutUint32(toSend, uint32(len(buff)))
	toSend = append(toSend, buff...)

	_, err := conn.Write(toSend)
	if err != nil {
		return errors.New("unable to send max message length ")
	}

	reply := make([]byte, 1)

	_, err = conn.Read(reply)
	if err != nil {
		return errors.New("did not received message length reply")
	}

	return nil
}

// 1st message received by the client
func (cc *Client) handshake(conn *net.Conn) error {

	err := cc.one(*conn)
	if err != nil {
		return err
	}

	err = cc.msgLength(*conn)
	if err != nil {
		return err
	}

	return nil
}

func (cc *Client) one(conn net.Conn) error {

	recv := make([]byte, 1)
	_, err := conn.Read(recv)
	if err != nil {
		return errors.New("failed to received handshake message")
	}

	if recv[0] != VERSION {
		cc.handshakeSendReply(conn, 1)
		return errors.New("server has sent a different VERSION number")
	}

	return cc.handshakeSendReply(conn, 0)
}

func (cc *Client) msgLength(conn net.Conn) error {

	buff := make([]byte, 4)

	_, err := conn.Read(buff)
	if err != nil {
		return errors.New("failed to received max message length 1")
	}

	var msgLen uint32
	err = binary.Read(bytes.NewReader(buff), binary.BigEndian, &msgLen) // message length
	if err != nil {
		return errors.New("failed to read binary")
	}

	buff = make([]byte, int(msgLen))

	_, err = conn.Read(buff)
	if err != nil {
		return errors.New("failed to received max message length 2")
	}

	var maxMsgSize uint32
	err = binary.Read(bytes.NewReader(buff), binary.BigEndian, &maxMsgSize) // message length
	if err != nil {
		return errors.New("failed to read binary")
	}

	cc.maxMsgSize = int(maxMsgSize)
	return cc.handshakeSendReply(conn, 0)
}

func (cc *Client) handshakeSendReply(conn net.Conn, result byte) error {

	buff := make([]byte, 1)
	buff[0] = result

	_, err := conn.Write(buff)
	return err
}
