// Package wake detects the activation word at the start of a transcript and
// returns the remaining command.
//
// Accent tolerance is a requirement: since the transcript comes from ASR
// (Whisper), "goto", "gotu", "gotwo", "go to", "gotô" must all count. We do
// that with (1) accent folding + normalization, (2) a curated set of variants
// and (3) edit distance (Levenshtein) with a tolerance.
package wake

import (
	"strings"

	"goto/internal/textutil"
)

// Detector recognizes the wake word and splits off the command.
type Detector struct {
	canonical string          // canonical form, e.g. "goto"
	prefix    string          // required prefix, e.g. "go" (anti-false-positive)
	maxDist   int             // edit-distance tolerance
	variants  map[string]bool // explicit variants (shortcut)
}

// New creates a detector. variants are extra forms accepted outright (besides
// the distance match). maxDist is the edit-distance tolerance vs the canonical.
//
// To avoid false positives on common words (moto, voto, gato are at distance
// 1 from "goto"), we require the candidate to start with the canonical's
// first 2 characters ("go"), unless it is in the variants list.
func New(canonical string, maxDist int, variants ...string) *Detector {
	can := textutil.Normalize(canonical)
	vs := map[string]bool{can: true}
	for _, v := range variants {
		vs[textutil.Normalize(v)] = true
	}
	prefix := can
	if r := []rune(can); len(r) >= 2 {
		prefix = string(r[:2])
	}
	return &Detector{
		canonical: can,
		prefix:    prefix,
		maxDist:   maxDist,
		variants:  vs,
	}
}

// GotoVariants are forms the ASR (Whisper) tends to emit when the person says
// "goto". Includes 2-word cases already joined (Detect tries joining the
// first 2 tokens), e.g. "good to" -> "goodto", "got to" -> "gotto". Those at
// edit distance 1 from "goto" wouldn't strictly need to be here, but the
// distance-2 ones (goodto, gotta) only pass if they are variants.
var GotoVariants = []string{
	"gotu", "gotwo", "gotoo", "gouto", "gotto", "gotta",
	"goodto", "goodtoo", "goodtwo", "gotit", "tothe", "togo",
	"huchu", "hutchu", "huchoo", "hoochoo", // ASR sometimes hears "goto" as "huchu"
	"gotchu", "gotcha", "gotchoo", // or as "gotchu"
	"to", // "goto" becomes just "to" (WEAK: only valid at the start, see weakVariants)
}

// Default is the detector for the "goto" wake word, accent-tolerant.
func Default() *Detector {
	return New("goto", 1, GotoVariants...)
}

// weakVariants are ambiguous forms that appear in normal speech ("to the",
// "to go"). They only count when the wake word is at the START of the
// transcript; mid-sentence they would be false positives.
var weakVariants = map[string]bool{"tothe": true, "togo": true, "to": true}

// scanLimit: how far we look for the wake word (tolerates junk the ASR puts
// in front, e.g. "I will go to vscode"). Beyond that, it is normal speech.
const scanLimit = 5

// Detect looks for the wake word in the first tokens of the transcript. If
// found, it returns the command (everything after it) and ok=true. The wake
// word may come as 1 token ("goto") or 2 ("go to"), possibly preceded by ASR
// junk.
func (d *Detector) Detect(transcript string) (command string, ok bool) {
	toks := textutil.Tokens(transcript)
	limit := len(toks)
	if limit > scanLimit {
		limit = scanLimit
	}
	for i := 0; i < limit; i++ {
		strict := i > 0 // away from the start, only strong forms (no ambiguous ones)
		if i+1 < len(toks) && d.isWake(toks[i]+toks[i+1], strict) {
			return strings.Join(toks[i+2:], " "), true
		}
		if d.isWake(toks[i], strict) {
			return strings.Join(toks[i+1:], " "), true
		}
	}
	return "", false
}

// isWake decides whether a candidate is the wake word. strict=true excludes
// the ambiguous forms (weakVariants), used when the wake word is not at index 0.
func (d *Detector) isWake(cand string, strict bool) bool {
	c := textutil.Normalize(cand)
	if c == "" {
		return false
	}
	if d.variants[c] {
		return !(strict && weakVariants[c])
	}
	if !strings.HasPrefix(c, d.prefix) {
		return false // anti-false-positive: must start with "go"
	}
	return textutil.Levenshtein(c, d.canonical) <= d.maxDist
}
