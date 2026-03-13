package driver

import (
	"image"
	"log"
	"sync"

	"github.com/HFO4/gbc-in-cloud/util"
)

const (
	screenWidth  = 160
	screenHeight = 144
	scaleRatio   = 4
	scaledWidth  = screenWidth * scaleRatio
	scaledHeight = screenHeight * scaleRatio
)

type StaticImage struct {
	pixelsDirty *[screenWidth][screenHeight][3]uint8
	pixelsClean [screenWidth][screenHeight][3]uint8
	pixelLock   sync.RWMutex

	renderBuf *image.RGBA

	inputStatus *byte
	inputQueue  []*inputCommand
	queueLock   sync.Mutex
}

type inputCommand struct {
	button byte
	ttl    int
	issued bool
}

func (s *StaticImage) InitStatus(b *byte) {
	s.inputStatus = b
}

func (s *StaticImage) UpdateInput() bool {
	s.queueLock.Lock()
	if len(s.inputQueue) == 0 {
		s.queueLock.Unlock()
		return false
	}
	newInput := s.inputQueue[0]
	if newInput.ttl > 0 {
		newInput.ttl--
	} else {
		s.inputQueue = s.inputQueue[1:]
	}
	s.queueLock.Unlock()

	statusCopy := *s.inputStatus
	if !newInput.issued {
		statusCopy = util.ClearBit(statusCopy, uint(newInput.button))
		newInput.issued = true
	} else {
		if newInput.ttl > 0 {
			return false
		} else {
			statusCopy = util.SetBit(statusCopy, uint(newInput.button))
		}
	}

	*s.inputStatus = statusCopy

	return true
}

func (s *StaticImage) NewInput(bytes []byte) {
	panic("implement me")
}

func (s *StaticImage) Init(pixels *[screenWidth][screenHeight][3]uint8, s2 string) {
	s.pixelsDirty = pixels
	s.renderBuf = image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
	log.Println("[Display] Initialize static image display")
}

func (s *StaticImage) Run(drawSignal chan bool, f func()) {
	for {
		<-drawSignal
		s.pixelLock.Lock()
		if s.pixelsDirty != nil {
			s.pixelsClean = *s.pixelsDirty
		}
		s.pixelLock.Unlock()
	}
}

func (s *StaticImage) Render() *image.RGBA {
	s.pixelLock.RLock()
	pix := s.renderBuf.Pix
	stride := s.renderBuf.Stride

	for y := 0; y < screenHeight; y++ {
		for x := 0; x < screenWidth; x++ {
			r, g, b := s.pixelsClean[x][y][0], s.pixelsClean[x][y][1], s.pixelsClean[x][y][2]

			srcY := y * scaleRatio
			srcX := x * scaleRatio

			for dy := 0; dy < scaleRatio; dy++ {
				rowOffset := (srcY + dy) * stride
				for dx := 0; dx < scaleRatio; dx++ {
					i := rowOffset + (srcX+dx)*4
					pix[i] = r
					pix[i+1] = g
					pix[i+2] = b
					pix[i+3] = 0xff
				}
			}
		}
	}
	s.pixelLock.RUnlock()

	return s.renderBuf
}

func (s *StaticImage) EnqueueInput(button byte) {
	s.queueLock.Lock()
	s.inputQueue = append(s.inputQueue, &inputCommand{button, 3, false})
	s.queueLock.Unlock()
}
