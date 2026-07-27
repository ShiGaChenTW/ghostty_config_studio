#!/usr/bin/env bash
# End-to-end checks for the config writer: snapshot, undo, validate-and-roll-back,
# and preview. Companion to check.sh, which covers portability rather than
# behaviour.
#
# Everything runs against a sandbox GHOSTTY_DIR — this suite must never be able
# to touch a real ~/.config/ghostty, since half of what it asserts is that a
# write went wrong and had to be undone.
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SANDBOX=$(mktemp -d)
export GHOSTTY_DIR="$SANDBOX/ghostty"
export GHOSTTY_STUDIO_DIR="$SANDBOX/studio"
mkdir -p "$GHOSTTY_DIR" "$GHOSTTY_STUDIO_DIR"

fails=0
ok()   { printf '  PASS  %s\n' "$1"; }
bad()  { printf '  FAIL  %s\n' "$1"; fails=$((fails + 1)); }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want '$3', got '$2')"; fi; }

run() { bash -c "source '$REPO/lib/menu.sh'; $*"; }

echo "sandbox: $SANDBOX"

echo
echo "== a config the user wrote by hand survives everything =="
printf 'font-size = 99\n' > "$GHOSTTY_DIR/config"
run 'apply_selection window 0.9 raw "" background-opacity' >/dev/null 2>&1
grep -q '^font-size = 99$' "$GHOSTTY_DIR/config" \
  && ok "unmanaged line untouched" || bad "unmanaged line lost"
grep -q '^background-opacity = 0.9$' "$GHOSTTY_DIR/config" \
  && ok "selection written" || bad "selection missing"

echo
echo "== undo walks back, byte for byte =="
cp "$GHOSTTY_DIR/config" "$SANDBOX/after-first"
run 'apply_selection window true raw "" background-blur' >/dev/null 2>&1
run 'undo_last_apply' >/dev/null 2>&1 && ok "undo exit 0" || bad "undo exit non-zero"
if diff -q "$SANDBOX/after-first" "$GHOSTTY_DIR/config" >/dev/null; then
  ok "restored byte-for-byte"
else
  bad "restored file differs"; diff "$SANDBOX/after-first" "$GHOSTTY_DIR/config" | head
fi

echo
echo "== undo twice reaches the pre-managed state =="
run 'undo_last_apply' >/dev/null 2>&1
if grep -q 'ghostty-picker managed' "$GHOSTTY_DIR/config"; then
  bad "second undo left a managed block"
else
  ok "second undo removed the managed block"
fi

echo
echo "== nothing left to undo is an honest failure, not a crash =="
while run 'undo_last_apply' >/dev/null 2>&1; do :; done
run 'undo_last_apply' >/dev/null 2>&1
check "exhausted undo returns 1" "$?" "1"

echo
echo "== a dangling include is caught and rolled back =="
printf 'font-size = 99\n' > "$GHOSTTY_DIR/config"
ghost="$SANDBOX/vanishing.conf"
printf 'background = #123456\n' > "$ghost"
run "apply_selection theme '$ghost' file" >/dev/null 2>&1
rm -f "$ghost"
before=$(cat "$GHOSTTY_DIR/config")
run "apply_selection theme '$ghost' file" >/dev/null 2>&1
rc=$?
after=$(cat "$GHOSTTY_DIR/config")
if [ -x /Applications/Ghostty.app/Contents/MacOS/ghostty ]; then
  check "broken apply returns non-zero" "$([ $rc -ne 0 ] && echo yes || echo no)" "yes"
  check "config rolled back" "$([ "$before" = "$after" ] && echo yes || echo no)" "yes"
else
  echo "  SKIP  no Ghostty binary — validation cannot run here"
fi

echo
echo "== preview writes nothing =="
printf 'font-size = 99\n' > "$GHOSTTY_DIR/config"
run 'apply_selection window 0.8 raw "" background-opacity' >/dev/null 2>&1
snap=$(cat "$GHOSTTY_DIR/config")
out=$(run 'preview_directive "0.5" "raw" "background-opacity" ""' 2>/dev/null)
check "preview prints the directive" "$out" "background-opacity = 0.5"
check "preview left the config alone" "$([ "$snap" = "$(cat "$GHOSTTY_DIR/config")" ] && echo yes || echo no)" "yes"

echo
echo "== an explicit choice outranks a shader theme's bundled value =="
# The reason this suite can assert it at all: +show-config --config-file=X does
# not work (exits 1, no output), but a sandboxed XDG_CONFIG_HOME does.
GHOSTTY_BIN=""
command -v ghostty >/dev/null 2>&1 && GHOSTTY_BIN=$(command -v ghostty)
[ -z "$GHOSTTY_BIN" ] && [ -x /Applications/Ghostty.app/Contents/MacOS/ghostty ] \
  && GHOSTTY_BIN=/Applications/Ghostty.app/Contents/MacOS/ghostty
THEME="$HOME/.config/ghostty-config-studio/assets/shader-themes/config.matrix"
if [ -z "$GHOSTTY_BIN" ]; then
  echo "  SKIP  no Ghostty binary — cannot ask what it would resolve"
elif [ ! -f "$THEME" ]; then
  echo "  SKIP  shader-themes pack not imported (run ghostty-setup)"
else
  # GHOSTTY_DIR is already inside the sandbox; point XDG at its parent so the
  # config we just wrote is the one Ghostty reads.
  resolved() { XDG_CONFIG_HOME="$(dirname "$GHOSTTY_DIR")" "$GHOSTTY_BIN" +show-config 2>/dev/null \
    | awk -v k="$1" '$1==k {print $3}'; }
  rm -rf "$GHOSTTY_DIR"; mkdir -p "$GHOSTTY_DIR"
  run "apply_selection theme '$THEME' file" >/dev/null 2>&1
  check "theme alone keeps its own opacity" "$(resolved background-opacity)" "0.97"
  run 'apply_selection opacity 0.55 raw "" background-opacity' >/dev/null 2>&1
  run 'apply_selection cursor-style bar raw "" cursor-style' >/dev/null 2>&1
  check "explicit opacity wins over the theme" "$(resolved background-opacity)" "0.55"
  check "explicit cursor-style wins over the theme" "$(resolved cursor-style)" "bar"
  check "keys the user did not choose keep the theme's value" "$(resolved background-blur)" "true"
fi

echo
echo "== the shadowing config can be listed and fixed =="
# GHOSTTY_SUPPORT_DIR is redirected, so this can never reach the real
# ~/Library copy — a test that edits that file would be editing the user's.
export GHOSTTY_SUPPORT_DIR="$SANDBOX/support"
mkdir -p "$GHOSTTY_SUPPORT_DIR"
rm -rf "$GHOSTTY_DIR"; mkdir -p "$GHOSTTY_DIR"
run 'apply_selection opacity 0.7 raw "" background-opacity' >/dev/null 2>&1
printf 'window-padding-x = 8\nbackground-opacity = 1.0\n# background-opacity = 0.3\n' \
  > "$GHOSTTY_SUPPORT_DIR/config"
cp "$GHOSTTY_SUPPORT_DIR/config" "$SANDBOX/shadow-before"
n=$(run 'list_shadow_conflicts' 2>/dev/null | wc -l | tr -d ' ')
check "finds the one active conflict" "$n" "1"
check "ignores the already-commented line" \
  "$(run 'list_shadow_conflicts' 2>/dev/null | grep -c '0\.3' || true)" "0"
run 'resolve_shadow_conflicts' >/dev/null 2>&1
check "conflict is gone afterwards" "$(run 'list_shadow_conflicts' 2>/dev/null | wc -l | tr -d ' ')" "0"
check "unrelated line survives untouched" \
  "$(grep -c '^window-padding-x = 8$' "$GHOSTTY_SUPPORT_DIR/config")" "1"
check "the already-commented line is byte-identical" \
  "$(grep -c '^# background-opacity = 0\.3$' "$GHOSTTY_SUPPORT_DIR/config")" "1"
# Anchored, and asserting the exact prefixed form. An unanchored substring
# count passes whether the line was commented once, twice, or not at all —
# and a double-commented line breaks the manual undo the header promises.
check "the line is commented exactly once, not deleted" \
  "$(grep -c '^# ghostty-config-studio disabled: background-opacity = 1\.0$' \
     "$GHOSTTY_SUPPORT_DIR/config")" "1"
check "and not double-commented" \
  "$(grep -c 'disabled: # ghostty-config-studio disabled:' "$GHOSTTY_SUPPORT_DIR/config" || true)" "0"

# The backup's whole purpose is to be the way back, so compare it to what was
# there rather than counting filenames — a zero-byte backup passes a count.
backup=$(ls "$GHOSTTY_STUDIO_DIR/history"/*shadow* 2>/dev/null | head -1)
check "a backup was taken" "$([ -n "$backup" ] && echo yes || echo no)" "yes"
check "the backup restores the original byte-for-byte" \
  "$([ -n "$backup" ] && diff -q "$SANDBOX/shadow-before" "$backup" >/dev/null && echo yes || echo no)" "yes"

# Returning 1 is not enough: the function returns 1 for five different
# reasons, including hard failures. Assert it declined for the right one and
# left the file alone.
before=$(md5 -q "$GHOSTTY_SUPPORT_DIR/config")
out=$(run 'resolve_shadow_conflicts' 2>&1); rc=$?
check "resolving with nothing to do returns 1" "$rc" "1"
check "and says that is why" \
  "$(printf '%s' "$out" | grep -c '沒有需要處理\|No shadow conflicts\|nothing to' || true)" "1"
check "and wrote nothing" "$(md5 -q "$GHOSTTY_SUPPORT_DIR/config")" "$before"
unset GHOSTTY_SUPPORT_DIR

echo
echo "== the config file itself survives being written to =="
# Every one of these is a real defect found by audit: mv from $TMPDIR installs
# mktemp's 0600 mode and its inode, so a write reset permissions and turned a
# symlinked config into a regular file, orphaning the real one.
rm -rf "$GHOSTTY_DIR"; mkdir -p "$GHOSTTY_DIR"
printf 'font-size = 99\n' > "$GHOSTTY_DIR/config"; chmod 644 "$GHOSTTY_DIR/config"
run 'apply_selection opacity 0.7 raw "" background-opacity' >/dev/null 2>&1
check "an apply preserves the file mode" "$(stat -f %Sp "$GHOSTTY_DIR/config")" "-rw-r--r--"

rm -rf "$GHOSTTY_DIR"; mkdir -p "$GHOSTTY_DIR"
printf 'font-size = 99\n' > "$SANDBOX/real-config"
ln -s "$SANDBOX/real-config" "$GHOSTTY_DIR/config"
run 'apply_selection opacity 0.7 raw "" background-opacity' >/dev/null 2>&1
check "a symlinked config stays a symlink" \
  "$([ -L "$GHOSTTY_DIR/config" ] && echo yes || echo no)" "yes"
check "and the file it points at is what changed" \
  "$(grep -c 'background-opacity' "$SANDBOX/real-config" || true)" "1"

echo
echo "== a config the tool cannot safely edit is refused, not half-written =="
# CRLF line endings used to match the marker as a substring but never as a
# line, so every apply was a silent no-op that reported success.
rm -rf "$GHOSTTY_DIR"; mkdir -p "$GHOSTTY_DIR"
printf '# >>> ghostty-picker managed >>>\r\n# category:opacity\r\nbackground-opacity = 1\r\n# <<< ghostty-picker managed <<<\r\n' \
  > "$GHOSTTY_DIR/config"
before=$(md5 -q "$GHOSTTY_DIR/config")
run 'apply_selection opacity 0.8 raw "" background-opacity' >/dev/null 2>&1
check "a CRLF config is refused, not silently ignored" "$?" "1"
check "and is left untouched" "$(md5 -q "$GHOSTTY_DIR/config")" "$before"

# Two managed blocks made the override include be emitted into both, which
# Ghostty rejects as a cycle — taking the whole config down with it.
rm -rf "$GHOSTTY_DIR"; mkdir -p "$GHOSTTY_DIR"
{ printf '# >>> ghostty-picker managed >>>\n# category:theme\ntheme = Dracula\n# <<< ghostty-picker managed <<<\n'
  printf '# >>> ghostty-picker managed >>>\n# category:font\nfont-size = 14\n# <<< ghostty-picker managed <<<\n'; } \
  > "$GHOSTTY_DIR/config"
run 'apply_selection opacity 0.9 raw "" background-opacity' >/dev/null 2>&1
if [ -n "$GHOSTTY_BIN" ]; then
  "$GHOSTTY_BIN" +validate-config --config-file="$GHOSTTY_DIR/config" >/dev/null 2>&1
  check "a two-block config never ends up invalid" "$?" "0"
else
  echo "  SKIP  no Ghostty binary — cannot validate the result"
fi

echo
echo "== the portability gate still passes =="
(cd "$REPO" && ./scripts/check.sh >/dev/null 2>&1) \
  && ok "scripts/check.sh" || bad "scripts/check.sh"

echo
echo "== the TUI still builds, vets and passes its own tests =="
(cd "$REPO/tui" && go build ./... && go vet ./...) >/dev/null 2>&1 \
  && ok "go build + go vet" || bad "go build/vet"
# tui/render_test.go is the Go half of this suite: layout budgets at the
# enforced minimum size, hostile conflict records, and the two performance and
# escape-sequence defects found by audit.
(cd "$REPO/tui" && go test ./...) >/dev/null 2>&1 \
  && ok "go test" || bad "go test (run it directly to see which case)"

echo
if [ "$fails" -eq 0 ]; then
  echo "all green"
else
  echo "$fails failing"
fi
exit "$fails"
