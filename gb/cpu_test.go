package gb

import (
	"testing"
)

func newTestCore() *Core {
	core := &Core{
		FPS:           60,
		Clock:         4194304,
		SpeedMultiple: 0,
		Cartridge: Cartridge{
			Props: &CartridgeProps{
				ROMLength: 0x8000,
			},
			MBC: &MBCRom{
				rom: make([]byte, 0x8000),
			},
		},
	}
	core.initMemory()
	core.initCPU()
	core.initCB()
	core.Screen = &core.Buffers[0]
	return core
}

func TestRegistersGetSet(t *testing.T) {
	core := newTestCore()

	// AF
	core.CPU.setAF(0x1234)
	if core.CPU.Registers.A != 0x12 || core.CPU.Registers.F != 0x34 {
		t.Errorf("setAF(0x1234): A=0x%02X, F=0x%02X", core.CPU.Registers.A, core.CPU.Registers.F)
	}
	if core.CPU.getAF() != 0x1234 {
		t.Errorf("getAF() = 0x%04X; want 0x1234", core.CPU.getAF())
	}

	// BC
	core.CPU.setBC(0xABCD)
	if core.CPU.Registers.B != 0xAB || core.CPU.Registers.C != 0xCD {
		t.Errorf("setBC(0xABCD): B=0x%02X, C=0x%02X", core.CPU.Registers.B, core.CPU.Registers.C)
	}
	if core.CPU.getBC() != 0xABCD {
		t.Errorf("getBC() = 0x%04X; want 0xABCD", core.CPU.getBC())
	}

	// DE
	core.CPU.setDE(0x5678)
	if core.CPU.Registers.D != 0x56 || core.CPU.Registers.E != 0x78 {
		t.Errorf("setDE(0x5678): D=0x%02X, E=0x%02X", core.CPU.Registers.D, core.CPU.Registers.E)
	}
	if core.CPU.getDE() != 0x5678 {
		t.Errorf("getDE() = 0x%04X; want 0x5678", core.CPU.getDE())
	}

	// HL byte setters
	core.setH(0xFE)
	core.setL(0xDC)
	if core.CPU.Registers.HL != 0xFEDC {
		t.Errorf("HL = 0x%04X; want 0xFEDC", core.CPU.Registers.HL)
	}
}

func TestArithmetic_ADD_ADC(t *testing.T) {
	core := newTestCore()

	// ADD A, B: 0x0F + 0x01 = 0x10 (HalfCarry=true, Carry=false, Zero=false)
	core.CPU.Registers.A = 0x0F
	core.CPU.Registers.B = 0x01
	core.OP80() // ADD A,B
	if core.CPU.Registers.A != 0x10 {
		t.Errorf("ADD A,B = 0x%02X; want 0x10", core.CPU.Registers.A)
	}
	if !core.CPU.Flags.HalfCarry || core.CPU.Flags.Carry || core.CPU.Flags.Zero {
		t.Errorf("Flags: H=%v, C=%v, Z=%v; want H=true, C=false, Z=false",
			core.CPU.Flags.HalfCarry, core.CPU.Flags.Carry, core.CPU.Flags.Zero)
	}

	// ADC A, B with Carry: 0xFF + 0x00 + 1 = 0x00 (Zero=true, Carry=true, HalfCarry=true)
	core.CPU.Flags.Carry = true
	core.CPU.Registers.A = 0xFF
	core.CPU.Registers.B = 0x00
	core.OP88() // ADC A,B
	if core.CPU.Registers.A != 0x00 {
		t.Errorf("ADC A,B = 0x%02X; want 0x00", core.CPU.Registers.A)
	}
	if !core.CPU.Flags.Zero || !core.CPU.Flags.Carry || !core.CPU.Flags.HalfCarry {
		t.Errorf("Flags: Z=%v, C=%v, H=%v; want all true",
			core.CPU.Flags.Zero, core.CPU.Flags.Carry, core.CPU.Flags.HalfCarry)
	}
}

func TestArithmetic_SUB_SBC_CP(t *testing.T) {
	core := newTestCore()

	// SUB B: 0x10 - 0x01 = 0x0F (HalfCarry=true, Carry=false)
	core.CPU.Registers.A = 0x10
	core.CPU.Registers.B = 0x01
	core.OP90() // SUB B
	if core.CPU.Registers.A != 0x0F {
		t.Errorf("SUB B = 0x%02X; want 0x0F", core.CPU.Registers.A)
	}
	if !core.CPU.Flags.HalfCarry || core.CPU.Flags.Carry || !core.CPU.Flags.Sub {
		t.Errorf("Flags: H=%v, C=%v, N=%v; want H=true, C=false, N=true",
			core.CPU.Flags.HalfCarry, core.CPU.Flags.Carry, core.CPU.Flags.Sub)
	}

	// CP B: compare 0x05 with 0x05 -> Zero=true, Carry=false, HalfCarry=false
	core.CPU.Registers.A = 0x05
	core.CPU.Registers.B = 0x05
	core.OPB8() // CP B
	if !core.CPU.Flags.Zero || core.CPU.Flags.Carry {
		t.Errorf("CP B: Z=%v, C=%v; want Z=true, C=false", core.CPU.Flags.Zero, core.CPU.Flags.Carry)
	}

	// CP B: compare 0x05 with 0x06 -> Zero=false, Carry=true (5 < 6)
	core.CPU.Registers.B = 0x06
	core.OPB8()
	if core.CPU.Flags.Zero || !core.CPU.Flags.Carry {
		t.Errorf("CP B (5 < 6): Z=%v, C=%v; want Z=false, C=true", core.CPU.Flags.Zero, core.CPU.Flags.Carry)
	}
}

func TestArithmetic_16Bit_ADD_HL(t *testing.T) {
	core := newTestCore()

	// ADD HL, BC: 0x0FFF + 0x0001 = 0x1000 (HalfCarry=true, Carry=false)
	core.CPU.Registers.HL = 0x0FFF
	core.CPU.setBC(0x0001)
	core.OP09() // ADD HL,BC
	if core.CPU.Registers.HL != 0x1000 {
		t.Errorf("ADD HL,BC = 0x%04X; want 0x1000", core.CPU.Registers.HL)
	}
	if !core.CPU.Flags.HalfCarry || core.CPU.Flags.Carry {
		t.Errorf("ADD HL,BC HalfCarry=%v, Carry=%v; want H=true, C=false",
			core.CPU.Flags.HalfCarry, core.CPU.Flags.Carry)
	}

	// ADD HL, DE: 0xFFFF + 0x0001 = 0x0000 (Carry=true, HalfCarry=true)
	core.CPU.Registers.HL = 0xFFFF
	core.CPU.setDE(0x0001)
	core.OP19() // ADD HL,DE
	if core.CPU.Registers.HL != 0x0000 {
		t.Errorf("ADD HL,DE = 0x%04X; want 0x0000", core.CPU.Registers.HL)
	}
	if !core.CPU.Flags.Carry || !core.CPU.Flags.HalfCarry {
		t.Errorf("ADD HL,DE Carry=%v, HalfCarry=%v; want both true",
			core.CPU.Flags.Carry, core.CPU.Flags.HalfCarry)
	}
}

func TestBitwise_AND_OR_XOR_CPL(t *testing.T) {
	core := newTestCore()

	// AND: 0xAA & 0x55 = 0x00 (Zero=true, HalfCarry=true)
	core.CPU.Registers.A = 0xAA
	core.CPU.Registers.B = 0x55
	core.OPA0() // AND B
	if core.CPU.Registers.A != 0x00 || !core.CPU.Flags.Zero || !core.CPU.Flags.HalfCarry {
		t.Errorf("AND: A=0x%02X, Z=%v, H=%v", core.CPU.Registers.A, core.CPU.Flags.Zero, core.CPU.Flags.HalfCarry)
	}

	// OR: 0xF0 | 0x0F = 0xFF
	core.CPU.Registers.A = 0xF0
	core.CPU.Registers.B = 0x0F
	core.OPB0() // OR B
	if core.CPU.Registers.A != 0xFF || core.CPU.Flags.Zero {
		t.Errorf("OR: A=0x%02X, Z=%v", core.CPU.Registers.A, core.CPU.Flags.Zero)
	}

	// XOR: 0xAA ^ 0xAA = 0x00 (Zero=true)
	core.CPU.Registers.A = 0xAA
	core.OPAF() // XOR A
	if core.CPU.Registers.A != 0x00 || !core.CPU.Flags.Zero {
		t.Errorf("XOR A: A=0x%02X, Z=%v", core.CPU.Registers.A, core.CPU.Flags.Zero)
	}

	// CPL: 0x55 -> 0xAA
	core.CPU.Registers.A = 0x55
	core.OP2F() // CPL
	if core.CPU.Registers.A != 0xAA || !core.CPU.Flags.Sub || !core.CPU.Flags.HalfCarry {
		t.Errorf("CPL: A=0x%02X", core.CPU.Registers.A)
	}
}

func TestStack_PushPopAF(t *testing.T) {
	core := newTestCore()
	core.CPU.Registers.SP = 0xFFFE

	// Set AF with all flag bits set (0x12F0)
	core.CPU.Registers.A = 0x12
	core.CPU.Flags.Zero = true
	core.CPU.Flags.Sub = true
	core.CPU.Flags.HalfCarry = true
	core.CPU.Flags.Carry = true
	core.OPF5() // PUSH AF

	// POP AF into empty state
	core.CPU.Registers.A = 0x00
	core.CPU.Registers.F = 0x00
	core.OPF1() // POP AF

	// In Game Boy, lower 4 bits of F register are ALWAYS 0!
	if core.CPU.Registers.A != 0x12 {
		t.Errorf("POP AF: A = 0x%02X; want 0x12", core.CPU.Registers.A)
	}
	if core.CPU.Registers.F != 0xF0 {
		t.Errorf("POP AF: F = 0x%02X; want 0xF0 (low 4 bits must be zero)", core.CPU.Registers.F)
	}
}

func TestCGB_SpeedSwitch_STOP(t *testing.T) {
	core := newTestCore()
	core.IsCGB = true
	core.SpeedMultiple = 0
	core.Memory.MainMemory[0xFF4D] = 0x01 // Speed switch requested (bit 0 = 1)

	// STOP opcode (0x10) is 2 bytes; set next byte at PC to 0x00
	core.CPU.Registers.PC = 0xC000
	core.Memory.MainMemory[0xC000] = 0x00

	core.OP10() // Execute STOP

	if core.SpeedMultiple != 1 {
		t.Errorf("SpeedMultiple = %d; want 1 (Double Speed)", core.SpeedMultiple)
	}
	if core.Memory.MainMemory[0xFF4D]&0x80 == 0 {
		t.Errorf("KEY1 0xFF4D bit 7 should be 1; got 0x%02X", core.Memory.MainMemory[0xFF4D])
	}

	// Switch back to normal speed
	core.Memory.MainMemory[0xFF4D] |= 0x01 // Request switch
	core.OP10()

	if core.SpeedMultiple != 0 {
		t.Errorf("SpeedMultiple = %d; want 0 (Normal Speed)", core.SpeedMultiple)
	}
	if core.Memory.MainMemory[0xFF4D]&0x80 != 0 {
		t.Errorf("KEY1 0xFF4D bit 7 should be 0; got 0x%02X", core.Memory.MainMemory[0xFF4D])
	}
}
