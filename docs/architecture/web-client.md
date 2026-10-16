# Web クライアントの設計規約

Web クライアント（`app/web`）と共有 UI キット（`app/ui`）を書くときの規約です。
根拠は [ADR-0025](../adr/0025-web-client-stack.md)（技術スタック）、[ADR-0026](../adr/0026-client-workspace-and-component-architecture.md)（構成と責務分離）、[ADR-0027](../adr/0027-ui-quality-gates.md)（品質ゲート）、[ADR-0028](../adr/0028-html-shell-link-previews-and-pwa.md)（HTML の外枠と PWA）です。
主な出典は react.dev、WAI-ARIA Authoring Practices、WCAG 2.2、Tailwind CSS・TanStack・Testing Library の公式ドキュメントです。

「強制」の列は、lint・型検査・テストで機械的に検査するものを示します。「レビュー」はレビューで確認します。

## 1. React の基本

| # | 規約 | 強制 |
| --- | --- | --- |
| R1 | コンポーネントとフックは純粋にする。レンダリング中に副作用・変更・`Date.now()` / `Math.random()` を書かない | `react-hooks/purity`, `globals` |
| R2 | props・state・フックに渡した値・JSX に渡した値を変更しない（`push` / `sort` ではなく `toSorted` / スプレッド） | `react-hooks/immutability` |
| R3 | フックはコンポーネントか `use*` 関数のトップレベルでだけ呼ぶ | `react-hooks/rules-of-hooks` |
| R4 | React Compiler を全コードに適用する。コンパイラが最適化を諦めるコードは lint で失敗させる | `react-hooks` のコンパイラ由来のルールを error、`vite build`（コンパイラが 1 つも関数を変換しないと失敗） |
| R5 | `useMemo` / `useCallback` / `memo` は書かない。必要なとき（Effect の依存に安定した参照が要るなど）は理由をコメントに書く | `react-hooks/use-memo`、レビュー |
| R6 | 他の値から計算できるものは state にせず、レンダリング中に計算する | `react-hooks/set-state-in-effect` |
| R7 | 利用者の操作で起きる処理（送信・通知・計測）はイベントハンドラに書く。Effect に書かない | レビュー |
| R8 | Effect は外部のシステム（SSE、ブラウザ API、React 以外の部品）と同期するときだけ使い、必ず後始末を返す | レビュー |
| R9 | Effect の中で反応させたくない値は `useEffectEvent` で読む。`exhaustive-deps` を無効化しない | `react-hooks/exhaustive-deps` を error、無効化コメントの禁止 |
| R10 | ID が変わったら state を初期化したい場合は `key` を使う。Effect で初期化しない | レビュー |
| R11 | コンポーネントやフックを、ほかのコンポーネントの中で定義しない | `react-hooks/static-components`、`@eslint-react` |
| R12 | リストの `key` には安定した ID を使う。動的なリストに配列の添字を使わない | `@eslint-react` |
| R13 | `forwardRef`・`<Ctx.Provider>`・関数コンポーネントの `defaultProps` を使わない（`ref` は普通の props、`<Ctx value>`、引数の既定値） | `@eslint-react` |
| R14 | 数値を `&&` の条件にしない（`{count && <X />}` は `0` を表示する）。`count > 0 &&` と書く | `@eslint-react`（型情報あり） |

## 2. 状態とデータ

| # | 規約 | 強制 |
| --- | --- | --- |
| S1 | サーバーのデータは TanStack Query だけで扱う。取得は `features/<x>/api` の `queryOptions`、書き込みは `useMutation` で行い、成功時にキャッシュを無効化または更新する | `no-restricted-imports`（`fetch` / API クライアントを `features/*/api` と `lib/` の外で禁止）、`@tanstack/eslint-plugin-query` |
| S2 | サーバーのデータを `useState`・Context・グローバルストアにコピーしない | レビュー |
| S3 | 共有・リロード・戻る / 進むで保ちたい状態（タイムラインの種類・タブ・検索語・開いているメディア）は URL に置き、Zod で検証する | TanStack Router の型 |
| S4 | 状態は使う場所の近くに置き、必要なときだけ最も近い共通の親に持ち上げる | レビュー |
| S5 | 状態は判別可能なユニオンで表す（`{ status: 'idle' } \| { status: 'error'; error: Problem }`）。矛盾し得る真偽値を並べない | レビュー |
| S6 | Context は、変化の少ない値（閲覧者・ロケール・テーマ・複合部品の内部）にだけ使い、更新頻度ごとに分ける | レビュー |
| S7 | ルートは薄くする。パラメータと検索条件を取り出し、loader で `ensureQueryData` を呼んでから機能の部品を表示する | `@tanstack/eslint-plugin-router`、レビュー |
| S8 | 楽観的更新は、まず保留中の mutation の `variables` を表示する方式にする。複数の画面に反映が要るときだけ、キャッシュを書き換えて失敗時に戻す | レビュー |
| S9 | API の型は OpenAPI からの生成物だけを使う。同じ型を手で書かない | 生成物の差分検査（ADR-0029） |
| S10 | SSE の購読は `lib/sse` の 1 つの Effect だけが行い、受け取った内容を Query のキャッシュに書き込む | レビュー |

## 3. 構成と責務分離

| # | 規約 | 強制 |
| --- | --- | --- |
| A1 | 依存の向きは `routes → features → (@my/ui, lib)` だけにする | `eslint-plugin-boundaries` |
| A2 | `@my/ui` は React・React Aria Components・Tailwind にだけ依存する。API・Query・Router・機能・i18n のカタログを import しない（文言は props / children で受け取る） | `eslint-plugin-boundaries`、パッケージの依存関係 |
| A3 | 機能は、ほかの機能を `index.ts` 経由でだけ使う | `eslint-plugin-boundaries` |
| A4 | 機能のロジックは `features/<x>/hooks` のカスタムフックに置く。フックは JSX を返さない。機能の部品はフックの戻り値を `@my/ui` に渡すだけにする | レビュー |
| A5 | 操作できる要素は `@my/ui`（React Aria Components を包んだ部品）から使う。`onClick` を付けた `div` / `span` を書かない | `no-restricted-syntax` |
| A6 | `@my/ui` の部品は表示に専念し、状態はできるだけ外から渡す（`value` / `onChange`） | レビュー |
| A7 | props は型を明示する（`React.FC` を使わない）。ネイティブ要素の props は `ComponentPropsWithRef<'button'>` で拡張する。多態的な `as` props を使わない | `no-restricted-types`、レビュー |
| A8 | `dangerouslySetInnerHTML` は、サーバーでサニタイズ済みの投稿本文を表示する 1 つの監査済み部品（`PostContent`）でだけ使う | `@eslint-react/dom/no-dangerously-set-innerhtml`（その部品だけ許可） |
| A9 | 利用者に見える文字列はすべて `react-intl` を通す。JSX に文字列リテラルを書かない | `formatjs/no-literal-string-in-jsx` |

## 4. エラー・読み込み・遷移

| # | 規約 | 強制 |
| --- | --- | --- |
| E1 | ルートごとに `errorComponent` を持ち、ルートにも 1 つエラー境界を置く。独立して失敗し得る区画（埋め込みのカードなど）には `react-error-boundary` を置く | レビュー |
| E2 | mutation の失敗はエラー境界ではなく、mutation の状態と通知で扱う。RFC 9457 の `type` を、翻訳済みの文言に対応づける | レビュー |
| E3 | Suspense の境界は、スケルトンを見せる意味がある区画にだけ置く | レビュー |
| E4 | 画面遷移とタブの切り替えはトランジション（`startTransition`）にし、表示中の画面を保つ | レビュー |

## 5. アクセシビリティ

| # | 規約 | 強制 |
| --- | --- | --- |
| Y1 | すべてのストーリーと E2E の画面で、axe の違反（`wcag2a` / `wcag2aa` / `wcag21a` / `wcag21aa` / `wcag22aa`）がない | Storybook（`a11y.test: 'error'`）、Playwright |
| Y2 | 操作できる要素はすべてロールとアクセシブルネームを持つ。テストは `getByRole` / `getByLabelText` で探し、テスト ID は最後の手段にする | `eslint-plugin-testing-library` |
| Y3 | コントラストは AA（文字 4.5:1、UI 3:1）をライト / ダークの両方で満たす | 実ブラウザの axe |
| Y4 | フォーカスできる要素はすべて `focus-visible` のリングを持つ。固定ヘッダでフォーカスを隠さない（`scroll-padding`） | 見た目のスナップショット、レビュー |
| Y5 | ルートが変わったら、新しい画面の `h1`（`tabindex="-1"`）にフォーカスを移し、`document.title` を更新する | Playwright |
| Y6 | タイムラインは feed パターンにする（`role="feed"`、`<article>`、`aria-posinset` / `aria-setsize`、`aria-busy` を必ず戻す、Page Down / Page Up） | ARIA スナップショット、キーボードのテスト |
| Y7 | 新しい投稿を自動で差し込まない。件数・完了・エラーは、共通の `role="status"` / `role="alert"` の通知部品だけで知らせる。フォーカスを奪わない | ARIA スナップショット、レビュー |
| Y8 | CW は disclosure パターン（`aria-expanded` のボタン、閉じた内容は `hidden`）にする | 部品のテスト |
| Y9 | 画像の添付には代替テキストの入力欄を必ず出す。リモートの画像で代替テキストがないものには「説明なし」のバッジを付ける | 部品のテスト |
| Y10 | 利用者が書いた内容（表示名・投稿・プロフィール）は `dir="auto"` で表示し、分かる場合は `lang` を付ける | 部品のテスト、ARIA スナップショット |

## 6. レスポンシブとスタイル

| # | 規約 | 強制 |
| --- | --- | --- |
| D1 | モバイルファーストで書く（接頭辞なしが最小幅、`md:` は「md 以上」） | レビュー |
| D2 | 画面幅のブレークポイントはアプリの外枠だけで使う。部品にはコンテナクエリ（`@container`、`@md:`）を使う | パスごとの lint |
| D3 | 320 px のどのロケールでも横スクロールが出ない | Playwright（`scrollWidth <= clientWidth`） |
| D4 | タップ領域は 24 × 24 px 以上、主要な操作は `pointer-coarse:` で 44 px 以上にする | axe `target-size`、レビュー |
| D5 | `100vh` を使わない（`svh` / `dvh`）。端に固定する要素には `env(safe-area-inset-*)` を足す | lint（grep） |
| D6 | アニメーションは `motion-safe:` の下に置く | lint、レビュー |
| D7 | 方向は論理プロパティのユーティリティで書く。`ml-` / `mr-` / `pl-` / `pr-` / `left-` / `right-` / `text-left` / `text-right` / `rounded-l` / `rounded-r` / `border-l` / `border-r` を使わない（方向のあるアイコンの反転だけ `rtl:` / `ltr:` で許可） | lint |
| D8 | 色は意味のトークン（`--color-surface`、`--color-fg-muted` など）だけを使う。生のパレットや任意の色を部品に書かない（`--color-*: initial` で未定義の色をビルドエラーにする） | Tailwind の設定 |
| D9 | 部品の見た目の違いは `tailwind-variants` の 1 部品 1 レシピで定義する。クラス名を動的に組み立てない。`className` は `tailwind-merge` で root / スロットにだけ合成する | `eslint-plugin-better-tailwindcss`、レビュー |
| D10 | CSP を壊さない。`setAttribute('style')` や `cssText` を使わない（React の `style={}` は可） | lint |

## 7. テスト

| # | 規約 | 強制 |
| --- | --- | --- |
| T1 | 利用者の使い方に近い形でテストする。フックの内部や state を直接検査しない | レビュー |
| T2 | 操作は `user-event` で再現する。`screen` を使い、`waitFor` + `getBy` ではなく `findBy` を使う | `eslint-plugin-testing-library` |
| T3 | ネットワークは MSW だけでモックする。`fetch` や API のモジュールをモックしない | `no-restricted-syntax`、レビュー |
| T4 | 既定は統合テスト（本物の QueryClient とルーター + MSW）にする。純粋なロジックは単体テスト、主要な流れは 4 ロケールの E2E にする | レビュー |
| T5 | `@my/ui` の部品は必ずストーリーを持つ。ストーリーはアクセシビリティ・見た目・RTL のテストの入力になる | レビュー、`knip` |
| T6 | 部品のテストは jsdom かブラウザモードで動かす。React Compiler の preset はクライアント向けの変換にだけ効くので、Vitest の `node` 環境では本番と違うコードをテストすることになる | Vitest の設定 |

### テストの置き場所と道具

- 部品のテストは対象の隣に `*.test.ts(x)` で置き、`make web-test` で動かす（`app/web/vitest.config.ts`、jsdom）。
  - `src/test/setup.ts` が jest-dom のマッチャーを読み込み、MSW のサーバーを起動する。どのハンドラにも合わない要求はテストを失敗させる。
  - 既定のハンドラは `src/test/msw/handlers.ts` に、`api/openapi.yaml` の操作ごとに 1 つ置く。テストごとの差し替えは `server.use()` で行う。
  - 部品は `src/test/render.tsx` の `renderWithProviders` で描画する。`user-event` は描画の前に `setup()` する。
- E2E は `app/web/e2e/*.spec.ts` に置き、`make web-e2e` で動かす（`app/web/playwright.config.ts`、ビルドした `vite preview` が相手）。
  - プロジェクトは画面幅 320 / 390 / 768 / 1024 / 1440 px × ロケール `en` / `ja` / `en-XA` / `ar-XB` の 20 個。768 px 未満はタッチと粗いポインタのスマートフォンとして動かす。
  - 毎 PR は全ロケールの 320 / 1440 px だけを動かし、nightly（`E2E_MATRIX=full`）は全体を動かす。
  - spec は `e2e/fixtures.ts` から `test` / `expect` を import する。axe は `makeAxeBuilder` と `expectNoAxeViolations`（規則と対象のセレクタの一覧が空であることを検査）、横スクロールは `expectNoHorizontalScroll` で検査する。
  - 構造は `toMatchAriaSnapshot` で検査する。ロケールで変わる名前は、スナップショットから省くか正規表現にする。
  - CI は、digest で固定した Playwright の公式イメージで動かす。イメージの版は `@playwright/test` の版と合わせる。
