package emulator

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// memory range is 0x200 - 0x300
var opcodeTestTable = []struct {
	opcode uint16
	before func(c *Chip8)
	assert func(t *testing.T, c *Chip8)
}{
	// clear display
	{
		0x00E0,
		func(c *Chip8) {
			for i := range c.disp {
				c.disp[i] = 1
			}
		},
		func(t *testing.T, c *Chip8) {
			for _, v := range c.disp {
				if assert.Equal(t, v, uint8(0)) {
					return
				}
			}
		},
	},
	// ret
	{
		0x00EE,
		func(c *Chip8) {
			c.stack[0xf] = 0x300
			c.sp = 0xf - 1
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.sp, uint8(0xf))
			assert.Equal(t, c.pc, uint16(0x300))
		},
	},
	// goto 0x0NNN
	{
		0x1234,
		nil,
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x234))
		},
	},
	// call 0x0NNN
	{
		0x2208,
		nil,
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x208))
			assert.Equal(t, c.sp, uint8(0xf-1))
			assert.Equal(t, c.stack[0xf], uint16(0x202))
		},
	},
	// 0x3XNN if(Vx==NN) [true]
	{
		0x3012,
		func(c *Chip8) {
			c.v[0] = 0x12
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x204))
		},
	},
	// 0x3XNN if(Vx==NN) [false]
	{
		0x3012,
		func(c *Chip8) {
			c.v[0] = 0x1
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x202))
		},
	},
	// 0x4XNN if(Vx!=NN) [true]
	{
		0x4012,
		func(c *Chip8) {
			c.v[0] = 0x1
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x204))
		},
	},
	// 0x4XNN if(Vx!=NN) [false]
	{
		0x4012,
		func(c *Chip8) {
			c.v[0] = 0x12
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x202))
		},
	},
	// 0x5XY0 if(Vx==Vy) [true]
	{
		0x5120,
		func(c *Chip8) {
			c.v[1] = 0x1
			c.v[2] = 0x1
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x204))
		},
	},
	// 0x5XY0 if(Vx==Vy) [false]
	{
		0x5120,
		func(c *Chip8) {
			c.v[1] = 0x1
			c.v[2] = 0x2
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x202))
		},
	},
	// 6XNN Vx = NN
	{
		0x6355,
		nil,
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[3], uint8(0x55))
		},
	},
	// 7XNN Vx += NN (Carry flag is not changed)
	{
		0x78f0,
		func(c *Chip8) {
			c.v[8] = 0xf
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[8], uint8(0xff))
			assert.Equal(t, c.v[0xf], uint8(0))
		},
	},
	// 8XY0	Vx=Vy
	{
		0x8450,
		func(c *Chip8) {
			c.v[5] = 0x33
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[4], uint8(0x33))
		},
	},
	// 8XY1	Vx=Vx|Vy
	{
		0x8231,
		func(c *Chip8) {
			c.v[2] = 0x01
			c.v[3] = 0x10
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[2], uint8(0x11))
		},
	},
	// 8XY2	Vx=Vx&Vy
	{
		0x8012,
		func(c *Chip8) {
			c.v[0] = 0x01
			c.v[1] = 0x10
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0], uint8(0))
		},
	},
	// 8XY3	Vx=Vx^Vy
	{
		0x8673,
		func(c *Chip8) {
			c.v[6] = 0x09
			c.v[7] = 0x0f
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[6], uint8(6))
		},
	},
	// 8XY4	Vx += Vy (not carry)
	{
		0x8894,
		func(c *Chip8) {
			c.v[8] = 0x12
			c.v[9] = 0x34
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[8], uint8(0x46))
			assert.Equal(t, c.v[0xf], uint8(0))
		},
	},
	// 8XY4	Vx += Vy (carry)
	{
		0x8894,
		func(c *Chip8) {
			c.v[8] = 0xab
			c.v[9] = 0xcd
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[8], uint8(0x0ff&(0xab+0xcd)))
			assert.Equal(t, c.v[0xf], uint8(1))
		},
	},
	// 8XY5	Vx -= Vy (not borrow)
	{
		0x8ab5,
		func(c *Chip8) {
			c.v[0xa] = 0x45
			c.v[0xb] = 0x23
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0xa], uint8(0x22))
			assert.Equal(t, c.v[0xf], uint8(1))
		},
	},
	// 8XY5	Vx -= Vy (borrow)
	{
		0x8ab5,
		func(c *Chip8) {
			c.v[0xa] = 0x45
			c.v[0xb] = 0x56
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0xa], uint8(0xff&(0x45-0x56)))
			assert.Equal(t, c.v[0xf], uint8(0))
		},
	},
	// 8XY6	Vx>>=1 (bit0 is 1)
	{
		0x8cd6,
		func(c *Chip8) {
			c.v[0xc] = 0x3
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0xc], uint8(1))
			assert.Equal(t, c.v[0xf], uint8(1))
		},
	},
	// 8XY6	Vx>>=1 (bit0 is 1)
	{
		0x8cd6,
		func(c *Chip8) {
			c.v[0xc] = 0x2
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0xc], uint8(1))
			assert.Equal(t, c.v[0xf], uint8(0))
		},
	},
	// 8XY7	Vx=Vy-Vx (not borrow)
	{
		0x8ef7,
		func(c *Chip8) {
			c.v[0xe] = 0x45
			c.v[0xf] = 0x67
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0xe], uint8(0x22))
			assert.Equal(t, c.v[0xf], uint8(1))
		},
	},
	// 8XY7	Vx=Vy-Vx (borrow)
	{
		0x8ef7,
		func(c *Chip8) {
			c.v[0xe] = 0x67
			c.v[0xf] = 0x45
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0xe], uint8(0xff&(0x45-0x67)))
			assert.Equal(t, c.v[0xf], uint8(0))
		},
	},
	// 8XYE Vx<<=1 (bit15 is 0)
	{
		0x801E,
		func(c *Chip8) {
			c.v[0] = 0x08
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0], uint8(0x10))
			assert.Equal(t, c.v[0xf], uint8(0))
		},
	},
	// 8XY6	Vx>>=1 (bit15 is 1)
	{
		0x801E,
		func(c *Chip8) {
			c.v[0] = 0x88
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[0], uint8(0x10))
			assert.Equal(t, c.v[0xf], uint8(1))
		},
	},
	// 9XY0 if(Vx!=Vy) (true)
	{
		0x9120,
		func(c *Chip8) {
			c.v[1] = 1
			c.v[2] = 2
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x204))
		},
	},
	// 9XY0 if(Vx!=Vy) (false)
	{
		0x9120,
		func(c *Chip8) {
			c.v[1] = 1
			c.v[2] = 1
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x202))
		},
	},
	// ANNN I = NNN
	{
		0xA123,
		nil,
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.i, uint16(0x123))
		},
	},
	// BNNN PC=V0+NNN
	{
		0xB100,
		func(c *Chip8) {
			c.v[0] = 0x23
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x123))
		},
	},
	// CXNN Vx=rand()&NN
	{
		0xC800,
		func(c *Chip8) {
			c.v[8] = 0xff
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[8], uint8(0))
		},
	},
	// DXYN draw(Vx,Vy,N) (not flip)
	{
		0xD128,
		func(c *Chip8) {
			c.v[1] = 8
			c.v[2] = 8
			for i := range c.mem[:32] {
				c.mem[i] = 0xff
			}
		},
		func(t *testing.T, c *Chip8) {
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					assert.Equal(t, c.disp[(y+8)*Chip8DisplayW+x+8], uint8(1))
				}
			}
			assert.Equal(t, c.v[0xf], uint8(0))
		},
	},
	// DXYN draw(Vx,Vy,N) (flip)
	{
		0xD128,
		func(c *Chip8) {
			c.v[1] = 8
			c.v[2] = 8
			for i := range c.mem[:32] {
				c.mem[i] = 0xff
			}
			for i := range c.disp {
				c.disp[i] = 1
			}
		},
		func(t *testing.T, c *Chip8) {
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					assert.Equal(t, c.disp[(y+8)*Chip8DisplayW+x+8], uint8(0))
				}
			}
			assert.Equal(t, c.v[0xf], uint8(1))
		},
	},
	// EX9E if(key()==Vx) (true)
	{
		0xE09E,
		func(c *Chip8) {
			c.v[0] = 7
			c.keys[7] = 1
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x204))
		},
	},
	// EX9E if(key()==Vx) (false)
	{
		0xE09E,
		func(c *Chip8) {
			c.v[0] = 7
			c.keys[7] = 0
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x202))
		},
	},
	// EXA1 if(key()!=Vx) (true)
	{
		0xE09E,
		func(c *Chip8) {
			c.v[0] = 7
			c.keys[7] = 1
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x204))
		},
	},
	// EXA1 if(key()!=Vx) (false)
	{
		0xE09E,
		func(c *Chip8) {
			c.v[0] = 7
			c.keys[7] = 0
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.pc, uint16(0x202))
		},
	},
	// FX07 Vx = get_delay()
	{
		0xF107,
		func(c *Chip8) {
			c.dt = 10
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[1], uint8(10))
		},
	},
	// FX0A Vx = get_key() (any key pressed)
	{
		0xF20A,
		func(c *Chip8) {
			c.keys[1] = 1
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[2], uint8(1))
			assert.Equal(t, c.pc, uint16(0x202))
		},
	},
	// FX0A Vx = get_key() (key not pressed)
	{
		0xF20A,
		nil,
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[2], uint8(0))
			assert.Equal(t, c.pc, uint16(0x200))
		},
	},
	// FX15 delay_timer(Vx)
	{
		0xF215,
		func(c *Chip8) {
			c.v[2] = 10
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.dt, uint8(10))
		},
	},
	// FX18 sound_timer(Vx)
	{
		0xF318,
		func(c *Chip8) {
			c.v[3] = 10
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.st, uint8(10))
		},
	},
	// FX1E I +=Vx
	{
		0xF41E,
		func(c *Chip8) {
			c.v[4] = 10
			c.i = 0x100
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.i, uint16(0x100+10))
		},
	},
	// FX29 I=sprite_addr[Vx]
	{
		0xF529,
		func(c *Chip8) {
			c.v[5] = 5
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.i, uint16(CharacterSpritesOffset+5*CharacterSpriteBytes))
		},
	},
	// FX33 set_BCD(Vx); Vx = 123
	{
		0xF633,
		func(c *Chip8) {
			c.v[6] = 123
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.mem[0], uint8(1))
			assert.Equal(t, c.mem[1], uint8(2))
			assert.Equal(t, c.mem[2], uint8(3))
		},
	},
	// FX33 set_BCD(Vx); Vx = 45
	{
		0xF633,
		func(c *Chip8) {
			c.v[6] = 45
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.mem[0], uint8(0))
			assert.Equal(t, c.mem[1], uint8(4))
			assert.Equal(t, c.mem[2], uint8(5))
		},
	},
	// FX33 set_BCD(Vx); Vx = 6
	{
		0xF633,
		func(c *Chip8) {
			c.v[6] = 6
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.mem[0], uint8(0))
			assert.Equal(t, c.mem[1], uint8(0))
			assert.Equal(t, c.mem[2], uint8(6))
		},
	},
	// FX55 reg_dump(Vx,&I)
	{
		0xF755,
		func(c *Chip8) {
			for i := range c.v {
				c.v[i] = uint8(i + 1)
			}
			c.v[7] = 4
			c.i = 12
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.mem[12:12+4+1], c.v[:4+1])
		},
	},
	// FX65 reg_load(Vx,&I)
	{
		0xF865,
		func(c *Chip8) {
			for i := range c.v {
				c.mem[i] = uint8(i + 1)
			}
			c.v[8] = 5
			c.i = 0
		},
		func(t *testing.T, c *Chip8) {
			assert.Equal(t, c.v[:5+1], c.mem[:5+1])
		},
	},
}

func TestExecOpcodes(t *testing.T) {
	for _, test := range opcodeTestTable {
		t.Run(fmt.Sprintf("opcode[%04X]", test.opcode), func(t *testing.T) {
			b := make([]byte, 0x100)
			binary.BigEndian.PutUint16(b, test.opcode)
			c := newChip8(b)

			if test.before != nil {
				test.before(c)
			}

			c.step()

			test.assert(t, c)
		})
	}
}
