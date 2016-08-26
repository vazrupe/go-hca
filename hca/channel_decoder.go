package hca

type channelDecoder struct {
	param1 uint32
	param2 uint32
	param3 uint32
	param4 uint32
	param5 uint32

	channel []*stChannel
}

func newChannelDecoder(channelCount, compCount, compOption, param1, param2, param3, param4, param5 uint32) *channelDecoder {
	var d channelDecoder

	d.channel = make([]*stChannel, channelCount)
	for i := range d.channel {
		d.channel[i] = newChannel()
	}

	tmp := make([]byte, 0x10)
	for i := range tmp {
		tmp[i] = 0
	}
	b := channelCount / compCount
	if param3 != 0 && b > 1 {
		for i := uint32(0); i < compCount; i++ {
			switch b {
			case 2, 3:
				tmp[b*i+0] = 1
				tmp[b*i+1] = 2
			case 4:
				tmp[b*i+0] = 1
				tmp[b*i+1] = 2
				if compOption == 0 {
					tmp[b*i+2] = 1
					tmp[b*i+3] = 2
				}
			case 5:
				tmp[b*i+0] = 1
				tmp[b*i+1] = 2
				if compOption <= 2 {
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
	for i := uint32(0); i < channelCount; i++ {
		d.channel[i].chType = int(tmp[i])
		d.channel[i].valueIndex = param2 + param3
		d.channel[i].count = param2
		if tmp[i] != 2 {
			d.channel[i].count += param3
		}
	}

	d.param1 = param1
	d.param2 = param2
	d.param3 = param3
	d.param4 = param4
	d.param5 = param5

	return &d
}

func (d *channelDecoder) decode(bitData *clData, athTable []byte) {
	a := (bitData.GetBit(9) << 8) - bitData.GetBit(7)
	// block header
	for _, ch := range d.channel {
		ch.Init(bitData, d.param5, a, athTable)
	}
	// block decode wave datas
	for waveLine := 0; waveLine < 8; waveLine++ {
		for _, ch := range d.channel {
			ch.Fetch(bitData)
			ch.BlockSet(d.param5, d.param4, d.param3+d.param2, d.param1)
		}
		for i := 0; i < (len(d.channel) - 1); i++ {
			d.channel[i].MixBlock(d.channel[i+1], waveLine, d.param1-d.param2, d.param2, d.param3)
		}
		for _, ch := range d.channel {
			calcBlock(ch.block)
			ch.buildWaveBytes(waveLine)
		}
	}
}

func (d *channelDecoder) waveSerialize(volume float32) []float32 {
	channelCount := len(d.channel)
	serialData := make([]float32, 8*0x80*channelCount)

	for i := 0; i < 8; i++ {
		for j := 0; j < 0x80; j++ {
			for k := 0; k < channelCount; k++ {
				f := d.channel[k].wave[i][j] * volume
				if f > 1 {
					f = 1
				} else if f < -1 {
					f = -1
				}
				serialData[(i*0x80+j)*channelCount+k] = f
			}
		}
	}

	return serialData
}
