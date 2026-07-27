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
HISTORY_DIR="$STUDIO_DIR/history"
# The macOS second config lives here. Overridable only so the behaviour suite can
# plant a fake one: a test that asserts this tool edits the shadowing file must
# never be able to reach the real ~/Library copy.
GHOSTTY_SUPPORT_DIR="${GHOSTTY_SUPPORT_DIR:-$HOME/Library/Application Support/com.mitchellh.ghostty}"

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

# One writer at a time. Two applies racing each other is not theoretical — the
# TUI shells out to apply_selection per selection and the whole config is a
# read-modify-write, which measured five lost writes out of five, both
# processes reporting success while only one survived. Worse, the loser's
# validation then failed against a file the winner had already replaced and
# rolled back to a snapshot predating both, wiping the managed block entirely.
#
# The lock IS a symlink whose target is the holder's pid. macOS ships no
# flock(1), and the obvious `mkdir` lock needs a second step to record who holds
# it — a waiter that looks in the gap between the mkdir and the pid write finds
# an ownerless lock, calls it abandoned, deletes it and takes a lock somebody
# already has. Measured: two lost writes in twenty-five with that shape, even
# with the staleness check held off for a full second. symlink(2) is one call
# that both tests and sets and carries the pid in the same operation, so the
# gap does not exist.
#
# The pid is what makes a killed holder recoverable: ^C during an apply must
# not wedge the tool forever, so a lock whose pid is gone is reclaimed rather
# than waited on. Re-entrant by depth count, because clear_categories_under
# calls clear_category in a loop and both take it.
LOCK_LINK="$STUDIO_DIR/.write-lock"
_LOCK_DEPTH=0

_lock_acquire() {
  local tries holder
  if [ "$_LOCK_DEPTH" -gt 0 ]; then
    _LOCK_DEPTH=$((_LOCK_DEPTH + 1))
    return 0
  fi
  mkdir -p "$STUDIO_DIR" 2>/dev/null || true
  tries=0
  while ! ln -s "$$" "$LOCK_LINK" 2>/dev/null; do
    tries=$((tries + 1))
    if [ "$tries" -gt 300 ]; then
      t "另一個 ghostty-config-studio 正在寫入設定檔，等待逾時，沒有任何變更。" \
        "Another ghostty-config-studio is writing the config; timed out waiting, nothing was changed." >&2
      return 1
    fi
    holder="$(readlink "$LOCK_LINK" 2>/dev/null)" || holder=""
    # Empty means it was released between the failed ln and this read — just
    # go round again. kill -0 also fails for a live process owned by somebody
    # else; this is a single-user dotfile tool, so counting that as abandoned
    # is the right trade. Refusing forever on a lock nobody can prove is alive
    # is the worse failure.
    if [ -n "$holder" ] && ! kill -0 "$holder" 2>/dev/null; then
      rm -f "$LOCK_LINK" 2>/dev/null || true
      continue
    fi
    sleep 0.1
  done
  _LOCK_DEPTH=1
  return 0
}

_lock_release() {
  [ "$_LOCK_DEPTH" -gt 0 ] || return 0
  _LOCK_DEPTH=$((_LOCK_DEPTH - 1))
  # Only ever drop our own: a lock somebody else has already reclaimed after
  # deciding we were gone is theirs, not ours to delete.
  if [ "$_LOCK_DEPTH" -eq 0 ] && [ "$(readlink "$LOCK_LINK" 2>/dev/null)" = "$$" ]; then
    rm -f "$LOCK_LINK" 2>/dev/null || true
  fi
  return 0
}

# This file is sourced, so the trap belongs to the entry-point script. Every
# entry point sources it before doing anything else. The lock has to survive an
# errexit death or a ^C, which is precisely when it would otherwise be left
# behind for the next run to reclaim.
_lock_release_all() { _LOCK_DEPTH=1; _lock_release; }
trap _lock_release_all EXIT

# Installing a rewritten config with `mv` from mktemp replaces the destination
# INODE, so the file comes back as mktemp's 0600, a symlinked config is turned
# into a regular file (leaving the dotfiles copy it pointed at silently stale),
# and a config the user deliberately made read-only is overwritten without a
# word. Measured on a same-device rename, so it is the inode swap that does it,
# not mv's cross-device copy fallback. Copying the bytes through the existing
# file keeps its mode, its owner and its symlink, and fails honestly when it is
# not writable.
_install_file() {
  local src="$1" dest="$2"
  cp "$src" "$dest" 2>/dev/null && return 0
  t "無法寫入 ${dest}（唯讀或權限不足），沒有任何變更。" \
    "Could not write ${dest} (read-only or not permitted); nothing was changed." >&2
  return 1
}

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

# The snapshot this process took most recently, so a failed apply can put back
# exactly the file it replaced rather than whatever happens to be newest in the
# history directory — a second terminal applying at the same time would
# otherwise have its snapshot rolled back by ours.
_LAST_SNAPSHOT=""

# A ".missing" snapshot records that there was no config at all, which is not
# the same state as an empty one: restoring it has to delete the file, because
# leaving an empty config behind still counts as a config to Ghostty.
_restore_snapshot_bytes() {
  local snapshot="$1"
  [ -n "$snapshot" ] && [ -e "$snapshot" ] || return 1
  mkdir -p "$GHOSTTY_DIR"
  case "$snapshot" in
    *.missing) rm -f "$GHOSTTY_CONFIG" ;;
    *)         _install_file "$snapshot" "$GHOSTTY_CONFIG" || return 1 ;;
  esac
  return 0
}

# Putting the bytes back is only half of it: the generated override include is
# derived from the block, so restoring an older block without regenerating the
# include would leave Ghostty resolving the newer values through a file the
# restored config still points at. Hence _sync_overrides here — undo needs it.
# The rollback path in apply_selection needs the bare _restore_snapshot_bytes
# as a fallback, because _sync_overrides is also the step that can turn a good
# snapshot straight back into the invalid file it was meant to undo.
_restore_snapshot() {
  local snapshot="$1" consume="${2:-0}"
  _restore_snapshot_bytes "$snapshot" || return 1
  [ "$consume" = "1" ] && rm -f "$snapshot"
  _sync_overrides
  return 0
}

# Nothing in here may fail the caller: a history directory that cannot be
# written is a worse reason to refuse a theme change than to lose the undo.
# Every failure path returns 0, and the empty _LAST_SNAPSHOT tells apply_selection
# there is nothing to roll back to.
snapshot_config() {
  local ts prefix idx pad snapshot kept f
  _LAST_SNAPSHOT=""
  ts="$(date +%Y%m%d-%H%M%S 2>/dev/null)" || return 0
  mkdir -p "$HISTORY_DIR" 2>/dev/null || return 0
  # date only resolves to the second, so two applies inside the same second
  # would land on the same name. The counter breaks the tie and is zero-padded
  # because the readers below order these by plain lexical sort, where "10"
  # would otherwise come before "9".
  prefix="$HISTORY_DIR/$ts-"
  idx=0
  while :; do
    pad="$(printf '%04d' "$idx")" || return 0
    if [ ! -e "${prefix}${pad}.conf" ] && [ ! -e "${prefix}${pad}.missing" ]; then
      break
    fi
    idx=$((idx + 1))
  done
  if [ -f "$GHOSTTY_CONFIG" ]; then
    snapshot="${prefix}${pad}.conf"
    cp "$GHOSTTY_CONFIG" "$snapshot" 2>/dev/null || return 0
  else
    snapshot="${prefix}${pad}.missing"
    : > "$snapshot" 2>/dev/null || return 0
  fi
  _LAST_SNAPSHOT="$snapshot"

  # Reverse lexical order is newest-first given the name format above, so
  # anything past the twentieth is older than the window worth keeping.
  # Only snapshots are counted and dropped. resolve_shadow_conflicts also parks
  # .bak copies of the user's Application Support config in here, and silently
  # deleting the only copy of a file they hand-wrote is not a retention policy.
  kept=0
  for f in $(
    LC_ALL=C ls -1r "$HISTORY_DIR" 2>/dev/null | grep -E '\.(conf|missing)$' || true
  ); do
    kept=$((kept + 1))
    if [ "$kept" -gt 20 ]; then
      rm -f "$HISTORY_DIR/$f"
    fi
  done
  return 0
}

# Prints one line and one line only on success: the TUI puts it straight into a
# single-row status bar, so a second line would push the layout out of shape.
undo_last_apply() {
  local latest base rc
  _lock_acquire || return 1
  latest=""
  if [ -d "$HISTORY_DIR" ]; then
    # Snapshots only: a resolve_shadow_conflicts .bak sorts newest and would
    # otherwise be restored straight over GHOSTTY_CONFIG — the wrong file
    # entirely. grep matching nothing exits 1, which pipefail would turn into a
    # dead shell inside this substitution.
    latest="$(
      {
        LC_ALL=C ls -1 "$HISTORY_DIR" 2>/dev/null | grep -E '\.(conf|missing)$' || true
      } | tail -n 1
    )"
  fi
  if [ -z "$latest" ]; then
    t "沒有可以還原的紀錄。" "Nothing to undo." >&2
    _lock_release
    return 1
  fi
  base="$HISTORY_DIR/$latest"
  if ! _restore_snapshot "$base" 1; then
    t "還原失敗：${latest}" "Could not restore: ${latest}" >&2
    _lock_release
    return 1
  fi
  rc=0
  case "$latest" in
    *.missing) t "已還原：套用前沒有設定檔，已將它移除。" \
                 "Restored: there was no config before this, removed it." ;;
    *)         t "已還原 ${latest%.conf} 當時的設定檔。" \
                 "Restored the config as it was at ${latest%.conf}." ;;
  esac
  _lock_release
  return "$rc"
}

validate_config() {
  local ghostty_bin out status
  ghostty_bin="$(command -v ghostty 2>/dev/null)" || ghostty_bin=""
  # The app does not put itself on PATH, so on a normal macOS install this
  # second location is the one that actually hits.
  if [ -z "$ghostty_bin" ] && [ -x /Applications/Ghostty.app/Contents/MacOS/ghostty ]; then
    ghostty_bin=/Applications/Ghostty.app/Contents/MacOS/ghostty
  fi
  # Not being able to validate is not a reason to refuse the write.
  [ -n "$ghostty_bin" ] || return 0
  # A config-file that does not exist exits 1 with no message at all, which
  # would read as "your config is broken" for someone who simply has not made
  # one yet.
  [ -f "$GHOSTTY_CONFIG" ] || return 0
  # +show-config exits clean when an include is missing and never mentions it,
  # so it cannot tell you the config is broken; +validate-config reports
  # `error opening config-file …: error.FileNotFound`. See DESIGN_NOTES.md.
  # It prints that on stdout, not stderr, and its stderr additionally carries
  # unrelated Sentry init noise on some machines — hence capturing stdout and
  # relaying that as the error.
  out="$("$ghostty_bin" +validate-config --config-file="$GHOSTTY_CONFIG" 2>/dev/null)" && status=0 || status=$?
  [ "$status" -eq 0 ] && return 0
  # Progress output is carriage-return terminated and would overprint the
  # message if it were passed through as-is.
  [ -n "$out" ] && printf '%s\n' "$out" | tr '\r' '\n' >&2
  return "$status"
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

_override_file_path() { echo "$STUDIO_DIR/generated/explicit-overrides.conf"; }

_replace_if_changed() {
  local dest="$1" src="$2" rc
  if [ -e "$dest" ] && cmp -s "$dest" "$src"; then
    rm -f "$src"
    return 0
  fi
  rc=0
  _install_file "$src" "$dest" || rc=1
  rm -f "$src"
  return "$rc"
}

_write_override_header() {
  local out="$1" mode="${2:-active}" summary
  if [ "$mode" = "preset" ]; then
    summary="$(t "由 ghostty-config-studio 產生，重新儲存這個預設時會自動重寫。" \
      "Generated by ghostty-config-studio; rewritten automatically whenever this preset is re-saved.")"
  else
    summary="$(t "由 ghostty-config-studio 產生，套用選擇時會自動重寫。" \
      "Generated by ghostty-config-studio; rewritten automatically on every apply.")"
  fi
  {
    echo "# $summary"
    echo "# $(t "Ghostty 會讓後面的 config-file 蓋過父檔的直接指定，所以明確的純量設定要放在最後的 include 才能壓過主題。" \
      "Ghostty lets a later config-file override direct assignments in the parent file, so explicit scalar settings live in this last include to beat theme values.")"
  } > "$out"
}

# Ghostty lets a later include beat direct assignments, but repeatable keys
# append rather than replace. Hoisting those into the override include would
# change meaning — most visibly by demoting the chosen font to a fallback.
_raw_lines_from_block() {
  [ -f "$GHOSTTY_CONFIG" ] || return 0
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v pre="$_PREAMBLE_RE" '
    function excluded(k) {
      return k == "config-file" || k == "custom-shader" || k == "theme" || \
             k == "font-family" || k == "font-family-bold" || \
             k == "font-family-italic" || k == "font-family-bold-italic" || \
             k == "palette" || k == "keybind" || k == "font-feature"
    }
    $0==b {if (!done) inb=1; next}
    $0==e {if (inb) done=1; inb=0; next}
    inb && /^# category:/ { cat=substr($0, 12); next }
    inb && cat != "" {
      if ($0 ~ pre) next
      if (cat != "_overrides") {
        key=$0
        sub(/^[[:space:]]*/, "", key)
        sub(/[[:space:]]*=.*/, "", key)
        if (!excluded(key)) print
      }
      cat=""
    }
  ' "$GHOSTTY_CONFIG"
}

# The include line goes through the environment, not `awk -v`: -v runs escape
# processing over its value, so a backslash anywhere in $STUDIO_DIR came out
# the other side collapsed — `stu\dio` written as `studio`. That is a
# config-file pointing at a path that does not exist, which is the silent
# whole-config fallback this project exists to avoid.
_rewrite_block_with_overrides() {
  local out="$1" include_line="${2:-}"
  GCS_OVERRIDE_LINE="$include_line" \
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" -v tag="# category:_overrides" -v pre="$_PREAMBLE_RE" '
    BEGIN {line=ENVIRON["GCS_OVERRIDE_LINE"]}
    $0==b {print; if (!done) inb=1; next}
    $0==e {
      if (inb && line != "") { print tag; print line }
      print; if (inb) done=1; inb=0; next
    }
    inb {
      if ($0==tag) { skip=1; next }
      if (skip) { if ($0 ~ pre) next; skip=0; next }
      print; next
    }
    { print }
  ' "$GHOSTTY_CONFIG" > "$out"
}

_managed_directives_without_overrides() {
  awk -v b="$BEGIN_MARK" -v e="$END_MARK" '
    $0==b {if (!done) inb=1; next}
    $0==e {if (inb) done=1; inb=0; next}
    inb && $0=="# category:_overrides" { skip=1; next }
    inb && skip { skip=0; next }
    inb && $0 !~ /^# category:/ { print }
  ' "$GHOSTTY_CONFIG"
}

# Keyed on the whole destination path, not just its basename. Two presets named
# the same in different directories used to share one sidecar, so saving the
# second silently rewrote the raw settings the first one applies. cksum rather
# than a hash tool because it is the one that is guaranteed present.
_preset_sidecar_path() {
  local dest="$1" base tag
  base="${dest##*/}"
  base="${base%.conf}"
  tag="$(printf '%s' "$dest" | cksum | awk '{print $1}')"
  echo "$STUDIO_DIR/generated/presets/${base}-${tag}.conf"
}

_sync_overrides() {
  local override_path raw_f cfg_tmp gen_tmp fault rc
  override_path="$(_override_file_path)"
  [ -f "$GHOSTTY_CONFIG" ] || { rm -f "$override_path"; return 0; }
  fault="$(_marker_fault "$GHOSTTY_CONFIG")"
  [ "$fault" = "none" ] && { rm -f "$override_path"; return 0; }
  # A block shape the rewriter cannot represent. Leave both the config and the
  # generated include exactly as they are rather than rewriting half of one.
  # apply_selection refuses these up front, so the only way here is the undo
  # path — where the user is entitled to restore a config this tool would not
  # itself have written, and must not have it silently mangled on the way back.
  [ "$fault" = "ok" ] || return 0

  raw_f="$(mktemp)"
  _raw_lines_from_block > "$raw_f"

  cfg_tmp="$(mktemp)"
  if [ ! -s "$raw_f" ]; then
    _rewrite_block_with_overrides "$cfg_tmp"
    rc=0
    _replace_if_changed "$GHOSTTY_CONFIG" "$cfg_tmp" || rc=1
    rm -f "$raw_f"
    # Only after the reference is gone from the config. Deleting the include
    # first would leave a window where the config points at a file that is not
    # there, and a Ghostty reading it in that window loses the whole config.
    [ "$rc" -eq 0 ] && rm -f "$override_path"
    return "$rc"
  fi

  mkdir -p "${override_path%/*}"
  gen_tmp="$(mktemp)"
  _write_override_header "$gen_tmp" active
  cat "$raw_f" >> "$gen_tmp"
  rm -f "$raw_f"
  # The include file has to exist before the config names it, for the same
  # reason.
  _replace_if_changed "$override_path" "$gen_tmp" || return 1

  _rewrite_block_with_overrides "$cfg_tmp" "config-file = $override_path"
  _replace_if_changed "$GHOSTTY_CONFIG" "$cfg_tmp" || return 1
  return 0
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

# apply_selection CATEGORY VALUE KIND [SHADER_SRC] [KEY]
# Standalone entry point (also used internally by run_picker's _apply_choice)
# so the Bubble Tea TUI can apply a selection by shelling out to
# `bash -c 'source lib/menu.sh; apply_selection ...'` instead of duplicating
# the managed-block logic in Go. KEY is only needed for kind=raw.
# Ghostty reads more than one config on macOS. Besides ~/.config/ghostty/config
# it also loads two files from ~/Library/Application Support/com.mitchellh.ghostty:
# `config` and `config.ghostty`, and those are applied AFTER, so every key they
# set beats whatever we just wrote. NOT a `config*` glob, whatever an earlier
# version of this comment claimed — measured with a sandboxed CFFIXED_USER_HOME,
# a `config.bak-…` sitting in that directory is not read at all, which is worth
# knowing before anyone "fixes" the loop below into a wildcard and starts
# reporting conflicts against backup files. A selection then appears to do nothing at all, with no error anywhere
# to explain it. Rather than succeed silently, say which file is winning and
# over which keys.
_shadow_configs() {
  local d="$GHOSTTY_SUPPORT_DIR"
  local f
  for f in "$d"/config "$d"/config.ghostty; do
    [ -f "$f" ] && echo "$f"
  done
}

# The keys a selection actually decides. `theme = X` is expanded, because the
# name alone never appears in the other file while the colors it stands for do.
_keys_decided_by() {
  local value="$1" kind="$2" key="$3"
  case "$kind" in
    raw)    echo "$key" ;;
    name)   echo "theme background foreground palette" ;;
    shader) echo "custom-shader" ;;
    *)
      [ -f "$value" ] || return 0
      # `tr -d ' ='` left a TAB attached to the key when the line was
      # tab-indented, so `\tcursor-style` never matched the plain
      # `cursor-style` in the shadowing config and the conflict went
      # unreported — the selection then silently did nothing, which is the
      # exact failure warn_if_shadowed exists to catch.
      sed -e 's/#.*//' "$value" | grep -oE '^[[:space:]]*[a-z0-9-]+[[:space:]]*=' \
        | tr -d '[:space:]=' | sort -u
      if grep -qE '^[[:space:]]*theme[[:space:]]*=' "$value"; then
        printf 'background\nforeground\npalette\n'
      fi
      ;;
  esac
  return 0
}

warn_if_shadowed() {
  local value="$1" kind="$2" key="$3"
  local other keys hits k
  keys="$(_keys_decided_by "$value" "$kind" "$key" | sort -u)"
  [ -n "$keys" ] || return 0
  # Read the paths line by line: "Application Support" has a space in it, and
  # word-splitting a command substitution would tear it in half.
  while IFS= read -r other; do
    [ -n "$other" ] || continue
    hits=""
    for k in $keys; do
      grep -qE "^[[:space:]]*$k[[:space:]]*=" "$other" && hits="$hits $k"
    done
    [ -n "$hits" ] || continue
    echo >&2
    t "⚠  這些設定不會生效：$hits" "⚠  These settings will not take effect:$hits" >&2
    t "   Ghostty 也會讀 ${other}，而且是在 ~/.config/ghostty/config 之後讀，" \
      "   Ghostty also reads $other, and reads it AFTER ~/.config/ghostty/config," >&2
    t "   所以那裡設的同名項目會蓋過這裡的選擇。把那幾行從該檔案移除或註解掉就會生效。" \
      "   so the keys it sets win. Remove or comment those lines there to let this selection through." >&2
    t "   ghostty-tui 可以直接幫你把那幾行註解掉。" \
      "   ghostty-tui can comment those lines out for you." >&2
  done < <(_shadow_configs)
  return 0
}

# What each managed-block pair decides, as `KIND<US>KEY<US>VALUE` records, so
# the conflict scan below can hand them to _keys_decided_by the same way
# apply_selection does for a single selection. The `_overrides` pair is skipped:
# its include only restates raw values that are already in the block, so
# counting it would just yield the same keys twice.
_managed_block_decisions() {
  local config="$1"
  awk -v begin="${BEGIN_MARK}" -v end="${END_MARK}" '
    $0 == begin { in_block = 1; next }
    $0 == end { exit }
    !in_block { next }
    /^[[:space:]]*# category:/ {
      category = $0
      sub(/^[[:space:]]*# category:/, "", category)
      next
    }
    category == "_overrides" { next }
    match($0, /^[[:space:]]*([a-z0-9-]+)[[:space:]]*=/) {
      key = substr($0, RSTART, RLENGTH)
      sub(/^[[:space:]]*/, "", key)
      sub(/[[:space:]]*=.*/, "", key)
      value = $0
      sub(/^[[:space:]]*[a-z0-9-]+[[:space:]]*=[[:space:]]*/, "", value)
      if (key ~ /^font-family[a-z-]*$/) {
        printf "raw\037%s\037%s\n", key, value
      } else if (key == "config-file") {
        printf "file\037%s\037%s\n", key, value
      } else if (key == "theme") {
        printf "name\037%s\037%s\n", key, value
      } else if (key == "custom-shader") {
        printf "shader\037%s\037%s\n", key, value
      } else {
        printf "raw\037%s\037%s\n", key, value
      }
    }
  ' "${config}"
}

# Every line in a shadowing config that is currently beating the managed block,
# one record per line, for the TUI to parse:
#
#     FILE<US>LINENO<US>KEY<US>the whole offending line, verbatim
#
# <US> is ASCII 0x1F. A Ghostty path or value may legally contain spaces, tabs,
# `=`, `#`, `|` and `:`, so every printable candidate can occur inside a field;
# a control character cannot. The verbatim line comes last so that even a value
# nobody anticipated cannot be mistaken for a separator.
#
# Read-only, and silent with exit 0 when there is nothing wrong — resolve_shadow_conflicts
# is the half that writes.
list_shadow_conflicts() {
  local config="$GHOSTTY_CONFIG"
  local us
  local keys
  local kind key value decided_key other line lineno

  [ -f "${config}" ] || return 0

  us="$(printf '\037')"
  keys=$'\n'

  while IFS="${us}" read -r kind key value; do
    [ -n "${kind}" ] || continue
    while IFS= read -r decided_key; do
      [ -n "${decided_key}" ] || continue
      case "${keys}" in
        *$'\n'"${decided_key}"$'\n'*) ;;
        *) keys="${keys}${decided_key}"$'\n' ;;
      esac
    done < <(_keys_decided_by "${value}" "${kind}" "${key}")
  done < <(_managed_block_decisions "${config}")

  [ "${keys}" != $'\n' ] || return 0

  while IFS= read -r other; do
    lineno=0
    while IFS= read -r line || [ -n "${line}" ]; do
      lineno=$((lineno + 1))
      [[ ${line} =~ ^[[:space:]]*([a-z0-9-]+)[[:space:]]*= ]] || continue
      key="${BASH_REMATCH[1]}"
      case "${keys}" in
        *$'\n'"${key}"$'\n'*)
          printf '%s\037%s\037%s\037%s\n' "${other}" "${lineno}" "${key}" "${line}"
          ;;
      esac
    done < "${other}"
  done < <(_shadow_configs)

  return 0
}

# The other half of list_shadow_conflicts: comment those exact lines out so the
# managed block finally wins. This is the only place the tool writes to a file
# outside its own markers, which is why it backs the file up first, only ever
# prefixes lines it already reported as conflicting, and leaves every other byte
# of the file where it was.
resolve_shadow_conflicts() {
  local rc
  _lock_acquire || return 1
  rc=0
  _resolve_shadow_conflicts_locked || rc=$?
  _lock_release
  return "$rc"
}

# The body, split out only so the many honest early returns below do not each
# have to remember to drop the lock.
_resolve_shadow_conflicts_locked() {
  local conflicts files backups
  local disabled_count seen existing
  local f lineno key line
  local ts idx pad base backup prefix
  local backup_file backup_path current_backup
  local first_lineno line_numbers
  local tmp header1 header2 header3 header4
  local has_trailing_newline

  conflicts="$(list_shadow_conflicts)"
  if [ -z "${conflicts}" ]; then
    t "沒有需要處理的 shadow 衝突。" "No shadow conflicts to resolve." >&2
    return 1
  fi

  files=""
  backups=""
  disabled_count=0
  while IFS=$'\037' read -r f lineno key line; do
    [ -n "${f}" ] || continue
    disabled_count=$((disabled_count + 1))
    seen=0
    if [ -n "${files}" ]; then
      while IFS= read -r existing; do
        [ "${existing}" = "${f}" ] || continue
        seen=1
        break
      done <<EOF
${files}
EOF
    fi
    if [ "${seen}" -eq 0 ]; then
      if [ -z "${files}" ]; then
        files="${f}"
      else
        files="${files}
${f}"
      fi
    fi
  done <<EOF
${conflicts}
EOF

  while IFS= read -r f; do
    [ -n "${f}" ] || continue
    if [ ! -w "${f}" ]; then
      t "檔案不可寫，無法處理 shadow 衝突：${f}" "File is not writable; cannot resolve shadow conflicts: ${f}" >&2
      return 1
    fi
  done <<EOF
${files}
EOF

  mkdir -p "${HISTORY_DIR}" || {
    t "無法建立歷史備份目錄：${HISTORY_DIR}" "Could not create history directory: ${HISTORY_DIR}" >&2
    return 1
  }
  ts="$(date +%Y%m%d-%H%M%S)" || return 1

  while IFS= read -r f; do
    [ -n "${f}" ] || continue
    base="${f##*/}"
    prefix="${HISTORY_DIR}/${ts}-"
    idx=0
    while :; do
      pad="$(printf '%04d' "${idx}")" || return 1
      backup="${prefix}${pad}-shadow-${base}.bak"
      [ ! -e "${backup}" ] && break
      idx=$((idx + 1))
    done
    cp "${f}" "${backup}" || {
      t "備份失敗，未修改任何檔案：${f}" "Backup failed; no files were modified: ${f}" >&2
      return 1
    }
    if [ -z "${backups}" ]; then
      backups="${f}"$'\037'"${backup}"
    else
      backups="${backups}
${f}"$'\037'"${backup}"
    fi
  done <<EOF
${files}
EOF

  while IFS= read -r f; do
    [ -n "${f}" ] || continue
    current_backup=""
    while IFS=$'\037' read -r backup_file backup_path; do
      [ "${backup_file}" = "${f}" ] || continue
      current_backup="${backup_path}"
      break
    done <<EOF
${backups}
EOF
    [ -n "${current_backup}" ] || {
      t "找不到對應的備份，未修改任何檔案：${f}" "Could not find the matching backup; no files were modified: ${f}" >&2
      return 1
    }

    first_lineno=""
    line_numbers=""
    while IFS=$'\037' read -r backup_file lineno key line; do
      [ "${backup_file}" = "${f}" ] || continue
      if [ -z "${first_lineno}" ]; then
        first_lineno="${lineno}"
      fi
      if [ -z "${line_numbers}" ]; then
        line_numbers="${lineno}"
      else
        line_numbers="${line_numbers},${lineno}"
      fi
    done <<EOF
${conflicts}
EOF

    header1="$(t '# ghostty-config-studio 已停用這些覆寫的 Ghostty 設定。' '# ghostty-config-studio disabled these shadowing Ghostty settings.')"
    header2="$(t '# macOS 會在 ~/.config/ghostty/config 之後讀取此檔，所以這些鍵原本會蓋過你的選擇。' '# macOS reads this file after ~/.config/ghostty/config, so these keys were overriding your selections.')"
    header3="$(t "# 備份：${current_backup}" "# Backup: ${current_backup}")"
    header4="$(t '# 手動還原：回復這個備份，或刪除被停用行前面的 "# ghostty-config-studio disabled: " 前綴。' '# To undo manually: restore that backup, or delete the "# ghostty-config-studio disabled: " prefix from the disabled lines.')"

    tmp="$(mktemp "${TMPDIR:-/tmp}/ghostty-config-studio.XXXXXX")" || {
      t "無法建立暫存檔，未修改任何檔案：${f}" "Could not create a temporary file; no files were modified: ${f}" >&2
      return 1
    }
    # Preserve configs that intentionally omit the final newline; awk's default ORS would add one.
    has_trailing_newline=1
    if [ -n "$(tail -c1 "${f}")" ]; then
      has_trailing_newline=0
    fi
    # The headers go through the environment rather than `awk -v`: -v runs
    # escape processing over its value, and header3 carries a filesystem path,
    # so a backslash anywhere in $HISTORY_DIR would be eaten and the file would
    # end up naming a backup that is not where it says.
    GCS_H1="${header1}" GCS_H2="${header2}" GCS_H3="${header3}" GCS_H4="${header4}" \
    awk \
      -v first_line="${first_lineno}" \
      -v disabled_lines="${line_numbers}" \
      -v has_trailing_newline="${has_trailing_newline}" '
      BEGIN {
        ORS = ""
        header1 = ENVIRON["GCS_H1"]; header2 = ENVIRON["GCS_H2"]
        header3 = ENVIRON["GCS_H3"]; header4 = ENVIRON["GCS_H4"]
        split(disabled_lines, wanted, ",")
        for (i in wanted) {
          disabled[wanted[i]] = 1
        }
      }
      {
        if (NR > 1) {
          printf "\n"
        }
        if (NR == first_line) {
          printf "%s\n%s\n%s\n%s\n", header1, header2, header3, header4
        }
        if (disabled[NR]) {
          printf "# ghostty-config-studio disabled: %s", $0
        } else {
          printf "%s", $0
        }
      }
      END {
        if (has_trailing_newline) {
          printf "\n"
        }
      }
    ' "${f}" > "${tmp}" || {
      rm -f "${tmp}" || true
      t "無法重寫檔案，未完成處理：${f}" "Could not rewrite file; resolution was not completed: ${f}" >&2
      return 1
    }
    # Copy, never `mv`: a rename replaces the destination inode, so the file
    # would come back as mktemp's 0600 and a symlinked config would be turned
    # into a regular file. Measured on a same-device rename — it is the inode
    # swap that does it, not mv's cross-device copy fallback.
    cp "${tmp}" "${f}" || {
      rm -f "${tmp}" || true
      t "無法寫回檔案，未完成處理：${f}" "Could not write the updated file back: ${f}" >&2
      return 1
    }
    rm -f "${tmp}" || true
  done <<EOF
${files}
EOF

  t "已停用 ${disabled_count} 行 shadow 衝突；原始檔已備份到 ${HISTORY_DIR}。" "Disabled ${disabled_count} shadow-conflicting lines; originals are backed up in ${HISTORY_DIR}."
  return 0
}

apply_selection() {
  local category="$1" value="$2" kind="${3:-file}" shader_src="${4:-}" key="${5:-}"
  local rc
  # Ghostty parses `key = ` fine and then ignores it, so an empty value would
  # show as an active setting that does nothing; `config-file = ` is worse,
  # since a path that is not there is the whole-config fallback. The TUI editor
  # already treats empty input as "not set" — this is the same rule for anyone
  # calling the shell entry point directly.
  if [ -z "$value" ] || { [ "$kind" = "raw" ] && [ -z "$key" ]; }; then
    t "沒有值可以套用（空值會被 Ghostty 忽略），沒有任何變更：${category}" \
      "Nothing to apply — an empty value is silently ignored by Ghostty; nothing was changed: ${category}" >&2
    return 1
  fi
  # Before the snapshot and before any shader is copied: a config whose markers
  # this tool cannot match must cost the user nothing but the message.
  _require_sane_markers || return 1
  _lock_acquire || return 1
  snapshot_config
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
  rc=0
  if [ "$category" = "preset" ] || [ "$category" = "custom" ]; then
    set_solo_path_for "$category" "$value" "$kind" "$key" || rc=1
  else
    set_path_for "$category" "$value" "$kind" "$key" || rc=1
  fi
  if [ "$rc" -ne 0 ]; then
    _lock_release
    return 1
  fi
  # A write that does not survive validation is the dangling-include failure
  # from DESIGN_NOTES.md: Ghostty gives up on the whole config and falls back to
  # defaults, so the user loses every other setting too, with nothing on screen
  # to explain it. Put the file back before that can reach a running terminal.
  # The rollback consumes this apply's own snapshot so the failed attempt leaves
  # no undo step pointing at the broken write.
  if ! validate_config; then
    if [ -n "$_LAST_SNAPSHOT" ]; then
      _restore_snapshot "$_LAST_SNAPSHOT" 0 || true
      # The restore regenerates the override include from the block it just put
      # back, which is right for undo and was wrong here: with two managed
      # blocks it re-emitted the include into both and left `cycle detected`
      # on disk under a message claiming the write had been rolled back. So
      # check the rollback's own work, and if it is still broken, put the
      # snapshot back byte for byte with nothing layered on top.
      if ! validate_config; then
        _restore_snapshot_bytes "$_LAST_SNAPSHOT" || true
        t "   （回復動作本身也沒通過驗證，已改用原始快照原封不動還原。）" \
          "   (The rollback itself did not validate either; restored the raw snapshot untouched instead.)" >&2
      fi
      rm -f "$_LAST_SNAPSHOT"
    fi
    t "✖ 設定檔驗證失敗，已還原：${category}" \
      "✖ Config validation failed, rolled back: ${category}" >&2
    _lock_release
    return 1
  fi
  # Advisory only: never let it turn a successful write into a failure.
  warn_if_shadowed "$value" "$kind" "$key" || true
  _lock_release
  return 0
}

# save_current_as DEST — snapshot whatever's currently active (across every
# category: theme/font/cursor/...) into DEST as a standalone preset file.
# Raw scalar lines stay readable in DEST, and when any are present they are
# also restated in a last-applied sidecar include so Ghostty resolves them
# after any theme include inside the preset.
save_current_as() {
  local dest="$1" raw_f dest_tmp sidecar sidecar_tmp
  _require_sane_markers || return 1
  if [ ! -f "$GHOSTTY_CONFIG" ] || [ "$(_marker_fault "$GHOSTTY_CONFIG")" != "ok" ]; then
    t "目前沒有可儲存的選擇，先挑一個主題、字體或游標。" \
      "Nothing currently selected to save — pick a theme, font, or cursor first." >&2
    return 1
  fi
  raw_f="$(mktemp)"
  _raw_lines_from_block > "$raw_f"

  dest_tmp="$(mktemp)"
  _managed_directives_without_overrides > "$dest_tmp"

  if [ -s "$raw_f" ]; then
    sidecar="$(_preset_sidecar_path "$dest")"
    mkdir -p "${sidecar%/*}"
    sidecar_tmp="$(mktemp)"
    _write_override_header "$sidecar_tmp" preset
    cat "$raw_f" >> "$sidecar_tmp"
    _replace_if_changed "$sidecar" "$sidecar_tmp"
    echo "config-file = $sidecar" >> "$dest_tmp"
  fi

  _replace_if_changed "$dest" "$dest_tmp"
  rm -f "$raw_f"
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
    t "已切換 ${cat}：${names[$idx]} -- ${descs[$idx]}" \
      "Switched $cat to: ${names[$idx]} -- ${descs[$idx]}"
  }

  if [ -n "$direct" ]; then
    local i
    for i in "${!names[@]}"; do
      if [ "${names[$i]}" = "$direct" ]; then _apply_choice "$i"; return 0; fi
    done
    t "找不到 ${category}：$direct" "Unknown $category: $direct" >&2
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
