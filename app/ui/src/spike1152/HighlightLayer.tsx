// SPDX-License-Identifier: AGPL-3.0-only
// Spike #1152 (ADR-0092 §4, C1): the mirror layer behind a native textarea. It lays out the same text with the
// textarea's computed font, letter spacing, line height, padding and borders, `white-space: pre-wrap`,
// `overflow-wrap: break-word` and `dir="auto"`, paints only the backgrounds of the token ranges (the text itself is
// transparent; the textarea draws it), follows the textarea's scroll and is re-measured by a ResizeObserver and when
// web fonts finish loading. A zero-width marker at the caret gives the caret line's position for the popup.
import { type RefObject, useCallback, useEffect, useImperativeHandle, useLayoutEffect, useRef } from "react";
import type { TextToken } from "./textTokens";

/** The computed properties of the textarea the layer copies (they decide where each character lands). */
const copiedProperties = [
  "fontFamily",
  "fontSize",
  "fontWeight",
  "fontStyle",
  "fontVariant",
  "fontStretch",
  "fontFeatureSettings",
  "fontVariationSettings",
  "fontKerning",
  "fontOpticalSizing",
  "letterSpacing",
  "wordSpacing",
  "lineHeight",
  "textTransform",
  "textIndent",
  "textRendering",
  "tabSize",
  "paddingTop",
  "paddingRight",
  "paddingBottom",
  "paddingLeft",
  "borderTopWidth",
  "borderRightWidth",
  "borderBottomWidth",
  "borderLeftWidth",
  "boxSizing",
] as const;

/** Spike switch: follow the textarea's scroll with the layer's own scrollTop, or with a transform of its content. */
export const layerOptions: { scrollMode: "scrollTop" | "transform" } = { scrollMode: "scrollTop" };

/** Applies the textarea's scroll offset to the layer. */
function followScroll(ta: HTMLTextAreaElement, layer: HTMLDivElement) {
  const inner = layer.firstElementChild as HTMLElement | null;
  if (layerOptions.scrollMode === "transform") {
    layer.scrollTop = 0;
    layer.scrollLeft = 0;
    if (inner !== null) inner.style.transform = `translate(${-ta.scrollLeft}px, ${-ta.scrollTop}px)`;
    return;
  }
  if (inner !== null) inner.style.transform = "";
  layer.scrollTop = ta.scrollTop;
  layer.scrollLeft = ta.scrollLeft;
}

export interface HighlightLayerHandle {
  /** The caret line's bottom edge and the textarea's box, relative to the textarea's offset parent. */
  caretLineBottom: () => number | null;
  /** Re-measure (copy the styles and sizes again). */
  sync: () => void;
}

export interface HighlightLayerProps {
  textareaRef: RefObject<HTMLTextAreaElement | null>;
  text: string;
  tokens: readonly TextToken[];
  /** Characters past this many code points are painted as over the limit. */
  characterLimit?: number;
  /** The caret offset in UTF-16 units, where the marker goes. */
  caret: number;
  /** Debug: paint the layer's text instead of leaving it transparent (the alignment harness). */
  showText?: boolean;
  ref?: RefObject<HighlightLayerHandle | null>;
}

interface Segment {
  text: string;
  kind: TextToken["kind"] | "over" | null;
}

/** The text split into token and plain runs, with the part past the limit as its own kind. */
function segments(text: string, tokens: readonly TextToken[], characterLimit?: number): Segment[] {
  const out: Segment[] = [];
  let at = 0;
  for (const t of tokens) {
    if (t.start > at) out.push({ text: text.slice(at, t.start), kind: null });
    out.push({ text: text.slice(t.start, t.end), kind: t.kind });
    at = t.end;
  }
  if (at < text.length) out.push({ text: text.slice(at), kind: null });
  if (characterLimit === undefined) return out;
  // Split at the UTF-16 offset of the limit-th code point.
  const limitOffset = Array.from(text).slice(0, characterLimit).join("").length;
  if (limitOffset >= text.length) return out;
  const split: Segment[] = [];
  let pos = 0;
  for (const s of out) {
    const end = pos + s.text.length;
    if (end <= limitOffset) split.push(s);
    else if (pos >= limitOffset) split.push({ text: s.text, kind: "over" });
    else {
      split.push({ text: s.text.slice(0, limitOffset - pos), kind: s.kind });
      split.push({ text: s.text.slice(limitOffset - pos), kind: "over" });
    }
    pos = end;
  }
  return split;
}

const markClass: Record<NonNullable<Segment["kind"]>, string> = {
  mention: "rounded-sm bg-accent/20 forced-colors:underline",
  hashtag: "rounded-sm bg-accent/20 forced-colors:underline",
  url: "rounded-sm bg-accent/15 forced-colors:underline",
  shortcode: "rounded-sm bg-badge-yellow forced-colors:underline",
  over: "bg-danger/25 forced-colors:underline",
};

export function HighlightLayer({ textareaRef, text, tokens, characterLimit, caret, showText = false, ref }: HighlightLayerProps) {
  const layerRef = useRef<HTMLDivElement>(null);
  const markerRef = useRef<HTMLSpanElement>(null);

  const sync = useCallback(() => {
    const ta = textareaRef.current;
    const layer = layerRef.current;
    if (ta === null || layer === null) return;
    const cs = getComputedStyle(ta);
    for (const p of copiedProperties) layer.style[p] = cs[p];
    layer.style.borderStyle = "solid";
    layer.style.borderColor = "transparent";
    layer.style.width = `${ta.offsetWidth}px`;
    layer.style.height = `${ta.offsetHeight}px`;
    layer.style.top = `${ta.offsetTop}px`;
    layer.style.left = `${ta.offsetLeft}px`;
    followScroll(ta, layer);
  }, [textareaRef]);

  useImperativeHandle(
    ref,
    () => ({
      sync,
      caretLineBottom: () => {
        const marker = markerRef.current;
        const layer = layerRef.current;
        const ta = textareaRef.current;
        if (marker === null || layer === null || ta === null) return null;
        const m = marker.getBoundingClientRect();
        const parent = (ta.offsetParent ?? document.body).getBoundingClientRect();
        const box = ta.getBoundingClientRect();
        // Keep the anchor inside the textarea's box when the caret line is scrolled out of view.
        return Math.min(Math.max(m.bottom, box.top), box.bottom) - parent.top;
      },
    }),
    [sync, textareaRef],
  );

  useLayoutEffect(sync);

  useEffect(() => {
    const ta = textareaRef.current;
    if (ta === null) return;
    const onScroll = () => {
      const layer = layerRef.current;
      if (layer === null) return;
      followScroll(ta, layer);
    };
    ta.addEventListener("scroll", onScroll, { passive: true });
    const observer = new ResizeObserver(sync);
    observer.observe(ta);
    // A web font (Noto Color Emoji with font-display: swap) changes the glyph widths after the first layout.
    document.fonts.addEventListener("loadingdone", sync);
    return () => {
      ta.removeEventListener("scroll", onScroll);
      observer.disconnect();
      document.fonts.removeEventListener("loadingdone", sync);
    };
  }, [sync, textareaRef]);

  const parts = segments(text, tokens, characterLimit);
  // Put the caret marker into the run that contains the caret.
  const rendered: React.ReactNode[] = [];
  let pos = 0;
  let placed = false;
  parts.forEach((s, i) => {
    const end = pos + s.text.length;
    const cls = s.kind === null ? undefined : markClass[s.kind];
    if (!placed && caret >= pos && caret <= end) {
      placed = true;
      const before = s.text.slice(0, caret - pos);
      const after = s.text.slice(caret - pos);
      rendered.push(
        <span key={i} className={cls} data-kind={s.kind ?? undefined}>
          {before}
          <span ref={markerRef} data-caret="" />
          {after}
        </span>,
      );
    } else {
      rendered.push(
        <span key={i} className={cls} data-kind={s.kind ?? undefined}>
          {s.text}
        </span>,
      );
    }
    pos = end;
  });
  if (!placed) rendered.push(<span key="caret" ref={markerRef} data-caret="" />);

  return (
    <div
      ref={layerRef}
      aria-hidden
      dir="auto"
      data-testid="highlight-layer"
      className={[
        "pointer-events-none absolute overflow-hidden [scrollbar-gutter:stable] whitespace-pre-wrap wrap-break-word select-none",
        showText ? "text-fg" : "text-transparent",
      ].join(" ")}
    >
      <div>
        {rendered}
        {/* A trailing newline needs a character after it to take up its line, as in the textarea. */}
        {"​"}
      </div>
    </div>
  );
}
