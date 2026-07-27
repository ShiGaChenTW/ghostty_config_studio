#!/usr/bin/env bash
# Paths, the bilingual t helper, the managed-block markers, the write lock
# and the two file installers everything else builds on. Sourced by
# lib/menu.sh before every other part; not run directly.

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
