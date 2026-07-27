#!/usr/bin/env bash
# The numbered-menu picker UI the ghostty-* entry points drive. Sourced by
# lib/menu.sh; not run directly.

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
