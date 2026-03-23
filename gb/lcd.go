package gb

/*
FF41 - STAT - LCDC Status (R/W)

	Bit 6 - LYC=LY Coincidence Interrupt (1=Enable) (Read/Write)
	Bit 5 - Mode 2 OAM Interrupt         (1=Enable) (Read/Write)
	Bit 4 - Mode 1 V-Blank Interrupt     (1=Enable) (Read/Write)
	Bit 3 - Mode 0 H-Blank Interrupt     (1=Enable) (Read/Write)
	Bit 2 - Coincidence Flag  (0:LYC<>LY, 1:LYC=LY) (Read Only)
	Bit 1-0 - Mode Flag       (Mode 0-3, see below) (Read Only)
*/
func (core *Core) SetLCDStatus() {
	status := core.Memory.MainMemory[0xFF41]
	if core.Memory.MainMemory[0xFF40]&0x80 == 0 {
		core.Timer.ScanlineCounter = 456
		core.Memory.MainMemory[0xFF44] = 0
		status &= 0xFC
		core.Memory.MainMemory[0xFF41] = status
		return
	}

	currentLine := core.Memory.MainMemory[0xFF44]
	currentMode := status & 0x3
	mode := byte(0)
	reqInt := false

	if currentLine >= 144 {
		mode = 1
		status = (status & 0xFC) | 0x01
		reqInt = status&0x10 != 0
	} else {
		mode2bounds := 456 - 80
		mode3bounds := mode2bounds - 172

		if core.Timer.ScanlineCounter >= mode2bounds {
			mode = 2
			status = (status & 0xFC) | 0x02
			reqInt = status&0x20 != 0
		} else if core.Timer.ScanlineCounter >= mode3bounds {
			mode = 3
			status = (status & 0xFC) | 0x03
		} else {
			mode = 0
			status = status & 0xFC
			reqInt = status&0x08 != 0
		}
	}

	if reqInt && (mode != currentMode) {
		core.RequestInterrupt(1)
	}

	if core.IsCGB && mode == 0 && currentMode != 0 {
		core.doHDMABlock()
	}

	if currentLine == core.Memory.MainMemory[0xFF45] {
		status |= 0x04
		if status&0x40 != 0 {
			core.RequestInterrupt(1)
		}
	} else {
		status &^= 0x04
	}
	core.Memory.MainMemory[0xFF41] = status
}

func (core *Core) IsLCDEnabled() bool {
	return core.Memory.MainMemory[0xFF40]&0x80 != 0
}

func (core *Core) UpdateGraphics(cycles int) {
	core.SetLCDStatus()

	if core.Memory.MainMemory[0xFF40]&0x80 == 0 {
		return
	}
	core.Timer.ScanlineCounter -= cycles

	if core.Timer.ScanlineCounter <= 0 {
		core.Memory.MainMemory[0xFF44]++

		currentLine := core.Memory.MainMemory[0xFF44]

		core.Timer.ScanlineCounter += 456

		if currentLine == 144 {
			core.RequestInterrupt(0)
		} else if currentLine > 153 {
			core.Memory.MainMemory[0xFF44] = 0
			core.DrawScanLine()
		} else if currentLine < 144 {
			core.DrawScanLine()
		}
	}
}
