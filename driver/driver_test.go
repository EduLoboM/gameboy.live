package driver

import (
	"testing"
)

func TestStaticImage_RenderAndDimensions(t *testing.T) {
	var pixels [160][144][3]uint8
	// Set top-left pixel to Red
	pixels[0][0] = [3]uint8{255, 0, 0}

	driver := &StaticImage{}
	driver.Init(&pixels, "Test")

	// Set clean buffer
	driver.pixelsClean = pixels

	img := driver.Render()
	bounds := img.Bounds()

	// Should be scaled 4x: 160*4 = 640, 144*4 = 576
	if bounds.Dx() != 640 || bounds.Dy() != 576 {
		t.Errorf("Render image dimensions = (%d, %d); want (640, 576)", bounds.Dx(), bounds.Dy())
	}

	// Verify top-left pixel (0,0) is Red (RGBA: 255, 0, 0, 255)
	r, g, b, a := img.At(0, 0).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("Pixel (0,0) RGBA = (%d, %d, %d, %d); want (255, 0, 0, 255)", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestStaticImage_InputQueueAndTTL(t *testing.T) {
	var status byte = 0xFF
	driver := &StaticImage{}
	driver.InitStatus(&status)

	// Enqueue button 4 (A button)
	driver.EnqueueInput(4)

	// First update -> button pressed (bit 4 cleared to 0)
	if !driver.UpdateInput() {
		t.Errorf("UpdateInput should return true when processing new input")
	}
	if status&(1<<4) != 0 {
		t.Errorf("Status bit 4 should be 0 (pressed); got 0x%02X", status)
	}

	// Decrement TTL (initial TTL is 3)
	driver.UpdateInput()
	driver.UpdateInput()

	// When TTL expires, button is released (bit 4 restored to 1)
	driver.UpdateInput()
	if status&(1<<4) == 0 {
		t.Errorf("Status bit 4 should be released back to 1; got 0x%02X", status)
	}
}

func TestStaticImage_QueueCapacity(t *testing.T) {
	driver := &StaticImage{}
	// Enqueue 60 inputs (limit is 50)
	for i := 0; i < 60; i++ {
		driver.EnqueueInput(byte(i % 8))
	}

	if len(driver.inputQueue) > 50 {
		t.Errorf("inputQueue length = %d; want <= 50", len(driver.inputQueue))
	}
}
