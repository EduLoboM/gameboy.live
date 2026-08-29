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

func TestGraphics_RenderSprites_CGB_Priority(t *testing.T) {
	core := newTestCore()
	core.IsCGB = true
	core.Memory.MainMemory[0xFF40] = 0x82 // LCD on, Sprites on
	core.Memory.MainMemory[0xFF44] = 0    // Scanline 0

	// Set Sprite Palette 0 (color 1 = red) and Palette 1 (color 1 = blue)
	core.Memory.SpritePaletteCache[0][1] = [3]uint8{255, 0, 0}
	core.Memory.SpritePaletteCache[1][1] = [3]uint8{0, 0, 255}

	// Tile 0 in VRAM Bank 0: row 0 has color 1 (data1 = 0xFF, data2 = 0x00)
	core.Memory.VRAMBanks[0][0] = 0xFF
	core.Memory.VRAMBanks[0][1] = 0x00

	// Sprite 0 at (X=8, Y=16) -> Screen (0, 0), tile 0, palette 0
	core.Memory.MainMemory[0xFE00] = 16 // Y
	core.Memory.MainMemory[0xFE01] = 8  // X
	core.Memory.MainMemory[0xFE02] = 0  // Tile 0
	core.Memory.MainMemory[0xFE03] = 0  // Palette 0

	// Sprite 1 at same location (X=8, Y=16), tile 0, palette 1 (blue)
	core.Memory.MainMemory[0xFE04] = 16 // Y
	core.Memory.MainMemory[0xFE05] = 8  // X
	core.Memory.MainMemory[0xFE06] = 0  // Tile 0
	core.Memory.MainMemory[0xFE07] = 1  // Palette 1

	core.RenderSprites(core.Memory.MainMemory[0xFF40])

	// In CGB mode, Sprite 0 has priority over Sprite 1; pixel (0,0) must be RED (palette 0)
	actualColor := [3]uint8{core.Screen[0][0][0], core.Screen[0][0][1], core.Screen[0][0][2]}
	expectedColor := [3]uint8{255, 0, 0}
	if actualColor != expectedColor {
		t.Errorf("CGB Sprite priority: got %v, want %v (Sprite 0 should not be overwritten by Sprite 1)", actualColor, expectedColor)
	}
}

func TestGraphics_RenderSprites_NegativeCoordinates(t *testing.T) {
	core := newTestCore()
	core.IsCGB = true
	core.Memory.MainMemory[0xFF40] = 0x82 // LCD on, Sprites on
	core.Memory.MainMemory[0xFF44] = 0    // Scanline 0

	// Set Sprite Palette 0 color 1 = green
	core.Memory.SpritePaletteCache[0][1] = [3]uint8{0, 255, 0}

	// Tile 0: all rows have color 1 (data1 = 0xFF, data2 = 0x00)
	for i := 0; i < 16; i += 2 {
		core.Memory.VRAMBanks[0][i] = 0xFF
		core.Memory.VRAMBanks[0][i+1] = 0x00
	}

	// Sprite 0 at Y=10 (partially off-screen at top: spriteY = -6), X=4 (partially off-screen at left: spriteX = -4)
	core.Memory.MainMemory[0xFE00] = 10 // Y = 10 - 16 = -6
	core.Memory.MainMemory[0xFE01] = 4  // X = 4 - 8 = -4
	core.Memory.MainMemory[0xFE02] = 0  // Tile 0
	core.Memory.MainMemory[0xFE03] = 0  // Palette 0

	core.RenderSprites(core.Memory.MainMemory[0xFF40])

	// Screen pixel (0, 0) should be rendered with Sprite 0 color (pixel = -4 + 4 = 0)
	actualColor := [3]uint8{core.Screen[0][0][0], core.Screen[0][0][1], core.Screen[0][0][2]}
	expectedColor := [3]uint8{0, 255, 0}
	if actualColor != expectedColor {
		t.Errorf("Negative coordinate sprite: got %v, want %v", actualColor, expectedColor)
	}
}

func TestGraphics_RenderSprites_DMG_XPriority(t *testing.T) {
	core := newTestCore()
	core.IsCGB = false
	core.Memory.MainMemory[0xFF40] = 0x82 // LCD on, Sprites on
	core.Memory.MainMemory[0xFF44] = 0    // Scanline 0
	core.Memory.MainMemory[0xFF48] = 0xE4 // OBP0: 11 10 01 00 (Color 1 = dmgPalette[1])
	core.Memory.MainMemory[0xFF49] = 0x1B // OBP1: 00 01 10 11 (Color 1 = dmgPalette[2])

	// Tile 0: row 0 has color 1
	core.Memory.MainMemory[0x8000] = 0xFF
	core.Memory.MainMemory[0x8001] = 0x00

	// Sprite 0 at (X=16, Y=16), OBP1
	core.Memory.MainMemory[0xFE00] = 16 // Y
	core.Memory.MainMemory[0xFE01] = 16 // X = 8 on screen
	core.Memory.MainMemory[0xFE02] = 0  // Tile 0
	core.Memory.MainMemory[0xFE03] = 0x10 // OBP1

	// Sprite 1 at (X=12, Y=16), OBP0 -> X = 4 on screen, overlaps Sprite 0 at screen pixel 8
	core.Memory.MainMemory[0xFE04] = 16 // Y
	core.Memory.MainMemory[0xFE05] = 12 // X = 4 on screen
	core.Memory.MainMemory[0xFE06] = 0  // Tile 0
	core.Memory.MainMemory[0xFE07] = 0  // OBP0

	core.RenderSprites(core.Memory.MainMemory[0xFF40])

	// At screen pixel 8 (covered by both Sprite 1 tile pixel 4 and Sprite 0 tile pixel 0),
	// Sprite 1 has smaller X (12 < 16) so Sprite 1 (OBP0 -> dmgPalette[1]) must win
	actualColor := [3]uint8{core.Screen[8][0][0], core.Screen[8][0][1], core.Screen[8][0][2]}
	expectedColor := dmgPalette[1]
	if actualColor != expectedColor {
		t.Errorf("DMG Sprite X priority: got %v, want %v", actualColor, expectedColor)
	}
}
