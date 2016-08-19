package hca

// Data

type clData struct {
	data []byte
	size int
	bit  int
}

func (d *clData) Init(data []byte, size int) {
	d.data = data
	d.size = size*8 - 16
	d.bit = 0
}

func (d *clData) CheckBit(bitSize int) int {
	v := 0
	if (d.bit + bitSize) <= d.size {
		mask := []int{0xFFFFFF, 0x7FFFFF, 0x3FFFFF, 0x1FFFFF, 0x0FFFFF, 0x07FFFF, 0x03FFFF, 0x01FFFF}
		idx := d.bit >> 3
		var data []byte
		if len(d.data) < (idx + 3) {
			data = d.data[idx:len(d.data)]

			for len(data) < 3 {
				data = append(data, 0)
			}
		} else {
			data = d.data[idx : idx+3]
		}

		v = int(data[0])
		v = (v << 8) | int(data[1])
		v = (v << 8) | int(data[2])
		v &= mask[d.bit&7]
		v >>= uint(24 - (d.bit & 7) - bitSize)
	}
	return v
}

func (d *clData) GetBit(bitSize int) int {
	v := d.CheckBit(bitSize)
	d.AddBit(bitSize)
	return v
}

func (d *clData) AddBit(bitSize int) {
	d.bit += bitSize
}
