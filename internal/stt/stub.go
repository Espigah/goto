//go:build !whisper

// Stub used when the binary is compiled WITHOUT voice support (default build,
// CI, or window-focus-only use). Keeps the app compiling without the C lib.
package stt

import "errors"

// Supported indicates this binary does NOT have the voice engine built in.
const Supported = false

// New returns an error: this binary has no voice engine built in.
func New(cfg Config) (Transcriber, error) {
	return nil, errors.New("goto built without Whisper support; rebuild with `make build-voice`")
}
