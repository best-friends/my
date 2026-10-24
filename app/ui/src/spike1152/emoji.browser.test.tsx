// SPDX-License-Identifier: AGPL-3.0-only
// Spike #1152: does the transparent layer still draw color emoji (glyph palettes ignore `color`)?
import { render } from "@testing-library/react";
import { test } from "vitest";
import { commands, page, server } from "vitest/browser";
import { fixedCandidates } from "./candidates";
import { TextAreaAutocompleteB1 } from "./TextAreaAutocompleteB1";

test("layer draws color emoji", async () => {
  render(
    <div style={{ width: 420 }}>
      <TextAreaAutocompleteB1 label="Post" candidates={fixedCandidates} defaultValue="😀🎉👍🏽 🦊🍣🚀 abc" />
    </div>,
  );
  await document.fonts.ready;
  const ta = document.querySelector("textarea");
  const layer = document.querySelector<HTMLElement>('[data-testid="highlight-layer"]');
  const box = document.querySelector<HTMLElement>('[data-testid="autocomplete-root"]');
  if (ta === null || layer === null || box === null) throw new Error("not mounted");
  ta.style.visibility = "hidden";
  await new Promise((r) => setTimeout(r, 200));
  const b64 = await page.screenshot({ element: box, save: false });
  const img = new Image();
  img.src = `data:image/png;base64,${b64}`;
  await img.decode();
  const c = document.createElement("canvas");
  c.width = img.width;
  c.height = img.height;
  const ctx = c.getContext("2d");
  if (ctx === null) throw new Error("no ctx");
  ctx.drawImage(img, 0, 0);
  const d = ctx.getImageData(0, 0, c.width, c.height).data;
  let colored = 0;
  for (let i = 0; i < d.length; i += 4) {
    const r = d[i] ?? 0;
    const g = d[i + 1] ?? 0;
    const b = d[i + 2] ?? 0;
    if (Math.max(r, g, b) - Math.min(r, g, b) > 80) colored++;
  }
  await page.screenshot({ element: box, path: `./screenshots/${server.browser}-layer-only-emoji.png` });
  await commands.writeFile(`src/spike1152/results/${server.browser}-layer-emoji.json`, JSON.stringify({ coloredPixelsInLayerOnly: colored }));
});
