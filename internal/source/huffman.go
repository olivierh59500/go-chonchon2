// Package source decodes the original intro's presentation data.
package source

import (
	"encoding/binary"
	"fmt"
)

type bitReader struct {
	data     []byte
	position int
}

func (r *bitReader) bit() (int, error) {
	if r.position >= len(r.data)*8 {
		return 0, fmt.Errorf("source: truncated Huffman stream")
	}
	p := r.position
	r.position++
	return int(r.data[p/8] >> uint(7-p%8) & 1), nil
}

type node struct {
	left, right *node
	value       byte
}

// UnpackPRG reproduces the wrapper's static Huffman tree and three RLE escapes.
func UnpackHuffmanPRG(prg []byte) ([]byte, error) {
	if len(prg) < 28+0x548 || binary.BigEndian.Uint16(prg) != 0x601a {
		return nil, fmt.Errorf("source: invalid packed executable")
	}
	b := prg[28:]
	text, data := int(binary.BigEndian.Uint32(prg[2:])), int(binary.BigEndian.Uint32(prg[6:]))
	if (text != 0x548 && text != 0x854) || text+data > len(b) {
		return nil, fmt.Errorf("source: invalid packed bounds")
	}
	treeStart, treeEnd, sizeOffset, limitOffset, markerOffset := 0x2a8, 0x438, 0x292, 0x28e, 0x166
	if text == 0x854 {
		treeStart, treeEnd, sizeOffset, limitOffset, markerOffset = 0x572, 0x748, 0x55c, 0x558, 0x2e0
	}
	decodedSize := int(binary.BigEndian.Uint32(b[sizeOffset:]))
	limit := int(binary.BigEndian.Uint32(b[limitOffset:]))
	if decodedSize < 28 || decodedSize > 16<<20 || limit < 28 || limit > 16<<20 {
		return nil, fmt.Errorf("source: invalid decoded size")
	}
	bits := &bitReader{data: b[treeStart:treeEnd]}
	nodes := 0
	var tree func(int) (*node, error)
	tree = func(depth int) (*node, error) {
		nodes++
		if depth > 32 || nodes > 511 {
			return nil, fmt.Errorf("source: invalid Huffman tree")
		}
		branch, e := bits.bit()
		if e != nil {
			return nil, e
		}
		n := &node{}
		if branch == 0 {
			for range 8 {
				v, e := bits.bit()
				if e != nil {
					return nil, e
				}
				n.value = n.value<<1 | byte(v)
			}
		} else {
			n.left, e = tree(depth + 1)
			if e != nil {
				return nil, e
			}
			n.right, e = tree(depth + 1)
			if e != nil {
				return nil, e
			}
		}
		return n, nil
	}
	root, e := tree(0)
	if e != nil {
		return nil, e
	}
	compressed := append([]byte(nil), b[text:text+data]...)
	compressed = append(compressed, 0, 0, 0)
	bits = &bitReader{data: compressed}
	decoded := make([]byte, decodedSize)
	for i := range decoded {
		n := root
		for n.left != nil {
			v, e := bits.bit()
			if e != nil {
				return nil, e
			}
			if v == 0 {
				n = n.left
			} else {
				n = n.right
			}
		}
		decoded[i] = n.value
	}
	out := make([]byte, 0, limit)
	markers := [3]byte{b[markerOffset+1], b[markerOffset+3], b[markerOffset+5]}
	at := 0
	appendRun := func(pattern []byte, count int) error {
		if len(out)+len(pattern)*count > limit+4096 {
			return fmt.Errorf("source: RLE output exceeds native allocation")
		}
		for range count {
			out = append(out, pattern...)
		}
		return nil
	}
	for at < len(decoded) {
		v := decoded[at]
		at++
		switch v {
		case markers[0]:
			if at+2 > len(decoded) {
				return nil, fmt.Errorf("source: truncated byte run")
			}
			value, count := decoded[at], int(decoded[at+1])
			at += 2
			if count == 0 {
				count = 65536
			}
			if e = appendRun([]byte{value}, count); e != nil {
				return nil, e
			}
		case markers[1]:
			if at >= len(decoded) {
				return nil, fmt.Errorf("source: truncated zero run")
			}
			count := int(decoded[at])
			at++
			if count == 0 {
				count = 65536
			}
			if e = appendRun([]byte{b[markerOffset+7]}, count); e != nil {
				return nil, e
			}
		case markers[2]:
			if at+2 > len(decoded) {
				return nil, fmt.Errorf("source: truncated pattern run")
			}
			width, count := int(decoded[at]), int(decoded[at+1])
			at += 2
			if width == 0 {
				width = 256
			}
			if count == 0 {
				count = 256
			}
			if at+width > len(decoded) {
				return nil, fmt.Errorf("source: incomplete repeated pattern")
			}
			if e = appendRun(decoded[at:at+width], count); e != nil {
				return nil, e
			}
			at += width
		default:
			if e = appendRun([]byte{v}, 1); e != nil {
				return nil, e
			}
		}
	}
	if len(out) < 28 || binary.BigEndian.Uint16(out) != 0x601a {
		return nil, fmt.Errorf("source: missing decoded TOS header")
	}
	return out, nil
}
