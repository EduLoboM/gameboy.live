package gb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRomBankMap(t *testing.T) {
	expected := map[byte]uint16{
		0x00: 2,
		0x01: 4,
		0x02: 8,
		0x03: 16,
		0x04: 32,
		0x05: 64,
		0x06: 128,
		0x07: 256,
		0x52: 72,
		0x53: 80,
		0x54: 96,
	}

	for k, v := range expected {
		if RomBankMap[k] != v {
			t.Errorf("RomBankMap[0x%02X] = %d; want %d", k, RomBankMap[k], v)
		}
	}
}

func TestRamBankMap(t *testing.T) {
	expected := map[byte]uint8{
		0x00: 0,
		0x01: 1,
		0x02: 1,
		0x03: 4,
		0x04: 16,
		0x05: 8,
	}

	for k, v := range expected {
		if RamBankMap[k] != v {
			t.Errorf("RamBankMap[0x%02X] = %d; want %d", k, RamBankMap[k], v)
		}
	}
}

func TestMBCRom(t *testing.T) {
	romData := make([]byte, 0x8000)
	romData[0x100] = 0xAA
	romData[0x4100] = 0xBB

	mbc := &MBCRom{rom: romData}
	if val := mbc.ReadRom(0x100); val != 0xAA {
		t.Errorf("ReadRom(0x100) = 0x%02X; want 0xAA", val)
	}
	if val := mbc.ReadRomBank(0x4100); val != 0xBB {
		t.Errorf("ReadRomBank(0x4100) = 0x%02X; want 0xBB", val)
	}
	if val := mbc.ReadRamBank(0xA000); val != 0x00 {
		t.Errorf("ReadRamBank(0xA000) = 0x%02X; want 0x00", val)
	}

	// Writes to RAM on ROM only should do nothing
	mbc.WriteRamBank(0xA000, 0xCC)
	mbc.HandleBanking(0x2000, 0x01)
	mbc.Tick(1000)
}

func TestMBC1_Banking(t *testing.T) {
	// 512KB ROM = 32 banks of 16KB
	romData := make([]byte, 32*0x4000)
	for bank := 0; bank < 32; bank++ {
		romData[bank*0x4000] = byte(bank)
	}

	ramData := make([]byte, 4*0x2000)

	mbc := &MBC1{
		rom:            romData,
		CurrentROMBank: 1,
		RAMBank:        ramData,
	}

	// Bank 0 read via ReadRom
	if val := mbc.ReadRom(0x0000); val != 0 {
		t.Errorf("ReadRom(0) = %d; want 0", val)
	}

	// Select bank 5 via 0x2000-0x3FFF
	mbc.HandleBanking(0x2000, 0x05)
	if mbc.CurrentROMBank != 5 {
		t.Errorf("CurrentROMBank = %d; want 5", mbc.CurrentROMBank)
	}
	if val := mbc.ReadRomBank(0x4000); val != 5 {
		t.Errorf("ReadRomBank(0x4000) with bank 5 = %d; want 5", val)
	}

	// Bank 0 written translates to Bank 1 in MBC1
	mbc.HandleBanking(0x2000, 0x00)
	if mbc.CurrentROMBank != 1 {
		t.Errorf("CurrentROMBank = %d; want 1 after writing 0", mbc.CurrentROMBank)
	}

	// RAM enable
	mbc.HandleBanking(0x0000, 0x0A) // Enable RAM
	if !mbc.EnableRAM {
		t.Errorf("EnableRAM should be true after writing 0x0A")
	}

	// Write and read RAM
	mbc.WriteRamBank(0xA000, 0x42)
	if val := mbc.ReadRamBank(0xA000); val != 0x42 {
		t.Errorf("ReadRamBank(0xA000) = 0x%02X; want 0x42", val)
	}

	// RAM Bank switching in RAM Mode (Mode 1)
	mbc.HandleBanking(0x6000, 0x01) // RAM Banking Mode
	mbc.HandleBanking(0x4000, 0x02) // Select RAM bank 2
	if mbc.CurrentRAMBank != 2 {
		t.Errorf("CurrentRAMBank = %d; want 2", mbc.CurrentRAMBank)
	}
	mbc.WriteRamBank(0xA000, 0x99)
	if val := mbc.ReadRamBank(0xA000); val != 0x99 {
		t.Errorf("ReadRamBank bank 2 = 0x%02X; want 0x99", val)
	}

	// Bank 0 should still hold 0x42
	mbc.HandleBanking(0x4000, 0x00)
	if val := mbc.ReadRamBank(0xA000); val != 0x42 {
		t.Errorf("ReadRamBank bank 0 = 0x%02X; want 0x42", val)
	}

	// Disable RAM
	mbc.HandleBanking(0x0000, 0x00)
	if mbc.EnableRAM {
		t.Errorf("EnableRAM should be false after writing 0x00")
	}
}

func TestMBC2_Banking(t *testing.T) {
	romData := make([]byte, 16*0x4000)
	for bank := 0; bank < 16; bank++ {
		romData[bank*0x4000] = byte(bank)
	}
	ramData := make([]byte, 512)

	mbc := &MBC2{
		rom:            romData,
		CurrentROMBank: 1,
		RAMBank:        ramData,
	}

	// ROM Bank switching: address bit 8 must be 1 (e.g. 0x2100)
	mbc.HandleBanking(0x2100, 0x07)
	if mbc.CurrentROMBank != 7 {
		t.Errorf("MBC2 CurrentROMBank = %d; want 7", mbc.CurrentROMBank)
	}

	// RAM Enable: address bit 8 must be 0 (e.g. 0x0000)
	mbc.HandleBanking(0x0000, 0x0A)
	if !mbc.EnableRAM {
		t.Errorf("MBC2 EnableRAM should be true")
	}

	mbc.WriteRamBank(0xA000, 0x0F)
	if val := mbc.ReadRamBank(0xA000); val != 0x0F {
		t.Errorf("MBC2 ReadRamBank = 0x%02X; want 0x0F", val)
	}
}

func TestMBC3_RTC_TickingAndLatching(t *testing.T) {
	romData := make([]byte, 128*0x4000) // 2MB
	ramData := make([]byte, 4*0x2000)

	mbc := &MBC3{
		rom:            romData,
		CurrentROMBank: 1,
		RAMBank:        ramData,
		rtc:            make([]byte, 13),
		latchedRtc:     make([]byte, 13),
	}

	// Enable RAM and RTC
	mbc.HandleBanking(0x0000, 0x0A)

	// Set RTC time: 23 hours, 59 minutes, 58 seconds, Day 0
	mbc.HandleBanking(0x4000, 0x08) // Select Seconds
	mbc.WriteRamBank(0xA000, 58)
	mbc.HandleBanking(0x4000, 0x09) // Select Minutes
	mbc.WriteRamBank(0xA000, 59)
	mbc.HandleBanking(0x4000, 0x0A) // Select Hours
	mbc.WriteRamBank(0xA000, 23)
	mbc.HandleBanking(0x4000, 0x0B) // Select Days Low
	mbc.WriteRamBank(0xA000, 0)
	mbc.HandleBanking(0x4000, 0x0C) // Select Days High / Flags
	mbc.WriteRamBank(0xA000, 0)

	// Verify unlatched read
	if mbc.ReadRamBank(0xA000) != 0 {
		t.Errorf("Day High = %d; want 0", mbc.ReadRamBank(0xA000))
	}

	// Tick 1 second (4194304 cycles) -> 59 seconds
	mbc.Tick(4194304)
	mbc.HandleBanking(0x4000, 0x08)
	if mbc.ReadRamBank(0xA000) != 59 {
		t.Errorf("Seconds = %d; want 59", mbc.ReadRamBank(0xA000))
	}

	// Tick 1 second -> Rollover to 00:00:00, Day 1
	mbc.Tick(4194304)
	if val := mbc.ReadRamBank(0xA000); val != 0 {
		t.Errorf("Seconds after rollover = %d; want 0", val)
	}
	mbc.HandleBanking(0x4000, 0x09)
	if val := mbc.ReadRamBank(0xA000); val != 0 {
		t.Errorf("Minutes after rollover = %d; want 0", val)
	}
	mbc.HandleBanking(0x4000, 0x0A)
	if val := mbc.ReadRamBank(0xA000); val != 0 {
		t.Errorf("Hours after rollover = %d; want 0", val)
	}
	mbc.HandleBanking(0x4000, 0x0B)
	if val := mbc.ReadRamBank(0xA000); val != 1 {
		t.Errorf("Days Low after rollover = %d; want 1", val)
	}

	// Latch RTC: write 0x00 then 0x01 to 0x6000
	mbc.HandleBanking(0x6000, 0x00)
	mbc.HandleBanking(0x6000, 0x01)
	if !mbc.latched {
		t.Errorf("MBC3 should be latched")
	}

	// Tick 5 more seconds in background while latched
	mbc.Tick(5 * 4194304)
	mbc.HandleBanking(0x4000, 0x08) // Seconds

	// Latched value should still be 0
	if val := mbc.ReadRamBank(0xA000); val != 0 {
		t.Errorf("Latched seconds = %d; want 0", val)
	}

	// Unlatch and re-latch to see updated time
	mbc.HandleBanking(0x6000, 0x00)
	mbc.HandleBanking(0x6000, 0x01)
	if val := mbc.ReadRamBank(0xA000); val != 5 {
		t.Errorf("Re-latched seconds = %d; want 5", val)
	}

	// Test RTC Halt flag (bit 6 of 0x0C): when halted, time should not advance
	mbc.HandleBanking(0x4000, 0x0C)
	mbc.WriteRamBank(0xA000, 0x40) // Set Halt
	mbc.Tick(10 * 4194304)

	mbc.HandleBanking(0x6000, 0x00)
	mbc.HandleBanking(0x6000, 0x01)
	mbc.HandleBanking(0x4000, 0x08)
	if val := mbc.ReadRamBank(0xA000); val != 5 {
		t.Errorf("Halted seconds = %d; want 5 (should not advance)", val)
	}
}

func TestMBC3_DayCarryOverflow(t *testing.T) {
	mbc := &MBC3{
		rom:        make([]byte, 0x8000),
		RAMBank:    make([]byte, 0x8000),
		rtc:        make([]byte, 13),
		latchedRtc: make([]byte, 13),
		EnableRAM:  true,
	}

	// Set to Day 511 (0x1FF), 23:59:59
	mbc.rtc[0x08] = 59
	mbc.rtc[0x09] = 59
	mbc.rtc[0x0A] = 23
	mbc.rtc[0x0B] = 0xFF // Low 8 bits of 511
	mbc.rtc[0x0C] = 0x01 // Bit 0 is Day bit 8

	// Tick 1 second -> Day 512 (overflow -> Day 0, Carry bit 7 set in 0x0C)
	mbc.Tick(4194304)

	if mbc.rtc[0x0B] != 0 {
		t.Errorf("Day Low = %d; want 0 after 512-day overflow", mbc.rtc[0x0B])
	}
	if mbc.rtc[0x0C]&0x80 == 0 {
		t.Errorf("Day Carry bit 7 should be set in 0x0C; got 0x%02X", mbc.rtc[0x0C])
	}
}

func TestMBC5_Banking(t *testing.T) {
	// 4MB ROM (256 banks)
	romData := make([]byte, 256*0x4000)
	for b := 0; b < 256; b++ {
		romData[b*0x4000] = byte(b)
	}
	ramData := make([]byte, 16*0x2000) // 128KB RAM (16 banks)

	mbc := &MBC5{
		rom:     romData,
		RAMBank: ramData,
	}

	// Enable RAM
	mbc.HandleBanking(0x0000, 0x0A)
	if !mbc.EnableRAM {
		t.Errorf("MBC5 EnableRAM should be true")
	}

	// Select bank 0 (MBC5 allows bank 0 in 0x4000-0x7FFF!)
	mbc.HandleBanking(0x2000, 0x00)
	if val := mbc.ReadRomBank(0x4000); val != 0 {
		t.Errorf("MBC5 ReadRomBank for bank 0 = %d; want 0", val)
	}

	// Select bank 200 (Lo = 200, Hi = 0)
	mbc.HandleBanking(0x2000, 200)
	if val := mbc.ReadRomBank(0x4000); val != 200 {
		t.Errorf("MBC5 ReadRomBank for bank 200 = %d; want 200", val)
	}

	// Test 16 RAM banks (0..15)
	for b := byte(0); b < 16; b++ {
		mbc.HandleBanking(0x4000, b)
		mbc.WriteRamBank(0xA000, b*10)
	}
	for b := byte(0); b < 16; b++ {
		mbc.HandleBanking(0x4000, b)
		if val := mbc.ReadRamBank(0xA000); val != b*10 {
			t.Errorf("MBC5 RAM bank %d = %d; want %d", b, val, b*10)
		}
	}
}

func TestCartridgeSaveRam(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cart_save_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	savePath := filepath.Join(tmpDir, "test.sav")
	ram := []byte{0xDE, 0xAD, 0xBE, 0xEF}

	mbc := &MBC1{RAMBank: ram}
	mbc.SaveRam(savePath)

	loaded := readDataFile(savePath, true)
	if len(loaded) != 4 || loaded[0] != 0xDE || loaded[3] != 0xEF {
		t.Errorf("loaded RAM mismatch: %v", loaded)
	}
}
