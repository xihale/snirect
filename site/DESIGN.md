---
version: alpha
name: Snirect-landing-design-v3
extends: ./DESIGN.base.md   # linear.app reference, read-only (inverse-* light palette)
description: "Light, airy, generous-whitespace landing page for Snirect derived from snirect_icon_light.svg and DESIGN.base.md inverse tokens."
lang: zh-CN
---

# Snirect Landing Page — Design Spec v3

Light, calm, documentation-first aesthetic with generous whitespace ("留白") and minimal visual chrome. Static files only (no build step, no `package.json`): `index.html` (≤170 lines), `styles.css` (≤180 lines), `script.js` (≤145 lines), `assets/snirect_icon_light.svg`.

## 1. Palette & Tokens

| Token | Value | Use |
| :-- | :-- | :-- |
| `--bg` | `#ffffff` | Page canvas (`color-scheme: light`, `theme-color: #ffffff`) |
| `--s1` | `#f7f8fa` | **Only** `#how` and `#quickstart` soft cards |
| `--s2` | `#f0f2f5` | Hover surface for secondary/copy controls |
| `--hair` | `#eaeef2` | Hairline borders, `#docs` row dividers, `#how` row separator |
| `--hair-strong` | `#d8dee4` | Node borders in `#how`, popover border |
| `--ctl` | `#85909a` | Interactive borders (≥3:1 contrast on `#ffffff` and `#f7f8fa`) |
| `--ink` | `#191c1f` | Headings, primary copy, code ink |
| `--ink-2` | `#4b5563` | Hero lede, prose body |
| `--ink-3` | `#64707d` | Eyebrow, muted labels, doc paths, comments, footer |
| `--indigo` | `#403d83` | Primary CTA (`hover #35326e`, `active #2b2959`), incoming SNI accent, active indicators, focus ring |
| `--teal` | `#086259` | **Restrained**: only the 3-element brand mark outbound line and outbound rewritten-SNI arrow/label in `#how` |

Brand mark: `assets/snirect_icon_light.svg` for favicon; bare 3-element inline SVG (`#403D83` incoming line + center circle, `#086259` outgoing line, `aria-hidden="true"`, no squircle `<rect>`) in navbar and footer.

## 2. Typography, Whitespace & Unboxing

- Fonts: `'Inter', -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Noto Sans CJK SC', 'Microsoft YaHei', sans-serif`; mono `'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`.
- `text-wrap: balance` strictly on `h1, h2, h3`; `text-wrap: pretty` strictly on `p, li`.
- Container: `--w: 840px`, `padding-inline: 24px`.
- Rhythm: Hero `padding-block: clamp(88px, 13vw, 144px) 64px`; Sections `padding-block-end: clamp(96px, 11vw, 136px)`.
- **Unboxing**: ONLY `#how` (flow diagram) and `#quickstart` (tabbed snippet card) sit inside `#f7f8fa` cards (`1px solid var(--hair)`, `border-radius: 14px`). `#features` and `#docs` sit directly on `#ffffff` with zero boxes or card borders (`#docs` uses `1px solid var(--hair)` bottom hairlines only).

## 3. Page Blueprint

1. **Sticky Header (`60px`, `rgba(255,255,255,0.86)` + `backdrop-filter: blur(12px)`)**:
   - Left: bare mark + `Snirect`; Nav (`aria-label="主导航"`): `原理` (`#how`), `开始` (`#quickstart`), `文档` (`#docs`); Right: `GitHub`, `下载`.
   - Responsive: `ResizeObserver` toggles `.is-wide` and `setAttribute`/`removeAttribute('popover', 'auto')` on `#menu`; popover uses static `inset-block-start: 64px` before `anchor(--menu-button bottom, 64px)` and `:is(:popover-open, .\:popover-open)`. `@oddbird/popover-polyfill` loads only when `!HTMLElement.prototype.hasOwnProperty('popover')`.
2. **Hero**: Eyebrow `v0.5.0 · MIT`, H1 `按规则改写 TLS SNI 的本地代理`, 1-sentence lede, CTAs `快速开始` (`#quickstart`) + `GitHub Releases`.
3. **`#how`**: Single `#f7f8fa` card (`padding: 28px 32px`), 2 horizontal flow rows separated by `1px solid var(--hair)`, stacking neatly `<600px`:
   - 命中规则: `ClientHello` — *原始 SNI* (`#403D83`) → **Snirect 本地 CA 中转** — *改写/剥离 SNI* (`#086259`) → `目标前置 IP`
   - 常规站点: `ClientHello` — *TCP 原样透传 · 零解密* (`var(--ink-3)`) → `远端服务器`
4. **`#features`**: H2 `克制而可靠` (no subtitle), open 2×2 typography grid (`gap: 36px 56px`, 1-col `<640px`): `静态内置规则`, `按需无感分流`, `内建加密 DNS`, `全平台轻量运行`.
5. **`#quickstart`**: H2 `快速开始`, single `#f7f8fa` card with 5 tabs (`Linux`, `macOS`, `Windows`, `Android`, `dae / Clash`): 4 copyable `<pre><code>` panels + `Android` summary (`VpnService` + `gVisor netstack`, `10.0.0.2/24`, `OnCertRejected` auto-exclusion) with `下载 APK (v0.4.1) ↗` and `证书说明 ↗`.
6. **`#docs`**: H2 `文档`, 6 single-line flex rows (`padding-block: 14px`, `border-block-end: 1px solid var(--hair)`): title left, monospace path right linking to `https://github.com/xihale/snirect/blob/main/<path>`.
7. **Footer**: Single airy row: bare mark · `MIT © 2026 xihale` · `致谢 Cealing-Host & gVisor` · `GitHub` · `Releases` · `AUR`.

## 4. Interaction & Accessibility Contracts

- **Tabs**: `role="tablist"`, `focusgroup="tablist nomemory"`, `focusgroupstart` initially on `#t-linux` and dynamically toggled via `tab.toggleAttribute('focusgroupstart', active)`, no `aria-controls` on `[role="tab"]`, active underline via `::after` `border-block-end`, `.tab-panels { display: grid }`, bare `.tabpanel { grid-area: 1 / 1 }` (padding strictly on `.tabpanel-inner`), `hidden="until-found"` + `beforematch` sync, Arrow/Home/End fallback with `scrollIntoView({ block: 'nearest', inline: 'nearest', container: 'nearest' })`.
- **Copy**: `command="--copy"` + `commandfor`, `28px` min size (`44px` coarse pointer), capture-phase `command` + `!('CommandEvent' in window)` click fallback, `.tip[role="status"]` via `opacity`, `WeakMap` timer + `lastMsg` `WeakMap` for VoiceOver NBSP alternation.
- **A11y**: `:focus-visible`, `prefers-reduced-motion: reduce`, `forced-colors: active`, zero horizontal overflow at `375px` and `1440px`.
