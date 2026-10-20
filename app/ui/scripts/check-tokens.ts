// SPDX-License-Identifier: AGPL-3.0-only
// Checks the design tokens of @my/ui (web-client.md D6, D8, Y3). Run by `make web-check`.
//
//   bun run scripts/check-tokens.ts [<app entry stylesheet>...]
//
// 1. Contrast: resolves every semantic color in src/theme.css for the light and the dark theme and
//    fails when a pair in src/tokens.ts is below WCAG 2.2 AA, when a palette color is outside the
//    sRGB gamut, or when a semantic color is in no pair and not listed as decorative.
// 2. Usage: for each app entry stylesheet, scans the sources Tailwind scans for it and fails on
//    a color utility that names Tailwind's default palette (`bg-red-500`), an arbitrary color
//    (`text-[#333]`, `bg-(--palette-gray-50)`), a transition / animation outside `motion-safe:`, an
//    arbitrary radius, shadow, duration, or easing (`rounded-[3px]`), or a padding or gap step
//    that app/ui/README.md does not list.
// 3. Generated files: fails when the overridable token list generated from theme.css and tokens.ts
//    (scripts/theme-tokens.ts) differs from the committed one.
import { readFileSync } from "node:fs";
import { dirname, extname, relative, resolve } from "node:path";
import { Scanner } from "@tailwindcss/oxide";
import { __unstable__loadDesignSystem, compile } from "tailwindcss";
import defaultColors from "tailwindcss/colors";
import { colorPairs, decorativeColors, minimumContrast } from "../src/tokens";
import { contrast, inSrgbGamut, type LinearRgba, parseColor } from "../src/color";
import { colorResolver, readDeclarations, readTheme, type Scheme, schemes } from "./theme-source";
import { checkGenerated } from "./theme-tokens";

const problems: string[] = [];

// --- 1. Contrast ---------------------------------------------------------------------------------

const themeCss = readTheme();
const palette = readDeclarations(themeCss, "palette");
const semantic = readDeclarations(themeCss, "color");
const resolveColor = colorResolver(themeCss);

for (const [name, value] of palette) {
  try {
    if (!inSrgbGamut(parseColor(value))) problems.push(`theme.css: --palette-${name} (${value}) is outside the sRGB gamut`);
  } catch (error) {
    problems.push(`theme.css: --palette-${name}: ${(error as Error).message}`);
  }
}

const resolved = new Map<string, Record<Scheme, LinearRgba>>();
for (const [name, value] of semantic) {
  try {
    resolved.set(name, { light: resolveColor(value, "light"), dark: resolveColor(value, "dark") });
  } catch (error) {
    problems.push(`theme.css: --color-${name}: ${(error as Error).message}`);
  }
}

const covered = new Set<string>(decorativeColors);
const rows: string[] = [];
for (const pair of colorPairs) {
  covered.add(pair.fg);
  covered.add(pair.bg);
  const fg = resolved.get(pair.fg);
  const bg = resolved.get(pair.bg);
  if (fg === undefined || bg === undefined) {
    problems.push(`tokens.ts: pair ${pair.fg} on ${pair.bg} names a color theme.css does not define`);
    continue;
  }
  const minimum = minimumContrast[pair.use];
  const ratios = schemes.map((scheme) => {
    if (bg[scheme].alpha < 1) problems.push(`tokens.ts: background ${pair.bg} is translucent in the ${scheme} theme`);
    const ratio = contrast(fg[scheme], bg[scheme]);
    if (ratio < minimum) {
      problems.push(
        `contrast: ${pair.fg} on ${pair.bg} is ${ratio.toFixed(2)}:1 in the ${scheme} theme; ${pair.use} needs ${minimum}:1`,
      );
    }
    return ratio.toFixed(2).padStart(5);
  });
  rows.push(`  ${`${pair.fg} on ${pair.bg}`.padEnd(32)} ${pair.use.padEnd(4)} light ${ratios[0] ?? ""}  dark ${ratios[1] ?? ""}`);
}
for (const name of semantic.keys()) {
  if (!covered.has(name)) {
    problems.push(`tokens.ts: --color-${name} is in no contrast pair; add its pairs or list it as decorative`);
  }
}
for (const name of decorativeColors) {
  if (!semantic.has(name)) problems.push(`tokens.ts: decorative color ${name} is not defined in theme.css`);
}

console.log(`Contrast of ${colorPairs.length} token pairs (WCAG 2.2 AA: text 4.5:1, ui 3:1):`);
console.log(rows.join("\n"));

// --- 2. Usage ------------------------------------------------------------------------------------

/** Utilities whose value is a color (https://tailwindcss.com/docs/colors#using-color-utilities). */
const colorRoots = new Set([
  "bg",
  "text",
  "decoration",
  "border",
  "border-x",
  "border-y",
  "border-s",
  "border-e",
  "border-t",
  "border-r",
  "border-b",
  "border-l",
  "border-bs",
  "border-be",
  "divide",
  "outline",
  "ring",
  "ring-offset",
  "inset-ring",
  "shadow",
  "inset-shadow",
  "drop-shadow",
  "text-shadow",
  "accent",
  "caret",
  "fill",
  "stroke",
  "placeholder",
  "from",
  "via",
  "to",
]);
/**
 * Utilities whose value must be a token, so that a server theme's override of the token reaches them
 * (ADR-0059): radius, shadow, and the duration and easing of motion.
 */
const tokenOnlyRoots = /^(?:rounded(?:-[a-z]+)?|shadow|duration|ease)$/;
/**
 * Padding and gap utilities, which use the spacing steps of app/ui/README.md only. Margins and sizes also
 * scale with --spacing but hold geometry (aligning text with a control's label), not a spacing role.
 */
const spacingRoots = /^(?:p[xytrblse]?|p[bi][se]|gap(?:-[xy])?)$/;
const spacingSteps = new Set(["0", "0.5", "1", "1.5", "2", "3", "4", "6", "8", "10"]);
/** Utilities that animate; web-client.md D6 allows them only under motion-safe:. */
const motionRoots = new Set(["transition", "animate"]);
/**
 * Whether a utility value names a color of Tailwind's default palette (`red-500`, `white`), which
 * theme.css removes. Other unknown values are not reported here: source text also contains words
 * that merely look like classes (a token name such as "accent-hover" in a string), and unknown
 * classes in general are the lint's job (web-client.md D9).
 */
function isDefaultPaletteColor(value: string): boolean {
  const [hue = "", ...rest] = value.split("-");
  if (!Object.hasOwn(defaultColors, hue) || ["inherit", "current", "transparent"].includes(hue)) return false;
  return rest.length === 0 || (rest.length === 1 && /^(?:50|[1-9]00|950)$/.test(rest[0] ?? ""));
}
const colorLiteral =/#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix|light-dark)\(/i;
const colorProperties = /(?:^|-)(?:color|fill|stroke|background)$|^background$|^border(?:-[a-z]+)*-color$/;

/** Resolves the stylesheet and package imports of an app's entry stylesheet the way Vite does. */
function loadStylesheet(id: string, base: string) {
  let path = id.startsWith(".") || id.startsWith("/") ? resolve(base, id) : Bun.resolveSync(id, base);
  // A bare `@import "tailwindcss"` resolves to the package's JS entry; its CSS entry is index.css.
  if (extname(path) !== ".css") path = Bun.resolveSync(`${id}/index.css`, base);
  return Promise.resolve({ path, base: dirname(path), content: readFileSync(path, "utf8") });
}

function lineOf(content: string, position: number): number {
  let line = 1;
  for (let i = 0; i < position; i++) if (content.charCodeAt(i) === 10) line++;
  return line;
}

for (const entry of process.argv.slice(2).map((arg) => resolve(arg))) {
  const css = readFileSync(entry, "utf8");
  const base = dirname(entry);
  const design = await __unstable__loadDesignSystem(css, { base, loadStylesheet });
  const compiler = await compile(css, { base, loadStylesheet });
  const sources = compiler.sources.map((source) => ({ ...source, negated: source.negated }));
  if (compiler.root === null) sources.push({ base, pattern: "**/*", negated: false });
  else if (compiler.root !== "none") sources.push({ ...compiler.root, negated: false });

  const scanner = new Scanner({ sources });
  scanner.scan();
  let files = 0;
  for (const file of scanner.files) {
    // Stylesheets are compiled, not scanned for class names, and their comments name examples.
    if (extname(file) === ".css") continue;
    files++;
    const content = readFileSync(file, "utf8");
    const where = (position: number) => `${relative(process.cwd(), file)}:${lineOf(content, position)}`;
    for (const { candidate, position } of scanner.getCandidatesWithPositions({ content, extension: extname(file).slice(1) })) {
      for (const parsed of design.parseCandidate(candidate)) {
        const variants = parsed.variants.map((variant) => design.printVariant(variant));
        if (parsed.kind === "arbitrary") {
          if (colorProperties.test(parsed.property) || colorLiteral.test(parsed.value) || parsed.value.includes("--palette-")) {
            problems.push(`${where(position)}: ${candidate} sets a color without a semantic token (D8)`);
          }
          continue;
        }
        if (motionRoots.has(parsed.root)) {
          const none = parsed.kind === "functional" && parsed.value?.kind === "named" && parsed.value.value === "none";
          if (!none && !variants.includes("motion-safe")) {
            problems.push(`${where(position)}: ${candidate} animates outside motion-safe: (D6)`);
          }
        }
        if (parsed.kind === "functional" && parsed.value !== null) {
          const value = parsed.value;
          if (value.kind === "arbitrary" && tokenOnlyRoots.test(parsed.root) && !(parsed.root === "shadow" && value.dataType === "color")) {
            problems.push(`${where(position)}: ${candidate} uses an arbitrary value; use a token (app/ui/README.md)`);
          } else if (
            value.kind === "named" &&
            spacingRoots.test(parsed.root) &&
            /^[0-9.]+$/.test(value.value) &&
            !spacingSteps.has(value.value)
          ) {
            problems.push(`${where(position)}: ${candidate} uses a spacing step outside ${[...spacingSteps].join(", ")} (app/ui/README.md)`);
          }
        }
        if (parsed.kind !== "functional" || !colorRoots.has(parsed.root) || parsed.value === null) continue;
        if (parsed.value.kind === "arbitrary") {
          const value = parsed.value.value;
          if (parsed.value.dataType === "color" || colorLiteral.test(value) || value.includes("--palette-")) {
            problems.push(`${where(position)}: ${candidate} uses an arbitrary color; use a semantic token (D8)`);
          }
        } else if (isDefaultPaletteColor(parsed.value.value) && design.candidatesToCss([candidate])[0] === null) {
          problems.push(`${where(position)}: ${candidate} uses Tailwind's default palette; use a semantic token (D8)`);
        }
      }
    }
  }
  console.log(`Scanned ${files} source file(s) for ${relative(process.cwd(), entry)}`);
}

// --- 3. Generated token list --------------------------------------------------------------------

try {
  problems.push(...checkGenerated());
} catch (error) {
  problems.push((error as Error).message);
}

if (problems.length > 0) {
  console.error(`\n${problems.length} design token problem(s):`);
  for (const problem of problems) console.error(`  ${problem}`);
  process.exit(1);
}
console.log("Design tokens OK");
