// Package source reads the original intro's data and animation tables.
package source

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// UnpackPRG reproduces the executable's LSD backward byte-bitstream decoder.
func UnpackLSD(prg []byte) ([]byte, error) {
	start := bytes.Index(prg, []byte("LSD!"))
	if start < 0 || start+12 > len(prg) {
		return nil, fmt.Errorf("source: missing packed data header")
	}
	size, packed := int(binary.BigEndian.Uint32(prg[start+4:])), int(binary.BigEndian.Uint32(prg[start+8:]))
	if size < 28 || size > 16<<20 || packed < 14 || start+packed+4 > len(prg) {
		return nil, fmt.Errorf("source: invalid packed data bounds")
	}
	input := start + packed + 4
	input -= 2
	if int16(binary.BigEndian.Uint16(prg[input:])) < 0 {
		input--
	}
	input--
	word := prg[input]
	var fault error
	readByte := func() byte {
		input--
		if input < start+12 {
			fault = fmt.Errorf("source: bitstream underflow")
			return 0
		}
		return prg[input]
	}
	bit := func() int {
		carry := int(word >> 7)
		word <<= 1
		if word == 0 {
			next := readByte()
			word = next<<1 | byte(carry)
			carry = int(next >> 7)
		}
		return carry
	}
	bits := func(n int) int {
		v := 0
		for range n {
			v = v<<1 | bit()
		}
		return v
	}
	out := make([]byte, size)
	position := size
	literalWidths := [4]int{10, 3, 2, 2}
	literalBases := [4]int{14, 7, 4, 1}
	lengthWidths := [5]int{10, 2, 1, 0, 0}
	lengthBases := [5]int{10, 6, 4, 3, 2}
	distanceWidths := [3]int{11, 4, 7}
	distanceBases := [3]int{288, 0, 32}
	for position > 0 {
		if bit() != 0 {
			count := 1
			if bit() != 0 {
				for group := 3; group >= 0; group-- {
					value := bits(literalWidths[group])
					if group == 0 || value != (1<<literalWidths[group])-1 {
						count = value + literalBases[group] + 1
						break
					}
				}
			}
			if count > position {
				return nil, fmt.Errorf("source: literal run exceeds output")
			}
			for range count {
				position--
				out[position] = readByte()
			}
		}
		if fault != nil {
			return nil, fault
		}
		if input <= start+12 {
			break
		}
		group := 3
		for group >= 0 && bit() != 0 {
			group--
		}
		group++
		length := bits(lengthWidths[group]) + lengthBases[group]
		distance := 0
		if length == 2 {
			if bit() == 0 {
				distance = bits(6)
			} else {
				distance = bits(9) + 64
			}
		} else {
			g := 1
			for g >= 0 && bit() != 0 {
				g--
			}
			g++
			distance = bits(distanceWidths[g]+1) + distanceBases[g]
		}
		distance += length
		if length > position {
			return nil, fmt.Errorf("source: match exceeds output")
		}
		for range length {
			position--
			from := position + distance
			if from < 0 || from >= len(out) {
				return nil, fmt.Errorf("source: invalid match distance")
			}
			out[position] = out[from]
		}
		if fault != nil {
			return nil, fault
		}
	}
	if position != 0 || binary.BigEndian.Uint16(out) != 0x601a {
		return nil, fmt.Errorf("source: incomplete native executable at %d", position)
	}
	return out, nil
}

// UnpackPRG resolves both native packing layers before asset extraction.
func UnpackPRG(prg []byte) ([]byte, error) {
	inner, err := UnpackLSD(prg)
	if err != nil {
		return nil, err
	}
	return UnpackHuffmanPRG(inner)
}
