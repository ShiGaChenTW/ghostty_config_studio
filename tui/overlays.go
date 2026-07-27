package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// overlay is one modal layer: open reports whether it currently owns the
// keyboard, update handles one key while it does.
type overlay struct {
	open   func(model) bool
	update func(model, tea.KeyMsg) (tea.Model, tea.Cmd)
}

var orderedOverlays = []overlay{
	{open: func(m model) bool { return m.showRestartPrompt }, update: model.updateRestartPrompt},
	{open: func(m model) bool { return m.editor != nil }, update: model.updateEditor},
	{open: func(m model) bool { return m.showTagPanel }, update: model.updateTagPanel},
	{open: func(m model) bool { return m.showDeleteConfirm }, update: model.updateDeleteConfirm},
	{open: func(m model) bool { return m.showConflictConfirm }, update: model.updateConflictConfirm},
	{open: func(m model) bool { return m.showConflicts }, update: model.updateConflictsPanel},
	{open: func(m model) bool { return m.showEditPicker }, update: model.updateEditPicker},
	{open: func(m model) bool { return m.showNameDialog }, update: model.updateNameDialog},
}

func (m model) updateOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	for _, overlay := range orderedOverlays {
		if overlay.open(m) {
			next, cmd := overlay.update(m, msg)
			return next, cmd, true
		}
	}
	return m, nil, false
}

func (m model) updateRestartPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Modal focus trap — per tui-design's dialog guidance, the
	// background receives no events while a confirmation is open.
	// Checked BEFORE the normal q/enter handling below, otherwise
	// "q" here would quit the whole program instead of dismissing.
	switch strings.ToLower(msg.String()) {
	case "y", "enter":
		restartGhostty()
		return m, tea.Quit
	case "n", "esc", "q", "ctrl+c":
		m.showRestartPrompt = false
	}
	return m, nil
}

func (m model) updateTagPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Same modal focus trap as the other overlays: while the panel
	// is open the list underneath receives nothing, so "q" closes
	// the panel instead of quitting the program.
	rows := tagRows()
	switch strings.ToLower(msg.String()) {
	case "esc", "q", "ctrl+c", "enter", "t":
		m.showTagPanel = false
	case "up", "k":
		if m.tagCursor > 0 {
			m.tagCursor--
		}
	case "down", "j":
		if m.tagCursor < len(rows)-1 {
			m.tagCursor++
		}
	case " ", "space", "x":
		t := rows[m.tagCursor]
		if m.activeTags[t] {
			delete(m.activeTags, t)
		} else {
			m.activeTags[t] = true
		}
		m = m.applyTagFilter()
		m.lastPreviewKey = ""
		m.updatePreviewForSelection()
	case "c":
		m.activeTags = map[string]bool{}
		m = m.applyTagFilter()
		m.lastPreviewKey = ""
		m.updatePreviewForSelection()
	}
	return m, nil
}

func (m model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		return m.deleteCustomConfig(m.deleteTarget)
	case "n", "esc", "q", "ctrl+c":
		m.showDeleteConfirm = false
	}
	return m, nil
}

func (m model) updateConflictConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Checked before the panel that raised it, the same way the
	// delete confirmation is checked before the edit picker.
	switch strings.ToLower(msg.String()) {
	case "y", "enter":
		return m.fixConflicts()
	case "n", "esc", "q", "ctrl+c":
		m.showConflictConfirm = false
	}
	return m, nil
}

func (m model) updateConflictsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Same modal focus trap as the other overlays: while the panel is
	// open "q" closes it rather than quitting the program.
	switch strings.ToLower(msg.String()) {
	case "esc", "q", "ctrl+c", "c":
		m.showConflicts = false
	case "f":
		// Nothing to confirm when there is nothing to comment out,
		// and the footer doesn't offer the key in that case either.
		if len(m.conflicts) > 0 {
			m.showConflictConfirm = true
		}
	}
	return m, nil
}

func (m model) updateEditPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "ctrl+c":
		m.showEditPicker = false
	case "up", "k":
		if m.editPickIndex > 0 {
			m.editPickIndex--
		}
	case "down", "j":
		if m.editPickIndex < len(m.editable)-1 {
			m.editPickIndex++
		}
	case "d":
		m.deleteTarget = m.editable[m.editPickIndex]
		m.showDeleteConfirm = true
	case "enter":
		m.showEditPicker = false
		return m.openFieldEditor(m.editable[m.editPickIndex])
	}
	return m, nil
}

func (m model) updateNameDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.showNameDialog = false
		m.nameInput.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.status = txtNameFirst()
			m.statusOK = false
			return m, nil
		}
		m.showNameDialog = false
		m.nameInput.Blur()
		if m.namePurpose == "new-file" {
			return m.startNewFile(name)
		}
		return m.saveCurrentCombo(name)
	}
	var tiCmd tea.Cmd
	m.nameInput, tiCmd = m.nameInput.Update(msg)
	return m, tiCmd
}

func (m model) updateBrowserKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
		return m, tea.Quit, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("t"))):
		m.showTagPanel = true
		return m, nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("L"))):
		// Uppercase L: lowercase l is vim-style "right" in the
		// editor, so the shifted key keeps both free.
		toggleLang()
		// Item descriptions are language-dependent, so the list's
		// items have to be rebuilt for the change to show.
		m = m.refreshEntries()
		m.list.SetStatusBarItemName(txtStatusItemName())
		m.list.FilterInput.Prompt = txtFilterPrompt()
		m.lastPreviewKey = ""
		m.updatePreviewForSelection()
		m.status = ""
		return m, nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		m.showNameDialog = true
		m.namePurpose = "save-current"
		m.nameInput.Placeholder = "my-preset-name"
		m.nameInput.SetValue("")
		m.nameInput.Focus()
		return m, textinput.Blink, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
		m.showNameDialog = true
		m.namePurpose = "new-file"
		m.nameInput.Placeholder = "my-new-config"
		m.nameInput.SetValue("")
		m.nameInput.Focus()
		return m, textinput.Blink, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("p"))):
		if item, ok := m.list.SelectedItem().(entry); ok {
			if err := previewEntry(m.dir, item); err != nil {
				m.status = txtPreviewFailed(err.Error())
				m.statusOK = false
			} else {
				m.status = txtPreviewOpened(item.source + "/" + item.name)
				m.statusOK = true
			}
		}
		return m, nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
		// Shadows one of bubbles/list's five prev-page keys; b, pgup,
		// ← and h all still page, so nothing is lost.
		line, err := undoLastApply(m.dir)
		if err != nil {
			m.status = txtUndoFailed(err.Error())
			m.statusOK = false
			return m, nil, true
		}
		if line == "" {
			line = txtUndoDone()
		}
		m.status = line
		m.statusOK = true
		// The managed block just moved underneath us — without this
		// the "●" markers keep naming what was applied before.
		m.current = currentSelections(m.dir)
		return m, nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
		// Show only what can actually be edited, rather than acting
		// on whatever happens to be selected and then refusing.
		m.editable = editableEntries(m.dir)
		if len(m.editable) == 0 {
			m.status = txtNoEditable()
			m.statusOK = false
			return m, nil, true
		}
		m.showEditPicker = true
		m.editPickIndex = 0
		return m, nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("c", "C"))):
		// `c` is free on both sides: no handler on this screen takes
		// it, and bubbles/list's own bindings never do either — its
		// paging keys are h/l/b/f/u/d and the arrows. Uppercase is
		// bound too, matching the restart hint's y/Y.
		m = m.openConflicts()
		return m, nil, true
	case m.restartHint && key.Matches(msg, key.NewBinding(key.WithKeys("y", "Y"))):
		m.restartHint = false
		m.showRestartPrompt = true
		return m, nil, true
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if item, ok := m.list.SelectedItem().(entry); ok {
			out, err := applyEntry(m.dir, item)
			if err != nil {
				detail := strings.TrimSpace(out)
				if detail == "" {
					detail = err.Error()
				}
				m.status = fmt.Sprintf("failed to switch %s to %s/%s: %s", item.category, item.source, item.name, detail)
				m.statusOK = false
			} else {
				m.status = txtSwitched(item.category, item.source+"/"+item.name, item.desc)
				m.statusOK = true
				// apply_selection warns on stderr when another config
				// that Ghostty loads later sets the same keys. That is
				// the difference between "nothing happened" and
				// "nothing happened, and here is why", so it replaces
				// the success line rather than scrolling past behind it.
				warning := firstWarningLine(out)
				if warning != "" {
					// The write itself went through — the warning is
					// only about a possible later override — so the
					// restart offer stays available, as a hint the
					// warning can sit next to rather than a popup
					// that would cover it. The conflict hint rides
					// along so the fix is found at the moment the
					// problem is hit, not by going looking for it.
					m.status = warning + "  ·  " + txtConflictHint() +
						"  ·  " + txtRestartHint()
					m.statusOK = false
				}
				m.current = currentSelections(m.dir)
				m.showRestartPrompt = warning == ""
				m.restartHint = warning != ""
				recordRecent(item.recentKey())
			}
		}
		return m, nil, true
	}
	return m, nil, false
}
