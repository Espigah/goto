// Package stt (speech-to-text) defines the offline transcription interface.
//
// The real implementation (whisper.cpp via CGO) lives in whisper.go, behind
// the `whisper` build tag, so the default build and CI (window focus, no
// audio) don't need the C lib. Without the tag, stub.go applies.
//
// Mirrors the role of whisper-rs in Handy: PCM in, text out, all local.
package stt

// Transcriber converts PCM audio (mono, s16, 16 kHz) into text, offline.
type Transcriber interface {
	Transcribe(samples []int16) (string, error)
	Close() error
}

// Config for the transcription engine.
type Config struct {
	ModelPath     string // path to the ggml-*.bin model
	Language      string // "pt", "en", "auto" (default "auto")
	InitialPrompt string // biases the vocabulary (e.g. "goto vscode. goto slack.")
}
