// +build !cgo

package main

import (
	"bufio"
	"bytes"
	"os/exec"
)

type RtlSdrScanner struct {
	cmd        *exec.Cmd
	dataLen    int
	dataCh     chan *SourceIQ
	err        error
	outScanner *bufio.Scanner
}

func NewRtlSdrScanner(dataLen int) *RtlSdrScanner {
	scanner := &RtlSdrScanner{
		dataLen: dataLen,
		dataCh:  make(chan *SourceIQ, 1),
	}

	scanner.cmd = exec.Command("rtl_sdr",
		"-d 0",
		"-s 2000000",
		"-f 1090000000",
		"-", )

	return scanner
}

func (s *RtlSdrScanner) Start() error {
	buff := make([]byte, s.dataLen)

	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = s.cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		for {
			n, err := stdout.Read(buff)
			if n > 0 {
				data := bytes.NewBuffer(buff)

				s.dataCh <- NewSourceIQ(data.Bytes(), data.Len())
			}

			if err != nil {
				break
			}
		}
	}()

	return s.cmd.Wait()
}

func (s *RtlSdrScanner) Close() {
	s.cmd.Process.Kill()
}

func (s *RtlSdrScanner) GetSourceIQCh() <-chan *SourceIQ {
	return s.dataCh
}

func (s *RtlSdrScanner) Error() error {
	return s.err
}
