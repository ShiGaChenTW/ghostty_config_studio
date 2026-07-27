#!/usr/bin/env bash
# Reading and writing the managed block in the user's Ghostty config: marker
# sanity, the directive lines a selection turns into, and the four writers that
# put them in or take them out. Sourced by lib/menu.sh; not run directly.

# The managed block's shape, as one word. Every writer below compares whole
# lines against the markers, so anything that stops a marker line comparing
# equal used to make the write silently succeed and do nothing: a CR left by a
# Windows editor, a BEGIN with no END. With the pair in the wrong order it was
# worse than nothing — the directive landed OUTSIDE the block and a fresh copy
# piled up on every apply, in the one region this tool promises never to touch.
# Classify it once, up front, so the writers can refuse instead of half-working.
_marker_fault() {
  local config="$1"
  [ -f "$config" ] || { echo none; return 0; }
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" '
    {
      line = $0
      cr = sub(/\r$/, "", line)
      if (line == b) { begins++; if (begins == 1) first_begin = NR; if (cr) crlf = 1 }
      else if (line == e) { ends++; if (ends == 1) first_end = NR; if (cr) crlf = 1 }
    }
    END {
      if (begins == 0 && ends == 0) { print "none"; exit }
      if (crlf) { print "crlf"; exit }
      if (begins == 0) { print "no-begin"; exit }
      if (ends == 0) { print "no-end"; exit }
      if (first_end < first_begin) { print "reversed"; exit }
      if (begins > 1 || ends > 1) { print "duplicate"; exit }
      print "ok"
    }' "$config"
}

# Refuses, and says why, for every shape the writers cannot represent. Returns
# 0 only for the two they can: a well-formed pair, and no block at all — which
# is what a first run looks like.
_require_sane_markers() {
  local fault
  fault="$(_marker_fault "$GHOSTTY_CONFIG")"
  case "$fault" in
    ok|none) return 0 ;;
    crlf)
      t "設定檔的管理標記行以 CRLF（Windows 換行）結尾，這個工具比對不到它們。" \
        "The managed markers in your config end with CRLF (Windows line endings), so this tool cannot match them." >&2
      t "先轉成 LF 再試一次：  tr -d '\r' < \"${GHOSTTY_CONFIG}\" > /tmp/g.lf && mv /tmp/g.lf \"${GHOSTTY_CONFIG}\"" \
        "Convert it to LF first:  tr -d '\r' < \"${GHOSTTY_CONFIG}\" > /tmp/g.lf && mv /tmp/g.lf \"${GHOSTTY_CONFIG}\"" >&2
      ;;
    no-end)
      t "設定檔裡有開始標記卻沒有結束標記，工具不敢猜這個區塊到哪裡結束。" \
        "Your config has the opening marker but no closing one; this tool will not guess where the block ends." >&2
      t "請在管理區塊的最後補上這一行：  ${END_MARK}" \
        "Add this line at the end of the managed block:  ${END_MARK}" >&2
      ;;
    no-begin)
      t "設定檔裡有結束標記卻沒有開始標記。" \
        "Your config has the closing marker but no opening one." >&2
      t "請在管理區塊的最前面補上這一行：  ${BEGIN_MARK}" \
        "Add this line at the start of the managed block:  ${BEGIN_MARK}" >&2
      ;;
    reversed)
      t "設定檔裡的結束標記出現在開始標記之前，順序反了。" \
        "The closing marker comes before the opening one in your config; the pair is the wrong way round." >&2
      t "把這兩行調換成正確順序後再試一次。" \
        "Swap those two lines back into the right order and try again." >&2
      ;;
    duplicate)
      t "設定檔裡有一個以上的管理區塊。Ghostty 會讓後面的區塊蓋過前面的，所以工具改哪一個都會被另一個蓋掉。" \
        "Your config has more than one managed block. Ghostty lets the later one win, so whichever this tool edited would be overridden by the other." >&2
      t "請只保留一個管理區塊後再試一次。" \
        "Keep exactly one managed block and try again." >&2
      ;;
  esac
  return 1
}

# Category tags live on their OWN comment line, directly above their
# `config-file = <path>` line — never trailing on the same line. Ghostty's
# `config-file` directive takes the rest of the line as the literal path with
# no inline-comment stripping, so `config-file = X  # category:Y` makes
# Ghostty try to open a path that includes the comment text and fails with
# error.FileNotFound (confirmed via `ghostty +show-config`), silently
# breaking the whole config. Learned the hard way — see DESIGN_NOTES.md.

# A managed pair's directive line is `config-file = <path>` for a vendored
# file, `theme = <name>` for one of Ghostty's own 460+ built-in themes (no
# file to point at — the name IS the value), `custom-shader = <path>` for a
# cursor/background shader referenced directly, or `<key> = <value>` for a
# raw scalar Ghostty setting (background-opacity, cursor-style, etc — KEY is
# the actual Ghostty config key name, required only for kind=raw). Kind
# defaults to "file".
# Ghostty treats font-family (and its bold/italic variants) as a LIST: a
# second `font-family = X` appends a fallback rather than replacing, and the
# FIRST entry stays the primary face. Two consequences, both learned the hard
# way with `+show-config`:
#
#   1. A config that sets a font does nothing for anyone who already set their
#      own font earlier in the file. Assigning an empty value clears the list,
#      so a reset goes in first.
#   2. `config-file` includes are applied AFTER every direct assignment in the
#      parent file, wherever the directive itself sits. So a font that arrives
#      through an include can never outrank one assigned directly, and a reset
#      cannot clear anything an include brought in.
#
# Hence: the font is restated directly, above the include that also carries it.
# The include still supplies everything else in the file (size, thickening,
# ligatures); this only settles which face wins. Emitted solely for the
# variants the target file actually sets, so picking a theme never wipes a
# font the theme had no opinion about.
_font_preamble() {
  local file="$1" k v
  [ -f "$file" ] || return 0
  for k in font-family font-family-bold font-family-italic font-family-bold-italic; do
    grep -qE "^[[:space:]]*$k[[:space:]]*=" "$file" || continue
    echo "$k = "
    sed -n "s/^[[:space:]]*$k[[:space:]]*=[[:space:]]*//p" "$file" | while IFS= read -r v; do
      [ -n "$v" ] && echo "$k = $v"
    done
  done
  return 0
}

_directive_line() {
  local value="$1" kind="${2:-file}" key="${3:-}"
  case "$kind" in
    name)   echo "theme = $value" ;;
    shader) echo "custom-shader = $value" ;;
    raw)    echo "$key = $value" ;;
    *)      _font_preamble "$value"; echo "config-file = $value" ;;
  esac
}

# awk -v cannot carry a newline on BSD awk, and a directive is now more than
# one line whenever a font list has to be cleared first. Write it to a file
# and let awk read it back where it is needed.
_write_directive() {
  local out="$1" value="$2" kind="$3" key="$4"
  _directive_line "$value" "$kind" "$key" > "$out"
}

# A directive pair may be preceded by font-family preamble lines. The reader
# and the eraser below both have to step over them to reach the directive they
# belong to; the directive is always the last line of the pair.
_PREAMBLE_RE="^font-family[a-z-]* ="

# Prints the value currently recorded for a category (empty if none set).
# Strips generically ("<anything> = ") rather than a fixed list of known
# directive names, since kind=raw's key varies per category.
#
# `done` here and in every marker-walking program below: a second managed block
# used to be walked as if it were a continuation of the first, which had the
# rewriters emit the override include into both and left Ghostty reporting
# `cycle detected` — the whole-config fallback. The writers refuse that shape
# outright now; the flag is what makes every reader agree that only the first
# block is ours even if one slips through.
current_path_for() {
  local category="$1"
  [ -f "$GHOSTTY_CONFIG" ] || return 0
  GCS_TAG="# category:$category" awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v pre="$_PREAMBLE_RE" '
    BEGIN {tag=ENVIRON["GCS_TAG"]}
    $0==b {if (!done) inb=1; next} $0==e {if (inb) done=1; inb=0; next}
    inb && $0==tag {want=1; next}
    inb && want {
      if ($0 ~ pre) next
      line=$0; sub(/^[a-zA-Z0-9_-]+ = /,"",line); print line; want=0
    }' "$GHOSTTY_CONFIG"
}

# Replaces (or creates) one category's tag+directive pair inside the managed
# block; everything else in the file, and every other category's pair, is
# left untouched.
# clear_category CATEGORY — drops one category's tag+directive pair from the
# managed block, leaving every other category and everything outside the
# block untouched. Needed when deleting a custom preset that's currently
# applied: leaving a `config-file = <deleted path>` line behind makes
# Ghostty fail to open it and silently fall back to defaults for the whole
# config (the same failure mode documented in DESIGN_NOTES.md).
clear_category() {
  local category="$1" tmp rc
  [ -f "$GHOSTTY_CONFIG" ] || return 0
  _lock_acquire || return 1
  if [ "$(_marker_fault "$GHOSTTY_CONFIG")" != "ok" ]; then
    _require_sane_markers; rc=$?
    _lock_release
    return "$rc"
  fi
  tmp="$(mktemp)"
  GCS_TAG="# category:$category" \
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v pre="$_PREAMBLE_RE" '
    BEGIN {tag=ENVIRON["GCS_TAG"]}
    $0==b {print; if (!done) inb=1; next}
    $0==e {print; if (inb) done=1; inb=0; next}
    inb {
      if ($0==tag) { skip=1; next }
      if (skip) { if ($0 ~ pre) next; skip=0; next }
      print; next
    }
    { print }
  ' "$GHOSTTY_CONFIG" > "$tmp"
  rc=0
  _replace_if_changed "$GHOSTTY_CONFIG" "$tmp" || rc=1
  [ "$rc" -eq 0 ] && { _sync_overrides || rc=1; }
  _lock_release
  return "$rc"
}

# clear_categories_under PREFIX — clears every managed-block category whose
# value points inside PREFIX. Used before deleting asset files: a reference
# left pointing at a removed file makes Ghostty fail to open the include and
# fall back to defaults for the whole config.

# Each managed category and the value it currently points at, as
# `CATEGORY<US>VALUE`. <US> is ASCII 0x1F for the same reason the conflict
# records use it: a Ghostty path may legally contain every printable candidate
# for a separator, and a control character cannot.
_managed_category_values() {
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v pre="$_PREAMBLE_RE" '
    $0==b {if (!done) inb=1; next}
    $0==e {if (inb) done=1; inb=0; next}
    inb && /^# category:/ { c=substr($0, 12); next }
    inb && c != "" {
      if ($0 ~ pre) next
      v=$0; sub(/^[a-zA-Z0-9_-]+[[:space:]]*=[[:space:]]*/, "", v)
      printf "%s\037%s\n", c, v
      c=""
    }
  ' "$GHOSTTY_CONFIG"
}

clear_categories_under() {
  local prefix="$1" us cat value
  [ -f "$GHOSTTY_CONFIG" ] || return 0
  [ "$(_marker_fault "$GHOSTTY_CONFIG")" = "ok" ] || return 0
  us="$(printf '\037')"
  _lock_acquire || return 1
  while IFS="$us" read -r cat value; do
    [ -n "$cat" ] || continue
    case "$value" in
      *"$prefix"*) ;;
      *)
        # A saved custom preset is a file of its own that names the pack file
        # from inside itself, so the managed block's line never mentions the
        # prefix at all. Removing a pack under one of those left the preset —
        # still applied — pointing at a file that no longer existed, and
        # Ghostty answers a missing config-file by abandoning the entire
        # config. One level of include is as deep as save_current_as ever
        # writes, so one level is what this follows.
        [ -f "$value" ] && grep -qF "$prefix" "$value" || continue
        ;;
    esac
    clear_category "$cat" || continue
    t "  （已從 Ghostty 設定移除仍在套用的 ${cat}）" \
      "  (removed the still-applied ${cat} from your Ghostty config)"
  done < <(_managed_category_values)
  _lock_release
}

# preview_directive VALUE KIND KEY [SHADER_SRC] — the directive lines a
# selection would write, for the TUI to drop into a throwaway config and open a
# preview window with. Deliberately shares _directive_line with the real write
# so a preview cannot show something the apply would not produce, and touches
# $GHOSTTY_CONFIG nowhere: previewing must never alter what the user is running.
preview_directive() {
  local value="$1" kind="$2" key="$3" shader_src="${4:-}"
  # A shader theme names ~/.config/ghostty/shaders/<name>.glsl by absolute path,
  # so the preview window looks for it there and finds nothing unless the copy
  # happens before the preview config is written, exactly as on a real apply.
  if [ -n "$shader_src" ]; then
    mkdir -p "$GHOSTTY_SHADERS"
    cp "$shader_src" "$GHOSTTY_SHADERS/"
  fi
  _directive_line "$value" "$kind" "$key"
}

set_path_for() {
  local category="$1" value="$2" kind="${3:-file}" key="${4:-}"
  local tmp dir_f rc
  # Lock first: the branch below trusts _marker_fault, so the check has to
  # happen where a concurrent writer cannot change the answer underneath it.
  _lock_acquire || return 1
  _require_sane_markers || { _lock_release; return 1; }
  mkdir -p "$GHOSTTY_DIR"
  tmp="$(mktemp)"; dir_f="$(mktemp)"
  _write_directive "$dir_f" "$value" "$kind" "$key"

  if [ "$(_marker_fault "$GHOSTTY_CONFIG")" = "ok" ]; then
    GCS_TAG="# category:$category" GCS_DIRFILE="$dir_f" \
    awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v pre="$_PREAMBLE_RE" '
      BEGIN {tag=ENVIRON["GCS_TAG"]; cfgfile=ENVIRON["GCS_DIRFILE"]}
      function emit(   l) { while ((getline l < cfgfile) > 0) print l; close(cfgfile) }
      $0==b {print; if (!done) inb=1; next}
      $0==e {
        if (inb && !wrote) { print tag; emit() }
        print; if (inb) done=1; inb=0; next
      }
      inb {
        if ($0==tag) { print tag; emit(); wrote=1; skip=1; next }
        if (skip) { if ($0 ~ pre) next; skip=0; next }
        print; next
      }
      { print }
    ' "$GHOSTTY_CONFIG" > "$tmp"
  else
    [ -f "$GHOSTTY_CONFIG" ] && cat "$GHOSTTY_CONFIG" > "$tmp"
    { [ -s "$tmp" ] && echo; echo "$BEGIN_MARK"; echo "# category:$category"; cat "$dir_f"; echo "$END_MARK"; } >> "$tmp"
  fi
  rm -f "$dir_f"
  rc=0
  _install_file "$tmp" "$GHOSTTY_CONFIG" || rc=1
  rm -f "$tmp"
  [ "$rc" -eq 0 ] && { _sync_overrides || rc=1; }
  _lock_release
  return "$rc"
}

# Presets are complete standalone configs (theme+font+cursor already combined) —
# picking one replaces the whole managed block instead of adding a "preset"
# pair alongside a possibly-conflicting leftover theme/font pair.
set_solo_path_for() {
  local category="$1" value="$2" kind="${3:-file}" key="${4:-}"
  local tmp dir_f rc
  # Lock first: the branch below trusts _marker_fault, so the check has to
  # happen where a concurrent writer cannot change the answer underneath it.
  _lock_acquire || return 1
  _require_sane_markers || { _lock_release; return 1; }
  mkdir -p "$GHOSTTY_DIR"
  tmp="$(mktemp)"; dir_f="$(mktemp)"
  _write_directive "$dir_f" "$value" "$kind" "$key"

  if [ "$(_marker_fault "$GHOSTTY_CONFIG")" = "ok" ]; then
    GCS_TAG="# category:$category" GCS_DIRFILE="$dir_f" \
    awk -v b="$BEGIN_MARK" -v e="$END_MARK" '
      BEGIN {tag=ENVIRON["GCS_TAG"]; cfgfile=ENVIRON["GCS_DIRFILE"]}
      function emit(   l) { while ((getline l < cfgfile) > 0) print l; close(cfgfile) }
      $0==b {print; if (done) next; print tag; emit(); inb=1; next}
      $0==e {print; if (inb) done=1; inb=0; next}
      inb {next}
      {print}
    ' "$GHOSTTY_CONFIG" > "$tmp"
  else
    [ -f "$GHOSTTY_CONFIG" ] && cat "$GHOSTTY_CONFIG" > "$tmp"
    { [ -s "$tmp" ] && echo; echo "$BEGIN_MARK"; echo "# category:$category"; cat "$dir_f"; echo "$END_MARK"; } >> "$tmp"
  fi
  rm -f "$dir_f"
  rc=0
  _install_file "$tmp" "$GHOSTTY_CONFIG" || rc=1
  rm -f "$tmp"
  [ "$rc" -eq 0 ] && { _sync_overrides || rc=1; }
  _lock_release
  return "$rc"
}

show_current() {
  local fault
  fault="$(_marker_fault "$GHOSTTY_CONFIG")"
  if [ "$fault" = "ok" ]; then
    t "目前的 ghostty-picker 選擇：" "Current ghostty-picker selections:"
    awk -v b="$BEGIN_MARK" -v e="$END_MARK" \
      '$0==b{if (!done) inb=1; next} $0==e{if (inb) done=1; inb=0; next} inb' "$GHOSTTY_CONFIG"
  elif [ "$fault" = "none" ]; then
    t "還沒有任何 ghostty-picker 選擇。" "No ghostty-picker selections set yet."
  else
    # Read-only, so this reports rather than refuses — but it must not answer
    # "nothing selected" for a block it simply cannot parse.
    _require_sane_markers || true
    return 1
  fi
}
