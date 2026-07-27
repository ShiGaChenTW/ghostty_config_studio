#!/usr/bin/env bash
# The second config macOS lets beat ours: finding the keys it overrides,
# warning about them, and commenting them out on request. Sourced by
# lib/menu.sh; not run directly.

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
  # The same marker sanity apply_selection demands, for the same reason and
  # then some. With a BEGIN and no END, _managed_block_decisions reads to the
  # end of the file, so every line below the marker looks like a managed
  # decision — and the resolver would then comment those keys out of the
  # user's Application Support config, which is the one file this tool writes
  # outside its own markers. Refusing to guess where the block ends has to
  # apply here too, not only on the write path.
  _require_sane_markers >/dev/null 2>&1 || return 0

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
