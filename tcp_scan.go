package main

import (
	"net"
)

type TCPScanner struct {
	dataBuff []uint8
	tcpConn  net.Conn
	dataCh   chan *SourceIQ
	err      error
}

func NewTCPScanner(servAddr string, dataLen int) *TCPScanner {
	scanner := &TCPScanner{
		dataBuff: make([]uint8, dataLen),
		dataCh:   make(chan *SourceIQ, 1),
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", servAddr)
	if err != nil {
		panic("ResolveTCPAddr failed: " + err.Error())
	}

	scanner.tcpConn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		panic(err)
	}

	return scanner
}

func (s *TCPScanner) Start() error {
	for {
		dataLen, err := s.tcpConn.Read(s.dataBuff)
		if err != nil {
			return err
		}

		s.dataCh <- NewSourceIQ(s.dataBuff, dataLen)
	}
}

func (s *TCPScanner) Close() {
	s.tcpConn.Close()
}

func (s *TCPScanner) GetSourceIQCh() <-chan *SourceIQ {
	return s.dataCh
}

func (s *TCPScanner) Error() error {
	return s.err
}
