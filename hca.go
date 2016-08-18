package hca

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"

	"github.com/vazrupe/endibuf"
)

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

	ath     ATH
	cipher  *Cipher
	channel [0x10]*stChannel
}

// NewDecoder is create hca with default option
func NewDecoder() *Hca {
	return &Hca{CiphKey1: 0x30DBE1AB,
		CiphKey2: 0xCC554639,
		Mode:     16,
		Loop:     0,
		Volume:   1.0,
		cipher:   NewCipher()}
}

func (h *Hca) DecodeFromBytes(data []byte) (decoded []byte, ok bool) {
	switch h.Mode {
	case 0, 8, 16, 24, 32:
		if h.Loop >= 0 {
			break
		}
		return []byte{}, false
	default:
		return []byte{}, false
	}

	base := bytes.NewReader(data)
	buf := io.NewSectionReader(base, 0, base.Size())
	r := endibuf.NewReader(buf)
	r.Endian = binary.BigEndian

	// read header
	var header stHeader
	header.hca, _ = r.ReadUint32()
	header.version, _ = r.ReadUint16()
	r.Endian = binary.BigEndian
	header.dataOffset, _ = r.ReadUint16()

	size := uint32(header.dataOffset)
	r.Seek(0, 0)
	data1, _ := r.ReadBytes(int(size))

	if !h.decode(data1, uint32(header.dataOffset), 0) {
		return []byte{}, false
	}

	wavRiff := NewWaveHeader()
	wavSmpl := NewWaveSmpl()
	wavNote := NewWaveNote()
	wavData := NewWaveData()

	if h.Mode > 0 {
		wavRiff.fmtType = 1
		wavRiff.fmtBitCount = uint16(h.Mode)
	} else {
		wavRiff.fmtType = 3
		wavRiff.fmtBitCount = 32
	}
	wavRiff.fmtChannelCount = uint16(h.channelCount)
	wavRiff.fmtSamplingRate = h.samplingRate
	wavRiff.fmtSamplingSize = wavRiff.fmtBitCount / 8 * wavRiff.fmtChannelCount
	wavRiff.fmtSamplesPerSec = wavRiff.fmtSamplingRate * uint32(wavRiff.fmtSamplingSize)

	if h.loopFlg {
		wavSmpl.samplePeriod = uint32(1 / float64(wavRiff.fmtSamplingRate) * 1000000000)
		wavSmpl.loopStart = h.loopStart * 0x80 * 8 * uint32(wavRiff.fmtSamplingSize)
		wavSmpl.loopEnd = h.loopEnd * 0x80 * 8 * uint32(wavRiff.fmtSamplingSize)
		if h.loopR01 == 0x80 {
			wavSmpl.loopPlayCount = 0
		} else {
			wavSmpl.loopPlayCount = h.loopR01
		}
	} else if h.Loop != 0 {
		wavSmpl.loopStart = 0
		wavSmpl.loopEnd = h.blockCount * 0x80 * 8 * uint32(wavRiff.fmtSamplingSize)
		h.loopStart = 0
		h.loopEnd = h.blockCount
	}
	if h.commLen > 0 {
		wavNote.noteSize = 4 + h.commLen + 1
		if (wavNote.noteSize & 3) != 0 {
			wavNote.noteSize += 4 - (wavNote.noteSize & 3)
		}
	}
	wavData.dataSize = h.blockCount*0x80*8*uint32(wavRiff.fmtSamplingSize) + (wavSmpl.loopEnd-wavSmpl.loopStart)*uint32(h.Loop)
	wavRiff.riffSize = 0x1C + 8 + wavData.dataSize
	if h.loopFlg && h.Loop == 0 {
		// wavSmpl Size
		wavRiff.riffSize += 17 * 4
	}
	if h.commLen > 0 {
		wavRiff.riffSize += 8 + wavNote.noteSize
	}

	tempfile, _ := ioutil.TempFile("", "hca_wav_temp_")
	defer os.Remove(tempfile.Name())
	w := endibuf.NewWriter(tempfile)
	w.Endian = binary.LittleEndian
	wavRiff.Write(w)
	if h.loopFlg && h.Loop == 0 {
		wavSmpl.Write(w)
	}
	if h.commLen > 0 {
		wavNote.Write(w, h.commComment)
	}
	wavData.Write(w)

	// adjust the relative volume
	h.rvaVolume *= h.Volume

	// decode
	if h.Loop == 0 {
		if !h.decodeFromBytesDecode(r, w, h.dataOffset, h.blockCount) {
			return []byte{}, false
		}
	} else {
		loopBlockOffset := h.dataOffset + h.loopStart*h.blockSize
		loopBlockCount := h.loopEnd - h.loopStart
		if !h.decodeFromBytesDecode(r, w, h.dataOffset, h.loopEnd) {
			return []byte{}, false
		}
		for i := 1; i < h.Loop; i++ {
			if !h.decodeFromBytesDecode(r, w, loopBlockOffset, loopBlockCount) {
				return []byte{}, false
			}
		}
		if !h.decodeFromBytesDecode(r, w, loopBlockOffset, h.blockCount-h.loopStart) {
			return []byte{}, false
		}
	}
	tempfile.Seek(0, 0)
	res, _ := ioutil.ReadAll(tempfile)

	return res, true
}

func (h *Hca) decodeFromBytesDecode(r *endibuf.Reader, w *endibuf.Writer, address, count uint32) bool {
	var f float32
	r.Seek(int64(address), 0)
	for l := uint32(0); l < count; l++ {
		data, _ := r.ReadBytes(int(h.blockSize))
		if !h.decode(data, h.blockSize, address) {
			return false
		}
		for i := 0; i < 8; i++ {
			for j := 0; j < 0x80; j++ {
				for k := uint32(0); k < h.channelCount; k++ {
					f = h.channel[k].wave[i][j] * h.rvaVolume
					if f > 1 {
						f = 1
					} else if f < -1 {
						f = -1
					}
					h.modeFunction(f, w)
				}
			}
		}

		address += h.blockSize
	}
	return true
}

func (h *Hca) modeFunction(f float32, w *endibuf.Writer) {
	switch h.Mode {
	case 0:
		w.WriteFloat32(f)
	case 8:
		v := int(f*0x7F) + 0x80
		w.WriteInt8(int8(v))
	case 16:
		v := int16(f * 0x7FFF)
		w.WriteInt16(v)
	case 24:
		v := int32(f * 0x7FFFFF)
		b := make([]byte, 3)
		if w.Endian == binary.LittleEndian {
			b[2] = byte((v & 0xFF0000) >> 16)
			b[1] = byte((v & 0xFF00) >> 8)
			b[0] = byte((v & 0xFF))
		} else {
			b[0] = byte((v & 0xFF0000) >> 16)
			b[1] = byte((v & 0xFF00) >> 8)
			b[2] = byte((v & 0xFF))
		}
		w.WriteBytes(b)
	case 32:
		v := int32(f * 0x7FFFFFFF)
		w.WriteInt32(v)
	}
}

func (h *Hca) decode(data []byte, size, address uint32) bool {
	if len(data) == 0 {
		return false
	}
	base := bytes.NewReader(data)
	buf := io.NewSectionReader(base, 0, base.Size())
	r := endibuf.NewReader(buf)
	r.Endian = binary.LittleEndian

	if address == 0 {
		// header
		if !h.loadHeader(r, size) {
			return false
		}

		// initialize
		if !h.ath.Init(int(h.athType), h.samplingRate) {
			return false
		}
		h.cipher = NewCipher()
		if !h.cipher.Init(int(h.ciphType), h.CiphKey1, h.CiphKey2) {
			return false
		}

		// value check(In order to avoid errors caused by modification mistake of header)
		if h.compR03 == 0 {
			h.compR03 = 1
		}

		// decode preparation
		for i := range h.channel {
			h.channel[i] = NewChannel()
		}
		if !(h.compR01 == 1 && h.compR02 == 15) {
			return false
		}
		h.compR09 = ceil2(h.compR05-(h.compR06+h.compR07), h.compR08)
		tmp := make([]byte, 0x10)
		for i := range tmp {
			tmp[i] = 0
		}
		b := h.channelCount / h.compR03
		if h.compR07 != 0 && b > 1 {
			for i := uint32(0); i < h.compR03; i++ {
				switch b {
				case 2, 3:
					tmp[b*i+0] = 1
					tmp[b*i+1] = 2
				case 4:
					tmp[b*i+0] = 1
					tmp[b*i+1] = 2
					if h.compR04 == 0 {
						tmp[b*i+2] = 1
						tmp[b*i+3] = 2
					}
				case 5:
					tmp[b*i+0] = 1
					tmp[b*i+1] = 2
					if h.compR04 <= 2 {
						tmp[b*i+3] = 1
						tmp[b*i+4] = 2
					}
				case 6, 7:
					tmp[b*i+0] = 1
					tmp[b*i+1] = 2
					tmp[b*i+4] = 1
					tmp[b*i+5] = 2
				case 8:
					tmp[b*i+0] = 1
					tmp[b*i+1] = 2
					tmp[b*i+4] = 1
					tmp[b*i+5] = 2
					tmp[b*i+6] = 1
					tmp[b*i+7] = 2
				}
			}
		}
		for i := uint32(0); i < h.channelCount; i++ {
			h.channel[i].t = int(tmp[i])
			h.channel[i].valueIndex = h.compR06 + h.compR07
			h.channel[i].count = h.compR06
			if tmp[i] != 2 {
				h.channel[i].count += h.compR07
			}
		}
	} else if address >= h.dataOffset {
		// block data
		if size < h.blockSize {
			return false
		}
		if checkSum(data, 0) != 0 {
			return false
		}
		mask := h.cipher.Mask(data)
		d := &clData{}
		d.Init(mask, int(h.blockSize))
		magic := d.GetBit(16)
		if magic == 0xFFFF {
			a := (d.GetBit(9) << 8) - d.GetBit(7)
			for i := uint32(0); i < h.channelCount; i++ {
				h.channel[i].Decode1(d, h.compR09, a, h.ath.GetTable())
			}
			for i := 0; i < 8; i++ {
				for j := uint32(0); j < h.channelCount; j++ {
					h.channel[j].Decode2(d)
				}
				for j := uint32(0); j < h.channelCount; j++ {
					h.channel[j].Decode3(h.compR09, h.compR08, h.compR07+h.compR06, h.compR05)
				}
				for j := uint32(0); j < (h.channelCount - 1); j++ {
					h.channel[j].Decode4(h.channel[j+1], i, h.compR05-h.compR06, h.compR06, h.compR07)
				}
				for j := uint32(0); j < h.channelCount; j++ {
					h.channel[j].Decode5(i)
				}
			}
		}
	}
	return true
}

func checkSum(data []byte, sum uint16) uint16 {
	res := sum
	v := []uint16{
		0x0000, 0x8005, 0x800F, 0x000A, 0x801B, 0x001E, 0x0014, 0x8011, 0x8033, 0x0036, 0x003C, 0x8039, 0x0028, 0x802D, 0x8027, 0x0022,
		0x8063, 0x0066, 0x006C, 0x8069, 0x0078, 0x807D, 0x8077, 0x0072, 0x0050, 0x8055, 0x805F, 0x005A, 0x804B, 0x004E, 0x0044, 0x8041,
		0x80C3, 0x00C6, 0x00CC, 0x80C9, 0x00D8, 0x80DD, 0x80D7, 0x00D2, 0x00F0, 0x80F5, 0x80FF, 0x00FA, 0x80EB, 0x00EE, 0x00E4, 0x80E1,
		0x00A0, 0x80A5, 0x80AF, 0x00AA, 0x80BB, 0x00BE, 0x00B4, 0x80B1, 0x8093, 0x0096, 0x009C, 0x8099, 0x0088, 0x808D, 0x8087, 0x0082,
		0x8183, 0x0186, 0x018C, 0x8189, 0x0198, 0x819D, 0x8197, 0x0192, 0x01B0, 0x81B5, 0x81BF, 0x01BA, 0x81AB, 0x01AE, 0x01A4, 0x81A1,
		0x01E0, 0x81E5, 0x81EF, 0x01EA, 0x81FB, 0x01FE, 0x01F4, 0x81F1, 0x81D3, 0x01D6, 0x01DC, 0x81D9, 0x01C8, 0x81CD, 0x81C7, 0x01C2,
		0x0140, 0x8145, 0x814F, 0x014A, 0x815B, 0x015E, 0x0154, 0x8151, 0x8173, 0x0176, 0x017C, 0x8179, 0x0168, 0x816D, 0x8167, 0x0162,
		0x8123, 0x0126, 0x012C, 0x8129, 0x0138, 0x813D, 0x8137, 0x0132, 0x0110, 0x8115, 0x811F, 0x011A, 0x810B, 0x010E, 0x0104, 0x8101,
		0x8303, 0x0306, 0x030C, 0x8309, 0x0318, 0x831D, 0x8317, 0x0312, 0x0330, 0x8335, 0x833F, 0x033A, 0x832B, 0x032E, 0x0324, 0x8321,
		0x0360, 0x8365, 0x836F, 0x036A, 0x837B, 0x037E, 0x0374, 0x8371, 0x8353, 0x0356, 0x035C, 0x8359, 0x0348, 0x834D, 0x8347, 0x0342,
		0x03C0, 0x83C5, 0x83CF, 0x03CA, 0x83DB, 0x03DE, 0x03D4, 0x83D1, 0x83F3, 0x03F6, 0x03FC, 0x83F9, 0x03E8, 0x83ED, 0x83E7, 0x03E2,
		0x83A3, 0x03A6, 0x03AC, 0x83A9, 0x03B8, 0x83BD, 0x83B7, 0x03B2, 0x0390, 0x8395, 0x839F, 0x039A, 0x838B, 0x038E, 0x0384, 0x8381,
		0x0280, 0x8285, 0x828F, 0x028A, 0x829B, 0x029E, 0x0294, 0x8291, 0x82B3, 0x02B6, 0x02BC, 0x82B9, 0x02A8, 0x82AD, 0x82A7, 0x02A2,
		0x82E3, 0x02E6, 0x02EC, 0x82E9, 0x02F8, 0x82FD, 0x82F7, 0x02F2, 0x02D0, 0x82D5, 0x82DF, 0x02DA, 0x82CB, 0x02CE, 0x02C4, 0x82C1,
		0x8243, 0x0246, 0x024C, 0x8249, 0x0258, 0x825D, 0x8257, 0x0252, 0x0270, 0x8275, 0x827F, 0x027A, 0x826B, 0x026E, 0x0264, 0x8261,
		0x0220, 0x8225, 0x822F, 0x022A, 0x823B, 0x023E, 0x0234, 0x8231, 0x8213, 0x0216, 0x021C, 0x8219, 0x0208, 0x820D, 0x8207, 0x0202,
	}
	for i := 0; i < len(data); i++ {
		res = (res << 8) ^ v[byte(res>>8)^data[i]]
	}
	return res
}

const (
	sigMask = 0x7F7F7F7F
	sigHCA  = 0x48434100
	sigFMT  = 0x666D7400
	sigCOMP = 0x636F6D70
	sigDEC  = 0x64656300
	sigVBR  = 0x76627200
	sigATH  = 0x61746800
	sigLOOP = 0x6C6F6F70
	sigCIPH = 0x63697068
	sigRVA  = 0x72766100
	sigCOMM = 0x636F6D6D
)

func (h *Hca) loadHeader(r *endibuf.Reader, size uint32) bool {
	endianSave := r.Endian
	r.Endian = binary.BigEndian

	var sig uint32

	// size check
	if size < 8 {
		return false
	}

	// HCA
	r.ReadData(&sig)
	if sig&sigMask == sigHCA {
		if !h.hcaHeaderRead(r) {
			return false
		}
		if size < h.dataOffset {
			return false
		}

		r.ReadData(&sig)
	} else {
		return false
	}

	// fmt
	if sig&sigMask == sigFMT {
		if !h.fmtHeaderRead(r) {
			return false
		}
		r.ReadData(&sig)
	} else {
		return false
	}

	if sig&sigMask == sigCOMP {
		// comp
		if !h.compHeaderRead(r) {
			return false
		}
		r.ReadData(&sig)
	} else if sig&sigMask == sigDEC {
		// dec
		if !h.decHeaderRead(r) {
			return false
		}
		r.ReadData(&sig)
	} else {
		return false
	}

	// vbr
	if sig&sigMask == sigVBR {
		if !h.vbrHeaderRead(r) {
			return false
		}
		r.ReadData(&sig)
	} else {
		h.vbrR01 = 0
		h.vbrR02 = 0
	}

	// ath
	if sig&sigMask == sigATH {
		if !h.athHeaderRead(r) {
			return false
		}
		r.ReadData(&sig)
	} else {
		if h.version < 0x200 {
			h.athType = 1
		} else {
			h.athType = 0
		}
	}

	// loop
	if sig&sigMask == sigLOOP {
		if !h.loopHeaderRead(r) {
			return false
		}
		r.ReadData(&sig)
	} else {
		h.loopStart = 0
		h.loopEnd = 0
		h.loopR01 = 0
		h.loopR02 = 0x400
		h.loopFlg = false
	}

	// ciph
	if sig&sigMask == sigCIPH {
		if !h.ciphHeaderRead(r) {
			return false
		}
		r.ReadData(&sig)
	} else {
		h.ciphType = 0
	}

	// rva
	if sig&sigMask == sigRVA {
		if !h.rvaHeaderRead(r) {
			return false
		}
		r.ReadData(&sig)
	} else {
		h.rvaVolume = 1
	}

	// comm
	if sig&sigMask == sigCOMM {
		if !h.commHeaderRead(r) {
			return false
		}
	} else {
		h.commLen = 0
		h.commComment = ""
	}
	r.Endian = endianSave
	return true
}

func (h *Hca) hcaHeaderRead(r *endibuf.Reader) bool {
	version, _ := r.ReadUint16()
	dataOffset, _ := r.ReadUint16()
	h.version = uint32(version)
	h.dataOffset = uint32(dataOffset)
	return true
}

func (h *Hca) fmtHeaderRead(r *endibuf.Reader) bool {
	ui, _ := r.ReadUint32()
	h.channelCount = (ui & 0xFF000000) >> 24
	h.samplingRate = ui & 0x00FFFFFF
	h.blockCount, _ = r.ReadUint32()
	fmtR01, _ := r.ReadUint16()
	fmtR02, _ := r.ReadUint16()
	h.fmtR01 = uint32(fmtR01)
	h.fmtR02 = uint32(fmtR02)
	if !(h.channelCount >= 1 && h.channelCount <= 16) {
		return false
	}
	if !(h.samplingRate >= 1 && h.samplingRate <= 0x7FFFFF) {
		return false
	}
	return true
}

func (h *Hca) compHeaderRead(r *endibuf.Reader) bool {
	blockSize, _ := r.ReadUint16()
	h.blockSize = uint32(blockSize)
	datas, _ := r.ReadBytes(10)
	h.compR01 = uint32(datas[0])
	h.compR02 = uint32(datas[1])
	h.compR03 = uint32(datas[2])
	h.compR04 = uint32(datas[3])
	h.compR05 = uint32(datas[4])
	h.compR06 = uint32(datas[5])
	h.compR07 = uint32(datas[6])
	h.compR08 = uint32(datas[7])
	if !((h.blockSize >= 8 && h.blockSize <= 0xFFFF) || (h.blockSize == 0)) {
		return false
	}
	if !(h.compR01 >= 0 && h.compR01 <= h.compR02 && h.compR02 <= 0x1F) {
		return false
	}
	return true
}

func (h *Hca) decHeaderRead(r *endibuf.Reader) bool {
	blockSize, _ := r.ReadUint16()
	h.blockSize = uint32(blockSize)
	datas, _ := r.ReadBytes(6)
	h.compR01 = uint32(datas[0])
	h.compR02 = uint32(datas[1])
	h.compR03 = uint32(datas[4] & 0xF)
	h.compR04 = uint32(datas[4] >> 4)
	h.compR05 = uint32(datas[2]) + 1
	if datas[5] > 0 {
		h.compR06 = uint32(datas[3]) + 1
	} else {
		h.compR06 = uint32(datas[2]) + 1
	}
	h.compR07 = h.compR05 - h.compR06
	h.compR08 = 0
	if !((h.blockSize >= 8 && h.blockSize <= 0xFFFF) || h.blockSize == 0) {
		return false
	}
	if !(h.compR01 >= 0 && h.compR01 <= h.compR02 && h.compR02 <= 0x1F) {
		return false
	}
	if h.compR03 == 0 {
		h.compR03 = 1
	}
	return true
}

func (h *Hca) vbrHeaderRead(r *endibuf.Reader) bool {
	tmp, _ := r.ReadUint16()
	h.vbrR01 = uint32(tmp)
	tmp, _ = r.ReadUint16()
	h.vbrR02 = uint32(tmp)
	return true
}

func (h *Hca) athHeaderRead(r *endibuf.Reader) bool {
	tmp, _ := r.ReadUint16()
	h.athType = uint32(tmp)
	return true
}

func (h *Hca) loopHeaderRead(r *endibuf.Reader) bool {
	h.loopStart, _ = r.ReadUint32()
	h.loopEnd, _ = r.ReadUint32()
	tmp, _ := r.ReadUint16()
	h.loopR01 = uint32(tmp)
	tmp, _ = r.ReadUint16()
	h.loopR02 = uint32(tmp)
	if !(h.loopStart >= 0 && h.loopStart <= h.loopEnd && h.loopEnd < h.blockCount) {
		return false
	}
	return true
}

func (h *Hca) ciphHeaderRead(r *endibuf.Reader) bool {
	tmp, _ := r.ReadUint16()
	h.ciphType = uint32(tmp)
	if !(h.ciphType == 0 || h.ciphType == 1 || h.ciphType == 0x38) {
		return false
	}
	return true
}

func (h *Hca) rvaHeaderRead(r *endibuf.Reader) bool {
	h.rvaVolume, _ = r.ReadFloat32()
	return true
}

func (h *Hca) commHeaderRead(r *endibuf.Reader) bool {
	tmp, _ := r.ReadByte()
	h.commLen = uint32(tmp)
	h.commComment, _ = r.ReadCString()
	return true
}

func ceil2(a, b uint32) uint32 {
	t := uint32(0)
	if b > 0 {
		t := a / b
		if (a % b) > 0 {
			t++
		}
	}
	return t
}

type stHeader struct {
	hca        uint32
	version    uint16
	dataOffset uint16
}

type stFormat struct {
	fmt          uint32
	channelCount uint8
	samplingRate uint32
	blockCount   uint32
	r01          uint16
	r02          uint16
}

type stDecode struct {
	dec          uint32
	blockSize    uint16
	r01          uint8
	r02          uint8
	count1       uint8
	count2       uint8
	r03          uint8
	r04          uint8
	enableCount2 uint8
}
