package emulator

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func (c *Chip8) execOpcode(op uint16) {
	pc := c.pc - 2
	h := op & 0xF000
	nnn := op & 0x0FFF
	nn := uint8(nnn & 0xff)
	x := uint8((nnn >> 8) & 0xf)
	y := uint8((nnn >> 4) & 0xf)
	n := nn & 0x0f

	mnemonic := ""
	switch h {
	case 0x0000:
		switch op {
		case 0x00E0: // clear display
			for i := range c.disp {
				c.disp[i] = 0
			}
			mnemonic = fmt.Sprintf("CLS  ")

		case 0x00EE: // return from subroutine
			r := c.popStack()
			c.pc = r
			mnemonic = fmt.Sprintf("RET  ")

		default:
			log.Fatalf("Not Implemented 0NNN %04X\n", op)
		}
	case 0x1000: // goto 0x0NNN
		c.pc = nnn
		mnemonic = fmt.Sprintf("GOTO %03X", nnn)

	case 0x2000: // call 0x0NNN
		c.pushStack(c.pc)
		c.pc = nnn
		mnemonic = fmt.Sprintf("CALL %03X", nnn)

	case 0x3000: // 0x3XNN if(Vx==NN)
		if c.v[x] == nn {
			c.pc += 2
		}
		mnemonic = fmt.Sprintf("SE   V%0X,#%02X", x, nn)

	case 0x4000: // 0x4XNN if(Vx!=NN)
		if c.v[x] != nn {
			c.pc += 2
		}
		mnemonic = fmt.Sprintf("SNE  V%0X,#%02X", x, nn)

	case 0x5000: // 0x5XY0 if(Vx==Vy)
		if c.v[x] == c.v[y] {
			c.pc += 2
		}
		mnemonic = fmt.Sprintf("SE   V%0X,V%0X", x, y)

	case 0x6000: // 6XNN Vx = NN
		c.v[x] = nn
		mnemonic = fmt.Sprintf("LD   V%0X,#%02X", x, nn)

	case 0x7000: // 7XNN Vx += NN (Carry flag is not changed)
		c.v[x] += nn
		mnemonic = fmt.Sprintf("ADD  V%0X,#%02X", x, nn)

	case 0x8000:
		switch nnn & 0xf {
		case 0: // 8XY0	Vx=Vy
			c.v[x] = c.v[y]
			mnemonic = fmt.Sprintf("LD   V%0X,V%0X", x, y)

		case 1: // 8XY1	Vx=Vx|Vy
			c.v[x] |= c.v[y]
			mnemonic = fmt.Sprintf("OR   V%0X,V%0X", x, y)

		case 2: // 8XY2	Vx=Vx&Vy
			c.v[x] &= c.v[y]
			mnemonic = fmt.Sprintf("AND  V%0X,V%0X", x, y)

		case 3: // 8XY3	Vx=Vx^Vy
			c.v[x] ^= c.v[y]
			mnemonic = fmt.Sprintf("XOR  V%0X,V%0X", x, y)

		case 4: // 8XY4	Vx += Vy
			carried := (uint16(c.v[x]) + uint16(c.v[y])) > 0xff
			c.v[x] += c.v[y]
			c.updateCarryFlag(carried)
			mnemonic = fmt.Sprintf("ADD  V%0X,V%0X", x, y)

		case 5: // 8XY5	Vx -= Vy
			borrowed := c.v[x] < c.v[y]
			c.v[x] -= c.v[y]
			c.updateCarryFlag(!borrowed)
			mnemonic = fmt.Sprintf("SUB  V%0X,V%0X", x, y)

		case 6: // 8XY6	Vx>>=1
			c.updateCarryFlag((c.v[x] & 0x01) == 1)
			c.v[x] = c.v[x] >> 1
			mnemonic = fmt.Sprintf("SHR  V%0X", x)

		case 7: // 8XY7	Vx=Vy-Vx
			borrowed := c.v[y] < c.v[x]
			c.v[x] = c.v[y] - c.v[x]
			c.updateCarryFlag(!borrowed)
			mnemonic = fmt.Sprintf("SUBN V%0X,V%0X", x, y)

		case 0xE: // 8XYE Vx<<=1
			c.updateCarryFlag((c.v[x] >> 7) == 1)
			c.v[x] = c.v[x] << 1
			mnemonic = fmt.Sprintf("SHL  V%0X", x)
		}
	case 0x9000: // 9XY0 if(Vx!=Vy)
		if c.v[x] != c.v[y] {
			c.pc += 2
		}
		mnemonic = fmt.Sprintf("SNE  V%0X,V%0X", x, y)

	case 0xA000: // ANNN I = NNN
		c.i = nnn
		mnemonic = fmt.Sprintf("LD   I,#%04X", nnn)

	case 0xB000: // BNNN PC=V0+NNN
		c.pc = uint16(c.v[0]) + nnn
		mnemonic = fmt.Sprintf("JP   V0,#%04X", nnn)

	case 0xC000: // CXNN Vx=rand()&NN
		c.v[x] = uint8(rand.Uint32() & uint32(nn))
		mnemonic = fmt.Sprintf("RND  V%0X,#%02X", x, nn)

	case 0xD000: // DXYN draw(Vx,Vy,N)
		flipped := c.draw(c.v[x], c.v[y], n)
		c.updateCarryFlag(flipped)
		mnemonic = fmt.Sprintf("DRW  V%0X,V%0X,%d", x, y, n)

	case 0xE000:
		switch nn {
		case 0x9E: // EX9E if(key()==Vx)
			if c.keys[c.v[x]] == 1 {
				c.pc += 2
			}
			mnemonic = fmt.Sprintf("SKP  V%0X", x)

		case 0xA1: // EXA1 if(key()!=Vx)
			if c.keys[c.v[x]] == 0 {
				c.pc += 2
			}
			mnemonic = fmt.Sprintf("SKNP V%0X", x)
		}
	case 0xF000:
		switch nn {
		case 0x07: // FX07 Vx = get_delay()
			c.v[x] = c.dt
			mnemonic = fmt.Sprintf("LD   V%0X,DT", x)

		case 0x0A: // FX0A Vx = get_key()
			if c.pressedAnyKey() == 0xff {
				// pc decrement for blocking
				c.pc -= 2
			} else {
				c.v[x] = c.pressedAnyKey()
			}
			mnemonic = fmt.Sprintf("LD   V%0X,K", x)

		case 0x15: // FX15 delay_timer(Vx)
			c.dt = c.v[x]
			mnemonic = fmt.Sprintf("LD   DT,V%0X", x)

		case 0x18: // FX18 sound_timer(Vx)
			c.st = c.v[x]
			mnemonic = fmt.Sprintf("LD   ST,V%0X", x)

		case 0x1E: // FX1E I +=Vx
			c.i += uint16(c.v[x])
			mnemonic = fmt.Sprintf("ADD  I,V%0X", x)

		case 0x29: // FX29 I=sprite_addr[Vx]
			c.i = CharacterSpritesOffset + uint16(c.v[x])*CharacterSpriteBytes
			mnemonic = fmt.Sprintf("LD   F,V%0X", x)

		case 0x33: // FX33 set_BCD(Vx);
			c.mem[c.i+0] = c.v[x] / 100
			c.mem[c.i+1] = (c.v[x] % 100) / 10
			c.mem[c.i+2] = c.v[x] % 10
			mnemonic = fmt.Sprintf("LD   B,V%0X", x)

		case 0x55: // FX55 reg_dump(Vx,&I)
			copy(c.mem[c.i:], c.v[:x+1])
			mnemonic = fmt.Sprintf("LD   [I],V%0X", x)

		case 0x65: // FX65 reg_load(Vx,&I)
			copy(c.v[:x+1], c.mem[c.i:])
			mnemonic = fmt.Sprintf("LD   V%0X,[I]", x)
		}
	}

	c.ophistory[c.ophistoryIndex] = fmt.Sprintf("%03X-%04X %s", pc, op, mnemonic)
	c.ophistoryIndex = (c.ophistoryIndex + 1) % OpHistoryNum
}
