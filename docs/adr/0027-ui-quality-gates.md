# ADR-0027: UI のアクセシビリティ・レスポンシブ・見た目の品質ゲート

- Status: Accepted
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #87

## Context

Web クライアントは WCAG 2.2 AA を目標とする（NFR-A11Y-1）。RTL を含む多言語（ADR-0009）、スマートフォンから PC まで（ADR-0008）が対象になる。
自動検査（axe）で見つかる問題は件数で約 57% にとどまる（Deque の調査）。自動化の層を重ね、残りは手動の確認で補う必要がある。
ADR-0015 は E2E を `en` / `ja` / `en-XA` / `ar-XB` で実行すると定めているが、部品単位の検査と画面幅の扱いを定めていない。

## Decision

### 検査の層

| 層 | 道具 | 何を保証するか | CI |
| --- | --- | --- | --- |
| lint | ESLint（Testing Library・境界・Tailwind のルール）、物理方向のユーティリティの禁止 | クリックできる `div` の禁止、論理プロパティだけを使う、ロールで要素を探すテスト | 毎 PR |
| 部品（ロジック） | Vitest + Testing Library + `user-event`（DOM のシミュレーション） | ロールとアクセシブルネーム、キーボード操作、`aria-*` の状態 | 毎 PR |
| 部品（実ブラウザ） | Storybook 10 + a11y アドオン + Vitest ブラウザモード（Playwright） | すべてのストーリーで axe の違反がない（`a11y.test: 'error'`）、コントラスト、`ar-XB` での表示 | 毎 PR |
| 見た目 | Vitest ブラウザモードのスクリーンショット | 部品をコンテナ幅 3 種 × ライト / ダーク × LTR / RTL で比較 | 毎 PR |
| E2E | Playwright + `@axe-core/playwright`、ARIA スナップショット | 主要な流れ。画面幅 320 / 390 / 768 / 1024 / 1440 px × 4 ロケール | 毎 PR（主要な流れ）/ nightly（全体） |
| 手動 | VoiceOver（iOS / macOS）、TalkBack、NVDA、200% / 400% 拡大、強制カラーモード | 自動化できない部分 | リリースごと |

- axe のタグは `wcag2a`, `wcag2aa`, `wcag21a`, `wcag21aa`, `wcag22aa` を使う。
- スクリーンショットの基準画像は、CI の固定したコンテナで生成してコミットする。手元で生成した画像はコミットしない。

### 部品と画面のルール

規約の全文は [Web クライアントの設計規約](../architecture/web-client.md) にある。要点:

- **タイムラインは WAI-ARIA の feed パターンで作る。**
  - `role="feed"`、各投稿は `<article>`、`aria-posinset` / `aria-setsize`、読み込み中は `aria-busy` を付ける。
  - Page Down / Page Up で投稿間を移動する。
- **新しい投稿をタイムラインに自動で差し込まない。**「N 件の新しい投稿」ボタン（`role="status"`）にする（WCAG 2.2.2）。
- **ルートが変わったら、新しい画面の `h1` にフォーカスを移し、`document.title` を更新する。**
- **CW は disclosure パターンで作る。** `aria-expanded` を付けたボタンで開閉し、閉じた内容は `hidden` にする。
- **文字数カウンタは入力が止まったとき、またはしきい値（残り 50 / 20 / 0）で読み上げる。**
- **レスポンシブ**
  - モバイルファーストにする。画面幅のブレークポイントはアプリの外枠だけで使い、部品にはコンテナクエリを使う。
  - 320 px で横スクロールが出ない。
  - タップ領域は 24 px 以上、主要な操作は粗いポインタ（`pointer-coarse:`）で 44 px 以上にする。
  - 端に固定する要素には `env(safe-area-inset-*)` を足す。高さには `100vh` ではなく `svh` / `dvh` を使う。
  - アニメーションは `motion-safe:` の下に置く。

## Consequences

- 部品のストーリーが、そのままアクセシビリティ・見た目・RTL のテストの入力になる。部品を作るときはストーリーを必ず書く。
- CI に実ブラウザのジョブが増え、時間が延びる。毎 PR の範囲（主要な流れ）と nightly の範囲を分ける。
