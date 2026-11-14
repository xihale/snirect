---
version: alpha
name: Snirect-landing-brutal
description: "New-brutalist landing page for Snirect: paper ground, 3px ink rules, hard offset shadows, acid accent, mono for anything the tool prints."
lang: zh-CN
---

# Snirect Landing — New Brutalism

Static files only, no build step: `index.html`, `styles.css`, `script.js`, `assets/snirect_icon_light.svg`.
Rule of the page: **show the effect, not the explanation.** Every claim is a real CLI output
(`snirect rules match`, `snirect status`) or a real rule row from `internal/rules/builtin_rules.go`.

## 1. Tokens

| Token | Value | Use |
| --- | --- | --- |
| `--paper` | `#FBF9EB` | page ground (matches the app icon) |
| `--ink` | `#111111` | text, 3px rules, terminal cards |
| `--indigo` | `#403D83` | brand accent: rewrote SNI value, brand mark |
| `--teal` | `#086259` | eyebrow / secondary text |
| `--acid` | `#D9F000` | one accent only: headline underline, 命中 badge, 下载 button |
| `--muted` | `#6B6B63` | captions, one-line lede |
| `--rule` | `3px solid var(--ink)` | every border |
| `--shadow` | `10px 10px 0 var(--ink)` | hard offset, never blurred |

Type: system sans for Chinese display (900), `ui-monospace` for anything the CLI prints.
Spacing on a 4px grid. Radius: 0. No gradients, no blurred shadows, no rounded pills.

## 2. Page structure

1. **Header** — wordmark + `CLI / 规则 / Releases / 下载`.
2. **Hero** — `改写 TLS SNI` (96px, acid underline on the latin half) + a packet card that shows the
   rewrite itself: `www.android.com` struck through → `g.cn`.
3. **Results** — two terminal cards, real output only:
   `snirect rules match www.android.com` (rewrite) and `snirect rules match github.com` (SNI stripped),
   plus `snirect status`.
4. **Split** — 命中 → MITM / 未命中 → 直连.
5. **Install** — `snirect cert install && snirect run -s` on an ink band, with a copy button.
6. **Platforms / footer** — Linux, macOS, Windows, Android.

## 3. Rules

- Copy is one line per idea. If a sentence can be deleted without losing the effect, delete it.
- Terminal cards print what the tool prints: never edit the output to look better.
- Accent covers <5% of the page. One acid element per screen region.
- Hard shadows are offset down-right and stay visible — never `box-shadow: none`.
- Hover moves the element up-left by 2px; press returns it. Nothing fades in except the entrance.
