#!/usr/bin/env bash
# The orchestrating write: refuse, snapshot, write, validate, roll back, warn.
# Sourced by lib/menu.sh; not run directly.

# apply_selection CATEGORY VALUE KIND [SHADER_SRC] [KEY]
# Standalone entry point (also used internally by run_picker's _apply_choice)
# so the Bubble Tea TUI can apply a selection by shelling out to
# `bash -c 'source lib/menu.sh; apply_selection ...'` instead of duplicating
# the managed-block logic in Go. KEY is only needed for kind=raw.
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
