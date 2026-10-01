package source

import (
	"github.com/olivierh59500/go-chonchon2/assets"
	"testing"
)

func nativeClock(t *testing.T) *Clock {
	t.Helper()
	rd := func(n string) []byte {
		b, e := assets.Files.ReadFile("original/" + n)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	c, e := NewClock(rd("background.bin"), rd("font.bin"), rd("message.bin"), rd("scroll-profile.bin"), rd("panel-script.bin"), rd("sprite-a-path.bin"), rd("sprite-b-path.bin"))
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestNativeTransportCheckpoint(t *testing.T) {
	c := nativeClock(t)
	for range 50 {
		c.Step()
	}
	if c.ProfileCursor != 306 || c.MessageCursor != 88 || c.GlyphCounter != 3 || c.ScrollY != 102 {
		t.Fatalf("native transport differs: profile=%d message=%d column=%d y=%d", c.ProfileCursor, c.MessageCursor, c.GlyphCounter, c.ScrollY)
	}
}

func TestMotionAdvancesEverySimulationTick(t *testing.T) {
	c := nativeClock(t)
	for tick := 1; tick <= 50; tick++ {
		before := c.ProfileCursor
		c.Step()
		if c.Tick != tick || c.ProfileCursor != before+6 {
			t.Fatalf("motion skipped at tick %d: cursor %d -> %d", tick, before, c.ProfileCursor)
		}
	}
}

func TestPlanarPixels(t *testing.T) {
	c := nativeClock(t)
	for i := range c.Screen {
		c.Screen[i] = byte(i*53 + 17)
	}
	pixels := make([]byte, 320*200*4)
	c.Pixels(pixels)
	for pixel := 0; pixel < 320*200; pixel++ {
		var index byte
		for plane := 0; plane < 4; plane++ {
			at := pixel/16*8 + plane*2
			word := uint16(c.Screen[at])<<8 | uint16(c.Screen[at+1])
			index |= byte(word>>uint(15-pixel%16)&1) << plane
		}
		at := pixel * 4
		if pixels[at] != index*17 || pixels[at+1] != 0 || pixels[at+2] != 0 || pixels[at+3] != 255 {
			t.Fatalf("incorrect planar conversion at pixel %d", pixel)
		}
	}
}
func TestNativeLoopBounds(t *testing.T) {
	c := nativeClock(t)
	for range 12000 {
		c.Step()
		if c.ScrollY < 0 || c.ScrollY > 200 {
			t.Fatal("scroll profile exceeds native canvas")
		}
		for _, p := range c.Poses {
			if p.X < 0 || p.X > 319 || p.Y < 0 || p.Y > 199 {
				t.Fatal("sprite pose exceeds native path bounds")
			}
		}
	}
}
