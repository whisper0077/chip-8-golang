package emulator

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"

	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	VBlankFrequency = 60
	DisplayScale    = 10
	EmulatorW       = Chip8DisplayW * DisplayScale
	EmulatorH       = Chip8DisplayH * DisplayScale
	WindowW         = EmulatorW
	WindowH         = EmulatorH + 256
	InformationH    = WindowH - EmulatorH
	FontSize        = 16
	FontPerW        = 32
	AudioSamples    = 64
)

type Emulator struct {
	rom      []byte
	chip8    *Chip8
	renderer *sdl.Renderer
	audio    sdl.AudioDeviceID
	font     *sdl.Texture
	running  bool
	focus    bool
	stepMode bool
}

var scanCode2Key = map[int]byte{
	sdl.SCANCODE_4: 0x1,
	sdl.SCANCODE_5: 0x2,
	sdl.SCANCODE_6: 0x3,
	sdl.SCANCODE_7: 0xc,
	sdl.SCANCODE_R: 0x4,
	sdl.SCANCODE_T: 0x5,
	sdl.SCANCODE_Y: 0x6,
	sdl.SCANCODE_U: 0xd,
	sdl.SCANCODE_F: 0x7,
	sdl.SCANCODE_G: 0x8,
	sdl.SCANCODE_H: 0x9,
	sdl.SCANCODE_J: 0xe,
	sdl.SCANCODE_V: 0xa,
	sdl.SCANCODE_B: 0x0,
	sdl.SCANCODE_N: 0xb,
	sdl.SCANCODE_M: 0xf,
}

func checkError(s string, e error) {
	if e != nil {
		log.Fatalf(s, e)
	}
}

func initRenderer() *sdl.Renderer {
	window, err := sdl.CreateWindow("Chip-8 Emulator", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, WindowW, WindowH, sdl.WINDOW_SHOWN)
	checkError("CreateWindow", err)

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_PRESENTVSYNC)
	checkError("CreateRenderer", err)

	// workaround for https://bugzilla.libsdl.org/show_bug.cgi?id=4272
	// 	or update sdl2 to 2.0.9
	window.Hide()
	sdl.PumpEvents()
	window.Show()

	return renderer
}

func initAudio() sdl.AudioDeviceID {
	want := &sdl.AudioSpec{
		Freq:     AudioSamples * VBlankFrequency,
		Format:   sdl.AUDIO_F32LSB,
		Channels: 1,
		Samples:  AudioSamples,
	}
	have := &sdl.AudioSpec{}
	audio, err := sdl.OpenAudioDevice("", false, want, have, sdl.AUDIO_ALLOW_ANY_CHANGE)
	checkError("OpenAudioDevice", err)

	sdl.PauseAudioDevice(audio, false)
	return audio
}

func initFont(r *sdl.Renderer) *sdl.Texture {
	surface, err := img.Load("image/font.png")
	checkError("Load", err)
	defer surface.Free()

	texture, err := r.CreateTextureFromSurface(surface)
	checkError("CreateTextureFromSurface", err)

	texture.SetBlendMode(sdl.BLENDMODE_ADD)

	return texture
}

func NewEmulator(b []byte, sm bool) *Emulator {
	err := sdl.Init(sdl.INIT_EVERYTHING)
	checkError("sdl.Init", err)

	renderer := initRenderer()
	audio := initAudio()
	font := initFont(renderer)

	return &Emulator{rom: b, chip8: newChip8(b), renderer: renderer, audio: audio, font: font, running: true, focus: true, stepMode: sm}
}

func (e *Emulator) Run() {
	perVblankCycle := Chip8Frequency / VBlankFrequency
	cycle := 0

	for e.running {
		cycle++
		if e.focus && !e.stepMode {
			e.chip8.step()
		}

		if cycle > perVblankCycle {
			cycle = 0
			e.draw()

			if e.focus {
				e.updateSound()
				e.chip8.decrementTimer()
			}
		}

		e.pollEvents()
	}
}

func (e *Emulator) draw() {
	e.renderer.SetDrawColor(0, 0, 0, 255)
	e.renderer.Clear()

	// chip8 display
	e.renderer.SetDrawColor(0, 255, 0, 255)
	for y := int32(0); y < Chip8DisplayH; y++ {
		for x := int32(0); x < Chip8DisplayW; x++ {
			if e.chip8.disp[y*Chip8DisplayW+x] != 0 {
				e.renderer.FillRect(&sdl.Rect{X: x * DisplayScale, Y: y * DisplayScale, W: DisplayScale, H: DisplayScale})
			}
		}
	}

	e.drawDebugInfo()

	e.renderer.Present()
}

func (e *Emulator) pollEvents() {
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch ev := event.(type) {
		case *sdl.QuitEvent:
			e.running = false
		case *sdl.KeyboardEvent:
			switch ev.Type {
			case sdl.KEYDOWN:
				if i, ok := scanCode2Key[int(ev.Keysym.Scancode)]; ok {
					e.chip8.keys[i] = 1
				} else {
					if ev.Keysym.Scancode == sdl.SCANCODE_SPACE {
						if e.stepMode {
							e.chip8.step()
						} else {
							e.stepMode = true
						}
					} else if ev.Keysym.Scancode == sdl.SCANCODE_RETURN {
						if e.stepMode {
							e.stepMode = false
						}
					} else if ev.Keysym.Scancode == sdl.SCANCODE_Z {
						e.chip8 = newChip8(e.rom)
					}
				}
			case sdl.KEYUP:
				if i, ok := scanCode2Key[int(ev.Keysym.Scancode)]; ok {
					e.chip8.keys[i] = 0
				}
			}
		case *sdl.WindowEvent:
			switch ev.Event {
			case sdl.WINDOWEVENT_FOCUS_LOST:
				e.focus = false
			case sdl.WINDOWEVENT_FOCUS_GAINED:
				e.focus = true
			}
		}
	}
}

func (e *Emulator) updateSound() {
	if e.chip8.st > 0 {
		samples := make([]byte, 4*AudioSamples)
		for i := 0; i < len(samples); i += 4 {
			// sin wave
			f := 2.0 * math.Pi / 180.0 * float64(360*i/AudioSamples)
			f = math.Sin(f)
			binary.LittleEndian.PutUint32(samples[i:], math.Float32bits(float32(f)))
		}

		err := sdl.QueueAudio(e.audio, samples)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (e *Emulator) drawDebugInfo() {
	e.renderer.SetDrawColor(32, 32, 32, 255)
	e.renderer.FillRect(&sdl.Rect{X: 0, Y: EmulatorH, W: EmulatorW, H: InformationH})

	// draw opcodes history
	offsetX := 0
	for i := 0; i < OpHistoryNum; i++ {
		e.drawText(e.chip8.ophistory[(e.chip8.ophistoryIndex+i)%OpHistoryNum], 0, EmulatorH+i*FontSize)
	}

	// draw v registers
	offsetX = EmulatorW/2 + 48
	for i, v := range e.chip8.v {
		e.drawText(fmt.Sprintf("V%X = %02X", i, v), offsetX, EmulatorH+i*FontSize)
	}

	// draw other registers
	offsetX = EmulatorW - FontSize*9
	e.drawText(fmt.Sprintf("DT = %02X", e.chip8.dt), offsetX, EmulatorH+FontSize*0)
	e.drawText(fmt.Sprintf("ST = %02X", e.chip8.st), offsetX, EmulatorH+FontSize*1)
	e.drawText(fmt.Sprintf("SP = %02X", e.chip8.sp), offsetX, EmulatorH+FontSize*2)
	e.drawText(fmt.Sprintf(" I = %04X", e.chip8.i), offsetX, EmulatorH+FontSize*3)

	// draw key inputs
	keys := e.chip8.keys
	e.drawText(fmt.Sprintf("KEYS %d%d%d%d", keys[0x01], keys[0x02], keys[0x03], keys[0x0c]), offsetX, EmulatorH+FontSize*5)
	e.drawText(fmt.Sprintf("     %d%d%d%d", keys[0x04], keys[0x05], keys[0x06], keys[0x0d]), offsetX, EmulatorH+FontSize*6)
	e.drawText(fmt.Sprintf("     %d%d%d%d", keys[0x07], keys[0x08], keys[0x09], keys[0x0e]), offsetX, EmulatorH+FontSize*7)
	e.drawText(fmt.Sprintf("     %d%d%d%d", keys[0x0a], keys[0x00], keys[0x0b], keys[0x0f]), offsetX, EmulatorH+FontSize*8)
}

func (e *Emulator) drawText(s string, x, y int) {
	for i, v := range []byte(s) {
		v -= byte(' ')
		fx := FontSize * (int32(v) % FontPerW)
		fy := FontSize * (int32(v) / FontPerW)
		e.renderer.Copy(e.font,
			&sdl.Rect{X: fx, Y: fy, W: FontSize, H: FontSize},
			&sdl.Rect{X: int32(x + i*FontSize), Y: int32(y), W: FontSize, H: FontSize})
	}
}
