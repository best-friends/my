// SPDX-License-Identifier: AGPL-3.0-only
// Spike #1152: the browser harness. Every check records what it saw into results/<engine>-dsf<scale>.json instead of
// failing, so one run per engine gives the whole table (criterion × engine × variant).
import type {} from "@vitest/browser-playwright";
import { cleanup, render, screen } from "@testing-library/react";
import { afterAll, describe, test } from "vitest";
import { cdp, commands, page, server, userEvent } from "vitest/browser";
import { fixedCandidates } from "./candidates";
import { layerOptions } from "./HighlightLayer";
import { type TextAreaAutocompleteB1Props, TextAreaAutocompleteB1 } from "./TextAreaAutocompleteB1";

const engine = server.browser;
const dpr = window.devicePixelRatio;
const results: Record<string, unknown> = { engine, devicePixelRatio: dpr, userAgent: navigator.userAgent };
function record(key: string, value: unknown) {
  results[key] = value;
}

afterAll(async () => {
  await commands.writeFile(`src/spike1152/results/${engine}-dsf${dpr}.json`, JSON.stringify(results, null, 2));
});

const tick = (ms = 60) => new Promise((r) => setTimeout(r, ms));
const frame = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));

interface Events {
  accepts: string[];
  paths: string[];
  submits: number;
  escapesOutside: number;
}

function mount(props: Partial<TextAreaAutocompleteB1Props> = {}, width = "600px") {
  cleanup();
  const events: Events = { accepts: [], paths: [], submits: 0, escapesOutside: 0 };
  render(
    // The outer listener stands for the compose Sheet, which closes on an Escape that reaches it.
    // oxlint-disable-next-line jsx-a11y/no-static-element-interactions
    <div
      data-testid="outer"
      style={{ width, padding: 8 }}
      onKeyDown={(e) => {
        if (e.key === "Escape") events.escapesOutside++;
      }}
    >
      <TextAreaAutocompleteB1
        label="Post"
        candidates={fixedCandidates}
        onAccept={(c, p) => {
          events.accepts.push(c.value);
          events.paths.push(p);
        }}
        onSubmit={() => {
          events.submits++;
        }}
        {...props}
      />
    </div>,
  );
  const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
  return { ta, events };
}

function options() {
  return [...document.querySelectorAll<HTMLElement>('[role="option"]')].filter((o) => o.offsetParent !== null || o.getClientRects().length > 0);
}
function listVisible() {
  const lb = document.querySelector<HTMLElement>('[role="listbox"]');
  return lb !== null && lb.getClientRects().length > 0 && !lb.closest("[hidden]");
}
function activeOption(ta: HTMLTextAreaElement) {
  const id = ta.getAttribute("aria-activedescendant");
  return id === null ? null : (document.getElementById(id)?.textContent ?? `missing:${id}`);
}
async function focusEnd(ta: HTMLTextAreaElement) {
  await userEvent.click(ta);
  ta.setSelectionRange(ta.value.length, ta.value.length);
  await tick();
}

const variants = ["raw", "b1"] as const;

describe("triggers and ARIA", () => {
  for (const variant of variants) {
    test(`triggers ${variant}`, async () => {
      const { ta } = mount({ variant });
      await focusEnd(ta);
      const out: Record<string, unknown> = {};
      for (const [typed, name] of [
        ["@al", "@"],
        [" #go", "#"],
        [" :bl", ":"],
        [" ＠al", "fullwidth＠"],
        [" mail@al", "email-like"],
      ] as const) {
        await userEvent.keyboard(typed);
        await tick(600);
        out[name] = { open: listVisible(), options: options().map((o) => o.textContent), active: activeOption(ta) };
        await userEvent.keyboard("{Escape}");
        await tick();
      }
      // ARIA of the open state.
      await userEvent.keyboard(" @a");
      await tick(600);
      const lb = document.querySelector('[role="listbox"]');
      out["aria"] = {
        textareaRole: ta.getAttribute("role"),
        ariaAutocomplete: ta.getAttribute("aria-autocomplete"),
        ariaControlsMatchesListbox: lb !== null && ta.getAttribute("aria-controls") === lb.id,
        ariaActivedescendant: activeOption(ta),
        ariaExpanded: ta.getAttribute("aria-expanded"),
        optionSelected: options().map((o) => `${o.textContent}:${o.getAttribute("aria-selected")}`),
        listboxLabel: lb?.getAttribute("aria-label") ?? lb?.getAttribute("aria-labelledby"),
        status: document.querySelector('[role="status"]')?.textContent,
        spellcheck: ta.getAttribute("spellcheck"),
        autocorrect: ta.getAttribute("autocorrect"),
        enterkeyhint: ta.getAttribute("enterkeyhint"),
        autocomplete: ta.getAttribute("autocomplete"),
        popoverRole: document.querySelector('[data-testid="popover"]')?.getAttribute("role") ?? null,
        focusStaysInTextarea: document.activeElement === ta,
      };
      if (engine === "chromium") {
        // The engine's accessibility tree (Chromium only; Firefox and WebKit give Playwright no AX tree).
        interface FrameTree { frame: { id: string; url: string }; childFrames?: FrameTree[] }
        const ft = (await cdp().send("Page.getFrameTree", {})) as { frameTree: FrameTree };
        const all: FrameTree[] = [];
        const walk = (t: FrameTree) => {
          all.push(t);
          for (const c of t.childFrames ?? []) walk(c);
        };
        walk(ft.frameTree);
        const frameId = all.find((t) => t !== ft.frameTree)?.frame.id;
        const tree = (await cdp().send("Accessibility.getFullAXTree", frameId === undefined ? {} : { frameId })) as {
          nodes: { role?: { value: string }; name?: { value: string }; properties?: { name: string; value: { value?: unknown; relatedNodes?: unknown[] } }[] }[];
        };
        const pick = (role: string) =>
          tree.nodes
            .filter((n) => n.role?.value === role)
            .map((n) => ({ name: n.name?.value, props: Object.fromEntries((n.properties ?? []).map((p) => [p.name, p.value.value ?? p.value.relatedNodes?.length])) }));
        out["axTree"] = { textbox: pick("textbox"), combobox: pick("combobox"), listbox: pick("listbox"), option: pick("option"), dialog: pick("dialog") };
      }
      record(`triggers.${variant}`, out);
    });
  }
});

describe("keyboard", () => {
  for (const variant of variants) {
    test(`keyboard ${variant}`, async () => {
      const out: Record<string, unknown> = {};
      {
        const { ta, events } = mount({ variant });
        await focusEnd(ta);
        await userEvent.keyboard("@a");
        await tick(600);
        out["openFirstActive"] = activeOption(ta);
        {
          // How long aria-activedescendant takes to follow the first ArrowDown after typing (React Aria delays it).
          const t0 = performance.now();
          let lag = -1;
          const mo = new MutationObserver(() => {
            if (lag < 0) lag = Math.round(performance.now() - t0);
          });
          mo.observe(ta, { attributes: true, attributeFilter: ["aria-activedescendant"] });
          await userEvent.keyboard("{ArrowDown}");
          await tick(700);
          mo.disconnect();
          out["firstArrowDownLagMs"] = lag;
          await userEvent.keyboard("{ArrowUp}");
        }
        await userEvent.keyboard("{ArrowDown}");
        await tick(600);
        out["afterDown"] = activeOption(ta);
        await userEvent.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}");
        await tick(600);
        out["afterManyDownStopsAtEnd"] = activeOption(ta);
        await userEvent.keyboard("{ArrowUp}");
        await tick(600);
        out["afterUp"] = activeOption(ta);
        await userEvent.keyboard("{Enter}");
        await tick();
        out["enter"] = { value: ta.value, accepts: [...events.accepts], caret: ta.selectionStart, open: listVisible() };
      }
      {
        const { ta, events } = mount({ variant });
        await focusEnd(ta);
        await userEvent.keyboard("@al");
        await tick(600);
        await userEvent.keyboard("{Tab}");
        await tick();
        out["tab"] = { value: ta.value, accepts: [...events.accepts], focusInTextarea: document.activeElement === ta };
      }
      {
        const { ta, events } = mount({ variant });
        await focusEnd(ta);
        await userEvent.keyboard("@al");
        await tick(600);
        await userEvent.keyboard("{Escape}");
        await tick();
        out["escape"] = { open: listVisible(), escapesReachedSheet: events.escapesOutside, value: ta.value };
      }
      {
        const { ta } = mount({ variant, defaultValue: "first line\nsecond " });
        await focusEnd(ta);
        await userEvent.keyboard("@al");
        await tick(600);
        await userEvent.keyboard("{Home}");
        await tick();
        out["homeWhileOpen"] = { caret: ta.selectionStart, expectedLineStart: "first line\n".length, active: activeOption(ta) };
        ta.setSelectionRange(ta.value.length, ta.value.length);
        await userEvent.keyboard("x{Backspace}");
        await tick(600);
        ta.setSelectionRange("first line\n".length, "first line\n".length);
        await tick();
        await userEvent.keyboard("{End}");
        await tick();
        out["endWhileOpen"] = { caret: ta.selectionStart, expected: ta.value.length };
      }
      {
        // After closing the list with Escape, the arrows move between lines again.
        const { ta } = mount({ variant, defaultValue: "first line\nsecond " });
        await focusEnd(ta);
        await userEvent.keyboard("@al");
        await tick(600);
        await userEvent.keyboard("{Escape}");
        await tick();
        const before = ta.selectionStart;
        await userEvent.keyboard("{ArrowUp}");
        await tick();
        out["arrowUpAfterEscape"] = { before, after: ta.selectionStart, movedToFirstLine: ta.selectionStart <= "first line".length };
      }
      {
        // With no matching candidate (closed list), the arrows move between lines.
        const { ta } = mount({ variant, defaultValue: "first line\nsecond " });
        await focusEnd(ta);
        await userEvent.keyboard("@zz");
        await tick(600);
        await userEvent.keyboard("{ArrowUp}");
        await tick();
        out["arrowUpNoMatch"] = { after: ta.selectionStart, movedToFirstLine: ta.selectionStart <= "first line".length };
      }
      {
        const { ta, events } = mount({ variant });
        await focusEnd(ta);
        await userEvent.keyboard("@al");
        await tick(600);
        await userEvent.keyboard("{Control>}{Enter}{/Control}");
        await tick();
        out["ctrlEnter"] = { submits: events.submits, accepts: [...events.accepts], value: ta.value };
      }
      {
        // Backspace keeps the list open: is the first option still active?
        const { ta } = mount({ variant });
        await focusEnd(ta);
        await userEvent.keyboard("@ali");
        await tick(600);
        await userEvent.keyboard("{Backspace}");
        await tick(600);
        out["afterBackspace"] = { open: listVisible(), active: activeOption(ta) };
        await userEvent.keyboard("{Enter}");
        await tick();
        out["enterAfterBackspace"] = { value: ta.value };
      }
      {
        // ArrowLeft / Right move the caret and clear the active option (React Aria, for NVDA).
        const { ta } = mount({ variant, defaultValue: "x " });
        await focusEnd(ta);
        await userEvent.keyboard("@ali");
        await tick(600);
        await userEvent.keyboard("{ArrowLeft}");
        await tick(600);
        out["arrowLeft"] = { caret: ta.selectionStart, open: listVisible(), active: activeOption(ta) };
      }
      {
        const { ta, events } = mount({ variant });
        await focusEnd(ta);
        await userEvent.keyboard("@al");
        await tick(600);
        const second = options()[1];
        if (second !== undefined) await userEvent.click(second);
        await tick();
        out["click"] = { value: ta.value, accepts: [...events.accepts], focusInTextarea: document.activeElement === ta };
      }
      record(`keyboard.${variant}`, out);
    });
  }
});

describe("refocus after delete", () => {
  test("b1 with refocusAfterDelete", async () => {
    const { ta } = mount({ variant: "b1", refocusAfterDelete: true });
    await focusEnd(ta);
    await userEvent.keyboard("@ali");
    await tick(600);
    await userEvent.keyboard("{Backspace}");
    await tick(600);
    const afterBackspace = { open: listVisible(), active: activeOption(ta) };
    await userEvent.keyboard("{Enter}");
    await tick();
    record("keyboard.b1-refocusAfterDelete", { afterBackspace, enterAfterBackspace: { value: ta.value } });
  });
});

/** Sets the textarea's value the way an IME's composition update does, for engines without an IME API. */
function setComposingValue(ta: HTMLTextAreaElement, value: string, inputType: string, data: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")?.set;
  ta.dispatchEvent(new InputEvent("beforeinput", { bubbles: true, cancelable: false, inputType, data, isComposing: true }));
  setter?.call(ta, value);
  ta.dispatchEvent(new InputEvent("input", { bubbles: true, inputType, data, isComposing: true }));
}
function syntheticKey(ta: HTMLTextAreaElement, key: string, isComposing: boolean, keyCode: number) {
  const e = new KeyboardEvent("keydown", { key, code: key, bubbles: true, cancelable: true, isComposing });
  Object.defineProperty(e, "keyCode", { get: () => keyCode });
  Object.defineProperty(e, "which", { get: () => keyCode });
  ta.dispatchEvent(e);
  ta.dispatchEvent(new KeyboardEvent("keyup", { key, code: key, bubbles: true, isComposing }));
  return e.defaultPrevented;
}

describe("IME", () => {
  for (const variant of variants) {
    test(`synthetic composition ${variant}`, async () => {
      const out: Record<string, unknown> = {};
      const { ta, events } = mount({ variant });
      await focusEnd(ta);
      await userEvent.keyboard("#");
      await tick();
      ta.dispatchEvent(new CompositionEvent("compositionstart", { bubbles: true, data: "" }));
      setComposingValue(ta, "#にほん", "insertCompositionText", "にほん");
      ta.dispatchEvent(new CompositionEvent("compositionupdate", { bubbles: true, data: "にほん" }));
      await tick(600);
      out["openDuringComposition"] = listVisible();
      // Chrome / Firefox: the committing Enter arrives during the composition with isComposing.
      const prevented = syntheticKey(ta, "Enter", true, 229);
      await tick();
      out["enterIsComposing"] = { accepts: [...events.accepts], defaultPrevented: prevented };
      setComposingValue(ta, "#日本", "insertCompositionText", "日本");
      ta.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true, data: "日本" }));
      await tick(600);
      out["openAfterCompositionEnd"] = { open: listVisible(), options: options().map((o) => o.textContent), active: activeOption(ta) };
      // Safari: the committing Enter arrives after compositionend with isComposing false and keyCode 229.
      const prevented229 = syntheticKey(ta, "Enter", false, 229);
      await tick();
      out["safariEnter229"] = { accepts: [...events.accepts], defaultPrevented: prevented229, value: ta.value };
      // A real Enter then accepts.
      const preventedEnter = syntheticKey(ta, "Enter", false, 13);
      await tick();
      out["plainEnterAfter"] = { accepts: [...events.accepts], defaultPrevented: preventedEnter, value: ta.value };
      record(`ime.synthetic.${variant}`, out);
    });

    test(`synthetic composition while open ${variant}`, async () => {
      const { ta, events } = mount({ variant });
      await focusEnd(ta);
      await userEvent.keyboard("#go");
      await tick(600);
      const openBefore = listVisible();
      ta.dispatchEvent(new CompositionEvent("compositionstart", { bubbles: true, data: "" }));
      setComposingValue(ta, "#goお", "insertCompositionText", "お");
      await tick(600);
      const openDuring = listVisible();
      const prevented = syntheticKey(ta, "Enter", true, 229);
      await tick();
      record(`ime.whileOpen.${variant}`, { openBefore, openDuring, accepts: [...events.accepts], defaultPrevented: prevented });
      ta.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true, data: "お" }));
    });

    if (engine === "chromium") {
      test(`CDP IME ${variant}`, async () => {
        const out: Record<string, unknown> = {};
        const log: string[] = [];
        const { ta, events } = mount({ variant });
        for (const type of ["compositionstart", "compositionupdate", "compositionend"]) {
          ta.addEventListener(type, (e) => log.push(`${type}:${(e as CompositionEvent).data}`));
        }
        ta.addEventListener("beforeinput", (e) => log.push(`beforeinput:${(e as InputEvent).inputType}:${(e as InputEvent).isComposing}`));
        ta.addEventListener("keydown", (e) => log.push(`keydown:${e.key}:composing=${e.isComposing}:keyCode=${e.keyCode}`));
        await focusEnd(ta);
        await userEvent.keyboard("#");
        await tick();
        await cdp().send("Input.imeSetComposition", { text: "にほん", selectionStart: 3, selectionEnd: 3 });
        await tick(150);
        out["openDuringComposition"] = listVisible();
        await cdp().send("Input.dispatchKeyEvent", { type: "rawKeyDown", key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 });
        await cdp().send("Input.dispatchKeyEvent", { type: "keyUp", key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 });
        await tick();
        out["enterDuringComposition"] = { accepts: [...events.accepts], value: ta.value };
        await cdp().send("Input.imeSetComposition", { text: "日本", selectionStart: 2, selectionEnd: 2 });
        await cdp().send("Input.insertText", { text: "日本" });
        await tick(150);
        out["afterCommit"] = { value: ta.value, open: listVisible(), options: options().map((o) => o.textContent), active: activeOption(ta) };
        // A trusted keydown with keyCode 229 (what shipped Safari sends for the committing Enter).
        await cdp().send("Input.dispatchKeyEvent", { type: "rawKeyDown", key: "Enter", code: "Enter", windowsVirtualKeyCode: 229 });
        await cdp().send("Input.dispatchKeyEvent", { type: "keyUp", key: "Enter", code: "Enter", windowsVirtualKeyCode: 229 });
        await tick();
        out["trusted229"] = { accepts: [...events.accepts], value: ta.value };
        await userEvent.keyboard("{Enter}");
        await tick();
        out["enterAfter"] = { accepts: [...events.accepts], value: ta.value };
        out["log"] = log;
        record(`ime.cdp.${variant}`, out);
      });
    }
  }
});

describe("undo", () => {
  for (const variant of variants) {
    test(`undo ${variant}`, async () => {
      const out: Record<string, unknown> = {};
      const mod = engine === "webkit" && navigator.platform.startsWith("Mac") ? "Meta" : "Control";
      {
        const { ta, events } = mount({ variant, defaultValue: "hi " });
        await focusEnd(ta);
        await userEvent.keyboard("@al");
        await tick(600);
        await userEvent.keyboard("{Enter}");
        await tick();
        out["accepted"] = { value: ta.value, caret: ta.selectionStart, path: events.paths[0] };
        await userEvent.keyboard(`{${mod}>}z{/${mod}}`);
        await tick();
        out["afterOneUndoKey"] = { value: ta.value, caret: ta.selectionStart, selectionEnd: ta.selectionEnd };
        await userEvent.keyboard(`{${mod}>}{Shift>}z{/Shift}{/${mod}}`);
        await tick();
        out["afterRedoKey"] = { value: ta.value };
        // oxlint-disable-next-line typescript/no-deprecated -- the history command as the browser runs it.
        document.execCommand("undo");
        await tick();
        out["afterExecUndo"] = { value: ta.value };
        // oxlint-disable-next-line typescript/no-deprecated
        document.execCommand("redo");
        await tick();
        out["afterExecRedo"] = { value: ta.value };
        await userEvent.keyboard("ok");
        await tick();
        out["typingAfterRedo"] = { value: ta.value };
      }
      {
        // The fallback (setRangeText) for comparison.
        const { ta } = mount({ variant, defaultValue: "hi " });
        await focusEnd(ta);
        await userEvent.keyboard("@al");
        await tick(600);
        const before = ta.value;
        ta.setRangeText("@alice ", 3, 6, "end");
        ta.dispatchEvent(new Event("input", { bubbles: true }));
        await tick();
        await userEvent.keyboard(`{${mod}>}z{/${mod}}`);
        await tick();
        out["setRangeTextUndo"] = { before, after: ta.value };
      }
      record(`undo.${variant}`, out);
    });
  }
});

describe("popup position", () => {
  for (const popover of [true, false]) {
    test(`position popover=${popover}`, async () => {
      const out: Record<string, unknown> = {};
      const { ta } = mount({ variant: "b1", popover, defaultValue: "line 1\nline 2\nline 3 ", rows: 6 });
      await focusEnd(ta);
      await userEvent.keyboard("@a");
      await tick(200);
      const lb = document.querySelector<HTMLElement>('[role="listbox"]');
      const marker = document.querySelector<HTMLElement>("[data-caret]");
      const lineHeight = Number.parseFloat(getComputedStyle(ta).lineHeight);
      const m = marker?.getBoundingClientRect();
      const l = lb?.getBoundingClientRect();
      out["caretLine3"] = { listTop: l?.top, markerBottom: m?.bottom, delta: l && m ? l.top - m.bottom : null, lineHeight, listWidth: l?.width, textareaWidth: ta.getBoundingClientRect().width };
      // Move to a new line and type again: does the list follow?
      await userEvent.keyboard("{Escape}{Enter}{Enter}@a");
      await tick(200);
      const m2 = document.querySelector<HTMLElement>("[data-caret]")?.getBoundingClientRect();
      const l2 = document.querySelector<HTMLElement>('[role="listbox"]')?.getBoundingClientRect();
      out["caretLine5"] = { listTop: l2?.top, markerBottom: m2?.bottom, delta: l2 && m2 ? l2.top - m2.bottom : null };
      record(`position.popover-${popover}`, out);
    });
  }

  test("position at 320px", async () => {
    await page.viewport(320, 700);
    const { ta } = mount({ variant: "b1", defaultValue: "a fairly long first line of text that wraps at this width " }, "calc(100vw - 32px)");
    await focusEnd(ta);
    await userEvent.keyboard("@a");
    await tick(200);
    const lb = document.querySelector<HTMLElement>('[role="listbox"]')?.getBoundingClientRect();
    const t = ta.getBoundingClientRect();
    record("position.320", {
      listLeft: lb?.left,
      listRight: lb?.right,
      textareaLeft: t.left,
      textareaRight: t.right,
      viewport: innerWidth,
      horizontalScroll: document.documentElement.scrollWidth > innerWidth,
    });
    await page.screenshot({ path: `./screenshots/${engine}-dsf${dpr}-popup-320.png` });
    await page.viewport(1024, 900);
  });
});

/** Decodes a PNG (base64) into pixels in the page. */
async function decode(b64: string) {
  const img = new Image();
  img.src = `data:image/png;base64,${b64}`;
  await img.decode();
  const c = document.createElement("canvas");
  c.width = img.width;
  c.height = img.height;
  const ctx = c.getContext("2d");
  if (ctx === null) throw new Error("no 2d context");
  ctx.drawImage(img, 0, 0);
  return ctx.getImageData(0, 0, c.width, c.height);
}
const luma = (d: Uint8ClampedArray, i: number) => 0.299 * (d[i] ?? 0) + 0.587 * (d[i + 1] ?? 0) + 0.114 * (d[i + 2] ?? 0);

/**
 * Draws the text once with the textarea only and once with the layer only, and counts the pixels where the two
 * differ, relative to the inked pixels. Aligned text differs only in anti-aliasing (a few %); a shift of one glyph
 * or a different line break differs in most of the ink.
 */
async function alignment(name: string, extra?: () => Promise<void> | void) {
  const ta = document.querySelector("textarea");
  const layer = document.querySelector<HTMLElement>('[data-testid="highlight-layer"]');
  const box = document.querySelector<HTMLElement>('[data-testid="autocomplete-root"]');
  if (ta === null || layer === null || box === null) throw new Error("not mounted");
  ta.blur();
  await extra?.();
  await frame();
  await tick(50);
  const marks = [...layer.querySelectorAll<HTMLElement>("[data-kind]")];
  for (const m of marks) m.style.background = "none";
  ta.style.color = "#000";
  ta.style.caretColor = "transparent";
  layer.style.color = "transparent";
  await frame();
  const a = await decode(await page.screenshot({ element: box, save: false }));
  ta.style.color = "transparent";
  layer.style.color = "#000";
  await frame();
  const b = await decode(await page.screenshot({ element: box, save: false }));
  let ink = 0;
  let diff = 0;
  for (let i = 0; i < a.data.length; i += 4) {
    const la = luma(a.data, i);
    const lb = luma(b.data, i);
    if (la < 140 || lb < 140) ink++;
    if (Math.abs(la - lb) > 96) diff++;
  }
  const ratio = ink === 0 ? 0 : diff / ink;
  // The vertical / horizontal shift (in device pixels) of the layer's text that best matches the textarea's.
  let best = { dx: 0, dy: 0, ratio };
  if (ratio > 0.02) {
    for (let dy = -4; dy <= 4; dy++) {
      for (let dx = -4; dx <= 4; dx++) {
        let d = 0;
        for (let y = 4; y < a.height - 4; y++) {
          for (let x = 4; x < a.width - 4; x++) {
            const i = (y * a.width + x) * 4;
            const j = ((y + dy) * a.width + (x + dx)) * 4;
            if (Math.abs(luma(a.data, i) - luma(b.data, j)) > 96) d++;
          }
        }
        const r = ink === 0 ? 0 : d / ink;
        if (r < best.ratio) best = { dx, dy, ratio: Math.round(r * 1000) / 1000 };
      }
    }
  }
  let shot: string | undefined;
  if (ratio > 0.08) {
    // Evidence: the textarea's text in red over the layer's text in blue.
    ta.style.color = "rgb(220,0,0)";
    layer.style.color = "rgba(0,0,255,0.6)";
    await frame();
    shot = await page.screenshot({ element: box, path: `./screenshots/${engine}-dsf${dpr}-${name}.png` });
  }
  ta.style.color = "";
  ta.style.caretColor = "";
  layer.style.color = "";
  for (const m of marks) m.style.background = "";
  const scroll = { taTop: ta.scrollTop, layerTop: layer.scrollTop, taHeight: ta.scrollHeight, layerHeight: layer.scrollHeight, taClient: ta.clientHeight, layerClient: layer.clientHeight, taClientWidth: ta.clientWidth, layerClientWidth: layer.clientWidth };
  const result = { scroll, best, ink, diff, ratio: Math.round(ratio * 1000) / 1000, sizeMismatch: a.width !== b.width || a.height !== b.height, screenshot: shot === undefined ? null : `${engine}-dsf${dpr}-${name}.png` };
  return result;
}

const texts = {
  latinWrap: `${"The quick brown fox jumps over the lazy dog. ".repeat(5)}https://example.com/${"a".repeat(90)} @alice@example.org #golang`,
  cjk: `${"日本語の文章は単語の間に空白がないので、折り返しの位置が英語と違います。".repeat(3)}「括弧」や、句読点。#日本語 @alice`,
  emoji: `絵文字 😀🎉👍🏽 👨‍👩‍👧‍👦 🇯🇵 ❤️ :blobcat: `.repeat(5),
  rtl: "مرحبا @alice كيف حالك اليوم؟ #日本語 هذا نص طويل باللغة العربية يلتف على عدة أسطر في مربع النص. ".repeat(3),
  bidi: "abc مرحبا 123 שלום def @bob #tag ".repeat(6),
  tabsAndSpaces: "a\tb\t\tc    d     e\n\n\ntrailing   \n",
  scroll: Array.from({ length: 40 }, (_, i) => `line ${i + 1} @user${i} #tag${i} some more words to wrap`).join("\n"),
};

describe("highlight layer alignment", () => {
  test("alignment cases", async () => {
    const out: Record<string, unknown> = {};
    for (const [name, text] of Object.entries(texts)) {
      const { ta } = mount({ variant: "b1", defaultValue: text, rows: 6 }, "420px");
      await document.fonts.ready;
      if (name === "scroll") {
        out[name] = await alignment(name, () => {
          ta.scrollTop = ta.scrollHeight / 2 + 7;
        });
        out[`${name}-bottom`] = await alignment(`${name}-bottom`, () => {
          ta.scrollTop = ta.scrollHeight;
        });
        // Mitigations: follow the scroll with a transform instead of scrollTop, and an integer line height.
        layerOptions.scrollMode = "transform";
        out[`${name}-transform`] = await alignment(`${name}-transform`, () => {
          ta.scrollTop = ta.scrollHeight / 2 + 7;
        });
        out[`${name}-bottom-transform`] = await alignment(`${name}-bottom-transform`, () => {
          ta.scrollTop = ta.scrollHeight;
        });
        layerOptions.scrollMode = "scrollTop";
        out[`${name}-integerLineHeight`] = await alignment(`${name}-integerLineHeight`, async () => {
          ta.style.lineHeight = "27px";
          ta.style.width = "401px";
          await tick(100);
          ta.scrollTop = ta.scrollHeight / 2 + 7;
        });
        out[`${name}-bottom-integerLineHeight`] = await alignment(`${name}-bottom-integerLineHeight`, () => {
          ta.scrollTop = ta.scrollHeight;
        });
        ta.style.lineHeight = "";
      } else {
        out[name] = await alignment(name);
      }
      // Resize the textarea (the user's resize handle, or the sheet's width): the ResizeObserver re-measures.
      if (name === "latinWrap") {
        // Control: a layer shifted by 0.3px of letter spacing must show up as misaligned.
        out["control-letterSpacing"] = await alignment("control-letterSpacing", () => {
          const layer = document.querySelector<HTMLElement>('[data-testid="highlight-layer"]');
          if (layer !== null) layer.style.letterSpacing = "0.3px";
        });
      }
      out[`${name}-resized`] = await alignment(`${name}-resized`, async () => {
        ta.style.width = "300px";
        await tick(100);
      });
      cleanup();
    }
    record("alignment.base", out);
  });

  test("alignment at 320px", async () => {
    await page.viewport(320, 800);
    const out: Record<string, unknown> = {};
    for (const name of ["latinWrap", "cjk", "emoji", "rtl"] as const) {
      mount({ variant: "b1", defaultValue: texts[name], rows: 6 }, "calc(100vw - 32px)");
      await document.fonts.ready;
      out[name] = await alignment(`320-${name}`);
      cleanup();
    }
    record("alignment.320", { ...out, horizontalScroll: document.documentElement.scrollWidth > innerWidth });
    await page.viewport(1024, 900);
  });

  test("alignment with CSS zoom 200%", async () => {
    const out: Record<string, unknown> = {};
    for (const name of ["latinWrap", "cjk", "emoji", "rtl", "scroll"] as const) {
      const { ta } = mount({ variant: "b1", defaultValue: texts[name], rows: 6 }, "420px");
      const outer = document.querySelector<HTMLElement>('[data-testid="outer"]');
      if (outer !== null) outer.style.zoom = "2";
      await document.fonts.ready;
      out[name] = await alignment(`zoom-${name}`, async () => {
        await tick(100);
        if (name === "scroll") ta.scrollTop = 333;
      });
      cleanup();
    }
    record("alignment.cssZoom2", out);
  });

  test("emoji font swap", async () => {
    // Text with emoji from groups that no earlier test drew, so their font splits load now (font-display: swap).
    const text = "🦊🦉🐙🦑🦀🐢🦎🐍🦖🦕 food 🍣🍜🍙🍘🍥🥟🥠🥡 travel 🚀🛸🚁🛶⛵🚤 ".repeat(4);
    const { ta } = mount({ variant: "b1", defaultValue: text, rows: 6 }, "420px");
    const early = await alignment("fontswap-early");
    const loaded = await document.fonts.ready.then(async () => {
      await tick(300);
      return alignment("fontswap-loaded");
    });
    record("alignment.fontSwap", { early, loaded, fontsStatus: document.fonts.status, height: ta.scrollHeight });
  });
});
