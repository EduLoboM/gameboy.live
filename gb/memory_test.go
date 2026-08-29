package gb

import (
	"testing"
)

func TestMemory_WRAMBanking(t *testing.T) {
	core := newTestCore()
	core.IsCGB = true

	// Write to WRAM Bank 1 at 0xD000
	core.WriteMemory(0xFF70, 0x01) // Select WRAM bank 1
	core.WriteMemory(0xD000, 0x11)

	// Write to WRAM Bank 2 at 0xD000
	core.WriteMemory(0xFF70, 0x02) // Select WRAM bank 2
	core.WriteMemory(0xD000, 0x22)

	// Bank 0 selection is treated as bank 1
	core.WriteMemory(0xFF70, 0x00)
	if val := core.ReadMemory(0xD000); val != 0x11 {
		t.Errorf("WRAM bank 0 (alias for 1) = 0x%02X; want 0x11", val)
	}

	// Read bank 2
	core.WriteMemory(0xFF70, 0x02)
	if val := core.ReadMemory(0xD000); val != 0x22 {
		t.Errorf("WRAM bank 2 = 0x%02X; want 0x22", val)
	}
}

func TestMemory_VRAMBanking(t *testing.T) {
	core := newTestCore()
	core.IsCGB = true

	// Write to VRAM Bank 0 at 0x8000
	core.WriteMemory(0xFF4F, 0x00)
	core.WriteMemory(0x8000, 0xAA)

	// Write to VRAM Bank 1 at 0x8000
	core.WriteMemory(0xFF4F, 0x01)
	core.WriteMemory(0x8000, 0xBB)

	// Verify isolation
	core.WriteMemory(0xFF4F, 0x00)
	if val := core.ReadMemory(0x8000); val != 0xAA {
		t.Errorf("VRAM bank 0 = 0x%02X; want 0xAA", val)
	}

	core.WriteMemory(0xFF4F, 0x01)
	if val := core.ReadMemory(0x8000); val != 0xBB {
		t.Errorf("VRAM bank 1 = 0x%02X; want 0xBB", val)
	}
}

func TestMemory_OAM_DMA(t *testing.T) {
	core := newTestCore()

	// Fill source memory at 0xC100-0xC19F (160 bytes)
	for i := 0; i < 0xA0; i++ {
		core.Memory.MainMemory[0xC100+uint16(i)] = byte(i)
	}

	// Trigger DMA from 0xC100 by writing 0xC1 to 0xFF46
	core.WriteMemory(0xFF46, 0xC1)

	// Verify OAM memory at 0xFE00-0xFE9F
	for i := 0; i < 0xA0; i++ {
		if val := core.Memory.MainMemory[0xFE00+uint16(i)]; val != byte(i) {
			t.Errorf("OAM DMA[%d] = 0x%02X; want 0x%02X", i, val, byte(i))
		}
	}
}

func TestMemory_CGB_HDMA_General(t *testing.T) {
	core := newTestCore()
	core.IsCGB = true

	// Source at 0xC000, Destination at 0x8000 (VRAM)
	for i := 0; i < 32; i++ {
		core.Memory.MainMemory[0xC000+uint16(i)] = byte(i + 1)
	}

	core.WriteMemory(0xFF51, 0xC0) // Source Hi
	core.WriteMemory(0xFF52, 0x00) // Source Lo
	core.WriteMemory(0xFF53, 0x00) // Dest Hi
	core.WriteMemory(0xFF54, 0x00) // Dest Lo
	core.WriteMemory(0xFF55, 0x01) // 2 blocks (32 bytes), General DMA (bit 7 = 0)

	for i := 0; i < 32; i++ {
		if val := core.Memory.VRAMBanks[0][i]; val != byte(i+1) {
			t.Errorf("HDMA VRAM[%d] = %d; want %d", i, val, i+1)
		}
	}
}

func TestMemory_CGB_Palettes(t *testing.T) {
	core := newTestCore()
	core.IsCGB = true

	// Test BG Palette with auto-increment (bit 7 = 1)
	core.WriteMemory(0xFF68, 0x80) // Index 0, auto-increment on

	// Color 0 RGB555: Pure White (0x7FFF -> R=31, G=31, B=31)
	core.WriteMemory(0xFF69, 0xFF) // Lo
	core.WriteMemory(0xFF69, 0x7F) // Hi

	cache := core.Memory.BGPaletteCache[0][0]
	if cache[0] != 255 || cache[1] != 255 || cache[2] != 255 {
		t.Errorf("BG Palette Cache RGB = (%d, %d, %d); want (255, 255, 255)", cache[0], cache[1], cache[2])
	}

	// Index should auto-increment to 2
	if core.Memory.BGPaletteIndex&0x3F != 2 {
		t.Errorf("BGPaletteIndex = %d; want 2", core.Memory.BGPaletteIndex&0x3F)
	}
}
