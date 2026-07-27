package main

// The settings editor: a checkbox browser over all 200 Ghostty config keys,
// split into category pages.
//
// Layout:
//
//	┌─ [外觀與顏色] 字型 游標 … ──────────┐┌─ detail ────┐
//	│ [x] background (視窗背景色) : 色碼  ││ background  │
//	│ [ ] foreground (視窗前景) : 色碼    ││ 視窗背景色  │
//	│ …                                   ││ 可填值 …    │
//	└─────────────────────────────────────┘└─────────────┘
//
// Space toggles a row's checkbox and immediately opens its value editor.
// Keys whose legal values Ghostty enumerates get an options picker (never
// free typing); everything else gets a text input showing the accepted
// range. Checked rows are exactly what gets written to the config file.

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// editorState is the whole settings-editor mode. Kept in its own struct so
// main.go's model stays readable and so "is the editor open" is a single
// nil check rather than a spread of booleans.
type editorState struct {
	path string // file being edited
	name string

	catIndex int   // which category page
	rowIndex []int // per-category cursor position, so switching tabs remembers where you were

	checked map[string]string // key -> value; presence means "include in file"

	// Value editing, opened by space on a row.
	editingKey  string // "" when no value editor is open
	optIndex    int    // cursor in the options picker
	usingPicker bool   // true = enumerated options picker, false = free text
	input       textinput.Model

	newFile bool // created this session; delete on exit if nothing was checked
}

func newEditorState(path, name string, existing []fieldRow, newFile bool) *editorState {
	checked := map[string]string{}
	for _, r := range existing {
		checked[r.key] = r.value
	}
	ti := textinput.New()
	ti.CharLimit = 160
	ti.Width = 46
	return &editorState{
		path:     path,
		name:     name,
		rowIndex: make([]int, len(categoryOrder)),
		checked:  checked,
		input:    ti,
		newFile:  newFile,
	}
}

// rowsFor returns the catalog entries on a category page. Rebuilt on demand
// rather than cached — 200 entries is nothing, and it keeps the state
// struct free of a second copy that could drift.
func rowsFor(cat string) []keyInfo {
	var out []keyInfo
	for _, k := range keyCatalog {
		if k.cat == cat {
			out = append(out, k)
		}
	}
	return out
}

// tabCount is the "(3/27)" a tab carries: how many of that page's settings
// are ticked, out of how many it has. Without it the only way to find where
// your changes are is to walk all fourteen pages.
func (e *editorState) tabCount(cat string) (checked, total int) {
	rows := rowsFor(cat)
	for _, k := range rows {
		if _, ok := e.checked[k.key]; ok {
			checked++
		}
	}
	return checked, len(rows)
}

func (e *editorState) curCat() string     { return categoryOrder[e.catIndex] }
func (e *editorState) curRows() []keyInfo { return rowsFor(e.curCat()) }

func (e *editorState) curRow() (keyInfo, bool) {
	rows := e.curRows()
	i := e.rowIndex[e.catIndex]
	if i < 0 || i >= len(rows) {
		return keyInfo{}, false
	}
	return rows[i], true
}

// orderedRows returns the checked settings in catalog order, so a saved
// file is grouped by category rather than in whatever order they happened
// to be clicked.
func (e *editorState) orderedRows() []fieldRow {
	var out []fieldRow
	seen := map[string]bool{}
	for _, k := range keyCatalog {
		if v, ok := e.checked[k.key]; ok {
			out = append(out, fieldRow{key: k.key, value: v})
			seen[k.key] = true
		}
	}
	// Keys already in the file that this catalog doesn't know about (e.g.
	// written by a newer Ghostty, or hand-added) must survive a save —
	// silently dropping them would corrupt the user's own config.
	for k, v := range e.checked {
		if !seen[k] {
			out = append(out, fieldRow{key: k, value: v})
		}
	}
	return out
}

// --- update ---

func (e *editorState) openValueEditor(ki keyInfo) tea.Cmd {
	e.editingKey = ki.key
	if len(ki.validVals) > 0 {
		e.usingPicker = true
		e.optIndex = 0
		// Start the cursor on the current value if there is one, else the
		// Ghostty default — so confirming without moving keeps what's set.
		want := e.checked[ki.key]
		if want == "" {
			want = ki.defaultVal
		}
		for i, v := range ki.validVals {
			if v == want {
				e.optIndex = i
				break
			}
		}
		return nil
	}
	e.usingPicker = false
	cur := e.checked[ki.key]
	if cur == "" {
		cur = ki.defaultVal
	}
	e.input.SetValue(cur)
	e.input.Placeholder = ki.rangeLabel()
	e.input.Focus()
	return textinput.Blink
}

func (e *editorState) update(msg tea.KeyMsg) tea.Cmd {
	// Value editor is modal over the row list.
	if e.editingKey != "" {
		ki, _ := lookupKey(e.editingKey)
		if e.usingPicker {
			switch msg.String() {
			case "esc":
				// Cancelling the value editor also un-checks, so you never
				// end up with a checked setting that has no chosen value.
				if _, had := e.checked[e.editingKey]; !had {
					delete(e.checked, e.editingKey)
				}
				e.editingKey = ""
			case "up", "k":
				if e.optIndex > 0 {
					e.optIndex--
				}
			case "down", "j":
				if e.optIndex < len(ki.validVals)-1 {
					e.optIndex++
				}
			case "enter", " ":
				e.checked[e.editingKey] = ki.validVals[e.optIndex]
				e.editingKey = ""
			}
			return nil
		}
		switch msg.String() {
		case "esc":
			if v, had := e.checked[e.editingKey]; !had || v == "" {
				delete(e.checked, e.editingKey)
			}
			e.editingKey = ""
			e.input.Blur()
			return nil
		case "enter":
			// An empty value writes "key = " — Ghostty silently ignores
			// such a line, so the user would see it checked here and get
			// no effect at all. Treat empty as "didn't set it".
			v := strings.TrimSpace(e.input.Value())
			if v == "" {
				delete(e.checked, e.editingKey)
			} else {
				e.checked[e.editingKey] = v
			}
			e.editingKey = ""
			e.input.Blur()
			return nil
		}
		var cmd tea.Cmd
		e.input, cmd = e.input.Update(msg)
		return cmd
	}

	rows := e.curRows()
	switch msg.String() {
	case "L":
		toggleLang()
		return nil
	case "left", "h", "shift+tab":
		if e.catIndex > 0 {
			e.catIndex--
		}
	case "right", "l", "tab":
		if e.catIndex < len(categoryOrder)-1 {
			e.catIndex++
		}
	case "up", "k":
		if e.rowIndex[e.catIndex] > 0 {
			e.rowIndex[e.catIndex]--
		}
	case "down", "j":
		if e.rowIndex[e.catIndex] < len(rows)-1 {
			e.rowIndex[e.catIndex]++
		}
	case " ":
		ki, ok := e.curRow()
		if !ok {
			return nil
		}
		if _, isChecked := e.checked[ki.key]; isChecked {
			delete(e.checked, ki.key) // space on a checked row un-checks it
			return nil
		}
		e.checked[ki.key] = "" // provisional; the value editor fills it in
		return e.openValueEditor(ki)
	case "enter":
		// Re-open the value editor for an already-checked row without
		// toggling it off.
		ki, ok := e.curRow()
		if !ok {
			return nil
		}
		return e.openValueEditor(ki)
	}
	return nil
}

// --- view ---

// sideMargin insets the whole editor from the window edges. Terminals
// address character cells, not pixels — at a typical ~8-10px cell width
// one column is the closest honest equivalent of the requested 10px.
const sideMargin = 1

func (e *editorState) view(width, height int, status string) string {
	innerW := maxInt(width-2*sideMargin, 40)

	detailW := minInt(maxInt(innerW/3, 26), 40)
	listW := innerW - detailW - 8
	if listW < 30 {
		listW = 30
	}

	// Styled tab bar, wrapped over as many rows as it needs: with padding
	// and shadows the 14 categories run ~181 columns, well past a normal
	// window, so a single line would silently truncate the later tabs.
	tabRows := e.renderTabs(innerW)
	// Rows consumed outside the two panes: 2 leading blanks + 4 for the
	// title band (3 filled rows + its shadow row) + 1 blank + the tab rows
	// + 2 pane borders + 1 footer.
	bodyH := maxInt(height-10-len(tabRows), 6)

	// Each pane spends its first two rows on its own name + a blank.
	listRows := maxInt(bodyH-2, 3)

	rows := e.curRows()
	cur := e.rowIndex[e.catIndex]
	// A blank line between entries: packed one per line, 25 settings of
	// similar shape ran together and the eye had nothing to land on. Each
	// entry costs two display lines now, so the scroll window has to count
	// entries rather than lines or the cursor walks off the bottom.
	const linesPerEntry = 2
	visible := maxInt((listRows+1)/linesPerEntry, 1)
	start := 0
	if cur >= visible {
		start = cur - visible + 1
	}
	end := minInt(start+visible, len(rows))

	lines := []string{labelStyle.Render(txtPaneItems()), ""}
	for i := start; i < end; i++ {
		ki := rows[i]
		box := "[ ]"
		if v, ok := e.checked[ki.key]; ok {
			box = "[x]"
			_ = v
		}
		label := box + " " + ki.name()
		val := ki.rangeLabel()
		if v, ok := e.checked[ki.key]; ok && v != "" {
			val = "= " + v
		}
		line := truncate(label+" : "+val, listW)
		if i == cur {
			lines = append(lines, statusStyle.Render("▸ "+line))
		} else if _, ok := e.checked[ki.key]; ok {
			lines = append(lines, phosphorStyle.Render("  "+line))
		} else {
			lines = append(lines, helpStyle.Render("  "+line))
		}
		if i < end-1 {
			lines = append(lines, "")
		}
	}
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	listPane := box(strings.Join(lines, "\n"), listW, bodyH)

	// Detail pane on the right, describing the row under the cursor.
	det := []string{labelStyle.Render(txtPaneDetail()), ""}
	if ki, ok := e.curRow(); ok {
		det = append(det, titleStyle.Render(ki.key))
		det = append(det, "")
		det = append(det, wrapText(ki.desc(), detailW)...)
		det = append(det, "")
		det = append(det, labelStyle.Render(txtAllowedValue()))
		det = append(det, wrapText(ki.rangeLabel(), detailW)...)
		if ki.defaultVal != "" {
			det = append(det, "")
			det = append(det, labelStyle.Render(txtDefault()))
			det = append(det, wrapText(ki.defaultVal, detailW)...)
		}
		if v, ok := e.checked[ki.key]; ok && v != "" {
			det = append(det, "")
			det = append(det, statusStyle.Render(txtCurrentValue(v)))
		}
	}
	for len(det) < bodyH {
		det = append(det, "")
	}
	detailPane := box(strings.Join(det[:bodyH], "\n"), detailW, bodyH)

	// List left, detail right — matches the main browser screen's
	// list-left/preview-right layout so the eye lands in the same place in
	// both modes.
	main := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)

	// Orange title band with a drop shadow, same component as the browser
	// screen so both headers read as one piece of chrome.
	left := bannerTitleStyle.Render("GHOSTTY CONFIG STUDIO") +
		bannerDimStyle.Render("  ·  "+txtEditorMode()+"  ·  ") +
		bannerTitleStyle.Render(e.name)
	right := bannerDimStyle.Render(txtCheckedCount(len(e.checked), len(keyCatalog)))
	head := bannerBlock(left, right, innerW)

	foot := helpStyle.Render(txtEditorFooter())

	// Two blank rows above the header, one below it.
	parts := []string{"", "", head, ""}
	parts = append(parts, tabRows...)
	parts = append(parts, main)
	// Same rule as the browser screen: footer directly under the card,
	// status row only when there's a message. Without the status line the
	// editor gave no feedback at all on save — the confirmation only
	// appeared after leaving the editor.
	if status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, foot)
	out := indentBlock(strings.Join(parts, "\n"), sideMargin)

	// The value editor floats over everything else when open.
	if e.editingKey != "" {
		ki, _ := lookupKey(e.editingKey)
		var v []string
		v = append(v, bracket(ki.key))
		v = append(v, helpStyle.Render(ki.desc()))
		v = append(v, "")
		if e.usingPicker {
			v = append(v, labelStyle.Render(txtPickValue()))
			v = append(v, "")
			for i, opt := range ki.validVals {
				mark := "  "
				if i == e.optIndex {
					mark = "▸ "
				}
				note := ""
				if opt == ki.defaultVal {
					note = helpStyle.Render(txtIsDefault())
				}
				label := opt
				if opt == " " {
					label = txtBlankValue()
				}
				if i == e.optIndex {
					v = append(v, statusStyle.Render(mark+label)+note)
				} else {
					v = append(v, phosphorStyle.Render(mark+label)+note)
				}
			}
			v = append(v, "")
			v = append(v, helpStyle.Render(txtValuePickerFooter()))
		} else {
			v = append(v, labelStyle.Render(txtAllowedRange())+" "+ki.rangeLabel())
			if ki.defaultVal != "" {
				v = append(v, labelStyle.Render(txtDefaultValue())+" "+ki.defaultVal)
			}
			v = append(v, "")
			v = append(v, e.input.View())
			v = append(v, "")
			v = append(v, helpStyle.Render(txtTextValueFooter()))
		}
		content := strings.Join(v, "\n")
		w := minInt(maxInt(lipglossMaxWidth(v)+2, 40), width-8)
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			box(content, w, len(v)))
	}

	return out
}

// renderTabs draws every category as an orange-ground / white-lettered
// button with a dark shadow cell on its right, wrapping onto extra rows
// rather than letting later categories fall off the edge. The active tab
// uses a brighter orange and bold text — with every tab coloured, that's
// the remaining way to show which page you're on.
//
// Returns display rows with a blank row between each wrapped line, so the
// shadow under one row doesn't crowd the tabs on the next.
func (e *editorState) renderTabs(width int) []string {
	var rows []string
	var line string
	lineW := 0
	for i, c := range categoryOrder {
		style := tabInactiveStyle
		if i == e.catIndex {
			style = tabActiveStyle
		}
		checked, total := e.tabCount(c)
		label := categoryLabel(c) + " (" + itoa(checked) + "/" + itoa(total) + ")"
		tab := style.Render(" "+label+" ") + tabShadowStyle.Render(" ")
		tw := lipgloss.Width(tab)
		if lineW > 0 && lineW+1+tw > width {
			rows = append(rows, line, "") // blank spacer between tab rows
			line, lineW = "", 0
		}
		if lineW > 0 {
			line += " "
			lineW++
		}
		line += tab
		lineW += tw
	}
	if line != "" {
		rows = append(rows, line)
	}
	return rows
}

// indentBlock shifts every line right by n columns, insetting the view
// from the window edge.
func indentBlock(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

// truncate cuts s to at most w display columns, appending … when cut.
// Uses lipgloss.Width so it stays correct for CJK (double-width) text and
// for already-styled input.
// The loop measures the whole string on every iteration, so trimming one rune
// at a time from a long line is quadratic: 20k characters took 2.5s and 100k
// took 85s, measured. The conflicts panel truncates lines copied verbatim out
// of a config file this tool does not own, and re-renders on every keystroke,
// so "nobody would have a line that long" is not a safe assumption. No rune is
// narrower than one column, so anything past the first w runes cannot appear
// in the result and can be dropped before measuring anything.
//
// Only for input with no escape sequences in it: a rune-wise cut through
// already-styled text can sever an escape, and those strings are ours and
// short.
func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	if len(r) > w && !strings.ContainsRune(s, 0x1b) {
		r = r[:w]
	}
	for len(r) > 0 && lipgloss.Width(string(r)+"…") > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// wrapText hard-wraps to w display columns, splitting on runes so CJK
// (which has no spaces to break on) wraps correctly.
func wrapText(s string, w int) []string {
	if w < 4 {
		w = 4
	}
	var out []string
	line := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, line)
			line = ""
			continue
		}
		if lipgloss.Width(line+string(r)) > w {
			out = append(out, line)
			line = ""
		}
		line += string(r)
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func lipglossMaxWidth(lines []string) int {
	m := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > m {
			m = w
		}
	}
	return m
}
