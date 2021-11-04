package reedsolomon

import (
	"errors"

	"github.com/RashadAnsari/go-qrcode/internal/bitset"
)

func Encode(data *bitset.Bitset, numECBytes int) (*bitset.Bitset, error) {
	// Create a polynomial representing |data|.
	//
	// The bytes are interpreted as the sequence of coefficients of a polynomial.
	// The last byte's value becomes the x^0 coefficient, the second to last
	// becomes the x^1 coefficient and so on.
	ecpoly, err := newGFPolyFromData(data)
	if err != nil {
		return nil, err
	}

	ecpoly = gfPolyMultiply(ecpoly, newGFPolyMonomial(gfOne, numECBytes))

	// Pick the generator polynomial.
	generator, err := rsGeneratorPoly(numECBytes)
	if err != nil {
		return nil, err
	}

	// Generate the error correction bytes.
	remainder, err := gfPolyRemainder(ecpoly, generator)
	if err != nil {
		return nil, err
	}

	// Combine the data & error correcting bytes.
	// The mathematically correct answer is:
	//
	//	result := gfPolyAdd(ecpoly, remainder).
	//
	// The encoding used by QR Code 2005 is slightly different this result: To
	// preserve the original |data| bit sequence exactly, the data and remainder
	// are combined manually below. This ensures any most significant zero bits
	// are preserved (and not optimised away).
	result := bitset.Clone(data)

	if err := result.AppendBytes(remainder.data(numECBytes)); err != nil {
		return nil, err
	}

	return result, nil
}

func rsGeneratorPoly(degree int) (gfPoly, error) {
	if degree < 2 {
		return gfPoly{}, errors.New("degree < 2")
	}

	generator := gfPoly{term: []gfElement{1}}

	for i := 0; i < degree; i++ {
		nextPoly := gfPoly{term: []gfElement{gfExpTable[i], 1}}
		generator = gfPolyMultiply(generator, nextPoly)
	}

	return generator, nil
}
