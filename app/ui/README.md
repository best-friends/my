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

## App-wide providers

The kit re-exports two React Aria providers that an app renders once at its root, so that the app needs no dependency on
`react-aria-components` of its own:

- `I18nProvider` with the app's locale: the components' built-in strings, number and date formats, and the direction of arrow keys.
- `RouterProvider` with the app router's `navigate`: every `Link` (and menu item or tab) with a same-origin `href` navigates
  through it instead of loading a new document. The kit itself knows no router.

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

### Type roles

Text takes its size from its role, never from the screen it is on
([web-client.md §16.2](../../docs/architecture/web-client.md#162-文字)):

| Role | Classes | Used for |
| --- | --- | --- |
| Page title | `text-2xl font-semibold` | The `h1` of `PageHeading`, one per page |
| Section title | `text-lg font-semibold` | `h2` of a region (replies, settings groups), the heading of `EmptyState` and of dialogs |
| Focused content | `text-lg` | The body of the post a detail page is about |
| Body | `text-base` | Post bodies, profile text, form values, menu items |
| Secondary | `text-sm text-fg-muted` | Handles, times, counts, descriptions and hints of fields |
| Label | `text-xs` | Badges, the labels of the bottom tab bar |

`text-xl` and `text-3xl` have no role and are not used by screens. Names (`font-semibold`) are the only other weight;
emphasis inside running text comes from the content, not from the UI.

### Names in rows

A user's name gets `dir="auto"` (Y10), which also turns `text-align: start` around for a right-to-left name.
When the `dir="auto"` element is a block (a `p`, or a `span` inside a flex or grid row), it also takes
`align-match-parent` (`text-align: match-parent`, with the `-webkit-` value Chromium needs), so it lines up with the page's
start like the rows around it and only the text inside it runs right to left
([G9](../../docs/architecture/web-client.md#162-文字)). Post bodies and content warnings keep their own direction's alignment.
The utility is not named `text-*` because `tailwind-merge` would read that as a color and drop the element's color class.

## Spacing, radius, shadow

- Spacing is Tailwind's 0.25rem step (`p-1` = 0.25rem, `gap-4` = 1rem), limited to `0.5`, `1`, `2`, `3`, `4`, `6`, `8` and `10`.
  Each role has one value:

  | Role | Value |
  | --- | --- |
  | Icon and its text, a name and its handle | `gap-1` |
  | Controls in a row (actions of a post, buttons of a dialog) | `gap-2` |
  | Avatar and the content beside it; lines inside a card | `gap-3` |
  | Inside a row of a list or a card | `px-4 py-3` |
  | Page gutter (set by the app layout only) | `px-4` |
  | Fields of a form; blocks inside a page | `gap-4` |
  | Sections of a page | `gap-6` (`gap-8` from `@3xl`) |
  | Around an `EmptyState` or a page-level spinner | `py-10` |

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

## Icons

Icons are part of the kit, so that every screen draws the same shapes at the same sizes
([web-client.md §16.3](../../docs/architecture/web-client.md#163-アイコン)):

- They are drawn in-house on a 24 × 24 grid with a 2px round stroke, like the existing `CloseIcon` and `ExternalLinkIcon`.
  A screen never writes SVG paths of its own; a missing icon is added to the kit, with a story.
- They are decorative: `aria-hidden="true"` and `focusable="false"`. The meaning is in the visible label or the accessible
  name of the control. An icon-only control has an `aria-label` and a `Tooltip` with the same text.
- The control sizes its icon: `Button`, `ToggleButton` and `Link` size a child `svg` (`size-5` in buttons, `size-4` next to
  `text-sm`), and the navigation uses `size-6`. An icon sets no size of its own, so it never renders at 0 × 0.
- A toggle shows its state by the icon's shape (filled, or a heavier stroke) as well as its color.
- Icons with a direction (reply, back, forward) mirror in right-to-left text with `rtl:-scale-x-100`.

## States of a region

The kit gives every region of data the same four states
([web-client.md §16.4](../../docs/architecture/web-client.md#164-空読み込み中エラーオフライン)):

| State | Component | Notes |
| --- | --- | --- |
| Empty | `EmptyState` | A heading of one short fact, a description of what appears here, and an action that leads on. `headingLevel` is one below the region's heading. |
| Loading | `PostSkeleton`, `AccountRowSkeleton`, `NotificationRowSkeleton` (or `Skeleton` shapes) for the first page of a list; `Spinner` for a button or a small region | Never both in one region. The region carries `aria-busy` and one loading label. |
| Error | `RegionError` | What happened (`heading`), the reason (`reason`) and the retry button (`retryLabel`, `onRetry`, `isRetrying`). It replaces only the region; content already loaded stays. |
| Offline | `OfflineBanner` | One `role="status"` bar from the app layout; regions show their own error only when a request fails. |

- The skeleton rows are hidden from assistive technology (`aria-hidden`) as a whole, so a region that repeats them
  still has only the one loading label it gives itself (a `VisuallyHidden` text, or a `Spinner`).
  `PostSkeleton` and `NotificationRowSkeleton` have the padding, bottom border and container layout of `PostCard`;
  `AccountRowSkeleton` has no border, because account lists separate their rows with `divide-y`.
- `RegionError` is not a live region by default: a failure that came with the page is found by reading.
  `alert` makes it `role="alert"`, only for the result of the user's own action, such as a retry that failed again
  ([APG alert](https://www.w3.org/WAI/ARIA/apg/patterns/alert/)). Without `headingLevel` the message is a paragraph
  (the failure of the next page of a list); with it, a heading one below the region's heading.
- `OfflineBanner` is rendered once for the whole session, empty while `isOffline` is false, so that going offline is
  announced (a live region announces only changes made after it is in the page). It sticks to the top edge
  (`position: sticky`) and pads itself for `env(safe-area-inset-top)`, so the app renders it before any element that
  already pads for that inset.

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
| `ReasonField` | `TextArea` | The reason for an operator's action: three rows and a required character count (`characterLimit`, `formatRemaining`) |
| `DurationSelect` | `Select` | `durations` in seconds with `formatDuration`; `indefiniteLabel` adds "no end", last; the value is seconds or `null` for no end |

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

## Overlays and announcements

The overlay components wrap React Aria Components, which handle focus, Esc and the reading direction
([WAI-ARIA APG](https://www.w3.org/WAI/ARIA/apg/patterns/)). All strings, including the accessible names of × buttons, come in as props.

| Component | Pattern | Notes |
| --- | --- | --- |
| `Dialog` / `Sheet` in a `DialogTrigger` | Modal dialog | Traps focus, closes on Esc, returns focus to the trigger. `closeLabel` adds the × button; `role="alertdialog"` for confirmations; `isOpen` / `onOpenChange` without a trigger. `Sheet` sits on the bottom edge below a 32rem container width, with safe-area padding and a height capped in `svh`. |
| `ConfirmDialog` in a `DialogTrigger` | Alert dialog | The confirmation before an action that cannot be undone (see below). |
| `Popover` in a `PopoverTrigger` | Non-modal dialog | Named by `title` or `aria-label`. |
| `Menu`, `MenuItem`, `MenuSeparator` in a `MenuTrigger`; `SubmenuTrigger` | Menu button | `intent="danger"` for destructive items. Submenus open towards the inline end. |
| `Tooltip` in a `TooltipTrigger` | Tooltip | Describes its trigger; never the trigger's only name. |
| `Tabs`, `TabList`, `Tab`, `TabPanel` | Tabs | `selectedKey` / `onSelectionChange` let a route keep the tab in the URL. |
| `AnnouncerProvider`, `useAnnouncer` | Live regions | One polite `role="status"` and one assertive `role="alert"` region for the app. |
| `ToastProvider`, `useToast` | Toasts | Announced through the Announcer; the visible list is a labelled region. |
| `StatusMessage` | Status | The result of an action that stays on the page: a `role="status"` region, empty until it has a message. |

Mount `AnnouncerProvider` once at the app root and `ToastProvider` inside it. `announce(message, politeness)` and
`toast.show(...)` are called from event handlers and mutation callbacks, and never move focus. Each announcement is a new node in its
region, so a repeated message is read again, and it is removed after `clearAfter` (7 s). The regions carry `data-live-announcer`, so they
stay exposed while a modal hides the rest of the page. The kit has its own regions and toast queue because React Aria Components' Toast is
`UNSTABLE_` and its live announcer is internal to react-aria. A toast without an action closes after `defaultTimeout` (5 s), but not
while the pointer is over the toasts or focus is in them; a toast with an action stays until it is dismissed.

`ConfirmDialog` follows the [APG alert dialog](https://www.w3.org/WAI/ARIA/apg/patterns/alertdialog/) on `Dialog`
(`role="alertdialog"`, named by `title`, described by `description`):

- the title is a question, the description says what happens and whether it can be undone, and the buttons are verbs
  (G22 in [web-client.md](../../docs/architecture/web-client.md)); `tone` is `danger` (default) or `primary`;
- on open, focus moves to the first field of `children` (a `ReasonField`, a `DurationSelect`), or else to the cancel button, never to the
  destructive one ([APG dialog, initial focus](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/)); Esc and cancel close it and focus
  returns to the trigger; there is no × button and a press outside does not close it;
- the body is a form: the confirm button submits it, the fields' constraints (`isRequired`) are checked first, and only then
  `onConfirm(close)` runs. The dialog stays open until `close` is called, `isPending` makes the confirm button wait, and `errorMessage`
  shows a failure in the dialog (`role="alert"`) so that the action can be tried again.

`StatusMessage` is rendered before the action, empty, and gets the result as children once the action has succeeded: a live region
announces only changes made after it is in the page. A failure is not a status; it goes to `ConfirmDialog`'s `errorMessage` or a toast.

## Display

| Component | Notes |
| --- | --- |
| `DescriptionList`, `DescriptionItem` | The facts of a record: a `<dl>` that is its own size container, with up to `columns` (1–4) columns as it widens; values wrap anywhere. |

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
1024 px container widths. The screenshots are taken with `prefers-reduced-motion: reduce` emulated, and a story fails when
anything in it still animates then; stories render and run their `play` functions (which find elements by role, with
`storybook/test`) without it. The screenshots in `src/__screenshots__/` come only from the pinned Playwright image: regenerate
them with `make ui-snapshots-update`, and compare like CI with `make ui-test-docker` (both need Docker).
A story's `play` function also tests behaviour in the real browser (focus trap and return, Esc, arrow keys in RTL, announcements that
keep focus); it runs before axe and the screenshots, in all four projects. The overlay components' jsdom tests cover the same
behaviour in `make check`. Overlays portal into the story frame, which has layout
containment, so a modal covers the frame and appears in its screenshots.
The test procedure is in [web-client.md §7](../../docs/architecture/web-client.md#7-テスト).
