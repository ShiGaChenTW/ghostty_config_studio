# Ghostty Config Studio

A terminal workbench for [Ghostty](https://ghostty.org): browse and apply
themes, and build your own config files field by field — without hand-writing
config syntax or memorising key names.

**[Website](https://shigachentw.github.io/ghostty_config_studio/)** · **[English](#english)** · **[繁體中文](#繁體中文)**

The app itself is bilingual too. Press `L` on any screen to switch; your choice
is remembered, and the shell commands follow it.

```
  GHOSTTY CONFIG STUDIO  ·  Browser                              489 items

 ┌────────────────────────────┐┌──────────────────────────────────────────┐
 │ Themes & Settings          ││ Preview                                  │
 │                            ││                                          │
 │ │ snedea/campfire          ││ ● snedea/campfire                        │
 │ │ [theme] (nature calm)…   ││                                          │
 │                            ││ [ COLORS ]                               │
 │   ghostty/Nord             ││   BG   ███  #0e0a06                      │
 │   [theme] (dark) built-in  ││   FG   ███  #d9c8a8                      │
 └────────────────────────────┘└──────────────────────────────────────────┘
```

---

## English

### What it is

- **Theme browser** — Ghostty's 460+ built-in themes plus optional community
  packs in one searchable list. Move the cursor and the right pane previews the
  theme's **actual colors**, not its raw text.
- **Settings editor** — all 200 Ghostty settings across 14 category pages, each
  with a description and its legal value range. Build a config by ticking
  boxes; settings with fixed choices give you a picker instead of a text field.
- **Non-destructive** — it manages exactly one marker-delimited block inside
  `~/.config/ghostty/config`. Everything outside that block is never touched.

### Install

```bash
brew install shigachentw/tap/ghostty-config-studio
```

Requires macOS and [Ghostty](https://ghostty.org). Go is only used at build
time by Homebrew — you don't need it installed.

<details>
<summary>From source instead</summary>

```bash
git clone https://github.com/ShiGaChenTW/ghostty_config_studio.git
cd ghostty_config_studio
cd tui && go build -o ../ghostty-tui . && cd ..
echo "export PATH=\"$PWD:\$PATH\"" >> ~/.zshrc
```

Needs Go 1.21+ and bash 3.2+ (macOS ships both — bash 3.2 is the stock shell).
</details>

### Getting started

```bash
ghostty-tui
```

**That's the whole setup.** Nothing to download — Ghostty's own 460+ themes and
all 200 settings already live on your machine, and the workbench reads them
directly.

#### Optional: community config packs

For more themes, including GLSL shader effects and cursor animations:

```bash
ghostty-setup
```

```
  1) [  --  ] snedea      12 visual themes with GLSL shaders
  2) [ done ] naydenoff   6 color themes, 4 fonts, 5 full presets
  3) [  --  ] sahaj-b     7 cursor-effect shaders

  a) import all
  r) remove everything (back to a blank workbench)
  q) quit
```


Pick packs **individually**, remove them any time, or import nothing at all.
Everything imported — and every config you create — lives in
`~/.config/ghostty-config-studio/`, so it survives `brew upgrade` and even
`brew uninstall`. Sources and licenses: [`NOTICE.md`](NOTICE.md).

### Keys

| Key | Action |
|-----|--------|
| `↑`/`↓` | Move; the right pane previews as you go |
| `/` | Fuzzy search over name, source, tag and description, indexed in both languages |
| `enter` | Apply the selected item |
| `s` | Save the current combination as a custom preset |
| `n` | Create a new config and open the settings editor |
| `e` | List your editable configs (`d` deletes one) |
| `L` | Switch 中文 / English |
| `q` | Quit |

Active items are marked `●` in the preview title; recently applied ones sort to
the top on next launch, marked `★`.

#### Settings editor

Opened with `n` or `e`. 200 settings across 14 category pages:

| Key | Action |
|-----|--------|
| `←`/`→` | Switch category page |
| `↑`/`↓` | Move; the right pane shows the full description |
| `space` | Tick / untick — ticking opens the value input |
| `enter` | Change a ticked setting's value |
| `s` | Save |
| `L` | Switch 中文 / English |
| `esc` | Back |

Two kinds of input: **fixed-choice** settings (50 of them, e.g. `cursor-style`)
list their options for arrow-key selection with no typing at all;
**free-value** settings (e.g. `font-size`) prefill Ghostty's default and show
the accepted range.

### Command line

Each category also has a standalone numbered-menu command. They read the same
language file the TUI writes, so pressing `L` in the TUI also switches what
these print.

```bash
ghostty-theme              # themes (--search browses the built-in 460+)
ghostty-font               # fonts
ghostty-preset             # full presets
ghostty-cursor             # cursor-effect shaders
ghostty-window             # background opacity / blur
ghostty-cursor-style       # cursor shape / blink
ghostty-clipboard          # clipboard behaviour
ghostty-custom             # your own saved combinations
ghostty-theme --current    # show the active selection per category
ghostty-tui --version
```

### How it works

Every selection is written into this marked block in
`~/.config/ghostty/config`:

```
# >>> ghostty-picker managed >>>
# category:theme
config-file = /Users/you/.config/ghostty-config-studio/assets/shader-themes/config.campfire
# category:font
config-file = /Users/you/.config/ghostty-config-studio/assets/fonts/iosevka.conf
# <<< ghostty-picker managed <<<
```

- Content **outside** the block is never modified
- theme / font / cursor each take one line and stack independently
- presets and custom combinations are complete sets — picking one replaces the
  whole block
- deleting a config that's currently applied also removes it from this block —
  otherwise Ghostty can't open the referenced file and **the entire config
  falls back to defaults**

After applying, reload with `cmd+shift+,` or open a new window. Some settings —
shaders especially — need a full quit and relaunch; the TUI offers to restart
Ghostty for you.

### Known limitations

- The 12 shader themes bake in their own `background-opacity`,
  `background-blur` and `cursor-style` as part of their ambiance. Combining one
  with an independent choice for the same key gives unintuitive precedence, and
  your explicit pick may lose. They behave predictably on their own, and
  alongside color themes, built-in themes and presets.
- Only tested on macOS.

### License

MIT — see [`LICENSE`](LICENSE).

The optional config packs belong to their respective authors (all MIT) and are
not included in this repository; see [`NOTICE.md`](NOTICE.md). Design decisions,
including the ones only learned by getting them wrong first, are in
[`DESIGN_NOTES.md`](DESIGN_NOTES.md).

---

## 繁體中文

### 這是什麼

- **主題瀏覽器**：把 Ghostty 內建的 460+ 個主題、以及選用的社群主題包收進同一個
  可搜尋清單，游標移到哪就即時預覽該主題的**實際顏色**，不是原始文字。
- **設定編輯器**：Ghostty 全部 200 個設定項目，依用途分成 14 個分類頁，每項都附
  說明與可填值範圍。用勾選的方式建立自己的設定檔——固定選項的直接從選單挑，
  不用打字。
- **不覆寫你的設定檔**：只管理 `~/.config/ghostty/config` 裡一段用註解標記包住的
  區塊，區塊外的內容完全不動。

### 安裝

```bash
brew install shigachentw/tap/ghostty-config-studio
```

一行就好。需要 macOS 與 [Ghostty](https://ghostty.org)；Go 只在 Homebrew 編譯時
用到，你不需要自己裝。

<details>
<summary>不用 Homebrew，直接從原始碼裝</summary>

```bash
git clone https://github.com/ShiGaChenTW/ghostty_config_studio.git
cd ghostty_config_studio
cd tui && go build -o ../ghostty-tui . && cd ..
echo "export PATH=\"$PWD:\$PATH\"" >> ~/.zshrc
```

需要 Go 1.21 以上與 bash 3.2 以上（macOS 兩者都內建，bash 3.2 就是系統預設）。
</details>

### 開始使用

```bash
ghostty-tui
```

**這樣就能用了。** 不需要下載任何額外素材——Ghostty 自己內建的 460+ 個主題和
全部 200 個設定項目本來就在你電腦上，工作台直接讀取。

#### 選用：匯入社群設定檔集

想要更多主題（含 GLSL shader 特效、游標動畫）的話：

```bash
ghostty-setup
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

可以**單獨挑**要哪幾包、隨時移除、也可以完全不匯入。匯入的素材與你自己建立的
設定檔都放在 `~/.config/ghostty-config-studio/`，不會被 `brew upgrade` 或
`brew uninstall` 帶走。來源與授權見 [`NOTICE.md`](NOTICE.md)。

### 操作按鍵

| 按鍵 | 動作 |
|------|------|
| `↑`/`↓` | 移動，右側即時預覽 |
| `/` | 模糊搜尋，比對名稱、來源、分類標籤與描述，中英文都可以（例如 `/游標`、`/cursor`） |
| `enter` | 套用選中的項目 |
| `s` | 把目前套用的組合存成自訂 preset |
| `n` | 新建一個自訂設定檔，進入設定編輯器 |
| `e` | 列出可編輯的自訂設定檔（可選擇編輯或按 `d` 刪除） |
| `L` | 切換中文 / English |
| `q` | 離開 |

目前生效的項目會在預覽標題前標 `●`，最近套用過的項目下次開啟時排最前面並標 `★`。

#### 設定編輯器

按 `n` 或 `e` 進入。200 個設定分成 14 個分類頁：

| 按鍵 | 動作 |
|------|------|
| `←`/`→` | 切換分類頁 |
| `↑`/`↓` | 移動，右側顯示該設定的完整說明 |
| `空白` | 勾選/取消，勾選時跳出數值輸入 |
| `enter` | 修改已勾選項目的值 |
| `s` | 儲存 |
| `L` | 切換中文 / English |
| `esc` | 離開 |

數值輸入分兩種：**固定選項**的（50 個，例如 `cursor-style`）直接列出選項用方向鍵
選，完全不用打字；**自由數值**的（例如 `font-size`）會預先帶入 Ghostty 的預設值
並顯示可填範圍。

### 指令列介面

TUI 之外，每個分類也有獨立的數字選單指令。它們讀的是 TUI 寫的同一個語言設定，
所以在 TUI 按 `L` 也會一併換掉這些指令的輸出語言。

```bash
ghostty-theme              # 主題（含 --search 搜尋內建 460+ 個）
ghostty-font               # 字型
ghostty-preset             # 完整 preset
ghostty-cursor             # 游標特效 shader
ghostty-window             # 視窗透明度 / 模糊
ghostty-cursor-style       # 游標形狀 / 閃爍
ghostty-clipboard          # 剪貼簿行為
ghostty-custom             # 你自己存的組合
ghostty-theme --current    # 顯示目前每個分類的選擇
ghostty-tui --version
```

### 運作方式

所有設定都寫在 `~/.config/ghostty/config` 的這段標記區塊裡：

```
# >>> ghostty-picker managed >>>
# category:theme
config-file = /Users/you/.config/ghostty-config-studio/assets/shader-themes/config.campfire
# category:font
config-file = /Users/you/.config/ghostty-config-studio/assets/fonts/iosevka.conf
# <<< ghostty-picker managed <<<
```

- 區塊**外**的內容永遠不會被動到
- theme / font / cursor 各佔一行，可以獨立疊加
- preset 與自訂組合是完整組合，選了會取代整個區塊
- 刪除正在套用中的自訂設定檔時，會一併把它從這個區塊移除——否則 Ghostty 找不到
  被引用的檔案，會導致**整份設定失效**

套用後在 Ghostty 按 `cmd+shift+,` 重新載入，或開新視窗。有些設定（尤其 shader）
需要完全重啟 Ghostty 才會生效，TUI 在套用後會問你要不要立刻重啟。

### 已知限制

- 12 個 shader 主題各自內建了 `background-opacity` / `background-blur` /
  `cursor-style` 等「氛圍」設定。同時使用 shader 主題與 `ghostty-window` 這類
  獨立設定時，同名鍵的優先權判定不直覺，你獨立選的值可能被主題內建值蓋掉。
  單獨使用、或搭配色彩主題 / 內建主題 / preset 時正常。
- 只在 macOS 測試過。

### 授權

MIT — 見 [`LICENSE`](LICENSE)。

選用的設定檔集屬於各自作者，皆為 MIT，未包含在本 repo 中；來源與授權見
[`NOTICE.md`](NOTICE.md)。設計決策與實作上踩過才學到的取捨記在
[`DESIGN_NOTES.md`](DESIGN_NOTES.md)。
