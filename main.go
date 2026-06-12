// goto: voice control for windows, offline-first.
//
// Two ways to use it:
//   - CLI one-shot: `goto vscode myproject` focuses the window and exits
//     (no voice; great for testing and for binding to an OS shortcut).
//   - Tray app: with no arguments, shows the tray icon with a listening
//     toggle and the activation modes (wake word "goto" / hotkey).
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"
	"github.com/gen2brain/beeep"

	"goto/internal/audio"
	"goto/internal/autostart"
	"goto/internal/config"
	"goto/internal/dispatch"
	"goto/internal/engine"
	"goto/internal/hotkey"
	"goto/internal/mcpserver"
	"goto/internal/winfocus"
)

// iconNormal / iconProcessing are embedded per-OS (icons_windows.go uses .ico,
// icons_other.go uses .png), because the Windows tray needs ICO format.

// version of goto, printed at the start of the log when the app opens.
const version = "0.3.28"

// startPaused: show the tray without turning listening on (used by the login
// autostart, so the mic does not go live by itself). Set by `--paused`.
var startPaused bool

// logFilePath is the file the tray's "Show logs" item opens (empty if file
// logging could not be set up). Populated by setupFileLog.
var logFilePath string

// maxLogLines caps how much the on-disk log keeps: only the most recent lines
// (~the last few dozen commands). It is a rolling buffer, so the file stays a
// few KB and never grows without bound.
const maxLogLines = 50

// ringWriter keeps the last maxLogLines log lines and rewrites the log file on
// each write. Writes are infrequent (one per status change), so rewriting a
// tiny file every time is cheap and keeps disk usage trivial.
type ringWriter struct {
	mu    sync.Mutex
	lines []string
	path  string
}

func (w *ringWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, ln := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		w.lines = append(w.lines, ln)
	}
	if len(w.lines) > maxLogLines {
		w.lines = w.lines[len(w.lines)-maxLogLines:]
	}
	_ = os.WriteFile(w.path, []byte(strings.Join(w.lines, "\n")+"\n"), 0o644)
	return len(p), nil
}

// setupFileLog mirrors the standard logger to a small rolling file under the
// cache dir, so the tray's "Show logs" item can show what happened even when
// goto was launched from the login autostart (no terminal attached).
// Best-effort: on any error it leaves logging on stderr only.
func setupFileLog() {
	path := config.LogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	logFilePath = path
	log.SetOutput(io.MultiWriter(os.Stderr, &ringWriter{path: path}))
}

// openPath opens a file/path with the OS's default handler (file manager or
// text editor). Best-effort; the caller surfaces any error on the status line.
func openPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func banner() { log.Printf("goto v%s", version) }

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help", "-h", "--help":
			fmt.Printf("goto v%s - Voice window control\n\n", version)
			fmt.Println("Usage:")
			fmt.Println("  goto                      Show tray icon (GUI mode)")
			fmt.Println("  goto <app> [target]       Focus window by app name and target (CLI mode)")
			fmt.Println("  goto listen               Start in tray and listen immediately")
			fmt.Println("  goto calibrate            Measure mic for 8s & auto-save the voice threshold")
			fmt.Println("  goto mcp                  Run as MCP server (for AI agents)")
			fmt.Println("  goto version              Show version")
			fmt.Println("\nFlags:")
			fmt.Println("  --paused                  Start tray without turning on the mic")
			return
		case "version", "-v", "--version":
			fmt.Printf("goto v%s\n", version)
			return
		case "calibrate":
			os.Exit(runCalibrate())
		case "listen":
			banner()
			os.Exit(runListen())
		case "mcp":
			// MCP server (stdio): goto's "hands" for Claude Code.
			// stdout is the JSON-RPC channel, so do NOT print the banner here.
			if err := mcpserver.Run(version); err != nil {
				log.Fatalf("mcp: %v", err)
			}
			return
		case "--paused":
			startPaused = true // show the tray without turning listening on (autostart)
			setupFileLog()
			banner()
			systray.Run(onReady, onExit)
			return
		default:
			os.Exit(runCommand(strings.Join(os.Args[1:], " ")))
		}
	}
	setupFileLog()
	banner()
	systray.Run(onReady, onExit)
}

// runCalibrate measures the mic level for 8s to help pick the vad_threshold.
// No voice needed (works even in a build without whisper).
func runCalibrate() int {
	var maxR float64
	cap, err := audio.New(func(s []int16) {
		if r := audio.RMS(s); r > maxR {
			maxR = r
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "audio:", err)
		return 1
	}
	if err := cap.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "audio:", err)
		return 1
	}
	defer cap.Close()
	fmt.Println("Speak normally (as you would a command) for the next 8s...")
	for i := 1; i <= 8; i++ {
		time.Sleep(time.Second)
		fmt.Printf("  t=%ds   peak RMS so far = %.4f\n", i, maxR)
	}
	if maxR <= 0 {
		fmt.Fprintln(os.Stderr, "no mic signal detected — check your input device")
		return 1
	}
	sug := maxR * 0.5
	cfg, _ := config.Load()
	cfg.VadThreshold = sug
	if err := cfg.Save(); err != nil {
		fmt.Printf("\nPeak = %.4f. Suggested threshold ~= %.4f (could not save: %v)\n", maxR, sug, err)
		fmt.Printf("Apply manually:  GOTO_VAD_THRESHOLD=%.4f goto listen\n", sug)
		return 0
	}
	fmt.Printf("\nPeak = %.4f. Saved vad_threshold = %.4f to %s\n", maxR, sug, config.Path())
	fmt.Println("It will be used automatically. Override anytime with GOTO_VAD_THRESHOLD.")
	return 0
}

// runListen shows the tray (with icon) already listening in wake-word mode and
// logs everything to the terminal. It is the shortcut to use/debug without
// clicking "Start listening" in the menu.
func runListen() int {
	setupFileLog()
	cfg, _ := config.Load()
	cfg.ActivationMode = config.ModeWakeWord
	be, err := winfocus.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "winfocus:", err)
		return 1
	}
	eng := engine.New(cfg, be, func(s string) { log.Println("[goto]", s) })
	if !eng.VoiceSupported() {
		fmt.Fprintln(os.Stderr, "build without voice; compile with `make build-voice`")
		return 1
	}

	onReady := func() {
		systray.SetIcon(iconNormal)
		systray.SetTitle("goto")
		systray.SetTooltip("goto (listening)")
		mVersion := systray.AddMenuItem("goto v"+version, "installed version")
		mVersion.Disable()
		mLogs := systray.AddMenuItem("Show logs", "open the goto log (recent activity)")
		mQuit := systray.AddMenuItem("Quit", "quit goto")

		eng.SetOnProcessing(func(processing bool) {
			if processing {
				systray.SetIcon(iconProcessing)
			} else {
				systray.SetIcon(iconNormal)
			}
		})

		go func() {
			log.Println("[goto] preparing voice (may download the model)...")
			if err := eng.EnableVoice(); err != nil {
				log.Println("[goto] voice:", err)
				return
			}
			if err := eng.Start(); err != nil {
				log.Println("[goto] start:", err)
				return
			}
			log.Println(`[goto] listening. Say: "goto vscode myproject".`)
		}()

		go func() {
			for range mLogs.ClickedCh {
				showLogs(func(s string) { log.Println("[goto]", s) })
			}
		}()

		go func() {
			<-mQuit.ClickedCh
			eng.Stop()
			systray.Quit()
		}()
	}
	systray.Run(onReady, func() { eng.Stop() })
	return 0
}

// runCommand resolves a command and focuses the window (CLI mode, no voice).
func runCommand(query string) int {
	query = strings.TrimSpace(query)
	if query == "" {
		fmt.Fprintln(os.Stderr, "usage: goto <app> [target]   e.g.: goto vscode myproject")
		return 2
	}
	be, err := winfocus.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "winfocus:", err)
		return 1
	}
	win, handled, err := dispatch.Resolve(query, be)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if handled {
		fmt.Println("command executed (adapter custom action)")
		return 0
	}
	if err := be.Activate(win); err != nil {
		fmt.Fprintln(os.Stderr, "activate:", err)
		return 1
	}
	fmt.Printf("focus -> %s (%s)\n", win.Title, win.Class)
	return 0
}

func onExit() { log.Println("goto stopped") }

func onReady() {
	systray.SetIcon(iconNormal)
	systray.SetTitle("goto")
	systray.SetTooltip("Voice window control")

	mVersion := systray.AddMenuItem("goto v"+version, "installed version")
	mVersion.Disable()
	mToggle := systray.AddMenuItem("Start listening", "turn voice capture on/off")
	systray.AddSeparator()
	mWake := systray.AddMenuItemCheckbox(`Mode: wake word "goto"`, "hands-free, say goto ...", false)
	mHotkey := systray.AddMenuItemCheckbox("Mode: hotkey (push-to-talk)", "hold the key and speak", false)
	systray.AddSeparator()
	mLang := systray.AddMenuItem("Language", "recognition language")
	mLangPT := mLang.AddSubMenuItemCheckbox("Português (BR)", "reconhecer em português do Brasil", false)
	mLangEN := mLang.AddSubMenuItemCheckbox("English", "recognize in English", false)
	mLangAuto := mLang.AddSubMenuItemCheckbox("Automatic", "auto-detect the language", false)
	mPrec := systray.AddMenuItem("Precision", "recognition accuracy vs speed/size")
	mPrecNormal := mPrec.AddSubMenuItemCheckbox("Normal", "lighter model, faster", false)
	mPrecHigh := mPrec.AddSubMenuItemCheckbox("High (downloads ~1.5GB)", "more accurate, slower; downloads the medium model on first use", false)
	mAutostart := systray.AddMenuItemCheckbox("Start at login", "launch goto in the tray on login", autostart.Enabled())
	systray.AddSeparator()
	mCalibrate := systray.AddMenuItem("Calibrate mic", "measure mic level for 8s & auto-tune voice detection")
	mThreshold := systray.AddMenuItem("Mic sensitivity: …", "current voice-detection threshold (lower = more sensitive); set by Calibrate mic")
	mThreshold.Disable()
	mLogs := systray.AddMenuItem("Show logs", "open the goto log (recent activity)")
	mQuit := systray.AddMenuItem("Quit", "quit goto")

	cfg, _ := config.Load()
	be, err := winfocus.New()
	if err != nil {
		log.Fatalf("winfocus: %v", err)
	}
	// status logs and updates the tray tooltip (shown on hover); we keep the
	// menu itself clean (no status line cluttering it).
	status := func(s string) { log.Println("[goto]", s); systray.SetTooltip(s) }
	eng := engine.New(cfg, be, status)

	// showThreshold reflects the active voice-detection sensitivity in the menu,
	// so the user can see the value (and watch it change after calibrating).
	showThreshold := func() {
		mThreshold.SetTitle(fmt.Sprintf("Mic sensitivity: %.4f", eng.VadThreshold()))
	}
	showThreshold()

	// downloading is set while a model download/switch is in progress so the
	// tray shows the "busy" icon (and a ⬇ title) the whole time, not just
	// during speech. applyIcon centralizes the icon so the download indicator
	// and the speech-processing indicator never fight each other.
	var iconMu sync.Mutex
	downloading := false
	applyIcon := func(processing bool) {
		iconMu.Lock()
		busy := downloading
		iconMu.Unlock()
		if processing || busy {
			systray.SetIcon(iconProcessing)
		} else {
			systray.SetIcon(iconNormal)
		}
	}
	setDownloading := func(on bool) {
		iconMu.Lock()
		downloading = on
		iconMu.Unlock()
		if on {
			systray.SetTitle("goto ⬇")
		} else {
			systray.SetTitle("goto")
		}
		applyIcon(false)
	}

	// swap the tray icon while processing (speech detected -> "listen";
	// done -> normal). SetIcon runs on another goroutine, which is safe.
	eng.SetOnProcessing(func(processing bool) { applyIcon(processing) })

	reflectMode := func() {
		if eng.Mode() == config.ModeWakeWord {
			mWake.Check()
			mHotkey.Uncheck()
		} else {
			mHotkey.Check()
			mWake.Uncheck()
		}
	}
	reflectMode()

	// the three language items act as a radio group reflecting eng.Language().
	reflectLang := func() {
		mLangPT.Uncheck()
		mLangEN.Uncheck()
		mLangAuto.Uncheck()
		switch eng.Language() {
		case "pt":
			mLangPT.Check()
		case "en":
			mLangEN.Check()
		default:
			mLangAuto.Check()
		}
	}
	reflectLang()
	setLang := func(lang string) {
		eng.SetLanguage(lang)
		cfg.Language = lang
		_ = cfg.Save()
		reflectLang()
		status("recognition language: " + lang)
	}

	// the two precision items act as a radio group reflecting the model tier.
	reflectPrecision := func() {
		if config.PrecisionOf(eng.ModelPath()) == config.PrecisionHigh {
			mPrecHigh.Check()
			mPrecNormal.Uncheck()
		} else {
			mPrecNormal.Check()
			mPrecHigh.Uncheck()
		}
	}
	reflectPrecision()
	var precMu sync.Mutex
	switchingModel := false
	setPrecision := func(tier string) {
		precMu.Lock()
		if switchingModel {
			precMu.Unlock()
			status("precision: a model switch is already in progress...")
			return
		}
		switchingModel = true
		precMu.Unlock()
		path := config.ModelPathFor(tier)
		if tier == config.PrecisionHigh {
			status("switching to High precision (downloading ~1.5GB on first use)...")
		} else {
			status("switching to Normal precision...")
		}
		// SetModel may download a large file; keep the UI responsive and show
		// the busy icon (⬇) for the whole download.
		setDownloading(true)
		go func() {
			defer func() {
				setDownloading(false)
				precMu.Lock()
				switchingModel = false
				precMu.Unlock()
			}()
			if err := eng.SetModel(path); err != nil {
				status("precision: " + err.Error())
				reflectPrecision()
				return
			}
			cfg.ModelPath = path
			_ = cfg.Save()
			reflectPrecision()
			status("precision: " + tier + " ready")
		}()
	}

	var hk *hotkey.Hotkey
	stopListen := func() {
		if hk != nil {
			hk.Close()
			hk = nil
		}
		eng.Stop()
	}
	startListen := func() error {
		if err := eng.Start(); err != nil {
			return err
		}
		if eng.Mode() == config.ModeHotkey {
			h, err := hotkey.New(cfg.Hotkey, eng.PTTStart, eng.PTTStop)
			if err != nil {
				eng.Stop()
				return err
			}
			if err := h.Start(); err != nil {
				eng.Stop()
				return err
			}
			hk = h
		}
		return nil
	}
	// startUI turns listening on and updates the menu/status (the icon swaps
	// via the engine).
	startUI := func() {
		if err := startListen(); err != nil {
			status("error: " + err.Error())
			mToggle.SetTitle("Start listening")
			return
		}
		mToggle.SetTitle("Stop listening")
		if eng.Mode() == config.ModeHotkey {
			status("hotkey mode: hold " + cfg.Hotkey + " and speak")
		} else {
			status(`listening: say "goto ..."`)
		}
	}
	switchMode := func(mode string) {
		if eng.Running() {
			stopListen()
		}
		eng.SetMode(mode)
		cfg.ActivationMode = mode
		_ = cfg.Save()
		reflectMode()
		startUI() // switching mode also starts listening in the chosen mode
	}

	// calibrate measures the mic peak RMS for 8s and reports it (plus a
	// suggested vad_threshold) via the status/log, so the user can tell whether
	// the mic is too quiet without a terminal. Pauses listening for the run (the
	// mic is exclusive) and resumes after. Runs in its own goroutine so it does
	// not block the menu event loop.
	var calMu sync.Mutex
	calibrating := false
	calibrate := func() {
		calMu.Lock()
		if calibrating {
			calMu.Unlock()
			return
		}
		calibrating = true
		calMu.Unlock()
		defer func() { calMu.Lock(); calibrating = false; calMu.Unlock() }()

		wasRunning := eng.Running()
		if wasRunning {
			stopListen()
			mToggle.SetTitle("Start listening")
		}
		var peakMu sync.Mutex
		var peak float64
		cap, err := audio.NewWithDevice(func(s []int16) {
			r := audio.RMS(s)
			peakMu.Lock()
			if r > peak {
				peak = r
			}
			peakMu.Unlock()
		}, cfg.InputDevice)
		if err != nil {
			status("calibrate: " + err.Error())
			if wasRunning {
				startUI()
			}
			return
		}
		if err := cap.Start(); err != nil {
			status("calibrate: " + err.Error())
			cap.Close()
			if wasRunning {
				startUI()
			}
			return
		}
		status("calibrating: speak normally for 8s...")
		// surface the countdown on the tray itself (not just the hover tooltip),
		// so the user knows it is recording without opening the menu.
		systray.SetIcon(iconProcessing)
		for i := 1; i <= 8; i++ {
			systray.SetTitle(fmt.Sprintf("goto 🎤%ds", 9-i))
			time.Sleep(time.Second)
			peakMu.Lock()
			p := peak
			peakMu.Unlock()
			status(fmt.Sprintf("calibrating %ds/8  peak RMS=%.4f", i, p))
		}
		systray.SetTitle("goto")
		applyIcon(false)
		cap.Close()
		peakMu.Lock()
		p := peak
		peakMu.Unlock()

		if p <= 0 {
			status("calibrate: no mic signal — check your input device")
			notify("goto — calibration failed", "No mic signal detected. Check your input device.")
			if wasRunning {
				startUI()
			}
			return
		}

		// Auto-tune: half the measured peak is a robust speech threshold. Apply
		// it live, persist it, and reflect the new value in the menu so the user
		// sees the change without reading the log or touching env vars.
		sug := p * 0.5
		eng.SetVadThreshold(sug)
		cfg.VadThreshold = sug
		if err := cfg.Save(); err != nil {
			status("calibrate: applied but could not save: " + err.Error())
		}
		showThreshold()
		status(fmt.Sprintf("mic calibrated ✓ (peak RMS=%.4f, sensitivity set to %.4f)", p, sug))
		notify("goto — mic calibrated ✓", fmt.Sprintf("Voice sensitivity set to %.4f (peak %.4f). Saved.", sug, p))
		if wasRunning {
			startUI()
		}
	}

	// Enable voice in the background (downloads the model on first use) and,
	// once it is ready, start listening on its own (no need to click "Start").
	autoStart := make(chan struct{}, 1)
	go func() {
		if !eng.VoiceSupported() {
			status("voice unavailable (build without whisper)")
			return
		}
		status("preparing voice (may download the model)...")
		if err := eng.EnableVoice(); err != nil {
			status("voice: " + err.Error())
			return
		}
		if startPaused {
			status(`ready (idle), click "Start listening"`)
			return
		}
		autoStart <- struct{}{}
	}()

	go func() {
		for {
			select {
			case <-autoStart:
				startUI()
			case <-mToggle.ClickedCh:
				if eng.Running() {
					stopListen()
					mToggle.SetTitle("Start listening")
					status("idle")
				} else {
					startUI()
				}
			case <-mWake.ClickedCh:
				switchMode(config.ModeWakeWord)
			case <-mHotkey.ClickedCh:
				switchMode(config.ModeHotkey)
			case <-mLangPT.ClickedCh:
				setLang("pt")
			case <-mLangEN.ClickedCh:
				setLang("en")
			case <-mLangAuto.ClickedCh:
				setLang("auto")
			case <-mPrecNormal.ClickedCh:
				setPrecision(config.PrecisionNormal)
			case <-mPrecHigh.ClickedCh:
				setPrecision(config.PrecisionHigh)
			case <-mCalibrate.ClickedCh:
				go calibrate()
			case <-mAutostart.ClickedCh:
				toggleAutostart(mAutostart, status)
			case <-mLogs.ClickedCh:
				showLogs(status)
			case <-mQuit.ClickedCh:
				stopListen()
				systray.Quit()
				return
			}
		}
	}()
}

// notify shows a desktop notification (best-effort). It is how the tray app
// surfaces results — like the calibration outcome — to users who launched goto
// from the login autostart and have no terminal or open log to look at.
func notify(title, message string) {
	if err := beeep.Notify(title, message, ""); err != nil {
		log.Printf("[goto] notify: %v", err)
	}
}

// showLogs opens the rolling log file in the OS default app, so the user can
// see recent activity without a terminal.
func showLogs(status func(string)) {
	if logFilePath == "" {
		status("logs: file logging is off")
		return
	}
	if err := openPath(logFilePath); err != nil {
		status("logs: " + err.Error())
	}
}

// toggleAutostart flips the "start at login" XDG autostart entry.
func toggleAutostart(item *systray.MenuItem, status func(string)) {
	if autostart.Enabled() {
		if err := autostart.Disable(); err != nil {
			status("autostart: " + err.Error())
			return
		}
		item.Uncheck()
		status("autostart disabled")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		status("autostart: " + err.Error())
		return
	}
	if err := autostart.Enable(exe); err != nil {
		status("autostart: " + err.Error())
		return
	}
	item.Check()
	status("autostart enabled")
}
