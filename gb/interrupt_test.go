package gb

import (
	"testing"
)

func TestInterrupt_PrioritiesAndVectors(t *testing.T) {
	tests := []struct {
		id             int
		expectedVector uint16
	}{
		{0, 0x0040}, // V-Blank
		{1, 0x0048}, // LCD STAT
		{2, 0x0050}, // Timer
		{3, 0x0058}, // Serial
		{4, 0x0060}, // Joypad
	}

	for _, tt := range tests {
		core := newTestCore()
		core.CPU.Flags.InterruptMaster = true
		core.CPU.Registers.PC = 0x1234
		core.CPU.Registers.SP = 0xFFFE
		core.Memory.MainMemory[0xFFFF] = byte(1 << tt.id) // Enable interrupt in IE
		core.Memory.MainMemory[0xFF0F] = byte(1 << tt.id) // Request interrupt in IF

		cycles := core.Interrupt()
		if cycles != 20 {
			t.Errorf("Interrupt cycles = %d; want 20", cycles)
		}
		if core.CPU.Registers.PC != tt.expectedVector {
			t.Errorf("Interrupt %d PC vector = 0x%04X; want 0x%04X", tt.id, core.CPU.Registers.PC, tt.expectedVector)
		}
		if core.CPU.Flags.InterruptMaster {
			t.Errorf("InterruptMaster should be disabled during interrupt service")
		}
		// Return address should be pushed on stack
		popped := core.StackPop()
		if popped != 0x1234 {
			t.Errorf("Stacked return PC = 0x%04X; want 0x1234", popped)
		}
	}
}

func TestInterrupt_PriorityOrder(t *testing.T) {
	core := newTestCore()
	core.CPU.Flags.InterruptMaster = true
	core.CPU.Registers.PC = 0x1000
	core.CPU.Registers.SP = 0xFFFE

	// Enable all interrupts in IE
	core.Memory.MainMemory[0xFFFF] = 0x1F
	// Request both VBlank (bit 0) and Timer (bit 2) in IF
	core.Memory.MainMemory[0xFF0F] = 0x05

	// Higher priority VBlank (vector 0x40) must be serviced first
	core.Interrupt()
	if core.CPU.Registers.PC != 0x0040 {
		t.Errorf("Expected VBlank (0x40) to take priority; got PC = 0x%04X", core.CPU.Registers.PC)
	}

	// IF bit 0 should be cleared, bit 2 should still be pending
	if core.Memory.MainMemory[0xFF0F]&0x01 != 0 {
		t.Errorf("VBlank flag in IF should be cleared after servicing")
	}
	if core.Memory.MainMemory[0xFF0F]&0x04 == 0 {
		t.Errorf("Timer flag in IF should still be pending")
	}
}

func TestInterrupt_UnhaltsCPU(t *testing.T) {
	core := newTestCore()
	core.CPU.Halt = true
	core.CPU.Flags.InterruptMaster = false // IME disabled

	core.Memory.MainMemory[0xFFFF] = 0x01 // Enable VBlank in IE
	core.Memory.MainMemory[0xFF0F] = 0x01 // Request VBlank in IF

	core.Interrupt()

	if core.CPU.Halt {
		t.Errorf("CPU should unhalt even if IME is disabled")
	}
}

func TestInterrupt_EI_Delay(t *testing.T) {
	core := newTestCore()
	core.CPU.Flags.InterruptMaster = false

	core.OPFB() // EI instruction sets PendingInterruptEnabled

	if core.CPU.Flags.InterruptMaster {
		t.Errorf("IME should not be enabled immediately on EI")
	}
	if !core.CPU.Flags.PendingInterruptEnabled {
		t.Errorf("PendingInterruptEnabled should be true")
	}

	// Next cycle/interrupt check promotes pending to active IME
	core.Interrupt()
	if !core.CPU.Flags.InterruptMaster {
		t.Errorf("IME should be enabled on the cycle following EI")
	}
}
