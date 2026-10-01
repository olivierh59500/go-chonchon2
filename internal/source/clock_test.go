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
	for range 100 {
		c.Step()
	}
	if c.ProfileCursor != 306 || c.MessageCursor != 88 || c.GlyphCounter != 3 || c.ScrollY != 102 {
		t.Fatalf("native transport differs: profile=%d message=%d column=%d y=%d", c.ProfileCursor, c.MessageCursor, c.GlyphCounter, c.ScrollY)
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
