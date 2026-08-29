package gb

import (
	"testing"
)

func TestGraphics_LCD_STAT_Modes(t *testing.T) {
	core := newTestCore()
	core.Memory.MainMemory[0xFF40] = 0x80 // LCD Enabled
	core.Memory.MainMemory[0xFF44] = 0    // Scanline 0

	// Mode 2: OAM search (ScanlineCounter in [376, 456])
	core.Timer.ScanlineCounter = 400
	core.SetLCDStatus()
	mode := core.Memory.MainMemory[0xFF41] & 0x03
	if mode != 2 {
		t.Errorf("LCD Mode at Counter 400 = %d; want 2 (OAM Search)", mode)
	}

	// Mode 3: Pixel transfer (ScanlineCounter in [204, 375])
	core.Timer.ScanlineCounter = 300
	core.SetLCDStatus()
	mode = core.Memory.MainMemory[0xFF41] & 0x03
	if mode != 3 {
		t.Errorf("LCD Mode at Counter 300 = %d; want 3 (Pixel Transfer)", mode)
	}

	// Mode 0: HBlank (ScanlineCounter < 204)
	core.Timer.ScanlineCounter = 100
	core.SetLCDStatus()
	mode = core.Memory.MainMemory[0xFF41] & 0x03
	if mode != 0 {
		t.Errorf("LCD Mode at Counter 100 = %d; want 0 (HBlank)", mode)
	}

	// Mode 1: VBlank (Scanline >= 144)
	core.Memory.MainMemory[0xFF44] = 144
	core.SetLCDStatus()
	mode = core.Memory.MainMemory[0xFF41] & 0x03
	if mode != 1 {
		t.Errorf("LCD Mode at Scanline 144 = %d; want 1 (VBlank)", mode)
	}
}

func TestGraphics_LYC_Coincidence(t *testing.T) {
	core := newTestCore()
	core.Memory.MainMemory[0xFF40] = 0x80 // LCD Enabled
	core.Memory.MainMemory[0xFF44] = 42   // LY = 42
	core.Memory.MainMemory[0xFF45] = 42   // LYC = 42
	core.Memory.MainMemory[0xFF41] = 0x40 // Enable LYC=LY STAT Interrupt (bit 6)

	core.SetLCDStatus()

	// Bit 2 in STAT should be set (Coincidence flag)
	if core.Memory.MainMemory[0xFF41]&0x04 == 0 {
		t.Errorf("LYC=LY coincidence flag (bit 2 of 0xFF41) should be set")
	}

	// LCD STAT Interrupt (bit 1 of IF) should be requested
	if core.Memory.MainMemory[0xFF0F]&0x02 == 0 {
		t.Errorf("LCD STAT interrupt (bit 1 of 0xFF0F) should be requested")
	}
}

func TestGraphics_RenderTiles_DMG(t *testing.T) {
	core := newTestCore()
	core.IsCGB = false
	core.Memory.MainMemory[0xFF40] = 0x91 // LCD on, BG on, Tile data 0x8000
	core.Memory.MainMemory[0xFF44] = 0    // Scanline 0
	core.Memory.MainMemory[0xFF47] = 0xE4 // 11 10 01 00 (Standard DMG Palette)

	// Tile 0 at 0x8000: Pattern with color 3 (both bytes 0xFF)
	core.Memory.MainMemory[0x8000] = 0xFF
	core.Memory.MainMemory[0x8001] = 0xFF

	// Map Tile 0 at 0x9800 (BG Map)
	core.Memory.MainMemory[0x9800] = 0x00

	core.RenderTiles(core.Memory.MainMemory[0xFF40])

	// First pixel on scanline 0 should be rendered with color 3 of dmgPalette
	expectedColor := dmgPalette[3]
	actualColor := [3]uint8{
		core.Screen[0][0][0],
		core.Screen[0][0][1],
		core.Screen[0][0][2],
	}

	if actualColor != expectedColor {
		t.Errorf("Rendered pixel color = %v; want %v", actualColor, expectedColor)
	}
}
