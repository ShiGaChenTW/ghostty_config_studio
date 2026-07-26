# Third-Party Config Packs

This repository contains **no third-party config files**. The optional packs
below are downloaded on demand by `./ghostty-setup`, into `assets/`, which is
git-ignored. Each import also copies that project's `LICENSE` alongside its
files.

All three are MIT-licensed, so redistributing them would have been permitted —
they are kept out of this repository so the packs stay owned and updated by
their authors, and so you always import the current upstream version rather
than a snapshot frozen here.

| Pack | Upstream | License | Contents |
|------|----------|---------|----------|
| `snedea` | <https://github.com/snedea/ghostty-themes> | MIT | 12 GLSL-shader visual themes |
| `naydenoff` | <https://github.com/naydenoff/ghostty-config> | MIT | 6 color themes, 4 fonts, 5 presets |
| `sahaj-b` | <https://github.com/sahaj-b/ghostty-cursor-shaders> | MIT | 7 cursor-effect shaders |

Importing nothing at all is a fully supported setup — see the README.

## Built-in theme catalog

The 460+ themes offered by `ghostty-theme --search` and listed in the TUI are
**not** redistributed here either. They are read from the Ghostty installation
already on your machine, via `ghostty +list-themes`.

## Config-key catalog

`tui/keycatalog.go` lists every Ghostty configuration key with its default and
its documented legal values. Those facts were extracted by parsing the output
of `ghostty +show-config --default=true --docs=true` from a local Ghostty
install; Ghostty itself is MIT-licensed. The English descriptions are
Ghostty's own documentation text from that same output (17 keys whose doc
line is empty or generic boilerplate are hand-written). The Traditional
Chinese names, descriptions, value hints, and the category assignments are
original to this project.

## Evaluated and not included

`anhsirk0/ghostty-themes`, `lexrus/lex-ghostty-shaders`,
`thijskok/ghostty-shaders`, and `0xhckr/ghostty-shaders` were considered as
additional packs. The first three ship without a `LICENSE` file or under a
"personal use only" note, so they are not offered for import.
