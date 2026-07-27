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

**The full precedence rule, measured rather than inferred.** Ghostty 1.3.1,
established by pointing `XDG_CONFIG_HOME` at a sandbox and reading back
`+show-config`:

| Situation | Winner |
|---|---|
| Two `config-file` includes | the later include |
| Direct assignment vs an include that sets the same key | the include, wherever the assignment sits |
| Theme include, then a generated override include after it | the override |
| `config-file` nested inside an include vs the parent's later include | the nested one — depth beats order |

The second row is why a scalar setting could never be made to win by restating
it directly in the managed block, and the third is the fix: an explicit choice
has to travel through an include of its own, emitted last.

Note `+show-config --config-file=X` does not work for this — it exits 1 with no
output. Sandbox `XDG_CONFIG_HOME` instead; that is the only way to ask Ghostty
what it would resolve for a config that is not the user's real one.

**`font-family` is a list, and the first entry wins.** A second
`font-family = X` appends a fallback rather than replacing. Combined with the
rule above, a font pack applied through an include was invisible to anyone who
had already set their own font: theirs was assigned directly, so it stayed the
primary face. The managed block now restates the font directly, above the
include that also carries it, after an empty-value line that clears the list.
The include still supplies size, thickening and ligatures; the direct lines
only settle which face wins.

**Ghostty reads more than one config on macOS.** Besides
`~/.config/ghostty/config` it loads exactly two files from
`~/Library/Application Support/com.mitchellh.ghostty/` — `config` and
`config.ghostty`, not a `config*` glob (measured with a sandboxed
`CFFIXED_USER_HOME`: a `config.bak-…` in that directory is not read) — and those are
applied afterwards, so every key it sets beats the managed block. A selection
then appears to do nothing at all with no error anywhere to explain it, which
is why `apply_selection` names the file and the shadowed keys instead of
reporting a silent success. The TUI used to also suppress its restart prompt in
that case, on the grounds that restarting re-reads the same overriding file. That
was over-corrected: the write itself did land, and the warning is about keys that
*may* be overridden, so suppressing the prompt left a red line and no way
forward. The warning now ends with "press Y to restart" and `y` opens the same
dialog the silent path pops automatically.

**An empty value is worse than an error.** Writing `key = ` parses fine and is
then silently ignored by Ghostty, so the UI would show a setting as active
while it does nothing. The editor treats an empty input as "not set".

## Winning against a theme, and against the other config

**An explicit choice travels through an override include, emitted last.** Raw
scalar settings used to be written as plain `key = value` lines in the managed
block, where the precedence table above guarantees they lose to any include —
including the shader theme the user is trying to override. They are now
collected into one generated file that the block includes after everything else,
which is the only position that wins. Keys the user did not choose are not in
that file, so a theme keeps its own ambiance for everything the user stayed
silent about.

**Commenting out the shadowing config is the one write outside the markers.**
`~/Library/Application Support/com.mitchellh.ghostty/config` is not ours, so:
the whole file is backed up into the history directory first, only lines already
reported as conflicting are touched, they are prefixed rather than deleted, and
a header block names the backup and the way back. A line that is already
commented out is not a conflict and is left exactly as it is. `GHOSTTY_SUPPORT_DIR`
exists so a test can plant a fake one — a suite asserting that this tool edits
that file must never be able to reach the real copy.

**The conflict records use ASCII 0x1F, not a tab.** A Ghostty path or value may
legally contain spaces, tabs, `=`, `#`, `|` and `:` — every printable candidate
can occur inside a field, and a control character cannot. The verbatim line is
the last field so that even an unanticipated value cannot be read as a
separator.

## Safety net around the write

**Every apply is validated, and a failed validation is rolled back.** The
dangling-include failure above is silent and total: Ghostty abandons the whole
config and falls back to defaults, so a user loses every unrelated setting too
with nothing on screen to explain it. `apply_selection` snapshots first, writes,
then runs `+validate-config`; if that fails the previous file goes straight back
and the apply reports failure. The rollback consumes its own snapshot, so a
failed attempt never leaves an undo step pointing at a broken write.

**Not being able to validate is not a reason to refuse the write.** If no
Ghostty binary can be found, `validate_config` returns success. Same for a
config file that does not exist yet: `+validate-config` exits 1 with no message
at all in that case, which would read as "your config is broken" to someone who
simply has not made one.

**A snapshot of "there was no config" is not the same as an empty config.** The
history keeps those as `.missing` files, and restoring one deletes the config
rather than leaving an empty file behind — to Ghostty, an empty config is still
a config.

**Rollback restores the snapshot this process took, not the newest one on
disk.** Two terminals applying at once would otherwise have one roll back the
other's snapshot.

**The rollback re-validates.** Restoring a snapshot re-syncs the generated
override include, which is the same transformation that just failed — so the
"safe" file put back could itself be broken, while the tool reported a
successful rollback. Found by audit, with a two-managed-block config as the
trigger.

**A config the tool cannot safely edit is refused, loudly.** Three shapes used
to half-work: CRLF line endings matched the marker as a substring but never as a
line, so every apply was a silent no-op that reported success; a missing end
marker updated existing categories while dropping new ones; markers in the wrong
order appended pairs *outside* the block, forever, breaking the project's
central promise. All three now stop with a message that names the problem —
the CRLF one says so specifically, since that config came from somewhere and
the user needs to know what to fix.

**Write with `cp`, never `mv`.** `mv` out of `$TMPDIR` installs mktemp's inode
*and* its 0600 mode — on the same device too, not only across devices as an
earlier comment here claimed. Every write therefore reset the user's config to
0600, turned a symlinked config into a regular file (orphaning the real dotfile
it pointed at), and silently overwrote a read-only config. This had already been
found and fixed once in `resolve_shadow_conflicts`; the fix had not been carried
to the four writers that touch the main config.

**One lock, taken by every writer.** Two applies at once lost a write outright,
and an ill-timed rollback could restore a snapshot predating both. `mkdir` is
the atomic primitive available in bash 3.2.

## Preview

**A preview window is a second Ghostty told to read nothing but the candidate.**
`open -na Ghostty.app --args --config-default-files=false --config-file=<tmp>`.
`config-default-files` is CLI-only and stops Ghostty loading any of the user's
config files, so the window shows the candidate alone — which also means the
macOS "Application Support config wins" problem cannot distort a preview, since
that file is not read at all. Verified by giving a sandbox config a
`working-directory` the candidate never mentions: without the flag the window
reports that directory, with it the window reports `$HOME`.

On macOS the emulator cannot be launched through the `ghostty` binary at all
(`ghostty +help` says so) — `open -na` is the supported path, and it finds the
app wherever LaunchServices has it registered rather than only under
`/Applications`.

**A split pane cannot be the preview surface, however much it should be.**
The obvious design is `new_split:right` inside the window you are already in.
Ghostty 1.3.1 rules it out three times over: `new_split` is a keybind action
taking only a direction, there is no CLI action that creates one (`+new-window`
answers "not supported on this platform"), and — the one that actually settles
it — config is loaded per app instance at launch, with no per-surface config
file. A split inherits the config of the instance that owns it, so it would
render in the theme you are trying to replace. A second instance is the only
surface that can carry its own config, so the preview window is placed where
the split would have been: the right half of the screen.

**Only the preview window's position is set, never its size.**
`window-width`/`window-height` are counted in grid cells, and cell size depends
on the font the previewed config selects — so a cell count computed by the
workbench would be wrong for exactly the font entries most worth previewing.

**Pressing `p` again replaces the preview rather than adding one.** The previous
instance is found by the marker in its argv (`ghostty-studio-preview-`, from the
temp config's own name) and signalled, which cannot reach a Ghostty the user
started themselves — their command line has no reason to contain that string.
Signalled rather than closed through the UI because the preview has a shell
running in it, which is precisely when Ghostty asks for confirmation; the
preview config also turns `confirm-close-surface` off, since a throwaway window
arguing about being closed is the wrong behaviour.

**The temp config is deleted on a delay, not on the way out.** `open -na`
returns as soon as the launch has been *requested*, so deleting immediately
races Ghostty's one read of the file at startup.

## Rendering

**Never nest already-styled multi-line content inside another
`lipgloss.Style{}` that measures it.** `.Width()` / `.Height()` mis-measure
ANSI-styled input in some terminal environments — observed as both mid-line
truncation and phantom extra rows (a `Height(31)` render producing 38 lines).
Boxes here are hand-drawn, using the ANSI-aware `lipgloss.Width()` for padding.

**The status row is budgeted only while a message is on screen.** Leaving it
unbudgeted was deliberate — a message pushing the footer down one row beats
permanently shrinking the list — but at exactly 24 rows the footer went off the
bottom instead. The recompute hangs off the tail of `Update`, the one point
every event passes through, rather than the couple of dozen places that assign
`m.status`. Caught by `tui/render_test.go`, which renders `View()` at the
enforced minimum and counts the lines; the eye had missed it because it only
shows at exactly the minimum height.

**Anything quoted out of a file we do not own is defused before it is drawn.**
The conflicts panel prints offending lines verbatim so the user can recognise
them — and a config value may contain escape sequences. A line holding `ESC[2J`
cleared the screen and an OSC sequence retitled the terminal, from a panel whose
only job is quoting a file back. Control characters are replaced with `·` at
both the parse and the render boundary.

**`truncate` bounds the input before measuring it.** It trimmed one rune at a
time, re-measuring the whole string each pass: quadratic, 2.5s at 20k characters
and 85s at 100k, measured. The conflicts panel truncates lines from a file this
tool does not own and re-renders on every keystroke, so a long line froze the
UI. No rune is narrower than one column, so everything past the first `w` runes
can be dropped before any measuring — but only when the input carries no escape
sequences, since a rune-wise cut through styled text can sever one.

**Measure display width, not byte or rune length.** CJK cells are double-width.
`len()` will silently misalign every layout that contains Chinese text.

**Colored fills need every segment styled.** A styled fragment ends with a
reset, so concatenating `styledA + "   " + styledB` and wrapping the result in
one outer `Render` leaves the middle spaces unstyled — it showed up as a dark
bar cutting through the title band. Each segment carries its own background.

**The in-pane sample is painted run by run, and each row padded to an exact
width.** A first attempt at a shell-session sample in the theme's colours was
abandoned because it truncated mid-row; the swatch strip exists because of that
failure. What was missing was the rule directly below — a shared outer
background leaves the joins between styled runs unpainted, and the padding on
each side of the block has to be part of the block rather than a gap in it. Every
row now measures the same number of display columns, which is the property the
old version silently lost.

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

## Layout of the code

**`lib/menu.sh` is a loader; the implementation is one file per concern.** It
reached 1340 lines holding five unrelated jobs. `core.sh` goes first and must:
it defines the paths, the markers, the `t` bilingual helper and the write lock
that every later part depends on. The rest are order-independent, because bash
resolves function calls at call time. Sourcing `lib/menu.sh` still exposes the
whole public surface, which is what the nine entry points and the TUI's
`bash -c 'source lib/menu.sh; …'` shell-out both rely on.

A part that fails to load stops with a message naming the file, rather than
leaving a half-built library to fail somewhere far away. The Homebrew formula
installs `lib/` wholesale, so extra files ship for free — but that layout is
worth testing directly, since `$BASH_SOURCE` resolution from a `libexec` install
is not what a git clone exercises.

**`scripts/check.sh` globs `lib/*.sh` rather than listing files.** The split
left the hardcoded list naming only `menu.sh`, so the UTF-8 lint — the most
important portability gate in the project — was covering a 36-line loader while
every bilingual string had moved somewhere it no longer looked.

**The overlay handlers live in `tui/overlays.go`.** `Update` had grown to 306
lines because each new overlay added another early-return block ahead of the
browser's own key switch, so adding the seventh meant understanding the other
six. The order they are checked in is still load-bearing — a confirmation is
checked before the panel that raised it — and that ordering is now stated in
one place instead of being implied by the layout of a long function.

## Catalog

`tui/keycatalog.go` is generated-assisted: key names, defaults and enumerated
legal values come from parsing `ghostty +show-config --default=true
--docs=true` on a real install, rather than being transcribed by hand. The
Traditional Chinese names, descriptions and category assignments are written by
hand.

**Do not take the enumerated values from the docs output.** A non-empty
`validVals` makes the editor a CLOSED picker — the user can only commit one of
those values, with no way to type another — so a list short by one value does
not degrade, it makes a legal setting unreachable. Parsing the doc prose
produced exactly that on Ghostty 1.3.1, live for 13 keys: `shell-integration`
offered 3 of its 7 values, `macos-icon` 4 of 11, and ten keys with a real third
option (`copy-on-select` has `clipboard`, `confirm-close-surface` has `always`)
were left as plain booleans.

The binary answers unambiguously if you ask it wrong. Feed a key a value it
cannot accept and `+validate-config` prints the real list:

```
$ printf 'shell-integration = __probe__\n' > c
$ ghostty +validate-config --config-file=c
invalid value "__probe__", valid values are: none, detect, bash, elvish, …
```

`scripts/check-catalog.sh` does that for every key and reports drift, in both
directions, against whatever Ghostty is installed. It runs in the release gate.
It is deliberately **not** a generator: Ghostty's own `--help` says the option
list lives in `src/config/Config.zig` and that a command-line interface for it
is future work, so the parseable surface is trusted only for what it answers
unambiguously — which keys exist, and what each enum accepts. Everything that
takes judgement stays hand-written.

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

**The reported version is injected, never typed.** It was
`const version = "0.1.8"` in `main.go` until v0.1.9 shipped a binary still
claiming to be 0.1.8 — the formula's own `test do` compares the two, so the
package was internally inconsistent and only running the binary would show it.
The formula now passes `-X main.version=#{version}` and `tui/version.go`
defaults to `dev` for a plain `go build`. `scripts/release.sh` is the other half:
it gates, tags, pushes, computes the tarball sha and rewrites the tap formula, so
the nine-step sequence cannot lose a step again.

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
