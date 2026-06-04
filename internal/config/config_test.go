package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultWhenMissing(t *testing.T) {
	// point XDG_CONFIG_HOME at an empty tmp dir
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ActivationMode != ModeHotkey {
		t.Errorf("default mode = %q, wanted %q", cfg.ActivationMode, ModeHotkey)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	in := Default()
	in.ActivationMode = ModeWakeWord
	in.Language = "pt"
	if err := in.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "goto", "config.json")); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	out, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.ActivationMode != ModeWakeWord || out.Language != "pt" {
		t.Errorf("roundtrip failed: %+v", out)
	}
}
