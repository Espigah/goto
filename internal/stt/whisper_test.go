//go:build whisper

package stt

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Paths relative to the package directory (internal/stt).
const (
	modelRel = "../../third_party/whisper.cpp/models/ggml-base.bin"
	wavRel   = "../../third_party/whisper.cpp/samples/jfk.wav"
)

// TestTranscribeJFK proves offline transcription end to end: it loads the
// model, reads a real speech WAV and checks the recognized text contains
// expected words from the JFK speech.
func TestTranscribeJFK(t *testing.T) {
	if _, err := os.Stat(modelRel); err != nil {
		t.Skip("model missing; run the download first")
	}
	tr, err := New(Config{ModelPath: modelRel, Language: "en"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer tr.Close()

	samples, err := readWAV16(wavRel)
	if err != nil {
		t.Fatalf("read wav: %v", err)
	}
	text, err := tr.Transcribe(samples)
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	t.Logf("transcription: %q", text)
	low := strings.ToLower(text)
	if !strings.Contains(low, "country") && !strings.Contains(low, "fellow") {
		t.Errorf("unexpected transcription: %q", text)
	}
}

// readWAV16 reads a mono 16-bit PCM WAV (canonical 44-byte header) into []int16.
func readWAV16(path string) ([]int16, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	if len(b) < 44 {
		return nil, os.ErrInvalid
	}
	data := b[44:] // skip the canonical RIFF header
	n := len(data) / 2
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return out, nil
}
