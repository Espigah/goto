//go:build !noaudio

// Package audio captures PCM from the microphone using miniaudio (malgo).
// It is the equivalent of Handy's `cpal`: it opens the capture device and
// delivers raw audio frames (PCM s16, mono, 16 kHz) via a callback.
//
// We chose malgo because it loads ALSA/PulseAudio/WASAPI/CoreAudio at
// runtime (dlopen), so the binary compiles without -dev headers and runs on
// as many platforms as possible.
package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// SampleRate is fixed at 16 kHz: what Whisper and Porcupine expect.
const SampleRate = 16000

// FrameHandler receives a block of PCM samples (mono, s16) already converted
// to []int16. The same slice must NOT be kept after the call returns.
type FrameHandler func(samples []int16)

// Capture manages a capture device. Start/Stop are idempotent and safe to
// call from the tray menu.
type Capture struct {
	mu         sync.Mutex
	ctx        *malgo.AllocatedContext
	device     *malgo.Device
	onFrame    FrameHandler
	deviceName string // capture source name substring ("" = default)
	running    bool
}

// New creates the audio context using the default microphone.
func New(onFrame FrameHandler) (*Capture, error) {
	return NewWithDevice(onFrame, "")
}

// NewWithDevice creates the context and pins the capture source whose name
// contains deviceName (case-insensitive). Empty = system default device.
func NewWithDevice(onFrame FrameHandler, deviceName string) (*Capture, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(msg string) {
		// miniaudio internal log; silent by default
	})
	if err != nil {
		return nil, fmt.Errorf("init audio context: %w", err)
	}
	return &Capture{ctx: ctx, onFrame: onFrame, deviceName: deviceName}, nil
}

// pickDeviceID finds a capture source whose name contains c.deviceName and
// returns a pointer to its ID (or nil = use the default). The DeviceInfo must
// stay alive until InitDevice; that's why we return it too.
func (c *Capture) pickDeviceID() (unsafe.Pointer, []malgo.DeviceInfo) {
	if c.deviceName == "" {
		return nil, nil
	}
	infos, err := c.ctx.Devices(malgo.Capture)
	if err != nil {
		return nil, nil
	}
	want := strings.ToLower(c.deviceName)
	for i := range infos {
		if strings.Contains(strings.ToLower(infos[i].Name()), want) {
			return unsafe.Pointer(&infos[i].ID), infos
		}
	}
	return nil, infos
}

// Start opens the microphone and starts pushing frames to the handler.
func (c *Capture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return nil
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = SampleRate
	cfg.Alsa.NoMMap = 1

	// pin the capture source if a name was requested (e.g. the echo-cancel
	// source). keepAlive keeps the DeviceInfo alive until InitDevice.
	id, keepAlive := c.pickDeviceID()
	if id != nil {
		cfg.Capture.DeviceID = id
	}

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, input []byte, frameCount uint32) {
			if c.onFrame == nil {
				return
			}
			n := int(frameCount)
			samples := make([]int16, n)
			for i := 0; i < n; i++ {
				samples[i] = int16(binary.LittleEndian.Uint16(input[i*2:]))
			}
			c.onFrame(samples)
		},
	}

	dev, err := malgo.InitDevice(c.ctx.Context, cfg, callbacks)
	runtime.KeepAlive(keepAlive) // ID must stay alive during InitDevice
	if err != nil {
		return fmt.Errorf("open microphone: %w", err)
	}
	if err := dev.Start(); err != nil {
		dev.Uninit()
		return fmt.Errorf("start capture: %w", err)
	}
	c.device = dev
	c.running = true
	return nil
}

// Stop stops capture but keeps the context alive (can Start again).
func (c *Capture) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	if c.device != nil {
		c.device.Uninit()
		c.device = nil
	}
	c.running = false
}

// Running reports whether capture is active.
func (c *Capture) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Close frees everything (call on app shutdown).
func (c *Capture) Close() {
	c.Stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx != nil {
		_ = c.ctx.Uninit()
		c.ctx.Free()
		c.ctx = nil
	}
}

// RMS computes the average level (0..1) of a block of samples. Useful to
// debug whether the microphone is actually picking up signal.
func RMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		f := float64(s) / 32768.0
		sum += f * f
	}
	return math.Sqrt(sum / float64(len(samples)))
}
