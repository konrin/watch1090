package main

import (
	"io"
	"os"
)

type FileScanner struct {
	file        *os.File
	dataBuff    []uint8
	dataBuffLen int
	// dataLen     int
	iqCh chan *SourceIQ
	err  error
}

func NewFileScanner(filePath string, dataLen int) *FileScanner {
	scanner := &FileScanner{
		dataBuff: make([]uint8, dataLen),
		iqCh:     make(chan *SourceIQ, 1),
		// dataLen:  dataLen,
	}

	var err error
	scanner.file, err = os.Open(filePath)
	if err != nil {
		panic(err)
	}

	return scanner
}

func (s *FileScanner) Start() error {
	for {
		dataLen, err := s.file.Read(s.dataBuff)
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		s.iqCh <- NewSourceIQ(s.dataBuff, dataLen)
	}
}

func (s *FileScanner) Close() {
	s.file.Close()
}

func (s *FileScanner) GetSourceIQCh() <-chan *SourceIQ {
	return s.iqCh
}

func (s *FileScanner) Error() error {
	return s.err
}
