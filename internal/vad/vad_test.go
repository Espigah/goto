package vad

import (
	"math"
	"testing"
)

const sr = 16000

// frame builds a frame of `n` samples: silence (amp=0) or a tone (high amp).
func frame(n int, amp float64) []int16 {
	out := make([]int16, n)
	for i := range out {
		// sine tone for a stable RMS
		out[i] = int16(amp * 32767 * math.Sin(float64(i)*0.2))
	}
	return out
}

func TestSegmenterEmitsSpeech(t *testing.T) {
	s := New(DefaultConfig(sr))
	const fn = 320 // 20ms @16k

	var got []int16
	// 200ms silence
	for i := 0; i < 10; i++ {
		if u, done := s.Push(frame(fn, 0)); done {
			t.Fatal("should not close during the initial silence")
			_ = u
		}
	}
	// 1s of speech (high amp)
	for i := 0; i < 50; i++ {
		if u, done := s.Push(frame(fn, 0.5)); done {
			got = u
		}
	}
	// 700ms of silence -> should close the segment (hang 600ms)
	for i := 0; i < 35; i++ {
		if u, done := s.Push(frame(fn, 0)); done {
			got = u
		}
	}
	if got == nil {
		t.Fatal("expected an emitted utterance")
	}
	// the segment should contain ~1s of speech (50*320 = 16000 samples) + some hang
	if len(got) < 40*fn {
		t.Errorf("utterance too short: %d samples", len(got))
	}
}

func TestSegmenterDropsShortNoise(t *testing.T) {
	s := New(DefaultConfig(sr))
	const fn = 320
	// 100ms of "speech" (< MinSpeechMS=250) followed by silence
	for i := 0; i < 5; i++ {
		s.Push(frame(fn, 0.5))
	}
	var emitted bool
	for i := 0; i < 40; i++ {
		if _, done := s.Push(frame(fn, 0)); done {
			emitted = true
		}
	}
	if emitted {
		t.Error("short noise should not become an utterance")
	}
}
