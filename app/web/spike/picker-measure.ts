// SPDX-License-Identifier: AGPL-3.0-only
// Spike for #1150 (ADR-0022: this file stays on the unmerged branch spike/1150-picker-playwright-measure).
// Measures the emoji picker (ADR-0091) and the Noto Color Emoji splits it loads (ADR-0095) with Playwright on the
// production build (app/web/dist, `scripts/task.sh web:dist`), served by `vite preview` as the E2E suite does.
// A small Node proxy in front of it rewrites the shell as e2e/fixtures.ts does (lang, apiOrigin, root-relative links)
// and answers the client API like e2e/emoji-picker.spec.ts / e2e/reactions.spec.ts, so that no page.route interception
// sits on the measured requests.
//
// Run from app/web:  node spike/picker-measure.ts [runs] [config-filter-regex]
// Browsers run one after another (never in parallel). Results: spike/out/results.jsonl and spike/out/summary.md.
import { spawn, type ChildProcess } from "node:child_process";
import { mkdirSync, appendFileSync, writeFileSync, readFileSync } from "node:fs";
import http from "node:http";
import net from "node:net";
import { chromium, firefox, webkit, type Browser, type BrowserContext, type BrowserType, type Page } from "@playwright/test";

const RUNS = Number(process.argv[2] ?? "7");
const FILTER = new RegExp(process.argv[3] ?? ".");
const OUT = new URL("./out/", import.meta.url);
mkdirSync(OUT, { recursive: true });

function freePort(): Promise<number> {
  return new Promise((ok) => {
    const s = net.createServer();
    s.listen(0, "127.0.0.1", () => {
      const port = (s.address() as net.AddressInfo).port;
      s.close(() => ok(port));
    });
  });
}

// ---------- servers ----------

const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAGUlEQVR4nGN4ViH3nxLMMGrAqAGjBgwXAwAvJnsfj06SzAAAAABJRU5ErkJggg==", "base64");

function mockApi(origin: string, path: string, method: string): { status: number; body: unknown } {
  const alice = {
    id: "0192f0c4-7a8e-7000-8000-000000000001", username: "alice", acct: "alice", domain: "my.example",
    uri: "https://my.example/ap/actors/0192f0c4-7a8e-7000-8000-000000000001", display_name: "Alice", note: "", fields: [], emojis: [], badges: [],
    url: "https://my.example/@alice", locked: false, bot: false, created_at: "2026-10-01T00:00:00Z",
  };
  const bob = { ...alice, id: "0192f0c4-7a8e-7000-8000-000000000002", username: "bob", acct: "bob", display_name: "Bob", url: "https://my.example/@bob" };
  const img = (n: string) => `${origin}/media/${n}.png`;
  const emojis = [
    { shortcode: "wave", url: img("wave"), static_url: img("wave"), aliases: ["hello"] },
    { shortcode: "thanks", url: img("thanks"), static_url: img("thanks"), aliases: [] },
    { shortcode: "blobcat", url: img("blobcat"), static_url: img("blobcat"), category: "Blobs", aliases: [] },
    { shortcode: "blobfox", url: img("blobfox"), static_url: img("blobfox"), category: "Blobs", aliases: ["fox"] },
  ];
  const postId = "0192f0c4-7a8e-7000-8000-0000000000f1";
  const post = {
    id: postId, uri: `https://my.example/ap/statuses/${postId}`, url: `https://my.example/@bob/status/${postId}`, account: bob,
    content: "<p>The trail along the river is open again.</p>", spoiler_text: "", sensitive: false, visibility: "public", language: "en",
    created_at: new Date(Date.now() - 3600_000).toISOString(), media_attachments: [], mentions: [], tags: [], emojis: [],
    replies_count: 0, boosts_count: 0, favourites_count: 1,
    reactions: [{ name: "👍", count: 12, me: false }, { name: "blobcat", shortcode: "blobcat", emoji: emojis[2], count: 3, me: false }],
    favourited: false, boosted: false, bookmarked: false,
  };
  const p = path.replace(/^\/api\/v1/, "");
  if (path === "/oauth/token/refresh") return { status: 200, body: { access_token: "e2e-token", token_type: "Bearer", expires_in: 3600, scope: "read write follow" } };
  if (p === "/accounts/verify_credentials") {
    return { status: 200, body: { ...alice, discoverable: true, indexable: true, ai_opt_out: false, source: { note: "", fields: [] },
      posting: { visibility_ceiling: "public", visibilities: ["public", "local", "unlisted", "private", "direct"] }, permissions: [] } };
  }
  if (p === "/instance") {
    return { status: 200, body: { domain: "my.example", name: "my example", version: "0.0.0-e2e", source_url: "https://github.com/mycroblog/my",
      registrations: { mode: "open" }, federation: { mode: "both" }, limits: { max_post_characters: 500, require_media_description: false },
      timelines: { local: "everyone", federated: "everyone" }, rules: [] } };
  }
  if (p === "/preferences") {
    return { status: 200, body: { locale: "en", time_zone: "UTC", default_visibility: "public", default_post_language: "en", require_media_description: false, timeline_languages: [] } };
  }
  if (p === "/notifications/marker") return { status: 200, body: { last_read_id: "0", unread_count: 0 } };
  if (p === "/timelines/home") return { status: 200, body: { items: [] } };
  if (p === "/custom_emojis") return { status: 200, body: { items: emojis } };
  if (p === `/statuses/${postId}/context`) return { status: 200, body: { ancestors: [], descendants: [] } };
  if (p === `/statuses/${postId}`) return { status: 200, body: post };
  void method;
  return { status: 404, body: { type: "about:blank", title: "Not Found", status: 404 } };
}

async function startServers(): Promise<{ origin: string; stop: () => void }> {
  const previewPort = await freePort();
  const proxyPort = await freePort();
  const origin = `http://127.0.0.1:${String(proxyPort)}`;
  const preview: ChildProcess = spawn("bun", ["run", "preview", "--host", "127.0.0.1", "--port", String(previewPort), "--strictPort"], {
    cwd: new URL("..", import.meta.url), stdio: "ignore",
  });
  for (let i = 0; ; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${String(previewPort)}/`);
      if (r.ok) break;
    } catch { /* not up yet */ }
    if (i > 100) throw new Error("vite preview did not start");
    await new Promise((r) => setTimeout(r, 200));
  }
  const server = http.createServer((req, res) => {
    const url = new URL(req.url ?? "/", origin);
    if (url.pathname.startsWith("/api/") || url.pathname.startsWith("/oauth/")) {
      const { status, body } = mockApi(origin, url.pathname, req.method ?? "GET");
      res.writeHead(status, { "content-type": status < 400 ? "application/json" : "application/problem+json" });
      res.end(JSON.stringify(body));
      return;
    }
    if (url.pathname.startsWith("/media/")) {
      res.writeHead(200, { "content-type": "image/png" });
      res.end(png);
      return;
    }
    const document = !/\.[a-z0-9]+$/i.test(url.pathname) || url.pathname.endsWith(".html");
    const headers = { ...req.headers, host: `127.0.0.1:${String(previewPort)}` };
    if (document) headers["accept-encoding"] = "identity";
    const upstream = http.request({ host: "127.0.0.1", port: previewPort, path: req.url, method: req.method, headers }, (up) => {
      if (!document || !(up.headers["content-type"] ?? "").includes("text/html")) {
        res.writeHead(up.statusCode ?? 502, up.headers);
        up.pipe(res);
        return;
      }
      const chunks: Buffer[] = [];
      up.on("data", (c: Buffer) => chunks.push(c));
      up.on("end", () => {
        const body = Buffer.concat(chunks).toString("utf8")
          .replace(/<html [^>]*>/, `<html lang="en" dir="ltr">`)
          .replace(/"apiOrigin":"[^"]*"/, `"apiOrigin":"${origin}"`)
          .replaceAll(/(src|href)="\.\//g, '$1="/');
        const h = { ...up.headers };
        delete h["content-length"];
        delete h["etag"];
        res.writeHead(up.statusCode ?? 200, h);
        res.end(body);
      });
    });
    upstream.on("error", () => { res.writeHead(502); res.end(); });
    req.pipe(upstream);
  });
  await new Promise<void>((ok) => server.listen(proxyPort, "127.0.0.1", ok));
  return { origin, stop: () => { server.close(); preview.kill("SIGTERM"); } };
}

// ---------- in-page instrumentation ----------

const instrument = `(() => {
  const m = window.__m = { lt: [], fd: [] };
  try {
    new PerformanceObserver((l) => { for (const e of l.getEntries()) m.lt.push([e.startTime, e.duration]); }).observe({ type: "longtask", buffered: true });
    m.ltSupported = PerformanceObserver.supportedEntryTypes.includes("longtask");
  } catch { m.ltSupported = false; }
  document.fonts.addEventListener("loadingdone", () => m.fd.push(performance.now()));
  // A time just after the next frame is painted: rAF runs before the paint, a posted message after it.
  const afterPaint = () => new Promise((r) => requestAnimationFrame(() => { const c = new MessageChannel(); c.port1.onmessage = () => r(performance.now()); c.port2.postMessage(0); }));
  m.armOpen = (threshold) => {
    const base = document.querySelectorAll('[role="dialog"]').length;
    m.baseDialogs = base;
    const s = m.open = { armed: performance.now(), t0: null, tDialog: null, tGrid: null, tPaint: null, tFonts: null, done: false };
    const onDown = (e) => { if (s.t0 === null) s.t0 = e.timeStamp; };
    for (const t of ["pointerdown", "touchstart", "mousedown", "click"]) document.addEventListener(t, onDown, { capture: true, once: true });
    const check = () => {
      if (document.querySelectorAll('[role="dialog"]').length > base && s.tDialog === null) s.tDialog = performance.now();
      if (s.tGrid !== null) return;
      const n = document.querySelectorAll('[role="dialog"] [role="grid"] [role="row"]').length;
      if (n >= threshold) {
        s.tGrid = performance.now(); s.rows = n; mo.disconnect();
        afterPaint().then(async (t) => { s.tPaint = t; await document.fonts.ready; s.tFonts = performance.now(); s.done = true; });
      }
    };
    const mo = new MutationObserver(check);
    mo.observe(document.body, { childList: true, subtree: true });
  };
  m.armKey = () => {
    const s = m.key = { t0: null, t1: null };
    document.addEventListener("keydown", (e) => { if (s.t0 === null) s.t0 = e.timeStamp; }, { capture: true, once: true });
    const grid = document.querySelector('[role="dialog"] [role="grid"]');
    const target = grid?.parentElement ?? document.body;
    const mo = new MutationObserver(() => { mo.disconnect(); afterPaint().then((t) => { s.t1 = t; }); });
    mo.observe(target, { childList: true, subtree: true, characterData: true });
  };
})();`;

// ---------- configs ----------

type Config = {
  name: string;
  browser: BrowserType;
  mobile: boolean;
  cpu?: number;
  net?: { latency: number; downloadThroughput: number; uploadThroughput: number };
};
// Chrome DevTools' "Fast 4G" preset: 9 Mbps down / 1.5 Mbps up × 0.9, 60 ms RTT × 2.75.
const fast4g = { latency: 165, downloadThroughput: (9 * 1024 * 1024 * 0.9) / 8, uploadThroughput: (1.5 * 1024 * 1024 * 0.9) / 8 };
const configs: Config[] = [
  { name: "chromium-desktop", browser: chromium, mobile: false },
  { name: "chromium-360", browser: chromium, mobile: true },
  { name: "chromium-360-cpu4x", browser: chromium, mobile: true, cpu: 4 },
  { name: "chromium-360-cpu6x", browser: chromium, mobile: true, cpu: 6 },
  { name: "chromium-360-cpu4x-fast4g", browser: chromium, mobile: true, cpu: 4, net: fast4g },
  { name: "chromium-360-cpu6x-fast4g", browser: chromium, mobile: true, cpu: 6, net: fast4g },
  { name: "firefox-desktop", browser: firefox, mobile: false },
  { name: "firefox-360", browser: firefox, mobile: true },
  { name: "webkit-desktop", browser: webkit, mobile: false },
  { name: "webkit-360", browser: webkit, mobile: true },
];

type Scenario = "compose" | "reaction";
const scenarios = (process.env["SCENARIOS"] ?? "compose,reaction").split(",") as Scenario[];
const ROWS = 1500;

async function newContext(browser: Browser, c: Config): Promise<BrowserContext> {
  const firefoxMobile = c.browser === firefox;
  return browser.newContext({
    viewport: c.mobile ? { width: 360, height: 800 } : { width: 1440, height: 900 },
    ...(c.mobile ? { hasTouch: true, ...(firefoxMobile ? {} : { isMobile: true, deviceScaleFactor: 3 }) } : {}),
    locale: "en",
    serviceWorkers: "block",
  });
}

async function press(page: Page, c: Config, locator: ReturnType<Page["getByRole"]>) {
  if (c.mobile) await locator.tap();
  else await locator.click();
}

async function waitFor(page: Page, fn: string, timeout = 90_000) {
  await page.waitForFunction(fn, undefined, { timeout, polling: 50 });
}

async function emojiButton(page: Page, s: Scenario, c: Config) {
  if (s === "compose") {
    const form = page.getByRole("form", { name: "New post" });
    return form.getByRole("button", { name: "Add emoji" });
  }
  void c;
  return page.getByRole("article").filter({ hasText: "The trail along the river" }).getByRole("button", { name: "React", exact: true });
}

async function runOnce(browser: Browser, origin: string, c: Config, s: Scenario): Promise<Record<string, unknown>> {
  const context = await newContext(browser, c);
  await context.addInitScript(instrument);
  const page = await context.newPage();
  try {
    if (s === "compose") {
      await page.goto(`${origin}/`);
      await press(page, c, page.getByRole("button", { name: "Write a post" }).first());
    } else {
      await page.goto(`${origin}/@bob/status/0192f0c4-7a8e-7000-8000-0000000000f1`);
    }
    const button = await emojiButton(page, s, c);
    await button.waitFor({ state: "visible", timeout: 60_000 });
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(300);
    const cdp = c.browser === chromium ? await context.newCDPSession(page) : undefined;
    if (cdp && c.cpu) await cdp.send("Emulation.setCPUThrottlingRate", { rate: c.cpu });
    if (cdp && c.net) {
      await cdp.send("Network.enable");
      await cdp.send("Network.emulateNetworkConditions", { offline: false, ...c.net });
    }

    // First open, cold cache (a new context per run).
    await page.evaluate(`window.__m.armOpen(${String(ROWS)})`);
    await press(page, c, button);
    await waitFor(page, "window.__m.open.done === true");
    await page.waitForTimeout(800); // late font loads, long tasks after the fonts arrive
    const first = await page.evaluate(`(() => {
      const m = window.__m, s = m.open;
      const d = document.querySelector('[role="dialog"] [role="grid"]')?.closest('[role="dialog"]');
      const res = performance.getEntriesByType("resource").filter((e) => e.startTime >= s.armed - 1);
      const cat = (n) => /noto-emoji-/.test(n) ? "font" : /\\/(base|en|ja)\\.gen-/.test(n) ? "data" : /\\.js(\\?|$)/.test(n) ? "js" : "other";
      const bytes = {};
      for (const e of res) {
        const k = cat(e.name);
        const b = bytes[k] ??= { n: 0, transfer: 0, encoded: 0, decoded: 0, firstStart: Infinity, lastEnd: 0 };
        b.n++; b.transfer += e.transferSize; b.encoded += e.encodedBodySize; b.decoded += e.decodedBodySize;
        b.firstStart = Math.min(b.firstStart, e.startTime - s.t0); b.lastEnd = Math.max(b.lastEnd, e.responseEnd - s.t0);
      }
      const fontFaces = [...document.fonts].filter((f) => f.family.includes("Noto Color Emoji"));
      const lastFd = Math.max(0, ...m.fd.filter((t) => t >= s.t0));
      const fontsRendered = Math.max(s.tFonts, lastFd, bytes.font ? bytes.font.lastEnd + s.t0 : 0) - s.t0;
      const end = s.t0 + fontsRendered + 500;
      const lt = m.lt.filter(([st, du]) => st + du > s.t0 && st < end);
      const tbt = (until) => lt.filter(([st]) => st < until).reduce((a, [, du]) => a + Math.max(0, du - 50), 0);
      return {
        dialog: s.tDialog - s.t0, grid: s.tGrid - s.t0, visible: s.tPaint - s.t0, fontsReady: s.tFonts - s.t0, fontsRendered,
        rows: s.rows, domDialog: d ? d.querySelectorAll("*").length : 0, domDocument: document.getElementsByTagName("*").length,
        ltSupported: m.ltSupported,
        tbtToVisible: m.ltSupported ? tbt(s.tPaint) : null, tbtToFonts: m.ltSupported ? tbt(end) : null,
        longTasks: m.ltSupported ? lt.length : null, longestTask: m.ltSupported ? Math.max(0, ...lt.map(([, du]) => du)) : null,
        fontFormats: [...new Set(res.filter((e) => /noto-emoji-/.test(e.name)).map((e) => /noto-emoji-(colrv1|svg)-/.exec(e.name)?.[1]))],
        fontsLoaded: fontFaces.filter((f) => f.status === "loaded").length, fontFaces: fontFaces.length, bytes,
      };
    })()`);

    // Search: three keystrokes of "cat", then clearing back to every emoji.
    const search = page.getByRole("dialog").filter({ has: page.getByRole("grid") }).getByRole("searchbox");
    await search.focus();
    const keyLatency = async (key: string) => {
      await page.evaluate("window.__m.armKey()");
      await page.keyboard.press(key);
      await waitFor(page, "window.__m.key.t1 !== null", 30_000);
      const v = (await page.evaluate("window.__m.key.t1 - window.__m.key.t0")) as number;
      await page.waitForTimeout(150);
      return v;
    };
    const key1 = await keyLatency("c");
    const key2 = await keyLatency("a");
    const key3 = await keyLatency("t");
    await page.keyboard.press("ArrowLeft"); // keep the text; select all then delete
    await page.keyboard.press(process.platform === "darwin" ? "Meta+A" : "Control+A");
    const clear = await keyLatency("Backspace");
    await waitFor(page, `document.querySelectorAll('[role="dialog"] [role="grid"] [role="row"]').length >= ${String(ROWS)}`);

    // Second open in the same page.
    await page.keyboard.press("Escape");
    await waitFor(page, `document.querySelectorAll('[role="dialog"]').length === window.__m.baseDialogs`, 10_000).catch(async () => {
      await page.keyboard.press("Escape");
      await waitFor(page, `document.querySelectorAll('[role="dialog"]').length === window.__m.baseDialogs`, 10_000);
    });
    await page.waitForTimeout(500);
    await page.evaluate(`window.__m.armOpen(${String(ROWS)})`);
    await press(page, c, button);
    await waitFor(page, "window.__m.open.done === true");
    await page.waitForTimeout(500);
    const second = await page.evaluate(`(() => {
      const m = window.__m, s = m.open;
      const lt = m.lt.filter(([st, du]) => st + du > s.t0 && st < s.tPaint);
      return { visible: s.tPaint - s.t0, grid: s.tGrid - s.t0, tbtToVisible: !m.ltSupported ? null : lt.reduce((a, [, du]) => a + Math.max(0, du - 50), 0) };
    })()`);
    return { first, search: { key1, key2, key3, clear }, second };
  } finally {
    await context.close();
  }
}

// ---------- main ----------

const { origin, stop } = await startServers();
const results = new URL("results.jsonl", OUT);
try {
  for (const c of configs.filter((x) => FILTER.test(x.name))) {
    const browser = await c.browser.launch();
    try {
      for (const s of scenarios) {
        // One warm-up run (not recorded): the JIT of the browser and vite preview's first reads.
        try { await runOnce(browser, origin, c, s); } catch (e) { console.error(`warm-up ${c.name} ${s}:`, (e as Error).message); }
        for (let i = 0; i < RUNS; i++) {
          try {
            const r = await runOnce(browser, origin, c, s);
            appendFileSync(results, JSON.stringify({ config: c.name, scenario: s, run: i, ...r }) + "\n");
            const f = r["first"] as Record<string, number>;
            console.log(`${c.name} ${s} #${String(i)} visible=${f["visible"]?.toFixed(0)} fonts=${f["fontsRendered"]?.toFixed(0)} tbt=${f["tbtToVisible"]?.toFixed(0)}`);
          } catch (e) {
            console.error(`${c.name} ${s} #${String(i)} failed:`, (e as Error).message);
          }
        }
      }
    } finally {
      await browser.close();
    }
  }
} finally {
  stop();
}

// ---------- summary ----------

type Row = Record<string, unknown> & { config: string; scenario: string };
const rows: Row[] = readFileSync(results, "utf8").trim().split("\n").map((l) => JSON.parse(l) as Row);
const get = (o: unknown, path: string): number | undefined => {
  let v: unknown = o;
  for (const k of path.split(".")) v = (v as Record<string, unknown> | undefined)?.[k];
  return typeof v === "number" ? v : undefined;
};
const q = (xs: number[], p: number) => {
  const s = [...xs].sort((a, b) => a - b);
  return s[Math.min(s.length - 1, Math.ceil(p * s.length) - 1)] ?? NaN;
};
const metrics = [
  "first.visible", "first.fontsRendered", "first.tbtToVisible", "first.tbtToFonts", "first.longestTask", "second.visible", "second.tbtToVisible",
  "search.key1", "search.key2", "search.key3", "search.clear", "first.domDialog", "first.domDocument", "first.fontsLoaded",
  "first.bytes.font.transfer", "first.bytes.font.encoded", "first.bytes.data.transfer", "first.bytes.data.encoded", "first.bytes.font.lastEnd", "first.bytes.data.lastEnd", "first.grid",
];
let md = `| config | scenario | n | ${metrics.join(" | ")} |\n|${" --- |".repeat(metrics.length + 3)}\n`;
const keys = [...new Set(rows.map((r) => `${r.config}\t${r.scenario}`))];
for (const k of keys) {
  const [config, scenario] = k.split("\t");
  const rs = rows.filter((r) => r.config === config && r.scenario === scenario);
  const cells = metrics.map((m) => {
    const xs = rs.map((r) => get(r, m)).filter((x): x is number => x !== undefined);
    if (xs.length === 0) return "–";
    return `${q(xs, 0.5).toFixed(0)} / ${q(xs, 0.9).toFixed(0)}`;
  });
  md += `| ${config ?? ""} | ${scenario ?? ""} | ${String(rs.length)} | ${cells.join(" | ")} |\n`;
}
writeFileSync(new URL("summary.md", OUT), md);
console.log(md);
