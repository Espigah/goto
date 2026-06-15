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
	"strings"
)

// baseURL is the official whisper.cpp ggml model repository.
const baseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/"

// approxSize returns a human-friendly download size for the known model files,
// so the one-time download message is accurate per tier (base/small/medium).
func approxSize(name string) string {
	quant := strings.Contains(name, "-q5") || strings.Contains(name, "-q8")
	switch {
	case strings.Contains(name, "medium"):
		if quant {
			return "~514 MB"
		}
		return "~1.5 GB"
	case strings.Contains(name, "small"):
		if quant {
			return "~181 MB"
		}
		return "~466 MB"
	case strings.Contains(name, "base"):
		if quant {
			return "~57 MB"
		}
		return "~147 MB"
	case strings.Contains(name, "tiny"):
		return "~75 MB"
	default:
		return "a few hundred MB"
	}
}

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
	logf(fmt.Sprintf("preparing voice model (%s), one-time download...", approxSize(name)))

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
	// progressReader surfaces download progress to logf (which the tray shows
	// in its status line and tooltip), so first run isn't a silent freeze.
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, logf: logf, lastPct: -1}
	if _, err := io.Copy(f, pr); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write model: %w", err)
	}
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	logf("voice model ready")
	return nil
}

// progressReader wraps the download body and reports progress through logf,
// throttled so it stays readable (every ~3%, or every ~32 MB if the server
// didn't send a Content-Length).
type progressReader struct {
	r       io.Reader
	total   int64 // -1 if unknown
	done    int64
	lastPct int
	lastMB  int64
	logf    func(string)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	const mb = 1 << 20
	switch {
	case p.total > 0:
		pct := int(p.done * 100 / p.total)
		if pct >= p.lastPct+3 || (pct == 100 && p.lastPct != 100) {
			p.lastPct = pct
			p.logf(fmt.Sprintf("downloading voice model %d%% (%d/%d MB), one-time", pct, p.done/mb, p.total/mb))
		}
	default: // unknown total: report MB downloaded
		if doneMB := p.done / mb; doneMB >= p.lastMB+32 {
			p.lastMB = doneMB
			p.logf(fmt.Sprintf("downloading voice model %d MB, one-time", doneMB))
		}
	}
	return n, err
}
