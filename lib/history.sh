#!/usr/bin/env bash
# The safety net around a write: snapshot before, validate after, restore when
# validation fails or the user asks to undo. Sourced by lib/menu.sh; not run
# directly.

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
