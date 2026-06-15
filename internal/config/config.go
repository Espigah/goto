// Package config loads/saves goto's configuration.
//
// JSON format at ~/.config/goto/config.json (zero-dependency). Stores the
// activation mode, the Whisper model, language, hotkey and the optional
// Porcupine key (Plan B). Offline-first: everything works without the key.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Activation modes.
const (
	ModeHotkey   = "hotkey"   // push-to-talk (default, offline, no account)
	ModeWakeWord = "wakeword" // wake word "goto" offline (VAD + Whisper)
)

// Config is the persisted state.
type Config struct {
	ActivationMode string   `json:"activation_mode"` // "hotkey" | "wakeword"
	ModelPath      string   `json:"model_path"`      // path to ggml-*.bin
	Language       string   `json:"language"`        // "pt" | "en" | "auto"
	Hotkey         string   `json:"hotkey"`          // e.g. "ctrl+alt+space"
	WakeWord       string   `json:"wake_word"`       // e.g. "goto"
	WakeVariants   []string `json:"wake_variants"`   // extra ASR mishearings (e.g. "good to")
	InputDevice    string   `json:"input_device"`    // capture source name substring ("" = default)
	// Aliases rewrites parts of the command before dispatch, to fix
	// app-name mistranscription (e.g. "vest code" -> "vscode").
	Aliases      map[string]string `json:"aliases"`
	PicovoiceKey string            `json:"picovoice_key"` // optional (Plan B)
	// VadThreshold is the RMS speech-detection threshold set by "Calibrate mic".
	// 0 = use the built-in default. GOTO_VAD_THRESHOLD overrides it at runtime.
	VadThreshold float64 `json:"vad_threshold"`
	// MaxCommandMS caps how long a single spoken command can be before it is
	// cut and transcribed (the tray "Command length"). 0 = built-in default (3s).
	MaxCommandMS int `json:"max_command_ms"`
}

// Default returns the default config (offline, push-to-talk).
func Default() Config {
	return Config{
		ActivationMode: ModeHotkey,
		ModelPath:      DefaultModelPath(),
		Language:       "pt", // português: melhor acurácia para usuários PT-BR
		Hotkey:         "ctrl+alt+space",
		WakeWord:       "goto",
	}
}

// Dir is the configuration directory.
func Dir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		// Fallback for systems without UserConfigDir
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "goto")
	}
	return filepath.Join(base, "goto")
}

// Path is the config file.
func Path() string { return filepath.Join(Dir(), "config.json") }

// ModelsDir is where Whisper models are stored.
func ModelsDir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "goto", "models")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "goto", "models")
}

// DefaultModelPath is the model downloaded on first use. It matches the
// "Normal" precision tier (small) so a fresh install and the tray agree on the
// same model — the base model was less accurate and only the explicit "Low"
// tier opts into it now.
func DefaultModelPath() string {
	return ModelPathFor(PrecisionNormal)
}

// Precision tiers exposed in the tray, smallest to largest. We use QUANTIZED
// (q5) models: on CPU they are bound by memory bandwidth, so a q5 model is
// ~1.5–2x faster than the full-precision one with virtually the same accuracy —
// the single biggest win for how long the tray sits on "processing".
//   - "low"    -> base   q5 (~57MB):  fastest/lightest, lowest accuracy
//   - "normal" -> small  q5 (~181MB): the default, good accuracy
//   - "high"   -> medium q5 (~514MB): most accurate (≈ full medium, ~2x faster)
const (
	PrecisionLow    = "low"
	PrecisionNormal = "normal"
	PrecisionHigh   = "high"
)

// ModelPathFor maps a precision tier to its model file (downloaded on first
// use). Unknown tiers fall back to the Normal model.
func ModelPathFor(tier string) string {
	name := "ggml-small-q5_1.bin"
	switch tier {
	case PrecisionLow:
		name = "ggml-base-q5_1.bin"
	case PrecisionHigh:
		name = "ggml-medium-q5_0.bin"
	}
	return filepath.Join(ModelsDir(), name)
}

// PrecisionOf reports the tier a model path belongs to, by filename.
func PrecisionOf(modelPath string) string {
	base := filepath.Base(modelPath)
	switch {
	case strings.Contains(base, "medium"):
		return PrecisionHigh
	case strings.Contains(base, "base"), strings.Contains(base, "tiny"):
		return PrecisionLow
	default:
		return PrecisionNormal
	}
}

// LogDir is where the tray app mirrors its log output.
func LogDir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "goto")
	}
	base, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cache", "goto")
	}
	return filepath.Join(base, "goto")
}

// LogPath is the file the tray mirrors its log to, so the "Show logs" menu
// item has something to open even when goto was launched from the login
// autostart (no terminal attached).
func LogPath() string { return filepath.Join(LogDir(), "goto.log") }

// Load reads the config; if it does not exist, returns the default (no error).
func Load() (Config, error) {
	b, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}
	cfg := Default()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Default(), err
	}
	return cfg, nil
}

// Save writes the config (creates the directory if needed).
func (c Config) Save() error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), b, 0o644)
}
