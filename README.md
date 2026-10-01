# Chonchon 2 Go

A Go/Ebitengine port of an Atari ST intro by **Chon-Chon of DMA**, using
Demo Construction Kit **v1.0.13**. Native artwork and authored motion tables
supply the screen's graphic resources.

## Run

```sh
go run ./cmd/chonchon2
```

Space or Escape closes the intro. The logical canvas is 320 × 200 pixels.
Simulation runs at 50 Hz independently of the display's refresh rate.

The scene combines the DMA backdrop, moving panels and greetings, two animated
bitmap objects, a colored scrolling ribbon, moving raster palettes and three
horizontal YM meters. The text, vertical profile and sprite positions retain
the native parameter tables. F1–F7 select the replacement YM tracks; a screen
tap selects the next track on Android. The seven tunes are arranged by Mad Max
(Jochen Hippel), and their full credits remain embedded in each YM file.

The transport test checks its message pointer, glyph counter and vertical
profile against an original 68000 runtime checkpoint. Bounds tests run all
motion clocks for 12,000 updates. The replacement playlist means music-driven
pixel output is not claimed to match the original replay exactly.

## Android

```sh
./scripts/run-android.sh --build-only
```

This builds an ARM64 APK with Ebitengine 2.9.11, Android SDK 36, NDK 28.2,
Java 17 and the included Gradle wrapper. The APK is
`android/app/build/outputs/apk/debug/app-debug.apk` and installs as **Chonchon 2**.
Without `--build-only`, the script installs and launches it on one authorized
USB device. The host preserves landscape orientation, screen wakefulness and
activity suspension; audio starts after the Android context is ready.

## Video and checks

```sh
go test ./...
go vet ./...
go run ./cmd/video
```

Video export writes a three-minute 50 fps H.264/AAC MP4, a PNG poster and a JSON
report under `recordings/`. DCK records only the game canvas and its own music,
using a shared simulation clock. Duration and poster time are configurable.
The canvas is enlarged by an integer factor of two for 640 × 400 output.

The original references, private analysis files, recordings and Android build
outputs remain local. Only presentation resources are embedded in the Go app.
