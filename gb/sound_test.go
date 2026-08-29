package gb

import (
	"testing"
)

func TestSound_MasterControl(t *testing.T) {
	sound := &Sound{}

	// NR52 (0xFF26): Sound ON/OFF
	sound.Trigger(0xFF26, 0x80, nil) // Master sound ON (bit 7)
	if !sound.enable {
		t.Errorf("Sound enable should be true after writing 0x80 to NR52")
	}

	sound.Trigger(0xFF26, 0x00, nil) // Master sound OFF
	if sound.enable {
		t.Errorf("Sound enable should be false after writing 0x00 to NR52")
	}

	// NR50 (0xFF24): Master Volume
	sound.Trigger(0xFF24, 0x73, nil) // Right=7, Left=3
	if sound.rightVolume != 7 || sound.leftVolume != 3 {
		t.Errorf("Volume Left=%d, Right=%d; want Left=3, Right=7", sound.leftVolume, sound.rightVolume)
	}
}

func TestSound_WaveRAM_SampleCache(t *testing.T) {
	sound := &Sound{}

	// Create 16 bytes of Wave RAM data (32 nibbles)
	vram := make([]byte, 0x40)
	for i := 0; i < 16; i++ {
		vram[0x20+i] = 0xF0 // High nibble = 0xF (1.0), Low nibble = 0x0 (0.0)
	}

	sound.Trigger(0xFF30, 0x00, vram)

	for i := 0; i < 32; i++ {
		expected := 0.0
		if i%2 == 0 {
			expected = 1.0 // 0xF / 0xF = 1.0
		}
		if sound.SampleCache[i] != expected {
			t.Errorf("SampleCache[%d] = %f; want %f", i, sound.SampleCache[i], expected)
		}
	}
}

func TestSound_Channel1_SweepCalculation(t *testing.T) {
	channel := &Channel{
		Freq:          440,
		freqLast:      440,
		sweepNumber:   2,
		sweepIncrease: true,
	}

	// Frequency sweep increase formula: freqLast + (freqLast >> sweepNumber)
	// 440 + (440 >> 2) = 440 + 110 = 550
	expectedNewFreq := 440 + (440 >> 2)
	newFreq := channel.freqLast + (channel.freqLast >> uint(channel.sweepNumber))
	if newFreq != expectedNewFreq {
		t.Errorf("Sweep newFreq = %d; want %d", newFreq, expectedNewFreq)
	}

	// Sweep decrease formula: 440 - (440 >> 2) = 330
	newFreqDec := channel.freqLast - (channel.freqLast >> uint(channel.sweepNumber))
	if newFreqDec != 330 {
		t.Errorf("Sweep decrease newFreq = %d; want 330", newFreqDec)
	}
}
