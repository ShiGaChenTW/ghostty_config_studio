package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Interface language. Held as a package-level variable rather than threaded
// through every render call: it's read by dozens of leaf functions (including
// keyInfo methods that have no access to the model) and only ever changes on
// an explicit keypress, so passing it everywhere would add noise without
// buying anything.
type language int

const (
	langZH language = iota
	langEN
)

var lang = langZH

// tr picks between a Traditional Chinese and an English string.
func tr(zh, en string) string {
	if lang == langEN {
		return en
	}
	return zh
}

func toggleLang() {
	if lang == langZH {
		lang = langEN
	} else {
		lang = langZH
	}
	saveLang()
}

func langName() string { return tr("中文", "English") }

// --- persistence -----------------------------------------------------------
//
// Stored next to the recent-items file so the choice survives restarts.
// Best-effort: a read or write failure just means the default language.

func langFilePath() string {
	return filepath.Join(ghosttyConfigDir(), ".ghostty-tui-lang")
}

func loadLang() {
	b, err := os.ReadFile(langFilePath())
	if err != nil {
		return
	}
	if strings.TrimSpace(string(b)) == "en" {
		lang = langEN
	}
}

func saveLang() {
	code := "zh"
	if lang == langEN {
		code = "en"
	}
	_ = os.MkdirAll(ghosttyConfigDir(), 0o755)
	_ = os.WriteFile(langFilePath(), []byte(code+"\n"), 0o644)
}

// --- Entry labels ------------------------------------------------------------
//
// The bracketed category and the style tags on every list row. Translated so
// the row reads in one language, but see entry.FilterValue: the search index
// keeps BOTH languages, so /cursor and /游標 both find the same rows whichever
// language the interface happens to be in.

var categoryZH = map[string]string{
	"theme":          "主題",
	"font":           "字型",
	"preset":         "完整設定",
	"custom":         "自訂",
	"cursor":         "游標特效",
	"cursor-style":   "游標形狀",
	"cursor-blink":   "游標閃爍",
	"opacity":        "背景透明度",
	"blur":           "背景模糊",
	"clipboard-trim": "剪貼簿修剪",
	"copy-on-select": "選取即複製",
}

var styleTagZH = map[string]string{
	"nature":       "自然",
	"calm":         "沉靜",
	"animated":     "動態",
	"retro":        "復古",
	"vibrant":      "鮮豔",
	"cyberpunk":    "賽博龐克",
	"dark":         "暗色",
	"light":        "亮色",
	"minimal":      "極簡",
	"professional": "專業",
}

// categoryTag is the bracketed label at the head of a row's description.
func categoryTag(cat string) string {
	if lang == langZH {
		if zh, ok := categoryZH[cat]; ok {
			return zh
		}
	}
	return cat
}

func styleTag(tag string) string {
	if lang == langZH {
		if zh, ok := styleTagZH[tag]; ok {
			return zh
		}
	}
	return tag
}

// searchAliases returns every spelling a term is known by, so the filter can
// index both languages at once regardless of which one is on screen.
func searchAliases(cat string, tags []string) string {
	out := []string{cat}
	if zh, ok := categoryZH[cat]; ok {
		out = append(out, zh)
	}
	for _, t := range tags {
		out = append(out, t)
		if zh, ok := styleTagZH[t]; ok {
			out = append(out, zh)
		}
	}
	return strings.Join(out, " ")
}

func txtBuiltinTheme() string { return tr("Ghostty 內建主題", "Ghostty built-in theme") }

// --- Tag filter ---------------------------------------------------------

func txtTagTitle() string { return tr("依標籤篩選", "Filter by tag") }
func txtTagBody() string {
	return tr("勾選多個標籤時取交集。可以和 [/] 搜尋疊加使用。",
		"Selecting several tags narrows to their intersection. Stacks with [/] search.")
}
func txtTagVibe() string { return tr("氛圍（人工標註）", "Character (hand-tagged)") }
func txtTagTone() string {
	return tr("明暗（由背景亮度推得）", "Tone (derived from background luminance)")
}
func txtTagFooter() string {
	return tr("[↑/↓] 移動  [空白] 勾選  [c] 全部清除  [enter] 套用  [esc] 取消",
		"[↑/↓] move  [space] toggle  [c] clear all  [enter] apply  [esc] cancel")
}
func txtTagNone() string { return tr("（沒有符合的項目）", "(nothing matches)") }
func txtEmptyByTags() string {
	return tr("沒有項目同時符合這些標籤。\n按 [t] 調整或清除。",
		"No entry carries all of these tags.\nPress [t] to adjust or clear.")
}
func txtTagActive(list string, n int) string {
	if lang == langEN {
		return "tags: " + list + "  (" + plural(n, "match", "matches") + ")"
	}
	return "標籤：" + list + "（" + itoa(n) + " 筆）"
}

// --- UI strings ------------------------------------------------------------

func txtBrowserMode() string  { return tr("設定瀏覽器", "Browser") }
func txtEditorMode() string   { return tr("設定編輯器", "Settings Editor") }
func txtPaneList() string     { return tr("主題與設定", "Themes & Settings") }
func txtPanePreview() string  { return tr("預覽", "Preview") }
func txtPaneItems() string    { return tr("設定項目", "Settings") }
func txtPaneDetail() string   { return tr("詳細說明", "Details") }
func txtAllowedValue() string { return tr("可填值", "Allowed") }
func txtDefault() string      { return tr("Ghostty 預設", "Ghostty default") }

func txtFilterPrompt() string { return tr("篩選：", "Filter: ") }

// bubbles/list builds its own "N items" status line from these two words.
func txtStatusItemName() (string, string) {
	return tr("個項目", "item"), tr("個項目", "items")
}

func txtItemCount(n int) string {
	if lang == langEN {
		return plural(n, "item", "items")
	}
	return itoa(n) + " 個項目"
}

func txtCheckedCount(n, total int) string {
	if lang == langEN {
		return itoa(n) + " / " + itoa(total) + " selected"
	}
	return "已勾選 " + itoa(n) + " / " + itoa(total) + " 項"
}

func txtCurrentValue(v string) string {
	return tr("目前設定 = ", "current = ") + v
}

func txtNoFieldsYet() string {
	return tr("  (還沒有任何欄位 — 按 [a] 新增第一個)",
		"  (nothing set yet — press [a] to add a setting)")
}

func txtPickValue() string {
	return tr("選一個值（不用打字）", "Pick a value (no typing)")
}

func txtIsDefault() string { return tr("  （Ghostty 預設）", "  (Ghostty default)") }
func txtBlankValue() string {
	return tr("(留空 — 交給程式決定)", "(leave empty — let the program decide)")
}

func txtAllowedRange() string { return tr("可填範圍：", "Allowed:") }
func txtDefaultValue() string { return tr("預設值：", "Default:") }

// Footers.

func txtBrowserFooter() string {
	return tr(
		"[↑/↓] 移動  [/] 搜尋  [t] 標籤  [enter] 套用  [s] 存目前組合  [n] 新設定檔  [e] 編輯設定  [L] English  [q] 離開",
		"[↑/↓] move  [/] search  [t] tags  [enter] apply  [s] save current  [n] new config  [e] edit  [L] 中文  [q] quit")
}

func txtEditorFooter() string {
	return tr(
		"[←/→] 切換分類  [↑/↓] 移動  [空白] 勾選/取消  [enter] 改值  [s] 儲存  [L] English  [esc] 離開",
		"[←/→] category  [↑/↓] move  [space] toggle  [enter] edit value  [s] save  [L] 中文  [esc] back")
}

func txtValuePickerFooter() string {
	return tr("[↑/↓] 選  [enter] 確定  [esc] 取消",
		"[↑/↓] choose  [enter] confirm  [esc] cancel")
}

func txtTextValueFooter() string {
	return tr("[enter] 確定  [esc] 取消", "[enter] confirm  [esc] cancel")
}

// Dialogs.

func txtRestartTitle() string { return tr("restart required", "restart required") }
func txtRestartBody() string {
	return tr("有些設定（例如 shader）reload 不一定會生效。\n需要完全關閉並重新開啟 Ghostty 才會確定套用。",
		"Some settings (shaders especially) don't reliably take effect on reload.\nA full quit and relaunch of Ghostty is needed to be sure.")
}
func txtRestartAsk() string { return tr("現在重新啟動 Ghostty 嗎？", "Restart Ghostty now?") }
func txtRestartYes() string { return tr("[y] 是，重啟", "[y] yes, restart") }
func txtRestartNo() string  { return tr("[n] 否，稍後手動重啟", "[n] no, I'll restart later") }
func txtConfirmFoot() string {
	return tr("[enter] 確定   [esc] 取消", "[enter] confirm   [esc] cancel")
}

func txtNewConfigTitle() string { return tr("new custom config", "new custom config") }
func txtNewConfigBody() string {
	return tr("建立一個空白的自訂設定檔，接著逐欄位加設定。",
		"Create an empty custom config, then add settings field by field.")
}
func txtSaveComboTitle() string { return tr("save custom preset", "save custom preset") }
func txtSaveComboBody() string {
	return tr("把目前套用的組合存成一個自訂 preset，以後直接切回來。",
		"Save what's currently applied as a custom preset you can switch back to.")
}

func txtPickConfigTitle() string { return tr("選擇要編輯的設定檔", "Pick a config to edit") }
func txtPickConfigBody() string {
	return tr("只列出你自己建立的設定檔；vendor 進來的是唯讀的，不會出現在這裡。",
		"Only your own configs are listed; imported packs are read-only and never appear here.")
}
func txtSettingCount(n int) string {
	if lang == langEN {
		return "(" + plural(n, "setting", "settings") + ")"
	}
	return "(" + itoa(n) + " 項設定)"
}
func txtPickConfigFooter() string {
	return tr("[↑/↓] 選擇  [enter] 編輯  [d] 刪除  [esc] 取消",
		"[↑/↓] choose  [enter] edit  [d] delete  [esc] cancel")
}

func txtDeleteTitle() string { return tr("刪除設定檔", "Delete config") }
func txtDeleteAsk(name string) string {
	return tr("確定要刪除「"+name+"」嗎？", "Delete \""+name+"\"?")
}
func txtDeleteWarn() string {
	return tr("檔案會直接從硬碟移除，無法復原。",
		"The file is removed from disk; this cannot be undone.")
}
func txtDeleteApplied(cats string) string {
	return tr("⚠ 這個設定檔目前正在套用中（"+cats+"）",
		"⚠ This config is currently applied ("+cats+")")
}
func txtDeleteAppliedNote() string {
	return tr("  刪除時會一併從 Ghostty 設定移除，否則設定會整份失效。",
		"  It will be removed from your Ghostty config too — otherwise the whole config breaks.")
}
func txtDeleteYes() string { return tr("[y] 確定刪除", "[y] delete") }
func txtDeleteNo() string  { return tr("[n] 取消", "[n] cancel") }

// Status messages.

func txtSaved(name string, n int) string {
	if lang == langEN {
		return "Saved " + name + " (" + plural(n, "setting", "settings") + ")"
	}
	return "已儲存 " + name + "（" + itoa(n) + " 項設定）"
}
func txtNameFirst() string { return tr("請先輸入名稱", "Give it a name first") }
func txtAlreadyExists(name string) string {
	return tr("已經有同名的設定檔："+name, "Already exists: "+name)
}
func txtReadOnly() string {
	return tr("唯讀：匯入的設定檔不能直接編輯，按 [n] 建立新的自訂設定檔",
		"Read-only: imported configs can't be edited. Press [n] to create your own.")
}
func txtNoEditable() string {
	return tr("還沒有可編輯的自訂設定檔 — 按 [n] 建立一個",
		"No editable configs yet — press [n] to create one")
}
func txtDeleted(name string) string { return tr("已刪除 "+name, "Deleted "+name) }
func txtDeletedAndCleared(name, cats string) string {
	return tr("已刪除 "+name+"，並從 Ghostty 設定移除（"+cats+"）",
		"Deleted "+name+" and removed it from your Ghostty config ("+cats+")")
}
func txtSwitched(cat, name, desc string) string {
	return tr("已切換 "+cat+"："+name+" — "+desc,
		"Switched "+cat+" to "+name+" — "+desc)
}
func txtNoPreview() string {
	return tr("（這個內建主題沒有本機檔案可預覽）",
		"(no local file to preview for this built-in theme)")
}
func txtTooSmall(w, h, minW, minH int) string {
	return tr(
		"終端機視窗太小 ("+itoa(w)+"x"+itoa(h)+")\n請放大到至少 "+itoa(minW)+"x"+itoa(minH)+" 再試一次",
		"Terminal window too small ("+itoa(w)+"x"+itoa(h)+")\nPlease resize to at least "+itoa(minW)+"x"+itoa(minH))
}

// itoa/plural keep this file free of fmt just for two trivial cases.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return itoa(n) + " " + one
	}
	return itoa(n) + " " + many
}
