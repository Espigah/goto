// Package model ensures the Whisper model is on disk, downloading it on first
// use. Keeps the distribution packages light (the .bin does not go inside the
// .deb/.rpm/AppImage/snap).
package model

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// baseURL is the official whisper.cpp ggml model repository.
const baseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/"

// Exists reports whether the model is already on disk.
func Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// Ensure downloads the model if missing. The file name (e.g. ggml-base.bin)
// determines what is downloaded from the official repository.
func Ensure(path string, logf func(string)) error {
	if Exists(path) {
		return nil
	}
	if logf == nil {
		logf = func(string) {}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	name := filepath.Base(path)
	url := baseURL + name
	logf(fmt.Sprintf("downloading model %s (one-time)...", name))

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download model: HTTP %d", resp.StatusCode)
	}

	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write model: %w", err)
	}
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	logf("model ready: " + path)
	return nil
}
