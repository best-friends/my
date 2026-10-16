# Web クライアントの設計規約

Web クライアント（`app/web`）と共有 UI キット（`app/ui`）を書くときの規約です。
根拠は [ADR-0025](../adr/0025-web-client-stack.md)（技術スタック）、[ADR-0026](../adr/0026-client-workspace-and-component-architecture.md)（構成と責務分離）、[ADR-0027](../adr/0027-ui-quality-gates.md)（品質ゲート）、[ADR-0028](../adr/0028-html-shell-link-previews-and-pwa.md)（HTML の外枠と PWA）です。
主な出典は react.dev、WAI-ARIA Authoring Practices、WCAG 2.2、Tailwind CSS・TanStack・Testing Library の公式ドキュメントです。

「強制」の列は、lint・型検査・テストで機械的に検査するものを示します。「レビュー」はレビューで確認します。
lint は `make web-lint`（`make web-check` と `make check` の一部）で実行します。設定は `app/eslint.config.js`、独自の制限は `app/eslint/restrictions.js` にあり、
`app/eslint/restrictions.test.js` が各制限の違反例と許可例を検査します。ESLint は `--max-warnings 0` で動かすので、警告も失敗になります。
ルールを無効化するコメントには理由（`-- 理由`）が必要です（`@eslint-community/eslint-comments/require-description`）。

## 1. React の基本

| # | 規約 | 強制 |
| --- | --- | --- |
| R1 | コンポーネントとフックは純粋にする。レンダリング中に副作用・変更・`Date.now()` / `Math.random()` を書かない | `react-hooks/purity`、`react-hooks/globals` |
| R2 | props・state・フックに渡した値・JSX に渡した値を変更しない（`push` / `sort` ではなく `toSorted` / スプレッド） | `react-hooks/immutability`（代入を検出する。`push` / `sort` などのメソッド呼び出しはレビュー） |
| R3 | フックはコンポーネントか `use*` 関数のトップレベルでだけ呼ぶ | `react-hooks/rules-of-hooks` |
| R4 | React Compiler を全コードに適用する。コンパイラが最適化を諦めるコードは lint で失敗させる | `eslint-plugin-react-hooks` の `recommended-latest`（`react-hooks/incompatible-library`・`react-hooks/unsupported-syntax` も error）、`vite build`（コンパイラが 1 つも関数を変換しないと失敗） |
| R5 | `useMemo` / `useCallback` / `memo` は書かない。必要なとき（Effect の依存に安定した参照が要るなど）は理由をコメントに書く | `no-restricted-imports`・`no-restricted-syntax`（`useMemo` / `useCallback` / `memo` の使用。必要なときは理由付きの無効化コメント）、`react-hooks/use-memo` |
| R6 | 他の値から計算できるものは state にせず、レンダリング中に計算する | `react-hooks/set-state-in-effect`、`react-hooks/no-deriving-state-in-effects` |
| R7 | 利用者の操作で起きる処理（送信・通知・計測）はイベントハンドラに書く。Effect に書かない | レビュー |
| R8 | Effect は外部のシステム（SSE、ブラウザ API、React 以外の部品）と同期するときだけ使い、必ず後始末を返す | レビュー |
| R9 | Effect の中で反応させたくない値は `useEffectEvent` で読む。`exhaustive-deps` を無効化しない | `react-hooks/exhaustive-deps` を error、`@eslint-community/eslint-comments/no-restricted-disable`（`react-hooks/*` の無効化を禁止） |
| R10 | ID が変わったら state を初期化したい場合は `key` を使う。Effect で初期化しない | レビュー |
| R11 | コンポーネントやフックを、ほかのコンポーネントの中で定義しない | `react-hooks/static-components`、`@eslint-react/no-nested-component-definitions` |
| R12 | リストの `key` には安定した ID を使う。動的なリストに配列の添字を使わない | `@eslint-react/no-array-index-key`、`@eslint-react/no-missing-key` |
| R13 | `forwardRef`・`<Ctx.Provider>`・関数コンポーネントの `defaultProps` を使わない（`ref` は普通の props、`<Ctx value>`、引数の既定値） | `@eslint-react/no-forward-ref`、`@eslint-react/no-context-provider`、`no-restricted-syntax`（`defaultProps`） |
| R14 | 数値を `&&` の条件にしない（`{count && <X />}` は `0` を表示する）。`count > 0 &&` と書く | `@eslint-react/no-leaked-conditional-rendering`（型情報あり） |

## 2. 状態とデータ

| # | 規約 | 強制 |
| --- | --- | --- |
| S1 | サーバーのデータは TanStack Query だけで扱う。取得は `features/<x>/api` の `queryOptions`、書き込みは `useMutation` で行い、成功時にキャッシュを無効化または更新する | `features/*/api` と `lib/` の外で `no-restricted-imports`（`openapi-fetch`・`openapi-react-query`・`lib/api`）・`no-restricted-globals` / `no-restricted-properties`（`fetch`）、`@tanstack/eslint-plugin-query` の `flat/recommended-strict`（`@tanstack/query/exhaustive-deps`・`@tanstack/query/prefer-query-options` など） |
| S2 | サーバーのデータを `useState`・Context・グローバルストアにコピーしない | レビュー |
| S3 | 共有・リロード・戻る / 進むで保ちたい状態（タイムラインの種類・タブ・検索語・開いているメディア）は URL に置き、Zod で検証する | TanStack Router の型 |
| S4 | 状態は使う場所の近くに置き、必要なときだけ最も近い共通の親に持ち上げる | レビュー |
| S5 | 状態は判別可能なユニオンで表す（`{ status: 'idle' } \| { status: 'error'; error: Problem }`）。矛盾し得る真偽値を並べない | レビュー |
| S6 | Context は、変化の少ない値（閲覧者・ロケール・テーマ・複合部品の内部）にだけ使い、更新頻度ごとに分ける | レビュー |
| S7 | ルートは薄くする。パラメータと検索条件を取り出し、loader で `ensureQueryData` を呼んでから機能の部品を表示する | `@tanstack/eslint-plugin-router` の `flat/recommended`（`@tanstack/router/create-route-property-order` など）、レビュー |
| S8 | 楽観的更新は、まず保留中の mutation の `variables` を表示する方式にする。複数の画面に反映が要るときだけ、キャッシュを書き換えて失敗時に戻す | レビュー |
| S9 | API の型は OpenAPI からの生成物だけを使う。同じ型を手で書かない | 生成物の差分検査（ADR-0029） |
| S10 | SSE の購読は `lib/sse` の 1 つの Effect だけが行い、受け取った内容を Query のキャッシュに書き込む | レビュー |

## 3. 構成と責務分離

| # | 規約 | 強制 |
| --- | --- | --- |
| A1 | 依存の向きは `routes → features → (@my/ui, lib)` だけにする | `boundaries/dependencies`（要素は `shell`（`web/src` 直下）・`route`・`feature`・`lib`・`ui`）、`boundaries/no-unknown-files` |
| A2 | `@my/ui` は React・React Aria Components・Tailwind にだけ依存する。API・Query・Router・機能・i18n のカタログを import しない（文言は props / children で受け取る） | `boundaries/dependencies`（`ui` が import できるのは `ui` 自身と React・React Aria・`tailwind-variants`・`tailwind-merge` だけ）、パッケージの依存関係（Bun の isolated linker） |
| A3 | 機能は、ほかの機能を `index.ts` 経由でだけ使う | `boundaries/dependencies`（ほかの `feature` は `index.ts` だけ） |
| A4 | 機能のロジックは `features/<x>/hooks` のカスタムフックに置く。フックは JSX を返さない。機能の部品はフックの戻り値を `@my/ui` に渡すだけにする | レビュー |
| A5 | 操作できる要素は `@my/ui`（React Aria Components を包んだ部品）から使う。`onClick` を付けた `div` / `span` を書かない | `no-restricted-syntax`（`div` / `span` の `onClick`・`onKeyDown`・`onPointerDown` など） |
| A6 | `@my/ui` の部品は表示に専念し、状態はできるだけ外から渡す（`value` / `onChange`） | レビュー |
| A7 | props は型を明示する（`React.FC` を使わない）。ネイティブ要素の props は `ComponentPropsWithRef<'button'>` で拡張する。多態的な `as` props を使わない | `@typescript-eslint/no-restricted-types`（`FC` / `FunctionComponent`）、レビュー |
| A8 | `dangerouslySetInnerHTML` は、サーバーでサニタイズ済みの投稿本文を表示する 1 つの監査済み部品（`PostContent`）でだけ使う | `@eslint-react/dom-no-dangerously-set-innerhtml`（`PostContent.tsx` だけ許可） |
| A9 | 利用者に見える文字列はすべて `react-intl` を通す。JSX に文字列リテラルを書かない | `formatjs/no-literal-string-in-jsx`（子要素と `aria-*`・`label`・`placeholder`・`title`・`alt`。テストとストーリーは除く） |

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
| Y2 | 操作できる要素はすべてロールとアクセシブルネームを持つ。テストは `getByRole` / `getByLabelText` で探し、テスト ID は最後の手段にする | `eslint-plugin-testing-library` の `flat/react`、`testing-library/no-test-id-queries`（使うときは理由付きの無効化コメント） |
| Y3 | コントラストは AA（文字 4.5:1、UI 3:1）をライト / ダークの両方で満たす | `make web-tokens`（トークンの組）、実ブラウザの axe |
| Y4 | フォーカスできる要素はすべて `focus-visible` のリングを持つ。固定ヘッダでフォーカスを隠さない（`scroll-padding`） | 見た目のスナップショット、レビュー |
| Y5 | ルートが変わったら、新しい画面の `h1`（`tabindex="-1"`）にフォーカスを移し、`document.title` を更新する | Playwright |
| Y6 | タイムラインは feed パターンにする（`role="feed"`、`<article>`、`aria-posinset` / `aria-setsize`、`aria-busy` を必ず戻す、Page Down / Page Up） | ARIA スナップショット、キーボードのテスト |
| Y7 | 新しい投稿を自動で差し込まない。件数・完了・エラーは、共通の `role="status"` / `role="alert"` の通知部品（`@my/ui` の `AnnouncerProvider` / `useAnnouncer`、見える通知は `ToastProvider` / `useToast`）だけで知らせる。フォーカスを奪わない | ARIA スナップショット、部品のテスト（Announcer のストーリー）、レビュー |
| Y8 | CW は disclosure パターン（`aria-expanded` のボタン、閉じた内容は `hidden`）にする | 部品のテスト |
| Y9 | 画像の添付には代替テキストの入力欄を必ず出す。リモートの画像で代替テキストがないものには「説明なし」のバッジを付ける | 部品のテスト |
| Y10 | 利用者が書いた内容（表示名・投稿・プロフィール）は `dir="auto"` で表示し、分かる場合は `lang` を付ける | 部品のテスト、ARIA スナップショット |

## 6. レスポンシブとスタイル

| # | 規約 | 強制 |
| --- | --- | --- |
| D1 | モバイルファーストで書く（接頭辞なしが最小幅、`md:` は「md 以上」） | レビュー |
| D2 | 画面幅のブレークポイントはアプリの外枠だけで使う。部品にはコンテナクエリ（`@container`、`@md:`）を使う | `ui/src` と `features` の `.tsx` で `better-tailwindcss/no-restricted-classes`（`sm:`〜`2xl:`・`max-*:`・`min-[…]:`） |
| D3 | 320 px のどのロケールでも横スクロールが出ない | Playwright（`scrollWidth <= clientWidth`） |
| D4 | タップ領域は 24 × 24 px 以上、主要な操作は `pointer-coarse:` で 44 px 以上にする | axe `target-size`、レビュー |
| D5 | `100vh` を使わない（`svh` / `dvh`）。端に固定する要素には `env(safe-area-inset-*)` を足す | `no-restricted-syntax`（文字列）、`better-tailwindcss/no-restricted-classes`（`h-screen` など）、`make web-lint` の grep（CSS） |
| D6 | アニメーションは `motion-safe:` の下に置く | `make web-tokens`、`better-tailwindcss/no-restricted-classes`（`animate-*`）、レビュー |
| D7 | 方向は論理プロパティのユーティリティで書く。`ml-` / `mr-` / `pl-` / `pr-` / `left-` / `right-` / `text-left` / `text-right` / `rounded-l` / `rounded-r` / `border-l` / `border-r` を使わない（方向のあるアイコンの反転だけ `rtl:` / `ltr:` で許可） | `better-tailwindcss/no-restricted-classes` |
| D8 | 色は `@my/ui/theme.css` の意味のトークン（`--color-surface`、`--color-fg-muted` など）だけを使う。生のパレット（`--palette-*`）や任意の色を部品に書かない（`--color-*: initial` で Tailwind の既定の色を消す）。トークンの一覧・コントラストの組・ライト / ダークの切り替えは [app/ui/README.md](../../app/ui/README.md) にある | Tailwind の設定、`make web-tokens`、`better-tailwindcss/no-unknown-classes` |
| D9 | 部品の見た目の違いは `tailwind-variants` の 1 部品 1 レシピで定義する。クラス名を動的に組み立てない。`className` は `tailwind-merge` で root / スロットにだけ合成する | `better-tailwindcss/no-concatenated-classes`、`better-tailwindcss/no-conflicting-classes`、レビュー |
| D10 | CSP を壊さない。`setAttribute('style')` や `cssText` を使わない（React の `style={}` は可） | `no-restricted-syntax` |

## 7. テスト

| # | 規約 | 強制 |
| --- | --- | --- |
| T1 | 利用者の使い方に近い形でテストする。フックの内部や state を直接検査しない | レビュー |
| T2 | 操作は `user-event` で再現する。`screen` を使い、`waitFor` + `getBy` ではなく `findBy` を使う | `testing-library/prefer-user-event`、`testing-library/prefer-screen-queries`、`testing-library/prefer-find-by` |
| T3 | ネットワークは MSW だけでモックする。`fetch` や API のモジュールをモックしない | テストの `no-restricted-syntax`（`api` を含むモジュールと `openapi-fetch` の `vi.mock` / `vi.doMock`、`fetch` の `vi.spyOn` / `vi.stubGlobal` / 代入）、レビュー |
| T4 | 既定は統合テスト（本物の QueryClient とルーター + MSW）にする。純粋なロジックは単体テスト、主要な流れは 4 ロケールの E2E にする | レビュー |
| T5 | `@my/ui` の部品は必ずストーリーを持つ。ストーリーはアクセシビリティ・見た目・RTL のテストの入力になる | レビュー、`knip`（使われないファイル・export・依存） |
| T6 | 部品のテストは jsdom かブラウザモードで動かす。React Compiler の preset はクライアント向けの変換にだけ効くので、Vitest の `node` 環境では本番と違うコードをテストすることになる | Vitest の設定 |

### テストの置き場所と道具

- 部品のテストは対象の隣に `*.test.ts(x)` で置き、`make web-test` で動かす（`app/web/vitest.config.ts`、jsdom）。
  - `src/test/setup.ts` が jest-dom のマッチャーを読み込み、MSW のサーバーを起動する。どのハンドラにも合わない要求はテストを失敗させる。
  - 既定のハンドラは `src/test/msw/handlers.ts` に、`api/openapi.yaml` の操作ごとに 1 つ置く。テストごとの差し替えは `server.use()` で行う。
  - 部品は `src/test/render.tsx` の `renderWithProviders` で描画する。`user-event` は描画の前に `setup()` する。
  - `@my/ui` の部品のテストも `app/ui/src` の部品の隣に置き、`make web-test` で動かす（`app/ui/vitest.config.ts` の `unit` プロジェクト、jsdom）。
    `app/ui/test/setup.ts` が jest-dom のマッチャーを読み込む。キットは通信しないので MSW もプロバイダもなく、Testing Library の `render` で描画する。
- E2E は `app/web/e2e/*.spec.ts` に置き、`make web-e2e` で動かす（`app/web/playwright.config.ts`、ビルドした `vite preview` が相手）。
  - サーバーが描画する画面（§8）の E2E（`e2e/auth-pages.spec.ts`）は、本物の `my serve` が相手になる。`e2e/server.ts` が一時ディレクトリの SQLite に `schema/sqlite.sql` を適用し、
    管理者とサードパーティの OAuth クライアントを入れてから、`make build` で作った `bin/my` を起動する。ホストは `localhost`（`__Host-` の Cookie を HTTP で受け取れるのは `localhost` だけ）。
  - JavaScript を無効にしたテスト（`javaScriptEnabled: false`）で、ログイン・登録・認可同意のフォームがスクリプトなしで動くことを確かめる。axe はページの中でスクリプトを動かすので、その画面の axe は同じ Cookie の JavaScript ありのコンテキストで行う。
  - サーバーのレート制限（登録は 1 つのアドレスから 30 分に 5 回、ログインの失敗は 1 分に 5 回）を使い切らないように、アカウントを作るテストは 1 つのプロジェクト（`en-320`）だけで動かし、
    エラーの表示は制限を数える前に断られる入力（期限切れのフォーム、形式の誤り）で確かめる。
  - プロジェクトは画面幅 320 / 390 / 768 / 1024 / 1440 px × ロケール `en` / `ja` / `en-XA` / `ar-XB` の 20 個。768 px 未満はタッチと粗いポインタのスマートフォンとして動かす。
  - 毎 PR は全ロケールの 320 / 1440 px だけを動かし、nightly（`E2E_MATRIX=full`）は全体を動かす。
  - spec は `e2e/fixtures.ts` から `test` / `expect` を import する。axe は `makeAxeBuilder` と `expectNoAxeViolations`（規則と対象のセレクタの一覧が空であることを検査）、横スクロールは `expectNoHorizontalScroll` で検査する。
  - 構造は `toMatchAriaSnapshot` で検査する。ロケールで変わる名前は、スナップショットから省くか正規表現にする。
  - CI は、digest で固定した Playwright の公式イメージで動かす。イメージの版は `@playwright/test` の版と合わせる。
- `@my/ui` のストーリーは `app/ui/src/*.stories.tsx` に置く（CSF 3、`satisfies Meta`）。Storybook は `make storybook` で開く（`app/ui/.storybook`）。
  - ツールバーでロケール（`en` / `ja` / `en-XA` / `ar-XB`）とテーマ（ライト / ダーク）を切り替える。ロケールは `<html>` の `lang` / `dir` と React Aria の `I18nProvider` に、テーマは `<html>` の `data-theme` に反映する。
  - ストーリーは `make ui-test` で実ブラウザのテストになる（`app/ui/vitest.config.ts`、Storybook の Vitest アドオンと Vitest のブラウザモード、Playwright の Chromium）。
    - プロジェクトは方向 × テーマの 4 つ（`en` / `ar-XB` × ライト / ダーク）。すべてのストーリーを 4 つすべてで動かす。
    - axe の違反が 1 つでもあれば失敗する（`parameters.a11y.test: 'error'`、Y1 のタグ）。
    - `@my/ui` の部品の振る舞いのテスト（フォーカスの閉じ込めと復帰、Esc、矢印キー）は、ストーリーの `play` 関数に `storybook/test` の `expect` と文脈の `userEvent` / `canvas` で書く。
      4 つのプロジェクトすべてで動くので、RTL の矢印キーは `document.documentElement.dir` で向きを決めて同じ `play` で検査する。
    - ダイアログ・ポップオーバー・トーストなどのオーバーレイは、`<body>` ではなくストーリーの枠に portal する（`UNSAFE_PortalProvider`）。枠は `contain: layout` なので、`position: fixed` のオーバーレイも枠を覆い、スクリーンショットに写る。
      オーバーレイのストーリーは、デコレーターで枠に高さを与える。
    - 描画と axe のあと、ストーリーの枠（コンテナクエリの基準）を幅 320 / 640 / 1024 px にして `toMatchScreenshot` で比較する（`.storybook/vitest.setup.ts`）。
    - スクリーンショットは `prefers-reduced-motion: reduce` を模擬して撮る。その状態で枠の中に動いているアニメーションが 1 つでもあれば失敗する（D6）。ストーリーの描画と `play` 関数は模擬なしで動く。
    - `play` 関数（`storybook/test`）で、部品をロールとアクセシブルネームで探して検査する（Y2）。
  - 基準画像は `app/ui/src/__screenshots__/` にあり、固定した Playwright のイメージ（`Makefile` の `PLAYWRIGHT_IMAGE`）の中でだけ作る。
    手元の OS のフォントや描画では一致しないので、手元で作った画像はコミットしない。
    - 見た目を変えたら `make ui-snapshots-update`（Docker でそのイメージを使う）で作り直し、差分の画像を確かめてからコミットする。
    - 同じ比較を手元でするときは `make ui-test-docker` を使う。`make ui-test` を手元で動かす（`make ui-test-install` でブラウザを入れる）と、axe は信用できるが、スクリーンショットは一致しない。
  - `make check` はブラウザなしで動く検査だけを集める。`make ui-test` は CI の `ui-test` ジョブがそのイメージの中で毎 PR 動かす。失敗したときの実際の画像と差分は成果物 `ui-test-screenshots` にある。
  - ワークフローのイメージと `PLAYWRIGHT_IMAGE`、`playwright` / `@playwright/test` の版が食い違うと、`make playwright-image-check`（`make check` の一部）が失敗する。

## 8. サーバーが描画する画面

ログイン・登録・メール確認・アカウントの状態・OAuth の認可同意の画面は、Go の `html/template`（`internal/interface/authpages`）が描画し、JavaScript なしで動く（ADR-0030）。

- スタイルは Vite のビルドの別のエントリ `app/web/src/server-pages.css` にある。Tailwind と `@my/ui/theme.css` を読み込み、テンプレートが使う少数のクラス（`page`・`card`・`field`・`hint`・`field-error`・`alert`・`notice`・`button` など）を
  トークンのユーティリティの `@apply` で定義する。テンプレートからクラスを集めないので、Tailwind はソースを走査しない（`source(none)`）。
- `my serve` はビルドのマニフェストのキー `src/server-pages.css` からファイル名を引き（`web.Handler.Stylesheet`）、画面の `<link rel="stylesheet">` にする。SPA の外枠はこの CSS を読み込まない。
  Vite の開発サーバーを使うときはビルドがないので、画面にスタイルは付かない。
- CSP は `style-src 'self'` で、インラインのスタイルとスクリプトを置かない。
- ダークモードは `prefers-color-scheme` に従う（トークンの `light-dark()` と `<meta name="color-scheme" content="light dark">`）。フォーカスのリングは `theme.css` の `:focus-visible`、
  タップ領域は `pointer-coarse:` で 44 px 以上、方向は論理プロパティ（`px-`・`ps-`・`pe-`）で書く（D4・D7・Y4）。
- フォームは `<label for>`、`autocomplete`（`username`・`current-password`・`new-password`）、ヒントとエラーの `aria-describedby`、エラーの `aria-invalid` を持ち、ブラウザの制約検証（`required`・`minlength`）を使う。
