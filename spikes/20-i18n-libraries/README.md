# Spike #20: i18n libraries (throwaway)

Throwaway code for [#20](https://github.com/best-friends/my/issues/20). Never import it from product code.
Findings: [docs/spikes/20-i18n-libraries.md](../../docs/spikes/20-i18n-libraries.md).

It shows one catalog, `locales/<BCP47>/<namespace>.json` (Format.JS JSON shape), formatted identically by
Go (`github.com/kaptinlin/messageformat-go/mf1`) and TypeScript (`intl-messageformat`) for `en`, `ja`, `en-XA`, `ar-XB`.

```sh
bun install
bun run test     # TS golden check + bun test + tsc (type-level test) + go test
bun run check    # catalog lint (ICU syntax, placeholders, CLDR categories, missing/obsolete keys, stale codegen)
go test -v ./... # per-engine parity matrix (kaptinlin / gotnospirit / fullpipe)

# regenerate
bun run pseudo   # locales/en-XA, locales/ar-XB from locales/en
bun run gen      # ts/messages.gen.ts + messages_gen.go
bun run golden   # testdata/golden.json, testdata/cldr.json (from the JS runtime's Intl)
bun run size     # bundle-size table for TS candidates
```

| Path | What |
| --- | --- |
| `locales/` | Shared catalog. `en-XA` / `ar-XB` are generated |
| `testdata/cases.json` | Keys + arguments formatted by both sides |
| `testdata/golden.json` | FormatJS output; Go must match it byte for byte |
| `testdata/cldr.json` | CLDR plural/number drift fixtures for 12 locales |
| `catalog.go`, `bundle.go` | Go loader (`embed`), precompiled bundle, fallback chain |
| `engines.go` | Go engine adapters under comparison |
| `messages_gen.go`, `ts/messages.gen.ts` | Generated typed accessors |
| `ts/` | Pseudo-locale generator, codegen, lint, translator, size measurement |
