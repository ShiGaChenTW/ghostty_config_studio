package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tag filtering, opened with `t`.
//
// Tags were already folded into the fuzzy search index, which meant you could
// type /復古 and get the six retro entries. That worked by accident more than
// design: the matcher sees one flat string per row and has no idea which part
// of it was a tag, so /retro also matched every theme with those five letters
// in its name, and there was nothing anywhere telling you the vocabulary was
// nine words long. This makes the tags a real axis: listed, counted,
// intersectable, and composable with the text search rather than competing
// with it.

// The two groups are kept apart on purpose. Character tags are hand-assigned
// per entry and describe intent. Tone is computed from background luminance,
// so it is available on all 460+ built-in themes and consequently splits the
// list roughly three-to-one rather than picking anything out. Mixing them in
// one list invites treating "dark" as a filter as meaningful as "retro".
var vibeTags = []string{
	"nature", "calm", "animated", "retro",
	"vibrant", "cyberpunk", "minimal", "professional",
}

var toneTags = []string{"dark", "light"}

// matchesTags reports whether an entry carries every active tag. Intersection,
// not union: adding a tag should always narrow, which is the only behaviour
// that makes stacking them worth doing.
func matchesTags(e entry, active map[string]bool) bool {
	if len(active) == 0 {
		return true
	}
	have := make(map[string]bool, len(e.tags))
	for _, t := range e.tags {
		have[t] = true
	}
	for t := range active {
		if !have[t] {
			return false
		}
	}
	return true
}

// tagCount is how many entries would remain if this tag were added to the
// current selection. Counting against the live selection rather than the whole
// catalog is what makes a second tag readable: it shows what the combination
// leaves, so a zero tells you not to bother before you press space.
func tagCount(entries []entry, active map[string]bool, tag string) int {
	probe := make(map[string]bool, len(active)+1)
	for t := range active {
		probe[t] = true
	}
	probe[tag] = true
	n := 0
	for _, e := range entries {
		if matchesTags(e, probe) {
			n++
		}
	}
	return n
}

func filterByTags(entries []entry, active map[string]bool) []entry {
	if len(active) == 0 {
		return entries
	}
	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		if matchesTags(e, active) {
			out = append(out, e)
		}
	}
	return out
}

// tagRows is the panel's flat cursor space: every selectable tag in display
// order. The group headings are drawn from the same order but are not
// selectable, so the cursor never lands on one.
func tagRows() []string {
	rows := make([]string, 0, len(vibeTags)+len(toneTags))
	rows = append(rows, vibeTags...)
	rows = append(rows, toneTags...)
	return rows
}

// activeTagLabel renders the current selection for the pane header, in
// whichever language is on screen.
func activeTagLabel(active map[string]bool) string {
	if len(active) == 0 {
		return ""
	}
	var parts []string
	for _, t := range tagRows() {
		if active[t] {
			parts = append(parts, styleTag(t))
		}
	}
	return strings.Join(parts, " · ")
}

// tagPanel draws the dialog, following the same shape as the other overlays:
// a bracketed title, the body, then the footer that names the keys.
func (m model) tagPanel() string {
	var v []string
	v = append(v, bracket(txtTagTitle()))
	v = append(v, helpStyle.Render(txtTagBody()))
	v = append(v, "")

	draw := func(group string, tags []string, offset int) {
		v = append(v, labelStyle.Render(group))
		for i, t := range tags {
			idx := offset + i
			box := "[ ]"
			if m.activeTags[t] {
				box = "[x]"
			}
			n := tagCount(m.allEntries, m.activeTags, t)
			row := box + " " + padRight(styleTag(t), 12) + "(" + itoa(n) + ")"
			switch {
			case idx == m.tagCursor:
				v = append(v, statusStyle.Render("▸ "+row))
			case m.activeTags[t]:
				v = append(v, phosphorStyle.Render("  "+row))
			default:
				// A zero here means the tag adds nothing to what is already
				// picked, which is worth seeing before pressing space.
				v = append(v, helpStyle.Render("  "+row))
			}
		}
		v = append(v, "")
	}
	draw(txtTagVibe(), vibeTags, 0)
	draw(txtTagTone(), toneTags, len(vibeTags))

	kept := len(filterByTags(m.allEntries, m.activeTags))
	if len(m.activeTags) == 0 {
		v = append(v, helpStyle.Render(txtItemCount(len(m.allEntries))))
	} else if kept == 0 {
		v = append(v, errorStyle.Render(txtTagNone()))
	} else {
		v = append(v, phosphorStyle.Render(txtTagActive(activeTagLabel(m.activeTags), kept)))
	}
	v = append(v, "")
	v = append(v, helpStyle.Render(txtTagFooter()))

	content := strings.Join(v, "\n")
	w := minInt(maxInt(lipglossMaxWidth(v)+2, 48), m.width-8)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		box(content, w, len(v)))
}
