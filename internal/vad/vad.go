// Package vad does simple voice activity detection (by energy/RMS).
//
// It is the lightweight equivalent of Handy's Silero: it avoids running
// Whisper on silence. The Segmenter receives audio frames and emits an
// "utterance" (a complete speech segment) when it detects the person stopped
// speaking.
//
// Energy/RMS is zero-dependency and good enough for push-to-talk and a
// reasonably quiet environment. It can be swapped for Silero (ONNX) later
// without changing the Segmenter's consumers.
package vad

import "goto/internal/audio"

// Config for the segmenter. Durations in milliseconds.
type Config struct {
	SampleRate  int
	Threshold   float64 // RMS above this = speech (0..1). e.g. 0.02
	HangMS      int     // continuous silence to close the segment. e.g. 600
	MinSpeechMS int     // drop segments that are too short (noise). e.g. 250
	MaxSpeechMS int     // cut segments that are too long. e.g. 8000
}

// DefaultConfig returns reasonable values for command speech.
func DefaultConfig(sampleRate int) Config {
	return Config{
		SampleRate:  sampleRate,
		Threshold:   0.02,
		HangMS:      600,
		MinSpeechMS: 250,
		MaxSpeechMS: 8000,
	}
}

// Segmenter accumulates frames and emits complete utterances.
type Segmenter struct {
	cfg       Config
	inSpeech  bool
	buf       []int16
	speechN   int // samples that were SPEECH (does not count hang silence)
	silentRun int // consecutive samples below the threshold
	hangN     int // samples of silence to close
	minN      int
	maxN      int
}

// New creates a segmenter.
func New(cfg Config) *Segmenter {
	ms := func(d int) int { return cfg.SampleRate * d / 1000 }
	return &Segmenter{
		cfg:   cfg,
		hangN: ms(cfg.HangMS),
		minN:  ms(cfg.MinSpeechMS),
		maxN:  ms(cfg.MaxSpeechMS),
	}
}

// Push feeds a frame. When an utterance ends, it returns its samples and
// done=true. Otherwise, done=false.
func (s *Segmenter) Push(frame []int16) (utterance []int16, done bool) {
	speech := audio.RMS(frame) >= s.cfg.Threshold

	if speech {
		s.inSpeech = true
		s.silentRun = 0
		s.speechN += len(frame)
		s.buf = append(s.buf, frame...)
	} else if s.inSpeech {
		// silence during speech: still buffer (so we don't cut words),
		// but count the silence to eventually close.
		s.buf = append(s.buf, frame...)
		s.silentRun += len(frame)
		if s.silentRun >= s.hangN {
			return s.flush()
		}
	}

	// cut segments that are too long (person didn't stop speaking)
	if s.inSpeech && len(s.buf) >= s.maxN {
		return s.flush()
	}
	return nil, false
}

// flush closes the current segment. Discards it if too short (noise).
func (s *Segmenter) flush() ([]int16, bool) {
	out := s.buf
	speech := s.speechN
	s.buf = nil
	s.inSpeech = false
	s.silentRun = 0
	s.speechN = 0
	if speech < s.minN {
		return nil, false // speech too short, probably noise
	}
	return out, true
}

// InSpeech reports whether the segmenter is currently capturing speech.
func (s *Segmenter) InSpeech() bool { return s.inSpeech }

// Reset clears the state (e.g. when turning listening on/off).
func (s *Segmenter) Reset() {
	s.buf = nil
	s.inSpeech = false
	s.silentRun = 0
	s.speechN = 0
}
