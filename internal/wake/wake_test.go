package wake

import "testing"

func TestDetectAccents(t *testing.T) {
	d := Default()
	cases := []struct {
		transcript string
		wantCmd    string
		wantOK     bool
	}{
		// wake-word pronunciation variations -> same command
		{"goto vscode myproject", "vscode myproject", true},
		{"gotu slack", "slack", true},
		{"gotwo terminal build", "terminal build", true},
		{"go to vscode backend", "vscode backend", true}, // split into 2 tokens
		{"gotô chrome", "chrome", true},                  // accent
		{"Goto Terminal Logs", "terminal logs", true},    // uppercase
		{"gotoo postman", "postman", true},
		{"Good to postman.", "postman", true},           // real Whisper mistranscription
		{"good to slack", "slack", true},                // same
		{"to the terminal logs", "terminal logs", true}, // same ("to the")
		{"Huchu Browser", "browser", true},              // ASR heard "goto" as "huchu"
		{"Gotchu vscode", "vscode", true},               // or as "gotchu"
		{"to browse stuff", "browse stuff", true},       // "goto" became just "to" (at the start)
		{"I will go to vscode", "vscode", true},         // ASR junk before "go to"
		{"hey go to terminal", "terminal", true},        // wake word a few tokens in
		{"I'm going to the west coast", "", false},      // "to the" mid-sentence does NOT trigger
		// no wake word -> no trigger
		{"open the slack", "", false},
		{"good morning everyone", "", false},
		{"vscode myproject", "", false},
		{"", "", false},
		// wake word alone -> empty command but triggers
		{"goto", "", true},
	}
	for _, c := range cases {
		cmd, ok := d.Detect(c.transcript)
		if ok != c.wantOK {
			t.Errorf("%q: ok=%v, wanted %v", c.transcript, ok, c.wantOK)
			continue
		}
		if ok && cmd != c.wantCmd {
			t.Errorf("%q: cmd=%q, wanted %q", c.transcript, cmd, c.wantCmd)
		}
	}
}

// makes sure it does not become too easy a trigger (dangerous false positives).
// NOTE: "voto" is intentionally NOT here, it's an accepted PT-accent variant of
// "goto" (see GotoVariants), even though it's also a real Portuguese word.
func TestNoFalsePositives(t *testing.T) {
	d := Default()
	for _, bad := range []string{"google", "photo", "moto", "gato"} {
		if _, ok := d.Detect(bad + " slack"); ok {
			t.Errorf("%q triggered wrongly", bad)
		}
	}
}

// accent / ASR variants that MUST keep triggering the wake word (PT and EN
// mishearings of "goto"). Locks the requirement in so it can't silently regress.
func TestAccentVariantsAccepted(t *testing.T) {
	d := Default()
	for _, w := range []string{"goto", "go to", "good to", "voto", "gotu", "gotchu", "huchu", "gocho"} {
		if _, ok := d.Detect(w + " slack"); !ok {
			t.Errorf("%q should be accepted as a wake variant, but was not", w)
		}
	}
}
