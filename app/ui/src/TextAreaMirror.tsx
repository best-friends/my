// SPDX-License-Identifier: AGPL-3.0-only
// The mirror of a text area (ADR-0092 §3 and §4, C1): an aria-hidden box behind the native textarea that lays out the
// same text with the textarea's own computed font, letter and word spacing, line height, padding, border widths and
// tab size, under `white-space: pre-wrap`, `overflow-wrap: break-word` and `dir="auto"`, so every character lands
// where the textarea draws it. Its text is transparent: the textarea draws the text, the mirror only gives positions
// (the caret line for the autocomplete) and, for the highlight layer, the backgrounds of ranges.
// It follows the textarea's scroll and is measured again when the textarea resizes and when web fonts finish loading
// (Noto Color Emoji swaps in after the first layout). A fractional line height lets the scrolled mirror drift by up to a
// pixel in Chromium and WebKit (#1152), so a text area with a mirror has a whole-pixel line height (`layered` in
// TextArea.tsx).
import { type Ref, type RefObject, useEffect, useEffectEvent, useLayoutEffect, useRef } from "react";
import { tv } from "tailwind-variants";

/** The computed properties of the textarea that decide where each character lands; the mirror copies them. */
const copiedProperties = [
  "font-family",
  "font-size",
  "font-weight",
  "font-style",
  "font-variant",
  "font-stretch",
  "font-feature-settings",
  "font-variation-settings",
  "font-kerning",
  "font-optical-sizing",
  "letter-spacing",
  "word-spacing",
  "line-height",
  "text-transform",
  "text-indent",
  "text-rendering",
  "tab-size",
  "padding-block-start",
  "padding-block-end",
  "padding-inline-start",
  "padding-inline-end",
  "border-block-start-width",
  "border-block-end-width",
  "border-inline-start-width",
  "border-inline-end-width",
  "box-sizing",
] as const;

// The scrollbar gutter is kept on both boxes, so a scrollbar that appears in the textarea does not narrow its lines
// without narrowing the mirror's. The borders are transparent: only their widths matter.
const mirror = tv({
  base: [
    "pointer-events-none absolute inset-0 overflow-hidden border-solid border-transparent text-transparent select-none",
    "scrollbar-gutter-stable wrap-break-word whitespace-pre-wrap",
  ],
});

/** Gives a final line break of the text its line, as the textarea does. */
const zeroWidthSpace = "\u200b";

/** Copies the textarea's layout properties onto the mirror and its scroll offset. */
function copyLayout(textarea: HTMLTextAreaElement, box: HTMLElement) {
  const computed = getComputedStyle(textarea);
  for (const property of copiedProperties) box.style.setProperty(property, computed.getPropertyValue(property));
  box.scrollTop = textarea.scrollTop;
  box.scrollLeft = textarea.scrollLeft;
}

export type TextAreaMirrorProps = {
  /** The textarea it mirrors; the mirror sits in the same positioned box (`inset-0`), right before it. */
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  /** The textarea's value. */
  text: string;
  /** Where to put the caret marker, in UTF-16 units, or null for none. */
  caret: number | null;
  /** The zero-width marker at the caret; its box is on the caret's line. */
  caretRef?: Ref<HTMLSpanElement>;
  /** Called after the mirror has copied the layout or the scroll again (resize, scroll, fonts). */
  onSync?: () => void;
};

/** The aria-hidden mirror behind a text area. */
export function TextAreaMirror({ textareaRef, text, caret, caretRef, onSync }: TextAreaMirrorProps) {
  const boxRef = useRef<HTMLDivElement>(null);
  const synced = useEffectEvent(() => {
    onSync?.();
  });

  // A render can change the textarea's classes (invalid, disabled), so the layout is copied after each one.
  useLayoutEffect(() => {
    const textarea = textareaRef.current;
    const box = boxRef.current;
    if (textarea !== null && box !== null) copyLayout(textarea, box);
  });

  useEffect(() => {
    const textarea = textareaRef.current;
    const box = boxRef.current;
    if (textarea === null || box === null) return;
    const sync = () => {
      copyLayout(textarea, box);
      synced();
    };
    const onScroll = () => {
      box.scrollTop = textarea.scrollTop;
      box.scrollLeft = textarea.scrollLeft;
      synced();
    };
    textarea.addEventListener("scroll", onScroll, { passive: true });
    // What follows a resize (the autocomplete moves its anchor, which React Aria observes) waits for the next frame, so
    // it does not resize observed elements inside this observer's own delivery.
    let frame = 0;
    const observer = new ResizeObserver(() => {
      copyLayout(textarea, box);
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(synced);
    });
    observer.observe(textarea);
    const fonts = textarea.ownerDocument.fonts;
    fonts.addEventListener("loadingdone", sync);
    return () => {
      textarea.removeEventListener("scroll", onScroll);
      observer.disconnect();
      cancelAnimationFrame(frame);
      fonts.removeEventListener("loadingdone", sync);
    };
  }, [textareaRef]);

  const at = caret === null ? null : Math.min(Math.max(caret, 0), text.length);
  return (
    <div ref={boxRef} aria-hidden dir="auto" className={mirror()}>
      {at === null ? text : text.slice(0, at)}
      {at !== null && <span ref={caretRef} />}
      {at !== null && text.slice(at)}
      {/* A final line break takes up a line in the textarea only when something follows it; a zero-width space does. */}
      {zeroWidthSpace}
    </div>
  );
}
