//go:build whisper

// Offline implementation with whisper.cpp (CGO). Build with:
//
//	make build-voice    (sets C_INCLUDE_PATH/LIBRARY_PATH and -tags whisper)
package stt

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// Supported indicates this binary has the voice engine built in.
const Supported = true

type whisperEngine struct {
	model   whisper.Model
	lang    string
	prompt  string
	threads uint
}

// chooseThreads uses a good fraction of the cores (CPU-bound), capped so it
// does not fight over memory bandwidth. Override via GOTO_THREADS.
func chooseThreads() uint {
	n := runtime.NumCPU() - 2
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	if v := os.Getenv("GOTO_THREADS"); v != "" {
		if t, err := strconv.Atoi(v); err == nil && t > 0 {
			n = t
		}
	}
	return uint(n)
}

// New loads the model and returns a ready Transcriber.
func New(cfg Config) (Transcriber, error) {
	m, err := whisper.New(cfg.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("load model %q: %w", cfg.ModelPath, err)
	}
	lang := cfg.Language
	if lang == "" {
		lang = "auto"
	}
	return &whisperEngine{model: m, lang: lang, prompt: cfg.InitialPrompt, threads: chooseThreads()}, nil
}

func (w *whisperEngine) Transcribe(samples []int16) (string, error) {
	ctx, err := w.model.NewContext()
	if err != nil {
		return "", fmt.Errorf("new context: %w", err)
	}
	// .en models are not multilingual; SetLanguage handles that internally.
	_ = ctx.SetLanguage(w.lang)
	if w.prompt != "" {
		ctx.SetInitialPrompt(w.prompt) // bias toward "goto" + known apps
	}

	// Tuned for short commands. A small beam beats pure greedy on 2-word
	// utterances. Temperature is pinned at 0 with NO fallback: the fallback
	// re-decodes at higher temperatures when the first pass looks "bad", which
	// on tiny clips produces random, creative hallucinations ("vá para o...")
	// instead of the faithful (if imperfect) tokens the fuzzy wake detector can
	// still match. No carry-over context between commands.
	ctx.SetThreads(w.threads)
	ctx.SetBeamSize(2)
	ctx.SetTemperature(0)
	ctx.SetTemperatureFallback(-1)
	ctx.SetMaxContext(0)

	// audio_ctx proportional to the clip size: Whisper's encoder normally
	// processes as if it were 30s (1500 frames); for a 2s command that's
	// wasteful. ~320 samples per frame; we derive it from the real clip
	// (+margin) to speed up without cutting the end of speech.
	// GOTO_AUDIO_CTX=off disables it; a number forces a fixed value.
	switch v := os.Getenv("GOTO_AUDIO_CTX"); {
	case v == "off":
		// use the model default (1500)
	case v != "":
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ctx.SetAudioCtx(uint(n))
		}
	default:
		frames := len(samples)/320 + 64
		if frames < 1024 { // generous floor: too-low audio_ctx makes the model hallucinate
			frames = 1024
		}
		if frames > 1500 {
			frames = 1500
		}
		ctx.SetAudioCtx(uint(frames))
	}

	data := pcm16ToFloat32(samples)
	if err := ctx.Process(data, nil, nil, nil); err != nil {
		return "", fmt.Errorf("process audio: %w", err)
	}

	var sb strings.Builder
	for {
		seg, err := ctx.NextSegment()
		if err != nil {
			break
		}
		sb.WriteString(seg.Text)
	}
	return strings.TrimSpace(sb.String()), nil
}

// SetLanguage swaps the recognition language for subsequent transcriptions.
// Each Transcribe builds a fresh context and calls ctx.SetLanguage(w.lang), so
// changing w.lang here takes effect immediately with no model reload.
func (w *whisperEngine) SetLanguage(lang string) {
	if lang == "" {
		lang = "auto"
	}
	w.lang = lang
}

func (w *whisperEngine) Close() error { return w.model.Close() }

// pcm16ToFloat32 normalizes s16 samples to [-1,1], which is what Whisper eats.
func pcm16ToFloat32(in []int16) []float32 {
	out := make([]float32, len(in))
	for i, s := range in {
		out[i] = float32(s) / 32768.0
	}
	return out
}
