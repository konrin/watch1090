package main

import (
	"log"

	"github.com/jpoirier/gortlsdr"
)

type RtlSdrScanner struct {
	device *rtlsdr.Context
	// dataBuff    []uint8
	// dataBuffLen int
	dataLen int
	dataCh  chan *SourceIQ
	err     error
}

func NewRtlSdrScanner(dataLen int) *RtlSdrScanner {
	scanner := &RtlSdrScanner{
		// dataBuff: make([]uint8, dataLen),
		dataLen: dataLen,
		dataCh:  make(chan *SourceIQ, 1),
	}

	deviceCount := rtlsdr.GetDeviceCount()
	if deviceCount == 0 {
		log.Fatalln("RTL device not found")
	}

	var err error

	scanner.device, err = rtlsdr.Open(0)
	if err != nil {
		log.Fatalln("Fail opened device: " + err.Error())
	}

	err = scanner.device.SetTunerGainMode(true)
	if err != nil {
		log.Fatalf("Error getting tuner gains\n")
	}

	scanner.device.SetAgcMode(false)
	scanner.device.SetFreqCorrection(0)
	scanner.device.SetCenterFreq(1090000000)
	scanner.device.SetSampleRate(2000000)
	scanner.device.SetTunerGainMode(false)
	scanner.device.ResetBuffer()

	gains, err := scanner.device.GetTunerGains()
	if err != nil {
		panic(err)
	}

	log.Printf("Set gain: %d\n", gains[len(gains)-1])
	scanner.device.SetTunerGain(gains[len(gains)-1])

	go func() {
		scanner.err = scanner.device.ReadAsync(scanner.readCb, nil, 12, dataLen)
	}()

	return scanner
}

func (s *RtlSdrScanner) readCb(data []byte) {
	s.dataCh <- NewSourceIQ(data, len(data))
}

func (s *RtlSdrScanner) Start() error {
	return s.device.ReadAsync(s.readCb, nil, 12, s.dataLen)
}

func (s *RtlSdrScanner) Close() {
	s.device.Close()
}

func (s *RtlSdrScanner) GetSourceIQCh() <-chan *SourceIQ {
	return s.dataCh
}

func (s *RtlSdrScanner) Error() error {
	return s.err
}
