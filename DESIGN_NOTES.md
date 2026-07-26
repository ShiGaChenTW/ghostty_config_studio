# Design Notes

Decisions and hard-won details behind this project, kept for anyone reading or
extending the code. These are the things that are not obvious from the source,
and several of them were only discovered by getting them wrong first.

## Config writing

**Ghostty's own `config-file` include is the composition mechanism.** No
merging or concatenating of config text — the tool only writes which file to
include. Selections stay independent, and the underlying assets are never
copied or rewritten.

**Everything lives inside marker comments.** Content outside
`# >>> ghostty-picker managed >>>` … `# <<< ghostty-picker managed <<<` is
never touched, so an existing hand-written config survives intact.

**`config-file` takes the rest of the line, literally.** An inline trailing
comment does not work:

```
config-file = /path/to/theme.conf  # category:theme   ← BROKEN
```

Ghostty tries to open a path that includes the comment text, fails with
`error.FileNotFound`, and then falls back to defaults for the *entire* config.
Category tags therefore live on their own comment line above the directive.

**A dangling include breaks everything, silently.** Deleting a config file that
is still referenced produces the same whole-config fallback. That's why
deletion clears the managed-block reference first.

**Verify with `+validate-config`, not `+show-config`.** `ghostty +show-config`
prints the resolved config and exits clean even when an include is missing —
it will not tell you the config is broken. `ghostty +validate-config` reports
`error opening config-file …: error.FileNotFound`. Use the latter.

**`config-file` includes are applied after every direct assignment in the
parent file**, wherever the directive itself sits. Verified with
`+show-config`: `font-family = BBB` written before, after, or with a reset
between makes no difference; the include's value always lands last. So a
setting that arrives through an include can never outrank one assigned
directly, and a reset in the parent cannot clear what an include brought in.

**`font-family` is a list, and the first entry wins.** A second
`font-family = X` appends a fallback rather than replacing. Combined with the
rule above, a font pack applied through an include was invisible to anyone who
had already set their own font: theirs was assigned directly, so it stayed the
primary face. The managed block now restates the font directly, above the
include that also carries it, after an empty-value line that clears the list.
The include still supplies size, thickening and ligatures; the direct lines
only settle which face wins.

**Ghostty reads more than one config on macOS.** Besides
`~/.config/ghostty/config` it loads
`~/Library/Application Support/com.mitchellh.ghostty/config*`, and that one is
applied afterwards, so every key it sets beats the managed block. A selection
then appears to do nothing at all with no error anywhere to explain it, which
is why `apply_selection` names the file and the shadowed keys instead of
reporting a silent success. The TUI also suppresses its restart prompt in that
case: restarting re-reads the same overriding file, so offering it would be a
lie.

**An empty value is worse than an error.** Writing `key = ` parses fine and is
then silently ignored by Ghostty, so the UI would show a setting as active
while it does nothing. The editor treats an empty input as "not set".

## Rendering

**Never nest already-styled multi-line content inside another
`lipgloss.Style{}` that measures it.** `.Width()` / `.Height()` mis-measure
ANSI-styled input in some terminal environments — observed as both mid-line
truncation and phantom extra rows (a `Height(31)` render producing 38 lines).
Boxes here are hand-drawn, using the ANSI-aware `lipgloss.Width()` for padding.

**Measure display width, not byte or rune length.** CJK cells are double-width.
`len()` will silently misalign every layout that contains Chinese text.

**Colored fills need every segment styled.** A styled fragment ends with a
reset, so concatenating `styledA + "   " + styledB` and wrapping the result in
one outer `Render` leaves the middle spaces unstyled — it showed up as a dark
bar cutting through the title band. Each segment carries its own background.

**Foreground block glyphs beat background fills for swatches.** An early color
preview painted a background-filled text panel and truncated unpredictably.
Drawing `█` in the foreground color has been completely stable.

**A box must clamp its content, not just pad it.** `box()` padded short lines
but left long ones alone, so any line over budget shoved the right-hand frame
out with it and the border stepped sideways mid-card. bubbles/list renders two
columns wider than the size it was given once its filter prompt is up, which is
exactly when it showed. Truncate with `ansi.Truncate`, which counts display
cells and leaves escape sequences intact; a rune-slice cut would sever them.

**bubbles/list filters asynchronously.** The keystroke returns a `Cmd`, and the
resulting `FilterMatchesMsg` arrives as a plain `tea.Msg`. Routing all non-key
messages to a single list means any other list's filter silently never applies.

**`list.SetSize(w, h)` budgets the item area only.** Title, status, pagination
and help chrome are added on top, so the rendered height exceeds `h`. Measure
the rendered output if you need panes to match.

## Catalog

`tui/keycatalog.go` is generated-assisted: key names, defaults and enumerated
legal values come from parsing `ghostty +show-config --default=true
--docs=true` on a real install, rather than being transcribed by hand. The
Traditional Chinese names, descriptions and category assignments are written by
hand. Regenerate against a newer Ghostty by re-parsing that same command.

A newer Ghostty may have keys this catalog lacks. Two consequences are handled:
the editor keeps a manual-key escape hatch, and saving preserves any key
already in the file that the catalog doesn't recognise rather than dropping it.

Two upstream data bugs are worked around at the description layer only, leaving
the vendored files byte-identical: in `snedea/ghostty-themes`,
`config.cyberpunk` duplicates `config.matrix` and `config.neon` duplicates
`config.pico`, comments and colors alike. The menu shows each its correct
description, but the 18 theme entries yield 16 visually distinct results.

## Search

The filter indexes both languages at once: `searchAliases` folds each row's
category and style tags in under their English and Chinese spellings, so
`/cursor` and `/游標` return the same rows whichever language is on screen.

Ghostty's 460+ built-in themes all carry one identical description line, which
is deliberately kept **out** of the index. It discriminates between none of
them while handing the fuzzy matcher another twenty characters to find a
subsequence in; including it took a `/retro` query from 31 hits to 282.

## Localisation

The UI ships in Traditional Chinese and English, toggled with `L` and
remembered in `$GHOSTTY_DIR/.ghostty-tui-lang`.

`lang` is a package-level variable rather than a field threaded through
render calls: it's read by dozens of leaf functions — including `keyInfo`
methods that have no access to the model — and only changes on an explicit
keypress, so threading it would add noise without buying anything.

English key descriptions are Ghostty's own doc text, extracted from the same
`+show-config --docs=true` parse that produced the defaults. Only 17 of the
200 needed hand-writing, where Ghostty's first doc line is empty or generic
boilerplate. The value-range hints are formulaic enough that a phrase map
covers all 200 with full coverage asserted at generation time.

Toggling on the browser screen rebuilds the list items: descriptions are
language-dependent and bubbles/list caches what it was given, so without a
rebuild the rows keep the old language until something else changes them.

## Packaging

**Imported packs live in `~/.config/ghostty-config-studio/`, never beside the
binary.** A Homebrew install lands under a Cellar prefix that `brew upgrade`
replaces wholesale, so assets written there would vanish on the next version
bump — and on Intel prefixes may not be writable at all. `lib/menu.sh` exports
`STUDIO_ASSETS` and `tui/main.go`'s `studioAssetsDir()` mirrors it; both honour
`GHOSTTY_STUDIO_DIR` so tests can redirect them.

**The formula uses exec wrappers, not symlinks.** The shell entry points find
`lib/menu.sh` through `$BASH_SOURCE`, which resolves to the symlink itself, not
its target — a symlinked `ghostty-theme` would look for `lib/` inside
Homebrew's `bin` and fail. `bin.write_exec_script` generates a wrapper that
execs the real path in `libexec`, so `$BASH_SOURCE` stays honest. (The Go
binary would survive either way; it calls `filepath.EvalSymlinks`.)

## Portability

**bash 3.2 in a UTF-8 locale eats the first byte of a multibyte character
into an unbraced variable name.** `echo "reads $other，then"` fails with
`other<ef>: unbound variable`, while the identical line with `LANG` unset runs
fine, which is exactly the environment a test harness tends to have. Every
user-facing string here is bilingual, so `$var` sitting right before a
full-width comma or colon is easy to write and impossible to notice. Brace it.
`scripts/check.sh` greps for the pattern and runs the commands under
`en_US.UTF-8` so the trap cannot come back.

**macOS ships bash 3.2.** No `declare -A`, no `${var^}`. Everything here runs
on the stock shell so no Homebrew bash is required.

**One implementation of the config write.** The TUI shells out to
`lib/menu.sh`'s `apply_selection` instead of reimplementing managed-block
handling in Go, so the CLI and the TUI cannot drift.

## Known limitation

The 12 shader themes bake in their own `background-opacity`, `background-blur`,
`cursor-style` and `cursor-style-blink` as part of their visual ambiance.
Combining one with an independent choice for the same key gives inconsistent
precedence — in testing, `cursor-style` respected the explicit choice while
`background-opacity`, `background-blur` and `cursor-style-blink` did not. The
exact rule in Ghostty's include precedence has not been pinned down. These
settings behave predictably on their own and alongside color themes, built-in
themes and presets.
