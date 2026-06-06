// Package dispatch is the core that links command -> adapter -> window.
//
// It does NOT know any specific program, but it EMBEDS the intelligence to
// deal with ASR noise: built-in mistranscription fixes (builtinSpoken),
// fuzzy app-name matching, and dropping filler words. The user config
// (SetUserAliases) is only an override on top.
package dispatch

import (
	"errors"
	"sort"
	"strings"

	"goto/internal/adapter"
	"goto/internal/textutil"
	"goto/internal/winfocus"
)

// fillers are connector words the ASR leaves at the start of the command
// (e.g. "goto" becomes "go to to vscode"). Dropped from the front. Includes
// a few non-English connectors since the ASR may emit them.
var fillers = map[string]bool{
	"to": true, "the": true, "a": true, "o": true, "as": true, "os": true,
	"no": true, "na": true, "do": true, "da": true, "pro": true, "pra": true,
	"para": true, "ao": true, "in": true, "on": true, "um": true, "uma": true,
	"e": true, "and": true, "que": true, "this": true,
}

// builtinSpoken: app-name mistranscription fixes, BUILT INTO the app
// (factory intelligence). Spoken form (normalized) -> canonical. The user
// only needs to touch this in exotic cases (SetUserAliases).
var builtinSpoken = map[string]string{
	// vscode EN mishearings
	"vest code":          "vscode",
	"vest cold":          "vscode",
	"west coast":         "vscode",
	"west code":          "vscode",
	"vs code":            "vscode",
	"v s code":           "vscode",
	"the s code":         "vscode",
	"this code":          "vscode",
	"visual studio code": "vscode",
	"westcoad":           "vscode",
	"vesco":              "vscode",
	// vscode PT mishearings
	"vis code": "vscode",
	"viscode":  "vscode",
	"biscote":  "vscode",
	"vis codi": "vscode",
	// navegador PT -> browser
	"navegador":   "browser",
	"navegadores": "browser",
	// "web browser" -> browser (Whisper PT às vezes emite a forma longa)
	"web browser":  "browser",
	"web browsers": "browser",
	"webbrowser":   "browser",
	// terminal PT
	"console": "terminal",
}

// userAliases is the end-user override (config), applied on top.
var userAliases map[string]string

// SetUserAliases installs the config aliases (called once at startup).
func SetUserAliases(m map[string]string) { userAliases = m }

// stripFiller drops connector words from the FRONT of the token list.
func stripFiller(toks []string) []string {
	for len(toks) > 0 && fillers[toks[0]] {
		toks = toks[1:]
	}
	return toks
}

// canonicalize applies the phrase fixes (built-in + user) over the already
// normalized command. Longer phrases first (whole words).
func canonicalize(norm string) string {
	merged := make(map[string]string, len(builtinSpoken)+len(userAliases))
	for k, v := range builtinSpoken {
		merged[textutil.Normalize(k)] = textutil.Normalize(v)
	}
	for k, v := range userAliases { // user takes precedence
		merged[textutil.Normalize(k)] = textutil.Normalize(v)
	}
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	padded := " " + norm + " "
	for _, k := range keys {
		padded = strings.ReplaceAll(padded, " "+k+" ", " "+merged[k]+" ")
	}
	return strings.TrimSpace(padded)
}

// lookupApp finds the adapter for a token, tolerating ASR near-misses (fuzzy
// edit distance). Exact first; then the closest alias.
func lookupApp(token string) (adapter.Adapter, bool) {
	if a, ok := adapter.Lookup(token); ok {
		return a, true
	}
	best := -1
	var bestAd adapter.Adapter
	for name, ad := range adapter.All() {
		if len(name) < 4 { // don't fuzzy short aliases (vs, o) to avoid false hits
			continue
		}
		thresh := 1
		if len(name) >= 7 {
			thresh = 2
		}
		if d := textutil.Levenshtein(token, name); d <= thresh && (best < 0 || d < best) {
			best, bestAd = d, ad
		}
	}
	if best >= 0 {
		return bestAd, true
	}
	return nil, false
}

// Resolve interprets the command and returns the target window. If
// handled=true, the adapter already performed the action (e.g. opened a
// file) and there is nothing to activate.
func Resolve(query string, be winfocus.Backend) (win winfocus.Window, handled bool, err error) {
	norm := canonicalize(textutil.Normalize(query))
	toks := stripFiller(strings.Fields(norm))
	if len(toks) == 0 {
		return winfocus.Window{}, false, errors.New("empty command")
	}

	wins, err := be.List()
	if err != nil {
		return winfocus.Window{}, false, err
	}

	// Is the 1st token an app (known or fuzzy)? Use its adapter and the rest
	// becomes the target. Otherwise, the generic adapter over everything.
	ad := adapter.Generic()
	target := toks
	if a, ok := lookupApp(toks[0]); ok {
		ad = a
		target = toks[1:]
	}

	var appWins []winfocus.Window
	for _, w := range wins {
		if ad.Match(w) {
			appWins = append(appWins, w)
		}
	}

	return ad.Resolve(target, appWins, be)
}
