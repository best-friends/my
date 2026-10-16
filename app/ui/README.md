# @my/ui

The shared UI kit of the web client and the Tauri app ([ADR-0026](../../docs/adr/0026-client-workspace-and-component-architecture.md)).
It ships TypeScript source and one stylesheet, `@my/ui/theme.css`, which holds the design tokens.
The rules for writing components are in [web-client.md](../../docs/architecture/web-client.md).

## Using the tokens

An app's Tailwind entry imports the theme right after Tailwind and scans the kit's source:

```css
@import "tailwindcss";
@import "@my/ui/theme.css";
@source "../../ui/src";
@source not "../../ui/src/**/*.stories.tsx";
```

Components use only the semantic tokens below as Tailwind utilities (`bg-surface-raised`, `text-fg-muted`, `border-border-strong`)
or, where a value is computed, as `var(--color-*)` in React's `style` ([D8](../../docs/architecture/web-client.md#6-レスポンシブとスタイル)).
`theme.css` resets `--color-*`, so Tailwind's default palette (`bg-red-500`, `text-white`) produces no CSS.

## Colors

Colors have two layers in [src/theme.css](src/theme.css):

- `--palette-*` is the primitive palette in OKLCH, inside the sRGB gamut so that a contrast ratio is the same on every screen.
  It is not a Tailwind namespace, so it has no utilities; only the semantic tokens refer to it.
- `--color-*` is the semantic layer. Each token is `light-dark(<light>, <dark>)` of palette colors.

| Token | Use |
| --- | --- |
| `surface` | Page background |
| `surface-raised` | Cards, menus, popovers, dialogs |
| `surface-sunken` | Inputs, wells |
| `fg` | Body text and icons |
| `fg-muted` | Secondary text: metadata, placeholders, hints |
| `accent` / `accent-hover` | Links, primary actions, selected state |
| `accent-fg` | Text and icons on `accent` / `accent-hover` |
| `danger` / `success` / `warning` | Status text on a surface, or a solid status background |
| `danger-fg` / `success-fg` / `warning-fg` | Text on the matching solid status background |
| `border` | Decorative separators between regions that are already distinct |
| `border-strong` | Boundaries of controls (inputs, checkboxes) that must be seen (WCAG 1.4.11) |
| `focus` | The focus ring |
| `overlay` | The translucent scrim behind modal dialogs and sheets |

### Contrast pairs

[src/tokens.ts](src/tokens.ts) lists every foreground / background pair the kit may draw, and `make web-check` (`make web-tokens`)
verifies each against WCAG 2.2 AA in both themes: 4.5:1 for text, 3:1 for control boundaries and the focus ring.

- On each of `surface`, `surface-raised`, `surface-sunken`:
  `fg`, `fg-muted`, `accent`, `accent-hover`, `danger`, `success`, `warning` (text) and `border-strong`, `focus` (UI).
- `accent-fg` on `accent` and `accent-hover`; `danger-fg` on `danger`; `success-fg` on `success`; `warning-fg` on `warning` (text).
- `border` and `overlay` are decorative and have no minimum.

A new semantic color needs its pairs (or a place in the decorative list) in `tokens.ts`; the check fails otherwise.
A color that only meets AA on some surfaces is not a token.

## Light and dark

`:root` declares `color-scheme: light dark`, so `light-dark()` follows the OS setting (`prefers-color-scheme`) by default.
CSS resolves it before the first paint, so the default never flashes.

The theme hook is the `data-theme` attribute on `<html>`:

| `<html>` | Theme |
| --- | --- |
| no `data-theme` | The OS setting |
| `data-theme="light"` | Light |
| `data-theme="dark"` | Dark |

The attribute is set only on `<html>`; a subtree cannot switch themes.
The `dark:` variant follows the same rule, but components rarely need it because the tokens switch by themselves.

A stored theme choice must reach `<html>` before the first paint, or the page flashes in the OS theme first.
The CSP allows no inline script, and the HTML shell is shared and cached across viewers
([ADR-0028](../../docs/adr/0028-html-shell-link-previews-and-pwa.md)), so the server cannot write a per-viewer attribute (from a cookie) into it.
The choice is therefore applied by a same-origin, render-blocking classic script (`<script src>` in `<head>`, without `defer`,
`async` or `type="module"`) that reads it from storage and sets `document.documentElement.dataset.theme`.

## Typography

- `font-sans` is OS fonts only: the platform UI font, then Japanese, Chinese and Korean system fonts (Hiragino, Yu Gothic UI, Meiryo,
  PingFang, Microsoft YaHei UI, Malgun Gothic), Arabic system fonts (Geeza Pro, Segoe UI Arabic, Tahoma), the Noto families
  of Linux and Android, and emoji. `font-mono` follows the same order. There is no serif family.
- Sizes are `text-xs` (0.75rem), `text-sm` (0.875rem), `text-base` (1rem), `text-lg` (1.125rem), `text-xl` (1.25rem), `text-2xl` (1.5rem)
  and `text-3xl` (1.875rem). Line heights are taller than Tailwind's defaults (1.7 for `text-base`) for CJK and Arabic, and `body`
  uses the `text-base` line height.

## Spacing, radius, shadow

- Spacing is Tailwind's 0.25rem step (`p-1` = 0.25rem, `gap-4` = 1rem).
- Radius: `rounded-sm` (0.25rem), `rounded-md` (0.5rem), `rounded-lg` (0.75rem), `rounded-xl` (1rem), and `rounded-full`.
- Shadow: `shadow-sm`, `shadow-md`, `shadow-lg`. In the dark theme raised surfaces are told apart by `surface-raised`, not by shadow.

## Focus

`focus-ring` is a 2px `focus` outline with a 2px offset. Apply it as `focus-visible:focus-ring`, or `data-focus-visible:focus-ring`
on React Aria Components. It is an outline, so it stays visible in forced-colors mode. Every `:focus-visible` element also gets the
same ring from the base layer when it sets none of its own.

## Motion

Transitions and animations go under `motion-safe:` only ([D6](../../docs/architecture/web-client.md#6-レスポンシブとスタイル)),
for example `motion-safe:transition-colors`; `make web-tokens` fails on `transition-*` / `animate-*` without it (`*-none` is allowed).

- Durations: `duration-fast` (100ms), `duration-normal` (200ms), `duration-slow` (300ms). A bare `transition-*` uses 200ms.
- Easing: `ease-standard` (the default), `ease-enter` (things appearing), `ease-exit` (things leaving).

## The token check

`make web-tokens` runs [scripts/check-tokens.ts](scripts/check-tokens.ts). It fails when:

- a pair in `tokens.ts` is below AA in either theme, a background in a pair is translucent, or a semantic color is in no pair and not decorative;
- a palette color is outside the sRGB gamut, or a semantic color is not `oklch()`, hex, a `var(--palette-*)` or `light-dark()` of those;
- a source file Tailwind scans for `src/storybook.css` or `app/web/src/index.css` uses a default-palette color (`text-gray-500`),
  an arbitrary color (`bg-[#fff]`, `text-(--palette-gray-50)`, `[color:red]`), or motion outside `motion-safe:`.

Other unknown class names produce no CSS; catching those is the job of the Tailwind lint rules ([D9](../../docs/architecture/web-client.md#6-レスポンシブとスタイル)).

## Form controls

[src/index.ts](src/index.ts) exports these wrappers of React Aria Components (RAC). Each takes RAC's props, is controlled or
uncontrolled as RAC is (`value` / `onChange` or `defaultValue`; `isSelected` / `defaultSelected` for toggles), and receives every
string through props or children.

| Component | Wraps | Variants and extras |
| --- | --- | --- |
| `Button` | `Button` | `variant`: `primary` / `secondary` (default) / `ghost` / `danger`; `size`: `sm` / `md` (default); `isPending` |
| `Link` | `Link` | `variant`: `inline` (default, underlined in running text) / `standalone` |
| `TextField` | `TextField` + `Input` | `label`, `description`, `errorMessage`, `placeholder` |
| `TextArea` | `TextField` + `TextArea` | as `TextField`, plus `rows` and the character count below |
| `Checkbox` | `CheckboxField` + `CheckboxButton` | children as the label, `description`, `errorMessage`, `isIndeterminate` |
| `Switch` | `SwitchField` + `SwitchButton` | children as the label, `description`, `errorMessage` |
| `RadioGroup`, `Radio` | `RadioGroup`, `RadioField` + `RadioButton` | `orientation`: `vertical` (default) / `horizontal`; a `description` per `Radio` |
| `Select`, `SelectItem` | `Select` + `Popover` + `ListBox`, `ListBoxItem` | `items` with a render function, or `SelectItem` children; single selection |

- Each component has one `tailwind-variants` recipe; its variant props are typed from the recipe (`VariantProps`).
  `className` (a string or RAC's function of the render props) is merged into the root with `tailwind-merge`, so a later class wins.
- RAC links the label, the description and the error message to the control (`aria-labelledby` / `aria-describedby`).
  The error message shows while the field is invalid: `isInvalid`, a failing `validate`, or a native constraint (`isRequired`).
- States are styled from RAC's `data-*` attributes (`data-hovered:`, `data-pressed:`, `data-selected:`, `data-invalid:`).
  Keyboard focus shows `focus-ring`; text inputs show it on any focus. Controls are at least 24 px tall and 44 px under `pointer-coarse:`.
- `TextArea` with `characterLimit` and `formatRemaining` (the count message for a remaining count, negative when over) shows a count
  after the control, as the [GOV.UK character count](https://design-system.service.gov.uk/components/character-count/) does:
  - the count is counted in Unicode code points, like the server ([data-model.md](../../docs/architecture/data-model.md));
  - the visible count is linked to the text area with `aria-describedby` and hidden from the reading order, so it is read with the field;
  - a visually hidden `role="status"` region announces it when typing pauses for a second, and at once when the remaining count
    reaches 50, 20 or 0 ([ADR-0027](../../docs/adr/0027-ui-quality-gates.md));
  - longer text is kept, not cut off: the count turns `danger` and the field becomes invalid.

## Component tests

Each component has Testing Library tests next to it (`src/<Component>.test.tsx`) that find elements by role and drive them with
`user-event`: keyboard operation and the label / description / error association. They run in jsdom in the `unit` project of
[vitest.config.ts](vitest.config.ts) (`bun run test:unit`), which `make web-test` (part of `make check`) runs.

## Stories

Every component has a story next to it (`src/<Component>.stories.tsx`, CSF 3). `make storybook` opens Storybook 10 on port 6006;
its toolbar switches the locale (`lang` / `dir` on `<html>` and React Aria's `I18nProvider`) and the theme (`data-theme`).
[src/Tokens.stories.tsx](src/Tokens.stories.tsx) shows the color pairs, the type scale in English, Japanese and Arabic, radius,
shadow and the focus ring. [src/storybook.css](src/storybook.css) is the Storybook preview stylesheet.

Stories are also the kit's browser tests ([vitest.config.ts](vitest.config.ts)). `make ui-test` runs each story in Chromium in
`en` and `ar-XB`, light and dark; a story fails on any axe violation or when it differs from its screenshots at 320, 640 and
1024 px container widths. The screenshots in `src/__screenshots__/` come only from the pinned Playwright image: regenerate
them with `make ui-snapshots-update`, and compare like CI with `make ui-test-docker` (both need Docker).
The test procedure is in [web-client.md §7](../../docs/architecture/web-client.md#7-テスト).
