// Package textutil normalizes spoken text and scores title matches.
// Shared between dispatch and adapters so there is a single definition of
// "how to compare what the user said with what is on screen".
package textutil

import "strings"

// foldDiacritics removes common accents (after ToLower). Zero-dependency;
// covers what an ASR tends to emit in pt/es. Turns "gotô" -> "goto".
var foldDiacritics = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// Normalize lowercases, removes accents and turns ANY non-letter/digit
// character into a space (quotes, period, comma, _-., emojis, spinner
// braille, etc.). So 'Go to "Vest Code".' becomes "go to vest code".
func Normalize(s string) string {
	s = strings.ToLower(s)
	s = foldDiacritics.Replace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// Levenshtein is the edit distance between two strings (insertions,
// deletions, substitutions). Used by the wake word to tolerate accents.
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// Tokens normalizes and splits into words.
func Tokens(s string) []string {
	return strings.Fields(Normalize(s))
}

// MatchToken reports whether a spoken target token matches some text (a window
// title or class), tolerating ASR/accent near-misses. This is what makes the
// TARGET — the user's own words (a project, a person, a browser tab) — robust
// to the same mistranscription the app name already tolerates.
//
// Order matters for speed and precision:
//  1. exact substring (fast; also catches partials like "git" in "github");
//  2. otherwise, a per-word edit-distance fallback ("guithub" ~ "github",
//     "jon" ~ "john"). Only tokens of 4+ chars are fuzzed, so short, ambiguous
//     words don't match the wrong window.
//
// text must already be Normalized.
func MatchToken(text, token string) bool {
	if token == "" || text == "" {
		return false
	}
	if strings.Contains(text, token) {
		return true
	}
	if len([]rune(token)) < 4 {
		return false // too short to fuzz safely
	}
	// length-scaled tolerance, same policy as dispatch's app fuzzy-lookup.
	thresh := 1
	if len([]rune(token)) >= 7 {
		thresh = 2
	}
	for _, w := range strings.Fields(text) {
		if len([]rune(w)) >= 4 && Levenshtein(w, token) <= thresh {
			return true
		}
	}
	return false
}

// ScoreTitle counts how many target tokens match the title (substring or fuzzy).
func ScoreTitle(title string, target []string) int {
	t := Normalize(title)
	if t == "" {
		return 0
	}
	n := 0
	for _, tok := range target {
		if MatchToken(t, tok) {
			n++
		}
	}
	return n
}
