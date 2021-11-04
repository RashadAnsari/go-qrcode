package qrcode

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"

	"github.com/signintech/gopdf"

	svgo "github.com/ajstarks/svgo"

	"github.com/RashadAnsari/go-qrcode/internal/bitset"
	"github.com/RashadAnsari/go-qrcode/internal/reedsolomon"
)

type QRCode struct {
	// Original content encoded.
	content string

	// QR Code type.
	level         RecoveryLevel
	versionNumber int

	// User settable drawing options.
	ForegroundColor color.Color
	BackgroundColor color.Color

	// Qr Code margin.
	Margin int

	// Base 64 output.
	Base64 bool

	encoder *dataEncoder
	version qrCodeVersion

	data   *bitset.Bitset
	symbol *symbol
	mask   int
}

func New(content string, level RecoveryLevel) (*QRCode, error) {
	encoders := []dataEncoderType{dataEncoderType1To9, dataEncoderType10To26, dataEncoderType27To40}

	var encoder *dataEncoder

	var encoded *bitset.Bitset

	var chosenVersion *qrCodeVersion

	var err error

	for _, t := range encoders {
		encoder, err = newDataEncoder(t)
		if err != nil {
			return nil, err
		}

		encoded, err = encoder.encode([]byte(content))
		if err != nil {
			continue
		}

		chosenVersion = chooseQRCodeVersion(level, encoder, encoded.Len())
		if chosenVersion != nil {
			break
		}
	}

	if err != nil {
		return nil, err
	} else if chosenVersion == nil {
		return nil, errors.New("content too long to encode")
	}

	q := &QRCode{
		content: content,

		level:         level,
		versionNumber: chosenVersion.version,

		ForegroundColor: color.Black,
		BackgroundColor: color.White,

		Margin: 4,

		encoder: encoder,
		data:    encoded,
		version: *chosenVersion,
	}

	return q, nil
}

func (q *QRCode) image(size int) (image.Image, error) {
	// Build QR code.
	if err := q.encode(); err != nil {
		return nil, err
	}

	// Minimum pixels (both width and height) required.
	realSize := q.symbol.size

	// Variable size support.
	if size < 0 {
		size = size * -1 * realSize
	}

	// Actual pixels available to draw the symbol. Automatically increase the
	// image size if it's not large enough.
	if size < realSize {
		size = realSize
	}

	// Output image.
	rect := image.Rectangle{Min: image.Point{}, Max: image.Point{X: size, Y: size}}

	// Saves a few bytes to have them in this order.
	p := color.Palette([]color.Color{q.BackgroundColor, q.ForegroundColor})
	img := image.NewPaletted(rect, p)

	// QR code bitmap.
	bitmap := q.symbol.bitmap()

	// Map each image pixel to the nearest QR code module.
	modulesPerPixel := float64(realSize) / float64(size)

	for y := 0; y < size; y++ {
		y2 := int(float64(y) * modulesPerPixel)

		for x := 0; x < size; x++ {
			x2 := int(float64(x) * modulesPerPixel)
			v := bitmap[y2][x2]

			if v {
				img.Set(x, y, q.ForegroundColor)
			}
		}
	}

	return img, nil
}

func (q *QRCode) PNG(size int) ([]byte, error) {
	img, err := q.image(size)
	if err != nil {
		return nil, err
	}

	encoder := png.Encoder{CompressionLevel: png.BestCompression}

	var b bytes.Buffer

	if err := encoder.Encode(&b, img); err != nil {
		return nil, err
	}

	bts := b.Bytes()

	if q.Base64 {
		bts = []byte(fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(bts)))
	}

	return bts, nil
}

func (q *QRCode) JPEG(size int) ([]byte, error) {
	img, err := q.image(size)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer

	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: jpeg.DefaultQuality}); err != nil {
		return nil, err
	}

	bts := b.Bytes()

	if q.Base64 {
		bts = []byte(fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(bts)))
	}

	return bts, nil
}

func (q *QRCode) PDF(size int) ([]byte, error) {
	img, err := q.image(size)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer

	pdf := gopdf.GoPdf{}

	rect := gopdf.Rect{W: float64(size), H: float64(size)}

	pdf.Start(gopdf.Config{Unit: gopdf.UnitPT, PageSize: rect})
	pdf.AddPage()

	if err := pdf.ImageFrom(img, 0, 0, &rect); err != nil {
		return nil, err
	}

	if err := pdf.Write(&b); err != nil {
		return nil, err
	}

	bts := b.Bytes()

	if q.Base64 {
		bts = []byte(fmt.Sprintf("data:application/pdf;base64,%s", base64.StdEncoding.EncodeToString(bts)))
	}

	return bts, nil
}

func (q *QRCode) SVG(size int) ([]byte, error) {
	if err := q.encode(); err != nil {
		return nil, err
	}

	var b bytes.Buffer

	bgR, bgG, bgB, bgA := q.BackgroundColor.RGBA()
	bgStyle := fmt.Sprintf("fill: rgb(%d, %d, %d); fill-opacity: %.2f",
		bgR>>8, bgG>>8, bgB>>8, float64(bgA>>8)/255,
	)

	fgR, fgG, fgB, fgA := q.ForegroundColor.RGBA()
	fgStyle := fmt.Sprintf("fill: rgb(%d, %d, %d); fill-opacity: %.2f",
		fgR>>8, fgG>>8, fgB>>8, float64(fgA>>8)/255,
	)

	scale := math.Floor(float64(size)/float64(q.symbol.size)) + float64(1)
	size = int(scale) * q.symbol.size

	svg := svgo.New(&b)

	svg.Start(size, size)
	svg.Rect(0, 0, size, size, bgStyle)
	svg.Group(fgStyle)
	svg.Scale(scale)

	bitmap := q.symbol.bitmap()

	for y := 0; y < q.symbol.size; y++ {
		for x := 0; x < q.symbol.size; x++ {
			v := bitmap[y][x]

			if v {
				svg.Rect(x, y, 1, 1)
			}
		}
	}

	svg.Gend()
	svg.Gend()
	svg.End()

	bts := b.Bytes()

	if q.Base64 {
		bts = []byte(fmt.Sprintf("data:image/svg+xml;base64,%s", base64.StdEncoding.EncodeToString(bts)))
	}

	return bts, nil
}

func (q *QRCode) encode() error {
	numTerminatorBits := q.version.numTerminatorBitsRequired(q.data.Len())

	q.addTerminatorBits(numTerminatorBits)

	if err := q.addPadding(); err != nil {
		return err
	}

	encoded, err := q.encodeBlocks()
	if err != nil {
		return err
	}

	const numMasks int = 8

	penalty := 0

	for mask := 0; mask < numMasks; mask++ {
		var s *symbol

		var err error

		s, err = buildRegularSymbol(q.version, mask, encoded, q.Margin)
		if err != nil {
			return err
		}

		numEmptyModules := s.numEmptyModules()
		if numEmptyModules != 0 {
			return fmt.Errorf("bug: numEmptyModules is %d (expected 0) (version=%d)",
				numEmptyModules, q.versionNumber)
		}

		p := s.penaltyScore()

		if q.symbol == nil || p < penalty {
			q.symbol = s
			q.mask = mask
			penalty = p
		}
	}

	return nil
}

func (q *QRCode) addTerminatorBits(numTerminatorBits int) {
	q.data.AppendNumBools(numTerminatorBits, false)
}

func (q *QRCode) encodeBlocks() (*bitset.Bitset, error) {
	// Split into blocks.
	type dataBlock struct {
		data          *bitset.Bitset
		ecStartOffset int
	}

	block := make([]dataBlock, q.version.numBlocks())

	start := 0
	end := 0
	blockID := 0

	for _, b := range q.version.block {
		for j := 0; j < b.numBlocks; j++ {
			start = end
			end = start + b.numDataCodewords*8

			// Apply error correction to each block.
			numErrorCodewords := b.numCodewords - b.numDataCodewords

			substr, err := q.data.Substr(start, end)
			if err != nil {
				return nil, err
			}

			data, err := reedsolomon.Encode(substr, numErrorCodewords)
			if err != nil {
				return nil, err
			}

			block[blockID].data = data
			block[blockID].ecStartOffset = end - start

			blockID++
		}
	}

	// Interleave the blocks.
	result := bitset.New()

	// Combine data blocks.
	working := true

	for i := 0; working; i += 8 {
		working = false

		for j, b := range block {
			if i >= block[j].ecStartOffset {
				continue
			}

			substr, err := b.data.Substr(i, i+8)
			if err != nil {
				return nil, err
			}

			if err := result.Append(substr); err != nil {
				return nil, err
			}

			working = true
		}
	}

	// Combine error correction blocks.
	working = true

	for i := 0; working; i += 8 {
		working = false

		for j, b := range block {
			offset := i + block[j].ecStartOffset
			if offset >= block[j].data.Len() {
				continue
			}

			substr, err := b.data.Substr(offset, offset+8)
			if err != nil {
				return nil, err
			}

			if err := result.Append(substr); err != nil {
				return nil, err
			}

			working = true
		}
	}

	// Append remainder bits.
	result.AppendNumBools(q.version.numRemainderBits, false)

	return result, nil
}

func (q *QRCode) addPadding() error {
	numDataBits := q.version.numDataBits()

	if q.data.Len() == numDataBits {
		return nil
	}

	// Pad to the nearest codeword boundary.
	q.data.AppendNumBools(q.version.numBitsToPadToCodeword(q.data.Len()), false)

	// Pad codewords 0b11101100 and 0b00010001.
	padding := [2]*bitset.Bitset{
		bitset.New(true, true, true, false, true, true, false, false),
		bitset.New(false, false, false, true, false, false, false, true),
	}

	// Insert pad codewords alternately.
	i := 0

	for numDataBits-q.data.Len() >= 8 {
		if err := q.data.Append(padding[i]); err != nil {
			return err
		}

		i = 1 - i // Alternate between 0 and 1.
	}

	if q.data.Len() != numDataBits {
		return fmt.Errorf("BUG: got len %d, expected %d", q.data.Len(), numDataBits)
	}

	return nil
}
