# ADR-0025: Web クライアントの技術スタック

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #85

## Context

Web クライアント（PWA、ADR-0008）は React と TailwindCSS をベースにする（オーナーの指定）。
React は、ネット上のコードや一般的な知識に古い・誤った書き方（Effect の濫用、手動メモ化、サーバーデータのコピーなど）が多い。
そのため、React 公式ドキュメント（react.dev）などの一次情報に基づいて、周辺ライブラリと書き方を決める必要がある。
調査結果は #84 の調査報告にある（各ライブラリの版は 2026-10-16 時点の npm `latest`）。

## Decision Drivers

1. React 公式の設計原則（Rules of React、React Compiler、You Might Not Need an Effect）に沿って書けること
2. アクセシビリティ（WCAG 2.2 AA）・RTL・タッチ操作を、部品の土台で保証できること（ADR-0009, NFR-A11Y-1）
3. サーバーが Go の SPA であること（RSC・SSR は使わない、ADR-0028）
4. OpenAPI の型（ADR-0016）をそのまま使えること
5. 依存のライセンスが AGPL-3.0 と両立すること（ADR-0011）
6. 後で Tauri 2（ADR-0008）と UI を共有できること

## Considered Options

- 部品の土台: React Aria Components / Base UI / Radix / Ark UI / 自作
- ルーター: TanStack Router / React Router（v8 の library / data mode）
- サーバー状態: TanStack Query / React 19 の `use` + Suspense だけ / SWR
- テストランナー: Vitest / `bun test`

## Decision（推奨案）

| 領域 | 採用 |
| --- | --- |
| UI ライブラリ | React 19（`react` / `react-dom` 19.3 系）、Strict Mode |
| メモ化 | React Compiler 1.0（`babel-plugin-react-compiler`）を全コードに適用。手動の `useMemo` / `useCallback` / `memo` は原則書かない |
| 言語 | TypeScript（`strict`、`noUncheckedIndexedAccess`、`exactOptionalPropertyTypes`）。typescript-eslint の対応範囲に合わせて 6.0 系に固定 |
| ビルド | Vite 8 + `@vitejs/plugin-react` 6。Bun をパッケージ管理とスクリプト実行に使う |
| スタイル | Tailwind CSS v4（`@tailwindcss/vite`）、`tailwind-variants`、`tailwind-merge` |
| アクセシブルな部品の土台 | React Aria Components（Apache-2.0）。自前の UI キット（ADR-0026）が包んで公開する |
| ルーティング | TanStack Router（ファイルベース、自動コード分割、Zod で検索パラメータを検証） |
| サーバー状態 | TanStack Query 5（`queryOptions`、無限スクロールは `useInfiniteQuery`） |
| 長いリスト | TanStack Virtual |
| API クライアント | `openapi-typescript` + `openapi-fetch` + `openapi-react-query`（ADR-0029） |
| i18n | `react-intl`（FormatJS、ADR-0009）。カタログと型付きアクセサは `tools/i18n` が生成 |
| 検証 | Zod 4（検索パラメータとフォーム） |
| エラー境界 | `react-error-boundary` |
| テスト | Vitest（ブラウザモードを含む）、Testing Library、`user-event`、MSW、Playwright + `@axe-core/playwright`（ADR-0027） |
| 部品カタログ | Storybook 10（`@storybook/react-vite`、a11y アドオン、Vitest アドオン） |
| lint | ESLint（flat config）、typescript-eslint、`eslint-plugin-react-hooks`（コンパイラ由来のルールを含む `recommended`）、`@eslint-react`、TanStack Query / Router のプラグイン、`eslint-plugin-testing-library`、`eslint-plugin-boundaries`、`eslint-plugin-better-tailwindcss`、`knip` |
| Node.js | Storybook・Playwright・Vitest のブラウザモードは Node.js を前提とするため、Node.js（LTS）を `.tool-versions` で固定する |

選定の理由:

- **React Aria Components**: 候補の中で i18n（`ja` / `ar` の組み込み文言、RTL でのキー操作の反転）が最も強い。主要なスクリーンリーダーでテストされている。マウス・タッチ・キーボードの押下を正規化するので、Tauri の WebView にも向く。
- **TanStack Router**: React Router の型安全なルートは framework mode 限定で、SPA では型安全にならない。
- **TanStack Query**: React 19 の `use` はキャッシュを持たず、無効化・ページングも提供しない。
- **Vitest**: React Compiler の変換をテストにも同じように適用でき、Storybook と組み合わせて実ブラウザでアクセシビリティの検査ができる。

具体的なバージョンは各パッケージの `package.json` と lockfile で固定する。公開から 2 週間未満の版は使わない。

## Consequences

- 書き方の規約は [Web クライアントの設計規約](../architecture/web-client.md) にまとめ、可能なものは lint で強制する。
- Node.js と Bun の 2 つの JS 実行環境を固定・保守する。
- React Compiler がルール違反のコンポーネントを黙って最適化から外すので、関連する lint は error にする。

## Open Questions / 要検証

- [ ] Vite 8 と Vitest 5 を `bun --bun` で動かすか、Node.js で動かすか。CI では Node.js でのビルドも確認する
- [ ] TypeScript 7（Go 製コンパイラ）への移行時期（typescript-eslint の対応待ち）
