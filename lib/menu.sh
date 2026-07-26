#!/usr/bin/env bash
# Shared numbered-menu picker + non-destructive config-file writer.
# Sourced by ghostty-theme / ghostty-font / ghostty-preset. Not run directly.
set -euo pipefail

GHOSTTY_DIR="${GHOSTTY_DIR:-$HOME/.config/ghostty}"
GHOSTTY_CONFIG="$GHOSTTY_DIR/config"
GHOSTTY_SHADERS="$GHOSTTY_DIR/shaders"

# Imported asset packs live in a user-owned directory, never next to the
# scripts. A Homebrew install puts the scripts under a Cellar prefix that
# `brew upgrade` replaces wholesale, so anything written there would silently
# vanish on the next version bump — and on Intel prefixes it may not even be
# writable. This location survives upgrades, `git pull`, and deleting the
# clone entirely.
STUDIO_DIR="${GHOSTTY_STUDIO_DIR:-$HOME/.config/ghostty-config-studio}"
STUDIO_ASSETS="$STUDIO_DIR/assets"

# Language is shared with the TUI: both read the same file, so toggling with
# [L] inside ghostty-tui also switches what these commands print. One switch
# for the whole tool rather than two that can disagree.
LANG_FILE="$GHOSTTY_DIR/.ghostty-tui-lang"
studio_lang() {
  if [ -r "$LANG_FILE" ] && [ "$(tr -d '[:space:]' < "$LANG_FILE")" = "en" ]; then
    echo en
  else
    echo zh
  fi
}
# t ZH EN — prints whichever the current language calls for.
t() { if [ "$(studio_lang)" = "en" ]; then echo "$2"; else echo "$1"; fi; }
BEGIN_MARK="# >>> ghostty-picker managed >>>"
END_MARK="# <<< ghostty-picker managed <<<"

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
_directive_line() {
  local value="$1" kind="${2:-file}" key="${3:-}"
  case "$kind" in
    name)   echo "theme = $value" ;;
    shader) echo "custom-shader = $value" ;;
    raw)    echo "$key = $value" ;;
    *)      echo "config-file = $value" ;;
  esac
}

# Prints the value currently recorded for a category (empty if none set).
# Strips generically ("<anything> = ") rather than a fixed list of known
# directive names, since kind=raw's key varies per category.
current_path_for() {
  local category="$1"
  [ -f "$GHOSTTY_CONFIG" ] || return 0
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v tag="# category:$category" '
    $0==b {inb=1; next} $0==e {inb=0; next}
    inb && $0==tag {want=1; next}
    inb && want {
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
  local category="$1"
  [ -f "$GHOSTTY_CONFIG" ] || return 0
  grep -qF "$BEGIN_MARK" "$GHOSTTY_CONFIG" || return 0
  local tmp
  tmp="$(mktemp)"
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v tag="# category:$category" '
    $0==b {print; inb=1; next}
    $0==e {print; inb=0; next}
    inb {
      if ($0==tag) { skip=1; next }
      if (skip) { skip=0; next }
      print; next
    }
    { print }
  ' "$GHOSTTY_CONFIG" > "$tmp"
  mv "$tmp" "$GHOSTTY_CONFIG"
}

# clear_categories_under PREFIX — clears every managed-block category whose
# value points inside PREFIX. Used before deleting asset files: a reference
# left pointing at a removed file makes Ghostty fail to open the include and
# fall back to defaults for the whole config.
clear_categories_under() {
  local prefix="$1"
  [ -f "$GHOSTTY_CONFIG" ] || return 0
  local cat
  for cat in $(awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v p="$prefix" '
      $0==b {inb=1; next} $0==e {inb=0; next}
      inb && /^# category:/ { c=substr($0, 12); next }
      inb && c != "" && index($0, p) { print c; c="" }
    ' "$GHOSTTY_CONFIG"); do
    clear_category "$cat"
    t "  （已從 Ghostty 設定移除仍在套用的 $cat）" \
      "  (removed the still-applied $cat from your Ghostty config)"
  done
}

set_path_for() {
  local category="$1" value="$2" kind="${3:-file}" key="${4:-}"
  mkdir -p "$GHOSTTY_DIR"
  local tmp directive
  tmp="$(mktemp)"
  directive="$(_directive_line "$value" "$kind" "$key")"

  if [ -f "$GHOSTTY_CONFIG" ] && grep -qF "$BEGIN_MARK" "$GHOSTTY_CONFIG"; then
    awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v tag="# category:$category" -v cfgline="$directive" '
      $0==b {print; inb=1; next}
      $0==e {
        if (!wrote) { print tag; print cfgline }
        print; inb=0; next
      }
      inb {
        if ($0==tag) { print tag; print cfgline; wrote=1; skip=1; next }
        if (skip) { skip=0; next }
        print; next
      }
      { print }
    ' "$GHOSTTY_CONFIG" > "$tmp"
  else
    [ -f "$GHOSTTY_CONFIG" ] && cat "$GHOSTTY_CONFIG" > "$tmp"
    { [ -s "$tmp" ] && echo; echo "$BEGIN_MARK"; echo "# category:$category"; echo "$directive"; echo "$END_MARK"; } >> "$tmp"
  fi
  mv "$tmp" "$GHOSTTY_CONFIG"
}

# Presets are complete standalone configs (theme+font+cursor already combined) —
# picking one replaces the whole managed block instead of adding a "preset"
# pair alongside a possibly-conflicting leftover theme/font pair.
set_solo_path_for() {
  local category="$1" value="$2" kind="${3:-file}" key="${4:-}"
  mkdir -p "$GHOSTTY_DIR"
  local tmp directive
  tmp="$(mktemp)"
  directive="$(_directive_line "$value" "$kind" "$key")"

  if [ -f "$GHOSTTY_CONFIG" ] && grep -qF "$BEGIN_MARK" "$GHOSTTY_CONFIG"; then
    awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v tag="# category:$category" -v cfgline="$directive" '
      $0==b {print; print tag; print cfgline; inb=1; next}
      $0==e {print; inb=0; next}
      inb {next}
      {print}
    ' "$GHOSTTY_CONFIG" > "$tmp"
  else
    [ -f "$GHOSTTY_CONFIG" ] && cat "$GHOSTTY_CONFIG" > "$tmp"
    { [ -s "$tmp" ] && echo; echo "$BEGIN_MARK"; echo "# category:$category"; echo "$directive"; echo "$END_MARK"; } >> "$tmp"
  fi
  mv "$tmp" "$GHOSTTY_CONFIG"
}

# apply_selection CATEGORY VALUE KIND [SHADER_SRC] [KEY]
# Standalone entry point (also used internally by run_picker's _apply_choice)
# so the Bubble Tea TUI can apply a selection by shelling out to
# `bash -c 'source lib/menu.sh; apply_selection ...'` instead of duplicating
# the managed-block logic in Go. KEY is only needed for kind=raw.
apply_selection() {
  local category="$1" value="$2" kind="${3:-file}" shader_src="${4:-}" key="${5:-}"
  if [ -n "$shader_src" ]; then
    mkdir -p "$GHOSTTY_SHADERS"
    cp "$shader_src" "$GHOSTTY_SHADERS/"
    # ponytail: copies the shader file on every selection instead of diffing
    # first; fine at 12 small .glsl files, add a checksum skip if this grows.
  fi
  # Presets and custom saved presets are complete standalone combos —
  # picking one replaces the whole managed block. Every other category
  # (theme/font/cursor/raw settings like opacity/blur/cursor-style/...)
  # stacks independently alongside each other.
  if [ "$category" = "preset" ] || [ "$category" = "custom" ]; then
    set_solo_path_for "$category" "$value" "$kind" "$key"
  else
    set_path_for "$category" "$value" "$kind" "$key"
  fi
}

# save_current_as DEST — snapshot whatever's currently active (across every
# category: theme/font/cursor/...) into DEST as a standalone preset file,
# by copying out the managed block's own directive lines. Applying DEST
# later just re-references the same underlying vendored files/built-in
# names — nothing is duplicated or flattened.
save_current_as() {
  local dest="$1"
  if [ ! -f "$GHOSTTY_CONFIG" ] || ! grep -qF "$BEGIN_MARK" "$GHOSTTY_CONFIG"; then
    echo "Nothing currently selected to save — pick a theme/font/cursor first." >&2
    return 1
  fi
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" '
    $0==b {inb=1; next} $0==e {inb=0; next}
    inb && $0 !~ /^# category:/ { print }
  ' "$GHOSTTY_CONFIG" > "$dest"
}

show_current() {
  if [ -f "$GHOSTTY_CONFIG" ] && grep -qF "$BEGIN_MARK" "$GHOSTTY_CONFIG"; then
    t "目前的 ghostty-picker 選擇：" "Current ghostty-picker selections:"
    awk -v b="$BEGIN_MARK" -v e="$END_MARK" '$0==b{inb=1;next} $0==e{inb=0;next} inb' "$GHOSTTY_CONFIG"
  else
    t "還沒有任何 ghostty-picker 選擇。" "No ghostty-picker selections set yet."
  fi
}

# run_picker CATEGORY LABEL [DIRECT_NAME]
# Reads caller-populated parallel arrays: names[] descs[] paths[] shader_srcs[]
# (shader_srcs[i] empty means "no shader to copy for this entry"), optionally
# kinds[] ("file" per-entry default, "name" for a Ghostty built-in theme, or
# "raw" for a scalar setting), optionally keys[] (the actual Ghostty config
# key, only meaningful for kind=raw entries), optionally categories[] (lets
# ONE command's flat menu mix several independent settings — e.g. opacity
# AND blur AND padding in one list — each tagged with its own category so
# picking one doesn't clobber the others; falls back to the CATEGORY param
# for any index left unset, which is how every other command keeps working
# unchanged), and optionally sources[] (the github username each entry came
# from, e.g. "snedea" — display-only, shown as "source/name" in the menu;
# direct-name switching still matches on the bare names[] value so
# `ghostty-theme campfire` keeps working unchanged).
# LABEL is the plural display heading ("Theme"/"Font"/"Preset") — passed explicitly
# rather than derived (macOS ships bash 3.2; ${var^} needs bash 4+).
run_picker() {
  local category="$1" label="$2" direct="${3:-}"
  local n=${#names[@]}

  # A blank workbench is a supported state: the asset packs are optional and
  # imported on demand. Say so rather than presenting an empty "[1-0]" menu.
  if [ "$n" -eq 0 ]; then
    t "沒有可用的${label}項目。" "No ${label} entries available." >&2
    t "這些素材是選用的，執行 ghostty-setup 可以挑要匯入哪些設定檔集。" \
      "These packs are optional. Run ghostty-setup to pick which ones to import." >&2
    return 1
  fi

  _cat_for() { echo "${categories[$1]:-$category}"; }

  _apply_choice() {
    local idx="$1"
    local kind="${kinds[$idx]:-file}"
    local cat; cat="$(_cat_for "$idx")"
    apply_selection "$cat" "${paths[$idx]}" "$kind" "${shader_srcs[$idx]:-}" "${keys[$idx]:-}"
    t "已切換 $cat：${names[$idx]} -- ${descs[$idx]}" \
      "Switched $cat to: ${names[$idx]} -- ${descs[$idx]}"
  }

  if [ -n "$direct" ]; then
    local i
    for i in "${!names[@]}"; do
      if [ "${names[$i]}" = "$direct" ]; then _apply_choice "$i"; return 0; fi
    done
    t "找不到 $category：$direct" "Unknown $category: $direct" >&2
    t "可用的有：${names[*]}" "Available: ${names[*]}" >&2
    return 1
  fi

  t "Ghostty ${label}" "Ghostty ${label}s"
  echo "──────────────"
  local i marker display cat cur
  for i in "${!names[@]}"; do
    cat="$(_cat_for "$i")"
    cur="$(current_path_for "$cat")"
    marker="  "
    [ -n "$cur" ] && [ "${paths[$i]}" = "$cur" ] && marker="▸ "
    if [ -n "${sources[$i]:-}" ]; then
      display="${sources[$i]}/${names[$i]}"
    else
      display="${names[$i]}"
    fi
    printf "  %s%d) %-32s %s\n" "$marker" "$((i + 1))" "$display" "${descs[$i]}"
  done
  echo ""
  if [ "$(studio_lang)" = "en" ]; then
    printf "Pick a %s [1-%d]: " "$category" "$n"
  else
    printf "選一個 %s [1-%d]： " "$category" "$n"
  fi
  read -r choice

  if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "$n" ]; then
    t "已取消。" "Cancelled."
    return 1
  fi
  _apply_choice $((choice - 1))
}
