package util

import (
	"testing"
)

func TestSetBit(t *testing.T) {
	for pos := uint(0); pos < 8; pos++ {
		res := SetBit(0, pos)
		expected := byte(1 << pos)
		if res != expected {
			t.Errorf("SetBit(0, %d) = %08b; want %08b", pos, res, expected)
		}
	}

	// Setting an already set bit should not change it
	if SetBit(0xFF, 3) != 0xFF {
		t.Errorf("SetBit(0xFF, 3) = %08b; want 0xFF", SetBit(0xFF, 3))
	}
}

func TestClearBit(t *testing.T) {
	for pos := uint(0); pos < 8; pos++ {
		res := ClearBit(0xFF, pos)
		expected := byte(0xFF ^ (1 << pos))
		if res != expected {
			t.Errorf("ClearBit(0xFF, %d) = %08b; want %08b", pos, res, expected)
		}
	}

	// Clearing an already clear bit should not change it
	if ClearBit(0, 4) != 0 {
		t.Errorf("ClearBit(0, 4) = %08b; want 0", ClearBit(0, 4))
	}
}

func TestTestBit(t *testing.T) {
	val := byte(0b10100101)
	expected := []bool{true, false, true, false, false, true, false, true}
	for pos := uint(0); pos < 8; pos++ {
		if TestBit(val, pos) != expected[pos] {
			t.Errorf("TestBit(%08b, %d) = %v; want %v", val, pos, TestBit(val, pos), expected[pos])
		}
	}
}

func TestGetVal(t *testing.T) {
	val := byte(0b11001010)
	expected := []byte{0, 1, 0, 1, 0, 0, 1, 1}
	for pos := uint(0); pos < 8; pos++ {
		if GetVal(val, pos) != expected[pos] {
			t.Errorf("GetVal(%08b, %d) = %d; want %d", val, pos, GetVal(val, pos), expected[pos])
		}
	}
}
