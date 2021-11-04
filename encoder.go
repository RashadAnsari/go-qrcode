package qrcode

import (
	"errors"
	"fmt"

	"github.com/RashadAnsari/go-qrcode/internal/bitset"
)

type dataMode uint8

const (
	dataModeNone dataMode = 1 << iota
	dataModeNumeric
	dataModeAlphanumeric
	dataModeByte
)

type dataEncoderType uint8

const (
	dataEncoderType1To9 dataEncoderType = iota
	dataEncoderType10To26
	dataEncoderType27To40
)

type segment struct {
	dataMode dataMode
	data     []byte
}

type dataEncoder struct {
	// Minimum & maximum versions supported.
	minVersion int
	maxVersion int

	// Mode indicator bit sequences.
	numericModeIndicator      *bitset.Bitset
	alphanumericModeIndicator *bitset.Bitset
	byteModeIndicator         *bitset.Bitset

	// Character count lengths.
	numNumericCharCountBits      int
	numAlphanumericCharCountBits int
	numByteCharCountBits         int

	// The raw input data.
	data []byte

	// The data classified into unoptimised segments.
	actual []segment

	// The data classified into optimised segments.
	optimised []segment
}

func newDataEncoder(t dataEncoderType) (*dataEncoder, error) {
	switch t {
	case dataEncoderType1To9:
		return &dataEncoder{
			minVersion:                   1,
			maxVersion:                   9,
			numericModeIndicator:         bitset.New(b0, b0, b0, b1),
			alphanumericModeIndicator:    bitset.New(b0, b0, b1, b0),
			byteModeIndicator:            bitset.New(b0, b1, b0, b0),
			numNumericCharCountBits:      10,
			numAlphanumericCharCountBits: 9,
			numByteCharCountBits:         8,
		}, nil
	case dataEncoderType10To26:
		return &dataEncoder{
			minVersion:                   10,
			maxVersion:                   26,
			numericModeIndicator:         bitset.New(b0, b0, b0, b1),
			alphanumericModeIndicator:    bitset.New(b0, b0, b1, b0),
			byteModeIndicator:            bitset.New(b0, b1, b0, b0),
			numNumericCharCountBits:      12,
			numAlphanumericCharCountBits: 11,
			numByteCharCountBits:         16,
		}, nil
	case dataEncoderType27To40:
		return &dataEncoder{
			minVersion:                   27,
			maxVersion:                   40,
			numericModeIndicator:         bitset.New(b0, b0, b0, b1),
			alphanumericModeIndicator:    bitset.New(b0, b0, b1, b0),
			byteModeIndicator:            bitset.New(b0, b1, b0, b0),
			numNumericCharCountBits:      14,
			numAlphanumericCharCountBits: 13,
			numByteCharCountBits:         16,
		}, nil
	default:
		return nil, errors.New("unknown dataEncoderType")
	}
}

func (d *dataEncoder) encode(data []byte) (*bitset.Bitset, error) {
	d.data = data
	d.actual = nil
	d.optimised = nil

	if len(data) == 0 {
		return nil, errors.New("no data to encode")
	}

	// Classify data into unoptimised segments.
	highestRequiredMode := d.classifyDataModes()

	// Optimise segments.
	err := d.optimiseDataModes()
	if err != nil {
		return nil, err
	}

	// Check if a single byte encoded segment would be more efficient.
	optimizedLength := 0

	for _, s := range d.optimised {
		length, err := d.encodedLength(s.dataMode, len(s.data))
		if err != nil {
			return nil, err
		}

		optimizedLength += length
	}

	singleByteSegmentLength, err := d.encodedLength(highestRequiredMode, len(d.data))
	if err != nil {
		return nil, err
	}

	if singleByteSegmentLength <= optimizedLength {
		d.optimised = []segment{{dataMode: highestRequiredMode, data: d.data}}
	}

	// Encode data.
	encoded := bitset.New()

	for _, s := range d.optimised {
		if err := d.encodeDataRaw(s.data, s.dataMode, encoded); err != nil {
			return nil, err
		}
	}

	return encoded, nil
}

func (d *dataEncoder) classifyDataModes() dataMode {
	var start int

	mode := dataModeNone
	highestRequiredMode := mode

	for i, v := range d.data {
		var newMode dataMode

		switch {
		case v >= 0x30 && v <= 0x39:
			newMode = dataModeNumeric
		case v == 0x20 || v == 0x24 || v == 0x25 || v == 0x2a || v == 0x2b || v ==
			0x2d || v == 0x2e || v == 0x2f || v == 0x3a || (v >= 0x41 && v <= 0x5a):
			newMode = dataModeAlphanumeric
		default:
			newMode = dataModeByte
		}

		if newMode != mode {
			if i > 0 {
				d.actual = append(d.actual, segment{dataMode: mode, data: d.data[start:i]})

				start = i
			}

			mode = newMode
		}

		if newMode > highestRequiredMode {
			highestRequiredMode = newMode
		}
	}

	d.actual = append(d.actual, segment{dataMode: mode, data: d.data[start:len(d.data)]})

	return highestRequiredMode
}

func (d *dataEncoder) optimiseDataModes() error {
	for i := 0; i < len(d.actual); {
		mode := d.actual[i].dataMode
		numChars := len(d.actual[i].data)

		j := i + 1
		for j < len(d.actual) {
			nextNumChars := len(d.actual[j].data)
			nextMode := d.actual[j].dataMode

			if nextMode > mode {
				break
			}

			coalescedLength, err := d.encodedLength(mode, numChars+nextNumChars)
			if err != nil {
				return err
			}

			seperateLength1, err := d.encodedLength(mode, numChars)
			if err != nil {
				return err
			}

			seperateLength2, err := d.encodedLength(nextMode, nextNumChars)
			if err != nil {
				return err
			}

			if coalescedLength < seperateLength1+seperateLength2 {
				j++

				numChars += nextNumChars
			} else {
				break
			}
		}

		optimised := segment{dataMode: mode, data: make([]byte, 0, numChars)}

		for k := i; k < j; k++ {
			optimised.data = append(optimised.data, d.actual[k].data...)
		}

		d.optimised = append(d.optimised, optimised)

		i = j
	}

	return nil
}

func (d *dataEncoder) encodeDataRaw(data []byte, dataMode dataMode, encoded *bitset.Bitset) error {
	modeIndicator, err := d.modeIndicator(dataMode)
	if err != nil {
		return err
	}

	charCountBits, err := d.charCountBits(dataMode)
	if err != nil {
		return err
	}

	// Append mode indicator.
	if err := encoded.Append(modeIndicator); err != nil {
		return err
	}

	// Append character count.
	if err := encoded.AppendUint32(uint32(len(data)), charCountBits); err != nil {
		return err
	}

	// Append data.
	switch dataMode {
	case dataModeNumeric:
		for i := 0; i < len(data); i += 3 {
			charsRemaining := len(data) - i

			var value uint32

			bitsUsed := 1

			for j := 0; j < charsRemaining && j < 3; j++ {
				value *= 10
				value += uint32(data[i+j] - 0x30)
				bitsUsed += 3
			}

			if err := encoded.AppendUint32(value, bitsUsed); err != nil {
				return err
			}
		}
	case dataModeAlphanumeric:
		for i := 0; i < len(data); i += 2 {
			charsRemaining := len(data) - i

			var value uint32

			for j := 0; j < charsRemaining && j < 2; j++ {
				value *= 45

				v, err := encodeAlphanumericCharacter(data[i+j])
				if err != nil {
					return err
				}

				value += v
			}

			bitsUsed := 6

			if charsRemaining > 1 {
				bitsUsed = 11
			}

			if err := encoded.AppendUint32(value, bitsUsed); err != nil {
				return err
			}
		}
	case dataModeByte:
		for _, b := range data {
			if err := encoded.AppendByte(b, 8); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *dataEncoder) modeIndicator(dataMode dataMode) (*bitset.Bitset, error) {
	switch dataMode {
	case dataModeNumeric:
		return d.numericModeIndicator, nil
	case dataModeAlphanumeric:
		return d.alphanumericModeIndicator, nil
	case dataModeByte:
		return d.byteModeIndicator, nil
	default:
		return nil, errors.New("unknown data mode")
	}
}

func (d *dataEncoder) charCountBits(dataMode dataMode) (int, error) {
	switch dataMode {
	case dataModeNumeric:
		return d.numNumericCharCountBits, nil
	case dataModeAlphanumeric:
		return d.numAlphanumericCharCountBits, nil
	case dataModeByte:
		return d.numByteCharCountBits, nil
	default:
		return 0, errors.New("unknown data mode")
	}
}

func (d *dataEncoder) encodedLength(dataMode dataMode, n int) (int, error) {
	modeIndicator, err := d.modeIndicator(dataMode)
	if err != nil {
		return 0, err
	}

	charCountBits, err := d.charCountBits(dataMode)
	if err != nil {
		return 0, err
	}

	if modeIndicator == nil {
		return 0, errors.New("mode not supported")
	}

	maxLength := (1 << uint8(charCountBits)) - 1

	if n > maxLength {
		return 0, errors.New("length too long to be represented")
	}

	length := modeIndicator.Len() + charCountBits

	switch dataMode {
	case dataModeNumeric:
		length += 10 * (n / 3)

		if n%3 != 0 {
			length += 1 + 3*(n%3)
		}
	case dataModeAlphanumeric:
		length += 11 * (n / 2)
		length += 6 * (n % 2)
	case dataModeByte:
		length += 8 * n
	}

	return length, nil
}

func encodeAlphanumericCharacter(v byte) (uint32, error) {
	c := uint32(v)

	switch {
	case c >= '0' && c <= '9':
		// 0-9 encoded as 0-9.
		return c - '0', nil
	case c >= 'A' && c <= 'Z':
		// A-Z encoded as 10-35.
		return c - 'A' + 10, nil
	case c == ' ':
		return 36, nil
	case c == '$':
		return 37, nil
	case c == '%':
		return 38, nil
	case c == '*':
		return 39, nil
	case c == '+':
		return 40, nil
	case c == '-':
		return 41, nil
	case c == '.':
		return 42, nil
	case c == '/':
		return 43, nil
	case c == ':':
		return 44, nil
	default:
		return 0, fmt.Errorf("encodeAlphanumericCharacter() with non alphanumeric char %v", v)
	}
}
