package main

import (
	"encoding/hex"
	"net"
	"sync"
)

const (
	STAR      uint8 = 0x2A
	LF        uint8 = 0x0A
	SEMICOLON uint8 = 0x3B
)

func NewNetListener(addr string, dataCh <-chan Message) *Net {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		panic("ResolveTCPAddr failed: " + err.Error())
	}

	n := &Net{
		clients: make([]net.Conn, 0, 1),
	}

	n.server, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			select {
			case msg := <-dataCh:
				mhb := make([]uint8, len(msg.Msg)*2)
				hex.Encode(mhb, msg.Msg)

				m := append([]uint8{STAR}, mhb...)
				m = append(m, SEMICOLON, LF)

				for i := 0; i < len(n.clients); i++ {
					_, err := n.clients[i].Write(m)
					if err != nil {
						n.connLost(n.clients[i])
					}
				}

				break
			}
		}
	}()

	return n
}

type Net struct {
	server *net.TCPListener
	sync.Mutex
	clients []net.Conn
}

func (n *Net) Start() error {
	for {
		conn, err := n.server.Accept()
		if err != nil {
			return err
		}

		go n.clientHandler(conn)
	}
}

func (n *Net) clientHandler(conn net.Conn) {
	n.Lock()
	n.clients = append(n.clients, conn)
	n.Unlock()

	bufZero := make([]byte, 1)

	for {
		_, err := conn.Read(bufZero)
		if err != nil {
			n.connLost(conn)
			return
		}
	}
}

func (n *Net) connLost(conn net.Conn) {
	n.Lock()
	defer n.Unlock()

	for i := 0; i < len(n.clients); i++ {
		if n.clients[i] == conn {
			n.clients = append(n.clients[0:i], n.clients[i+1:]...)
			return
		}
	}
}

func (n *Net) Close() {
	n.server.Close()
}
