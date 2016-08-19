package hca

import (
	"encoding/binary"

	"github.com/vazrupe/endibuf"
)

type stWaveHeader struct {
	Riff *stWAVEriff
	Smpl *stWAVEsmpl
	Note *stWAVEnote
	Data *stWAVEdata

	RiffOk bool
	SmplOk bool
	NoteOk bool
	DataOk bool
}

func newWaveHeader() *stWaveHeader {
	return &stWaveHeader{
		Riff: newWaveRiff(),
		Smpl: newWaveSmpl(),
		Note: newWaveNote(),
		Data: newWaveData(),

		RiffOk: true,
		SmplOk: false,
		NoteOk: false,
		DataOk: true,
	}
}

func (wv *stWaveHeader) Write(w *endibuf.Writer) {
	if wv.RiffOk {
		wv.Riff.Write(w)
	}
	if wv.SmplOk {
		wv.Smpl.Write(w)
	}
	if wv.NoteOk {
		wv.Note.Write(w)
	}
	if wv.DataOk {
		wv.Data.Write(w)
	}
}

type stWAVEriff struct {
	riff             []byte
	riffSize         uint32
	wave             []byte
	fmt              []byte
	fmtSize          uint32
	fmtType          uint16
	fmtChannelCount  uint16
	fmtSamplingRate  uint32
	fmtSamplesPerSec uint32
	fmtSamplingSize  uint16
	fmtBitCount      uint16
}

func newWaveRiff() *stWAVEriff {
	return &stWAVEriff{
		riff:             []byte{'R', 'I', 'F', 'F'},
		riffSize:         0,
		wave:             []byte{'W', 'A', 'V', 'E'},
		fmt:              []byte{'f', 'm', 't', ' '},
		fmtSize:          0x10,
		fmtType:          0,
		fmtChannelCount:  0,
		fmtSamplingRate:  0,
		fmtSamplesPerSec: 0,
		fmtSamplingSize:  0,
		fmtBitCount:      0,
	}
}

func (h *stWAVEriff) Write(w *endibuf.Writer) {
	endianSave := w.Endian

	w.Endian = binary.BigEndian
	w.WriteBytes(h.riff)

	w.Endian = binary.LittleEndian
	w.WriteUint32(h.riffSize)

	w.Endian = binary.BigEndian
	w.WriteBytes(h.wave)
	w.WriteBytes(h.fmt)

	w.Endian = binary.LittleEndian
	w.WriteUint32(h.fmtSize)
	w.WriteUint16(h.fmtType)
	w.WriteUint16(h.fmtChannelCount)
	w.WriteUint32(h.fmtSamplingRate)
	w.WriteUint32(h.fmtSamplesPerSec)
	w.WriteUint16(h.fmtSamplingSize)
	w.WriteUint16(h.fmtBitCount)

	w.Endian = endianSave
}

type stWAVEsmpl struct {
	smpl              []byte
	smplSize          uint32
	manufacturer      uint32
	product           uint32
	samplePeriod      uint32
	MIDIUnityNote     uint32
	MIDIPitchFraction uint32
	SMPTEFormat       uint32
	SMPTEOffset       uint32
	sampleLoops       uint32
	samplerData       uint32
	loopIdentifier    uint32
	loopType          uint32
	loopStart         uint32
	loopEnd           uint32
	loopFraction      uint32
	loopPlayCount     uint32
}

func newWaveSmpl() *stWAVEsmpl {
	return &stWAVEsmpl{
		smpl:              []byte{'s', 'm', 'p', 'l'},
		smplSize:          0x3C,
		manufacturer:      0,
		product:           0,
		samplePeriod:      0,
		MIDIUnityNote:     0x3C,
		MIDIPitchFraction: 0,
		SMPTEFormat:       0,
		SMPTEOffset:       0,
		sampleLoops:       1,
		samplerData:       0x18,
		loopIdentifier:    0,
		loopType:          0,
		loopStart:         0,
		loopEnd:           0,
		loopFraction:      0,
		loopPlayCount:     0,
	}
}

func (s *stWAVEsmpl) Write(w *endibuf.Writer) {
	endianSave := w.Endian

	w.Endian = binary.BigEndian
	w.WriteBytes(s.smpl)

	w.Endian = binary.LittleEndian
	w.WriteUint32(s.smplSize)
	w.WriteUint32(s.manufacturer)
	w.WriteUint32(s.product)
	w.WriteUint32(s.samplePeriod)
	w.WriteUint32(s.MIDIUnityNote)
	w.WriteUint32(s.MIDIPitchFraction)
	w.WriteUint32(s.SMPTEFormat)
	w.WriteUint32(s.SMPTEOffset)
	w.WriteUint32(s.sampleLoops)
	w.WriteUint32(s.samplerData)
	w.WriteUint32(s.loopIdentifier)
	w.WriteUint32(s.loopType)
	w.WriteUint32(s.loopStart)
	w.WriteUint32(s.loopEnd)
	w.WriteUint32(s.loopFraction)
	w.WriteUint32(s.loopPlayCount)

	w.Endian = endianSave
}

type stWAVEnote struct {
	note     []byte
	noteSize uint32
	dwName   uint32
	comm     string
}

func newWaveNote() *stWAVEnote {
	return &stWAVEnote{
		note:     []byte{'n', 'o', 't', 'e'},
		noteSize: 0,
		dwName:   0,
	}
}

func (n *stWAVEnote) Write(w *endibuf.Writer) {
	endianSave := w.Endian

	w.Endian = binary.BigEndian
	w.WriteBytes(n.note)

	w.Endian = binary.LittleEndian
	w.WriteUint32(n.noteSize)
	w.WriteUint32(n.dwName)
	w.WriteCString(n.comm)

	w.Endian = endianSave
}

type stWAVEdata struct {
	data     []byte
	dataSize uint32
}

func newWaveData() *stWAVEdata {
	return &stWAVEdata{
		data:     []byte{'d', 'a', 't', 'a'},
		dataSize: 0,
	}
}

func (d *stWAVEdata) Write(w *endibuf.Writer) {
	endianSave := w.Endian

	w.Endian = binary.BigEndian
	w.WriteBytes(d.data)

	w.Endian = binary.LittleEndian
	w.WriteUint32(d.dataSize)

	w.Endian = endianSave
}
