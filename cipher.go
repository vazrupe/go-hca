package hca

// Cipher is hca byte cipher
type Cipher struct {
	table [0x100]byte
}

// NewCipher is default mask bind
func NewCipher() *Cipher {
	var ci Cipher
	ci.init0()
	return &ci
}

// Init is Cipher key initialize
func (ci *Cipher) Init(t int, key1, key2 uint32) bool {
	if key1 == 0 && key2 == 0 {
		t = 0
	}
	switch t {
	case 0:
		ci.init0()
	case 1:
		ci.init1()
	case 56:
		ci.init56(key1, key2)
	default:
		return false
	}
	return true
}

// Mask return size mask
func (ci *Cipher) Mask(data []byte) []byte {
	mask := make([]byte, len(data))

	for i := range mask {
		mask[i] = ci.table[data[i]&0xFF]
	}
	return mask
}

func (ci *Cipher) init0() {
	for i := range ci.table {
		ci.table[i] = byte(i)
	}
}

func (ci *Cipher) init1() {
	for i, v := 1, 0; i < 0xFF; i++ {
		v = (v*13 + 11) & 0xFF
		if v == 0 || v == 0xFF {
			v = (v*13 + 11) & 0xFF
		}
		ci.table[i] = byte(v)
	}
	ci.table[0] = 0
	ci.table[0xFF] = 0xFF
}

func (ci *Cipher) init56(key1, key2 uint32) {
	// create table1
	t1 := make([]byte, 8)
	if key1 == 0 {
		key2--
	}
	key1--
	for i := range t1 {
		t1[i] = byte(key1)
		key1 = (key1 >> 8) | (key2 << 24)
		key2 >>= 8
	}

	t2 := []byte{t1[1], t1[1] ^ t1[6],
		t1[2] ^ t1[3], t1[2],
		t1[2] ^ t1[1], t1[3] ^ t1[4],
		t1[3], t1[3] ^ t1[2],
		t1[4] ^ t1[5], t1[4],
		t1[4] ^ t1[3], t1[5] ^ t1[6],
		t1[5], t1[5] ^ t1[4],
		t1[6] ^ t1[1], t1[6]}

	t3 := make([]byte, 0x100)
	t31, t32 := make([]byte, 0x10), make([]byte, 0x10)
	init56CreateTable(t31, t1[0])
	for i := 0; i < 0x10; i++ {
		init56CreateTable(t32, t2[i])
		v := t31[i] << 4
		for j := range t32 {
			t3[i*0x10+j] = v | t32[j]
		}
	}

	v := 0
	for i := 1; i < 0x100; {
		v = (v + 0x11) & 0xFF
		a := t3[v]
		if a != 0 && a != 0xFF {
			ci.table[i] = a
			i++
		}
	}
	ci.table[0] = 0
	ci.table[0xFF] = 0xFF
}

func init56CreateTable(table []byte, key byte) {
	mul := ((int(key) & 1) << 3) | 5
	add := (int(key) & 0xE) | 1
	key >>= 4
	for i := 0; i < 0x10; i++ {
		key = byte(int(key)*mul+add) & 0xF
		table[i] = key
	}
}
