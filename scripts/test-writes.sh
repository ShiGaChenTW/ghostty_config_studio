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
echo "== the portability gate still passes =="
(cd "$REPO" && ./scripts/check.sh >/dev/null 2>&1) \
  && ok "scripts/check.sh" || bad "scripts/check.sh"

echo
echo "== the TUI still builds and vets =="
(cd "$REPO/tui" && go build ./... && go vet ./...) >/dev/null 2>&1 \
  && ok "go build + go vet" || bad "go build/vet"

echo
if [ "$fails" -eq 0 ]; then
  echo "all green"
else
  echo "$fails failing"
fi
exit "$fails"
