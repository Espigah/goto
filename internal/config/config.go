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
}

// Default returns the default config (offline, push-to-talk).
func Default() Config {
	return Config{
		ActivationMode: ModeHotkey,
		ModelPath:      DefaultModelPath(),
		Language:       "en", // fixed > auto: auto-detect fails badly on short clips
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

// DefaultModelPath is the ggml-base downloaded on first use.
func DefaultModelPath() string {
	return filepath.Join(ModelsDir(), "ggml-base.bin")
}

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
