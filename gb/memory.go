package gb

import (
	"log"
	"os"
	"time"

	"github.com/HFO4/gbc-in-cloud/util"
)

type Memory struct {
	MainMemory [0x10000]byte
	dirty      bool

	VRAMBanks        [2][0x2000]byte
	WRAMBanks        [8][0x1000]byte
	VRAMBankSelected byte
	WRAMBankSelected byte

	BGPaletteData      [64]byte
	SpritePaletteData  [64]byte
	BGPaletteIndex     byte
	SpritePaletteIndex byte

	BGPaletteCache     [8][4][3]uint8
	SpritePaletteCache [8][4][3]uint8

	HDMASource    uint16
	HDMADest      uint16
	HDMAActive    bool
	HDMALength    byte
	HDMARemaining int
}

func (core *Core) initMemory() {
	log.Println("[Core] Start to initialize memory...")

	log.Println("[Memory] Load first 32KByte of rom data into memory")
	for i := 0x0000; i < core.Cartridge.Props.ROMLength && i < 0x8000; i++ {
		core.Memory.MainMemory[i] = core.Cartridge.MBC.ReadRom(uint16(i))
	}

	core.Memory.MainMemory[0xFF05] = 0x00
	core.Memory.MainMemory[0xFF06] = 0x00
	core.Memory.MainMemory[0xFF07] = 0x00
	core.Memory.MainMemory[0xFF0F] = 0xE1
	core.Memory.MainMemory[0xFF10] = 0x80
	core.Memory.MainMemory[0xFF11] = 0xBF
	core.Memory.MainMemory[0xFF12] = 0xF3
	core.Memory.MainMemory[0xFF14] = 0xBF
	core.Memory.MainMemory[0xFF16] = 0x3F
	core.Memory.MainMemory[0xFF17] = 0x00
	core.Memory.MainMemory[0xFF19] = 0xBF
	core.Memory.MainMemory[0xFF1A] = 0x7F
	core.Memory.MainMemory[0xFF1B] = 0xFF
	core.Memory.MainMemory[0xFF1C] = 0x9F
	core.Memory.MainMemory[0xFF1E] = 0xBF
	core.Memory.MainMemory[0xFF20] = 0xFF
	core.Memory.MainMemory[0xFF21] = 0x00
	core.Memory.MainMemory[0xFF22] = 0x00
	core.Memory.MainMemory[0xFF23] = 0xBF
	core.Memory.MainMemory[0xFF24] = 0x77
	core.Memory.MainMemory[0xFF25] = 0xF3
	core.Memory.MainMemory[0xFF26] = 0xF1
	core.Memory.MainMemory[0xFF40] = 0x91
	core.Memory.MainMemory[0xFF42] = 0x00
	core.Memory.MainMemory[0xFF43] = 0x00
	core.Memory.MainMemory[0xFF45] = 0x00
	core.Memory.MainMemory[0xFF47] = 0xFC
	core.Memory.MainMemory[0xFF48] = 0xFF
	core.Memory.MainMemory[0xFF49] = 0xFF
	core.Memory.MainMemory[0xFF4A] = 0x00
	core.Memory.MainMemory[0xFF4B] = 0x00
	core.Memory.MainMemory[0xFFFF] = 0x00

	if core.IsCGB {
		core.Memory.VRAMBankSelected = 0
		core.Memory.WRAMBankSelected = 1
	}

	core.setupSaveLoop()
}

func (core *Core) SaveRAM() {
	if core.Memory.dirty {
		core.Memory.dirty = false
		core.Cartridge.MBC.SaveRam(core.RamPath)
		util.TriggerSaveSync(core.RamPath)
	}
}

func (core *Core) setupSaveLoop() {
	saveTimer := time.Tick(time.Second)
	go func() {
		for range saveTimer {
			core.SaveRAM()
		}
	}()
}

func (core *Core) ReadMemory(address uint16) byte {
	switch {
	case address < 0x4000:
		return core.Memory.MainMemory[address]
	case address <= 0x7FFF:
		return core.Cartridge.MBC.ReadRomBank(address)
	case address <= 0x9FFF:
		if core.IsCGB {
			return core.Memory.VRAMBanks[core.Memory.VRAMBankSelected][address-0x8000]
		}
		return core.Memory.MainMemory[address]
	case address <= 0xBFFF:
		return core.Cartridge.MBC.ReadRamBank(address)
	case address <= 0xCFFF:
		return core.Memory.MainMemory[address]
	case address <= 0xDFFF:
		if core.IsCGB {
			bank := core.Memory.WRAMBankSelected
			if bank == 0 {
				bank = 1
			}
			return core.Memory.WRAMBanks[bank][address-0xD000]
		}
		return core.Memory.MainMemory[address]
	case address == 0xFF00:
		return core.GetJoypadStatus()
	case address == 0xFF01:
		return core.SerialByte
	case address >= 0xFF4D && address <= 0xFF70 && core.IsCGB:
		switch address {
		case 0xFF4D:
			return core.Memory.MainMemory[0xFF4D]
		case 0xFF4F:
			return core.Memory.VRAMBankSelected | 0xFE
		case 0xFF55:
			if core.Memory.HDMAActive {
				return core.Memory.HDMALength
			}
			return 0xFF
		case 0xFF68:
			return core.Memory.BGPaletteIndex
		case 0xFF69:
			return core.Memory.BGPaletteData[core.Memory.BGPaletteIndex&0x3F]
		case 0xFF6A:
			return core.Memory.SpritePaletteIndex
		case 0xFF6B:
			return core.Memory.SpritePaletteData[core.Memory.SpritePaletteIndex&0x3F]
		case 0xFF70:
			return core.Memory.WRAMBankSelected
		}
	}
	return core.Memory.MainMemory[address]
}

func (core *Core) WriteMemory(address uint16, data byte) {
	if address < 0x8000 {
		core.Cartridge.MBC.HandleBanking(address, data)
	} else if core.IsCGB && (address >= 0x8000) && (address <= 0x9FFF) {
		core.Memory.VRAMBanks[core.Memory.VRAMBankSelected][address-0x8000] = data
		core.Memory.MainMemory[address] = data
	} else if (address >= 0xA000) && (address < 0xC000) {
		core.Cartridge.MBC.WriteRamBank(address, data)
		core.Memory.dirty = true
	} else if core.IsCGB && (address >= 0xD000) && (address <= 0xDFFF) {
		bank := core.Memory.WRAMBankSelected
		if bank == 0 {
			bank = 1
		}
		core.Memory.WRAMBanks[bank][address-0xD000] = data
		core.Memory.MainMemory[address] = data
	} else if (address >= 0xE000) && (address < 0xFE00) {
		core.Memory.MainMemory[address] = data
		core.WriteMemory(address-0x2000, data)
	} else if (address >= 0xFEA0) && (address < 0xFF00) {
		// restricted
	} else if 0xFF04 == address {
		core.Memory.MainMemory[0xFF04] = 0
	} else if address == 0xFF44 {
		core.Memory.MainMemory[0xFF44] = 0
	} else if address == 0xFF46 {
		core.DoDMA(data)
	} else if address == 0xFF07 {
		currentFreq := core.GetClockFreq()
		core.Memory.MainMemory[0xFF07] = data
		newFreq := core.GetClockFreq()
		if currentFreq != newFreq {
			core.SetClockFreq()
		}
	} else if address >= 0xFF10 && address <= 0xFF3F {
		core.Memory.MainMemory[address] = data
		if core.ToggleSound {
			core.Sound.Trigger(address, data, core.Memory.MainMemory[0xFF10:0xFF40])
		}
	} else if address == 0xFF02 {
		core.Serial.SetChannelStatus(util.TestBit(data, 0), util.TestBit(data, 7))
		if util.TestBit(data, 7) {
			if util.TestBit(data, 0) {
			}
			core.Serial.SendByte(core.Memory.MainMemory[0xFF01])
		}
		core.Memory.MainMemory[address] = data
	} else if core.IsCGB && address == 0xFF4D {
		// Speed switch — only bit 0 is writable
		current := core.Memory.MainMemory[0xFF4D]
		core.Memory.MainMemory[0xFF4D] = (current & 0x80) | (data & 0x01)
	} else if core.IsCGB && address == 0xFF4F {
		core.Memory.VRAMBankSelected = data & 0x01
	} else if core.IsCGB && address == 0xFF51 {
		core.Memory.HDMASource = (core.Memory.HDMASource & 0x00FF) | (uint16(data) << 8)
	} else if core.IsCGB && address == 0xFF52 {
		core.Memory.HDMASource = (core.Memory.HDMASource & 0xFF00) | (uint16(data) & 0xF0)
	} else if core.IsCGB && address == 0xFF53 {
		core.Memory.HDMADest = (core.Memory.HDMADest & 0x00FF) | (uint16(data&0x1F) << 8)
	} else if core.IsCGB && address == 0xFF54 {
		core.Memory.HDMADest = (core.Memory.HDMADest & 0xFF00) | (uint16(data) & 0xF0)
	} else if core.IsCGB && address == 0xFF55 {
		core.doHDMA(data)
	} else if core.IsCGB && address == 0xFF68 {
		core.Memory.BGPaletteIndex = data
	} else if core.IsCGB && address == 0xFF69 {
		idx := core.Memory.BGPaletteIndex & 0x3F
		core.Memory.BGPaletteData[idx] = data
		core.updateBGPaletteCache(idx)
		if core.Memory.BGPaletteIndex&0x80 != 0 {
			core.Memory.BGPaletteIndex = 0x80 | ((idx + 1) & 0x3F)
		}
	} else if core.IsCGB && address == 0xFF6A {
		core.Memory.SpritePaletteIndex = data
	} else if core.IsCGB && address == 0xFF6B {
		idx := core.Memory.SpritePaletteIndex & 0x3F
		core.Memory.SpritePaletteData[idx] = data
		core.updateSpritePaletteCache(idx)
		if core.Memory.SpritePaletteIndex&0x80 != 0 {
			core.Memory.SpritePaletteIndex = 0x80 | ((idx + 1) & 0x3F)
		}
	} else if core.IsCGB && address == 0xFF70 {
		core.Memory.WRAMBankSelected = data & 0x07
		if core.Memory.WRAMBankSelected == 0 {
			core.Memory.WRAMBankSelected = 1
		}
	} else {
		core.Memory.MainMemory[address] = data
	}
}

func (core *Core) doHDMA(data byte) {
	if core.Memory.HDMAActive {
		if data&0x80 == 0 {
			core.Memory.HDMAActive = false
			core.Memory.HDMALength = data | 0x80
			return
		}
	}

	length := int((data&0x7F)+1) * 16

	if data&0x80 == 0 {
		src := core.Memory.HDMASource
		dst := core.Memory.HDMADest | 0x8000
		for i := 0; i < length; i++ {
			b := core.ReadMemory(src + uint16(i))
			core.WriteMemory(dst+uint16(i), b)
		}
		core.Memory.HDMAActive = false
		core.Memory.HDMALength = 0xFF
	} else {
		core.Memory.HDMAActive = true
		core.Memory.HDMARemaining = length
		core.Memory.HDMALength = data & 0x7F
	}
}

func (core *Core) doHDMABlock() {
	if !core.Memory.HDMAActive {
		return
	}

	src := core.Memory.HDMASource
	dst := core.Memory.HDMADest | 0x8000

	for i := 0; i < 16; i++ {
		b := core.ReadMemory(src + uint16(i))
		core.WriteMemory(dst+uint16(i), b)
	}

	core.Memory.HDMASource += 16
	core.Memory.HDMADest += 16
	core.Memory.HDMARemaining -= 16

	if core.Memory.HDMARemaining <= 0 {
		core.Memory.HDMAActive = false
		core.Memory.HDMALength = 0xFF
	} else {
		core.Memory.HDMALength = byte((core.Memory.HDMARemaining/16)-1) & 0x7F
	}
}

func (core *Core) DoDMA(data byte) {
	address := uint16(data) << 8
	for i := 0; i < 0xA0; i++ {
		core.WriteMemory(0xFE00+uint16(i), core.ReadMemory(address+uint16(i)))
	}
}

func (core *Core) StackPush(val uint16) {
	hi := val >> 8
	lo := val & 0xFF
	core.CPU.Registers.SP--
	core.WriteMemory(core.CPU.Registers.SP, byte(hi))
	core.CPU.Registers.SP--
	core.WriteMemory(core.CPU.Registers.SP, byte(lo))

	if core.Debug {
		//log.Printf("[Debug] Stack Push: %X, SP:%X", val, core.CPU.Registers.SP)
	}
}

func (core *Core) StackPop() uint16 {
	lo := core.ReadMemory(core.CPU.Registers.SP)
	hi := core.ReadMemory(core.CPU.Registers.SP + 1)
	core.CPU.Registers.SP += 2
	return uint16(lo) + (uint16(hi) << 8)
}

func (memory *Memory) Dump(path string) {
	err := os.WriteFile(path, memory.MainMemory[:], 0644)
	if err != nil {
		panic(err)
	}
}

func (core *Core) updateBGPaletteCache(byteIdx byte) {
	colorIdx := byteIdx / 2
	paletteNum := colorIdx / 4
	colorInPalette := colorIdx % 4
	base := int(colorIdx) * 2
	lo := core.Memory.BGPaletteData[base]
	hi := core.Memory.BGPaletteData[base+1]
	rgb555 := uint16(hi)<<8 | uint16(lo)
	core.Memory.BGPaletteCache[paletteNum][colorInPalette][0] = uint8(((rgb555 & 0x1F) * 255) / 31)
	core.Memory.BGPaletteCache[paletteNum][colorInPalette][1] = uint8((((rgb555 >> 5) & 0x1F) * 255) / 31)
	core.Memory.BGPaletteCache[paletteNum][colorInPalette][2] = uint8((((rgb555 >> 10) & 0x1F) * 255) / 31)
}

func (core *Core) updateSpritePaletteCache(byteIdx byte) {
	colorIdx := byteIdx / 2
	paletteNum := colorIdx / 4
	colorInPalette := colorIdx % 4
	base := int(colorIdx) * 2
	lo := core.Memory.SpritePaletteData[base]
	hi := core.Memory.SpritePaletteData[base+1]
	rgb555 := uint16(hi)<<8 | uint16(lo)
	core.Memory.SpritePaletteCache[paletteNum][colorInPalette][0] = uint8(((rgb555 & 0x1F) * 255) / 31)
	core.Memory.SpritePaletteCache[paletteNum][colorInPalette][1] = uint8((((rgb555 >> 5) & 0x1F) * 255) / 31)
	core.Memory.SpritePaletteCache[paletteNum][colorInPalette][2] = uint8((((rgb555 >> 10) & 0x1F) * 255) / 31)
}
