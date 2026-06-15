package textutil

import "testing"

// MatchToken must tolerate the same ASR/accent near-misses on the TARGET (the
// user's own words) that the app name already tolerates — while not matching
// short, ambiguous tokens that would pick the wrong window.
func TestMatchToken(t *testing.T) {
	cases := []struct {
		text  string // already-normalized title/class
		token string
		want  bool
	}{
		// exact substring (incl. partials)
		{"github pull requests", "github", true},
		{"github pull requests", "git", true}, // partial substring
		// fuzzy: ASR/accent near-miss of the user's word
		{"github pull requests", "guithub", true}, // 1 edit
		{"john doe slack", "jon", false},          // 3-char token: too short to fuzz
		{"john doe slack", "jonn", true},          // jonn ~ john: 1 edit, len4
		{"john doe slack", "joao", false},         // joao vs john: distance 2, len4 thresh 1 -> no
		{"backend service", "backend", true},      // exact
		{"frontend service", "backend", false},    // must NOT cross-match siblings
		// short tokens never fuzz (avoid wrong-window matches)
		{"logs terminal", "log", true}, // substring ok
		{"build terminal", "logs", false},
	}
	for _, c := range cases {
		if got := MatchToken(c.text, c.token); got != c.want {
			t.Errorf("MatchToken(%q,%q)=%v, want %v", c.text, c.token, got, c.want)
		}
	}
}

// siblings that differ by a couple of letters must not be confused: the fuzzy
// fallback is length-scaled and conservative.
func TestScoreTitleNoSiblingConfusion(t *testing.T) {
	front := ScoreTitle("FRONTEND", []string{"backend"})
	if front != 0 {
		t.Errorf("backend wrongly matched FRONTEND (score=%d)", front)
	}
	if s := ScoreTitle("BACKEND", []string{"backend"}); s != 1 {
		t.Errorf("backend should match BACKEND (score=%d)", s)
	}
}
