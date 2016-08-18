package hca

import "github.com/vazrupe/endibuf"

type Hca struct {
	CiphKey1 uint32
	CiphKey2 uint32

	Mode int
	Loop int

	Volume float32

	version    uint32
	dataOffset uint32

	channelCount uint32
	samplingRate uint32
	blockCount   uint32
	fmtR01       uint32
	fmtR02       uint32

	blockSize uint32
	compR01   uint32
	compR02   uint32
	compR03   uint32
	compR04   uint32
	compR05   uint32
	compR06   uint32
	compR07   uint32
	compR08   uint32
	compR09   uint32

	vbrR01 uint32
	vbrR02 uint32

	athType uint32

	loopStart uint32
	loopEnd   uint32
	loopR01   uint32
	loopR02   uint32
	loopFlg   bool

	ciphType uint32

	rvaVolume float32

	commLen     uint32
	commComment string

	ath     stATH
	cipher  *Cipher
	channel []*stChannel

	saver func(f float32, w *endibuf.Writer)
}

// Modes is writting mode num
const (
	ModeFloat = 0
	Mode8Bit  = 8
	Mode16Bit = 16
	Mode24Bit = 24
	Mode32Bit = 32
)

// NewDecoder is create hca with default option
func NewDecoder() *Hca {
	return &Hca{CiphKey1: 0x30DBE1AB,
		CiphKey2: 0xCC554639,
		Mode:     16,
		Loop:     0,
		Volume:   1.0,
		cipher:   NewCipher()}
}
