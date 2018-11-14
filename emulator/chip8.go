package emulator

const (
	Chip8DisplayW          = 64
	Chip8DisplayH          = 32
	CharacterSpritesOffset = 0x100
	CharacterSpriteBytes   = 5
	ProgramOffset          = 0x200
	Chip8Frequency         = 60 * 8
	OpHistoryNum           = 16
)

type Chip8 struct {
	mem   [4096]uint8    // memory
	pc    uint16         // program counter
	v     [16]uint8      // registers
	i     uint16         // index register
	dt    uint8          // delay timer
	st    uint8          // sound timer
	sp    uint8          // stack pointer
	stack [16]uint16     // stack
	keys  [16]uint8      // keyboards state
	disp  [64 * 32]uint8 // graphics

	ophistory      [OpHistoryNum]string
	ophistoryIndex int
}

var characterSprites = []uint8{
	0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
	0x20, 0x60, 0x20, 0x20, 0x70, // 1
	0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
	0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
	0x90, 0x90, 0xF0, 0x10, 0x10, // 4
	0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
	0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
	0xF0, 0x10, 0x20, 0x40, 0x40, // 7
	0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
	0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
	0xF0, 0x90, 0xF0, 0x90, 0x90, // A
	0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
	0xF0, 0x80, 0x80, 0x80, 0xF0, // C
	0xE0, 0x90, 0x90, 0x90, 0xE0, // D
	0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
	0xF0, 0x80, 0xF0, 0x80, 0x80, // F
}

func newChip8(b []byte) *Chip8 {
	c := &Chip8{}
	c.pc = ProgramOffset
	c.sp = 0x0f

	copy(c.mem[ProgramOffset:], []uint8(b))
	copy(c.mem[CharacterSpritesOffset:], characterSprites)
	return c
}

func (c *Chip8) step() {
	op := c.fetchOpcode()
	c.execOpcode(op)
}

func (c *Chip8) decrementTimer() {
	if c.dt > 0 {
		c.dt--
	}
	if c.st > 0 {
		c.st--
	}
}

func (c *Chip8) fetchOpcode() uint16 {
	op := uint16(c.mem[c.pc])<<8 | uint16(c.mem[c.pc+1])
	c.pc += 2
	return op
}

func (c *Chip8) updateCarryFlag(b bool) {
	if b {
		c.v[0xf] = 1
	} else {
		c.v[0xf] = 0
	}
}

func (c *Chip8) pushStack(v uint16) {
	c.stack[c.sp] = v
	c.sp--
}

func (c *Chip8) popStack() uint16 {
	c.sp++
	return c.stack[c.sp]
}

func (c *Chip8) draw(x, y, n uint8) bool {
	flipped := false
	sm := c.mem[c.i:]
	for iy := uint8(0); iy < n; iy++ {
		for ix := uint8(0); ix < 8; ix++ {
			tx := int(x) + int(ix)
			ty := int(y) + int(iy)
			if tx >= Chip8DisplayW || ty >= Chip8DisplayH {
				continue
			}

			s := c.disp[ty*Chip8DisplayW+tx]
			d := (sm[iy] >> (7 - ix)) & 0x01
			c.disp[ty*Chip8DisplayW+tx] ^= d
			if s == 1 && d == 1 {
				flipped = true
			}
		}
	}
	return flipped
}

func (c *Chip8) pressedAnyKey() uint8 {
	for i, v := range c.keys {
		if v == 1 {
			return uint8(i)
		}
	}
	return 0xff
}
