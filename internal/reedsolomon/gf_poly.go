package reedsolomon

import (
	"errors"

	"github.com/RashadAnsari/go-qrcode/internal/bitset"
)

type gfPoly struct {
	term []gfElement
}

func newGFPolyFromData(data *bitset.Bitset) (gfPoly, error) {
	numTotalBytes := data.Len() / 8
	if data.Len()%8 != 0 {
		numTotalBytes++
	}

	result := gfPoly{term: make([]gfElement, numTotalBytes)}

	i := numTotalBytes - 1

	for j := 0; j < data.Len(); j += 8 {
		by, err := data.ByteAt(j)
		if err != nil {
			return gfPoly{}, err
		}

		result.term[i] = gfElement(by)

		i--
	}

	return result, nil
}

func newGFPolyMonomial(term gfElement, degree int) gfPoly {
	if term == gfZero {
		return gfPoly{}
	}

	result := gfPoly{term: make([]gfElement, degree+1)}
	result.term[degree] = term

	return result
}

func (e gfPoly) data(numTerms int) []byte {
	result := make([]byte, numTerms)

	i := numTerms - len(e.term)

	for j := len(e.term) - 1; j >= 0; j-- {
		result[i] = byte(e.term[j])
		i++
	}

	return result
}

func (e gfPoly) numTerms() int {
	return len(e.term)
}

func gfPolyMultiply(a, b gfPoly) gfPoly {
	numATerms := a.numTerms()
	numBTerms := b.numTerms()

	result := gfPoly{term: make([]gfElement, numATerms+numBTerms)}

	for i := 0; i < numATerms; i++ {
		for j := 0; j < numBTerms; j++ {
			if a.term[i] != 0 && b.term[j] != 0 {
				monomial := gfPoly{term: make([]gfElement, i+j+1)}
				monomial.term[i+j] = gfMultiply(a.term[i], b.term[j])

				result = gfPolyAdd(result, monomial)
			}
		}
	}

	return result.normalised()
}

func gfPolyRemainder(numerator, denominator gfPoly) (gfPoly, error) {
	if denominator.equals(gfPoly{}) {
		return gfPoly{}, errors.New("remainder by zero")
	}

	remainder := numerator

	for remainder.numTerms() >= denominator.numTerms() {
		degree := remainder.numTerms() - denominator.numTerms()

		coefficient, err := gfDivide(remainder.term[remainder.numTerms()-1], denominator.term[denominator.numTerms()-1])
		if err != nil {
			return gfPoly{}, err
		}

		divisor := gfPolyMultiply(denominator,
			newGFPolyMonomial(coefficient, degree))

		remainder = gfPolyAdd(remainder, divisor)
	}

	return remainder.normalised(), nil
}

func gfPolyAdd(a, b gfPoly) gfPoly {
	numATerms := a.numTerms()
	numBTerms := b.numTerms()

	numTerms := numATerms
	if numBTerms > numTerms {
		numTerms = numBTerms
	}

	result := gfPoly{term: make([]gfElement, numTerms)}

	for i := 0; i < numTerms; i++ {
		switch {
		case numATerms > i && numBTerms > i:
			result.term[i] = gfAdd(a.term[i], b.term[i])
		case numATerms > i:
			result.term[i] = a.term[i]
		default:
			result.term[i] = b.term[i]
		}
	}

	return result.normalised()
}

func (e gfPoly) normalised() gfPoly {
	numTerms := e.numTerms()
	maxNonzeroTerm := numTerms - 1

	for i := numTerms - 1; i >= 0; i-- {
		if e.term[i] != 0 {
			break
		}

		maxNonzeroTerm = i - 1
	}

	if maxNonzeroTerm < 0 {
		return gfPoly{}
	} else if maxNonzeroTerm < numTerms-1 {
		e.term = e.term[0 : maxNonzeroTerm+1]
	}

	return e
}

func (e gfPoly) equals(other gfPoly) bool {
	var minecPoly *gfPoly

	var maxecPoly *gfPoly

	if e.numTerms() > other.numTerms() {
		minecPoly = &other
		maxecPoly = &e
	} else {
		minecPoly = &e
		maxecPoly = &other
	}

	numMinTerms := minecPoly.numTerms()
	numMaxTerms := maxecPoly.numTerms()

	for i := 0; i < numMinTerms; i++ {
		if e.term[i] != other.term[i] {
			return false
		}
	}

	for i := numMinTerms; i < numMaxTerms; i++ {
		if maxecPoly.term[i] != 0 {
			return false
		}
	}

	return true
}
