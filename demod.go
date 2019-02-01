package main

import (
	"math"
	"time"
)

const (
	// 256kb
	DataLen = 16 * 16384
	// microseconds
	PreambleUs   = 8
	LongMsgBits  = 112
	ShortMsgBits = 56
)

var (
	magLutTable = make([]uint16, 129*129*2)
	FullLen     = PreambleUs + LongMsgBits
	DataLenComp = DataLen + (FullLen-1)*4

	modesChecksumTable = [...]uint32{
		0x3935ea, 0x1c9af5, 0xf1b77e, 0x78dbbf, 0xc397db, 0x9e31e9, 0xb0e2f0, 0x587178,
		0x2c38bc, 0x161c5e, 0x0b0e2f, 0xfa7d13, 0x82c48d, 0xbe9842, 0x5f4c21, 0xd05c14,
		0x682e0a, 0x341705, 0xe5f186, 0x72f8c3, 0xc68665, 0x9cb936, 0x4e5c9b, 0xd8d449,
		0x939020, 0x49c810, 0x24e408, 0x127204, 0x093902, 0x049c81, 0xfdb444, 0x7eda22,
		0x3f6d11, 0xe04c8c, 0x702646, 0x381323, 0xe3f395, 0x8e03ce, 0x4701e7, 0xdc7af7,
		0x91c77f, 0xb719bb, 0xa476d9, 0xadc168, 0x56e0b4, 0x2b705a, 0x15b82d, 0xf52612,
		0x7a9309, 0xc2b380, 0x6159c0, 0x30ace0, 0x185670, 0x0c2b38, 0x06159c, 0x030ace,
		0x018567, 0xff38b7, 0x80665f, 0xbfc92b, 0xa01e91, 0xaff54c, 0x57faa6, 0x2bfd53,
		0xea04ad, 0x8af852, 0x457c29, 0xdd4410, 0x6ea208, 0x375104, 0x1ba882, 0x0dd441,
		0xf91024, 0x7c8812, 0x3e4409, 0xe0d800, 0x706c00, 0x383600, 0x1c1b00, 0x0e0d80,
		0x0706c0, 0x038360, 0x01c1b0, 0x00e0d8, 0x00706c, 0x003836, 0x001c1b, 0xfff409,
		0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000,
		0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000,
		0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000, 0x000000,
	}
)

func init() {
	magLutCalc()
}

type (
	SourceIQ struct {
		Data        []uint8
		DataLen     int
		ReceiptTime time.Time
		mag         []uint16
	}

	Demod struct {
		MessageCh chan Message
		icaoCache map[uint]time.Time
	}
)

func NewDemod() *Demod {
	return &Demod{
		MessageCh: make(chan Message, 1),
		icaoCache: make(map[uint]time.Time),
	}
}

func NewSourceIQ(data []uint8, dataLen int) *SourceIQ {
	return &SourceIQ{
		Data:        data,
		DataLen:     dataLen,
		ReceiptTime: time.Now(),
	}
}

func (d *Demod) DetectModeS(chunk *SourceIQ) {
	d.computeMagnitudeVector(chunk)

	var (
		bits          = make([]uint8, LongMsgBits)
		msg           = make([]uint8, LongMsgBits/2)
		aux           = make([]uint16, LongMsgBits*2)
		useCorrection bool
	)

	// main each
	for j := 0; j < len(chunk.mag)-(FullLen*2); j++ {
		var high, delta, low, errors int
		goodMessage := false

		if useCorrection {
			for i := j + PreambleUs*2; i < len(aux); i++ {
				aux[i] = chunk.mag[i]
			}

			if j > 0 && detectOutOfPhase(chunk.mag, j) > 0 {
				applyPhaseCorrection(chunk.mag, j)
			}
		} else {
			if !(chunk.mag[j] > chunk.mag[j+1] &&
				chunk.mag[j+1] < chunk.mag[j+2] &&
				chunk.mag[j+2] > chunk.mag[j+3] &&
				chunk.mag[j+3] < chunk.mag[j] &&
				chunk.mag[j+4] < chunk.mag[j] &&
				chunk.mag[j+5] < chunk.mag[j] &&
				chunk.mag[j+6] < chunk.mag[j] &&
				chunk.mag[j+7] > chunk.mag[j+8] &&
				chunk.mag[j+8] < chunk.mag[j+9] &&
				chunk.mag[j+9] > chunk.mag[j+6]) {
				continue
			}

			high = int((chunk.mag[j] + chunk.mag[j+2] + chunk.mag[j+7] + chunk.mag[j+9]) / 6)
			if int(chunk.mag[j+4]) >= high ||
				int(chunk.mag[j+5]) >= high {
				continue
			}

			if int(chunk.mag[j+11]) >= high ||
				int(chunk.mag[j+12]) >= high ||
				int(chunk.mag[j+13]) >= high ||
				int(chunk.mag[j+14]) >= high {
				continue
			}
		}

		for i := 0; i < LongMsgBits*2; i += 2 {
			low = int(chunk.mag[j+i+PreambleUs*2])
			high = int(chunk.mag[j+i+PreambleUs*2+1])
			delta = int(low - high)
			if delta < 0 {
				delta = -delta
			}

			if i > 0 && delta < 256 {
				bits[i/2] = bits[i/2-1]
			} else if low == high {
				bits[i/2] = 2 // error
				if i < ShortMsgBits*2 {
					errors++
				}
			} else if low > high {
				bits[i/2] = 1
			} else {
				// (low < high) for exclusion
				bits[i/2] = 0
			}
		}

		if useCorrection {
			for i := 0; i < len(aux); i++ {
				chunk.mag[j+PreambleUs*2] = aux[i]
			}
		}

		for i := 0; i < LongMsgBits; i += 8 {
			msg[i/8] =
				bits[i]<<7 |
					bits[i+1]<<6 |
					bits[i+2]<<5 |
					bits[i+3]<<4 |
					bits[i+4]<<3 |
					bits[i+5]<<2 |
					bits[i+6]<<1 |
					bits[i+7]
		}

		var (
			msgType = msg[0] >> 3
			msgLen  = msgLenBits(msgType) / 8
		)

		delta = 0
		for i := 0; i < msgLen*8*2; i += 2 {
			delta += int(
				math.Abs(float64(int16(chunk.mag[j+i+PreambleUs*2]) -
					int16(chunk.mag[j+i+PreambleUs*2+1]))),
			)
		}
		delta /= int(msgLen * 4)

		if delta < (10 * 255) {
			useCorrection = false
			continue
		}

		if errors == 0 {
			mmsg := make([]uint8, msgLen)
			for i := 0; i < msgLen; i++ {
				mmsg[i] = msg[i]
			}

			crc := CRC(mmsg, msgLen)
			sum := modesChecksum(mmsg, msgLen*8)

			var crcOK = crc == sum

			var errorBit int

			if !crcOK && isADSB(msgType) {
				if errorBit = fixSingleBitErrors(mmsg, msgLen*8); errorBit != -1 {
					crcOK = true
					crc = modesChecksum(mmsg, int(msgLen*8))
				}

				if errorBit == -1 {
					if errorBit = fixTwoBitsErrors(mmsg, msgLen*8); errorBit != -1 {
						crcOK = true
						crc = modesChecksum(mmsg, int(msgLen*8))
					}
				}
			}

			var icao []uint8

			if crcOK && isADSB(msgType) {
				icao = []uint8{mmsg[1], mmsg[2], mmsg[3]}
			}

			// DF
			if !crcOK && isDownlinkRequest(msgType) {
				if ok, _icao := d.bruteForceAp(msg, int(msgLen*8)); ok {
					crcOK = true
					icao = _icao
				}

			}

			if crcOK {
				j += (PreambleUs + (msgLen * 8)) * 2
				goodMessage = true

				if isADSB(msgType) {
					icaoMask := (uint(mmsg[1]) << 16) | (uint(mmsg[2]) << 8) | uint(mmsg[3])
					d.addICAOForCache(icaoMask)
				}

				d.MessageCh <- Message{
					ReceiptTime: chunk.ReceiptTime,
					Msg:         mmsg,
					DF:          msgType,
					ICAO:        icao,
				}
			}
		}

		if !goodMessage && !useCorrection {
			j--
			useCorrection = true
		} else {
			useCorrection = false
		}
	}
}

func (d *Demod) addICAOForCache(icao uint) {
	d.icaoCache[icao] = time.Now()
}

func (d *Demod) hasICAOFromCache(icao uint) bool {
	t, ok := d.icaoCache[icao]
	if !ok {
		return false
	}

	if time.Since(t).Seconds() > 60 {
		delete(d.icaoCache, icao)

		return false
	}

	return true
}

func (d *Demod) DetectModeAC() {
	//TODO
}

func (d *Demod) computeMagnitudeVector(chunk *SourceIQ) {
	chunk.mag = make([]uint16, chunk.DataLen/2)

	for j := 0; j < chunk.DataLen; j += 2 {
		var i = int(chunk.Data[j]) - 127
		var q = int(chunk.Data[j+1]) - 127

		if i < 0 {
			i = -i
		}

		if q < 0 {
			q = -q
		}

		chunk.mag[j/2] = magLutTable[uint16((i*129)+q)]
	}
}

func isADSB(t uint8) bool {
	switch t {
	case 11, 17:
		return true
	}

	return false
}

func isDownlinkRequest(t uint8) bool {
	switch t {
	case 0, 4, 5, 16, 20, 21, 24:
		return true
	}

	return false
}

func CRC(msg []uint8, msgLen int) uint32 {
	return (uint32(msg[msgLen-3]) << 16) | (uint32(msg[msgLen-2]) << 8) | uint32(msg[msgLen-1])
}

func (d *Demod) bruteForceAp(msg []uint8, bits int) (bool, []uint8) {
	aux := make([]uint8, LongMsgBits/8)
	lastbyte := (bits / 8) - 1

	for i := 0; i < (bits / 8); i++ {
		aux[i] = msg[i]
	}

	crc := modesChecksum(aux, bits)
	aux[lastbyte] ^= uint8(crc & 0xff)
	aux[lastbyte-1] ^= uint8((crc >> 8) & 0xff)
	aux[lastbyte-2] ^= uint8((crc >> 16) & 0xff)

	addr := uint(aux[lastbyte]) | (uint(aux[lastbyte-1]) << 8) | (uint(aux[lastbyte-2]) << 16)
	if d.hasICAOFromCache(addr) {
		return true, []uint8{aux[lastbyte-2], aux[lastbyte-1], aux[lastbyte]}
	}

	return false, nil
}

func fixSingleBitErrors(msg []uint8, bits int) int {
	aux := make([]uint8, LongMsgBits/8)

	for j := 0; j < bits; j++ {
		b := (j / 8) >> 0
		bitmask := 1 << uint((7 - (j % 8)))

		for i := 0; i < (bits / 8); i++ {
			aux[i] = msg[i]
		}

		aux[b] ^= uint8(bitmask)

		crc := CRC(aux, bits/8)
		crc2 := modesChecksum(aux, bits)

		if crc == crc2 {
			// rollback
			for i := 0; i < (bits / 8); i++ {
				msg[i] = aux[i]
			}

			return j
		}
	}

	return -1
}

func fixTwoBitsErrors(msg []uint8, bits int) int {
	aux := make([]uint8, LongMsgBits/8)

	for j := 0; j < bits; j++ {
		b1 := (j / 8) >> 0
		bitmask1 := 1 << uint((7 - (j % 8)))

		for i := j + 1; i < bits; i++ {
			b2 := (i / 8) >> 0
			bitmask2 := 1 << uint((7 - (i % 8)))

			for i := 0; i < (bits / 8); i++ {
				aux[i] = msg[i]
			}

			aux[b1] ^= uint8(bitmask1) // Flip j-th bit.
			aux[b2] ^= uint8(bitmask2) // Flip i-th bit.

			crc1 := CRC(aux, bits/8)
			crc2 := modesChecksum(aux, bits)

			if crc1 == crc2 {
				for i := 0; i < (bits / 8); i++ {
					msg[i] = aux[i]
				}

				return j | i<<8
			}
		}
	}

	return -1
}

func msgLenBits(t uint8) int {
	switch t {
	case 0, 4, 5, 16, 17, 20, 21, 24:
		return LongMsgBits
	}

	return ShortMsgBits
}

func modesChecksum(msg []uint8, bits int) uint32 {
	var crc uint32

	offset := 112 - 56
	if bits == 112 {
		offset = 0
	}

	for j := 0; j < bits; j++ {
		var bit int
		b := j / 8
		bit = j % 8
		bitmask := 1 << uint(7-bit)

		if (int(msg[b]) & bitmask) > 0 {
			crc ^= modesChecksumTable[j+offset]
		}
	}

	return crc
}

func detectOutOfPhase(mag []uint16, offset int) int {
	if mag[offset+3] > mag[offset+2]/3 {
		return 1
	}
	if mag[offset+10] > mag[offset+9]/3 {
		return 1
	}
	if mag[offset+6] > mag[offset+7]/3 {
		return -1
	}
	if mag[offset+-1] > mag[offset+1]/3 {
		return -1
	}
	return 0
}

func applyPhaseCorrection(mag []uint16, offset int) {
	// Move ahead 16 to skip preamble.
	for j := 16; j < (LongMsgBits-1)*2; j += 2 {
		if mag[offset+j] > mag[offset+j+1] {
			// One
			mag[offset+j+2] = (mag[offset+j+2] * 5) / 4
		} else {
			// Zero
			mag[offset+j+2] = (mag[offset+j+2] * 4) / 5
		}
	}
}

func magLutCalc() {
	for i := 0; i < 128; i++ {
		for q := 0; q < 128; q++ {
			magLutTable[i*129+q] = uint16(math.Round(math.Sqrt(float64(i*i+q*q)) * 360))
		}
	}
}
