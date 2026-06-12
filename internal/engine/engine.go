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
	// persisted "Calibrate mic" result (0 = keep the built-in default).
	if cfg.VadThreshold > 0 {
		vc.Threshold = cfg.VadThreshold
	}
	// quick override to calibrate without recompiling: GOTO_VAD_THRESHOLD=0.01
	if v := os.Getenv("GOTO_VAD_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			vc.Threshold = f
		}
	}
	// user app aliases (override on top of the factory intelligence)
	dispatch.SetUserAliases(cfg.Aliases)

	return &Engine{
		cfg:          cfg,
		be:           be,
		wd:           buildDetector(cfg),
		seg:          vad.New(vc),
		vadThreshold: vc.Threshold,
		log:          logf,
	}
}

// VadThreshold returns the active RMS speech-detection threshold.
func (e *Engine) VadThreshold() float64 { return e.seg.Threshold() }

// SetVadThreshold applies a new RMS speech-detection threshold live (no
// restart needed) and updates the in-memory config so callers can persist it.
func (e *Engine) SetVadThreshold(t float64) {
	e.seg.SetThreshold(t)
	e.mu.Lock()
	e.vadThreshold = t
	e.cfg.VadThreshold = t
	e.mu.Unlock()
}

// buildDetector assembles the wake detector: built-in variants, the
// language-specific forms (how Whisper writes "goto" in that language), and
// any the user adds in config — no recompile needed.
func buildDetector(cfg config.Config) *wake.Detector {
	variants := append([]string{}, wake.GotoVariants...)
	variants = append(variants, wake.GotoLangVariants[cfg.Language]...)
	variants = append(variants, cfg.WakeVariants...)
	return wake.New(cfgWakeWord(cfg), 1, variants...)
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
	tr, err := e.loadTranscriber(e.cfg.ModelPath)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.tr = tr
	e.mu.Unlock()
	return nil
}

// resolveLang picks the recognition language (env override > config > pt).
func (e *Engine) resolveLang() string {
	lang := e.cfg.Language
	if v := os.Getenv("GOTO_LANG"); v != "" {
		lang = v // quick override to test: GOTO_LANG=en ./goto listen
	}
	if lang == "" {
		lang = "pt" // matches config.Default(); better for PT-BR users
	}
	return lang
}

// loadTranscriber ensures the model file exists (downloading if needed) and
// builds a transcriber for it. Shared by EnableVoice and SetModel.
func (e *Engine) loadTranscriber(modelPath string) (stt.Transcriber, error) {
	if err := model.Ensure(modelPath, e.log); err != nil {
		return nil, err
	}
	lang := e.resolveLang()
	e.log("recognition language: " + lang)
	return stt.New(stt.Config{
		ModelPath:     modelPath,
		Language:      lang,
		InitialPrompt: biasPrompt(e.cfg.WakeWord),
	})
}

// ModelPath returns the path of the model currently configured.
func (e *Engine) ModelPath() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.ModelPath
}

// SetModel switches the Whisper model at runtime: ensures the file exists
// (downloading if needed — can take a while for the larger models, with
// progress logged), loads it, and swaps the live transcriber, closing the old
// one. Safe to call while listening; transcription is briefly serialized
// during the swap. Blocks until done, so callers should run it off the UI loop.
func (e *Engine) SetModel(modelPath string) error {
	if !stt.Supported {
		return fmt.Errorf("no voice support in this build")
	}
	tr, err := e.loadTranscriber(modelPath)
	if err != nil {
		return err
	}
	// procMu so we don't swap mid-transcription; mu for the fields.
	e.procMu.Lock()
	e.mu.Lock()
	old := e.tr
	e.tr = tr
	e.cfg.ModelPath = modelPath
	e.mu.Unlock()
	e.procMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
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

// Language returns the current recognition language ("pt", "en", "auto").
func (e *Engine) Language() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.Language
}

// SetLanguage changes the recognition language at runtime. It takes effect on
// the next transcription (no model reload, no need to restart listening).
func (e *Engine) SetLanguage(lang string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.Language = lang
	e.wd = buildDetector(e.cfg) // refresh the language-specific wake variants
	if e.tr != nil {
		e.tr.SetLanguage(lang)
	}
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
//
// We must NOT hold e.mu while calling cap.Close(): Close -> device.Uninit()
// blocks until the in-flight audio callback returns, and that callback
// (onFrame) starts by taking e.mu. Holding the lock here would deadlock the
// two against each other — leaving running=true, the mic open, and the tray
// icon stuck on "listening". So we flip the state under the lock, release it,
// and only then tear the device down.
func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	cap := e.cap
	e.cap = nil
	e.running = false
	e.mu.Unlock()

	if cap != nil {
		cap.Close() // any in-flight onFrame can now take e.mu and finish
	}
	// We may have stopped mid-speech (segmenter left InSpeech), so the
	// "processing" icon would never be cleared on its own — force it back.
	e.setProcessing(false)
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
	wd := e.wd
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
		c, ok := wd.Detect(text)
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
