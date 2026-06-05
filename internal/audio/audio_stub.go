//go:build noaudio

package audio

import (
	"fmt"
)

const SampleRate = 16000

type FrameHandler func(samples []int16)

type Capture struct {
	running bool
}

func New(onFrame FrameHandler) (*Capture, error) {
	return &Capture{}, nil
}

func NewWithDevice(onFrame FrameHandler, deviceName string) (*Capture, error) {
	return &Capture{}, nil
}

func (c *Capture) Start() error {
	return fmt.Errorf("audio disabled in this build")
}

func (c *Capture) Stop() {}

func (c *Capture) Running() bool {
	return false
}

func (c *Capture) Close() {}

func RMS(samples []int16) float64 {
	return 0
}
