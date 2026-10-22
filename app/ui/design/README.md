**my** is a Japanese microblog for small, everyday feelings and finds — “Small thoughts. A brighter you.” The identity is *Playful Human*: a hand-lettered coral wordmark with a mango spark, warm cream grounds, soft round shapes and a serif voice for the big moments. The app's UI kit is **@my/ui** (React Aria Components + Tailwind, `app/ui`); this system restyles its components in the brand and adds the brand kit's own patterns.

## Content fundamentals

- **Write in Japanese, gently.** Plain, warm, everyday words; never shouty. Brand lines: 「小さな気持ちを、気軽にここへ。」「ちいさな思いを、安心して残せる場所。」「自分らしい場所に、整える。」
- **Latin is for eyebrows and the brand statement only.** Uppercase, tracked: `TODAY'S MOMENTS`, `WELCOME TO MY`, `A SMALL MOMENT`. Set them in `eyebrow` with `rose-text`.
- **Headings are short phrases, often ending in 。** — 「いつものひとことに、あたたかい色を。」 Pair a page heading with one `fg-muted` sentence of explanation.
- **Buttons are verbs:** 投稿する, 変更を保存, フォロー, 削除する, キャンセル. Toasts are past tense: 「下書きを保存しました」. Dialog titles ask: 「この投稿を削除しますか?」.
- **Destructive moments offer a softer path:** 「この操作は元に戻せません。まずは下書きとして保存することもできます。」
- **Errors say what happened, then what to do:** 「投稿を読み込めませんでした」 + 「もう一度試す」.
- **Emoji:** users' posts and custom emoji are welcome (☀️ 🍰 :my_heart:). UI chrome never uses emoji — it uses the @my/ui icons.
- Relative time in Japanese: 12分, 1時間, 昨日. Counts with 件 / 票: 「1,284件の投稿」「126票 · あと22時間」.
- Every label a component shows arrives through props, translated by the app; components carry no copy of their own.

## Visual foundations

**Color.** Five brand colors carry the identity:

| Token | Hex | Role |
| --- | --- | --- |
| `coral` | #FF6B6B | Primary fill — primary buttons, counts, NewItemsButton, play discs; always with a charcoal label |
| `cream` | #FFF6EE | Warm ground — wells, logo tiles, empty-state tiles, audio player |
| `rose` | #D97C7C | Secondary — waveforms and decorative shapes |
| `mango` | #FFB347 | Accent / spark — ✳ ✧ sparks; the focus ring in Dark |
| `charcoal` | #2E2E2E | Ink in Light, the monochrome logo, labels on coral |

The components speak in **@my/ui's semantic tokens**, each set to a brand value:

| @my/ui token | Light | Dark | Use |
| --- | --- | --- | --- |
| `surface` | `page` #FCF9F6 | #241F20 | the page |
| `surface-raised` | `card` #FFFFFF | #302829 | cards, posts, menus, dialogs |
| `surface-sunken` | `cream` #FFF6EE | #1C1819 | input wells, hover fills, media cells |
| `fg` / `fg-muted` | #2E2E2E / #746E6C | #F5EBEA / #B7A7A5 | text, secondary text |
| `accent` / `accent-hover` | #B64D4D / #9B3C3C | #FF8585 / #FFA8A8 | links, selected tabs and toggles, checkboxes, switches |
| `accent-fg` | #FFFFFF | #241F20 | marks on an `accent` fill |
| `danger` / `danger-fg` | #A3214A / white | #FF8FB1 / #241F20 | destructive actions, errors — a berry red kept apart from the coral |
| `success` / `success-fg` | #397557 / white | #86D0A5 / #241F20 | success, boosted |
| `warning` / `warning-fg` | #8A5A1F / white | #F7C47A / #241F20 | warnings, content warnings |
| `border` / `border-strong` | #F0E8E3 / #928A86 | #493B3D / #7D7374 | dividers / control outlines |
| `focus` | #B88133 | `mango` | the focus ring |
| `overlay` | #211C1C 62% | #0D0A0B 70% | scrims |

- **Brand additions** beside them: `accent-fill` / `accent-fill-hover` / `accent-fill-fg` (the bright coral primary fill — add `--color-accent-fill*` to `theme.css` so `primary` keeps it), `danger-bg`, `success-bg`, `warning-bg` (tinted boxes), `soft` (selection tint), `toast-*` (the dark pill of Toast and Tooltip), `avatar-*` tints, and the `logo-*` inks.
- Spend `accent-fill` on one action per view; `mango` only as a small spark. Never put white on coral.
- Two themes: **Light · Cream** and **Dark · Cocoa** (`data-theme`, else the OS).
- **Contrast: both themes meet WCAG AA** for every pair @my/ui's `check-tokens` verifies (text 4.5:1 on all three surfaces; `border-strong` and `focus` 3:1; each `-fg` on its fill) and for the brand additions. Each token's note gives its ratios.

**Type.** System fonts only, as @my/ui ships them.

- `sans` — system-ui and the CJK / Arabic system faces — for all UI. @my/ui's scale maps to the brand styles: `text-xs` 12 → `small`; `text-sm` 14 → `label` / `button`; `text-base` 16 → `body` (the brand sets UI body at 15px, post text at 15px / 1.85); `text-lg` 18 → dialog titles (the brand uses 20px/800); `text-3xl` 30 → `h1`.
- `serif` — Georgia, then Noto Serif JP — only for brand moments: `brand-display`, `page-title`, EmptyState headings, avatar initials. @my/ui sets `--font-serif: initial`; define it to use these.
- UI weights run heavy (700–800); serif headings stay light (400–500).

**Spacing.** 4 / 8 / 12 / 16 / 24 / 32px (`space-1`…`space-6`) — @my/ui's 0.25rem step. Layout: lg 3 columns (260 / ≤640 / 300px), md 2 columns under 1120px, sm 1 column with a bottom tab bar under 700px — see the Layouts group. Components answer to their container (`@container`), not the viewport.

**Shape.** Round and soft — this is the brand's main change to @my/ui. Buttons, badges, tags, toasts and the search field are pills (`radius-pill`); cards, posts and dialogs `radius-card` (19px); inputs 11px; menus 16px; list items and toggles 10px; media 16px; sheets 24px at the top; avatars `radius-avatar` (21%, the App-Icon's corner) — never circles.

**Elevation.** Borders first: cards and posts have a 1px `border` plus the barely-there `shadow-card`. Floating layers use `elevation-sm` (media badges), `elevation-md` (lists, tooltips, NewItemsButton) and `elevation-lg` (dialogs, menus, popovers, toasts); Dark drops `shadow-card` and rings the dark pills with `border`.

**Motion.** Only under `prefers-reduced-motion: no-preference`: colours ease over 200ms (`cubic-bezier(.2,0,0,1)`), the primary button lifts 1px, dialogs scale from 95%, menus and tooltips fade in 100ms, spinners spin, skeletons pulse. Animated images and custom emoji move only when the reader allows it.

**Focus.** A 3px solid `focus` ring, 2px offset, on every control (inset inside tabs and media cells, which clip).

**States.** Hover: `surface-sunken` fill or `accent-hover`. Selected: `accent` plus a change of shape (filled icon, check, dot). Invalid: `danger` border plus a WarningIcon message. Disabled: opacity .5. Pending: a Spinner, and the control keeps its focus.

**Imagery.** Warm daylight, unstaged — the brand's placeholders are soft gradient landscapes (peach, lilac, sage). Media sit in 16px-radius frames; while loading or concealed they show their Blurhash.

## Iconography

- @my/ui's **Phosphor** set (Regular weight) — see the **Icons** group: 35 names, five with an “on” shape. Import the named component (`StarIcon`, `BoostIcon`…) from `@my/ui`; never import Phosphor directly.
- Icons are `currentColor`, sized by their control (16 / 20 / 24px), decorative (`aria-hidden`), and mirror in right-to-left text when they point along the line.
- Sparks (✳ ✧) in `mango` remain the brand's one typographic ornament after a heading.

## Logo

- Use the files in **Logos**; never redraw the hand-lettered “my”.
- `Wordmark-Primary-Color` on `cream` or white; `Wordmark-LightBg` (deeper `logo-red`) on busy light grounds; `Wordmark-DarkBg` (white letters) on `logo-dark-surface` or photos; `Wordmark-Monochrome` (`charcoal`) for single-ink uses; `App-Icon` for app and avatar slots.
- Keep the three spark strokes — `mango`, `logo-spark-coral`, `logo-spark-amber` — exactly as drawn.

## Components

Each component's card shows it in the brand and lists the @my/ui props it takes. They are CSS classes in `components/bundle.css` (no JavaScript bundle) named after the @my/ui component and slot; to apply the brand in the app, map these values onto the `tv()` recipes in `app/ui/src`.

- **Forms:** Button, ToggleButton, Link, ExternalLink, TextField, TextArea, ReasonField, Select, DurationSelect, Checkbox, RadioGroup, Switch, EmojiPicker.
- **Display:** Avatar, Badge, Blurhash, DescriptionList, EmptyState, Picture, QrCode, Skeleton, SkeletonRows, Spinner, VisuallyHidden.
- **Feedback:** Announcer, OfflineBanner, RegionError, StatusMessage, Toast.
- **Overlays:** Dialog, Sheet, ConfirmDialog, Popover, Menu, Tooltip.
- **Navigation:** Tabs.
- **Timeline:** PostCard, PostContent, ContentWarning, CustomEmoji, Feed, NewItemsButton, MediaGrid, MediaViewer, LinkCard.
- **Layouts:** LayoutLanding (logged-out landing), LayoutInfoPage (server info, rules, terms — the same signed in or out), LayoutTimeline (no header and no inline composer; auto-inserted posts keep the reading position), LayoutSettings, LayoutAdmin (moderation and server administration). Each shows PC (1280px) and phone (390px).
- **Brand kit** (patterns from the brand kit that @my/ui does not have yet): Card, Poll, AudioPlayer, Pagination, RailItem, Segmented, Tag.

Where the brand departs from @my/ui's current look, the component's notes say so: posts are raised cards rather than flat rows; Toast and Tooltip are dark pills; RegionError, StatusMessage and OfflineBanner use tinted boxes with an icon instead of a start-edge bar or a solid bar.
