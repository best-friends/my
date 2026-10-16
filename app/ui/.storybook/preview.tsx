// SPDX-License-Identifier: AGPL-3.0-only
// The Storybook preview of @my/ui (ADR-0027): the token stylesheet, the locale and theme toolbars,
// and the accessibility gate that fails a story test on any axe violation.
import type { Decorator, Preview } from "@storybook/react-vite";
import { type ReactNode, useEffect, useState } from "react";
import { UNSAFE_PortalProvider } from "react-aria";
import { I18nProvider } from "react-aria-components";
import "../src/storybook.css";

/**
 * The locales the client ships: the source locale, Japanese and the two pseudo-locales (ADR-0009).
 * The kit does not read the catalogs (web-client.md A2), so the list and directions are spelled out here.
 */
const locales = {
  en: { title: "English", dir: "ltr" },
  ja: { title: "日本語", dir: "ltr" },
  "en-XA": { title: "en-XA (accented pseudo-locale)", dir: "ltr" },
  "ar-XB": { title: "ar-XB (right-to-left pseudo-locale)", dir: "rtl" },
} as const;
type Locale = keyof typeof locales;

const themes = ["light", "dark"] as const;
type Theme = (typeof themes)[number];

function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && Object.hasOwn(locales, value);
}

function isTheme(value: unknown): value is Theme {
  return themes.some((theme) => theme === value);
}

type PreviewRootProps = { locale: Locale; theme: Theme; children: ReactNode };

/**
 * Applies the globals the way the app does: lang, dir and data-theme on <html> (app/ui/README.md) and
 * the React Aria locale. The frame is a size container, so components answer to its width
 * (web-client.md D2); the visual tests set its inline size.
 * Overlays (dialogs, popovers, toasts) are portalled into the frame instead of <body>, so its screenshots
 * show them. Layout containment makes the frame the containing block of fixed-position descendants too,
 * so a modal covers the frame rather than the page.
 */
function PreviewRoot({ locale, theme, children }: PreviewRootProps) {
  const { dir } = locales[locale];
  const [frame, setFrame] = useState<HTMLDivElement | null>(null);
  useEffect(() => {
    const root = document.documentElement;
    root.lang = locale;
    root.dir = dir;
    root.dataset["theme"] = theme;
  }, [locale, dir, theme]);
  return (
    <I18nProvider locale={locale}>
      <div ref={setFrame} className="contain-layout @container" data-story-frame="">
        {/* The story renders once the frame exists, so an overlay that starts open already has its container. */}
        {frame !== null && <UNSAFE_PortalProvider getContainer={() => frame}>{children}</UNSAFE_PortalProvider>}
      </div>
    </I18nProvider>
  );
}

const withGlobals: Decorator = (Story, { globals }) => (
  <PreviewRoot locale={isLocale(globals["locale"]) ? globals["locale"] : "en"} theme={isTheme(globals["theme"]) ? globals["theme"] : "light"}>
    <Story />
  </PreviewRoot>
);

const preview: Preview = {
  decorators: [withGlobals],
  globalTypes: {
    locale: {
      description: "Locale: lang and dir of the page and the React Aria locale",
      toolbar: {
        title: "Locale",
        icon: "globe",
        items: Object.entries(locales).map(([value, { title }]) => ({ value, title })),
        dynamicTitle: true,
      },
    },
    theme: {
      description: "Theme: data-theme on <html>",
      toolbar: {
        title: "Theme",
        icon: "mirror",
        items: [
          { value: "light", title: "Light", icon: "sun" },
          { value: "dark", title: "Dark", icon: "moon" },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { locale: "en", theme: "light" },
  parameters: {
    layout: "padded",
    // The theme paints the page (body uses --color-surface); Storybook's backgrounds would hide it.
    backgrounds: { disable: true },
    a11y: {
      // Every story test fails on an axe violation (web-client.md Y1).
      test: "error",
      options: { runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"] } },
    },
  },
};

export default preview;
