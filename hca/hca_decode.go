package hca

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/vazrupe/endibuf"
)

// DecodeFromFile is file decode, return decode success/failed
func (h *Hca) DecodeFromFile(src, dst string) bool {
	f, err := os.Open(src)
	if err != nil {
		return false
	}
	defer f.Close()
	r := endibuf.NewReader(f)
	f2, err := os.Create(dst)
	if err != nil {
		return false
	}
	w := endibuf.NewWriter(f2)

	success := h.decodeBuffer(r, w)

	f2.Close()
	if !success {
		os.Remove(dst)
		return false
	}

	return true
}

// DecodeFromBytes is []byte data decode
func (h *Hca) DecodeFromBytes(data []byte) (decoded []byte, ok bool) {
	decodedData := []byte{}

	if len(data) < 8 {
		return decodedData, false
	}

	headerSize := binary.BigEndian.Uint16(data[6:])
	if len(data) < int(headerSize) {
		return decodedData, false
	}

	// create read buffer
	base := bytes.NewReader(data)
	buf := io.NewSectionReader(base, 0, base.Size())
	r := endibuf.NewReader(buf)

	// create temp file (write)
	tempfile, _ := ioutil.TempFile("", "hca_wav_temp_")
	defer os.Remove(tempfile.Name())
	w := endibuf.NewWriter(tempfile)
	w.Endian = binary.LittleEndian

	if !h.decodeBuffer(r, w) {
		return decodedData, false
	}

	tempfile.Seek(0, 0)
	decodedData, _ = ioutil.ReadAll(tempfile)

	return decodedData, true
}

func (h *Hca) decodeBuffer(r *endibuf.Reader, w *endibuf.Writer) bool {
	saveEndian := r.Endian

	r.Endian = binary.BigEndian

	// size check
	if h.Loop < 0 {
		return false
	}
	switch h.Mode {
	case ModeFloat, Mode8Bit, Mode16Bit, Mode24Bit, Mode32Bit:
		break
	default:
		return false
	}

	// header read
	if !h.loadHeader(r) {
		return false
	}
	r.Seek(int64(h.dataOffset), 0)

	// create temp file (write)
	w.Endian = binary.LittleEndian

	wavHeader := h.buildWaveHeader()
	wavHeader.Write(w)

	// adjust the relative volume
	h.rvaVolume *= h.Volume

	// decode
	if h.Loop == 0 {
		if !h.decodeFromBytesDecode(r, w, h.dataOffset, h.blockCount) {
			return false
		}
	} else {
		loopBlockOffset := h.dataOffset + h.loopStart*h.blockSize
		loopBlockCount := h.loopEnd - h.loopStart
		if !h.decodeFromBytesDecode(r, w, h.dataOffset, h.loopEnd) {
			return false
		}
		for i := 1; i < h.Loop; i++ {
			if !h.decodeFromBytesDecode(r, w, loopBlockOffset, loopBlockCount) {
				return false
			}
		}
		if !h.decodeFromBytesDecode(r, w, loopBlockOffset, h.blockCount-h.loopStart) {
			return false
		}
	}

	r.Endian = saveEndian

	return true
}

func (h *Hca) buildWaveHeader() *stWaveHeader {
	wavHeader := newWaveHeader()

	riff := wavHeader.Riff
	smpl := wavHeader.Smpl
	note := wavHeader.Note
	data := wavHeader.Data

	if h.Mode > 0 {
		riff.fmtType = 1
		riff.fmtBitCount = uint16(h.Mode)
	} else {
		riff.fmtType = 3
		riff.fmtBitCount = 32
	}
	riff.fmtChannelCount = uint16(h.channelCount)
	riff.fmtSamplingRate = h.samplingRate
	riff.fmtSamplingSize = riff.fmtBitCount / 8 * riff.fmtChannelCount
	riff.fmtSamplesPerSec = riff.fmtSamplingRate * uint32(riff.fmtSamplingSize)

	if h.loopFlg {
		smpl.samplePeriod = uint32(1 / float64(riff.fmtSamplingRate) * 1000000000)
		smpl.loopStart = h.loopStart * 0x80 * 8 * uint32(riff.fmtSamplingSize)
		smpl.loopEnd = h.loopEnd * 0x80 * 8 * uint32(riff.fmtSamplingSize)
		if h.loopR01 == 0x80 {
			smpl.loopPlayCount = 0
		} else {
			smpl.loopPlayCount = h.loopR01
		}
	} else if h.Loop != 0 {
		smpl.loopStart = 0
		smpl.loopEnd = h.blockCount * 0x80 * 8 * uint32(riff.fmtSamplingSize)
		h.loopStart = 0
		h.loopEnd = h.blockCount
	}
	if h.commLen > 0 {
		wavHeader.NoteOk = true

		note.noteSize = 4 + h.commLen + 1
		note.comm = h.commComment
		if (note.noteSize & 3) != 0 {
			note.noteSize += 4 - (note.noteSize & 3)
		}
	}
	data.dataSize = h.blockCount*0x80*8*uint32(riff.fmtSamplingSize) + (smpl.loopEnd-smpl.loopStart)*uint32(h.Loop)
	riff.riffSize = 0x1C + 8 + data.dataSize
	if h.loopFlg && h.Loop == 0 {
		// smpl Size
		riff.riffSize += 17 * 4
		wavHeader.SmplOk = true
	}
	if h.commLen > 0 {
		riff.riffSize += 8 + note.noteSize
	}

	return wavHeader
}

func (h *Hca) decodeFromBytesDecode(r *endibuf.Reader, w *endibuf.Writer, address, count uint32) bool {
	var f float32
	r.Seek(int64(address), 0)
	saveBlock := make([]float32, 8*0x80*h.channelCount)
	for l := uint32(0); l < count; l++ {
		data, _ := r.ReadBytes(int(h.blockSize))
		if !h.decode(data) {
			return false
		}
		for i := 0; i < 8; i++ {
			channelBlockIdx := i * 0x80 * int(h.channelCount)
			for j := 0; j < 0x80; j++ {
				byteIdx := j * int(h.channelCount)
				for k := uint32(0); k < h.channelCount; k++ {
					f = h.channel[k].wave[i][j] * h.rvaVolume
					if f > 1 {
						f = 1
					} else if f < -1 {
						f = -1
					}
					saveBlock[channelBlockIdx+byteIdx+int(k)] = f
				}
			}
		}
		h.save(saveBlock, w)

		address += h.blockSize
		//break
	}
	return true
}

func (h *Hca) decode(data []byte) bool {
	// block data
	if len(data) < int(h.blockSize) {
		return false
	}
	if checkSum(data, 0) != 0 {
		return false
	}
	mask := h.cipher.Mask(data)
	d := &clData{}
	d.Init(mask, int(h.blockSize))
	fmt.Printf("% x\n", mask)
	magic := d.GetBit(16)
	if magic == 0xFFFF {
		a := (d.GetBit(9) << 8) - d.GetBit(7)
		for i := uint32(0); i < h.channelCount; i++ {
			h.channel[i].Init(d, h.compR09, a, h.ath.GetTable())
		}
		for waveLine := 0; waveLine < 8; waveLine++ {
			for j := uint32(0); j < h.channelCount; j++ {
				h.channel[j].Fetch(d)
				h.channel[j].BlockSetup1(h.compR09, h.compR08, h.compR07+h.compR06, h.compR05)
			}
			for j := uint32(0); j < (h.channelCount - 1); j++ {
				h.channel[j].BlockSetup2(h.channel[j+1], waveLine, h.compR05-h.compR06, h.compR06, h.compR07)
			}
			for j := uint32(0); j < h.channelCount; j++ {
				h.channel[j].CalcBlock()
				h.channel[j].buildWaveBytes(waveLine)
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

func (h *Hca) save(base []float32, w *endibuf.Writer) {
	switch h.Mode {
	case ModeFloat:
		w.WriteData(base)
	case Mode8Bit:
		w.WriteData(mode8BitConvert(base))
	case Mode16Bit:
		w.WriteData(mode16BitConvert(base))
	case Mode24Bit:
		w.WriteData(mode24BitConvert(base))
	case Mode32Bit:
		w.WriteData(mode32BitConvert(base))
	}
}

func mode8BitConvert(base []float32) []int8 {
	res := make([]int8, len(base))
	for i := range res {
		res[i] = int8(int(base[i]*0x7F) + 0x80)
	}
	return res
}

func mode16BitConvert(base []float32) []int16 {
	res := make([]int16, len(base))
	for i := range res {
		res[i] = int16(base[i] * 0x7FFF)
	}
	return res
}

func mode24BitConvert(base []float32) []byte {
	res := make([]byte, len(base)*3)

	for i := range res {
		v := int32(base[i] * 0x7FFFFF)
		res[i] = byte((v & 0xFF0000) >> 16)
		res[i+1] = byte((v & 0xFF00) >> 8)
		res[i+2] = byte((v & 0xFF))
	}
	return res
}

func mode32BitConvert(base []float32) []int32 {
	res := make([]int32, len(base))
	for i := range res {
		res[i] = int32(base[i] * 0x7FFFFFFF)
	}
	return res
}
