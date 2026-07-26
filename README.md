# Ghostty Config Studio

A terminal workbench for [Ghostty](https://ghostty.org): browse and apply
themes, and build your own config files field by field — without hand-writing
config syntax or memorising key names.

> **中文 / English** — 介面與全部 200 個設定的說明都可以雙語切換，在任何畫面按
> `L` 即可，選擇會被記住。English UI and descriptions available: press `L`.

```
┌───────────────────────────────────────────────┐┌──────────────────────┐
│ 主題與設定                                    ││ 預覽                 │
│                                               ││                      │
│ ▸ snedea/campfire                             ││ ● snedea/campfire    │
│   [theme] (nature calm animated) Campfire ——  ││                      │
│   ghostty/Nord                                ││ [ COLORS ]           │
│   [theme] (dark) Ghostty 內建主題             ││   BG   ███  #0e0a06  │
└───────────────────────────────────────────────┘└──────────────────────┘
```

## 這是什麼

- **主題瀏覽器**：把 Ghostty 內建的 460+ 個主題、以及選用的社群主題包收進同一個
  可搜尋清單，游標移到哪就即時預覽該主題的**實際顏色**（不是原始文字）。
- **設定編輯器**：Ghostty 全部 200 個設定項目，依用途分成 14 個分類頁，每項都附
  **中文／英文說明與可填值範圍**（英文說明取自 Ghostty 官方文件）。用勾選的方式
  建立自己的設定檔——固定選項的直接從選單挑，不用打字。
- **不覆寫你的設定檔**：只管理 `~/.config/ghostty/config` 裡一段用註解標記包住的
  區塊，區塊外的內容完全不動。

## 需求

- [Ghostty](https://ghostty.org)
- macOS（目前只在 macOS 測試過；Linux 應該可跑但未驗證）
- bash 3.2 以上（macOS 內建即可，不需要另外裝）
- Go 1.21 以上（只有編譯 TUI 時需要）

## 安裝

```bash
git clone https://github.com/<你的帳號>/ghostty-config-studio.git
cd ghostty-config-studio
cd tui && go build -o ../ghostty-tui . && cd ..
```

可選：加進 `PATH`，這樣任何目錄都能直接呼叫。

```bash
echo "export PATH=\"$PWD:\$PATH\"" >> ~/.zshrc
```

## 開始使用

```bash
./ghostty-tui
```

**這樣就能用了。** 不需要下載任何額外素材——Ghostty 自己內建的 460+ 個主題和
全部 200 個設定項目本來就在你電腦上，工作台直接讀取。

### 選用：匯入社群設定檔集

想要更多主題（含 GLSL shader 特效、游標動畫）的話：

```bash
./ghostty-setup
```

```
Ghostty Config Studio — 素材匯入
════════════════════════════════
這些設定檔集都是選用的。完全不匯入也能用——工作台會只顯示你自己建立的設定檔。

  1) [  --  ] snedea      12 個帶 GLSL shader 的視覺主題
  2) [已匯入] naydenoff   6 個色彩主題 + 4 個字型 + 5 個完整 preset
  3) [  --  ] sahaj-b     7 個游標特效 shader

  a) 全部匯入
  r) 移除全部素材（回到空白工作台）
  q) 離開
```

可以**單獨挑**要哪幾包、隨時移除、也可以完全不匯入。素材放在 git-ignore 的
`assets/`，來源與授權見 [`NOTICE.md`](NOTICE.md)。

## TUI 操作

| 按鍵 | 動作 |
|------|------|
| `↑`/`↓` | 移動，右側即時預覽 |
| `/` | 模糊搜尋（中文也可以，例如 `/游標`、`/retro`） |
| `enter` | 套用選中的項目 |
| `s` | 把目前套用的組合存成自訂 preset |
| `n` | 新建一個自訂設定檔，進入設定編輯器 |
| `e` | 列出可編輯的自訂設定檔（可選擇編輯或按 `d` 刪除） |
| `L` | 切換中文 / English |
| `q` | 離開 |

目前生效的項目會在預覽標題前標 `●`，最近套用過的項目下次開啟時排最前面並標 `★`。

### 設定編輯器

按 `n` 或 `e` 進入。200 個設定分成 14 個分類頁：

| 按鍵 | 動作 |
|------|------|
| `←`/`→` | 切換分類頁 |
| `↑`/`↓` | 移動，右側顯示該設定的完整中文說明 |
| `空白` | 勾選/取消，勾選時跳出數值輸入 |
| `enter` | 修改已勾選項目的值 |
| `s` | 儲存 |
| `L` | 切換中文 / English |
| `esc` | 離開 |

數值輸入分兩種：**固定選項**的（50 個，例如 `cursor-style`）直接列出選項用方向鍵
選，完全不用打字；**自由數值**的（例如 `font-size`）會預先帶入 Ghostty 的預設值
並顯示可填範圍。

## 指令列介面

TUI 之外，每個分類也有獨立的數字選單指令：

```bash
./ghostty-theme              # 主題（含 --search 搜尋內建 460+ 個）
./ghostty-font               # 字型
./ghostty-preset             # 完整 preset
./ghostty-cursor             # 游標特效 shader
./ghostty-window             # 視窗透明度 / 模糊
./ghostty-cursor-style       # 游標形狀 / 閃爍
./ghostty-clipboard          # 剪貼簿行為
./ghostty-custom             # 你自己存的組合
./ghostty-theme --current    # 顯示目前每個分類的選擇
```

## 運作方式

所有設定都寫在 `~/.config/ghostty/config` 的這段標記區塊裡：

```
# >>> ghostty-picker managed >>>
# category:theme
config-file = /path/to/assets/shader-themes/config.campfire
# category:font
config-file = /path/to/assets/fonts/iosevka.conf
# <<< ghostty-picker managed <<<
```

- 區塊**外**的內容永遠不會被動到
- theme / font / cursor 各佔一行，可以獨立疊加
- preset 與自訂組合是完整組合，選了會取代整個區塊
- 刪除正在套用中的自訂設定檔時，會一併把它從這個區塊移除——否則 Ghostty 找不到
  被引用的檔案，會導致**整份設定失效**

套用後在 Ghostty 按 `cmd+shift+,` 重新載入，或開新視窗。有些設定（尤其 shader）
需要完全重啟 Ghostty 才會生效，TUI 在套用後會問你要不要立刻重啟。

## 已知限制

- 12 個 shader 主題各自內建了 `background-opacity` / `background-blur` /
  `cursor-style` 等「氛圍」設定。同時使用 shader 主題與 `ghostty-window` 這類
  獨立設定時，同名鍵的優先權判定不直覺，你獨立選的值可能被主題內建值蓋掉。
  單獨使用、或搭配色彩主題 / 內建主題 / preset 時正常。
- 只在 macOS 測試過。

## 授權

MIT — 見 [`LICENSE`](LICENSE)。

選用的設定檔集屬於各自作者，皆為 MIT，未包含在本 repo 中；來源與授權見
[`NOTICE.md`](NOTICE.md)。設計決策與實作上的取捨記在
[`DESIGN_NOTES.md`](DESIGN_NOTES.md)。
