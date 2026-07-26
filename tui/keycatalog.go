package main

import "strings"

// keyCatalog is the full catalog of Ghostty config keys shown by the
// settings editor, grouped into categoryOrder pages.
//
// GENERATED-ASSISTED, then hand-reviewed: key names, defaults and
// validVals came from parsing this machine's own
// `ghostty +show-config --default=true --docs=true` output (200 unique
// keys), so they match the installed Ghostty rather than being guessed or
// hand-transcribed. The zh-TW name/desc/valueHint text and the category
// assignment are hand-written. A newer Ghostty may add keys absent here;
// the editor keeps a manual-key escape hatch so nothing is blocked by a
// stale catalog.
type keyInfo struct {
	key        string
	zhName     string   // zh-TW: short name, shown inline as "key (中文名)"
	desc       string   // zh-TW: full explanation, shown in the detail panel
	valueHint  string   // zh-TW: accepted format / range
	defaultVal string   // Ghostty's own default
	validVals  []string // enumerated legal values, when Ghostty documents them
	cat        string   // which category page this belongs to
}

// rangeLabel is what a list row shows after the colon, and what the value
// editor shows as the allowed input.
func (k keyInfo) rangeLabel() string {
	if len(k.validVals) > 0 {
		return strings.Join(k.validVals, " / ")
	}
	if k.valueHint != "" {
		return k.valueHint
	}
	if k.defaultVal != "" {
		return "預設 " + k.defaultVal
	}
	return "自由文字"
}

var categoryOrder = []string{
	"外觀與顏色",
	"字型",
	"游標",
	"視窗",
	"分頁與分割",
	"鍵盤與滑鼠",
	"剪貼簿與選取",
	"終端機行為",
	"通知與響鈴",
	"Shader 特效",
	"快速終端機",
	"macOS 專屬",
	"Linux 專屬",
	"進階與系統",
}

var keyCatalog = []keyInfo{
	// --- 外觀與顏色 ---
	{"alpha-blending", "半透明混色使用的色彩空間", "半透明混色使用的色彩空間。半透明背景看起來字太細或太糊時可改 linear-corrected", "", "native", []string{"native", "linear", "linear-corrected"}, "外觀與顏色"},
	{"background", "視窗背景色", "視窗背景色", "十六進位色碼，如 #1d1f21", "#282c34", nil, "外觀與顏色"},
	{"bold-color", "粗體字專用的顏色", "粗體字專用的顏色，不設定則沿用一般文字色", "色碼或 cell-foreground／cell-background", "", nil, "外觀與顏色"},
	{"faint-opacity", "淡色", "淡色（faint）文字的不透明度", "0 到 1 的小數", "0.5", nil, "外觀與顏色"},
	{"foreground", "視窗前景", "視窗前景（文字）色", "十六進位色碼，如 #ffffff", "#ffffff", nil, "外觀與顏色"},
	{"minimum-contrast", "前景與背景的最低對比度", "前景與背景的最低對比度，數字越大字越容易讀", "1 到 21 的數字", "1", nil, "外觀與顏色"},
	{"osc-color-report-format", "程式查詢顏色時的回報格式", "程式查詢顏色時的回報格式", "", "16-bit", []string{"none", "8-bit", "16-bit"}, "外觀與顏色"},
	{"palette", "256 色調色盤", "256 色調色盤（可重複設定多筆）", "格式 索引=色碼，如 0=#1d1f21", "0=#1d1f21", nil, "外觀與顏色"},
	{"palette-generate", "只給前 16 色", "只給前 16 色，其餘 240 色自動生成", "true 或 false", "false", nil, "外觀與顏色"},
	{"palette-harmonious", "自動生成調色盤時反轉色相", "自動生成調色盤時反轉色相，配色更協調", "true 或 false", "false", nil, "外觀與顏色"},
	{"split-divider-color", "分割視窗之間分隔線的顏色", "分割視窗之間分隔線的顏色", "色碼", "", nil, "外觀與顏色"},
	{"theme", "套用內建或自訂主題名稱", "套用內建或自訂主題名稱，可用 ghostty +list-themes 查全部", "主題名稱，或 light:名稱,dark:名稱", "", nil, "外觀與顏色"},
	{"unfocused-split-fill", "未聚焦分割視窗的變暗覆蓋色", "未聚焦分割視窗的變暗覆蓋色", "色碼", "", nil, "外觀與顏色"},
	{"unfocused-split-opacity", "未聚焦分割視窗的不透明度", "未聚焦分割視窗的不透明度，越小越暗", "0 到 1 的小數", "0.7", nil, "外觀與顏色"},
	{"window-colorspace", "視窗色彩空間", "視窗色彩空間，display-p3 在廣色域螢幕上顏色更飽和（僅 macOS）", "", "srgb", []string{"srgb", "display-p3"}, "外觀與顏色"},
	// --- 字型 ---
	{"adjust-box-thickness", "製表框線字元的粗細微調", "製表框線字元的粗細微調", "像素或百分比，如 150% 或 2px", "", nil, "字型"},
	{"adjust-cell-height", "每個字元格的高度微調", "每個字元格的高度微調，行距太擠或太鬆時調這個", "像素或百分比，如 10% 或 -2px", "", nil, "字型"},
	{"adjust-cell-width", "每個字元格的寬度微調", "每個字元格的寬度微調，字太擠或太開時調這個", "像素或百分比，如 5% 或 1px", "", nil, "字型"},
	{"adjust-font-baseline", "文字基線位置微調", "文字基線位置微調，字看起來偏高或偏低時用", "像素或百分比", "", nil, "字型"},
	{"adjust-icon-height", "圖示高度微調", "Nerd Font 圖示的最大高度微調", "像素或百分比", "", nil, "字型"},
	{"adjust-overline-position", "上劃線位置微調", "上劃線位置微調", "像素或百分比", "", nil, "字型"},
	{"adjust-overline-thickness", "上劃線粗細微調", "上劃線粗細微調", "像素或百分比", "", nil, "字型"},
	{"adjust-strikethrough-position", "刪除線位置微調", "刪除線位置微調", "像素或百分比", "", nil, "字型"},
	{"adjust-strikethrough-thickness", "刪除線粗細微調", "刪除線粗細微調", "像素或百分比", "", nil, "字型"},
	{"adjust-underline-position", "底線位置微調", "底線位置微調", "像素或百分比", "", nil, "字型"},
	{"adjust-underline-thickness", "底線粗細微調", "底線粗細微調", "像素或百分比", "", nil, "字型"},
	{"font-codepoint-map", "字元指定字型", "指定某些 Unicode 字元強制用特定字型顯示", "格式 U+XXXX-U+YYYY=字型名", "", nil, "字型"},
	{"font-family", "主要字型", "主要字型（可重複設定多個做後備字型）", "字型名稱", "", nil, "字型"},
	{"font-family-bold", "粗體專用字型", "粗體專用字型", "字型名稱", "", nil, "字型"},
	{"font-family-bold-italic", "粗斜體專用字型", "粗斜體專用字型", "字型名稱", "", nil, "字型"},
	{"font-family-italic", "斜體專用字型", "斜體專用字型", "字型名稱", "", nil, "字型"},
	{"font-feature", "字型 OpenType 特性", "開關字型的 OpenType 特性，例如關掉連字 -liga（可重複設定多筆）", "如 +calt、-liga", "", nil, "字型"},
	{"font-shaping-break", "在哪些位置切斷字型排版", "在哪些位置切斷字型排版，避免連字跨過游標", "", "cursor", nil, "字型"},
	{"font-size", "字型大小", "字型大小（點），可以填小數", "數字，如 13 或 14.5", "13", nil, "字型"},
	{"font-style", "指定要用字型家族裡的哪個命名樣式", "指定要用字型家族裡的哪個命名樣式", "樣式名稱、default 或 false", "default", nil, "字型"},
	{"font-style-bold", "粗體要用的命名樣式", "粗體要用的命名樣式", "樣式名稱、default 或 false", "default", nil, "字型"},
	{"font-style-bold-italic", "粗斜體要用的命名樣式", "粗斜體要用的命名樣式", "樣式名稱、default 或 false", "default", nil, "字型"},
	{"font-style-italic", "斜體要用的命名樣式", "斜體要用的命名樣式", "樣式名稱、default 或 false", "default", nil, "字型"},
	{"font-synthetic-style", "字型沒有粗體／斜體時", "字型沒有粗體／斜體時，是否由程式自己合成一個", "以逗號分隔，可加 no- 前綴關閉", "bold,italic,bold-italic", nil, "字型"},
	{"font-thicken", "把字畫粗一點", "把字畫粗一點，在 Retina 螢幕上比較好讀", "true 或 false", "false", nil, "字型"},
	{"font-thicken-strength", "字體加粗的強度", "字體加粗的強度", "0 到 255 的整數", "255", nil, "字型"},
	{"font-variation", "可變字型的參數調整", "可變字型的參數調整，例如字重（可重複設定多筆）", "格式 wght=600", "", nil, "字型"},
	{"font-variation-bold", "粗體的可變字型參數", "粗體的可變字型參數", "格式 wght=700", "", nil, "字型"},
	{"font-variation-bold-italic", "粗斜體的可變字型參數", "粗斜體的可變字型參數", "格式 wght=700", "", nil, "字型"},
	{"font-variation-italic", "斜體的可變字型參數", "斜體的可變字型參數", "格式 wght=600", "", nil, "字型"},
	{"freetype-load-flags", "FreeType 字型載入選項", "FreeType 字型載入選項，影響字的銳利度", "以逗號分隔，可加 no- 前綴關閉", "hinting,no-force-autohint,no-monochrome,autohint,light", []string{"hinting", "force-autohint", "monochrome", "autohint", "light"}, "字型"},
	{"grapheme-width-method", "計算字元寬度的方式", "計算字元寬度的方式，中日韓與 emoji 排版跑掉時可改 legacy", "", "unicode", []string{"legacy", "unicode"}, "字型"},
	{"window-title-font-family", "視窗與分頁標題用的字型", "視窗與分頁標題用的字型（僅 macOS）", "字型名稱", "", nil, "字型"},
	// --- 游標 ---
	{"adjust-cursor-height", "游標高度微調", "游標高度微調", "像素或百分比", "", nil, "游標"},
	{"adjust-cursor-thickness", "bar/框線型游標的粗細微調", "bar/框線型游標的粗細微調", "像素或百分比", "", nil, "游標"},
	{"cursor-click-to-move", "點擊移動游標", "在提示字元列點一下就能把游標移過去", "true 或 false", "true", nil, "游標"},
	{"cursor-color", "游標顏色", "游標顏色", "色碼或 cell-foreground／cell-background", "", nil, "游標"},
	{"cursor-opacity", "游標不透明度", "游標不透明度，太小會幾乎看不見", "0 到 1 的小數", "1", nil, "游標"},
	{"cursor-style", "游標形狀", "游標形狀", "", "block", []string{"block", "bar", "underline", "block_hollow"}, "游標"},
	{"cursor-style-blink", "游標是否閃爍", "游標是否閃爍，留空表示交給程式決定", "", "", []string{" ", "true", "false"}, "游標"},
	{"cursor-text", "游標底下那個字的顏色", "游標底下那個字的顏色", "色碼", "", nil, "游標"},
	// --- 視窗 ---
	{"background-blur", "背景模糊程度", "背景模糊程度，需搭配 background-opacity 小於 1 才看得出來", "false／true／數字強度（20 為預設）／macOS 玻璃效果", "false", []string{"false", "true", "macos-glass-regular", "macos-glass-clear"}, "視窗"},
	{"background-image", "終端機背景圖片", "終端機背景圖片", "圖片檔的絕對路徑", "", nil, "視窗"},
	{"background-image-fit", "背景圖片的縮放方式", "背景圖片的縮放方式", "", "contain", []string{"contain", "cover", "stretch", "none"}, "視窗"},
	{"background-image-opacity", "背景圖片的不透明度", "背景圖片的不透明度", "0 到 1 的小數", "1", nil, "視窗"},
	{"background-image-position", "背景圖片的對齊位置", "背景圖片的對齊位置", "", "center", []string{"top-left", "top-center", "top-right", "center-left", "center", "center-right", "bottom-left", "bottom-center", "bottom-right"}, "視窗"},
	{"background-image-repeat", "背景圖片是否重複平鋪", "背景圖片是否重複平鋪", "true 或 false", "false", nil, "視窗"},
	{"background-opacity", "背景不透明度", "背景不透明度，1 為完全不透明、越小越透明", "0 到 1 的小數，如 0.9", "1", nil, "視窗"},
	{"background-opacity-cells", "字元格也套用透明度", "是否讓有明確背景色的字元格也套用透明度", "true 或 false", "false", nil, "視窗"},
	{"fullscreen", "啟動時就進入全螢幕", "啟動時就進入全螢幕", "", "false", []string{"false", "true", "non-native", "non-native-visible-menu", "non-native-padded-notch"}, "視窗"},
	{"maximize", "啟動時就最大化視窗", "啟動時就最大化視窗", "true 或 false", "false", nil, "視窗"},
	{"resize-overlay", "尺寸提示框時機", "調整視窗大小時顯示尺寸提示框的時機", "", "after-first", []string{"always", "never", "after-first"}, "視窗"},
	{"resize-overlay-duration", "尺寸提示框顯示多久後消失", "尺寸提示框顯示多久後消失", "時間，如 750ms", "750ms", []string{"y", "d", "h", "m", "s", "ms", "us", "ns", "1h30m", "45s"}, "視窗"},
	{"resize-overlay-position", "尺寸提示框顯示的位置", "尺寸提示框顯示的位置", "", "center", []string{"center", "top-left", "top-center", "top-right", "bottom-left", "bottom-center", "bottom-right"}, "視窗"},
	{"window-decoration", "視窗邊框裝飾的偏好設定", "視窗邊框裝飾的偏好設定", "", "auto", []string{"none", "auto", "client", "server"}, "視窗"},
	{"window-height", "啟動時的視窗高度", "啟動時的視窗高度，單位是字元列數", "整數，0 表示用預設", "0", nil, "視窗"},
	{"window-inherit-font-size", "新視窗沿用目前的字型大小", "新視窗沿用目前的字型大小", "true 或 false", "true", nil, "視窗"},
	{"window-inherit-working-directory", "新視窗沿用目前的工作目錄", "新視窗沿用目前的工作目錄", "true 或 false", "true", nil, "視窗"},
	{"window-new-tab-position", "新分頁位置", "新分頁要插在目前分頁旁邊還是排到最後", "", "current", []string{"current", "end"}, "視窗"},
	{"window-padding-balance", "把除不盡的留白平均分到兩側", "把除不盡的留白平均分到兩側，避免偏一邊", "true 或 false", "false", nil, "視窗"},
	{"window-padding-color", "留白區域要用什麼顏色填", "留白區域要用什麼顏色填", "", "background", []string{"background", "extend", "extend-always"}, "視窗"},
	{"window-padding-x", "視窗左右內距", "視窗左右內距（留白）", "像素，或「左,右」分別指定", "2", nil, "視窗"},
	{"window-padding-y", "視窗上下內距", "視窗上下內距（留白）", "像素，或「上,下」分別指定", "2", nil, "視窗"},
	{"window-position-x", "啟動時視窗的水平位置", "啟動時視窗的水平位置", "像素", "", nil, "視窗"},
	{"window-position-y", "啟動時視窗的垂直位置", "啟動時視窗的垂直位置", "像素", "", nil, "視窗"},
	{"window-save-state", "是否記住並還原視窗狀態", "是否記住並還原視窗狀態（僅 macOS）", "", "default", []string{"default", "never", "always"}, "視窗"},
	{"window-show-tab-bar", "分頁列的顯示時機", "分頁列的顯示時機", "auto／always／never", "auto", nil, "視窗"},
	{"window-step-resize", "以字元格為單位縮放", "調整視窗大小時以整個字元格為單位跳動", "true 或 false", "false", nil, "視窗"},
	{"window-subtitle", "視窗副標題要顯示什麼", "視窗副標題要顯示什麼", "", "false", []string{"false", "working-directory"}, "視窗"},
	{"window-theme", "視窗的亮色／暗色主題", "視窗的亮色／暗色主題", "", "auto", []string{"auto", "system", "light", "dark", "ghostty"}, "視窗"},
	{"window-titlebar-background", "標題列背景色", "標題列背景色，需搭配 macos-titlebar-style = tabs", "色碼", "", nil, "視窗"},
	{"window-titlebar-foreground", "標題列文字色", "標題列文字色，需搭配 macos-titlebar-style = tabs", "色碼", "", nil, "視窗"},
	{"window-width", "啟動時的視窗寬度", "啟動時的視窗寬度，單位是字元行數", "整數，0 表示用預設", "0", nil, "視窗"},
	// --- 分頁與分割 ---
	{"confirm-close-surface", "關閉分頁／分割視窗前先跳確認", "關閉分頁／分割視窗前先跳確認", "true 或 false", "true", nil, "分頁與分割"},
	{"focus-follows-mouse", "滑鼠移到哪個分割視窗", "滑鼠移到哪個分割視窗，焦點就跟到哪", "true 或 false", "false", nil, "分頁與分割"},
	{"split-inherit-working-directory", "新分割視窗沿用目前的工作目錄", "新分割視窗沿用目前的工作目錄", "true 或 false", "true", nil, "分頁與分割"},
	{"split-preserve-zoom", "何時保留分割視窗的放大狀態", "何時保留分割視窗的放大狀態", "以逗號分隔，可加 no- 前綴關閉", "no-navigation", nil, "分頁與分割"},
	{"tab-inherit-working-directory", "新分頁沿用目前的工作目錄", "新分頁沿用目前的工作目錄", "true 或 false", "true", nil, "分頁與分割"},
	// --- 鍵盤與滑鼠 ---
	{"click-repeat-interval", "連點判定的間隔時間", "連點判定的間隔時間，0 表示用系統設定", "毫秒", "0", nil, "鍵盤與滑鼠"},
	{"key-remap", "重新對應修飾鍵", "重新對應修飾鍵，例如把 Caps Lock 換成 Ctrl", "格式 physical:key=key", "", nil, "鍵盤與滑鼠"},
	{"keybind", "快捷鍵綁定", "快捷鍵綁定（可重複設定多筆），格式為 觸發鍵=動作", "如 super+n=new_window", "super+shift+,=reload_config", nil, "鍵盤與滑鼠"},
	{"link-previews", "滑鼠移到網址上時顯示預覽", "滑鼠移到網址上時顯示預覽", "true／false／osc8", "true", nil, "鍵盤與滑鼠"},
	{"link-url", "辨識文字裡的網址", "辨識文字裡的網址，讓它可以被點擊", "true 或 false", "true", nil, "鍵盤與滑鼠"},
	{"mouse-hide-while-typing", "打字時自動把滑鼠游標藏起來", "打字時自動把滑鼠游標藏起來", "true 或 false", "false", nil, "鍵盤與滑鼠"},
	{"mouse-reporting", "讓終端機裡的程式", "讓終端機裡的程式（如 vim）能接收滑鼠事件", "true 或 false", "true", nil, "鍵盤與滑鼠"},
	{"mouse-scroll-multiplier", "滾輪捲動距離倍率", "滾輪捲動距離倍率，覺得太慢或太快就調這個", "格式 precision:1,discrete:3", "precision:1,discrete:3", nil, "鍵盤與滑鼠"},
	{"mouse-shift-capture", "Shift 滑鼠事件歸屬", "按住 Shift 時滑鼠事件要給程式還是給終端機自己", "", "false", []string{"true", "false", "always", "never"}, "鍵盤與滑鼠"},
	{"right-click-action", "在終端機上按右鍵的行為", "在終端機上按右鍵的行為", "", "context-menu", []string{"context-menu", "paste", "copy", "copy-or-paste", "ignore"}, "鍵盤與滑鼠"},
	// --- 剪貼簿與選取 ---
	{"clipboard-codepoint-map", "複製時字元替換", "複製文字時把特定 Unicode 字元換成別的字元", "格式 U+XXXX=替代字", "", nil, "剪貼簿與選取"},
	{"clipboard-paste-bracketed-safe", "是否把括號貼上模式視為安全", "是否把括號貼上模式視為安全（不跳確認）", "true 或 false", "true", nil, "剪貼簿與選取"},
	{"clipboard-paste-protection", "貼上看起來有風險的文字前先跳確認", "貼上看起來有風險的文字前先跳確認，防止複製貼上攻擊", "true 或 false", "true", nil, "剪貼簿與選取"},
	{"clipboard-read", "終端機裡的程式能否讀取你的剪貼簿", "終端機裡的程式能否讀取你的剪貼簿", "", "ask", []string{"ask", "allow", "deny"}, "剪貼簿與選取"},
	{"clipboard-trim-trailing-spaces", "複製時自動去掉每行結尾的空白", "複製時自動去掉每行結尾的空白", "true 或 false", "true", nil, "剪貼簿與選取"},
	{"clipboard-write", "終端機裡的程式能否寫入你的剪貼簿", "終端機裡的程式能否寫入你的剪貼簿", "ask／allow／deny", "allow", nil, "剪貼簿與選取"},
	{"copy-on-select", "選取文字後自動複製到剪貼簿", "選取文字後自動複製到剪貼簿", "true 或 false", "true", nil, "剪貼簿與選取"},
	{"search-background", "搜尋符合項目的背景色", "搜尋符合項目的背景色", "色碼", "#ffe082", nil, "剪貼簿與選取"},
	{"search-foreground", "搜尋符合項目的文字色", "搜尋符合項目的文字色", "色碼", "#000000", nil, "剪貼簿與選取"},
	{"search-selected-background", "目前選中的搜尋結果背景色", "目前選中的搜尋結果背景色", "色碼", "#f2a57e", nil, "剪貼簿與選取"},
	{"search-selected-foreground", "目前選中的搜尋結果文字色", "目前選中的搜尋結果文字色", "色碼", "#000000", nil, "剪貼簿與選取"},
	{"selection-background", "選取文字的背景色", "選取文字的背景色", "色碼或 cell-foreground／cell-background", "", nil, "剪貼簿與選取"},
	{"selection-clear-on-copy", "複製後自動取消選取", "複製後自動取消選取", "true 或 false", "false", nil, "剪貼簿與選取"},
	{"selection-clear-on-typing", "開始打字時自動取消選取", "開始打字時自動取消選取", "true 或 false", "true", nil, "剪貼簿與選取"},
	{"selection-foreground", "選取文字的文字色", "選取文字的文字色", "色碼或 cell-foreground／cell-background", "", nil, "剪貼簿與選取"},
	{"selection-word-chars", "雙擊選字時", "雙擊選字時，哪些字元算是「不是文字」的分界", "字元集合字串", " 	'\"│`|:;,()[]{}<>$", nil, "剪貼簿與選取"},
	// --- 終端機行為 ---
	{"abnormal-command-exit-runtime", "異常結束判定時間", "指令執行時間短於這個值就視為「異常結束」，會提示你", "毫秒，如 250", "250", nil, "終端機行為"},
	{"command", "啟動終端機時執行的指令", "啟動終端機時執行的指令，通常是你的 shell", "指令路徑，或 SHELL／passwd", "", nil, "終端機行為"},
	{"enquiry-response", "ENQ 回應字串", "收到 ENQ 控制字元時要回傳的字串", "任意字串", "", nil, "終端機行為"},
	{"env", "傳給終端機程式的額外環境變數", "傳給終端機程式的額外環境變數（可重複設定多筆）", "格式 NAME=value", "", nil, "終端機行為"},
	{"image-storage-limit", "終端機圖片協定可用的記憶體上限", "終端機圖片協定可用的記憶體上限", "位元組數", "320000000", nil, "終端機行為"},
	{"initial-command", "只有第一個終端機視窗要執行的指令", "只有第一個終端機視窗要執行的指令", "指令路徑", "", nil, "終端機行為"},
	{"input", "啟動後自動送進去的輸入內容", "啟動後自動送進去的輸入內容", "raw:字串 或 path:檔案路徑", "", nil, "終端機行為"},
	{"scroll-to-bottom", "什麼情況下要自動捲到最底", "什麼情況下要自動捲到最底", "以逗號分隔，可加 no- 前綴關閉", "keystroke,no-output", nil, "終端機行為"},
	{"scrollback-limit", "歷史紀錄上限", "往回捲的歷史紀錄可佔用的記憶體上限", "位元組數", "10000000", nil, "終端機行為"},
	{"scrollbar", "捲軸的顯示時機", "捲軸的顯示時機", "", "system", []string{"system", "never"}, "終端機行為"},
	{"shell-integration", "shell 整合注入方式", "shell 整合注入方式，提供游標定位、標題等進階功能", "none／detect／bash／zsh／fish／elvish", "detect", []string{"none", "detect", "bash"}, "終端機行為"},
	{"shell-integration-features", "要啟用哪些 shell 整合功能", "要啟用哪些 shell 整合功能", "以逗號分隔，可加 no- 前綴關閉", "cursor,no-sudo,title,no-ssh-env,no-ssh-terminfo,path", []string{"cursor", "sudo", "title", "ssh-env", "ssh-terminfo", "path"}, "終端機行為"},
	{"term", "設定 TERM 環境變數", "設定 TERM 環境變數，遠端連線出問題時可改成 xterm-256color", "字串", "xterm-ghostty", nil, "終端機行為"},
	{"title", "固定視窗標題", "固定視窗標題，設了之後程式就不能自己改標題", "字串", "", nil, "終端機行為"},
	{"title-report", "允許程式反查目前的視窗標題", "允許程式反查目前的視窗標題（有安全疑慮，預設關閉）", "true 或 false", "false", nil, "終端機行為"},
	{"undo-timeout", "復原操作的可用時限", "復原操作的可用時限，超過就不能復原了", "時間，如 5s", "5s", []string{"y", "d", "h", "m", "s", "ms", "us", "ns", "1h30m", "45s"}, "終端機行為"},
	{"vt-kam-allowed", "允許 KAM 鎖鍵盤", "允許程式用 KAM 模式鎖住鍵盤輸入（很少用）", "true 或 false", "false", nil, "終端機行為"},
	{"wait-after-command", "指令結束後不要立刻關閉終端機", "指令結束後不要立刻關閉終端機", "true 或 false", "false", nil, "終端機行為"},
	{"working-directory", "啟動後要切換到哪個目錄", "啟動後要切換到哪個目錄", "路徑、home 或 inherit", "", nil, "終端機行為"},
	// --- 通知與響鈴 ---
	{"app-notifications", "應用內提示", "控制 Ghostty 自己的應用內提示要顯示哪些", "以逗號分隔，可加 no- 前綴關閉", "clipboard-copy,config-reload", nil, "通知與響鈴"},
	{"bell-audio-path", "響鈴時播放的音效檔", "響鈴時播放的音效檔", "音效檔的絕對路徑", "", nil, "通知與響鈴"},
	{"bell-audio-volume", "響鈴音效的音量", "響鈴音效的音量", "0 到 1 的小數", "0.5", nil, "通知與響鈴"},
	{"bell-features", "響鈴時要觸發哪些效果", "響鈴時要觸發哪些效果（系統音、音效檔、跳動提示、標題、邊框）", "以逗號分隔，可加 no- 前綴關閉", "no-system,no-audio,attention,title,no-border", []string{"system", "audio", "attention", "title", "border"}, "通知與響鈴"},
	{"desktop-notifications", "允許終端機裡的程式發系統通知", "允許終端機裡的程式發系統通知", "true 或 false", "true", nil, "通知與響鈴"},
	{"notify-on-command-finish", "長時間指令跑完時發通知的時機", "長時間指令跑完時發通知的時機", "", "never", []string{"never", "unfocused", "always"}, "通知與響鈴"},
	{"notify-on-command-finish-action", "指令完成通知方式", "指令跑完的通知方式：響鈴或系統通知", "以逗號分隔，可加 no- 前綴關閉", "bell,no-notify", []string{"bell", "notify"}, "通知與響鈴"},
	{"notify-on-command-finish-after", "指令跑多久以上才值得發通知", "指令跑多久以上才值得發通知", "時間，如 5s、1m30s", "5s", []string{"y", "d", "h", "m", "s", "ms", "us", "ns", "1h30m", "45s"}, "通知與響鈴"},
	{"progress-style", "允許程式顯示進度指示", "允許程式顯示進度指示", "true／false／no-osc9-4", "true", nil, "通知與響鈴"},
	// --- Shader 特效 ---
	{"custom-shader", "自訂 shader 特效", "套用自訂 GLSL shader 特效（可重複設定多筆疊加）", "shader 檔案路徑", "", nil, "Shader 特效"},
	{"custom-shader-animation", "是否讓 shader 持續動畫", "是否讓 shader 持續動畫，關掉比較省 CPU", "true／false／always", "true", nil, "Shader 特效"},
	// --- 快速終端機 ---
	{"quick-terminal-animation-duration", "快速終端機滑入滑出的動畫時間", "快速終端機滑入滑出的動畫時間", "秒，如 0.2", "0.2", nil, "快速終端機"},
	{"quick-terminal-autohide", "焦點離開時自動收起快速終端機", "焦點離開時自動收起快速終端機", "true 或 false", "true", nil, "快速終端機"},
	{"quick-terminal-keyboard-interactivity", "快速終端機何時接收鍵盤輸入", "快速終端機何時接收鍵盤輸入（僅 Linux）", "", "on-demand", []string{"none", "on-demand", "exclusive"}, "快速終端機"},
	{"quick-terminal-position", "快速終端機從哪一邊滑出來", "快速終端機從哪一邊滑出來", "", "top", []string{"top", "bottom", "left", "right", "center"}, "快速終端機"},
	{"quick-terminal-screen", "快速終端機出現在哪個螢幕", "快速終端機出現在哪個螢幕", "", "main", []string{"main", "mouse", "macos-menu-bar"}, "快速終端機"},
	{"quick-terminal-space-behavior", "跨桌面行為", "切換桌面時快速終端機要跟著移動還是留在原桌面", "", "move", []string{"move", "remain"}, "快速終端機"},
	// --- macOS 專屬 ---
	{"macos-applescript", "允許 AppleScript", "開放 AppleScript 控制 Ghostty（僅 macOS）", "true 或 false", "true", nil, "macOS 專屬"},
	{"macos-auto-secure-input", "自動安全輸入", "偵測到在輸入密碼時自動開啟安全輸入（僅 macOS）", "true 或 false", "true", nil, "macOS 專屬"},
	{"macos-custom-icon", "自訂 App 圖示", "自訂 App 圖示（僅 macOS）", "圖示檔的絕對路徑", "", nil, "macOS 專屬"},
	{"macos-dock-drop-behavior", "拖到 Dock 的行為", "把檔案拖到 Dock 圖示時要開新分頁還是新視窗（僅 macOS）", "", "new-tab", []string{"new-tab", "new-window"}, "macOS 專屬"},
	{"macos-hidden", "從 Dock 隱藏", "把 Ghostty 從 Dock 和切換器裡隱藏（僅 macOS）", "", "never", []string{"never", "always"}, "macOS 專屬"},
	{"macos-icon", "選擇 App 圖示樣式", "選擇 App 圖示樣式（僅 macOS）", "", "official", []string{"official", "blueprint", "custom", "custom-style"}, "macOS 專屬"},
	{"macos-icon-frame", "App 圖示外框材質", "App 圖示外框材質（僅 macOS）", "", "aluminum", []string{"aluminum", "beige", "plastic", "chrome"}, "macOS 專屬"},
	{"macos-icon-ghost-color", "App 圖示上小鬼的顏色", "App 圖示上小鬼的顏色（僅 macOS）", "色碼", "", nil, "macOS 專屬"},
	{"macos-icon-screen-color", "App 圖示上螢幕的顏色", "App 圖示上螢幕的顏色（僅 macOS）", "色碼，可用漸層多個色", "", nil, "macOS 專屬"},
	{"macos-non-native-fullscreen", "用非原生方式全螢幕", "用非原生方式全螢幕，切換比較快也不會開新桌面（僅 macOS）", "", "false", []string{"true", "false", "visible-menu", "padded-notch"}, "macOS 專屬"},
	{"macos-option-as-alt", "Option 當 Alt", "把 Option 鍵當成 Alt 用，寫終端機程式常需要（僅 macOS）", "true／false／left／right", "", nil, "macOS 專屬"},
	{"macos-secure-input-indication", "安全輸入啟用時顯示視覺提示", "安全輸入啟用時顯示視覺提示（僅 macOS）", "true 或 false", "true", nil, "macOS 專屬"},
	{"macos-shortcuts", "允許捷徑 App", "允許「捷徑」App 控制 Ghostty（僅 macOS）", "", "ask", []string{"ask", "allow", "deny"}, "macOS 專屬"},
	{"macos-titlebar-proxy-icon", "標題列上的檔案代理圖示是否顯示", "標題列上的檔案代理圖示是否顯示（僅 macOS）", "", "visible", []string{"visible", "hidden"}, "macOS 專屬"},
	{"macos-titlebar-style", "標題列樣式", "標題列樣式，transparent 可做出無邊框感（僅 macOS）", "native／transparent／tabs／hidden", "transparent", nil, "macOS 專屬"},
	{"macos-window-buttons", "紅綠燈視窗按鈕是否顯示", "紅綠燈視窗按鈕是否顯示（僅 macOS）", "", "visible", []string{"visible", "hidden"}, "macOS 專屬"},
	{"macos-window-shadow", "視窗陰影", "視窗陰影（僅 macOS）", "true 或 false", "true", nil, "macOS 專屬"},
	// --- Linux 專屬 ---
	{"class", "應用程式的 class 名稱", "應用程式的 class 名稱，Linux 視窗管理器辨識用", "", "", nil, "Linux 專屬"},
	{"gtk-custom-css", "載入自訂 CSS 美化介面", "載入自訂 CSS 美化介面（僅 Linux GTK）", "CSS 檔案路徑", "", nil, "Linux 專屬"},
	{"gtk-opengl-debug", "OpenGL 除錯訊息", "開啟 GTK OpenGL 除錯訊息（僅 Linux）", "true 或 false", "false", nil, "Linux 專屬"},
	{"gtk-quick-terminal-layer", "快速終端機視窗的圖層高度", "快速終端機視窗的圖層高度（僅 Linux）", "", "top", []string{"overlay", "top", "bottom", "background"}, "Linux 專屬"},
	{"gtk-quick-terminal-namespace", "快速終端機視窗的命名空間", "快速終端機視窗的命名空間（僅 Linux）", "字串", "ghostty-quick-terminal", nil, "Linux 專屬"},
	{"gtk-single-instance", "是否只跑單一實例", "是否只跑單一實例，所有視窗共用一個行程（僅 Linux）", "true／false／detect", "detect", nil, "Linux 專屬"},
	{"gtk-tabs-location", "分頁列要靠哪一邊", "分頁列要靠哪一邊（僅 Linux）", "top／bottom／left／right／hidden", "top", nil, "Linux 專屬"},
	{"gtk-titlebar", "顯示完整的 GTK 標題列", "顯示完整的 GTK 標題列（僅 Linux）", "true 或 false", "true", nil, "Linux 專屬"},
	{"gtk-titlebar-hide-when-maximized", "視窗最大化時隱藏標題列", "視窗最大化時隱藏標題列（僅 Linux）", "true 或 false", "false", nil, "Linux 專屬"},
	{"gtk-titlebar-style", "GTK 標題列樣式", "GTK 標題列樣式（僅 Linux）", "native 或 tabs", "native", nil, "Linux 專屬"},
	{"gtk-toolbar-style", "上下工具列的外觀", "上下工具列的外觀（僅 Linux）", "", "raised", []string{"flat", "raised", "raised-border"}, "Linux 專屬"},
	{"gtk-wide-tabs", "分頁是否撐滿寬度", "分頁是否撐滿寬度（僅 Linux）", "true 或 false", "true", nil, "Linux 專屬"},
	{"linux-cgroup", "systemd scope 隔離", "把每個終端機放進獨立的 systemd scope（僅 Linux）", "", "never", []string{"never", "always", "single-instance"}, "Linux 專屬"},
	{"linux-cgroup-hard-fail", "cgroup 失敗時報錯", "建立 cgroup 失敗時是否直接報錯（僅 Linux）", "true 或 false", "false", nil, "Linux 專屬"},
	{"linux-cgroup-memory-limit", "每個終端機行程的記憶體上限", "每個終端機行程的記憶體上限（僅 Linux）", "位元組數", "", nil, "Linux 專屬"},
	{"linux-cgroup-processes-limit", "每個終端機行程的子行程數上限", "每個終端機行程的子行程數上限（僅 Linux）", "整數", "", nil, "Linux 專屬"},
	{"x11-instance-name", "X11 實例名稱", "X11 WM_CLASS 的實例名稱（僅 Linux）", "字串", "", nil, "Linux 專屬"},
	// --- 進階與系統 ---
	{"async-backend", "底層非同步 IO 的實作方式", "底層非同步 IO 的實作方式，一般不用動", "", "auto", []string{"auto", "epoll", "io_uring"}, "進階與系統"},
	{"auto-update", "自動更新行為", "自動更新行為：關閉／只檢查／自動下載", "", "", []string{"off", "check", "download"}, "進階與系統"},
	{"auto-update-channel", "更新頻道", "自動更新的版本頻道：穩定版或最新開發版", "", "", []string{"stable", "tip"}, "進階與系統"},
	{"command-palette-entry", "在指令面板裡加入自訂項目", "在指令面板裡加入自訂項目（可重複設定多筆）", "title:\"名稱\",action:\"動作\"", "title:\"Change Tab Title\\xe2\\x80\\xa6\",description:\"Prompt for a new title for the current tab.\",action:\"prompt_tab_title\"", nil, "進階與系統"},
	{"config-default-files", "是否照常載入預設路徑的設定檔", "是否照常載入預設路徑的設定檔", "true 或 false", "true", nil, "進階與系統"},
	{"config-file", "額外要載入的設定檔", "額外要載入的設定檔（可重複設定多筆）——這個工具就是用它接主題", "設定檔路徑", "", nil, "進階與系統"},
	{"initial-window", "啟動時自動開視窗", "啟動 Ghostty 時是否自動開一個視窗", "true 或 false", "true", nil, "進階與系統"},
	{"language", "Ghostty 介面語言", "Ghostty 介面語言", "語言代碼，如 zh-Hant", "", nil, "進階與系統"},
	{"quit-after-last-window-closed", "關完視窗就結束", "關掉最後一個視窗後是否結束整個 App", "true 或 false", "false", nil, "進階與系統"},
	{"quit-after-last-window-closed-delay", "結束前等待時間", "關掉最後一個視窗後等多久才真的結束", "時間，如 5s", "", []string{"y", "d", "h", "m", "s", "ms", "us", "ns", "1h30m", "45s"}, "進階與系統"},
	{"window-vsync", "垂直同步", "垂直同步，關掉可能降低延遲但會有畫面撕裂", "true 或 false", "true", nil, "進階與系統"},
}
