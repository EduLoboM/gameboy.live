package gb

import (
	"testing"
)

func TestCB_SWAP(t *testing.T) {
	core := newTestCore()
	core.CPU.Registers.A = 0xF0

	// SWAP A (0x37)
	core.cbMap[0x37]()
	if core.CPU.Registers.A != 0x0F {
		t.Errorf("SWAP A = 0x%02X; want 0x0F", core.CPU.Registers.A)
	}
	if core.CPU.Flags.Zero || core.CPU.Flags.Carry {
		t.Errorf("SWAP Flags: Z=%v, C=%v; want false, false", core.CPU.Flags.Zero, core.CPU.Flags.Carry)
	}

	// SWAP 0x00 -> Zero=true
	core.CPU.Registers.A = 0x00
	core.cbMap[0x37]()
	if !core.CPU.Flags.Zero {
		t.Errorf("SWAP 0x00 Zero flag should be true")
	}
}

func TestCB_Shifts_SLA_SRA_SRL(t *testing.T) {
	core := newTestCore()

	// SLA A (0x27): 0b10000001 << 1 = 0b00000010, Carry=1
	core.CPU.Registers.A = 0x81
	core.cbMap[0x27]()
	if core.CPU.Registers.A != 0x02 || !core.CPU.Flags.Carry {
		t.Errorf("SLA A: A=0x%02X, Carry=%v; want 0x02, true", core.CPU.Registers.A, core.CPU.Flags.Carry)
	}

	// SRA A (0x2F): 0b10000011 >> 1 = 0b11000001 (MSB preserved), Carry=1
	core.CPU.Registers.A = 0x83
	core.cbMap[0x2F]()
	if core.CPU.Registers.A != 0xC1 || !core.CPU.Flags.Carry {
		t.Errorf("SRA A: A=0x%02X, Carry=%v; want 0xC1, true", core.CPU.Registers.A, core.CPU.Flags.Carry)
	}

	// SRL A (0x3F): 0b10000011 >> 1 = 0b01000001 (MSB 0), Carry=1
	core.CPU.Registers.A = 0x83
	core.cbMap[0x3F]()
	if core.CPU.Registers.A != 0x41 || !core.CPU.Flags.Carry {
		t.Errorf("SRL A: A=0x%02X, Carry=%v; want 0x41, true", core.CPU.Registers.A, core.CPU.Flags.Carry)
	}
}

func TestCB_Rotates_RLC_RRC_RL_RR(t *testing.T) {
	core := newTestCore()

	// RLC A (0x07): 0b10000000 -> 0b00000001, Carry=1
	core.CPU.Registers.A = 0x80
	core.cbMap[0x07]()
	if core.CPU.Registers.A != 0x01 || !core.CPU.Flags.Carry {
		t.Errorf("RLC A: A=0x%02X, C=%v", core.CPU.Registers.A, core.CPU.Flags.Carry)
	}

	// RRC A (0x0F): 0b00000001 -> 0b10000000, Carry=1
	core.CPU.Registers.A = 0x01
	core.cbMap[0x0F]()
	if core.CPU.Registers.A != 0x80 || !core.CPU.Flags.Carry {
		t.Errorf("RRC A: A=0x%02X, C=%v", core.CPU.Registers.A, core.CPU.Flags.Carry)
	}

	// RL A (0x17) with Carry=0: 0b10000000 -> 0b00000000, Carry=1, Zero=true
	core.CPU.Flags.Carry = false
	core.CPU.Registers.A = 0x80
	core.cbMap[0x17]()
	if core.CPU.Registers.A != 0x00 || !core.CPU.Flags.Carry || !core.CPU.Flags.Zero {
		t.Errorf("RL A: A=0x%02X, C=%v, Z=%v", core.CPU.Registers.A, core.CPU.Flags.Carry, core.CPU.Flags.Zero)
	}
}

func TestCB_BIT_RES_SET(t *testing.T) {
	core := newTestCore()

	// BIT 3, B: B = 0b00001000 -> Zero=false, HalfCarry=true
	core.CPU.Registers.B = 0x08
	core.cbMap[0x58]() // BIT 3, B
	if core.CPU.Flags.Zero || !core.CPU.Flags.HalfCarry || core.CPU.Flags.Sub {
		t.Errorf("BIT 3 (bit set): Z=%v, H=%v, N=%v; want Z=false, H=true, N=false",
			core.CPU.Flags.Zero, core.CPU.Flags.HalfCarry, core.CPU.Flags.Sub)
	}

	// BIT 3, B: B = 0b00000000 -> Zero=true
	core.CPU.Registers.B = 0x00
	core.cbMap[0x58]()
	if !core.CPU.Flags.Zero {
		t.Errorf("BIT 3 (bit clear): Zero should be true")
	}

	// SET 5, C (0xE9)
	core.CPU.Registers.C = 0x00
	core.cbMap[0xE9]() // SET 5, C
	if core.CPU.Registers.C != 0x20 {
		t.Errorf("SET 5, C = 0x%02X; want 0x20", core.CPU.Registers.C)
	}

	// RES 5, C (0xA9)
	core.cbMap[0xA9]() // RES 5, C
	if core.CPU.Registers.C != 0x00 {
		t.Errorf("RES 5, C = 0x%02X; want 0x00", core.CPU.Registers.C)
	}

	// Test (HL) memory operand: SET 7, (HL) and BIT 7, (HL)
	core.CPU.Registers.HL = 0xC000
	core.Memory.MainMemory[0xC000] = 0x00
	core.cbMap[0xFE]() // SET 7, (HL)
	if core.Memory.MainMemory[0xC000] != 0x80 {
		t.Errorf("SET 7, (HL) = 0x%02X; want 0x80", core.Memory.MainMemory[0xC000])
	}
	core.cbMap[0x7E]() // BIT 7, (HL)
	if core.CPU.Flags.Zero {
		t.Errorf("BIT 7, (HL): Zero flag should be false")
	}
}
