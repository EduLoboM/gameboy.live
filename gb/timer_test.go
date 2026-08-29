package gb

import (
	"testing"
)

func TestTimer_DividerRegister(t *testing.T) {
	core := newTestCore()

	// Update 256 cycles -> DIV increments from 0 to 1
	core.DoDividerRegister(256)
	if val := core.Memory.MainMemory[0xFF04]; val != 1 {
		t.Errorf("DIV after 256 cycles = %d; want 1", val)
	}

	// Writing any value to 0xFF04 resets DIV to 0
	core.WriteMemory(0xFF04, 0x55)
	if val := core.Memory.MainMemory[0xFF04]; val != 0 {
		t.Errorf("DIV after write = %d; want 0", val)
	}
}

func TestTimer_TAC_Frequencies(t *testing.T) {
	core := newTestCore()

	tests := []struct {
		tac      byte
		expected int
	}{
		{0b100, 1024}, // 4096 Hz
		{0b101, 16},   // 262144 Hz
		{0b110, 64},   // 65536 Hz
		{0b111, 256},  // 16384 Hz
	}

	for _, tt := range tests {
		core.WriteMemory(0xFF07, tt.tac)
		if count := core.GetClockFreqCount(); count != tt.expected {
			t.Errorf("GetClockFreqCount() for TAC %03b = %d; want %d", tt.tac, count, tt.expected)
		}
	}
}

func TestTimer_OverflowAndInterrupt(t *testing.T) {
	core := newTestCore()

	// Enable timer with 262144 Hz (16 cycles per tick)
	core.WriteMemory(0xFF07, 0b101) // TAC: Enabled, 16 cycles
	core.WriteMemory(0xFF06, 0xA0)  // TMA (Modulo reload value)
	core.WriteMemory(0xFF05, 0xFE)  // TIMA (Initial counter: 254)
	core.Memory.MainMemory[0xFF0F] = 0x00 // Clear IF

	// Update 16 cycles -> TIMA becomes 255 (0xFF)
	core.UpdateTimers(16)
	if val := core.ReadMemory(0xFF05); val != 0xFF {
		t.Errorf("TIMA = 0x%02X; want 0xFF", val)
	}
	if core.Memory.MainMemory[0xFF0F]&0x04 != 0 {
		t.Errorf("Timer interrupt should not be requested yet")
	}

	// Update 16 more cycles -> TIMA overflows to 0, reloaded from TMA (0xA0), and triggers INT 2
	core.UpdateTimers(16)
	if val := core.ReadMemory(0xFF05); val != 0xA0 {
		t.Errorf("TIMA after overflow = 0x%02X; want TMA (0xA0)", val)
	}
	if core.Memory.MainMemory[0xFF0F]&0x04 == 0 {
		t.Errorf("Timer interrupt (bit 2) should be requested in IF (0xFF0F)")
	}
}
