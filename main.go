// goto: voice control for windows, offline-first.
//
// Two ways to use it:
//   - CLI one-shot: `goto vscode myproject` focuses the window and exits
//     (no voice; great for testing and for binding to an OS shortcut).
//   - Tray app: with no arguments, shows the tray icon with a listening
//     toggle and the activation modes (wake word "goto" / hotkey).
package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"fyne.io/systray"

	"goto/internal/audio"
	"goto/internal/autostart"
	"goto/internal/config"
	"goto/internal/dispatch"
	"goto/internal/engine"
	"goto/internal/hotkey"
	"goto/internal/mcpserver"
	"goto/internal/winfocus"
)

//go:embed packaging/icons/goto.png
var iconNormal []byte

// icon shown while a command is being processed (speech detected).
//
//go:embed packaging/icons/goto-icon-listen.png
var iconProcessing []byte

// version of goto, printed at the start of the log when the app opens.
const version = "0.3.11"

// startPaused: show the tray without turning listening on (used by the login
// autostart, so the mic does not go live by itself). Set by `--paused`.
var startPaused bool

func banner() { log.Printf("goto v%s", version) }

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
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
			banner()
			systray.Run(onReady, onExit)
			return
		default:
			os.Exit(runCommand(strings.Join(os.Args[1:], " ")))
		}
	}
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
	sug := maxR * 0.5
	fmt.Printf("\nPeak = %.4f. Suggested threshold ~= %.4f\n", maxR, sug)
	fmt.Printf("Try:  GOTO_VAD_THRESHOLD=%.4f ./goto listen\n", sug)
	return 0
}

// runListen shows the tray (with icon) already listening in wake-word mode and
// logs everything to the terminal. It is the shortcut to use/debug without
// clicking "Start listening" in the menu.
func runListen() int {
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

	mStatus := systray.AddMenuItem("starting...", "current status")
	mStatus.Disable()
	mToggle := systray.AddMenuItem("Start listening", "turn voice capture on/off")
	systray.AddSeparator()
	mWake := systray.AddMenuItemCheckbox(`Mode: wake word "goto"`, "hands-free, say goto ...", false)
	mHotkey := systray.AddMenuItemCheckbox("Mode: hotkey (push-to-talk)", "hold the key and speak", false)
	mAutostart := systray.AddMenuItemCheckbox("Start at login", "launch goto in the tray on login", autostart.Enabled())
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "quit goto")

	cfg, _ := config.Load()
	be, err := winfocus.New()
	if err != nil {
		log.Fatalf("winfocus: %v", err)
	}
	status := func(s string) { log.Println("[goto]", s); mStatus.SetTitle(s) }
	eng := engine.New(cfg, be, status)

	// swap the tray icon while processing (speech detected -> "listen";
	// done -> normal). SetIcon runs on another goroutine, which is safe.
	eng.SetOnProcessing(func(processing bool) {
		if processing {
			systray.SetIcon(iconProcessing)
		} else {
			systray.SetIcon(iconNormal)
		}
	})

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
			case <-mAutostart.ClickedCh:
				toggleAutostart(mAutostart, status)
			case <-mQuit.ClickedCh:
				stopListen()
				systray.Quit()
				return
			}
		}
	}()
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
