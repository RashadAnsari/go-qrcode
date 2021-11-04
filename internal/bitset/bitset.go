package bitset

import (
	"fmt"
)

type Bitset struct {
	// The number of bits stored.
	numBits int

	// Storage for individual bits.
	bits []byte
}

func New(v ...bool) *Bitset {
	b := &Bitset{numBits: 0, bits: make([]byte, 0)}
	b.AppendBools(v...)

	return b
}

func Clone(from *Bitset) *Bitset {
	return &Bitset{numBits: from.numBits, bits: from.bits[:]}
}

func (b *Bitset) Substr(start int, end int) (*Bitset, error) {
	if start > end || end > b.numBits {
		return nil, fmt.Errorf("out of range start=%d end=%d numBits=%d", start, end, b.numBits)
	}

	result := New()
	result.ensureCapacity(end - start)

	for i := start; i < end; i++ {
		bo, err := b.At(i)
		if err != nil {
			return nil, err
		}

		if bo {
			result.bits[result.numBits/8] |= 0x80 >> uint(result.numBits%8)
		}

		result.numBits++
	}

	return result, nil
}

func (b *Bitset) AppendBytes(data []byte) error {
	for _, d := range data {
		if err := b.AppendByte(d, 8); err != nil {
			return err
		}
	}

	return nil
}

func (b *Bitset) AppendByte(value byte, numBits int) error {
	b.ensureCapacity(numBits)

	if numBits > 8 {
		return fmt.Errorf("numBits %d out of range 0-8", numBits)
	}

	for i := numBits - 1; i >= 0; i-- {
		if value&(1<<uint(i)) != 0 {
			b.bits[b.numBits/8] |= 0x80 >> uint(b.numBits%8)
		}

		b.numBits++
	}

	return nil
}

func (b *Bitset) AppendUint32(value uint32, numBits int) error {
	b.ensureCapacity(numBits)

	if numBits > 32 {
		return fmt.Errorf("numBits %d out of range 0-32", numBits)
	}

	for i := numBits - 1; i >= 0; i-- {
		if value&(1<<uint(i)) != 0 {
			b.bits[b.numBits/8] |= 0x80 >> uint(b.numBits%8)
		}

		b.numBits++
	}

	return nil
}

func (b *Bitset) ensureCapacity(numBits int) {
	numBits += b.numBits

	newNumBytes := numBits / 8
	if numBits%8 != 0 {
		newNumBytes++
	}

	if len(b.bits) >= newNumBytes {
		return
	}

	b.bits = append(b.bits, make([]byte, newNumBytes+2*len(b.bits))...)
}

func (b *Bitset) Append(other *Bitset) error {
	b.ensureCapacity(other.numBits)

	for i := 0; i < other.numBits; i++ {
		bo, err := other.At(i)
		if err != nil {
			return err
		}

		if bo {
			b.bits[b.numBits/8] |= 0x80 >> uint(b.numBits%8)
		}

		b.numBits++
	}

	return nil
}

func (b *Bitset) AppendBools(bits ...bool) {
	b.ensureCapacity(len(bits))

	for _, v := range bits {
		if v {
			b.bits[b.numBits/8] |= 0x80 >> uint(b.numBits%8)
		}

		b.numBits++
	}
}

func (b *Bitset) AppendNumBools(num int, value bool) {
	for i := 0; i < num; i++ {
		b.AppendBools(value)
	}
}

func (b *Bitset) Len() int {
	return b.numBits
}

func (b *Bitset) At(index int) (bool, error) {
	if index >= b.numBits {
		return false, fmt.Errorf("index %d out of range", index)
	}

	return (b.bits[index/8] & (0x80 >> byte(index%8))) != 0, nil
}

func (b *Bitset) ByteAt(index int) (byte, error) {
	if index < 0 || index >= b.numBits {
		return 0, fmt.Errorf("index %d out of range", index)
	}

	var result byte

	for i := index; i < index+8 && i < b.numBits; i++ {
		result <<= 1

		bo, err := b.At(i)
		if err != nil {
			return 0, err
		}

		if bo {
			result |= 1
		}
	}

	return result, nil
}
