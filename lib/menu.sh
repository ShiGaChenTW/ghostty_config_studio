#!/usr/bin/env bash
# Shared numbered-menu picker + non-destructive config-file writer.
# Sourced by ghostty-theme / ghostty-font / ghostty-preset and the rest of the
# ghostty-* entry points, and by the Go TUI through
# `bash -c 'source lib/menu.sh; <fn> ...'`. Not run directly.
#
# The implementation lives in the part files sourced below, one per concern.
# This file is the loader and the only path anything outside lib/ ever names,
# so every caller keeps working unchanged.
#
# Order matters. core.sh defines the paths, the markers, the `t` bilingual
# helper and the write lock that every later part's functions read, and it
# installs the EXIT trap that releases that lock in the sourcing shell —
# so it goes first. The rest are order-independent between themselves, since
# bash resolves function calls at call time, and are listed in the order the
# data flows: read/write the block, regenerate the override include, snapshot
# and validate around it, check the shadowing config, apply, present.
#
# $BASH_SOURCE rather than $0 or the caller's cwd: the entry points locate
# lib/ the same way, and this has to resolve from a git clone and from a
# Homebrew libexec install alike. The formula installs lib/ wholesale
# (`libexec.install "lib"`), so extra files here ship for free — but a part
# that failed to load would leave a half-built library behind and break every
# command with a confusing error somewhere far away, hence the loud stop.
set -euo pipefail

GCS_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for _gcs_part in core config-block overrides history shadow apply picker; do
  if [ ! -r "${GCS_LIB_DIR}/${_gcs_part}.sh" ]; then
    echo "ghostty-config-studio: cannot read ${GCS_LIB_DIR}/${_gcs_part}.sh — the installation is incomplete." >&2
    exit 1
  fi
  . "${GCS_LIB_DIR}/${_gcs_part}.sh"
done
unset _gcs_part
