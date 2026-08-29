package gb

import "sort"

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
	} else {
		scanline := int(core.Memory.MainMemory[0xFF44])
		if scanline >= 0 && scanline < 144 {
			for pixel := 0; pixel < 160; pixel++ {
				core.ScanLineBG[pixel] = true
				core.ScanLineBGPriority[pixel] = false
				c := dmgPalette[0]
				core.Screen[pixel][scanline][0] = c[0]
				core.Screen[pixel][scanline][1] = c[1]
				core.Screen[pixel][scanline][2] = c[2]
			}
		}
	}

	if testBit(control, 1) {
		core.RenderSprites(control)
	}
}

func (core *Core) RenderTiles(lcdControl byte) {
	var tileData uint16
	var backgroundMemory uint16
	var windowMemory uint16
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

	// Select background memory
	if testBit(lcdControl, 3) {
		backgroundMemory = 0x9C00
	} else {
		backgroundMemory = 0x9800
	}

	// Select window memory
	if testBit(lcdControl, 6) {
		windowMemory = 0x9C00
	} else {
		windowMemory = 0x9800
	}

	isCGB := core.IsCGB
	finally := int(scanline)
	if finally < 0 || finally > 143 {
		return
	}

	var cachedTileCol uint16 = 0xFFFF
	var cachedIsWindow bool = false
	var cachedData1, cachedData2 byte
	var cachedXFlip bool
	var cachedBgPalette byte
	var cachedVramBank byte
	var cachedBgPriority bool
	var currentMemoryBase uint16

	for pixel := byte(0); pixel < 160; pixel++ {
		xPos := int(pixel + scrollX)
		useWindowForThisPixel := usingWindow && int(pixel) >= windowX

		var yPos byte
		if useWindowForThisPixel {
			xPos = int(pixel) - windowX
			yPos = scanline - windowY
			currentMemoryBase = windowMemory
		} else {
			yPos = scrollY + scanline
			currentMemoryBase = backgroundMemory
		}

		tileRow := (uint16(yPos / 8)) * 32
		tileLine := yPos % 8

		tileCol := uint16((xPos / 8) % 32)
		tileAddress := currentMemoryBase + ((tileRow + tileCol) & 0x3FF)

		if tileCol != cachedTileCol || useWindowForThisPixel != cachedIsWindow {
			cachedTileCol = tileCol
			cachedIsWindow = useWindowForThisPixel

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
				cachedBgPriority = tileAttr&0x80 != 0

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
			core.ScanLineBGPriority[pixel] = cachedBgPriority && colourNum != 0

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

type spriteEntry struct {
	index      int
	x          int
	y          int
	tileLoc    byte
	attributes byte
}

func (core *Core) RenderSprites(lcdControl byte) {
	use8x16 := testBit(lcdControl, 2)
	scanline := int(core.Memory.MainMemory[0xFF44])
	if scanline > 143 {
		return
	}
	isCGB := core.IsCGB
	bgMasterPriority := testBit(lcdControl, 0)

	ysize := 8
	if use8x16 {
		ysize = 16
	}

	oam := &core.Memory.MainMemory

	var spritesOnLine []spriteEntry
	for sprite := 0; sprite < 40; sprite++ {
		base := 0xFE00 + sprite*4
		spriteY := int(oam[base]) - 16
		spriteX := int(oam[base+1]) - 8

		if scanline < spriteY || scanline >= spriteY+ysize {
			continue
		}

		tileLoc := oam[base+2]
		if use8x16 {
			tileLoc &= 0xFE
		}
		attributes := oam[base+3]

		spritesOnLine = append(spritesOnLine, spriteEntry{
			index:      sprite,
			x:          spriteX,
			y:          spriteY,
			tileLoc:    tileLoc,
			attributes: attributes,
		})

		if len(spritesOnLine) >= 10 {
			break
		}
	}

	if len(spritesOnLine) == 0 {
		return
	}

	// In DMG mode, priority is determined by smallest X coordinate (OAM index breaks ties)
	if !isCGB {
		sort.SliceStable(spritesOnLine, func(i, j int) bool {
			if spritesOnLine[i].x != spritesOnLine[j].x {
				return spritesOnLine[i].x < spritesOnLine[j].x
			}
			return spritesOnLine[i].index < spritesOnLine[j].index
		})
	}

	var spriteDrawn [160]bool

	for _, sp := range spritesOnLine {
		yFlip := sp.attributes&0x40 != 0
		xFlip := sp.attributes&0x20 != 0
		priority := sp.attributes&0x80 == 0

		line := scanline - sp.y
		if yFlip {
			line = ysize - 1 - line
		}
		line *= 2
		dataAddress := uint16(int(sp.tileLoc)*16 + line)

		var data1, data2 byte
		if isCGB {
			vramBank := byte(0)
			if sp.attributes&0x08 != 0 {
				vramBank = 1
			}
			data1 = core.Memory.VRAMBanks[vramBank][dataAddress]
			data2 = core.Memory.VRAMBanks[vramBank][dataAddress+1]
		} else {
			data1 = core.Memory.MainMemory[0x8000+dataAddress]
			data2 = core.Memory.MainMemory[0x8000+dataAddress+1]
		}

		for tilePixel := 0; tilePixel < 8; tilePixel++ {
			pixel := sp.x + tilePixel
			if pixel < 0 || pixel >= 160 {
				continue
			}

			if spriteDrawn[pixel] {
				continue
			}

			colourbit := uint(7 - tilePixel)
			if xFlip {
				colourbit = uint(tilePixel)
			}

			colourNum := ((data2 >> colourbit) & 1) << 1
			colourNum |= (data1 >> colourbit) & 1

			if colourNum == 0 {
				continue
			}

			spriteDrawn[pixel] = true

			if isCGB {
				if bgMasterPriority {
					if core.ScanLineBGPriority[pixel] {
						continue
					}
					if !core.ScanLineBG[pixel] && !priority {
						continue
					}
				}
			} else {
				if !core.ScanLineBG[pixel] && !priority {
					continue
				}
			}

			var c [3]uint8
			if isCGB {
				cgbPalette := sp.attributes & 0x07
				c = core.Memory.SpritePaletteCache[cgbPalette][colourNum]
			} else {
				palAddr := uint16(0xFF48)
				if sp.attributes&0x10 != 0 {
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
	core.DrawSignal <- core.Screen
	if core.Screen == &core.Buffers[0] {
		core.Screen = &core.Buffers[1]
	} else {
		core.Screen = &core.Buffers[0]
	}
}
