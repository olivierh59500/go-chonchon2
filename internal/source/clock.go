package source

import (
	"encoding/binary"
	"fmt"
)

type Pose struct {
	X, Y int
	Half bool
}
type Clock struct {
	Tick, ProfileCursor, MessageCursor, GlyphColumn, GlyphCounter, ScrollY, OldY int
	SpriteCursor                                                                 [2]int
	Slice, SliceCue, SliceHold                                                   int
	Paths                                                                        [2][]byte
	Profile, Message, Font, Panel                                                []byte
	CaptionCursor, CaptionHold, CaptionWait, CaptionShift                        int
	Background                                                                   []byte
	Screen                                                                       [32000]byte
	Glyph                                                                        [184]byte
	Ribbon                                                                       [23][320]byte
	Poses                                                                        [2]Pose
	PanelPattern                                                                 []byte
}

func NewClock(bg, font, message, profile, panel, pathA, pathB []byte) (*Clock, error) {
	if len(bg) != 32000 || len(font) < 184 || len(message) < 8 || len(profile) < 12 || len(panel) < 6 {
		return nil, fmt.Errorf("source: incomplete Chonchon 2 data")
	}
	c := &Clock{Background: bg, Font: font, Message: message, Profile: profile, Panel: panel, Paths: [2][]byte{pathA, pathB}, GlyphCounter: 4, ScrollY: 70, CaptionHold: 1, CaptionWait: 100}
	c.Step()
	c.Tick = 0
	return c, nil
}
func (c *Clock) Step() {
	c.Tick++
	// One authored motion sample is consumed on every 50 Hz simulation tick.
	copy(c.Screen[:], c.Background)
	if c.ProfileCursor+6 > len(c.Profile) || int16(binary.BigEndian.Uint16(c.Profile[c.ProfileCursor:])) < 0 {
		c.ProfileCursor = 0
	}
	c.OldY = c.ScrollY
	c.ScrollY = int(binary.BigEndian.Uint16(c.Profile[c.ProfileCursor:])) / 160
	c.ProfileCursor += 6
	if c.GlyphCounter == 4 {
		c.GlyphCounter = 0
		if c.MessageCursor+4 > len(c.Message) {
			c.MessageCursor = 0
		}
		if int16(binary.BigEndian.Uint16(c.Message[c.MessageCursor:])) < 0 {
			c.GlyphCounter = 2
			c.MessageCursor += 4
		}
		off := int(binary.BigEndian.Uint32(c.Message[c.MessageCursor:]))
		c.MessageCursor += 4
		clear(c.Glyph[:])
		if off >= 0 && off+184 <= len(c.Font) {
			copy(c.Glyph[:], c.Font[off:off+184])
		}
	}
	for row := 0; row < 23; row++ {
		copy(c.Ribbon[row][:312], c.Ribbon[row][8:])
		for x := 0; x < 8; x++ {
			var index byte
			for plane := 0; plane < 2; plane++ {
				at := row*8 + c.GlyphColumn/2*4 + plane*2 + c.GlyphColumn%2
				index |= (c.Glyph[at] >> uint(7-x) & 1) << plane
			}
			c.Ribbon[row][312+x] = index
		}
		y := c.ScrollY + row
		if y >= 0 && y < 200 {
			for x, v := range c.Ribbon[row] {
				c.pixel(x, y, v)
			}
		}
	}
	c.GlyphColumn = (c.GlyphColumn + 1) % 4
	c.GlyphCounter++
	for i := range c.Poses {
		p := c.SpriteCursor[i]
		if p+4 > len(c.Paths[i]) || int16(binary.BigEndian.Uint16(c.Paths[i][p:])) < 0 {
			p = 0
		}
		off := int(binary.BigEndian.Uint16(c.Paths[i][p:]))
		c.Poses[i] = Pose{X: off%160/8*16 + off%8/4*8, Y: off / 160, Half: int16(binary.BigEndian.Uint16(c.Paths[i][p+2:])) < 0}
		c.SpriteCursor[i] = p + 4
	}
	if len(c.PanelPattern) >= 69*160 && c.CaptionCursor+4 < len(c.Panel) {
		y := int(binary.BigEndian.Uint16(c.Panel[c.CaptionCursor:]))
		rows := int(binary.BigEndian.Uint16(c.Panel[c.CaptionCursor+2:]))
		for row := 0; row <= rows; row++ {
			yy := y + row
			if yy < 0 || yy >= 69 {
				continue
			}
			src := yy*160 + c.CaptionShift*16
			dst := yy*160 + 0x200
			for k := 0; k < 16; k++ {
				if src+k < len(c.PanelPattern) && dst+k < len(c.Screen) {
					c.Screen[dst+k] = c.PanelPattern[src+k]
				}
				if src+k < len(c.PanelPattern) && dst+80+k < len(c.Screen) {
					c.Screen[dst+80+k] = c.PanelPattern[src+k]
				}
			}
		}
	}
	c.CaptionWait--
	if c.CaptionWait <= 0 {
		c.CaptionShift = (c.CaptionShift + 1) % 10
		if c.CaptionShift == 0 {
			c.CaptionHold--
			if c.CaptionHold < 0 {
				c.CaptionCursor += 6
				if c.CaptionCursor+6 > len(c.Panel) || int16(binary.BigEndian.Uint16(c.Panel[c.CaptionCursor:])) < 0 {
					c.CaptionCursor = 0
				}
				c.CaptionHold = int(binary.BigEndian.Uint16(c.Panel[c.CaptionCursor+4:]))
			}
		}
		c.CaptionWait = 2
	}
}
func (c *Clock) pixel(x, y int, v byte) {
	at := y*160 + x/16*8
	mask := uint16(1 << uint(15-x%16))
	for p := 0; p < 4; p++ {
		w := binary.BigEndian.Uint16(c.Screen[at+p*2:])
		w = w &^ mask
		if v>>uint(p)&1 != 0 {
			w |= mask
		}
		binary.BigEndian.PutUint16(c.Screen[at+p*2:], w)
	}
}
func (c *Clock) Pixels(dst []byte) {
	// Load each planar word once for all sixteen pixels in its group.
	for group := 0; group < 4000; group++ {
		at := group * 8
		p0 := binary.BigEndian.Uint16(c.Screen[at:])
		p1 := binary.BigEndian.Uint16(c.Screen[at+2:])
		p2 := binary.BigEndian.Uint16(c.Screen[at+4:])
		p3 := binary.BigEndian.Uint16(c.Screen[at+6:])
		for x := 0; x < 16; x++ {
			shift := uint(15 - x)
			v := byte(p0>>shift&1 | (p1>>shift&1)<<1 | (p2>>shift&1)<<2 | (p3>>shift&1)<<3)
			i := (group*16 + x) * 4
			dst[i], dst[i+1], dst[i+2], dst[i+3] = v*17, 0, 0, 255
		}
	}
}
