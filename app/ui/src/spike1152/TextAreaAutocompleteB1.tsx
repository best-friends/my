// SPDX-License-Identifier: AGPL-3.0-only
// Spike #1152 (ADR-0092 §3, B1): React Aria Components' Autocomplete on a native TextArea with a ListBox, plus the
// §3 overrides, and the C1 highlight layer. `variant="raw"` is the plain composition without any override (to show
// what each override is for); `variant="b1"` applies them:
// - spellCheck / autoCorrect / enterKeyHint are given back on the TextField (its props win over the context's),
// - the input's onKeyDown from Autocomplete's FieldInputContext is wrapped: keys with isComposing or keyCode 229
//   never reach it, Home / End / PageUp / PageDown stay in the textarea, Ctrl / Cmd + Enter stays the submit,
//   Tab accepts the focused option, Escape closes the list and stops there (the sheet does not close),
// - the ListBox is unmounted while the list is closed, so the arrow keys move between lines again.
// The completion is inserted with execCommand("insertText") (one undo step), falling back to setRangeText.
import { type KeyboardEvent, type ReactNode, useContext, useEffect, useLayoutEffect, useRef, useState } from "react";
import {
  Autocomplete,
  AutocompleteStateContext,
  FieldInputContext,
  ListBox,
  ListBoxItem,
  Popover,
  TextArea,
  TextField,
  useSlottedContext,
} from "react-aria-components";
import { fieldControl } from "../Field";
import type { Candidate } from "./candidates";
import { HighlightLayer, type HighlightLayerHandle } from "./HighlightLayer";
import { type ActiveWord, activeWord, textTokens } from "./textTokens";

export type { Candidate };

export interface SpikeEvents {
  onAccept?: (c: Candidate, path: "execCommand" | "setRangeText") => void;
  onSubmit?: () => void;
}

export interface TextAreaAutocompleteB1Props extends SpikeEvents {
  variant?: "raw" | "b1";
  label: string;
  defaultValue?: string;
  characterLimit?: number;
  /** The candidates of the word being typed (the API part that features/compose passes in production). */
  candidates: (word: ActiveWord) => Candidate[];
  showLayerText?: boolean;
  /** Use React Aria's Popover (true) or an absolutely placed box under the caret line (false). */
  popover?: boolean;
  dir?: "auto";
  rows?: number;
  className?: string;
  /** b1 only: move virtual focus back to the first option after a deletion. */
  refocusAfterDelete?: boolean;
  /** b1 only: put aria-selected on the active option. */
  ariaSelected?: boolean;
}

/** Replace the active word with `value` and a space, keeping one undo step when the browser allows it. */
export function insertCompletion(ta: HTMLTextAreaElement, word: ActiveWord, value: string): "execCommand" | "setRangeText" {
  const after = ta.value.charAt(word.end);
  const insert = after === " " ? value : `${value} `;
  ta.focus();
  ta.setSelectionRange(word.start, word.end);
  let ok = false;
  try {
    // oxlint-disable-next-line typescript/no-deprecated -- ADR-0092 §3: the only way to keep the undo history.
    ok = document.execCommand("insertText", false, insert);
  } catch {
    ok = false;
  }
  if (!ok) {
    ta.setRangeText(insert, word.start, word.end, "end");
    ta.dispatchEvent(new Event("input", { bubbles: true }));
  }
  if (after === " ") {
    const at = word.start + insert.length + 1;
    ta.setSelectionRange(at, at);
  }
  return ok ? "execCommand" : "setRangeText";
}

interface KeyOverridesProps {
  open: boolean;
  onTab: () => boolean;
  onEscape: () => void;
  children: ReactNode;
}

/** Wraps the onKeyDown that Autocomplete gives the input (the §3 overrides). */
function KeyOverrides({ open, onTab, onEscape, children }: KeyOverridesProps) {
  const ctx = useSlottedContext(FieldInputContext);
  if (ctx === null || ctx === undefined) return children;
  const inner = ctx.onKeyDown;
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    // IME: Chrome / Firefox send isComposing; Safari sends the committing Enter after compositionend with keyCode 229.
    if (e.nativeEvent.isComposing || e.keyCode === 229) return;
    if (!open || inner === undefined) return;
    if (e.key === "Home" || e.key === "End" || e.key === "PageUp" || e.key === "PageDown") return;
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) return;
    if (e.key === "Tab" && !e.shiftKey) {
      if (onTab()) e.preventDefault();
      return;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      onEscape();
      return;
    }
    inner(e as unknown as Parameters<typeof inner>[0]);
  };
  return <FieldInputContext.Provider value={{ ...ctx, onKeyDown }}>{children}</FieldInputContext.Provider>;
}

/** The DOM id of the option that has virtual focus, from Autocomplete's state. */
function useFocusedOptionId(): string | null {
  return useContext(AutocompleteStateContext)?.focusedNodeId ?? null;
}

function FocusedOptionReader({ readRef, ariaSelected }: { readRef: { current: () => string | null }; ariaSelected: boolean }) {
  const id = useFocusedOptionId();
  readRef.current = () => id;
  // ADR-0092 §3 wants aria-selected="true" on the option aria-activedescendant points at (APG). React Aria's ListBox
  // without a selection mode renders no aria-selected, so the attribute is set on the DOM (React does not own it).
  useLayoutEffect(() => {
    if (!ariaSelected) return;
    for (const option of document.querySelectorAll<HTMLElement>('[data-spike-listbox] [role="option"]')) {
      option.setAttribute("aria-selected", String(option.id === id));
    }
  });
  return null;
}

/** React Aria's internal event that moves virtual focus into the collection (useAutocomplete's focusFirstItem). */
const focusEvent = "react-aria-focus";

export function TextAreaAutocompleteB1({
  variant = "b1",
  label,
  defaultValue = "",
  characterLimit,
  candidates,
  showLayerText = false,
  popover = true,
  rows = 4,
  className,
  onAccept,
  onSubmit,
  refocusAfterDelete = false,
  ariaSelected = true,
}: TextAreaAutocompleteB1Props) {
  const [text, setText] = useState(defaultValue);
  const [caret, setCaret] = useState(defaultValue.length);
  const [composing, setComposing] = useState(false);
  const [dismissedAt, setDismissedAt] = useState<number | null>(null);
  const [anchorTop, setAnchorTop] = useState(0);
  const taRef = useRef<HTMLTextAreaElement>(null);
  const layerRef = useRef<HighlightLayerHandle>(null);
  const anchorRef = useRef<HTMLDivElement>(null);
  const focusedId = useRef<() => string | null>(() => null);

  const word = activeWord(text, caret);
  const items = word === null ? [] : candidates(word);
  const open = !composing && word !== null && items.length > 0 && dismissedAt !== word.start;
  const overrides = variant === "b1";

  const readCaret = () => {
    const ta = taRef.current;
    if (ta !== null) setCaret(ta.selectionStart);
  };

  useEffect(() => {
    const onSelectionChange = () => {
      if (document.activeElement === taRef.current) readCaret();
    };
    document.addEventListener("selectionchange", onSelectionChange);
    return () => {
      document.removeEventListener("selectionchange", onSelectionChange);
    };
  });

  // React Aria clears the active option on a deletion (for screen readers); ADR-0092 §3 keeps the first one active while
  // the list is open, so a deletion moves virtual focus back to the first option.
  useEffect(() => {
    const ta = taRef.current;
    if (ta === null || !overrides || !refocusAfterDelete) return;
    const onInput = (e: Event) => {
      if (!(e instanceof InputEvent) || !e.inputType.startsWith("delete")) return;
      setTimeout(() => {
        const id = ta.getAttribute("aria-controls");
        const listbox = id === null ? null : document.getElementById(id);
        if (listbox === null) return;
        // The "first" strategy of the focus event only acts when the first key changes, which a deletion that keeps
        // the same candidates does not do; Home on the collection moves virtual focus to the first option.
        listbox.dispatchEvent(new CustomEvent(focusEvent, { bubbles: true, cancelable: true }));
        listbox.dispatchEvent(new KeyboardEvent("keydown", { key: "Home", bubbles: true, cancelable: true }));
        listbox.dispatchEvent(new KeyboardEvent("keyup", { key: "Home", bubbles: true, cancelable: true }));
      }, 0);
    };
    ta.addEventListener("input", onInput);
    return () => {
      ta.removeEventListener("input", onInput);
    };
  }, [overrides, refocusAfterDelete]);

  useLayoutEffect(() => {
    const top = layerRef.current?.caretLineBottom();
    if (top !== null && top !== undefined && top !== anchorTop) setAnchorTop(top);
  });

  const accept = (c: Candidate) => {
    const ta = taRef.current;
    if (ta === null) return;
    const w = activeWord(ta.value, ta.selectionStart);
    if (w === null) return;
    const path = insertCompletion(ta, w, c.value);
    readCaret();
    onAccept?.(c, path);
  };

  const acceptFocused = () => {
    const id = focusedId.current();
    if (id === null) return false;
    const key = document.getElementById(id)?.dataset["key"];
    const c = items.find((i) => i.id === key);
    if (c === undefined) return false;
    accept(c);
    return true;
  };

  const listBox = (
    <ListBox
      data-spike-listbox=""
      aria-label="Suggestions"
      items={items}
      onAction={(key) => {
        const c = items.find((i) => i.id === key);
        if (c !== undefined) accept(c);
      }}
      className="max-h-60 overflow-y-auto rounded-md border border-border bg-surface-raised p-1 text-fg shadow-lg outline-none"
    >
      {(c) => (
        <ListBoxItem id={c.id} textValue={c.label} className="cursor-default rounded-sm px-2 py-1 outline-none data-focused:bg-accent data-focused:text-accent-fg">
          {c.label}
        </ListBoxItem>
      )}
    </ListBox>
  );

  const field = (
    <TextField
      aria-label={label}
      value={text}
      onChange={(v) => {
        setText(v);
        setDismissedAt(null);
        // React Aria's Autocomplete swallows the input's keyup (a capture listener on document), so the caret is read
        // on input and on selectionchange, not on keyup.
        readCaret();
      }}
      {...(overrides ? { spellCheck: "true", autoCorrect: "on", enterKeyHint: "enter" as const } : {})}
      className="flex"
    >
      <TextArea
        ref={taRef}
        rows={rows}
        dir="auto"
        onSelect={readCaret}
        onKeyUp={readCaret}
        onClick={readCaret}
        onCompositionStart={() => {
          setComposing(true);
        }}
        onCompositionEnd={() => {
          setComposing(false);
          readCaret();
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
            e.preventDefault();
            onSubmit?.();
          }
        }}
        className={fieldControl({ className: "relative z-[1] min-h-24 resize-y bg-transparent [scrollbar-gutter:stable] whitespace-pre-wrap wrap-break-word" })}
      />
    </TextField>
  );

  // Raw keeps the ListBox mounted (hidden) while closed, the way an Autocomplete with an always-present ListBox works.
  const showList = overrides ? open : word !== null && !composing;

  return (
    <div className={["relative flex flex-col", className].join(" ")} data-testid="autocomplete-root">
      <Autocomplete inputValue={text} onInputChange={setText}>
        <FocusedOptionReader readRef={focusedId} ariaSelected={overrides && ariaSelected} />
        <div className="relative rounded-md bg-surface-sunken">
          <HighlightLayer
            ref={layerRef}
            textareaRef={taRef}
            text={text}
            tokens={textTokens(text)}
            {...(characterLimit === undefined ? {} : { characterLimit })}
            caret={caret}
            showText={showLayerText}
          />
          {overrides ? (
            <KeyOverrides open={open} onTab={acceptFocused} onEscape={() => word !== null && setDismissedAt(word.start)}>
              {field}
            </KeyOverrides>
          ) : (
            field
          )}
          <div ref={anchorRef} data-testid="caret-anchor" aria-hidden className="pointer-events-none absolute inset-x-0 h-0" style={{ top: anchorTop }} />
          {!popover && (showList || !overrides) && (
            <div className="absolute inset-x-0 z-10" style={{ top: anchorTop }} hidden={!showList}>
              {listBox}
            </div>
          )}
        </div>
        {popover && showList && (
          <Popover isOpen isNonModal triggerRef={anchorRef} placement="bottom start" offset={0} className="w-(--trigger-width)" data-testid="popover">
            {listBox}
          </Popover>
        )}
      </Autocomplete>
      <div role="status" className="sr-only">
        {open ? `${items.length} suggestions` : ""}
      </div>
    </div>
  );
}
