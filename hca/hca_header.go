package hca

import (
	"encoding/binary"

	"github.com/vazrupe/endibuf"
)

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

func (h *Hca) loadHeader(r *endibuf.Reader) bool {
	endianSave := r.Endian
	r.Endian = binary.BigEndian

	var sig uint32

	// HCA
	r.ReadData(&sig)
	if sig&sigMask == sigHCA {
		if !h.hcaHeaderRead(r) {
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
	if !(h.compR01 == 1 && h.compR02 == 15) {
		return false
	}
	h.compR09 = ceil2(h.compR05-(h.compR06+h.compR07), h.compR08)
	h.decoder = newChannelDecoder(h.channelCount, h.compR03, h.compR04, h.compR05, h.compR06, h.compR07, h.compR08, h.compR09)

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
		t = a / b
		if (a % b) > 0 {
			t++
		}
	}
	return t
}
