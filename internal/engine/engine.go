// Package engine stitches the voice pipeline together: audio -> VAD ->
// Whisper -> wake word -> dispatch -> window focus.
//
// It is where the two activation modes converge:
//   - wakeword: VAD segments speech; the transcript must start with the wake
//     word ("goto ...") to become a command.
//   - hotkey  : while the key is held, it buffers; on release it transcribes
//     everything and treats it as a command (no wake word required).
package engine

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"goto/internal/audio"
	"goto/internal/config"
	"goto/internal/dispatch"
	"goto/internal/model"
	"goto/internal/stt"
	"goto/internal/vad"
	"goto/internal/wake"
	"goto/internal/winfocus"
)

// Engine orchestrates the pipeline.
type Engine struct {
	cfg config.Config
	be  winfocus.Backend
	wd  *wake.Detector
	seg *vad.Segmenter
	log func(string)

	mu           sync.Mutex
	tr           stt.Transcriber
	cap          *audio.Capture
	running      bool
	onProcessing func(bool) // notifies start/end of processing (e.g. swap the icon)

	// push-to-talk
	pttActive bool
	pttBuf    []int16

	vadThreshold float64

	procMu sync.Mutex // serializes transcription+dispatch (whisper/xgb are not concurrent)
}

// New creates the engine (without opening the mic or loading the model yet).
func New(cfg config.Config, be winfocus.Backend, logf func(string)) *Engine {
	if logf == nil {
		logf = func(string) {}
	}
	vc := vad.DefaultConfig(audio.SampleRate)
	// quick override to calibrate without recompiling: GOTO_VAD_THRESHOLD=0.01
	if v := os.Getenv("GOTO_VAD_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			vc.Threshold = f
		}
	}
	// user app aliases (override on top of the factory intelligence)
	dispatch.SetUserAliases(cfg.Aliases)

	// built-in variants + the ones the user adds in config (no recompile)
	variants := append(append([]string{}, wake.GotoVariants...), cfg.WakeVariants...)
	return &Engine{
		cfg:          cfg,
		be:           be,
		wd:           wake.New(cfgWakeWord(cfg), 1, variants...),
		seg:          vad.New(vc),
		vadThreshold: vc.Threshold,
		log:          logf,
	}
}

func cfgWakeWord(c config.Config) string {
	if c.WakeWord != "" {
		return c.WakeWord
	}
	return "goto"
}

// biasPrompt builds a SHORT "initial prompt" for Whisper, biasing the
// transcription toward the wake word. Short on purpose: a long prompt
// degrades/slows decoding. Reduces the chance the ASR hears "good to" / "Gochua".
// Bilingual (PT+EN) so it works regardless of GOTO_LANG.
func biasPrompt(wakeWord string) string {
	if wakeWord == "" {
		wakeWord = "goto"
	}
	w := wakeWord
	// PT apps the user is likely to say + EN equivalents
	return w + " vscode. " + w + " navegador. " + w + " terminal. " + w + " chrome. " +
		w + " slack. " + w + " explorador. " + w + " editor."
}

// VoiceSupported reports whether this binary was compiled with the voice engine.
func (e *Engine) VoiceSupported() bool { return stt.Supported }

// EnableVoice downloads the model (if missing) and loads the transcriber.
func (e *Engine) EnableVoice() error {
	if !stt.Supported {
		return fmt.Errorf("no voice support in this build; rebuild with `make build-voice`")
	}
	if err := model.Ensure(e.cfg.ModelPath, e.log); err != nil {
		return err
	}
	lang := e.cfg.Language
	if v := os.Getenv("GOTO_LANG"); v != "" {
		lang = v // quick override to test: GOTO_LANG=en ./goto listen
	}
	if lang == "" {
		lang = "en"
	}
	e.log("recognition language: " + lang)
	tr, err := stt.New(stt.Config{
		ModelPath:     e.cfg.ModelPath,
		Language:      lang,
		InitialPrompt: biasPrompt(e.cfg.WakeWord),
	})
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.tr = tr
	e.mu.Unlock()
	return nil
}

// SetOnProcessing registers a callback called with true when a command starts
// being processed (transcription/dispatch) and false when it ends. Useful for
// visual feedback (swapping the tray icon).
func (e *Engine) SetOnProcessing(f func(bool)) {
	e.mu.Lock()
	e.onProcessing = f
	e.mu.Unlock()
}

// SetMode switches the activation mode at runtime.
func (e *Engine) SetMode(mode string) {
	e.mu.Lock()
	e.cfg.ActivationMode = mode
	e.seg.Reset()
	e.mu.Unlock()
}

// Mode returns the current mode.
func (e *Engine) Mode() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.ActivationMode
}

// Start opens the microphone and begins listening.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return nil
	}
	if e.tr == nil {
		return fmt.Errorf("voice not enabled (call EnableVoice first)")
	}
	cap, err := audio.NewWithDevice(e.onFrame, e.cfg.InputDevice)
	if err != nil {
		return err
	}
	if err := cap.Start(); err != nil {
		return err
	}
	e.cap = cap
	e.seg.Reset()
	e.running = true
	e.log(fmt.Sprintf("listening ON (mode=%s, vad_threshold=%.4f)", e.cfg.ActivationMode, e.vadThreshold))
	return nil
}

// Stop closes the microphone.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	if e.cap != nil {
		e.cap.Close()
		e.cap = nil
	}
	e.running = false
}

// Running reports whether listening is active.
func (e *Engine) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// onFrame runs on the audio goroutine (must not block).
func (e *Engine) onFrame(samples []int16) {
	e.mu.Lock()
	mode := e.cfg.ActivationMode
	if mode == config.ModeHotkey {
		if e.pttActive {
			e.pttBuf = append(e.pttBuf, samples...)
		}
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()

	// wakeword: swap the icon as soon as speech STARTS (immediate feedback),
	// and process when it ends.
	was := e.seg.InSpeech()
	utt, done := e.seg.Push(samples)
	now := e.seg.InSpeech()
	if !was && now {
		e.setProcessing(true) // started speaking -> "processing" icon
	}
	if done {
		go e.processUtterance(utt, true)
	} else if was && !now {
		e.setProcessing(false) // short speech discarded -> back to normal
	}
}

// PTTStart/PTTStop: turn push-to-talk on/off (called by the hotkey).
func (e *Engine) PTTStart() {
	e.mu.Lock()
	e.pttActive = true
	e.pttBuf = nil
	e.mu.Unlock()
	e.setProcessing(true) // key pressed -> "processing" icon
}

func (e *Engine) PTTStop() {
	e.mu.Lock()
	buf := e.pttBuf
	e.pttActive = false
	e.pttBuf = nil
	e.mu.Unlock()
	if len(buf) > 0 {
		go e.processUtterance(buf, false)
	} else {
		e.setProcessing(false) // nothing recorded -> back to normal
	}
}

// processUtterance transcribes and dispatches. requireWake=true requires the
// wake word.
func (e *Engine) processUtterance(samples []int16, requireWake bool) {
	e.procMu.Lock()
	defer e.procMu.Unlock()

	e.mu.Lock()
	tr := e.tr
	e.mu.Unlock()
	if tr == nil {
		return
	}

	// visual feedback: as soon as speech is detected, swap the icon to
	// "processing" and back to normal at the end (with a minimum visible time).
	e.setProcessing(true)
	procStart := time.Now()
	defer func() {
		if d := time.Since(procStart); d < 500*time.Millisecond {
			time.Sleep(500*time.Millisecond - d)
		}
		e.setProcessing(false)
	}()

	dur := float64(len(samples)) / float64(audio.SampleRate)
	e.log(fmt.Sprintf("speech detected (%.1fs), transcribing...", dur))

	text, err := tr.Transcribe(samples)
	if err != nil {
		e.log("transcription error: " + err.Error())
		return
	}
	text = strings.TrimSpace(text)
	e.log(fmt.Sprintf("heard: %q", text))
	if text == "" {
		return
	}

	cmd := text
	if requireWake {
		c, ok := e.wd.Detect(text)
		if !ok {
			e.log(`wake word not recognized (expected to start with "goto")`)
			return
		}
		cmd = c
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		e.log("wake word recognized, but no command")
		return
	}
	e.runCommand(cmd) // dispatch canonicalizes/fuzzies the app name
}

func (e *Engine) setProcessing(p bool) {
	e.mu.Lock()
	f := e.onProcessing
	e.mu.Unlock()
	if f != nil {
		f(p)
	}
}

func (e *Engine) runCommand(cmd string) {
	e.log("command: " + cmd)
	win, handled, err := dispatch.Resolve(cmd, e.be)
	if err != nil {
		e.log("dispatch: " + err.Error())
		return
	}
	if handled {
		return
	}
	if err := e.be.Activate(win); err != nil {
		e.log("activate: " + err.Error())
		return
	}
	e.log("focus -> " + win.Title)
}
