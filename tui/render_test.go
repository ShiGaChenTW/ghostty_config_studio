package main

// Rendering and parsing checks for the TUI half, the companion to
// scripts/test-writes.sh on the shell side.
//
// Every case here is a bug that actually shipped, or the property whose loss
// caused one. The recurring theme in DESIGN_NOTES is that layout bugs are
// invisible until a real terminal at a real size draws them, so most of this
// file renders View() and counts what came out rather than testing helpers in
// isolation.

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// sandbox points the model's writable paths at a temp dir. Nothing here writes,
// but the recent-items and language files are read from there, and a test whose
// result depends on the developer's own config is not a test.
func sandbox(t *testing.T) {
	t.Helper()
	t.Setenv("GHOSTTY_DIR", t.TempDir())
	t.Setenv("GHOSTTY_STUDIO_DIR", t.TempDir())
	lang = langZH
}

// render draws the browser at a terminal size, the way the program would.
//
// The extra Update after mutate matters: in the running program a status is
// always assigned *inside* an Update, so the recompute at the tail of that same
// call sees it. A test that mutated the model and rendered straight away would
// be testing a state the event loop never produces.
type noopMsg struct{}

func render(t *testing.T, w, h int, mutate func(*model)) string {
	t.Helper()
	mm, _ := newModel("..").Update(tea.WindowSizeMsg{Width: w, Height: h})
	m := mm.(model)
	if mutate != nil {
		mutate(&m)
		mm, _ = m.Update(noopMsg{})
		m = mm.(model)
	}
	return m.View()
}

// fits is the whole point of the height budget: the rendered card must not be
// taller than the terminal, or its bottom rows are simply gone. DESIGN_NOTES
// records two separate bugs of this shape (an unbudgeted footer row, and a
// status line that wrapped).
func fits(t *testing.T, label string, out string, w, h int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) > h {
		t.Errorf("%s at %dx%d: rendered %d lines, %d over budget", label, w, h, len(lines), len(lines)-h)
	}
	for i, l := range lines {
		if got := lipgloss.Width(l); got > w {
			t.Errorf("%s at %dx%d: line %d is %d cols, %d over", label, w, h, i, got, got-w)
			break
		}
	}
}

func TestBrowserFitsEveryTerminalSize(t *testing.T) {
	sandbox(t)
	sizes := [][2]int{{80, 24}, {80, 25}, {80, 40}, {100, 24}, {120, 30}, {200, 50}}
	for _, en := range []bool{false, true} {
		for _, s := range sizes {
			w, h := s[0], s[1]
			lang = langZH
			if en {
				lang = langEN
			}
			fits(t, "browser", render(t, w, h, nil), w, h)
		}
	}
}

// The status row is allowed to push the footer down one line. It is not
// allowed to wrap, which would cost two — the override warning plus its two
// hints runs to 111 columns against an 80-column minimum.
func TestLongStatusNeverOverflows(t *testing.T) {
	sandbox(t)
	warn := "⚠  These settings will not take effect: shell-integration-features"
	for _, en := range []bool{false, true} {
		lang = langZH
		if en {
			lang = langEN
		}
		msg := warn + "  ·  " + txtConflictHint() + "  ·  " + txtRestartHint()
		fits(t, "browser+status", render(t, 80, 24, func(m *model) {
			m.status, m.statusOK = msg, false
		}), 80, 24)
	}
}

// Each overlay is centred by lipgloss.Place, which does not scroll: content
// past the budget runs off both ends of the screen rather than being clipped.
// The conflicts panel is the one with unbounded input — it lists whatever the
// other config happens to contain.
func TestOverlaysFitAtTheMinimumSize(t *testing.T) {
	sandbox(t)
	long := strings.Repeat("very-long-", 12)
	for _, en := range []bool{false, true} {
		lang = langZH
		if en {
			lang = langEN
		}
		for _, n := range []int{0, 1, 5, 12, 60} {
			cs := make([]shadowConflict, n)
			for i := range cs {
				cs[i] = shadowConflict{
					file: "/Users/x/Library/Application Support/com.mitchellh.ghostty/" + long,
					line: "1234", key: "background-opacity", text: "background-opacity = " + long,
				}
			}
			fits(t, "conflictsPanel", render(t, 80, 24, func(m *model) {
				m.showConflicts, m.conflicts = true, cs
			}), 80, 24)
			if n > 0 {
				fits(t, "conflictConfirm", render(t, 80, 24, func(m *model) {
					m.showConflictConfirm, m.conflicts = true, cs
				}), 80, 24)
			}
		}
		fits(t, "tagPanel", render(t, 80, 24, func(m *model) { m.showTagPanel = true }), 80, 24)
	}
}

// Hostile records, all of which a config file can legitimately produce. The
// contract is that a malformed record costs one row, never the screen.
func TestParseShadowRecordsSurvivesHostileInput(t *testing.T) {
	const us = "\x1f"
	cases := []struct {
		name, in string
		want     int
	}{
		{"well formed", "/a/config" + us + "12" + us + "font-size" + us + "font-size = 12", 1},
		{"too few fields", "/a/config" + us + "12" + us + "font-size", 0},
		{"extra separator folds into the text", "/a/config" + us + "1" + us + "k" + us + "k = a" + us + "b", 1},
		{"empty key dropped", "/a/config" + us + "12" + us + us + "x = 1", 0},
		{"empty file dropped", us + "12" + us + "font-size" + us + "x = 1", 0},
		{"non-numeric line kept", "/a/config" + us + "not-a-number" + us + "k" + us + "k = v", 1},
		{"blank input", "", 0},
		{"only separators", us + us + us, 0},
		{"cjk path", "/Users/使用者/設定/config" + us + "1" + us + "font-size" + us + "font-size = 12", 1},
	}
	for _, c := range cases {
		if got := len(parseShadowRecords(c.in)); got != c.want {
			t.Errorf("%s: got %d records, want %d", c.name, got, c.want)
		}
	}
}

// A config line is quoted back to the user verbatim, and a config value may
// contain escape sequences: one holding ESC[2J cleared the screen, and an OSC
// sequence retitled the terminal, from a panel that is only quoting a file.
func TestControlCharactersNeverReachTheTerminal(t *testing.T) {
	lang = langEN
	evil := "title = \x1b[2J\x1b[1;1H\x1b]0;pwned\x07HIJACKED"
	row := conflictRow(shadowConflict{file: "/a/config", line: "1", key: "title", text: evil}, 60)
	for _, bad := range []string{"\x1b[2J", "\x1b]0;pwned", "\x07"} {
		if strings.Contains(row, bad) {
			t.Errorf("escape sequence %q survived into the rendered row", bad)
		}
	}
	if got := parseShadowRecords("/a/config\x1f1\x1ftitle\x1f" + evil); len(got) != 1 ||
		strings.Contains(got[0].text, "\x1b") {
		t.Errorf("parser did not defuse the record: %+v", got)
	}
}

// truncate re-measured the whole string on every pass, which is quadratic:
// 2.5s at 20k characters and 85s at 100k. The conflicts panel truncates lines
// from a file this tool does not own, on a path that redraws every keystroke.
func TestTruncateIsNotQuadratic(t *testing.T) {
	start := time.Now()
	got := truncate(strings.Repeat("a", 200000), 34)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("truncating 200k chars took %v — the quadratic path is back", elapsed)
	}
	if lipgloss.Width(got) > 34 {
		t.Errorf("result is %d cols, wanted at most 34", lipgloss.Width(got))
	}
}

// The sample is a solid block or it is nothing: an earlier version truncated
// mid-row, which is why the swatch strip was written instead.
func TestSamplePanelRowsAreUniform(t *testing.T) {
	cs := colorSet{background: "#000000", foreground: "#00ff41", cursor: "#00ff41"}
	for i := range cs.palette {
		cs.palette[i] = "#00b32c"
	}
	for _, paneW := range []int{28, 40, 60, 120} {
		rows := samplePanel(cs, paneW)
		if len(rows) == 0 {
			t.Fatalf("pane %d: nothing rendered", paneW)
		}
		want := lipgloss.Width(rows[0])
		for i, r := range rows {
			if got := lipgloss.Width(r); got != want {
				t.Errorf("pane %d row %d: %d cols, first row %d", paneW, i, got, want)
			}
		}
		if want > paneW {
			t.Errorf("pane %d: rows are %d cols, wider than the pane", paneW, want)
		}
	}
	// A theme with no colours must render nothing rather than black on black.
	if samplePanel(colorSet{}, 60) != nil {
		t.Error("a colourless theme should produce no sample")
	}
}
