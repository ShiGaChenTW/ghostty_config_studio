#!/usr/bin/env bash
# Does tui/keycatalog.go still match the Ghostty that is installed?
#
# The catalog's `validVals` drives a CLOSED picker: when it is non-empty the
# editor only lets you commit one of those values, with no way to type another.
# So a list that is short by one value does not degrade, it makes a legal
# setting unreachable — and on Ghostty 1.3.1 that was live for 13 keys, with
# `shell-integration` offering 3 of its 7 values and `macos-icon` 4 of 11.
#
# The enums are read back from the binary rather than from
# `+show-config --docs=true`, because prose is what produced those wrong lists
# in the first place. Feeding a key a value it cannot accept makes
# +validate-config answer with the real list:
#
#     $ printf 'shell-integration = __probe__\n' > c
#     $ ghostty +validate-config --config-file=c
#     invalid value "__probe__", valid values are: none, detect, bash, elvish, …
#
# Deliberately NOT a generator. Ghostty's own --help says the option list lives
# in src/config/Config.zig and that a command-line interface for it is future
# work, so this parses the binary for the two things it answers unambiguously —
# which keys exist, and what each enum accepts — and leaves every judgement
# call (the Chinese names, the descriptions, the category assignments) to a
# human. It reports drift; it never rewrites the file.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

CATALOG=tui/keycatalog.go

ghostty_bin=""
command -v ghostty >/dev/null 2>&1 && ghostty_bin=$(command -v ghostty)
if [ -z "$ghostty_bin" ] && [ -x /Applications/Ghostty.app/Contents/MacOS/ghostty ]; then
  ghostty_bin=/Applications/Ghostty.app/Contents/MacOS/ghostty
fi
if [ -z "$ghostty_bin" ]; then
  echo "no Ghostty binary found — cannot check the catalog against it"
  exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
probe="$work/probe.conf"
fail=0

# key<TAB>comma-separated validVals (empty when the catalog says nil). One
# entry per line, and validVals is the last []string{…} or nil on it, right
# before the category string that closes the entry.
#
# awk rather than sed: this needs alternation, and BSD sed has no \| — a GNU-ism
# that silently matches nothing here rather than erroring.
awk '
  /^[[:space:]]*\{"[a-z0-9-]+",/ {
    key = $0
    sub(/^[[:space:]]*\{"/, "", key)
    sub(/".*/, "", key)
    vals = ""
    if (match($0, /\[\]string\{[^}]*\}/)) {
      vals = substr($0, RSTART + 9, RLENGTH - 10)
      gsub(/[" ]/, "", vals)
    }
    print key "\t" vals
  }
' "$CATALOG" > "$work/catalog"

catalog_keys=$(cut -f1 "$work/catalog" | sort)
[ -n "$catalog_keys" ] || { echo "could not parse $CATALOG — the entry format must have changed"; exit 1; }

# Every key the installed Ghostty knows about.
"$ghostty_bin" +show-config --default=true 2>/dev/null \
  | sed -n 's/^\([a-z0-9-]*\)[[:space:]]*=.*/\1/p' | sort -u > "$work/live"

added=$(comm -13 <(echo "$catalog_keys") "$work/live")
removed=$(comm -23 <(echo "$catalog_keys") "$work/live")
if [ -n "$added" ]; then
  echo "Ghostty has keys the catalog does not:"
  echo "$added" | sed 's/^/    /'
  fail=1
fi
if [ -n "$removed" ]; then
  echo "the catalog has keys Ghostty no longer knows:"
  echo "$removed" | sed 's/^/    /'
  fail=1
fi

# Enum drift, which is the one that silently locks a user out of a value.
while IFS="	" read -r key have; do
  [ -n "$key" ] || continue
  printf '%s = __probe__\n' "$key" > "$probe"
  want=$("$ghostty_bin" +validate-config --config-file="$probe" 2>&1 \
    | tr '\r' '\n' | sed -n 's/.*valid values are:[[:space:]]*//p' | head -1 | sed 's/, /,/g')
  # No enum reported: the key takes free-form input, and the catalog agreeing
  # by saying nil is the correct answer.
  [ -n "$want" ] || continue
  # A plain boolean needs no picker; the editor already handles true/false.
  [ "$want" = "false,true" ] && [ -z "$have" ] && continue
  if [ "$(echo "$have" | tr ',' '\n' | sort | tr '\n' ' ')" \
     != "$(echo "$want" | tr ',' '\n' | sort | tr '\n' ' ')" ]; then
    printf '%s\n' "$key"
    printf '    catalog: %s\n' "${have:-nil}"
    printf '    ghostty: %s\n' "$want"
    fail=1
  fi
done < "$work/catalog"

if [ "$fail" -eq 0 ]; then
  echo "catalog matches $("$ghostty_bin" +version 2>/dev/null | head -1)"
else
  echo
  echo "Fix tui/keycatalog.go by hand — this reports drift, it does not rewrite."
fi
exit "$fail"
