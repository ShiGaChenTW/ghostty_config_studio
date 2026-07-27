// ghostty-tui: interactive Bubble Tea browser for Ghostty Config Studio.
//
// Left pane lists every theme/font/preset (18 curated + 460+ Ghostty
// built-in themes); right pane previews the raw config content live as the
// cursor moves. Enter applies the selection. Applying is delegated to
// lib/menu.sh's apply_selection (shelled out via bash) so the managed-block
// write logic has exactly one implementation, not one per language.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/x/ansi"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type entry struct {
	source      string // github username: snedea / naydenoff / ghostty
	name        string
	desc        string
	category    string // theme | font | preset
	kind        string // file | name
	value       string // config-file path, or built-in theme name
	shaderSrc   string // .glsl to copy alongside, or ""
	previewPath string // file to show in the preview pane, or ""
	recent      bool   // pinned to the top of the list — applied in a recent session
	tags        []string
	settingKey  string // Ghostty config key for kind="raw" entries (e.g. "background-opacity"); empty otherwise
}

func (e entry) recentKey() string { return e.category + "|" + e.kind + "|" + e.value }

func (e entry) Title() string {
	if e.recent {
		return "★ " + e.source + "/" + e.name
	}
	return e.source + "/" + e.name
}
func (e entry) Description() string {
	tag := "[" + categoryTag(e.category) + "]"
	if len(e.tags) > 0 {
		labels := make([]string, len(e.tags))
		for i, t := range e.tags {
			labels[i] = styleTag(t)
		}
		tag += " (" + strings.Join(labels, " ") + ")"
	}
	return tag + " " + e.desc
}
func (e entry) FilterValue() string {
	// Category and style tags go in under both languages, always. Someone
	// reading the Chinese interface still types /nord, and someone reading
	// the English one may well have learned the row as 游標.
	return e.source + " " + e.name + " " + e.filterDesc() + " " +
		searchAliases(e.category, e.tags)
}

// filterDesc is the description as the fuzzy filter sees it. Ghostty's 460+
// built-in themes all carry the same one-line label, which discriminates
// between exactly none of them while handing the matcher another twenty
// characters to find a subsequence in. Leaving it out keeps a query like
// /retro ranked on the handful of rows that really are retro.
func (e entry) filterDesc() string {
	if e.category == "theme" && e.kind == "name" {
		return ""
	}
	return e.desc
}

var shaderDescOverride = map[string]string{
	"cyberpunk": "Cyberpunk -- neon synthwave with wandering bug behind the screen",
	"neon":      "Neon -- VHS retro glow, neon distortion flicker",
}

var colorDesc = map[string]string{
	"catppuccin-mocha": "Catppuccin Mocha -- modern & cozy, warm purples, soft pastels",
	"dracula":          "Dracula -- classic gothic elegance, rich purples, blood red accents",
	"gruvbox":          "Gruvbox -- retro warmth, warm browns, vintage yellows",
	"nord":             "Nord -- arctic minimalism, cool blues, polar ice palette",
	"tokyo-night":      "Tokyo Night -- cyberpunk vibes, deep blues, electric accents",
	"warp-dark":        "Warp Dark -- matches Warp terminal's default dark theme",
}

var fontDesc = map[string]string{
	"cascadia-code":  "Cascadia Code -- Microsoft's modern font, excellent cursive italics",
	"fira-code":      "Fira Code -- beautiful coding ligatures, clean and programming-focused",
	"iosevka":        "Iosevka -- ultra-narrow, space-efficient, maximizes code density",
	"jetbrains-mono": "JetBrains Mono -- excellent readability, built-in ligatures",
}

var presetDesc = map[string]string{
	"cozy-coding":          "Cozy Coding -- Gruvbox + JetBrains Mono, warm coffee-shop marathon sessions",
	"cyberpunk-dev":        "Cyberpunk Dev -- Tokyo Night + Fira Code, neon-lit React/TypeScript coding",
	"minimal-focus":        "Minimal Focus -- Nord + Iosevka, clean Scandinavian, max density",
	"professional-elegant": "Professional Elegant -- Dracula + Cascadia Code, refined for client demos",
	"warp-dark":            "Warp Dark -- matches Warp terminal's appearance and typography",
}

var cursorShaderDesc = map[string]string{
	"cursor_tail":             "Cursor Tail -- comet-like trail, Kitty-style",
	"cursor_sweep":            "Cursor Sweep -- animated shrinking trail",
	"cursor_warp":             "Cursor Warp -- Neovide-like warp animation",
	"sonic_boom_cursor":       "Sonic Boom -- expanding filled circle on cursor move",
	"ripple_cursor":           "Ripple -- circular ring ripple on cursor move",
	"rectangle_boom_cursor":   "Rectangle Boom -- rectangular boom variant",
	"ripple_rectangle_cursor": "Rectangle Ripple -- rectangular ripple variant",
}

// curatedTags hand-tags the 34 vendored/curated items (12 shader themes + 6
// color themes + 4 fonts + 5 presets + 7 cursor shaders) by vibe/intensity.
// Vocabulary kept small and fixed on purpose: retro, cyberpunk, nature,
// minimal, professional (vibe — pick the one that fits best, not a grab
// bag) plus calm/vibrant/animated (intensity — not mutually exclusive).
// The 460+ built-in themes are NOT hand-tagged (not scalable); they get
// dark/light auto-derived from resolved background luminance instead — see
// autoTagsFor().
var curatedTags = map[string][]string{
	// shader themes (snedea)
	"aquarium":  {"nature", "calm", "animated"},
	"aurora":    {"nature", "calm", "animated"},
	"campfire":  {"nature", "calm", "animated"},
	"cyberpunk": {"cyberpunk", "vibrant", "animated"},
	"dude":      {"retro", "calm", "animated"},
	"hotdog":    {"retro", "vibrant", "animated"},
	"matrix":    {"cyberpunk", "vibrant", "animated"},
	"neon":      {"retro", "vibrant", "animated"},
	"noir":      {"minimal", "calm", "animated"},
	"pico":      {"retro", "vibrant", "animated"},
	"pipboy":    {"retro", "vibrant", "animated"},
	"sakura":    {"nature", "calm", "animated"},
	// color themes (naydenoff)
	"catppuccin-mocha": {"minimal", "calm"},
	"dracula":          {"professional", "vibrant"},
	"gruvbox":          {"retro", "calm"},
	"nord":             {"minimal", "calm"},
	"tokyo-night":      {"cyberpunk", "vibrant"},
	"warp-dark":        {"professional", "minimal"},
	// fonts (naydenoff) — use-case rather than vibe
	"cascadia-code":  {"professional"},
	"fira-code":      {"professional"},
	"iosevka":        {"minimal"},
	"jetbrains-mono": {"professional"},
	// presets (naydenoff)
	"cozy-coding":          {"nature", "calm"},
	"cyberpunk-dev":        {"cyberpunk", "vibrant"},
	"minimal-focus":        {"minimal", "calm"},
	"professional-elegant": {"professional", "calm"},
	// cursor shaders (sahaj-b)
	"cursor_tail":             {"calm", "animated"},
	"cursor_sweep":            {"calm", "animated"},
	"cursor_warp":             {"vibrant", "animated"},
	"sonic_boom_cursor":       {"vibrant", "animated"},
	"ripple_cursor":           {"calm", "animated"},
	"rectangle_boom_cursor":   {"vibrant", "animated"},
	"ripple_rectangle_cursor": {"calm", "animated"},
}

// autoTagsFor derives dark/light for any entry with resolvable colors —
// used for the 460+ built-in themes, where hand-tagging doesn't scale.
// Relative luminance (perceived-brightness weighted, not a straight
// average) against a mid-point threshold; good enough for a binary split,
// not trying to be a color-science library.
// tagsFor combines an entry's hand-assigned character tags with the tone
// derived from its own background. Tone used to be computed only for the
// built-in themes, which made the two groups in the tag panel mutually
// exclusive: every "retro AND dark" combination came back empty because no
// curated entry carried a tone at all. Copies the curated slice rather than
// appending to it, or the shared backing array leaks tone tags between
// entries that happen to sit next to each other in the map.
func tagsFor(name, previewPath string) []string {
	curated := curatedTags[name]
	out := make([]string, 0, len(curated)+1)
	out = append(out, curated...)
	out = append(out, autoTagsFor(previewPath)...)
	return out
}

func autoTagsFor(previewPath string) []string {
	cs := parseColorSet(previewPath, 0)
	if !cs.hasColor() {
		return nil
	}
	r, g, b, ok := hexToRGB(cs.background)
	if !ok {
		return nil
	}
	luminance := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if luminance > 127.5 {
		return []string{"light"}
	}
	return []string{"dark"}
}

func hexToRGB(hex string) (r, g, b int, ok bool) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) < 6 {
		return 0, 0, 0, false
	}
	var ri, gi, bi int64
	var err error
	if ri, err = parseHexByte(hex[0:2]); err != nil {
		return 0, 0, 0, false
	}
	if gi, err = parseHexByte(hex[2:4]); err != nil {
		return 0, 0, 0, false
	}
	if bi, err = parseHexByte(hex[4:6]); err != nil {
		return 0, 0, 0, false
	}
	return int(ri), int(gi), int(bi), true
}

func parseHexByte(s string) (int64, error) {
	return strconv.ParseInt(s, 16, 32)
}

func scriptDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		real = exe
	}
	return filepath.Dir(real), nil
}

func firstCommentLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

var shaderLineRe = regexp.MustCompile(`(?m)^custom-shader\s*=\s*(.+)$`)

func shaderFileFor(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	m := shaderLineRe.FindSubmatch(data)
	if m == nil {
		return ""
	}
	full := strings.TrimSpace(string(m[1]))
	return filepath.Base(full)
}

func ghosttyBinary() string {
	if p, err := exec.LookPath("ghostty"); err == nil {
		return p
	}
	const bundled = "/Applications/Ghostty.app/Contents/MacOS/ghostty"
	if _, err := os.Stat(bundled); err == nil {
		return bundled
	}
	return ""
}

func ghosttyThemesResourceDir() string {
	return "/Applications/Ghostty.app/Contents/Resources/ghostty/themes"
}

// --- Recently-applied items — pinned to the top of the list on next launch ---

const maxRecent = 5

// ghosttyConfigDir mirrors lib/menu.sh's GHOSTTY_DIR default so the recent-
// items file lives next to the config it's describing, and so sandboxed
// testing (GHOSTTY_DIR env override) isolates it the same way it isolates
// everything else this tool touches.
func ghosttyConfigDir() string {
	if d := os.Getenv("GHOSTTY_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ghostty")
}

// studioAssetsDir mirrors lib/menu.sh's STUDIO_ASSETS: imported packs live in
// a user-owned directory, not beside the binary, so a `brew upgrade` replacing
// the install prefix doesn't take them with it.
// studioDir is the user-owned root: imported packs live in assets/ under it,
// and anything the user adds themselves lives beside that, out of reach of
// ghostty-setup's pack removal.
func studioDir() string {
	if d := os.Getenv("GHOSTTY_STUDIO_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ghostty-config-studio")
}

func studioAssetsDir() string {
	return filepath.Join(studioDir(), "assets")
}

func recentFilePath() string {
	return filepath.Join(ghosttyConfigDir(), ".ghostty-tui-recent")
}

func loadRecentKeys() []string {
	data, err := os.ReadFile(recentFilePath())
	if err != nil {
		return nil
	}
	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			keys = append(keys, line)
		}
	}
	return keys
}

// recordRecent moves key to the front of the persisted recent list,
// deduplicates, and caps at maxRecent. Best-effort — a write failure here
// just means next launch doesn't remember this pick, not worth surfacing.
func recordRecent(key string) {
	existing := loadRecentKeys()
	updated := []string{key}
	for _, k := range existing {
		if k != key {
			updated = append(updated, k)
		}
	}
	if len(updated) > maxRecent {
		updated = updated[:maxRecent]
	}
	_ = os.MkdirAll(ghosttyConfigDir(), 0o755)
	_ = os.WriteFile(recentFilePath(), []byte(strings.Join(updated, "\n")+"\n"), 0o644)
}

// applyRecentOrdering moves entries matching recentKeys to the front of
// items, in recency order, marking them .recent for the "★" Title() prefix.
func applyRecentOrdering(items []entry, recentKeys []string) []entry {
	if len(recentKeys) == 0 {
		return items
	}
	byKey := make(map[string]int, len(items))
	for i, it := range items {
		byKey[it.recentKey()] = i
	}
	var front []entry
	used := make(map[string]bool, len(recentKeys))
	for _, k := range recentKeys {
		if idx, ok := byKey[k]; ok && !used[k] {
			e := items[idx]
			e.recent = true
			front = append(front, e)
			used[k] = true
		}
	}
	if len(front) == 0 {
		return items
	}
	var rest []entry
	for _, it := range items {
		if !used[it.recentKey()] {
			rest = append(rest, it)
		}
	}
	return append(front, rest...)
}

// --- Rendering effect preview: show the actual colors, not raw text ---

type colorSet struct {
	background, foreground, cursor, selBg, selFg string
	palette                                      [16]string
	fontFamily, fontSize, shader                 string
	resolvedFrom                                 string // set when colors came from a `theme = X` lookup, not this file directly
}

func (c colorSet) hasColor() bool { return c.background != "" }

var kvLineRe = regexp.MustCompile(`(?m)^([a-zA-Z0-9_-]+)\s*=\s*(.+?)\s*$`)
var paletteEntryRe = regexp.MustCompile(`^(\d+)=(.+)$`)

// parseColorSet reads a Ghostty-format config/theme file. If it references a
// built-in theme via `theme = <Name>` instead of literal hex values, resolve
// one level deeper into Ghostty's own bundled theme resource — this is how
// naydenoff's color-themes/*.conf and presets/*.conf (which just say
// `theme = Nord` etc.) end up with an actual renderable palette.
func parseColorSet(path string, depth int) colorSet {
	var cs colorSet
	if path == "" || depth > 2 {
		return cs
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cs
	}
	themeRef := ""
	for _, m := range kvLineRe.FindAllStringSubmatch(string(data), -1) {
		key, val := m[1], strings.Trim(m[2], `"`)
		switch key {
		case "background":
			cs.background = val
		case "foreground":
			cs.foreground = val
		case "cursor-color":
			cs.cursor = val
		case "selection-background":
			cs.selBg = val
		case "selection-foreground":
			cs.selFg = val
		case "font-family":
			if cs.fontFamily == "" {
				cs.fontFamily = val
			}
		case "font-size":
			cs.fontSize = val
		case "custom-shader":
			cs.shader = filepath.Base(val)
		case "theme":
			themeRef = val
		case "palette":
			if pm := paletteEntryRe.FindStringSubmatch(val); pm != nil {
				var idx int
				fmt.Sscanf(pm[1], "%d", &idx)
				if idx >= 0 && idx < 16 {
					cs.palette[idx] = pm[2]
				}
			}
		}
	}
	if cs.background == "" && themeRef != "" {
		resolved := parseColorSet(filepath.Join(ghosttyThemesResourceDir(), themeRef), depth+1)
		if resolved.hasColor() {
			cs.background, cs.foreground, cs.cursor, cs.selBg, cs.selFg = resolved.background, resolved.foreground, resolved.cursor, resolved.selBg, resolved.selFg
			cs.palette = resolved.palette
			cs.resolvedFrom = themeRef
		}
	}
	return cs
}

// swatchGlyph paints w solid block characters (foreground color, no
// Background()) — deliberately NOT a background-filled padded-text block.
// An earlier version painted a background-colored sample-text panel and it
// intermittently truncated after a handful of visible characters, in both
// headless tmux testing AND a real Ghostty terminal; the palette strip using this
// exact foreground-block technique never once broke across the same
// testing. Block glyphs also read better against the tactical-telemetry
// aesthetic than a decorative fake shell prompt did.
func swatchGlyph(hex string, w int) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render(strings.Repeat("█", w))
}

// cell paints one run of text in the theme's colours. Every segment carries
// its own background, never an outer Render() wrapping several: a styled
// fragment ends with a reset, so a shared outer background leaves the joins
// unpainted — the same bug that once cut a dark bar through the title band.
func cell(bg, fg, text string) string {
	s := lipgloss.NewStyle().Background(lipgloss.Color(bg))
	if fg != "" {
		s = s.Foreground(lipgloss.Color(fg))
	}
	return s.Render(text)
}

// samplePanel paints a few lines of a shell session in the theme's own
// colours, so the pane shows what the theme looks like rather than what it is
// made of. This is the closest a text pane can get to the real thing: `p`
// opens an actual Ghostty for the rest, and a GLSL shader needs a GPU either
// way.
//
// An earlier attempt at this truncated after a handful of characters, which is
// why the swatches below exist at all. The difference now is that every run is
// styled separately and each row is padded to an exact display width, rather
// than one Render() being trusted to measure already-styled content — see
// DESIGN_NOTES, "Colored fills need every segment styled".
func samplePanel(cs colorSet, width int) []string {
	if cs.background == "" || cs.foreground == "" {
		return nil
	}
	w := minInt(maxInt(width-4, 24), 46)

	// Fall back to the foreground for any palette slot the theme left unset,
	// so a partial palette degrades to a readable line rather than to black
	// text on a black background.
	p := func(i int) string {
		if i >= 0 && i < len(cs.palette) && cs.palette[i] != "" {
			return cs.palette[i]
		}
		return cs.foreground
	}
	cur := cs.cursor
	if cur == "" {
		cur = cs.foreground
	}

	// Each row is a list of runs; the renderer pads the remainder in the
	// background colour so the panel is a solid block whatever the widths.
	rows := [][][2]string{
		{{"~/code", p(4)}, {" ❯ ", p(2)}, {"ls", cs.foreground}},
		{{"src", p(4)}, {"  ", ""}, {"README.md", cs.foreground}, {"  ", ""}, {"go.mod", cs.foreground}},
		{{"~/code", p(4)}, {" ❯ ", p(2)}, {"git status", cs.foreground}},
		{{"● main", p(3)}, {"  ", ""}, {"✓ clean", p(2)}},
		{{"~/code", p(4)}, {" ❯ ", p(2)}, {"█", cur}},
	}

	out := make([]string, 0, len(rows)+2)
	blank := cell(cs.background, "", strings.Repeat(" ", w))
	out = append(out, "  "+blank)
	for _, row := range rows {
		line, used := "", 0
		for _, run := range row {
			text, fg := run[0], run[1]
			if lipgloss.Width(text)+used > w-2 {
				break
			}
			line += cell(cs.background, fg, text)
			used += lipgloss.Width(text)
		}
		// The leading and trailing pad are part of the painted block, not
		// gaps in it, so they are drawn in the background colour too.
		out = append(out, "  "+cell(cs.background, "", " ")+line+
			cell(cs.background, "", strings.Repeat(" ", maxInt(w-1-used, 0))))
	}
	out = append(out, "  "+blank)
	return out
}

// renderColorPreview draws the actual rendering effect as labeled color
// swatches: BG/FG/CURSOR blocks, the 16-color palette strip, then metadata.
// GLSL shader effects can't be rendered in a text-mode preview, so that's
// noted rather than faked.
func renderColorPreview(width int, cs colorSet) string {
	if !cs.hasColor() {
		return ""
	}
	labelWidth := 8
	swatchWidth := 10

	row := func(label, hex string) string {
		return fmt.Sprintf("  %s %s  %s",
			labelStyle.Render(padRight(label, labelWidth)),
			swatchGlyph(hex, swatchWidth),
			helpStyle.Render(hex))
	}

	var lines []string
	if sample := samplePanel(cs, width); len(sample) > 0 {
		lines = append(lines, bracket("sample"))
		lines = append(lines, sample...)
		lines = append(lines, "")
	}
	lines = append(lines, bracket("colors"))
	lines = append(lines, "")
	lines = append(lines, row("BG", cs.background))
	if cs.foreground != "" {
		lines = append(lines, row("FG", cs.foreground))
	}
	if cs.cursor != "" {
		lines = append(lines, row("CURSOR", cs.cursor))
	}
	lines = append(lines, "")

	var blocks []string
	for _, hex := range cs.palette {
		if hex == "" {
			continue
		}
		blocks = append(blocks, swatchGlyph(hex, 3))
	}
	if len(blocks) > 0 {
		lines = append(lines, "  "+labelStyle.Render(padRight("PALETTE", labelWidth))+" "+strings.Join(blocks, ""))
		lines = append(lines, "")
	}

	if cs.shader != "" || cs.resolvedFrom != "" {
		lines = append(lines, bracket("meta"))
		lines = append(lines, "")
		if cs.shader != "" {
			lines = append(lines, fmt.Sprintf("  %s %s",
				labelStyle.Render(padRight("SHADER", labelWidth)),
				helpStyle.Render(cs.shader+" (GLSL effect not renderable in a text preview)")))
		}
		if cs.resolvedFrom != "" {
			lines = append(lines, fmt.Sprintf("  %s %s",
				labelStyle.Render(padRight("SOURCE", labelWidth)),
				helpStyle.Render(fmt.Sprintf("resolved from Ghostty's built-in \"%s\" theme", cs.resolvedFrom))))
		}
	}

	_ = width
	return strings.Join(lines, "\n")
}

// padRight right-pads s to w runes with plain spaces — used instead of
// lipgloss's Style.Width() reflow, which (in some terminal environments,
// including a real Ghostty terminal, confirmed independently of headless
// tmux testing) mis-measured already-styled short strings. Pre-padding
// plain unstyled labels ourselves, then styling the whole fixed-width
// result, sidesteps that wrap engine entirely.
// padRight pads to w DISPLAY columns, not runes. A CJK glyph occupies two
// cells, so counting runes left every Chinese label short by its own length
// and the column after it ragged.
func padRight(s string, w int) string {
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// padVertical appends blank lines until s has at least target lines.
func padVertical(s string, target int) string {
	lines := strings.Count(s, "\n") + 1
	if lines >= target {
		return s
	}
	return s + strings.Repeat("\n", target-lines)
}

func buildEntries(dir string) []entry {
	var items []entry

	// 12 shader themes (snedea/ghostty-themes)
	shaderFiles, _ := filepath.Glob(filepath.Join(studioAssetsDir(), "shader-themes/config.*"))
	for _, f := range shaderFiles {
		name := strings.TrimPrefix(filepath.Base(f), "config.")
		desc := shaderDescOverride[name]
		if desc == "" {
			desc = firstCommentLine(f)
		}
		shaderSrc := ""
		if sf := shaderFileFor(f); sf != "" {
			shaderSrc = filepath.Join(studioAssetsDir(), "shader-themes/shaders", sf)
		}
		items = append(items, entry{
			source: "snedea", name: name, desc: desc, category: "theme",
			kind: "file", value: f, shaderSrc: shaderSrc, previewPath: f, tags: tagsFor(name, f),
		})
	}

	// 6 color themes (naydenoff/ghostty-config)
	colorFiles, _ := filepath.Glob(filepath.Join(studioAssetsDir(), "color-themes/*.conf"))
	for _, f := range colorFiles {
		name := strings.TrimSuffix(filepath.Base(f), ".conf")
		desc := colorDesc[name]
		if desc == "" {
			desc = name
		}
		items = append(items, entry{
			source: "naydenoff", name: name, desc: desc, category: "theme",
			kind: "file", value: f, previewPath: f, tags: tagsFor(name, f),
		})
	}

	// 4 fonts (naydenoff/ghostty-config)
	fontFiles, _ := filepath.Glob(filepath.Join(studioAssetsDir(), "fonts/*.conf"))
	for _, f := range fontFiles {
		name := strings.TrimSuffix(filepath.Base(f), ".conf")
		desc := fontDesc[name]
		if desc == "" {
			desc = name
		}
		items = append(items, entry{
			source: "naydenoff", name: name, desc: desc, category: "font",
			kind: "file", value: f, previewPath: f, tags: tagsFor(name, f),
		})
	}

	// Your own font configs, from a directory the packs never touch.
	// See ghostty-font for why they do not live under assets/.
	ownFonts, _ := filepath.Glob(filepath.Join(studioDir(), "fonts/*.conf"))
	for _, f := range ownFonts {
		name := strings.TrimSuffix(filepath.Base(f), ".conf")
		desc := firstCommentLine(f)
		if desc == "" {
			desc = tr("你自己的字型設定", "your own font config")
		}
		items = append(items, entry{
			source: "you", name: name, desc: desc, category: "font",
			kind: "file", value: f, previewPath: f,
		})
	}

	// 5 presets (naydenoff/ghostty-config)
	presetFiles, _ := filepath.Glob(filepath.Join(studioAssetsDir(), "presets/*.conf"))
	for _, f := range presetFiles {
		name := strings.TrimSuffix(filepath.Base(f), ".conf")
		desc := presetDesc[name]
		if desc == "" {
			desc = name
		}
		items = append(items, entry{
			source: "naydenoff", name: name, desc: desc, category: "preset",
			kind: "file", value: f, previewPath: f, tags: tagsFor(name, f),
		})
	}

	// 7 cursor-effect shaders (sahaj-b/ghostty-cursor-shaders). Distinct
	// from the background shader themes above: applied via a direct
	// `custom-shader = <path>` line (kind=shader), not a config-file
	// include, and Ghostty stacks multiple custom-shader directives so
	// this coexists with a background theme rather than replacing it.
	cursorFiles, _ := filepath.Glob(filepath.Join(studioAssetsDir(), "cursor-shaders/*.glsl"))
	for _, f := range cursorFiles {
		name := strings.TrimSuffix(filepath.Base(f), ".glsl")
		desc := cursorShaderDesc[name]
		if desc == "" {
			desc = name
		}
		items = append(items, entry{
			source: "sahaj-b", name: name, desc: desc, category: "cursor",
			kind: "shader", value: f, previewPath: f, tags: tagsFor(name, f),
		})
	}

	// Raw scalar settings — Phase 1 of the "full settings editor" discussed
	// curated value-picker entries for a few high-value Ghostty
	// options, using the same architecture as everything else (kind="raw"
	// writes "<settingKey> = <value>" directly). NOT window-padding-x/y
	// (needs two directive lines from one pick, not built yet) and NOT a
	// free-form "type any value" editor (Phase 2, separate, larger effort).
	//
	// Known limitation, found via testing: a shader theme (snedea) that
	// bakes in its own background-opacity/background-blur/cursor-style/
	// cursor-style-blink internally can override these independent picks
	// through Ghostty's config-file precedence, inconsistently per key
	// (verified cursor-style respects the override, opacity/blur/blink did
	// not, in the same combined test). Works correctly and predictably
	// with fonts/color-themes/built-in themes/presets — just not
	// guaranteed alongside one of the 12 shader themes.
	rawSettings := []entry{
		{source: "", name: "opacity-100", desc: "Opacity 100% -- fully opaque", category: "opacity", kind: "raw", value: "1", settingKey: "background-opacity"},
		{source: "", name: "opacity-95", desc: "Opacity 95%", category: "opacity", kind: "raw", value: "0.95", settingKey: "background-opacity"},
		{source: "", name: "opacity-90", desc: "Opacity 90%", category: "opacity", kind: "raw", value: "0.90", settingKey: "background-opacity"},
		{source: "", name: "opacity-85", desc: "Opacity 85%", category: "opacity", kind: "raw", value: "0.85", settingKey: "background-opacity"},
		{source: "", name: "opacity-80", desc: "Opacity 80% -- quite see-through", category: "opacity", kind: "raw", value: "0.80", settingKey: "background-opacity"},
		{source: "", name: "blur-off", desc: "Blur off", category: "blur", kind: "raw", value: "false", settingKey: "background-blur"},
		{source: "", name: "blur-light", desc: "Blur light (intensity 10)", category: "blur", kind: "raw", value: "10", settingKey: "background-blur"},
		{source: "", name: "blur-medium", desc: "Blur medium (intensity 20, Ghostty default when on)", category: "blur", kind: "raw", value: "20", settingKey: "background-blur"},
		{source: "", name: "blur-heavy", desc: "Blur heavy (intensity 40)", category: "blur", kind: "raw", value: "40", settingKey: "background-blur"},
		{source: "", name: "style-block", desc: "Block cursor -- Ghostty default", category: "cursor-style", kind: "raw", value: "block", settingKey: "cursor-style"},
		{source: "", name: "style-bar", desc: "Bar cursor -- thin vertical line", category: "cursor-style", kind: "raw", value: "bar", settingKey: "cursor-style"},
		{source: "", name: "style-underline", desc: "Underline cursor", category: "cursor-style", kind: "raw", value: "underline", settingKey: "cursor-style"},
		{source: "", name: "style-block-hollow", desc: "Hollow block cursor (outline only)", category: "cursor-style", kind: "raw", value: "block_hollow", settingKey: "cursor-style"},
		{source: "", name: "blink-on", desc: "Blinking cursor -- Ghostty default", category: "cursor-blink", kind: "raw", value: "true", settingKey: "cursor-style-blink"},
		{source: "", name: "blink-off", desc: "Solid, non-blinking cursor", category: "cursor-blink", kind: "raw", value: "false", settingKey: "cursor-style-blink"},
		{source: "", name: "select-on", desc: "Copy on select -- Ghostty default on macOS", category: "copy-on-select", kind: "raw", value: "true", settingKey: "copy-on-select"},
		{source: "", name: "select-off", desc: "Selecting text does NOT copy it automatically", category: "copy-on-select", kind: "raw", value: "false", settingKey: "copy-on-select"},
		{source: "", name: "trim-on", desc: "Trim trailing spaces on copy -- Ghostty default", category: "clipboard-trim", kind: "raw", value: "true", settingKey: "clipboard-trim-trailing-spaces"},
		{source: "", name: "trim-off", desc: "Keep trailing spaces as-is when copying", category: "clipboard-trim", kind: "raw", value: "false", settingKey: "clipboard-trim-trailing-spaces"},
	}
	for _, e := range rawSettings {
		e.source = "ghostty-settings"
		items = append(items, e)
	}

	// Your own saved custom presets — the "workbench" output. See
	// saveCurrentAsCustom(); applying one is category=custom (a solo
	// standalone combo, like preset), listed here purely by scanning the
	// directory, so anything saved (in this session or a past one) shows
	// up automatically with no separate registration step.
	customFiles, _ := filepath.Glob(filepath.Join(customPresetDir(), "*.conf"))
	for _, f := range customFiles {
		name := strings.TrimSuffix(filepath.Base(f), ".conf")
		items = append(items, entry{
			source: "you", name: name, desc: "你儲存的自訂 preset", category: "custom",
			kind: "file", value: f, previewPath: f,
		})
	}

	// 460+ built-in Ghostty themes — zero vendoring, native `theme = <name>`.
	if bin := ghosttyBinary(); bin != "" {
		out, err := exec.Command(bin, "+list-themes").Output()
		if err == nil {
			resourceDir := ghosttyThemesResourceDir()
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				idx := strings.LastIndex(line, " (")
				name := line
				if idx > 0 {
					name = line[:idx]
				}
				preview := filepath.Join(resourceDir, name)
				if _, err := os.Stat(preview); err != nil {
					preview = ""
				}
				items = append(items, entry{
					source: "ghostty", name: name, desc: txtBuiltinTheme(), category: "theme",
					kind: "name", value: name, previewPath: preview, tags: autoTagsFor(preview),
				})
			}
		}
	}

	return items
}

// allCategories lists every category run_picker/apply_selection can write —
// used so the "●" current-selection marker works for all of them. Note:
// this was previously just {theme, font, preset}, silently missing cursor
// and custom (their "●" marker never worked) — caught while wiring in the
// new raw-setting categories below.
var allCategories = []string{
	"theme", "font", "preset", "cursor", "custom",
	"opacity", "blur", "cursor-style", "cursor-blink", "copy-on-select", "clipboard-trim",
}

func currentSelections(dir string) map[string]string {
	result := map[string]string{}
	for _, cat := range allCategories {
		out, err := exec.Command("bash", "-c",
			fmt.Sprintf(`source %q; current_path_for %q`, filepath.Join(dir, "lib/menu.sh"), cat)).Output()
		if err == nil {
			v := strings.TrimSpace(string(out))
			if v != "" {
				result[cat] = v
			}
		}
	}
	return result
}

// firstWarningLine picks the ⚠ line out of apply_selection's output. Only the
// first line: the rest is the explanation, which is too long for a status bar
// and is printed in full by the command-line tools.
func firstWarningLine(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "⚠") {
			return t
		}
	}
	return ""
}

func applyEntry(dir string, e entry) (string, error) {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`source %q; apply_selection "$1" "$2" "$3" "$4" "$5"`, filepath.Join(dir, "lib/menu.sh")),
		"--", e.category, e.value, e.kind, e.shaderSrc, e.settingKey)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// firstLine keeps a message to the one row the status bar has. The bash side
// explains itself over several lines; the rest is for the command-line tools.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// --- Live preview: render an entry in a throwaway Ghostty window ---

// previewConfig asks lib/menu.sh what this entry's directive lines look like,
// for the same reason applyEntry asks it to write them: one implementation of
// the config write, so the CLI and the TUI cannot drift. preview_directive
// prints to stdout and never touches ~/.config/ghostty/config.
func previewConfig(dir string, e entry) (string, error) {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`source %q; preview_directive "$1" "$2" "$3" "$4"`, filepath.Join(dir, "lib/menu.sh")),
		"--", e.value, e.kind, e.settingKey, e.shaderSrc)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := firstLine(strings.TrimSpace(stderr.String()))
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("%s", detail)
	}
	return stdout.String(), nil
}

// previewEntry opens a throwaway Ghostty window showing the entry for real —
// the only way to see one of the 12 GLSL shader themes before committing to
// it, since a text pane can only ever say the effect exists.
//
// `--config-default-files=false` is a CLI-only Ghostty option that stops it
// loading any of the user's config files, so the window shows the candidate
// alone. That also sidesteps the macOS "Application Support config overrides
// you" problem that warn_if_shadowed exists for: the file that would win
// isn't read at all here. On macOS the emulator cannot be started through the
// `ghostty` binary (`ghostty +help` says so) — `open -na` is the supported
// path, and it finds the app wherever LaunchServices has it registered rather
// than only under /Applications.
//
// Read-only with respect to the user's config by construction: nothing is
// written but the temp file, and the launched process is told to read
// nothing else.
// previewMarker appears in both the temp config's name and therefore in the
// preview process's argv, which is how the previous preview is found again.
// Matching on it cannot reach a Ghostty the user started themselves: their
// command line has no reason to contain this string.
const previewMarker = "ghostty-studio-preview-"

// closePreviousPreview ends the preview opened by the last `p`, so pressing it
// down a list leaves one preview surface rather than a window per entry.
// Signalled rather than closed through the UI: the surface has a shell running
// in it, which is exactly when Ghostty asks "close this?", and a throwaway
// preview should never ask.
func closePreviousPreview() {
	_ = exec.Command("pkill", "-f",
		"Ghostty.app/Contents/MacOS/ghostty.*"+previewMarker).Run()
}

// dockRight returns config lines placing the window on the right half of the
// screen, so the preview sits beside the workbench instead of on top of it.
//
// This is where a split would have gone. Ghostty cannot do it: `new_split`
// is a keybind action taking only a direction, there is no CLI action that
// creates one (`+new-window` answers "not supported on this platform"), and a
// split inherits the config of the app instance that owns it — there is no
// per-surface config file. A split would therefore render in the theme you are
// trying to replace, which is the one thing a preview must not do. A second
// instance is the only surface that can carry its own config, so it is placed
// where the split would have been.
//
// Only the position is set. window-width/height are counted in grid cells and
// the cell size depends on the font the previewed config chooses, so a cell
// count computed here would be wrong for exactly the entries most worth
// previewing. Ghostty's own default size, placed correctly, beats a guess.
func dockRight() string {
	out, err := exec.Command("osascript", "-e",
		"tell application \"Finder\" to get bounds of window of desktop").Output()
	if err != nil {
		return ""
	}
	// "0, 0, 2048, 1332" — the third field is the screen width.
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) != 4 {
		return ""
	}
	w, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil || w <= 0 {
		return ""
	}
	return fmt.Sprintf("window-position-x = %d\nwindow-position-y = 0\n", w/2)
}

func previewEntry(dir string, e entry) error {
	body, err := previewConfig(dir, e)
	if err != nil {
		return err
	}
	// A preview is throwaway by definition: it should not argue about being
	// closed, and it should take the whole instance with it when it goes.
	body += "\nconfirm-close-surface = false\nquit-after-last-window-closed = true\n"
	body += "title = " + previewTitle(e.source+"/"+e.name) + "\n"
	body += dockRight()

	f, err := os.CreateTemp("", previewMarker+"*.conf")
	if err != nil {
		return err
	}
	_, writeErr := f.WriteString(body)
	closeErr := f.Close()
	// Ghostty reads the config once, at startup, and never looks at the file
	// again — but `open -na` returns as soon as the launch has been
	// *requested*, so deleting on the way out of this function would race
	// that read. A detached sleep-then-delete (same fire-and-forget shape as
	// restartGhostty) is the one cleanup that neither races the read nor
	// depends on how this process ends: a minute is orders of magnitude more
	// than a cold start, and it still runs if the TUI is killed meanwhile.
	defer scheduleDelete(f.Name())
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}

	closePreviousPreview()
	out, err := exec.Command("open", "-na", "Ghostty.app", "--args",
		"--config-default-files=false", "--config-file="+f.Name()).CombinedOutput()
	if err != nil {
		detail := firstLine(strings.TrimSpace(string(out)))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
}

func scheduleDelete(path string) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("sleep 60; rm -f %q", path))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = cmd.Start()
}

// undoLastApply walks the managed block back one snapshot. The bash side owns
// both the snapshots and the wording of the result, so its summary line is
// shown verbatim rather than reworded here.
func undoLastApply(dir string) (string, error) {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`source %q; undo_last_apply`, filepath.Join(dir, "lib/menu.sh")))
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := firstLine(strings.TrimSpace(stderr.String()))
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("%s", detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// --- Shadow conflicts: the macOS config that outranks the managed block ---
//
// apply_selection already detects that ~/Library/Application Support/
// com.mitchellh.ghostty/config is read after ~/.config/ghostty/config and so
// beats every key it sets. Detecting it and stopping there was a dead end: the
// warning told the user to go edit a file it never gave a path for. lib/menu.sh
// still owns every byte written — these two calls only list and act.

type shadowConflict struct {
	file string
	line string // kept as text: it is only ever displayed, and a non-numeric
	// field from a drifting format should still show rather than drop the row
	key  string
	text string
}

// parseShadowRecords reads list_shadow_conflicts' stdout, and is the only
// place that knows its format: one record per line, four fields separated by
// ASCII 0x1F (unit separator), in the order file, line number, key, and the
// whole offending line verbatim.
//
// 0x1F rather than a tab because every printable candidate can legally occur
// inside a Ghostty path or value — spaces, tabs, `=`, `#`, `|`, `:` — and a
// control character cannot. The verbatim line comes last so that even a value
// nobody anticipated cannot be read as a separator.
//
// Unparseable input is skipped, never surfaced as an error: this panel is what
// the user reaches for when their config is already misbehaving, and a format
// drift should cost them one row rather than the screen that explains the
// problem.
func parseShadowRecords(out string) []shadowConflict {
	var found []shadowConflict
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.SplitN(line, "\x1f", 4)
		if len(f) < 4 {
			continue
		}
		c := shadowConflict{
			file: strings.TrimSpace(f[0]),
			line: strings.TrimSpace(f[1]),
			key:  strings.TrimSpace(f[2]),
			text: strings.TrimSpace(f[3]),
		}
		// A record naming neither a file nor a key describes nothing that can
		// be shown or fixed.
		if c.file == "" || c.key == "" {
			continue
		}
		found = append(found, c)
	}
	return found
}

func listShadowConflicts(dir string) ([]shadowConflict, error) {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`source %q; list_shadow_conflicts`, filepath.Join(dir, "lib/menu.sh")))
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := firstLine(strings.TrimSpace(stderr.String()))
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s", detail)
	}
	return parseShadowRecords(stdout.String()), nil
}

// resolveShadowConflicts comments the offending lines out. Same shape as
// undoLastApply: the bash side owns the backup, the write and the wording, so
// its one-line summary is shown verbatim rather than reworded here.
func resolveShadowConflicts(dir string) (string, error) {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`source %q; resolve_shadow_conflicts`, filepath.Join(dir, "lib/menu.sh")))
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := firstLine(strings.TrimSpace(stderr.String()))
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("%s", detail)
	}
	return firstLine(strings.TrimSpace(stdout.String())), nil
}

// conflictFileCounts groups records by file, keeping first-seen order so the
// confirmation names every file it is about to touch and how much of each.
// Ghostty reads two candidate paths, so more than one is possible.
func conflictFileCounts(cs []shadowConflict) ([]string, map[string]int) {
	var order []string
	counts := map[string]int{}
	for _, c := range cs {
		if _, seen := counts[c.file]; !seen {
			order = append(order, c.file)
		}
		counts[c.file]++
	}
	return order, counts
}

// customPresetDir is where saveCurrentAsCustom writes, and where
// buildEntries scans for the "custom" category — the on-disk half of the
// TUI-driven workbench (bash side: `ghostty-custom --save <name>`).
func customPresetDir() string {
	return filepath.Join(ghosttyConfigDir(), "ghostty-tui-custom")
}

// saveCurrentAsCustom shells out to lib/menu.sh's save_current_as, same
// reasoning as applyEntry: one implementation of "what does a snapshot of
// the current managed block look like," shared with the bash ghostty-custom
// command, not duplicated in Go.
func saveCurrentAsCustom(dir, name string) (string, error) {
	dest := filepath.Join(customPresetDir(), name+".conf")
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`mkdir -p %q; source %q; save_current_as "$1"`, customPresetDir(), filepath.Join(dir, "lib/menu.sh")),
		"--", dest)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// --- Field editor: create/edit a custom config as key=value rows ---
//
// Deliberately flat and comment-free on save — this is a generated custom
// preset, not a hand-authored file meant to be read later, so round-
// tripping comments/blank-line layout isn't worth the complexity. Vendored
// assets are never edited in place (see the 'e' key handler) — only files
// already under customPresetDir() get this treatment, so the "vendored
// files stay byte-identical to upstream" invariant kept all session never
// has to be reconsidered here.

type fieldRow struct{ key, value string }

func parseFieldRows(path string) []fieldRow {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var rows []fieldRow
	for _, m := range kvLineRe.FindAllStringSubmatch(string(data), -1) {
		rows = append(rows, fieldRow{key: m[1], value: strings.Trim(m[2], `"`)})
	}
	return rows
}

func writeFieldRows(path string, rows []fieldRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s = %s\n", r.key, r.value)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// restartGhostty quits and relaunches the Ghostty app — some settings
// (notably custom-shader) don't reliably pick up via reload_config alone.
// This process (ghostty-tui) is itself running inside a Ghostty window, so
// quitting Ghostty kills us too; the restart sequence runs detached
// (Setsid, stdio to /dev/null) so it survives after we call tea.Quit and
// exit, and it sleeps briefly before/after the quit so the app has time to
// actually shut down before "open" tries to relaunch it. Fire-and-forget —
// errors here have nowhere left to report to once we've quit.
func restartGhostty() {
	script := `sleep 1; osascript -e 'quit app "Ghostty"' >/dev/null 2>&1; sleep 1; open -a Ghostty`
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = cmd.Start()
}

// --- Bubble Tea model ---
//
// Visual language: tactical-telemetry terminal — near-black ground, phosphor
// white body text, square (not rounded) frames, ASCII-bracket panel labels,
// uppercase monospace metadata. Red is reserved for errors only; green is
// reserved for exactly one thing (the current-selection marker) — per the
// "one accent, used with intent" rule, not sprinkled as decoration.
var (
	phosphor    = lipgloss.Color("#EAEAEA")
	dim         = lipgloss.Color("#6C6C6C")
	accentGreen = lipgloss.Color("#4AF626")
	accentRed   = lipgloss.Color("#FF5A5A")
	frameColor  = lipgloss.Color("#4A4A4A")

	// Category-tab chrome: orange ground, white lettering, with a darker
	// cell to the right standing in for a drop shadow (a single text row
	// can't offset a shadow downward, so depth is suggested horizontally).
	// Active vs inactive is carried by brightness of the same orange —
	// with every tab coloured, brightness is the remaining affordance.
	tabActiveBg   = lipgloss.Color("#FF8C21")
	tabInactiveBg = lipgloss.Color("#8A4A12")
	tabShadowBg   = lipgloss.Color("#2A1606")

	// Title-band fill. Slightly deeper than the active tab so the band
	// reads as a surface behind the tabs rather than a giant tab itself.
	bannerBg = lipgloss.Color("#E07818")
	// On an orange ground, the usual dim grey secondary text is unreadable;
	// dark brown carries the same "secondary" weight with real contrast.
	bannerFg    = lipgloss.Color("#FFFFFF")
	bannerDimFg = lipgloss.Color("#3A1D05")

	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(phosphor)
	phosphorStyle  = lipgloss.NewStyle().Foreground(phosphor)
	tabActiveStyle = lipgloss.NewStyle().Background(tabActiveBg).
			Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	tabInactiveStyle = lipgloss.NewStyle().Background(tabInactiveBg).
				Foreground(lipgloss.Color("#FFFFFF"))
	tabShadowStyle = lipgloss.NewStyle().Background(tabShadowBg)
	// Text styles for the orange title band. Background is set here too so
	// the fill is unbroken across styled fragments.
	bannerTitleStyle = lipgloss.NewStyle().Background(bannerBg).
				Foreground(bannerFg).Bold(true)
	bannerDimStyle = lipgloss.NewStyle().Background(bannerBg).Foreground(bannerDimFg)
	labelStyle     = lipgloss.NewStyle().Bold(true).Foreground(dim)
	statusStyle    = lipgloss.NewStyle().Foreground(accentGreen)
	errorStyle     = lipgloss.NewStyle().Foreground(accentRed)
	helpStyle      = lipgloss.NewStyle().Foreground(dim)
	currentMarker  = lipgloss.NewStyle().Foreground(accentGreen).Bold(true).Render("● ")
)

// bracket frames a panel title the way the tactical-telemetry style marks
// section headers: "[ THEMES ]" rather than plain text.
func bracket(s string) string {
	return titleStyle.Render("[ " + strings.ToUpper(s) + " ]")
}

// box draws a plain square-cornered frame around content by hand — NOT via
// lipgloss's Style.Border()+Width()+Height() chain. That chain measured
// already-ANSI-styled multi-line content unreliably in testing (both a
// mid-line width truncation and, separately, a height overcount that added
// phantom extra rows — see DESIGN_NOTES.md, "Rendering").
// Manual drawing uses lipgloss.Width() (ANSI-aware) for per-line padding
// and never re-wraps already-styled text in another Width()/Height() call.
// bannerBlock draws the screen's title field as a solid orange band with a
// drop shadow, instead of an outlined box: with the interior filled there's
// nothing left for a border to do. The content row is padded with a blank
// row above and below, all three sharing the fill.
//
// Total rendered size is width × 4 — the shadow occupies one column on the
// right and one row underneath (offset right by one, the way a light source
// above-left would cast it), so callers pass the full width they want the
// block to occupy.
func bannerBlock(left, right string, width int) string {
	inner := width - 1 // last column belongs to the shadow
	fill := lipgloss.NewStyle().Background(bannerBg)
	shade := lipgloss.NewStyle().Background(tabShadowBg)

	// Every segment carries the fill itself. `left` and `right` arrive
	// already styled, and a styled fragment ends with a reset — so wrapping
	// the concatenation in one outer Render leaves the spaces *between* the
	// fragments unstyled, which showed up as a dark bar cutting through the
	// middle of the band.
	// 1 column of lead-in, 3 columns of gutter before the right edge.
	const leadIn, gutter = 1, 3
	gap := inner - leadIn - gutter - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	body := fill.Render(strings.Repeat(" ", leadIn)) + left +
		fill.Render(strings.Repeat(" ", gap)) + right +
		fill.Render(strings.Repeat(" ", gutter))
	if short := inner - lipgloss.Width(body); short > 0 {
		body += fill.Render(strings.Repeat(" ", short))
	}

	blank := fill.Render(strings.Repeat(" ", inner))
	rows := []string{
		blank + shade.Render(" "),
		body + shade.Render(" "),
		blank + shade.Render(" "),
		" " + shade.Render(strings.Repeat(" ", inner)),
	}
	return strings.Join(rows, "\n")
}

func box(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	lines = lines[:height]

	frame := lipgloss.NewStyle().Foreground(frameColor)
	vbar := frame.Render("│")
	horiz := strings.Repeat("─", width+2)

	out := make([]string, 0, height+2)
	out = append(out, frame.Render("┌"+horiz+"┐"))
	for _, l := range lines {
		// Truncate before padding. bubbles/list renders two columns wider
		// once its filter prompt is up than the size it was given, and a
		// line longer than the budget used to shove the right-hand frame
		// out with it, so the border stepped sideways mid-card. ansi.Truncate
		// counts display cells and leaves escape sequences intact, which a
		// rune-slice cut would not.
		l = ansi.Truncate(l, width, "")
		visible := lipgloss.Width(l)
		pad := width - visible
		if pad < 0 {
			pad = 0
		}
		out = append(out, vbar+" "+l+strings.Repeat(" ", pad)+" "+vbar)
	}
	out = append(out, frame.Render("└"+horiz+"┘"))
	return strings.Join(out, "\n")
}

type model struct {
	dir            string
	list           list.Model
	previewContent string
	// listWidth/previewWidth/bodyHeight are the single source of truth for
	// both panel boxes' dimensions — View() must read these, never
	// m.list.Width()/Height(), which can drift from what SetSize() was
	// actually given (bubbles/list applies its own internal adjustments),
	// producing two boxes of different heights side by side.
	listWidth         int
	previewWidth      int
	bodyHeight        int
	current           map[string]string
	status            string
	statusOK          bool
	width             int
	height            int
	lastPreviewKey    string // identity of the entry last rendered into preview
	showRestartPrompt bool
	// restartHint is set instead of showRestartPrompt when the apply
	// succeeded but another config may override the keys: the warning stays
	// on screen, and `y` opens the restart dialog on demand.
	restartHint bool

	// Tag filtering, opened with `t`. allEntries is the unfiltered catalog;
	// the list itself only ever holds what the active tags leave, so
	// bubbles/list's own `/` search composes on top rather than competing.
	allEntries   []entry
	activeTags   map[string]bool
	showTagPanel bool
	tagCursor    int

	// Name-prompt dialog — shared by two purposes (namePurpose tells
	// Enter's handler which): "save-current" snapshots the active managed
	// block (existing `s` behavior); "new-file" creates a blank custom
	// config and immediately opens it in the field editor below.
	showNameDialog bool
	namePurpose    string
	nameInput      textinput.Model

	// Settings editor — the category/checkbox workbench. nil when closed.
	editor *editorState

	// "Which config do I want to edit?" picker, shown by `e`. Lists only
	// editable (custom) configs.
	showEditPicker bool
	editable       []entry
	editPickIndex  int

	// Delete confirmation, raised from the picker with `d`. Deleting a
	// file is irreversible, so it never happens on a single keypress.
	showDeleteConfirm bool
	deleteTarget      entry

	// Shadow-conflict panel, opened with `c`. conflictsErr holds the reason
	// the list could not be read at all, which is a different thing from an
	// empty list and has to read differently on screen.
	showConflicts       bool
	conflicts           []shadowConflict
	conflictsErr        string
	showConflictConfirm bool
}

func newModel(dir string) model {
	loadLang()
	entries := applyRecentOrdering(buildEntries(dir), loadRecentKeys())
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = e
	}
	delegate := list.NewDefaultDelegate()
	// Override the delegate's default AdaptiveColor styles with our fixed
	// tactical-telemetry palette — one less source of terminal-dependent
	// variance, and keeps the item list visually consistent with the rest
	// of the UI instead of bubbles' default purple/pink accents.
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(accentGreen).BorderForeground(accentGreen)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(accentGreen).BorderForeground(accentGreen)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(phosphor)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(dim)

	l := list.New(items, delegate, 0, 0)
	// The product name now lives in the screen's boxed title block, and the
	// pane names itself on its first row — so the list's own title would
	// just be a third copy of the same words.
	l.SetShowTitle(false)
	l.SetShowStatusBar(true) // kept: this is what reports filter state
	l.SetStatusBarItemName(txtStatusItemName())
	l.FilterInput.Prompt = txtFilterPrompt()
	// The screen already has its own footer; the list's built-in help line
	// would repeat the same keys a second time inside the pane.
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = helpStyle
	l.Styles.HelpStyle = helpStyle

	ti := textinput.New()
	ti.Placeholder = "my-preset-name"
	ti.CharLimit = 64
	ti.Width = 40

	return model{
		dir:        dir,
		list:       l,
		allEntries: entries,
		current:    currentSelections(dir),
		nameInput:  ti,
		activeTags: map[string]bool{},
	}
}

func (m model) Init() tea.Cmd { return nil }

// updatePreviewForSelection renders directly into a plain string rather than
// a bubbles/viewport.Model — none of our config previews are long enough to
// need scrolling, and viewport's line-wrapping mis-measured already-ANSI-
// styled content (colored blocks got cut after a handful of visible
// characters), so it added a bug class with no matching benefit here.
func (m *model) updatePreviewForSelection() {
	item, ok := m.list.SelectedItem().(entry)
	if !ok {
		m.previewContent = ""
		return
	}
	if item.previewPath == "" {
		if item.kind == "raw" {
			m.previewContent = helpStyle.Render(fmt.Sprintf("%s = %s", item.settingKey, item.value))
			return
		}
		m.previewContent = helpStyle.Render(txtNoPreview())
		return
	}

	cs := parseColorSet(item.previewPath, 0)
	if cs.hasColor() {
		m.previewContent = renderColorPreview(m.previewWidth, cs)
		return
	}

	// No background/palette to render (font-only entries) — raw text is the
	// only meaningful preview here.
	data, err := os.ReadFile(item.previewPath)
	if err != nil {
		m.previewContent = errorStyle.Render("couldn't read: " + item.previewPath)
		return
	}
	// Wrap to the pane width — GLSL sources and commented config files have
	// lines far wider than the pane, and box() pads but never wraps, so an
	// over-long line runs straight through the right border.
	//
	// Only the raw-text path wraps. The colour-swatch path above builds its
	// own short, already-ANSI-styled lines; running those through a
	// rune-wise wrapper risks splitting an escape sequence in half.
	w := maxInt(m.previewWidth, 20)
	var wrapped []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		wrapped = append(wrapped, wrapText(line, w)...)
	}
	m.previewContent = strings.Join(wrapped, "\n")
}

// selectionKey identifies the currently selected entry so we can detect a
// change regardless of *why* it changed — direct navigation, or a filter
// match recomputed asynchronously by the list component a message later.
func (m model) selectionKey() string {
	item, ok := m.list.SelectedItem().(entry)
	if !ok {
		return ""
	}
	return item.category + "|" + item.kind + "|" + item.value
}

// refreshEntries re-scans everything (including the custom-preset
// directory) and replaces the list's items — used after any change that
// could add/remove a custom preset, so it shows up without restarting.
func (m model) refreshEntries() model {
	m.allEntries = applyRecentOrdering(buildEntries(m.dir), loadRecentKeys())
	return m.applyTagFilter()
}

// applyTagFilter narrows the list to the active tags. Kept separate from
// refreshEntries because toggling a tag must not re-scan the disk.
func (m model) applyTagFilter() model {
	entries := filterByTags(m.allEntries, m.activeTags)
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = e
	}
	m.list.SetItems(items)
	return m
}

func (m model) saveCurrentCombo(name string) (tea.Model, tea.Cmd) {
	out, err := saveCurrentAsCustom(m.dir, name)
	if err != nil {
		detail := strings.TrimSpace(out)
		if detail == "" {
			detail = err.Error()
		}
		m.status = "failed to save preset " + name + ": " + detail
		m.statusOK = false
		return m, nil
	}
	m.status = txtSaved(name, 0)
	m.statusOK = true
	m = m.refreshEntries()
	return m, nil
}

// startNewFile creates a blank custom config and drops straight into the
// field editor on it — the "add a config file directly in the TUI" half
// of the workbench. Empty on creation; fields are added one at a
// time via 'a' inside the editor. Deleted on exit if still empty (see
// updateEditor's esc handling) so cancelling doesn't litter the
// custom-preset directory with junk files.
func (m model) startNewFile(name string) (tea.Model, tea.Cmd) {
	path := filepath.Join(customPresetDir(), name+".conf")
	if _, err := os.Stat(path); err == nil {
		m.status = txtAlreadyExists(name)
		m.statusOK = false
		return m, nil
	}
	if err := os.MkdirAll(customPresetDir(), 0o755); err != nil {
		m.status = "failed to create " + name + ": " + err.Error()
		m.statusOK = false
		return m, nil
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		m.status = "failed to create " + name + ": " + err.Error()
		m.statusOK = false
		return m, nil
	}
	m.editor = newEditorState(path, name, nil, true)
	return m, nil
}

// openFieldEditor is the "edit the content of a config file, field by
// field" half of the workbench. Vendored assets stay read-only here on
// purpose — this session's whole vendoring story depends on those files
// staying byte-identical to upstream (see NOTICE.md); only files already
// under customPresetDir() (category "custom") are editable in place.
func (m model) openFieldEditor(item entry) (tea.Model, tea.Cmd) {
	if item.category != "custom" || item.kind != "file" {
		m.status = txtReadOnly()
		m.statusOK = false
		return m, nil
	}
	m.editor = newEditorState(item.value, item.name, parseFieldRows(item.value), false)
	return m, nil
}

// appliedCategoriesFor reports which managed-block categories currently
// point at this file. Used before deleting it — a leftover
// `config-file = <deleted path>` line makes Ghostty fail to open the
// include and fall back to defaults for the *whole* config, which is the
// silent-breakage mode documented in DESIGN_NOTES.md.
func (m model) appliedCategoriesFor(e entry) []string {
	var cats []string
	for cat, v := range m.current {
		if v == e.value {
			cats = append(cats, cat)
		}
	}
	sort.Strings(cats) // map order is random; keep the message stable
	return cats
}

// deleteCustomConfig removes a saved config file, first clearing any
// managed-block reference to it so the user's Ghostty config can't be left
// pointing at a file that no longer exists.
func (m model) deleteCustomConfig(e entry) (tea.Model, tea.Cmd) {
	m.showDeleteConfirm = false
	applied := m.appliedCategoriesFor(e)
	for _, cat := range applied {
		out, err := exec.Command("bash", "-c",
			fmt.Sprintf(`source %q; clear_category "$1"`, filepath.Join(m.dir, "lib/menu.sh")),
			"--", cat).CombinedOutput()
		if err != nil {
			detail := strings.TrimSpace(string(out))
			if detail == "" {
				detail = err.Error()
			}
			m.status = "無法從 Ghostty 設定移除 " + e.name + "：" + detail + "（檔案未刪除）"
			m.statusOK = false
			return m, nil
		}
	}
	if err := os.Remove(e.value); err != nil {
		m.status = "刪除失敗 " + e.name + "：" + err.Error()
		m.statusOK = false
		return m, nil
	}
	if len(applied) > 0 {
		m.status = txtDeletedAndCleared(e.name, strings.Join(applied, "、"))
	} else {
		m.status = txtDeleted(e.name)
	}
	m.statusOK = true
	m.current = currentSelections(m.dir)
	m = m.refreshEntries()
	// Reopen the picker on the remaining configs so deleting several in a
	// row doesn't mean pressing `e` again between each.
	m.editable = editableEntries(m.dir)
	if len(m.editable) == 0 {
		m.showEditPicker = false
	} else {
		m.showEditPicker = true
		m.editPickIndex = minInt(m.editPickIndex, len(m.editable)-1)
	}
	return m, nil
}

// openConflicts re-reads the list every time rather than caching it: the
// overriding file is not ours, so anything could have changed it since the
// warning that sent the user here.
func (m model) openConflicts() model {
	conflicts, err := listShadowConflicts(m.dir)
	m.conflicts, m.conflictsErr = conflicts, ""
	if err != nil {
		m.conflictsErr = err.Error()
	}
	m.showConflicts = true
	return m
}

// fixConflicts is only ever reached from the confirmation dialog, never
// straight off a keypress.
func (m model) fixConflicts() (tea.Model, tea.Cmd) {
	m.showConflictConfirm = false
	m.showConflicts = false
	line, err := resolveShadowConflicts(m.dir)
	if err != nil {
		m.status = txtFixFailed(err.Error())
		m.statusOK = false
		return m, nil
	}
	if line == "" {
		line = txtFixDone()
	}
	m.status = line
	m.statusOK = true
	return m, nil
}

// editableEntries lists only the configs that can actually be edited in
// place: the user's own saved presets. Vendored assets are deliberately
// excluded — they stay byte-identical to upstream (see NOTICE.md), so
// offering them here and then refusing would be a dead end.
func editableEntries(dir string) []entry {
	var out []entry
	for _, e := range buildEntries(dir) {
		if e.category == "custom" && e.kind == "file" {
			out = append(out, e)
		}
	}
	return out
}

func lookupKey(key string) (keyInfo, bool) {
	for _, k := range keyCatalog {
		if k.key == key {
			return k, true
		}
	}
	return keyInfo{}, false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Widths are budgeted from the margin-inset width, not m.width —
		// the screen is indented by sideMargin, so sizing panes to the full
		// window pushed the right border past the edge.
		innerW := maxInt(m.width-2*sideMargin, 40)
		// 30 / 70 split: the list only needs to show names, while the
		// preview carries shader source and full config files.
		//
		// The ratio is applied to the *rendered card* widths, which is what
		// is actually visible — box() adds 4 columns of chrome (border +
		// a space of padding each side) that each pane then gives back.
		// Splitting the post-chrome content instead would land at 31.5/68.5,
		// since the fixed 8 columns get shared evenly rather than 30/70.
		listCard := innerW * 3 / 10
		m.listWidth = maxInt(listCard-4, 20)
		m.previewWidth = maxInt(innerW-listCard-4, 20)
		// Rows spent outside the list itself: 2 leading blanks + 4 for the
		// title band (3 filled rows + its shadow row) + 1 blank + 2 pane
		// borders + 1 pane label + however many rows the footer needs. The
		// status row is transient and deliberately not budgeted — a message
		// just pushes the footer down one row rather than permanently
		// shrinking the list.
		m.bodyHeight = m.height - 10 - len(footerRows(innerW))
		m.list.SetSize(m.listWidth, m.bodyHeight)

	case tea.KeyMsg:
		if m.showRestartPrompt {
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
		if m.editor != nil {
			return m.updateEditor(msg)
		}
		if m.showTagPanel {
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
		if m.showDeleteConfirm {
			switch strings.ToLower(msg.String()) {
			case "y":
				return m.deleteCustomConfig(m.deleteTarget)
			case "n", "esc", "q", "ctrl+c":
				m.showDeleteConfirm = false
			}
			return m, nil
		}
		if m.showConflictConfirm {
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
		if m.showConflicts {
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
		if m.showEditPicker {
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
		if m.showNameDialog {
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
		if m.list.FilterState() != list.Filtering {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
				return m, tea.Quit
			case key.Matches(msg, key.NewBinding(key.WithKeys("t"))):
				m.showTagPanel = true
				return m, nil
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
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
				m.showNameDialog = true
				m.namePurpose = "save-current"
				m.nameInput.Placeholder = "my-preset-name"
				m.nameInput.SetValue("")
				m.nameInput.Focus()
				return m, textinput.Blink
			case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
				m.showNameDialog = true
				m.namePurpose = "new-file"
				m.nameInput.Placeholder = "my-new-config"
				m.nameInput.SetValue("")
				m.nameInput.Focus()
				return m, textinput.Blink
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
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
				// Shadows one of bubbles/list's five prev-page keys; b, pgup,
				// ← and h all still page, so nothing is lost.
				line, err := undoLastApply(m.dir)
				if err != nil {
					m.status = txtUndoFailed(err.Error())
					m.statusOK = false
					return m, nil
				}
				if line == "" {
					line = txtUndoDone()
				}
				m.status = line
				m.statusOK = true
				// The managed block just moved underneath us — without this
				// the "●" markers keep naming what was applied before.
				m.current = currentSelections(m.dir)
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
				// Show only what can actually be edited, rather than acting
				// on whatever happens to be selected and then refusing.
				m.editable = editableEntries(m.dir)
				if len(m.editable) == 0 {
					m.status = txtNoEditable()
					m.statusOK = false
					return m, nil
				}
				m.showEditPicker = true
				m.editPickIndex = 0
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("c", "C"))):
				// `c` is free on both sides: no handler on this screen takes
				// it, and bubbles/list's own bindings never do either — its
				// paging keys are h/l/b/f/u/d and the arrows. Uppercase is
				// bound too, matching the restart hint's y/Y.
				m = m.openConflicts()
				return m, nil
			case m.restartHint && key.Matches(msg, key.NewBinding(key.WithKeys("y", "Y"))):
				m.restartHint = false
				m.showRestartPrompt = true
				return m, nil
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
				return m, nil
			}
		}
		m.list, cmd = m.list.Update(msg)

	default:
		// Non-key messages must reach whichever list is actually on screen.
		// bubbles/list does its filtering asynchronously (the keystroke
		// returns a Cmd, and the resulting FilterMatchesMsg arrives here as
		// a plain tea.Msg) — routing every non-key message to m.list meant
		// the key/value pickers' filters silently never applied.
		if m.editor == nil {
			m.list, cmd = m.list.Update(msg)
		}
	}

	if key := m.selectionKey(); key != m.lastPreviewKey {
		m.lastPreviewKey = key
		m.updatePreviewForSelection()
	}
	return m, cmd
}

const minWidth = 80
const minHeight = 24 // tui-design skill: 80x24 is the standard minimum gate

// footerRows packs the footer's key hints into as few rows as fit in w
// display columns, measuring with the ANSI/CJK-aware lipgloss.Width. The
// footer was a single joined line and already overflowed the enforced
// 80-column minimum before [p] and [u] joined it — eleven hints need 131
// columns in Chinese, 134 in English. A footer that wraps on its own steals
// a row the height budget never knew it lost, which walks the bottom of the
// card off the screen; the budget in WindowSizeMsg counts these rows instead.
func footerRows(w int) []string {
	var rows []string
	cur := ""
	for _, part := range txtBrowserFooterParts() {
		if cur == "" {
			cur = part
			continue
		}
		if next := cur + "  " + part; lipgloss.Width(next) <= w {
			cur = next
			continue
		}
		rows = append(rows, cur)
		cur = part
	}
	if cur != "" {
		rows = append(rows, cur)
	}
	return rows
}

// statusLine renders the current status message, green on success and red
// on failure, or "" when there's nothing to report. Shared so the browser
// and editor screens can't drift on how a message looks.
func (m model) statusLine() string {
	if m.status == "" {
		return ""
	}
	// Clamped to the row it is given. A status message is allowed to push the
	// footer down one row — that is the budget's one deliberate omission — but
	// a message that WRAPS costs two, which walks the bottom of the card off
	// the screen. The override warning is what forced this: the file name plus
	// the two hints that ride along with it reach 111 columns against an
	// 80-column minimum.
	msg := truncate(m.status, maxInt(m.width-2*sideMargin, 20))
	if m.statusOK {
		return statusStyle.Render(msg)
	}
	return errorStyle.Render(msg)
}

// elidePath shortens a path from the middle rather than the end. The two files
// Ghostty reads here differ only in their last characters — config against
// config.ghostty — so a plain right-hand truncation draws them as the same row
// twice, in the one dialog where knowing which file is being written matters.
func elidePath(p string, w int) string {
	if lipgloss.Width(p) <= w {
		return p
	}
	base := filepath.Base(p)
	if lipgloss.Width(base)+1 >= w {
		// Not even the file name fits; keep its tail, which is the part that
		// tells the two apart.
		r := []rune(base)
		for len(r) > 0 && lipgloss.Width("…"+string(r)) > w {
			r = r[1:]
		}
		return "…" + string(r)
	}
	head := []rune(strings.TrimSuffix(p, base))
	for len(head) > 0 && lipgloss.Width(string(head)+"…"+base) > w {
		head = head[:len(head)-1]
	}
	return string(head) + "…" + base
}

// conflictRow lays out one offending line: line number, the key it sets, then
// the line itself. Each field is truncated before it is padded, so a long path
// or a long value cannot push the row past the frame box() draws around it, and
// padRight counts display columns — a rune count would leave every row after a
// CJK label ragged.
func conflictRow(c shadowConflict, w int) string {
	num := padRight(truncate(c.line, 5), 6)
	keyW := minInt(maxInt(w/3, 10), 24)
	keyCol := padRight(truncate(c.key, keyW), keyW+2)
	textW := maxInt(w-2-lipgloss.Width(num)-lipgloss.Width(keyCol), 8)
	return helpStyle.Render("  "+num) +
		phosphorStyle.Render(keyCol) +
		errorStyle.Render(truncate(c.text, textW))
}

// conflictsPanel lists what is overriding the managed block. An empty result
// says so in a sentence rather than showing an empty box, which would read as a
// broken panel instead of as good news.
func (m model) conflictsPanel() string {
	rowW := maxInt(m.width-14, 24)
	var v []string
	v = append(v, bracket(txtConflictsTitle()))

	footer := txtConflictsFooterClean()
	switch {
	case m.conflictsErr != "":
		v = append(v, "")
		v = append(v, errorStyle.Render(truncate(txtConflictsFailed(m.conflictsErr), rowW)))
	case len(m.conflicts) == 0:
		v = append(v, "")
		v = append(v, phosphorStyle.Render(truncate(txtConflictsNone(), rowW)))
		v = append(v, helpStyle.Render(truncate(txtConflictsNoneHint(), rowW)))
	default:
		footer = txtConflictsFooter()
		v = append(v, helpStyle.Render(truncate(txtConflictsBody1(), rowW)))
		v = append(v, helpStyle.Render(truncate(txtConflictsBody2(), rowW)))
		v = append(v, "")
		// The overlay is centred by lipgloss.Place, which does not scroll, so
		// the rows are budgeted against the window rather than the list length:
		// past the budget the box would run off both ends of the screen.
		budget := maxInt(m.height-12, 4)
		used, shown, lastFile := 0, 0, ""
		for _, c := range m.conflicts {
			need := 1
			if c.file != lastFile {
				need = 2 // this file needs a heading of its own first
			}
			if used+need > budget {
				break
			}
			if c.file != lastFile {
				lastFile = c.file
				v = append(v, labelStyle.Render(elidePath(c.file, rowW)))
			}
			v = append(v, conflictRow(c, rowW))
			used += need
			shown++
		}
		if rest := len(m.conflicts) - shown; rest > 0 {
			v = append(v, helpStyle.Render(txtConflictsMore(rest)))
		}
	}
	v = append(v, "")
	v = append(v, helpStyle.Render(footer))

	content := strings.Join(v, "\n")
	w := minInt(maxInt(lipglossMaxWidth(v)+2, 56), m.width-8)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		box(content, w, len(v)))
}

// conflictConfirm names the whole of what is about to happen before any of it
// happens — how many lines, in which files, commented rather than removed, and
// backed up first. Everything else this tool writes stays inside its own
// managed block; this one does not, so it asks first.
func (m model) conflictConfirm() string {
	rowW := maxInt(m.width-14, 24)
	var v []string
	v = append(v, bracket(txtFixTitle()))
	v = append(v, "")
	v = append(v, phosphorStyle.Render(truncate(txtFixAsk(len(m.conflicts)), rowW)))
	files, counts := conflictFileCounts(m.conflicts)
	for _, f := range files {
		suffix := "  " + txtFixFileCount(counts[f])
		path := elidePath(f, maxInt(rowW-2-lipgloss.Width(suffix), 12))
		v = append(v, phosphorStyle.Render("  "+path)+helpStyle.Render(suffix))
	}
	v = append(v, "")
	v = append(v, helpStyle.Render(truncate(txtFixNotDeleted(), rowW)))
	v = append(v, helpStyle.Render(truncate(txtFixBackup(), rowW)))
	v = append(v, errorStyle.Render(truncate(txtFixOutside(), rowW)))
	v = append(v, "")
	v = append(v, errorStyle.Render(txtFixYes())+"   "+helpStyle.Render(txtFixNo()))

	content := strings.Join(v, "\n")
	w := minInt(maxInt(lipglossMaxWidth(v)+2, 52), m.width-8)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		box(content, w, len(v)))
}

func (m model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	if m.width < minWidth || m.height < minHeight {
		msg := txtTooSmall(m.width, m.height, minWidth, minHeight)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, errorStyle.Render(msg))
	}
	if m.showRestartPrompt {
		msg := bracket(txtRestartTitle()) + "\n\n" +
			helpStyle.Render(txtRestartBody()) + "\n\n" +
			titleStyle.Render(txtRestartAsk()) + "\n\n" +
			statusStyle.Render(txtRestartYes()) + "   " + errorStyle.Render(txtRestartNo())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	}
	if m.showTagPanel {
		return m.tagPanel()
	}
	if m.showConflictConfirm {
		return m.conflictConfirm()
	}
	if m.showConflicts {
		return m.conflictsPanel()
	}
	if m.showDeleteConfirm {
		t := m.deleteTarget
		var v []string
		v = append(v, bracket(txtDeleteTitle()))
		v = append(v, "")
		v = append(v, phosphorStyle.Render(txtDeleteAsk(t.name)))
		v = append(v, helpStyle.Render(txtDeleteWarn()))
		if applied := m.appliedCategoriesFor(t); len(applied) > 0 {
			v = append(v, "")
			v = append(v, errorStyle.Render(txtDeleteApplied(strings.Join(applied, "、"))))
			v = append(v, helpStyle.Render(txtDeleteAppliedNote()))
		}
		v = append(v, "")
		v = append(v, errorStyle.Render(txtDeleteYes())+"   "+helpStyle.Render(txtDeleteNo()))
		content := strings.Join(v, "\n")
		w := minInt(maxInt(lipglossMaxWidth(v)+2, 48), m.width-8)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			box(content, w, len(v)))
	}
	if m.showEditPicker {
		var v []string
		v = append(v, bracket(txtPickConfigTitle()))
		v = append(v, helpStyle.Render(txtPickConfigBody()))
		v = append(v, "")
		for i, it := range m.editable {
			n := len(parseFieldRows(it.value))
			row := it.name + "  " + txtSettingCount(n)
			if i == m.editPickIndex {
				v = append(v, statusStyle.Render("▸ "+row))
			} else {
				v = append(v, phosphorStyle.Render("  "+row))
			}
		}
		v = append(v, "")
		v = append(v, helpStyle.Render(txtPickConfigFooter()))
		content := strings.Join(v, "\n")
		w := minInt(maxInt(lipglossMaxWidth(v)+2, 44), m.width-8)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			box(content, w, len(v)))
	}
	if m.showNameDialog {
		var title, body string
		if m.namePurpose == "new-file" {
			title, body = txtNewConfigTitle(), txtNewConfigBody()
		} else {
			title, body = txtSaveComboTitle(), txtSaveComboBody()
		}
		msg := bracket(title) + "\n\n" +
			helpStyle.Render(body) + "\n\n" +
			m.nameInput.View() + "\n\n" +
			helpStyle.Render(txtConfirmFoot())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	}
	if m.editor != nil {
		return m.editor.view(m.width, m.height, m.statusLine())
	}

	// Left pane gets the same "name your own function on row one" treatment
	// as the editor's panes.
	// Name the active tags in the pane header. A filter you cannot see is a
	// filter you will blame the catalog for.
	paneTitle := labelStyle.Render(txtPaneList())
	if lbl := activeTagLabel(m.activeTags); lbl != "" {
		paneTitle += "  " + statusStyle.Render("["+lbl+"]")
	}
	// bubbles/list writes its own "No items" line, which comes out as
	// "No 個項目" once the item name is Chinese. An empty result is a normal
	// outcome of intersecting tags, so it gets a sentence that says what to
	// do about it instead.
	listBody := m.list.View()
	if len(m.list.Items()) == 0 && len(m.activeTags) > 0 {
		// Pad back to the height the list would have taken. The card height
		// is measured from this string, so a two-line message would collapse
		// both panes to a stub.
		empty := []string{"", helpStyle.Render(txtEmptyByTags())}
		for len(empty) < m.bodyHeight {
			empty = append(empty, "")
		}
		listBody = strings.Join(empty, "\n")
	}
	listView := paneTitle + "\n" + listBody
	// list.Model's SetSize(width, height) budgets height for the item area
	// only — its title/status/pagination/help chrome is added on top, so
	// the actual listView is taller than m.bodyHeight. Measure it directly
	// and use THAT as the shared target for both boxes, rather than trying
	// to predict bubbles' internal chrome overhead.
	targetHeight := strings.Count(listView, "\n") + 1

	previewName := ""
	marker := ""
	if item, ok := m.list.SelectedItem().(entry); ok {
		if v, exists := m.current[item.category]; exists && v == item.value {
			marker = currentMarker
		}
		previewName = item.source + "/" + item.name
	}
	previewBody := labelStyle.Render(txtPanePreview()) + "\n\n" +
		marker + titleStyle.Render(previewName) + "\n\n" + m.previewContent
	// Pad to targetHeight ourselves rather than trusting lipgloss's
	// Style.Height() to add the missing blank lines — it undercounted
	// lines in this already-ANSI-styled multi-line string, closing the
	// preview box's border many rows above the list box's (same root
	// cause class as the earlier width-measurement bug; see ISA
	// Changelog "20260726-tmux-render-artifact").
	previewBody = padVertical(previewBody, targetHeight)

	// Both boxes render at targetHeight (measured from the list's actual
	// output, not the m.bodyHeight budget passed into SetSize — see the
	// targetHeight comment above for why those two numbers can differ).
	row := lipgloss.JoinHorizontal(lipgloss.Top,
		box(listView, m.listWidth, targetHeight),
		box(previewBody, m.previewWidth, targetHeight),
	)

	status := m.statusLine()
	help := helpStyle.Render(strings.Join(footerRows(maxInt(m.width-2*sideMargin, 40)), "\n"))

	// Orange title band with a drop shadow, spanning the same width as the
	// two panes below (listWidth+previewWidth+8 counts each pane's chrome).
	left := bannerTitleStyle.Render("GHOSTTY CONFIG STUDIO") +
		bannerDimStyle.Render("  ·  "+txtBrowserMode())
	right := bannerDimStyle.Render(txtItemCount(len(m.list.Items())))
	head := bannerBlock(left, right, m.listWidth+m.previewWidth+8)

	// Two blank rows above the title block, one below — same rhythm as the
	// editor screen.
	// The footer sits directly under the card. The status row is only
	// emitted when there's a message, so an always-present empty row can't
	// push the footer away from the content.
	parts := []string{"", "", head, "", row}
	if status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, help)
	body := strings.Join(parts, "\n")
	return indentBlock(body, sideMargin)
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("ghostty-config-studio", version)
		return
	}
	dir, err := scriptDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "couldn't resolve script directory:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(newModel(dir), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// updateEditor bridges the main model to the settings editor, handling the
// two actions that need model-level state (saving to disk, and closing —
// which must refresh the item list so a newly-saved config appears).
func (m model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.editor
	// Save/exit are only meaningful when no value editor is open; while one
	// is, "s" is just a character being typed into a text field.
	if e.editingKey == "" {
		switch msg.String() {
		case "esc":
			if e.newFile && len(e.checked) == 0 {
				_ = os.Remove(e.path)
			}
			m.editor = nil
			m = m.refreshEntries()
			return m, nil
		case "s", "ctrl+s":
			if err := writeFieldRows(e.path, e.orderedRows()); err != nil {
				m.status = "failed to save " + e.name + ": " + err.Error()
				m.statusOK = false
				return m, nil
			}
			m.status = txtSaved(e.name, len(e.checked))
			m.statusOK = true
			e.newFile = false
			return m, nil
		}
	}
	return m, e.update(msg)
}
