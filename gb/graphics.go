package gb

var dmgPalette = [4][3]uint8{
	{0x9b, 0xbc, 0x0f},
	{0x8b, 0xac, 0x0f},
	{0x30, 0x62, 0x30},
	{0x0f, 0x38, 0x0f},
}

func testBit(n byte, pos uint) bool {
	return n&(1<<pos) != 0
}

func (core *Core) DrawScanLine() {
	control := core.Memory.MainMemory[0xFF40]

	if core.IsCGB || testBit(control, 0) {
		core.RenderTiles(control)
	}

	if testBit(control, 1) {
		core.RenderSprites(control)
	}
}

func (core *Core) RenderTiles(lcdControl byte) {
	var tileData uint16
	var backgroundMemory uint16
	unsig := true

	scrollY := core.Memory.MainMemory[0xFF42]
	scrollX := core.Memory.MainMemory[0xFF43]
	windowY := core.Memory.MainMemory[0xFF4A]
	windowX := int(core.Memory.MainMemory[0xFF4B]) - 7
	scanline := core.Memory.MainMemory[0xFF44]

	usingWindow := false
	if testBit(lcdControl, 5) && windowY <= scanline {
		usingWindow = true
	}

	if testBit(lcdControl, 4) {
		tileData = 0x8000
	} else {
		tileData = 0x8800
		unsig = false
	}

	if !usingWindow {
		if testBit(lcdControl, 3) {
			backgroundMemory = 0x9C00
		} else {
			backgroundMemory = 0x9800
		}
	} else {
		if testBit(lcdControl, 6) {
			backgroundMemory = 0x9C00
		} else {
			backgroundMemory = 0x9800
		}
	}

	var yPos byte
	if !usingWindow {
		yPos = scrollY + scanline
	} else {
		yPos = scanline - windowY
	}

	tileRow := (uint16(yPos / 8)) * 32
	isCGB := core.IsCGB
	finally := int(scanline)
	if finally < 0 || finally > 143 {
		return
	}

	tileLine := yPos % 8

	// Tile row caching: track previously loaded tile data to avoid re-reading
	var cachedTileCol uint16 = 0xFFFF
	var cachedData1, cachedData2 byte
	var cachedXFlip bool
	var cachedBgPalette byte
	var cachedVramBank byte

	for pixel := byte(0); pixel < 160; pixel++ {
		xPos := int(pixel + scrollX)

		if usingWindow && int(pixel) >= windowX {
			xPos = int(pixel) - windowX
		}

		tileCol := uint16(xPos / 8)
		tileAddress := backgroundMemory + tileRow + tileCol

		// Only reload tile data when we move to a new tile column
		if tileCol != cachedTileCol {
			cachedTileCol = tileCol

			var tileNum int16
			if isCGB {
				rawTile := core.Memory.VRAMBanks[0][tileAddress-0x8000]
				if unsig {
					tileNum = int16(rawTile)
				} else {
					tileNum = int16(int8(rawTile))
				}
			} else {
				if unsig {
					tileNum = int16(core.Memory.MainMemory[tileAddress])
				} else {
					tileNum = int16(int8(core.Memory.MainMemory[tileAddress]))
				}
			}

			var tileLocation uint16
			if unsig {
				tileLocation = tileData + uint16(tileNum*16)
			} else {
				tileLocation = uint16(int32(tileData) + int32((tileNum+128)*16))
			}

			if isCGB {
				tileAttr := core.Memory.VRAMBanks[1][tileAddress-0x8000]
				cachedBgPalette = tileAttr & 0x07
				cachedVramBank = (tileAttr >> 3) & 0x01
				cachedXFlip = tileAttr&0x20 != 0
				yFlip := tileAttr&0x40 != 0

				line := tileLine
				if yFlip {
					line = 7 - line
				}
				line *= 2

				offset := tileLocation - 0x8000 + uint16(line)
				cachedData1 = core.Memory.VRAMBanks[cachedVramBank][offset]
				cachedData2 = core.Memory.VRAMBanks[cachedVramBank][offset+1]
			} else {
				line := tileLine * 2
				cachedData1 = core.Memory.MainMemory[tileLocation+uint16(line)]
				cachedData2 = core.Memory.MainMemory[tileLocation+uint16(line)+1]
			}
		}

		if isCGB {
			colourBit := uint(xPos % 8)
			if !cachedXFlip {
				colourBit = 7 - colourBit
			}

			colourNum := ((cachedData2 >> colourBit) & 1) << 1
			colourNum |= (cachedData1 >> colourBit) & 1

			core.ScanLineBG[pixel] = colourNum == 0

			c := core.Memory.BGPaletteCache[cachedBgPalette][colourNum]
			core.Screen[pixel][finally][0] = c[0]
			core.Screen[pixel][finally][1] = c[1]
			core.Screen[pixel][finally][2] = c[2]
		} else {
			colourBit := uint(7 - (xPos % 8))

			colourNum := ((cachedData2 >> colourBit) & 1) << 1
			colourNum |= (cachedData1 >> colourBit) & 1

			palette := core.Memory.MainMemory[0xFF47]
			shift := uint(colourNum) * 2
			colour := (palette >> shift) & 0x03

			core.ScanLineBG[pixel] = colour == 0

			c := dmgPalette[colour]
			core.Screen[pixel][finally][0] = c[0]
			core.Screen[pixel][finally][1] = c[1]
			core.Screen[pixel][finally][2] = c[2]
		}
	}
}

func (core *Core) RenderSprites(lcdControl byte) {
	use8x16 := testBit(lcdControl, 2)
	scanline := core.Memory.MainMemory[0xFF44]
	isCGB := core.IsCGB

	ysize := byte(8)
	if use8x16 {
		ysize = 16
	}

	oam := &core.Memory.MainMemory
	spriteCount := 0

	for sprite := 0; sprite < 40; sprite++ {
		base := 0xFE00 + sprite*4
		spriteY := oam[base] - 16
		spriteX := oam[base+1] - 8
		tileLoc := oam[base+2]
		attributes := oam[base+3]

		if scanline < spriteY || scanline >= spriteY+ysize {
			continue
		}

		spriteCount++
		if spriteCount > 10 {
			break
		}

		yFlip := attributes&0x40 != 0
		xFlip := attributes&0x20 != 0
		priority := attributes&0x80 == 0

		line := int(scanline - spriteY)
		if yFlip {
			line = int(ysize) - 1 - line
		}
		line *= 2
		dataAddress := uint16(int(tileLoc)*16 + line)

		var data1, data2 byte
		if isCGB {
			vramBank := byte(0)
			if attributes&0x08 != 0 {
				vramBank = 1
			}
			data1 = core.Memory.VRAMBanks[vramBank][dataAddress]
			data2 = core.Memory.VRAMBanks[vramBank][dataAddress+1]
		} else {
			data1 = core.Memory.MainMemory[0x8000+dataAddress]
			data2 = core.Memory.MainMemory[0x8000+dataAddress+1]
		}

		for tilePixel := 7; tilePixel >= 0; tilePixel-- {
			colourbit := uint(tilePixel)
			if xFlip {
				colourbit = uint(7 - tilePixel)
			}

			colourNum := ((data2 >> colourbit) & 1) << 1
			colourNum |= (data1 >> colourbit) & 1

			if colourNum == 0 {
				continue
			}

			pixel := int(spriteX) + (7 - tilePixel)
			if pixel < 0 || pixel > 159 || scanline > 143 {
				continue
			}

			if !core.ScanLineBG[pixel] && !priority {
				continue
			}

			var c [3]uint8
			if isCGB {
				cgbPalette := attributes & 0x07
				c = core.Memory.SpritePaletteCache[cgbPalette][colourNum]
			} else {
				palAddr := uint16(0xFF48)
				if attributes&0x10 != 0 {
					palAddr = 0xFF49
				}
				palette := core.Memory.MainMemory[palAddr]
				shift := uint(colourNum) * 2
				colour := (palette >> shift) & 0x03
				c = dmgPalette[colour]
			}

			core.Screen[pixel][scanline][0] = c[0]
			core.Screen[pixel][scanline][1] = c[1]
			core.Screen[pixel][scanline][2] = c[2]
		}
	}
}

func (core *Core) GetColour(colourNum byte, address uint16) int {
	palette := core.ReadMemory(address)
	shift := uint(colourNum) * 2
	return int((palette >> shift) & 0x03)
}

func (core *Core) GetCGBColour(colourNum byte, paletteNum byte, isSprite bool) (uint8, uint8, uint8) {
	if isSprite {
		c := core.Memory.SpritePaletteCache[paletteNum][colourNum]
		return c[0], c[1], c[2]
	}
	c := core.Memory.BGPaletteCache[paletteNum][colourNum]
	return c[0], c[1], c[2]
}

func (core *Core) RenderScreen() {
	core.DrawSignal <- true
}
