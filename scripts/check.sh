#!/usr/bin/env bash
# Pre-release checks for the shell half of the tool.
#
# The interesting one is the UTF-8 trap. macOS ships bash 3.2, and in a UTF-8
# locale it swallows the first byte of a following multibyte character into an
# unbraced variable name:
#
#     other=/tmp/x; echo "reads $other，then"   ->  other<ef>: unbound variable
#
# Every user-facing string in this tool is bilingual, so `$var` sitting right
# before a full-width comma or colon is easy to write and impossible to notice:
# with LANG unset the same line works fine, which is exactly the environment a
# test harness tends to have. Brace the variable and it is unambiguous again.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

fail=0
say() { printf '%-46s %s\n' "$1" "$2"; }

SHELL_FILES=(lib/menu.sh ghostty-setup ghostty-theme ghostty-font ghostty-preset
             ghostty-cursor ghostty-custom ghostty-window ghostty-cursor-style
             ghostty-clipboard)

# 1. Syntax.
for f in "${SHELL_FILES[@]}"; do
  if ! /bin/bash -n "$f" 2>/dev/null; then
    say "syntax $f" "FAIL"; fail=1
  fi
done
say "bash 3.2 syntax" "ok"

# 2. No unbraced variable directly followed by a non-ASCII character.
hits=$(grep -nHE '\$[A-Za-z_][A-Za-z0-9_]*[^ -~]|\$[0-9@?][^ -~]' "${SHELL_FILES[@]}" 2>/dev/null || true)
if [ -n "$hits" ]; then
  say "unbraced \$var before a multibyte char" "FAIL"
  echo "$hits" | sed 's/^/    /'
  fail=1
else
  say "unbraced \$var before a multibyte char" "none"
fi

# 3. The strings actually render, in both languages, under the locale that
#    triggers the trap. Sandboxed: this must never touch a real config.
sandbox="$(mktemp -d)"
trap 'rm -rf "$sandbox"' EXIT
mkdir -p "$sandbox/g" "$sandbox/s"
for lang in zh en; do
  echo "$lang" > "$sandbox/g/.ghostty-tui-lang"
  out=$(LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 GHOSTTY_DIR="$sandbox/g" \
        GHOSTTY_STUDIO_DIR="$sandbox/s" /bin/bash ./ghostty-theme --current 2>&1)
  if echo "$out" | grep -q 'unbound variable'; then
    say "utf-8 locale, lang=$lang" "FAIL"; echo "$out" | sed 's/^/    /'; fail=1
  else
    say "utf-8 locale, lang=$lang" "ok"
  fi
done

[ "$fail" -eq 0 ] && echo "all checks passed"
exit "$fail"
