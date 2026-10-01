// Package demo composes the intro's authored motion through DCK.
package demo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/sound"
	playback "github.com/olivierh59500/democonstructionkit/sound/ebiten"
	"github.com/olivierh59500/democonstructionkit/sprites"
	"github.com/olivierh59500/go-chonchon2/assets"
	"github.com/olivierh59500/go-chonchon2/internal/source"
	"image"
	"image/color"
	_ "image/png"
	"io"
)

const Width, Height, FPS = 320, 200, 50

type Game struct {
	clock                                                     *source.Clock
	image, stage                                              *ebiten.Image
	sprites                                                   [2]*ebiten.Image
	lookup                                                    *composite.IndexedPalette
	slots                                                     *sprites.ImageSlots
	player                                                    *playback.Player
	visual                                                    *sound.Stream
	pixels                                                    []byte
	pcm                                                       [960 * 8]byte
	volumes                                                   [3]byte
	closed                                                    bool
	shader                                                    *ebiten.Shader
	colors                                                    [200 * 4]float32
	foreground                                                [200 * 4]float32
	paletteWords                                              []byte
	baseRaster, rasterProgram, foregroundBase, scrollMaterial []byte
	uniforms                                                  map[string]any
	mute                                                      bool
	track                                                     int
}

func data(n string) ([]byte, error) { return assets.Files.ReadFile("original/" + n) }
func NewGame(mute bool) (_ *Game, err error) {
	g := &Game{pixels: make([]byte, Width*Height*4), mute: mute}
	defer func() {
		if err != nil {
			g.Close()
		}
	}()
	read := func(n string) []byte {
		if err != nil {
			return nil
		}
		var b []byte
		b, err = data(n)
		return b
	}
	bg, font, msg, profile, panel, pa, pb := read("background.bin"), read("font.bin"), read("message.bin"), read("scroll-profile.bin"), read("panel-script.bin"), read("sprite-a-path.bin"), read("sprite-b-path.bin")
	if err != nil {
		return nil, err
	}
	g.clock, err = source.NewClock(bg, font, msg, profile, panel, pa, pb)
	if err != nil {
		return nil, err
	}
	g.clock.PanelPattern = read("panel-pattern.bin")
	if err != nil {
		return nil, err
	}
	load := func(n string) (*ebiten.Image, error) {
		b, e := data(n)
		if e != nil {
			return nil, e
		}
		im, _, e := image.Decode(bytes.NewReader(b))
		if e != nil {
			return nil, e
		}
		return ebiten.NewImageFromImage(im), nil
	}
	for i, n := range []string{"sprite-a.png", "sprite-b.png"} {
		g.sprites[i], err = load(n)
		if err != nil {
			return nil, err
		}
	}
	g.image = ebiten.NewImage(Width, Height)
	g.stage = ebiten.NewImage(Width, Height)
	g.slots, err = sprites.NewImageSlots(sprites.ImageSlotsConfig{Images: g.sprites[:], MaxSlots: 34})
	if err != nil {
		return nil, err
	}
	pal := read("palette.bin")
	if err != nil {
		return nil, err
	}
	colors := make([]color.NRGBA, 16)
	for i := range colors {
		w := binary.BigEndian.Uint16(pal[i*2:])
		colors[i] = color.NRGBA{R: byte(w>>8&7) * 34, G: byte(w>>4&7) * 34, B: byte(w&7) * 34, A: 255}
	}
	g.lookup, err = composite.NewIndexedPalette(composite.IndexedPaletteConfig{Palette: colors, Channel: composite.BitplaneRed, Blend: ebiten.BlendCopy})
	if err != nil {
		return nil, err
	}
	if err = g.SelectMusic(0); err != nil {
		return nil, err
	}
	g.baseRaster = read("raster-background.bin")
	g.rasterProgram = read("raster-program.bin")
	g.foregroundBase = read("raster-base.bin")
	g.scrollMaterial = read("raster-scroll.bin")
	if err != nil {
		return nil, err
	}
	g.shader, err = ebiten.NewShader([]byte(rasterShader))
	if err != nil {
		return nil, err
	}
	g.uniforms = map[string]any{"Background": g.colors[:], "Foreground": g.foreground[:]}
	return g, nil
}
func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return ebiten.Termination
	}
	for i, key := range []ebiten.Key{ebiten.KeyF1, ebiten.KeyF2, ebiten.KeyF3, ebiten.KeyF4, ebiten.KeyF5, ebiten.KeyF6, ebiten.KeyF7} {
		if inpututil.IsKeyJustPressed(key) {
			if e := g.SelectMusic(i); e != nil {
				return e
			}
		}
	}
	if len(inpututil.AppendJustPressedTouchIDs(nil)) > 0 {
		if e := g.SelectMusic((g.track + 1) % 7); e != nil {
			return e
		}
	}
	g.clock.Step()
	g.updateRasters()
	if _, e := io.ReadFull(g.visual, g.pcm[:]); e != nil {
		return e
	}
	if r, ok := g.visual.YMRegisters(); ok {
		for i := range g.volumes {
			g.volumes[i] = r[8+i] & 15
		}
	}
	for channel, volume := range g.volumes {
		for row := 0; row < 7; row++ {
			for cell := 0; cell <= int(volume); cell++ {
				off := 0x6868 + channel*0x5a0 + row*160 + cell*8
				if off+4 <= 32000 {
					binary.BigEndian.PutUint32(g.clock.Screen[off:], 0x0000ffff)
				}
			}
		}
	}
	g.clock.Pixels(g.pixels)
	g.image.WritePixels(g.pixels)
	g.stage.DrawImage(g.image, &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy})
	for i, p := range g.clock.Poses {
		var slots [34]sprites.ImageSlot
		for row := range slots {
			sourceY := row
			if i == 1 && row >= 2 && row < 32 {
				sourceY = 2 + (g.clock.Tick/2+row-2)%192
			}
			slots[row] = sprites.ImageSlot{Image: i, Source: image.Rect(0, sourceY, 80, sourceY+1), X: float64(p.X), Y: float64(p.Y + row)}
		}
		if e := g.slots.SetSlots(slots[:]); e != nil {
			return e
		}
		g.slots.Draw(g.stage)
	}
	return nil
}
func (g *Game) Draw(dst *ebiten.Image) {
	_ = g.lookup.Draw(g.image, g.stage)
	op := ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{g.stage, g.image}, Uniforms: g.uniforms, Blend: ebiten.BlendCopy}
	dst.DrawRectShader(Width, Height, g.shader, &op)
}
func (*Game) Layout(int, int) (int, int) { return Width, Height }
func (g *Game) Tick() int                { return g.clock.Tick }
func (g *Game) Close() {
	if g == nil || g.closed {
		return
	}
	g.closed = true
	if g.player != nil {
		g.player.Close()
	}
	if g.visual != nil {
		g.visual.Close()
	}
	if g.slots != nil {
		g.slots.Close()
	}
	if g.lookup != nil {
		g.lookup.Close()
	}
	for _, im := range append(g.sprites[:], g.image, g.stage) {
		if im != nil {
			im.Deallocate()
		}
	}
}

func (g *Game) updateRasters() {
	var background [400]byte
	copy(background[:], g.baseRaster)
	count := len(g.rasterProgram) / 180
	frame := g.clock.Tick
	if frame >= count {
		frame = 25 + (frame-count)%max(1, count-25)
	}
	copy(background[136:316], g.rasterProgram[frame*180:frame*180+180])
	var fg [400]byte
	copy(fg[:], g.foregroundBase)
	at := g.clock.OldY * 2
	if at >= 0 && at+len(g.scrollMaterial) <= 400 {
		copy(fg[at:], g.scrollMaterial)
	}
	for y := 0; y < 200; y++ {
		for i, b := range [][]byte{background[:], fg[:]} {
			w := binary.BigEndian.Uint16(b[y*2:])
			dst := g.colors[y*4:]
			if i == 1 {
				dst = g.foreground[y*4:]
			}
			dst[0], dst[1], dst[2], dst[3] = float32(w>>8&7)*34/255, float32(w>>4&7)*34/255, float32(w&7)*34/255, 1
		}
	}
}

const rasterShader = `//kage:unit pixels
package main
var Background [200]vec4
var Foreground [200]vec4
func Fragment(position vec4,source vec2,color vec4)vec4{p:=source-imageSrc0Origin();i:=int(clamp(floor(imageSrc0At(source).r*15+0.5),0,15));y:=int(clamp(p.y,0,199));if i==0{return Background[y]*color};if i==2{return Foreground[y]*color};return imageSrc1At(source)*color}
`

func (g *Game) SelectMusic(track int) error {
	if track < 0 || track >= 7 {
		return fmt.Errorf("invalid track")
	}
	b, e := data(fmt.Sprintf("music-%d.ym", track+1))
	if e != nil {
		return e
	}
	stream, e := sound.Open("music.ym", b, sound.Options{SampleRate: 48000, BlockFrames: 960, Loop: true})
	if e != nil {
		return e
	}
	var player *playback.Player
	if !g.mute {
		player, e = playback.Open(nil, "music.ym", b, sound.Options{SampleRate: 48000, Loop: true})
		if e != nil {
			stream.Close()
			return e
		}
	}
	if g.visual != nil {
		g.visual.Close()
	}
	if g.player != nil {
		g.player.Close()
	}
	g.visual, g.player, g.track = stream, player, track
	if player != nil {
		player.Play()
	}
	return nil
}
