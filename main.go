package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"time"
)

var (
	fileFlag      string
	tcpIQServer   string
	tcpNetMsgAddr = "localhost:30001"
	isExit        bool

	// 254kb
	dataBuffLen = 16 * 16384
)

func main() {
	flag.StringVar(&fileFlag, "ifile", fileFlag, "")
	flag.StringVar(&tcpIQServer, "tcp_iq_server", tcpIQServer, "")
	flag.StringVar(&tcpNetMsgAddr, "net_msg_addr", tcpNetMsgAddr, "")
	flag.IntVar(&dataBuffLen, "iq_buff_len", dataBuffLen, "")

	flag.Parse()

	var scanner Scanner

	switch true {
	case len(fileFlag) > 0:
		scanner = NewFileScanner(fileFlag, dataBuffLen)
		break
	case len(tcpIQServer) > 0:
		scanner = NewTCPScanner(tcpIQServer, dataBuffLen)
		break
	default:
		scanner = NewRtlSdrScanner(dataBuffLen)
	}

	defer scanner.Close()

	demod := NewDemod()

	var msgChs = make([]chan Message, 0)

	var netListener *Net

	if len(tcpNetMsgAddr) > 0 {
		netMsgCh := make(chan Message, 1)
		netListener = NewNetListener(tcpNetMsgAddr, netMsgCh)
		msgChs = append(msgChs, netMsgCh)

		go netListener.Start()
	}

	go func(demod *Demod) {
		lastTime := time.Now()

		for {
			select {
			case msg := <-demod.MessageCh:
				fmt.Printf(
					"%q,\n",
					//"%d, %s, %s +%s\n",
					//msg.DF,
					//hex.EncodeToString(msg.ICAO),
					hex.EncodeToString(msg.Msg),
					//lastTime.Sub(msg.ReceiptTime).String(),
					//msg.ReceiptTime.Sub(lastTime).String(),
				)
				_ = lastTime

				lastTime = msg.ReceiptTime

				for i := 0; i < len(msgChs); i++ {
					msgChs[i] <- msg
				}
			}
		}
	}(demod)

	go func() {
		ch := scanner.GetSourceIQCh()

		for {
			select {
			case iq := <-ch:
				demod.DetectModeS(iq)
			}
		}
	}()

	err := scanner.Start()
	if err != nil {
		log.Println("Scanner err: " + err.Error())
	}

	if err := scanner.Error(); err != nil {
		log.Println("Scanner err: " + err.Error())
	}

	if netListener != nil {
		netListener.Close()
	}
}

type Scanner interface {
	Start() error
	Close()
	GetSourceIQCh() <-chan *SourceIQ
	Error() error
}
