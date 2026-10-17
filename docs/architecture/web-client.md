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
| R4 | React Compiler を全コードに適用する。コンパイラが最適化を諦めるコードは lint とビルドで失敗させる。コンパイラから外す（`"use no memo"`）のは、コンパイルできないライブラリを包む 1 つのフックだけにし、理由を書く（`@my/ui` の `Feed` が TanStack Virtual の仮想化を読むフック） | `eslint-plugin-react-hooks` の `recommended-latest`（`react-hooks/incompatible-library`・`react-hooks/unsupported-syntax` も error）、`vite build`（`app/web/vite-plugins/react-compiler-check.ts`。コンパイラが関数を 1 つでも諦めると、場所と理由を出して失敗する。`"use no memo"` で外した関数は除く。1 つも関数を変換しなくても失敗）、`no-restricted-syntax`（`"use no memo"` / `"use no forget"`。必要なときは理由付きの無効化コメント。API クライアントのフック `api.query.useQuery` / `api.query.useMutation` の呼び出し、§10） |
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
| S7 | ルートは薄くする。パラメータと検索条件を取り出し、loader でキャッシュになければ取得して（`queryClient.query({ ...options, staleTime: "static" })`。無限スクロールは `queryClient.infiniteQuery`。非推奨の `ensureQueryData` は使わない）から機能の部品を表示する | `@tanstack/eslint-plugin-router` の `flat/recommended`（`@tanstack/router/create-route-property-order` など）、レビュー |
| S8 | 楽観的更新は、まず保留中の mutation の `variables` を表示する方式にする。複数の画面に反映が要るときだけ、キャッシュを書き換えて失敗時に戻す | レビュー |
| S9 | API の型は OpenAPI からの生成物だけを使う。同じ型を手で書かない | 生成物の差分検査（ADR-0029） |
| S10 | SSE の購読は `lib/sse` の 1 つの Effect だけが行い、受け取った内容を Query のキャッシュに書き込む | レビュー |

## 3. 構成と責務分離

| # | 規約 | 強制 |
| --- | --- | --- |
| A1 | 依存の向きは `routes → features → (@my/ui, lib)` だけにする | `boundaries/dependencies`（要素は `shell`（`web/src` 直下）・`route`・`feature`・`lib`・`ui`、テストだけが import できる `test`（`web/src/test`）と `feature-test`（`web/src/features/*/test`））、`boundaries/no-unknown-files` |
| A2 | `@my/ui` は React・React Aria Components・Tailwind のユーティリティと、ドメインを持たない振る舞いのライブラリ（許可リストは ADR-0031）にだけ依存する。API・Query・Router・機能・i18n のカタログを import しない（文言は props / children で受け取る） | `boundaries/dependencies`（`ui` が import できるのは `ui` 自身と React・React Aria・`@tanstack/react-virtual`・`tailwind-variants`・`tailwind-merge` だけ）、パッケージの依存関係（Bun の isolated linker） |
| A3 | 機能は、ほかの機能を `index.ts` 経由でだけ使う | `boundaries/dependencies`（ほかの `feature` は `index.ts` だけ） |
| A4 | 機能のロジックは `features/<x>/hooks` のカスタムフックに置く。フックは JSX を返さない。機能の部品はフックの戻り値を `@my/ui` に渡すだけにする | レビュー |
| A5 | 操作できる要素は `@my/ui`（React Aria Components を包んだ部品）から使う。`onClick` を付けた `div` / `span` を書かない | `no-restricted-syntax`（`div` / `span` の `onClick`・`onKeyDown`・`onPointerDown` など） |
| A6 | `@my/ui` の部品は表示に専念し、状態はできるだけ外から渡す（`value` / `onChange`） | レビュー |
| A7 | props は型を明示する（`React.FC` を使わない）。ネイティブ要素の props は `ComponentPropsWithRef<'button'>` で拡張する。多態的な `as` props を使わない | `@typescript-eslint/no-restricted-types`（`FC` / `FunctionComponent`）、レビュー |
| A8 | `dangerouslySetInnerHTML` は、サーバーでサニタイズ済みの投稿本文を表示する 1 つの監査済み部品（`PostContent`）でだけ使う。`PostContent` は本文を `SanitizedHtml` 型で受け取る。この型は `serverSanitizedHtml()` でだけ作り、API の応答でサーバーがサニタイズ済みと定める項目を読む場所でだけ呼ぶ | `@eslint-react/dom-no-dangerously-set-innerhtml`（`app/ui/src/PostContent.tsx` だけ許可）、型検査（ただの `string` は `SanitizedHtml` にならない）、レビュー |
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
| Y5 | ルートが変わったら、新しい画面の `h1`（`tabindex="-1"`）にフォーカスを移し、`document.title` を更新する。外枠が 1 か所で行う（§9） | Playwright、統合テスト |
| Y6 | タイムラインは feed パターンにする（`@my/ui` の `Feed`。`role="feed"`、`<article>`、`aria-posinset` / `aria-setsize`（総数が分からないときは `-1`）、`aria-busy` を必ず戻す、Page Down / Page Up で記事の間を移動し、Control + End / Control + Home で feed の外に出る）。仮想化しても、フォーカスのある記事は表示範囲の外でもマウントしておく | ARIA スナップショット、キーボードのテスト（`Feed` の部品のテストとストーリー） |
| Y7 | 新しい投稿を自動で差し込まない。「N 件の新しい投稿」は `@my/ui` の `NewItemsButton`（`role="status"` の中のボタン）で知らせ、押されたときだけ差し込む。それ以外の件数・完了・エラーは、共通の `role="status"` / `role="alert"` の通知部品（`@my/ui` の `AnnouncerProvider` / `useAnnouncer`、見える通知は `ToastProvider` / `useToast`、操作の結果を画面に残すときは `StatusMessage`）だけで知らせる。フォーカスを奪わない | ARIA スナップショット、部品のテスト（Announcer・NewItemsButton のテストとストーリー）、レビュー |
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
| D7 | 方向は論理プロパティのユーティリティで書く。`ml-` / `mr-` / `pl-` / `pr-` / `left-` / `right-` / `text-left` / `text-right` / `rounded-l` / `rounded-r` / `border-l` / `border-r` / `mask-l-` / `mask-r-` を使わない（方向のあるアイコンの反転と、論理方向の版がない `mask-l-` / `mask-r-`（端のフェード）だけ、`rtl:` / `ltr:` の下で許可） | `better-tailwindcss/no-restricted-classes` |
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
| T7 | 非同期の結果は Testing Library の `findBy` / `waitFor` で待つ。`vi.waitFor` は使わない。待ち時間の上限はテストごとに変えず、setup の `asyncUtilTimeout` と Vitest の `testTimeout` にまとめる（待ちは実時間なので、負荷の高い機械では既定の 1 秒・5 秒を超える）。React の描画の外で動くフォーカス（React Aria のフォーカスの復帰は `requestAnimationFrame`、ルート変更の見出しへのフォーカスはルーターの `onRendered`）は `waitFor` の中で `toHaveFocus()` を検証する | テストの `no-restricted-syntax`（`vi.waitFor`）、Vitest の設定、レビュー |

### テストの置き場所と道具

- 部品のテストは対象の隣に `*.test.ts(x)` で置き、`make web-test` で動かす（`app/web/vitest.config.ts`、jsdom）。
  - `src/test/setup.ts` が jest-dom のマッチャーを読み込み、MSW のサーバーを起動する。どのハンドラにも合わない要求はテストを失敗させる。
    `findBy` / `waitFor` の上限（`asyncUtilTimeout`）は 10 秒、1 つのテストの上限（`vitest.config.ts` の `testTimeout`）は 30 秒にする（T7）。
    通るテストはこの上限まで待たない。上限は壊れたテストが止まるまでの時間だけを決める。
  - 既定のハンドラは `api/openapi.yaml` の操作ごとに 1 つ置き、その操作を使う機能が持つ。テストごとの差し替えは `server.use()` で行う。
    - 機能のハンドラは `features/<x>/test/handlers.ts` に置き、`handlers` として export する。その機能のテストだけが使うフィクスチャも同じファイルに置く。
    - 複数の機能が使うフィクスチャ（インスタンス・アカウント・投稿・関係・設定・メディア・タイムラインのページ）は `src/test/fixtures.ts` に 1 つずつ置き、機能のファイルで作り直さない。
      API の型は生成された型（`Schemas`）から取る（S9）。
    - ハンドラが共有する部品（`apiBase`・`problem()`・アクセストークンの検査 `withViewer()`・OAuth の応答・`signedInHandler`）と、インスタンスと OAuth のエンドポイントの既定のハンドラは `src/test/msw/http.ts`、
      `GET /streaming` は `src/test/msw/streaming.ts` に置く。
    - `src/test/msw/handlers.ts` はこれらのモジュールを 1 行ずつ `export * as` で束ねるだけにし、`src/test/msw/server.ts` がその全部の `handlers` で MSW のサーバーを作る。
      機能を足すときに共有ファイルで変えるのは、`handlers.ts` への 1 行だけである。
      1 つの操作に既定のハンドラは 1 つなので、束ねる順番は結果を変えない（`handlers.test.ts` が重複を検査する）。
    - `features/<x>/test` は `boundaries` の要素 `feature-test` で、`src/test` と同じ機能のテストからだけ import できる。`src/test` を import できる。
  - 部品は `src/test/render.tsx` の `renderWithProviders`（ランタイム設定と i18n のプロバイダ）で描画する。`user-event` は描画の前に `setup()` する。
  - 統合テストは同じファイルの `renderApp` でアプリ全体（本物のルーターをメモリ上の履歴で、新しい QueryClient）を描画する。`path` で開くアドレス、`locale` でロケールを指定する。
    サインインした状態は `server.use(signedInHandler)`（リフレッシュがハンドラの受け付けるアクセストークンを返す）で作る。
  - jsdom の `FormData` と `File` の本文は、MSW が相手にする Node.js の `fetch` で送れない。ファイルを送るテストは `src/test/multipart.ts` の `nodeUploadFile()` で
    Node.js の `File` を作り、そのテストの間だけ `FormData` を Node.js のものにする。
  - `@my/ui` の部品のテストも `app/ui/src` の部品の隣に置き、`make web-test` で動かす（`app/ui/vitest.config.ts` の `unit` プロジェクト、jsdom）。
    `app/ui/test/setup.ts` が jest-dom のマッチャーを読み込み、`app/web` と同じ待ち時間の上限を設定する。キットは通信しないので MSW もプロバイダもなく、Testing Library の `render` で描画する。
    構造は `app/ui/test/aria-snapshot.ts` の `ariaSnapshot()`（Playwright の ARIA スナップショットと同じ形の、ロール・名前・説明・状態の木）で検査する。
- E2E は `app/web/e2e/*.spec.ts` に置き、`make web-e2e` で動かす（`app/web/playwright.config.ts`、ビルドした `vite preview` が相手）。
  - `vite preview` と `e2e/server.ts` は、実行ごとに選んだ空きポートで待ち受ける（`E2E_PREVIEW_PORT` / `E2E_SERVER_PORT` で固定できる）。
    この実行が起動していないサーバーは再利用しないので、同じマシンの複数の worktree で同時に動かしても互いのサーバーを相手にしない。
    起動済みのサーバーを相手にするときは `E2E_BASE_URL`（クライアント）と `E2E_SERVER_URL`（`my serve -pseudo-locales`）を指定する。
  - サーバーが描画する画面（§8）の E2E（`e2e/auth-pages.spec.ts`）と、本物の CSP の下で SPA がログインして API に届くことを確かめる E2E（`e2e/csp.spec.ts`、CSP の違反が 1 件でもあれば失敗）は、本物の `my serve` が相手になる。`e2e/server.ts` が一時ディレクトリの SQLite に `schema/sqlite.sql` を適用し、
    管理者とサードパーティの OAuth クライアントを入れてから、`make build` で作った `bin/my` を起動する。ホストは `localhost`（`__Host-` の Cookie を HTTP で受け取れるのは `localhost` だけ）。
    `MY_WEB_ORIGIN` は `http://localhost:<port>` なので、外枠のランタイム設定の `apiOrigin` もそれになり、SPA は同じサーバーの API に HTTP で届く。
    疑似ロケールは利用者に出さないので、`my serve` は開発・テスト専用のフラグ `-pseudo-locales` を付けたときだけ `en-XA` / `ar-XB` を選べる。`e2e/server.ts` はこれを付けて起動し、各ページがプロジェクトのロケールの `lang` と向き（`ar-XB` は `rtl`）で描画されることを確かめる。
  - Playwright は Service Worker を止めて動かす（`serviceWorkers: 'block'`）。Service Worker が遷移に応えると、ロケールの fixture の `page.route()` を通らないため。
    Service Worker を検査する `e2e/pwa.spec.ts` だけが許可し、本物の `my serve` を相手に 1 つのプロジェクト（`en-1440`、Chromium）で動かす（§11）。
  - JavaScript を無効にしたテスト（`javaScriptEnabled: false`）で、ログイン・登録・認可同意のフォームがスクリプトなしで動くことを確かめる。axe はページの中でスクリプトを動かすので、その画面の axe は同じ Cookie の JavaScript ありのコンテキストで行う。
  - M2 の主要な流れ（`e2e/main-flow.spec.ts`）も本物の `my serve` が相手になる。2 人の利用者がそれぞれのブラウザコンテキストで、登録、サーバーの画面からのログイン（PKCE）、CW と代替テキスト付きの画像のある投稿、
    ホームとローカルのタイムライン、フォロー（フォロワーを承認する鍵アカウントへのリクエストと承認を含む）、フォロワー限定の投稿の見え方、通知までを通し、途中の画面ごとに axe・横スクロール・ARIA スナップショットを検査する。
    利用者は毎回ランダムな名前で作るので、再試行や他のプロジェクトの投稿・フォローと混ざらない。他のプロジェクトも投稿するローカルタイムラインでは、Page Down で feed を進めて自分の投稿を探す。
    `bin/my` のコマンド（`my admin create`、`my registration approve`）は、`e2e/server-cli.ts` が `e2e/server.ts` と同じ設定（実行ごとに選ぶ一時ディレクトリと鍵）でサーバーのデータベースに対して実行する。
    アプリは利用者の表示言語で表示されるので、ログインした利用者の表示言語をクライアント API（`PATCH /api/v1/preferences`）でプロジェクトのロケールにしてから進める。
  - サーバーのレート制限（登録は 1 つのアドレスから 30 分に 5 回、ログインの失敗は 1 分に 5 回）を使い切らないように、登録の画面からアカウントを作るテストは 1 つのプロジェクト（`en-320`）だけで動かす。
    主要な流れも `en-320` でだけ登録の画面から登録し、`my registration approve` で承認する。他のプロジェクトと 2 人目の利用者は `my admin create` で作る。
    エラーの表示は制限を数える前に断られる入力（期限切れのフォーム、形式の誤り）で確かめる。
  - axe は React Aria のライブアナウンサー（`[data-live-announcer]`）を検査しない（`e2e/fixtures.ts` の `axeBuilder`）。React Aria の Button は待機の始まりと終わりを、ボタンを `aria-labelledby` で指す要素で 7 秒間そこに知らせるので、
    その間に画面の移動でボタンが消えると、名前のない要素が残るため。
  - プロジェクトは画面幅 320 / 390 / 768 / 1024 / 1440 px × ロケール `en` / `ja` / `en-XA` / `ar-XB` の 20 個。768 px 未満はタッチと粗いポインタのスマートフォンとして動かす。
  - 毎 PR は全ロケールの 320 / 1440 px だけを動かし、nightly（`E2E_MATRIX=full`）は全体を動かす。
  - spec は `e2e/fixtures.ts` から `test` / `expect` を import する。`vite preview` は `index.html`（ランタイム設定の代わりを含む）をそのまま返すので、fixture がプロジェクトのロケールを `<html lang dir>` に書き込み、サーバーの外枠と同じ状態にする。ビルドのファイルへの相対のリンクは、`my serve` と同じくルートからのものにする（ルートより下の画面のため）。ランタイム設定の `apiOrigin` もその回の `vite preview` のオリジンに書き換えるので、クライアントの API と OAuth の要求はプレビューに届き、spec が `page.route` で応答できる（ログイン済みの画面は、`/oauth/token/refresh` と API の操作に応答して作る）。axe は `makeAxeBuilder` と `expectNoAxeViolations`（規則と対象のセレクタの一覧が空であることを検査）、横スクロールは `expectNoHorizontalScroll` で検査する。
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
      `play` 関数のためだけにある長いストーリー（仮想化した `Feed`）は `no-screenshot` のタグで比較を外す。同じ部品のほかのストーリーが見た目を検査する。
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

## 9. アプリの外枠

`app/web/src` 直下（`shell`）と `routes/__root.tsx` が、すべての画面に共通する部分を 1 か所で受け持つ。機能とルートはこれを作り直さない。

- **CSP に収める**: 外枠の CSP（`require-trusted-types-for 'script'` と nonce のない `style-src 'self'`）を破るライブラリの動作は止めておく。
  Zod はスキーマを作るときに `eval` でのコンパイルを試すので、`"zod"` の import は Vite の alias で `lib/zod.ts`（`jitless` にした Zod）に向ける。
  React Aria の `usePress` が `<head>` に差し込む `<style>` は、`main.tsx` が最初に同じ id の空の `<template>` を置いて止め（`lib/csp.ts`）、その規則は `index.css` に置く。
  Trusted Types の下では `innerHTML` に文字列を入れられないので、`PostContent`（A8）はサニタイズ済みの本文を、そのまま通すだけの Trusted Types のポリシー `my-server-sanitized-html` で TrustedHTML にしてから入れる。
  文字列を TrustedHTML にするのはこのポリシーだけで、受け取るのは `SanitizedHtml` だけにする。
  このポリシーがそのまま通してよいのは、渡る HTML がすべてサーバーのサニタイザ（Go の許可リスト）を通ったものだからである。連合で届くリモートの HTML（M3）も、クライアントに渡す前に同じサニタイザを通す。
  - 外枠の CSP は `trusted-types my-server-sanitized-html my-service-worker` で、作れるポリシーをクライアントの 2 つ（`PostContent` の HTML と、`lib/pwa.ts` の Service Worker の URL）に限る。
    どちらもそれぞれ 1 つのモジュールで、最初に使うときに 1 回だけ作るので、`'allow-duplicates'` は付けない。ほかのスクリプト（依存ライブラリを含む）は自分のポリシーを作れない。
    Vite・React・React Aria・Zod（`jitless`）はポリシーを作らない。ポリシーを作るコードを足すときは、この一覧と `internal/interface/web/shell.go` の CSP を一緒に変える。
  - 止められない動作が 1 つ残る: iOS / iPadOS の WebKit では、React Aria の `usePreventScroll`（モーダルの overlay と、Select / Menu のようなモーダルの popover が使う）が、開いている間 `<head>` の先頭に `* { overscroll-behavior: contain }` の `<style>` を差し込む。
    これを止める手段はなく、外枠の CSP がこの `<style>` を拒むので、iOS では overlay を開くたびに CSP の違反が 1 件報告される。この報告は受け入れ、CSP（ADR-0019 §9）は変えない。
    代わりに `@my/ui` の Dialog / Sheet / Popover / Select / Menu は、スクロールする面に `overscroll-contain` を自分で付け、overlay の中のスクロールがページに伝わらないようにする。overlay の中に別のスクロールする領域を作るときも、その領域に `overscroll-contain` を付ける。
    Chromium はこの経路を通らないので、`e2e/csp.spec.ts` はこの違反を検出しない。
- **起動**（`main.tsx`）: サーバーが外枠に書いた `<html lang>` から、カタログのある最も近いロケールを決め（`ja-JP` → `ja`、ない言語は `en`）、`<html>` の `lang` / `dir` をそれに合わせる。
  次に `<script type="application/json" id="config">` のランタイム設定を Zod で検証して読む（`lib/config`、`useConfig()`）。読めなければアプリの代わりに起動エラーの画面を出す。
  読めたら、表示のロケール・QueryClient・セッション・API クライアント（`lib/services.ts`）を作り、リフレッシュトークンの Cookie からセッションを再開し始める（§10）。
- **表示のロケールとタイムゾーン**（`lib/locale.tsx`、FR-007）: ページを開いている間に変わるので、React の外のストア（`AppLocale`、`useSyncExternalStore` で読む）に置く。
  サインインした利用者の設定（`GET /preferences`）を読むと、外枠（`AppLayout` が呼ぶ `features/settings` の `useUserDisplaySettings`）がその `locale` と `time_zone` をストアに渡す。
  ストアはそのロケールのカタログを読み込んでから `<html>` の `lang` / `dir` を変え、i18n のプロバイダを描画し直す。読み込みが前後したときは最後に頼んだロケールが勝つ。
  タイムゾーンは `react-intl` の `timeZone` になる。ロケールが変わると、サーバーが `Accept-Language` で訳した内容を取り直すために、Query をすべて無効にする。
  サインインしていないときと設定を読むまでは、サーバーのロケールのままになる。表示言語の選択肢は、カタログのあるロケール（疑似ロケールは、今それで表示しているときだけ）を、それぞれの言語の名前で並べる。
  タイムゾーンの選択肢は、表示中のロケールの CLDR の名前（`Intl.DisplayNames` にタイムゾーンの種類はないので、`Intl.DateTimeFormat` の `timeZoneName: "longGeneric"`。名前のないゾーンは UTC からの差だけ）に、
  画面を開いた時点の UTC からの差を付けて「（UTC+09:00）日本標準時」と出し、その差と名前の順に並べる。IANA の名前（`Asia/Tokyo`）は、同じ名前のゾーンを見分けるための選択肢の説明（`SelectItem` の `description`）にする。
- **未保存の変更**: プロフィールの編集のように、まとめて保存するフォームは、保存していない変更があるまま別の画面へ移るとき（検索条件とハッシュだけの変化は除く）に、TanStack Router の `useBlocker`（`withResolver`）で遷移を止め、
  `ConfirmDialog`（`alertdialog`）で確かめる。フォーカスは「編集を続ける」から始まり、Esc もそれと同じく画面に残る。再読み込みとタブを閉じるときは、ブラウザ自身の確認（`beforeunload`）になる。
  変更は保存済みの値と比べて求めるので、手で元に戻した変更や保存した後は確かめない。設定を変えたとたんに保存する画面（表示と投稿）は対象にしない。
- **プロバイダ**（`App.tsx`）: 外側から、ランタイム設定、表示のロケール、i18n（`react-intl` と React Aria の `I18nProvider`）、QueryClient（`lib/query`）、セッションと API クライアント（`lib/auth/SessionProvider`、§10）、`AnnouncerProvider` / `ToastProvider`、React Aria の `RouterProvider`、ルーターの順に置く。
- **文言**（`lib/i18n`）: クライアントの文言は `locales/<locale>/client.json` にあり、ロケールごとに別のチャンクで読み込む。足りない文言は `ja-JP` → `ja` → `en` の順に補う。
  ID の型は `tools/i18n` が生成する `MessageArgs` から作る（`ClientMessageKey`）。文字列にするときは `useT()` の `t("client:…", 引数)` を使い、ID と引数の型を検査する。
- **ルーティング**（`router.ts`）: TanStack Router のファイルルート。Vite のプラグインが `routes/` から `routeTree.gen.ts` を生成する（自動コード分割）。生成物はコミットし、`tsc` がビルドの前に使う。
  - ルーターは遷移を `startTransition` の中で確定するので、次の画面を読み込む間も今の画面が残る（E4）。読み込みが `defaultPendingMs` を超えたときだけ `RoutePending` を出す。
  - `-` で始まるファイルとディレクトリはルートにならない。外枠の部品は `routes/-layout/` に置く。
  - プロフィールのアドレスはサーバーの外枠と同じ `/@username`（リモートのアカウントは `/@username@domain`）で、ルートは接頭辞付きのパラメータ（`routes/@{$acct}.tsx`）にする。
    パラメータの `@` を `%40` にしないように、ルーターは `pathParamsAllowedCharacters: ["@"]` にする。複数の機能が指す画面のアドレスは `lib/paths.ts` で作る。
- **リンク**: 画面の間のリンクは `@my/ui` の `Link` に `href` を渡す。React Aria の `RouterProvider` が同じオリジンの `href` をルーターの遷移に変える。`#id` の `href`（本文へ移動するリンク）は、その要素にフォーカスを移す。
- **レイアウト**（`routes/-layout/AppLayout.tsx`）: モバイルファーストで、`md` 未満は下端のタブバー、`md` から横のレール、`lg` から文言付きのサイドバー、`xl` から投稿フォームとサーバーについての 3 列目を出す。
  `xl` 未満では、投稿フォームはタブバーの上（`md` からは画面の隅）に浮かぶボタンから開くシートになる（§12）。どちらにするかは外枠が `matchMedia` で決め（`useMediaQuery`）、ページに置くフォームは常に 1 つにする。
  浮かぶボタンは、ルートの静的なデータ（TanStack Router の `staticData`、`composeButton: true`）を持つ画面（タイムラインとプロフィール）にだけ出す（G4）。外枠は今のマッチ（`useMatches`）からそれを読む。
  ほかの画面でもシートは置いたままにし、返信のボタンはそこで開く。`md` 未満のボタンはアイコンだけで、「投稿を書く」をアクセシブルネーム（`aria-label`）と `Tooltip` にする（G12）。
  `main` は終わりに、タブバーとボタン（出ているとき、`data-compose-fab`）の高さぶんの余白を取り、`index.css` の `scroll-padding-block-end` もフォーカスした要素を同じだけ避ける。`md` 未満のトーストは、タブバーの上に、ボタンが出ているときはボタンの上に出す（`with-compose-fab:` の variant）。
  `md` 未満で高さが 32rem 以下の画面（横向きの電話、400% の拡大の 320 × 225 CSS px）は `short:` の variant（`index.css`）で、タブバーの項目を `min-h-12` にし、ボタンとの間を詰めて、固定した要素を画面の高さの半分に収める（WCAG 1.4.10）。
  ナビゲーション（`routes/-layout/Navigation.tsx`）は 1 つの `nav` で、今いる区画のリンクに `aria-current="page"` を付ける。画面の端に接する要素は `env(safe-area-inset-*)` を足し、高さは `svh` で指定する。
  - 項目は [ADR-0039](../adr/0039-mobile-navigation-and-compose-entry.md) に従う。`md` 未満のタブバーは、ホーム・タイムライン・通知・検索・メニューの 5 つで、
    `md` からのレールとサイドバーは、ホーム・ローカル・連合・通知・検索・プロフィール・設定の 7 つにする。どちらも 1 つのリストの同じ順番で、各項目はその幅で出ないときに `display: none`（アクセシビリティツリーからも外れる）にする。
    jsdom はスタイルを当てないので、統合テストでは両方の項目が文書にあり、どの幅で何が出るかは E2E（`e2e/shell.spec.ts`・`e2e/federated.spec.ts`）で確かめる。
  - タブバーの各項目は文言の幅を基本にし、残りの幅を等分する（`flex: 1 1 auto`）。長い文言（「Notifications」）が短い文言（「Menu」）の余りを使うので、320 px の `en`・`ja`・`ar-XB` で 1 行に収まる。
    文言は省略せず、収まらない翻訳では単語の途中でも折り返して、全体を読めるようにする。
  - 「タイムライン」はローカルのタイムライン（ローカルを無効にし連合を有効にしたサーバーでは連合）へのリンクで、`/local` と `/public` のどちらでも `aria-current="page"` になる。
    `/local` と `/public` の画面は、見出しの下に 2 つのタイムラインを切り替えるタブ（`features/timeline` の `TimelineTabs`）を持つ。タブは React Aria の Tabs に `href` を付けたリンクで、
    選択は URL から決まる（S3）。タブを移ると別の画面になり、フォーカスはその画面の `h1` に移るので、矢印キーではフォーカスだけを動かし、Enter / Space で開く（`keyboardActivation="manual"`）。
    タブはサーバーが提供するタイムラインと今の画面のものだけを出し、`GET /instance` の答えを待たずに最初から置く（答えが後から届いても、下のタイムラインを作り直さない）。
  - 「メニュー」は場所ではなくボタンなので `aria-current` を持たない。押すと `Sheet`（モーダルのダイアログ）を開き、タブバーにないレールの項目（プロフィール・設定）を並べて、今いる区画のリンクに `aria-current="page"` を付ける。
    シートはパスの変わる遷移が始まったとき（ルーターの `onBeforeNavigate`）に閉じるので、新しい画面の `h1` がフォーカスを受け取れる。未保存の変更の確かめで止めた遷移は始まらないので、シートは開いたまま残る。
  - ローカル（`/local`）と連合（`/public`）のタイムラインは、インスタンスの設定で無効にでき、そのときは API が 404 を返す（FR-202）。レールとタイムラインのタブは `GET /instance` の `timelines` で、
    サーバーが提供するものだけを出す（`features/timeline` の `useTimelineAvailability`）。ローカルはサーバーが無効と答えるまで出し、連合はサーバーが有効と答えてから出すので、無効にしたサーバーでは読み込み中も失敗したときも出ない。
    タブバーの項目の数は、サーバーが何を提供しても 5 つのまま変わらない。
  - 最後に受け取った `timelines` は localStorage の `my:instance:timelines` に残し（インスタンスの設定で、利用者のデータではない）、ナビゲーションを置いたときに 1 回だけ読む。
    オフラインで開いたりリロードしたりして `GET /instance` が届かないときも、前と同じ項目を出す。サーバーの答えが届けばそちらを使う。
  - 無効にしたタイムラインのアドレスを直接開いたとき（両方を無効にしたサーバーでタブバーの「タイムライン」を開いたときを含む）は、「もう一度試す」ではなく無効であることを出す（`routes/local.tsx`・`routes/public.tsx`）。
  先頭に、フォーカスしたときだけ見える「本文へ移動」のリンクを置く。
- **フォーカスとタイトル**（`routes/-layout/useViewChange.ts`、Y5）: ルーターが画面を描画するたびに、`main` の `h1` の文字列から `document.title` を `client:shell.document_title`（画面名とサーバー名）で作る。
  パスが変わったときだけ、その `h1` にフォーカスを移す。最初の画面と、検索条件やハッシュだけの変化（タブ・絞り込み）ではフォーカスを動かさない。画面の見出しは `PageHeading`（`tabindex="-1"` の `h1`）で書く。
- **エラー**（E1）: 画面の失敗はルーターの `defaultErrorComponent`（`RouteError`）がレイアウトの中に出し、「もう一度試す」で Query のエラー境界を戻してから loader を再実行する。
  レイアウト自体の失敗はルートの `errorComponent`（`RootError`）が画面全体に出す。合うルートのないアドレスは `NotFound` を出す。独立した区画（3 列目など）は `react-error-boundary` で囲む。
- **オフライン**（`lib/network.ts`、G17）: オンラインかどうかは TanStack Query の `onlineManager` 1 つに置き、Query のキャッシュとオフラインの帯が同じ状態を読む。
  ブラウザが `navigator.onLine` と `offline` / `online` のイベントでネットワークがないと言うとき（起動時も `navigator.onLine` から始める）と、
  API クライアントの要求が応答を受け取れなかったとき（`NetworkError`。次に応答が届くまで）にオフラインとする。
  - レイアウトが画面の上端に `OfflineBanner`（`role="status"`）を 1 つ置き、最初から空のまま置いておくので、オフラインになると読み上げられる。安全領域の上の余白は帯が自分で取るので、その余白を取る要素より前に置く。
  - query と mutation は `networkMode: "always"` にする。オフラインでも止めて待たずにすぐ `NetworkError` で失敗するので、区画は終わらないスピナーではなく読み込めない理由を出し、loader も待ち続けない。
    ブラウザがオフラインのときの `NetworkError` の文言は「オフラインのため、サーバーに接続できません」（`client:api.error.offline`）になる。`NetworkError` の再試行は待たずにすぐ行い、知らせを遅らせない（`retry` は既定のままにし、loader の `queryClient.query` は再試行しない）。
  - オンラインに戻ると、失敗していた query（表示中のもの）だけを取り直す（既定の `refetchOnReconnect` が失敗した query にだけ `"always"` を返す）。タイムラインは自分で `refetchOnReconnect: false` にしているので取り直さない（§10）。
    `NetworkError` で失敗した画面は、`RouteError` がオンラインに戻ったときに「もう一度試す」と同じことをして読み込み直す。
  - 通知の画面の loader は失敗しても画面を開き、一覧が見出しの下に自分の失敗を出す（`useNotifications`。読み込みの失敗はマウントで再試行しない）。

## 10. API クライアントと認証

API の呼び出しとトークンは `lib/api`・`lib/auth` が 1 か所で扱う（ADR-0029、ADR-0030）。機能はこれを作り直さない。

- **型**（S9）: `make generate` が openapi-typescript で `api/openapi.yaml` から `lib/api/schema.gen.ts` を生成する。生成物はコミットし、`make generate-check` が差分を検査する。
  機能は API の型を自分の `features/<x>/api` から export して使う（`lib/api` を直接 import できるのは `features/*/api` と `lib/` だけ。S1）。
- **クライアント**（`lib/api/client.ts`）: openapi-fetch のクライアントと、その上の openapi-react-query のクライアントを 1 つずつ作る（`Api` の `fetch` と `query`）。
  機能は `features/<x>/api` で `api.query.queryOptions(...)` から `queryOptions` を作る。キーは `[method, path, params]` になる。
  書き込みは、`features/<x>/api` が `api.fetch` で作る `mutationKey` / `mutationFn` を TanStack Query の `useMutation` に渡す。
  openapi-react-query のフック（`api.query.useQuery` / `api.query.useMutation` など）は呼ばない（`no-restricted-syntax`）。オブジェクトから読むフックは描画ごとに同じ関数だと示せないので、React Compiler が呼んだフックごと最適化を諦め、`vite build` が失敗する（R4、[Don't dynamically use Hooks](https://react.dev/reference/rules/react-calls-components-and-hooks#dont-dynamically-use-hooks)）。
  ページを `next_cursor` でたどる一覧（タイムライン）は、openapi-react-query に無限クエリの `queryOptions` がないので、TanStack Query の `infiniteQueryOptions` で `api.fetch` を呼んで作る。
  キーは同じ形の末尾に `"pages"` を足したもの（`["get", "/timelines/local", "pages"]`）にし、同じ操作の普通のクエリと重ならないようにする（`features/timeline/api`）。
  - 表示のロケール（§9）を要求ごとに `Accept-Language` で送り、サーバーが problem の `title` / `detail` をそのロケールで返すようにする。
  - ミドルウェアが、セッションのアクセストークンを `Authorization: Bearer` で付ける。最初の要求は、起動時のセッションの再開（`session.restore()`）を待ってから送る。
  - トークンを付けた要求が 401 になったら、トークンを 1 度だけ更新し、要求の写しを新しいトークンで送り直す。2 度目の 401 はそのまま失敗にする。
  - 失敗した応答は `lib/errors.ts` の `ApiError`（RFC 9457 の problem そのものに `message` を足したもの）に、通信の失敗は `NetworkError` にして投げる。
    どちらも `message` がローカライズ済みで、そのまま表示できる（`NetworkError` は、ブラウザがオフラインならそう言う文言になる。§9 のオフライン）。応答が届いたか届かなかったかは `lib/network.ts` に伝える。problem でない本文（プロキシの HTML など）は、クライアントのカタログの文言（`client:api.error.status`）にする。
    生成された型は失敗をすべて problem と呼ぶので、表示する文言は `errorMessage(error)` で取る。
- **セッション**（`lib/auth/session.ts`）: アクセストークンはこのオブジェクトのメモリにだけ置き、localStorage・sessionStorage・IndexedDB・読める Cookie には書かない。
  リフレッシュトークンは HttpOnly の `__Host-my_rt` Cookie にあり、`POST /oauth/token/refresh`（`X-My-Token-Refresh: 1`、`credentials: "include"`）だけがそれを読む。
  - サーバーはリフレッシュトークンを厳格にローテーションし、同じトークンが 2 度使われるとトークン系列ごと失効させる。そこで、更新はオリジンで 1 つの Web Lock（`navigator.locks.request("my:oauth-refresh", …)`）の中で行う。
    更新したタブは新しいアクセストークンを BroadcastChannel（`my:oauth-session`）で他のタブに送る。ロックを待っていたタブは、その間に届いたトークンを使う。
    届いていなければ自分で更新するが、前の更新でローテーションされた Cookie を送るので、再利用にはならない。同じタブの中の同時の更新は 1 つの要求を共有する。
  - 認可コードの交換も同じロックの中で行い、起動時の再開と競合しない。
  - サインイン・サインアウト・セッションの終了（更新が 4xx で断られた）も BroadcastChannel で他のタブに伝える。
  - 利用者が変わったとき（サインイン、またはセッションのあったタブでの終了）、`lib/auth/userData.ts` が Query のキャッシュをすべて消す。セッションが終わったときは、`lib/pwa` の `clearCaches()` で Cache Storage もすべて消す（§11）。
    セッションが終わったときは、sessionStorage の `my:user:` で始まるキー（利用者が入力したもの。投稿の下書き、§12）も消す。サインインでは消さない（キーにアカウントの ID を含める）。
  - 状態（`unknown` / `signed-in` / `signed-out`）は `useSessionStatus()` で読む。サーバーのデータである閲覧者のアカウントは Context に置かず、`features/session` の `useViewer()` が Query で取る（S2、S6）。
- **ログイン**（`lib/auth/signIn.ts`、Authorization Code + PKCE）: `/login` のボタンが、新しい verifier（32 バイトの乱数）と `state` を作り、S256 の challenge を付けてサーバーの認可画面（`/oauth/authorize`）にページごと遷移する。
  scope は指定せず、Web クライアントに登録されたすべてのスコープを受け取る。
  - verifier・`state`・戻り先は、往復の間だけ sessionStorage（このタブだけ）に置き、コールバックが読んだ時点で消す。どれも資格情報ではない（コードは 1 回限り・短命で、verifier に結び付く）。
  - コールバック（`/login/callback`、`routes/login_.callback.tsx`）は `beforeLoad` で `state` を照合し、コードを交換してから、戻り先に `replace` で遷移する。コードは履歴にもアドレスバーにも残らない。
    失敗（拒否・`state` の不一致・交換の失敗・通信の失敗）は、そのルートの `errorComponent` が理由とやり直しのリンクを出す。
  - 戻り先（`return_to`）は、1 つの `/` で始まるパスだけを受け付ける（`//example.com` や `javascript:` は捨てる）。
- **リアルタイム更新**（`lib/sse`、S10、FR-207）: レイアウトの `useLiveUpdates`（`routes/-layout`）が、サインイン中だけ `lib/sse` の `useEventStream` で `GET /streaming` を 1 本開く。
  `EventSource` はヘッダを送れないので、API クライアントの `fetch`（`parseAs: "stream"`）で読む。トークンの付与と 401 のあとの 1 度の更新は、ほかの要求と同じミドルウェアが行う。
  本文は `lib/sse/parse.ts` が HTML Standard の手順で解釈する（`id` と `retry` は使わない）。
  - ストリームが終わったら約 1 秒後に、失敗したら失敗のたびに倍の間隔（最大 1 分、後半の半分にばらす）で接続し直す。セッションが終わるか外枠が消えると、Effect の後始末が要求を中止する。
  - サーバーは何も再送しないので、2 本目以降のストリームが開いたら、ホームの最初のページを読み直して新着を数え、未読数を読み直し、通知の一覧を古いことにする（再取得はしない）。
  - `update` は表示中のタイムラインに差し込まず、Query のキャッシュの別の項目（`features/timeline/api/newStatuses.ts`）に待たせる。ホームの `NewItemsButton` が、読み込み済みの投稿にないものの数を出し、押されたら最初のページの先頭に差し込んで見出しにフォーカスを移す（Y7）。
    待つ投稿が読み込み済みの投稿とつながらないとき（再接続で 1 ページ分の新着が見つかった、または 200 件を超えた）は、押したときに最初のページだけを取り直す。
    タイムラインは、ウィンドウのフォーカスやオンライン復帰では取り直さない（取り直すと先頭に投稿が差し込まれる）。
  - `notification` は読み取り位置（`/notifications/marker`）の未読数を 1 つ増やし、通知の一覧を古いことにする。表示中の一覧は変わらない。
- **ログアウト**（設定画面の「ログアウト」）: アクセストークンを `POST /oauth/revoke` で失効させ（サーバーはトークン系列ごと失効させ、Cookie を消し、その系列を認可した認可サーバーの画面のセッションも終わらせる）、メモリのトークンを消して他のタブに伝え、`/login` を開く。
  サーバーに届かなかったときも、このタブはログアウトし、セッションが残り得ることをトーストで知らせる。
  画面のセッションが終わるので、続けて「ログインに進む」を押すとパスワードを聞かれる（共用端末で次の人がそのままログインできない）。
- **サインインが要る画面**: ルートの `beforeLoad` に `lib/auth/requireSignIn.ts` の `requireSignIn` を置く。セッションの再開を待ち、サインインしていなければ、戻り先をその画面にして `/login` に送る（TanStack Router の Authenticated Routes）。
  画面を守るのは API で、これはサインインを先に促すだけである。
- **テスト**: `src/test/msw/http.ts` が OAuth のエンドポイント（既定ではリフレッシュ Cookie がない状態）の既定のハンドラを持ち、API の操作の既定のハンドラ（Bearer が必要）は機能ごとに置く（§7）。
  `renderApp` は main.tsx と同じくセッションを再開し、ページの外への遷移（認可画面）を `leftTo` に記録する（jsdom は別の文書を読み込めない）。
  `GET /streaming` の既定のハンドラは、何も送らずにすぐ終わるストリームを返す（MSW は Node.js の `fetch` をソケットで横取りするので、テストの終わりに中止された流れ続ける応答があると、後のテストの要求が本物のネットワークに出ることがある）。イベントを送るテストは `streamingServer()` の `current().send()` / `close()` を使う。
  jsdom には Web Locks がないので、タブ間の直列化のテストは `src/test/locks.ts` の代わりの実装を 2 つの `Session` に共有させる。BroadcastChannel は Node.js のものを使う。

## 11. PWA

Service Worker は `app/web/sw/sw.ts` にあり、`vite-plugin-pwa` の `injectManifest` が Workbox のモジュールと一緒に `dist/sw.js` にビルドする（ADR-0028）。
Web App Manifest と HTML は Go が返すので、プラグインはどちらも作らず、登録のスクリプトも差し込まない。

- **キャッシュの規則**（`sw/routing.ts` の `classify`。単体テストは `sw/routing.test.ts`）
  - **ビルドの成果物**: ハッシュ付きのアセット・アイコンを事前にキャッシュする。Vite の `index.html`（`vite preview` 用の代わりの外枠）とサーバーが描画する画面の CSS は含めない。
  - **汎用の外枠**: `/` をビルドの版から作った revision で事前にキャッシュし、オフラインの外枠にする。外枠は訪問者ごとに変わらない（ADR-0028）。
  - **遷移**: ネットワークだけに問い合わせる（3 秒でタイムアウト）。失敗かタイムアウトのときだけ、事前にキャッシュした汎用の外枠を返す。遷移の応答は保存しないので、ある URL の外枠（OGP・404・`noindex`）を別の URL に返すことはない。
  - **Service Worker が応えないパス**: `/api`・`/oauth`・`/auth`・`/.well-known`・`/nodeinfo`・`/ap`・`/media`・`/uploads`（サーバーの予約パス）と `/manifest.webmanifest` への遷移。
    サーバーが描画する画面（§8）はネットワークが要り、SPA の外枠に置き換えてはいけない。
  - **クライアント API**: `/api` への要求には応えない（ネットワークだけになり、何もキャッシュしない）。`Authorization` 付きの要求はどのパスでもキャッシュしない。
  - **公開メディア**: 同じオリジンの `/media/` の画像だけを、キャッシュ優先（最大 300 件、30 日、200 の応答だけ）にする。動画は範囲要求の部分応答を再生できないので対象にしない。
- **登録と更新**（`src/lib/pwa.ts`）: ビルドしたアプリだけが `/sw.js` をスコープ `/` で登録する。外枠の CSP が Trusted Types を要求するので、URL は `/sw.js` だけを通すポリシーで作る。
  - 最初の Service Worker は有効になるとすぐページを制御する（`clientsClaim`）。新しい版は待機し、`UpdatePrompt` が「新しい版があります」のトースト（`timeout: null`、「更新する」の操作付き）を出す。
    利用者が押したときだけ `SKIP_WAITING` を送り、新しい版がページを制御したら読み直す。ページが表示に戻るたびに更新を確かめる。
  - デプロイで古いチャンクが消えたとき（`vite:preloadError`）は読み直す。10 秒以内に繰り返したら読み直さず、エラー境界に任せる。
- **ログアウト**: セッションが終わると、`lib/auth/userData.ts` が `clearCaches()` を呼ぶ（§10）。事前のキャッシュも含めてすべてのキャッシュと、メディアの期限のメタデータを消す（ADR-0028）。
  消したあともオンラインでは同じように動き、新しい版の Service Worker が事前にキャッシュし直すまでオフラインの外枠は使えない。
- **`my serve` の配信**: `sw.js` は長期キャッシュせず（`no-cache`）、Service Worker 自身に効く CSP（`default-src 'none'; connect-src 'self'`）で返す。既定の `default-src 'none'` のままだと Service Worker の `fetch` が止まる。
- **インストール**: マニフェストのアイコンは `icon.svg`・192 / 512 px の PNG・512 px の maskable の PNG、外枠は iOS 用に `apple-touch-icon.png`（180 px）を指す。
  PNG は SVG から `scripts/web-icons.sh`（ImageMagick）で作り、コミットする。インストールの条件は E2E が Chrome の `Page.getInstallabilityErrors` で検査する。

## 12. 投稿フォーム

`features/compose` が新しい投稿と返信のフォームを受け持つ（FR-101〜105、FR-110）。

- **置き場所**: 外枠が `ComposeProvider` に `panel`（`xl` 以上の 3 列目）か `sheet`（浮かぶボタンと `Sheet`）を渡し、`ComposeEntry` がその場所に描画する。
  シートは、フォームに要る `GET /instance` と `GET /preferences` の応答が届いて（または失敗して）から開き、開いたときに本文へ直接フォーカスを移す。
  読み込み中に開くとダイアログ自身にフォーカスが移り、React Aria がその 500 ms 後に（iOS の VoiceOver への対策として）フォーカスが body にあればダイアログへ戻すので、
  閉じた直後でトリガーへフォーカスを戻す途中のポップオーバーからフォーカスを奪う。
  ほかの機能の返信ボタンは `useReply()` の関数に投稿を渡す。フォームが返信先を示し、返信先の作者とメンション（閲覧者を除く）を本文の先頭に入れ、シートを開くか本文にフォーカスを移す。サインインしていない間はどちらも出さない（`useReply()` は `null`）。
- **下書き**（`hooks/draftStore.ts`）: フォームの外のストア（`useSyncExternalStore`）に置き、パネル・シート・返信ボタンが同じ下書きを共有する。
  リロードやタブの異常終了で失わないように、アカウントごとのキー（`my:user:compose:<アカウント ID>`）で sessionStorage に保存する。このタブだけのもので、タブを閉じるかセッションが終わると消える（§10）。
  アップロードの途中のファイルは保存しない（再開できない）。アップロード済みの画像は ID と説明を保存し、リロード後はサーバーのサムネイルを表示する。
- **既定値**: 公開範囲と言語は、利用者が選ぶまで `GET /preferences` の既定値に従う（state にコピーせず、表示のたびに求める）。返信の公開範囲の既定は、既定値と返信先のうち狭いほう（ADR-0017。`local` への返信は `local`）。
  公開範囲は 5 段階を説明付きのラジオグループ（ポップオーバーの中）で選ぶ。
  制限で公開範囲に上限があるとき（data-model.md §5.4）は、閲覧者の `CredentialAccount` の `posting`（サーバーがドメインの規則から求めたもの）の `visibilities` にない選択肢を無効にし、ラジオグループの説明で上限を示す。
  上限より広い選択（制限の前に選んだもの、既定値）は、上限（`visibility_ceiling`）として表示して送る。上限は投稿の間にも変わり得るので、結果の公開範囲が送ったものより狭いときは、投稿の後にトーストで知らせる。
  `local` の投稿がほかのサーバーのアカウントにメンションするときは警告する（ADR-0017）。本文が 400 ms 変わらなくなってから、ドメイン付きのメンションを `GET /accounts/lookup`（`resolve=true`。サーバーが投稿のメンションを解決するのと同じく、未知のハンドルは WebFinger で解決する。data-model.md §4.4）で引き、
  見つかったほかのサーバーのアカウント（`acct` にドメインがある）を `GET /accounts/relationships` で調べ、閲覧者をフォローしていない相手には投稿が届かないこと、ほかのサーバーのフォロワーにはそのメンションがメンションではなくただのリンクとして見えること（ADR-0035）を本文の下に示す。
  解決できないハンドル（404）はサーバーもテキストのままにするので示さない。解決の上限に達した（429）ときも示さない。
  言語の選択肢の名前は `Intl.DisplayNames` で表示中のロケールにする（カタログに言語名を持たない）。
- **文字数**: `GET /instance` の `limits.max_post_characters` から CW の文字数を引いたものを本文の上限にし、`TextArea` の文字数カウンタで示す。数え方はサーバーと同じコードポイント。上限を超えると送信できない。
- **画像**: 1 投稿 4 枚まで、JPEG・PNG・GIF・WebP。選んだファイルは `prepareImage` が `createImageBitmap`（EXIF の向きを適用）と `OffscreenCanvas` で 3840 × 2160 ピクセルの面積以内に縮め、描き直して EXIF などのメタデータを捨ててから送る。
  GIF はアニメーションを保つためそのまま送る（GIF に EXIF はなく、残りはサーバーが除く）。`POST /media`（202）の後は、`GET /media/{id}` の query が `processing` の間だけポーリングする。
  どの画像にも代替テキストの入力欄を必ず出し（Y9）、本文と一緒に `POST /statuses` の `media[].description` で送る。`require_media_description` が真で説明のない画像があるときは、送る前に `alertdialog` で確かめる。
- **送信**: 送信は下書きを「送信中」の一覧（outbox）に移してフォームを空にし、一覧が送った内容を表示する（S8）。成功したら一覧から消し、トーストで知らせ、タイムラインの query を無効にする。
  失敗したら、フォームが空ならそこへ下書きを戻し、空でなければ一覧に残して「もう一度送る」「編集する」「破棄する」を出す。一覧も保存するので、送信中にリロードしても失わない。
  1 つの下書きの試行はすべて同じ `Idempotency-Key` を送り、応答が届かなかった再送で二重に投稿しない。下書きを編集するとキーを作り直す。
- **テスト**: `features/compose/compose.test.tsx` がフォームの流れを MSW で検査する。jsdom には `matchMedia`・`createImageBitmap`・`OffscreenCanvas` がないので、必要なテストが差し替える。
  jsdom の `FormData` と `Blob` は Node.js の fetch が本文にできないので、アップロードのテストは Node.js のものに差し替える。
  E2E（`e2e/compose.spec.ts`）は、Playwright の要求の差し替えでクライアント API とトークンの更新に答え、サインイン済みの状態で axe・横スクロール・ARIA スナップショット・送信の失敗と再送を検査する。

## 13. 投稿の詳細と投稿への操作

`features/status` が投稿のカード（`StatusCard`）と操作、投稿の詳細の画面を受け持つ（FR-106〜108、FR-113）。タイムラインなどの一覧は、このカードを `index.ts` から使う。

- **アドレス**: 投稿の詳細は `/@{acct}/{id}`（`routes/@{$acct}_.$id.tsx`）。`acct` はローカルのアカウントではユーザー名、リモートでは `username@domain` で、読みやすさのためだけにあり、投稿は ID で引く。
  `_` で、プロフィールの画面（`/@{acct}`）のレイアウトの下に入らないようにする。カードの時刻はこのアドレスへのリンクになる（ブーストはブーストされた投稿へ）。
- **画面**: 見出しは `h1`「投稿」、その下に返信先の投稿（`h2`「スレッドの前の投稿」、古い順）、投稿そのもの（日時を省略せずに出し、自分へのリンクはない）、返信（`h2`「返信」、深さ優先）を並べる。
  ローダーは投稿を待ち、スレッド（`GET /statuses/{id}/context`）は待たずに読み込み始める。スレッドの失敗は返信の区画に理由と「もう一度試す」を出し、投稿は表示したままにする（E1）。
  見えない投稿と存在しない投稿はどちらも 404 で、ルートの `errorComponent` が「投稿が見つかりません」を出す。
- **操作**: 返信（`useReply()`、§12）、ブースト・お気に入り・ブックマークのトグル（`@my/ui` の `ToggleButton`。名前は状態で変えず `aria-pressed` で示し、数は説明にする。オンのアイコンは塗りつぶしで、色だけに頼らない）、
  「その他の操作」のメニューの「リンクをコピー」と、自分の投稿だけの「削除」。サインインしていない間は操作を出さない。公開範囲がブーストを許さない投稿（他人のフォロワー限定・ダイレクト）は、ブーストのボタンを無効にする。
- **ほかのサーバーのアカウントと投稿**（`acct` にドメインがあるもの）: 名前の横のハンドルは `@username@domain` のまま出す。カードの操作の列の末尾に、元のサーバーの投稿のページ（`url`、なければ `uri`）へのリンクを置く。
  リンクはホスト名だけを見せ、アクセシブルネームはそのホスト名を含む文（「{host} で元の投稿を開く」）にする。サインインしていない間も出す。
  プロフィールはハンドルの下に、元のサーバーのプロフィール（`Account.url`）へのリンクを置く。どちらのリンクも外部へのリンクのアイコン（`@my/ui` の `ExternalLinkIcon`）を付け、`rel="noreferrer"` で開く。
  画像は `preview_url` で表示する。アップロードではサムネイル、リモートの添付ではメディアプロキシの URL で、リモートの添付は `variants` を持たない。説明のない画像には「説明なし」のバッジが付く（Y9）。CW はローカルの投稿と同じ disclosure で出す。
- **楽観的更新**（S8）: 同じ投稿がタイムラインのページ・ブースト・スレッド・投稿の query に同時にあるので、`api/cache.ts` がキャッシュの答えを形（`Status`・`items`・`pages`・`ancestors` / `descendants`）で見分け、すべての写しを書き換える。
  トグルは押した時点で状態と数を変え、失敗したらその 1 回分だけ戻してトーストで理由を知らせる。同じ投稿の同じ操作は TanStack Query の mutation の `scope` で 1 つずつ順に送り、最後に押したものが残る。
  成功の応答の数（他の人の操作を含む）は、その操作の最後の要求の応答からだけ取り込む。結果は `useAnnouncer` で知らせる。
- **削除**: `alertdialog` で確かめ（最初のフォーカスは「キャンセル」）、サーバーが消してから、投稿とそのブーストをすべての一覧から除き、投稿とスレッドの query を消し、返信先の返信数を 1 減らす。
  詳細の画面で消したときは前の画面に戻る（戻る先がなければホーム）。失敗したらダイアログを残してトーストで理由を知らせる。
- **テスト**: `features/status/status.test.tsx` が画面・楽観的更新と失敗時の巻き戻し・順に送ること・複数の写しへの反映・削除を MSW で検査し、`api/cache.test.ts` がキャッシュの書き換えを単体で検査する。
  E2E（`e2e/status.spec.ts`）は、サインイン済みの状態でスレッドの ARIA スナップショット・axe・横スクロール・失敗したお気に入りの巻き戻し・メニューと削除の確認を検査する。

## 14. 管理画面

`features/admin` が、運用者の画面（連合先のサーバーの状態・ドメインへの処分・dead job）を受け持つ（[federation.md §7.3](federation.md)、[client-api.md](client-api.md) の管理の操作）。

- **出し分け**: 閲覧者の `CredentialAccount.permissions` で、設定の画面の「管理」の区画と各画面を出す。`manage_domains` が連合先とドメインへの処分、`manage_jobs` が dead job の画面に要る。
  ルートは `requireSignIn` のあと、loader で閲覧者を読み、権限があるときだけ一覧を読み込む（`routes/-admin/adminRoute.tsx`）。権限がなければ、要求を送らずに理由（権限の名前）を出す。
  守るのはサーバーで、権限が途中で外れて 403 `missing-permission` が返ったときは、ルートの `errorComponent` が同じ理由を出す。
- **連合先**（`/admin/instances`）: 状態のタブ（`?state=unavailable` / `available`、URL に置く。S3）と、ドメイン順の一覧。各サーバーは見出し（ドメインへのリンク）と、状態・その状態になった日時・失敗した日数・最後に失敗した日（UTC の日付）の説明リストにする。状態は文言で示し、色はそれを繰り返すだけにする。
- **連合先の詳細**（`/admin/instances/{domain}`）: 状態と、最新の `instance_availability_events` の理由、最後に配送を再開した日時（最新の `marked_available`）、新しい順の履歴。
  データモデルは成功した配送の日時を持たない（`instance_health` は失敗の間だけ更新する。[data-model §4.8](data-model.md#48-federation)）ので、「最後の成功」は配送の再開の日時で示す。
  失敗の日数があるか停止中なら「配送を再開する」を出し、成功したら詳細の答えを置き換え、一覧と履歴を無効にして結果を知らせる。失敗はトーストで知らせ、状態とボタンを残す（E2）。
  「モデレーション」の区画から、そのドメインのドメインへの処分の画面（`/admin/domain-actions?domain=…`）へリンクする。
- **ドメインへの処分**（`/admin/domain-actions`、FR-702）: 新しい処分のフォームと、新しい順の処分の一覧の 2 つの区画。一覧は「有効」（`active=true`）と「すべて」のタブ（`?filter=all`、URL に置く。S3）で、
  `?domain=` があればそのドメインの処分だけを出し、フォームのドメインにも入れ、「すべてのドメインを表示」のリンクで外せる。各処分は見出し（ドメイン）と、種類・状態・処分した日時・終了（なければ「解除するまで」）・解除した日時・公開の一覧での伏せ字の説明リスト、公開コメントと非公開のメモにする。
  - **ドメインの 2 つの形**: サーバーはドメインを IDNA の A-label（`xn--r8jz45g.jp`）で記録して返す。画面は `lib/idn.ts`（RFC 3492 の Punycode の復号。表示専用）で U-label（`例え.jp`）に戻し、違うときは A-label を括弧で後に添える（`features/admin` の `DomainName`、連合先の一覧と詳細の見出しも同じ）。
    似た字形の別の文字のドメインを見分けられるようにするためである。文の中では U+2066 / U+2069 で囲む。フォームは U-label のまま送り、A-label への変換はサーバーが行う。
  - **フォーム**: ドメイン（必須）、種類（`RadioGroup`、必須、既定の選択なし、軽いものから `suspend` まで、それぞれの効果の説明付き）、期間（`DurationSelect`、1・7・30・90・180・365 日と「解除するまで」）、公開コメントと非公開のメモ（`ReasonField`、5,000 文字）、伏せ字（`Checkbox`）。
    ブラウザの制約検証が通ってから送る。`suspend` だけは送る前に `ConfirmDialog` で確かめ（NFR-MOD-7）、失敗はダイアログの中に出す。ほかの種類はすぐに送り、失敗はフォームの下（`role="alert"`）に出して入力を残す。
    成功したらドメインを残して入力を空にし、一覧を無効にして、結果をフォームの `StatusMessage` に出す。
  - **解除**: 有効な処分の「解除」（アクセシブルネームに種類・ドメイン・処分した日時を含める）は確かめずに送る。成功したら、キャッシュの「有効」の一覧から除いてフォーカスを次の行のボタンに移し（`lib/useFocusAfterRemoval.ts`）、
    「すべて」の一覧では解除済みに置き換えてフォーカスをその行の見出しに移す。結果は一覧の区画の `StatusMessage` に出し、失敗はトーストで知らせる（E2）。
- **dead job**（`/admin/dead-jobs`）: 失敗した新しい順の一覧。各ジョブは型の見出し、失敗と作成の日時、試行回数、冪等キーと順序キー、最後のエラー、ペイロード（JSON の文字列のまま）を出す。型・キー・ペイロードは `dir="ltr"` で表示する。
  「再実行」はジョブの型と失敗の日時をアクセシブルネームに含める。成功したジョブはキャッシュの一覧から除き、フォーカスを次の行のボタン（なければ見出し）に移す（`lib/useFocusAfterRemoval.ts`）。
  複数のジョブを同時に再実行できるので、成功後の処理は `mutate` ではなく `useMutation` 自身のコールバックに書き、送信中のジョブは `useMutationState` で求める。
- **共通の部品**: 管理画面の一覧と詳細は同じ型で組む。権限の出し分けは `RequirePermission`、記録の一覧（区切り線の行・空の状態・「さらに読み込む」、行の `data-row-id`）は `RecordList`、詳細の画面の区画（`h2` で名前を付けた `section`）は `DetailSection` にし（`features/admin/components/parts.tsx`）、
  記録の項目と値は `@my/ui` の `DescriptionList` / `DescriptionItem` にする。
- **取り消せない操作**（BAN・ドメインの凍結・投稿の削除。NFR-MOD-7）: `@my/ui` の `ConfirmDialog`（APG の alertdialog）を挟む。理由は `ReasonField`、期限は `DurationSelect` をダイアログの中に置く。
  開くと最初の入力欄（なければ「キャンセル」）にフォーカスが移り、Esc で閉じ、閉じると開いたボタンにフォーカスが戻る。失敗はダイアログの中（`errorMessage`）に出して開いたままにし、成功したら閉じて、結果を画面の `StatusMessage`（`role="status"`）に出す。
  操作で開いたボタンが消えるとき（一覧から行が消えるなど）は、`lib/useFocusAfterRemoval.ts` でフォーカスの行き先を決める。
- **テスト**: `features/admin/admin.test.tsx` が出し分け・一覧と絞り込み・ページの読み足し・再開・再実行・失敗・403 を MSW で検査する。既定の閲覧者は権限を持たないので、テストは `operatorHandler()` で権限を与える。
  `features/admin/domainActions.test.tsx` は、IDN のドメインの処分（A-label で記録され、両方の形で出る）、必須の入力、`suspend` の確認と取り消し、ダイアログの中とフォームの下の失敗、解除とフォーカス、連合先の詳細からのリンクとドメインの絞り込みを検査する。
  E2E（`e2e/admin.spec.ts`、`e2e/domain-actions.spec.ts`）は、権限のある閲覧者として各画面の axe・横スクロール・ARIA スナップショット・配送の再開・再実行・`suspend` の確認のダイアログ・解除とフォーカスの移動を検査する。

## 15. 検索とほかのサーバーのアカウントのフォロー

`features/search` が、ハンドルか URL でアカウントを探す画面を受け持つ（FR-305）。フォローのボタンとフォローの一覧は `features/profile`（§13 の外のプロフィールの画面）にある。

- **画面**（`/search`、ナビゲーションの「検索」）: 見出しが名前を与える `role="search"` のフォームに、ラベル付きの入力欄（`type="search"`）と送信ボタンを置く。
  入力欄は `@username@domain`・`username@domain`・アクターかプロフィールの `https` の URL を受け付け、前後の空白を除いて `GET /accounts/lookup` に送る。
  サインイン中は `resolve=true` を付け、サーバーが知らないアカウントを相手のサーバーから取ってこさせる。サインインしていない間は付けず（解決にはトークンが要る）、このサーバーが知っているアカウントだけが見つかることを説明に書き、サインインのリンクを出す。
- **送信**: 解決は相手のサーバーに届き、回数に制限がある（30 回 / 5 分）ので、画面を開いたときに走る query ではなく、利用者が送る mutation にする（R7）。
  見つかったアカウントをプロフィールの query のキャッシュに入れ、検索語を `?q=` にして今の履歴を置き換えてから、プロフィール（`/@{acct}`）に移る。戻ると、同じ検索語が入った画面に戻る（S3）。
  見つからない（404）・制限された（429）ときはクライアントのカタログの文言、それ以外はサーバーの理由を、入力欄のエラー（`aria-invalid` と `aria-describedby`）に出し、`useAnnouncer` で知らせる。フォーカスは入力欄に残る。エラーは、それを起こした検索語のあいだだけ出し、入力を変えると消える。
- **フォローの承認待ち**: ほかのサーバーのアカウントのフォローは、相手のサーバーが `Accept` を返すまで `requested` のままになる（[federation.md](federation.md)）。
  フォローのボタンは「リクエスト中」になり、その下に、承認を待っているサーバー（`Account.domain`）を示す文を出して、ボタンの説明（`aria-describedby`）にする。押すと、ローカルのリクエストと同じく取り消しを確かめる。
- **フォローの一覧**: ほかのサーバーのアカウントも、ローカルのアカウントと同じ行に `@username@domain` のハンドルで並び、そのプロフィール（`/@username@domain`）へリンクする。
- **テスト**: `features/search/search.test.tsx` が、キーボードでの検索からプロフィールへの移動、URL での検索、404・429・その他の失敗、サインインしていないときの検索、承認待ちのフォローと取り消し、フォローの一覧のリモートのアカウントを MSW で検査する。
  E2E（`e2e/search.spec.ts`）は、キーボードだけで検索からフォローまでを通し、承認待ちの表示・axe・横スクロール・ARIA スナップショットを検査する。

## 16. 見た目・状態・文言

画面ごとに表現がばらつかないように、間隔・文字・アイコン・状態の表示・文言を 1 つの型にそろえる。
トークンと `@my/ui` の部品の側の決まり（値の一覧、部品の props）は [app/ui/README.md](../../app/ui/README.md) にあり、ここは画面を組み立てる側の規約である。
主な出典は WCAG 2.2、WAI-ARIA APG、[GOV.UK Design System](https://design-system.service.gov.uk/)（エラーと空の状態の書き方）、[Material Design 3 の Typography](https://m3.material.io/styles/typography/applying-type)（文字の役割の考え方）。

### 16.1 間隔

| # | 規約 | 強制 |
| --- | --- | --- |
| G1 | 間隔は Tailwind の 0.25rem 刻みのうち、`0.5`・`1`・`2`・`3`・`4`・`6`・`8`・`10` だけを使う。役割ごとの値は README の「Spacing」の表に従い、同じ役割に別の値を使わない | レビュー |
| G2 | 画面の左右の余白（ガター）は `px-4`（16 px）にし、外枠（`AppLayout`）だけが付ける。画面と機能の部品は自分でガターを足さない。一覧の行とカードは、行の中の余白（`px-4 py-3`）だけを持つ | レビュー |
| G3 | 一覧の項目の区切りは `divide-y divide-border` の線にし、項目のあいだに `gap` を足さない（タイムライン・通知・アカウントの一覧で同じ見た目にする） | レビュー |
| G4 | 画面の端に浮かぶ要素（投稿のボタン、`NewItemsButton`、トースト）が一覧の最後の項目や操作を隠さないように、外枠がその高さぶんの余白を `main` の終わりに取る。浮かぶ要素は、それが役に立つ画面（タイムラインとプロフィール）にだけ出す | E2E のスクリーンショット、レビュー |

### 16.2 文字

文字の大きさは役割で選び、画面ごとに決めない。役割と大きさの対応は README の「Type roles」の表にある。

| # | 規約 | 強制 |
| --- | --- | --- |
| G5 | 画面の見出しは `PageHeading`（`h1`）、区画の見出しは `h2` の役割（`text-lg font-semibold`）にする。見出しの大きさを部品ごとに変えない。見た目のためだけに見出しの段を飛ばさない | レビュー、ARIA スナップショット |
| G6 | 補足の情報（ハンドル・時刻・件数・説明）は `text-sm text-fg-muted`、バッジとラベルは `text-xs` にする。`text-fg-muted` を本文や操作の名前に使わない | レビュー |
| G7 | 1 行に収める表示名とハンドルは `truncate`（`min-w-0` の親）で省略し、`title` ではなくアクセシブルネームで全体を読ませる。本文・プロフィール・URL のような折り返してよいものは `wrap-anywhere` にする。長いハンドル（`@user@長いドメイン`）でカードの頭が 3 行以上にならないようにする | E2E（`ar-XB` と長いドメインのフィクスチャで 320 px）、レビュー |
| G8 | 数（件数・残りの文字数）は `tabular-nums` にし、`Intl.NumberFormat`（`react-intl`）で書く。時刻は一覧では相対（「3 分前」）、詳細の画面と通知の一覧でも同じ規則にし、絶対の日時は `<time>` の `dateTime` と説明（ツールチップか詳細の画面）で示す | レビュー |
| G9 | 利用者が書いた名前を UI の文の中や行の中に置くときは、要素ごとに `dir="auto"` を付けるだけでなく、行の揃え（`text-start`）を親の向きに合わせる。`dir="auto"` の要素がブロックのときは `text-align: match-parent` にして、RTL の名前だけ右に寄らないようにする | ARIA スナップショット、`ar-XB` のスクリーンショット |

### 16.3 アイコン

| # | 規約 | 強制 |
| --- | --- | --- |
| G10 | アイコンは `@my/ui` のアイコン集から使う。機能やルートに SVG のパスを書かない。アイコン集は 24 × 24 の格子、線幅 2、角は丸めて描き、新しいアイコンも同じ形で足す（ライブラリを足すときは ADR-0031 の許可リストの手順に従う） | レビュー、`knip` |
| G11 | アイコンの大きさは置き場所で決める: 文と並ぶものは `size-4`、ボタンの中は `size-5`、ナビゲーションは `size-6`。`Button`・`ToggleButton`・`Link` が子の SVG の大きさを決め、アイコンの側では大きさを指定しない | 部品のテスト（ボタンの中の SVG が 0 × 0 でない）、スクリーンショット |
| G12 | アイコンは飾り（`aria-hidden="true"`、`focusable="false"`）にし、意味は見える文言かアクセシブルネームで伝える。アイコンだけのボタンには `aria-label` と、同じ文言の `Tooltip` を付ける | `@my/ui` の部品のテスト、レビュー |
| G13 | オン / オフはアイコンの形（塗りつぶし・線の太さ）でも示し、色だけに頼らない（WCAG 1.4.1）。向きのあるアイコン（返信の矢印、戻る）は `rtl:-scale-x-100` で反転し、時計回りの意味を持つもの（再読み込み）は反転しない | スクリーンショット（`ar-XB`）、レビュー |

### 16.4 空・読み込み中・エラー・オフライン

どの画面も、データの区画は次の 4 つの状態を同じ部品と同じ文の型で出す。部品は `@my/ui` にあり、機能はフックの状態（S5 の判別可能なユニオン）からどれを出すかを選ぶだけにする。

| # | 規約 | 強制 |
| --- | --- | --- |
| G14 | **空**は `EmptyState` にする。見出しは事実を短く（「まだ投稿はありません」）、説明はここに何が出るか、操作は次にできること（「ローカルのタイムラインを見る」）にする。絞り込みで空になったときは、絞り込みを外す操作を出す | レビュー |
| G15 | **読み込み中**は、最初の 1 ページを待つ一覧ではスケルトン（`Skeleton`、見せる項目の形に合わせる）、ボタンや小さな区画ではその場の `Spinner` にし、同じ区画に両方を出さない。読み込みの名前は `aria-busy` と区画のラベル（`role="status"` の `Spinner` か視覚的に隠した文）で 1 回だけ伝える。`defaultPendingMs` より早く終わる読み込みには何も出さない | レビュー、ARIA スナップショット |
| G16 | **エラー**は、何が起きたか・理由（`errorMessage(error)`）・次にできること（「もう一度試す」）の 3 つを出す。画面の読み込みの失敗は、その画面の `h1` を残したまま本文の位置に出し、`document.title` も画面の名前のままにする。一覧の続きや区画の失敗は、その区画の中に出して、読み込み済みの内容を消さない。利用者の操作の失敗はトースト（E2）、入力の誤りはその入力欄のエラー（`aria-invalid`・`aria-describedby`）にする。`role="alert"` は利用者の操作の結果にだけ使い、画面を開いただけの失敗には使わない | レビュー |
| G17 | **オフライン**は外枠が 1 か所で扱う。`navigator.onLine` と通信の失敗（`NetworkError`）からオフラインと判断し、画面の上端に「オフライン」の帯（`role="status"`）を出す。オフラインの間は送信する操作を無効にせず、失敗したら下書きと outbox（§12）に残す。オンラインに戻ったら帯を消し、失敗していた区画だけを取り直す（タイムラインは取り直さない。§10）。仕組みは §9 の「オフライン」 | E2E（`context.setOffline`。`e2e/offline.spec.ts`・`e2e/pwa.spec.ts`）、統合テスト（`src/offline.test.tsx`）、レビュー |
| G18 | 4 つの状態の文言は、同じ種類のものを同じ型にする（「<対象>を読み込めませんでした：{reason}」「もう一度試す」）。新しい区画を作るときは、既存のメッセージ ID の型（`<feature>.empty.*`・`<feature>.loading`・`<feature>.failed`・`<feature>.retry`）に合わせる | レビュー |

### 16.5 文言のトーン

| # | 規約 | 強制 |
| --- | --- | --- |
| G19 | 文は利用者に向けて、短く、平らに書く。感嘆符・絵文字・冗談・責める言い方（「無効な入力です」）を使わない。エラーは利用者が直せることを書く（GOV.UK の [Error message](https://design-system.service.gov.uk/components/error-message/)） | レビュー |
| G20 | 英語は sentence case（`Write a post`、`Follow requests`）、ボタンと項目は動詞で始め、`Click` / `Tap` のような操作の手段を書かない。日本語は です・ます 調、ボタンと項目は「〜する」か名詞で終え（「投稿する」「フォロー」）、文の中の区切りは全角の記号（「：」「、」）にする | `make i18n-check`（カタログの lint）、レビュー |
| G21 | 同じものは同じ語で呼ぶ。用語（投稿・ブースト・お気に入り・ブックマーク・公開範囲・ローカル・連合・CW）は [vision](../requirements/00-vision.md) と既存のカタログの語に合わせ、画面ごとに言い換えない | レビュー |
| G22 | 確かめのダイアログは、見出しを問い（「この投稿を削除しますか？」）、本文を結果（取り消せるかどうか）、ボタンを動詞（「削除」「キャンセル」）にし、「はい / いいえ」にしない | レビュー |

## 17. 通報

`features/report` が、アカウントとその投稿をモデレーターに通報するダイアログを受け持つ（FR-701、FR-709）。投稿の「その他の操作」のメニュー（自分の投稿にはない）とプロフィールのメニューの「通報する」が開く。

- **段**: モーダルのダイアログ（APG の dialog パターン、`@my/ui` の `Dialog`）の中で、理由 → ルール（理由がルール違反のときだけ）→ 投稿 → コメントと転送の順に 1 段ずつ出し、送信したら結果の段にする。
  各段は見出し（`h3`。ダイアログの題が `h2`）が名前を与える `form` で、段の数（「手順 2 / 4」）を見出しの上に出す。「次へ」はブラウザの制約検証が通ってから進み、「戻る」は答えを残す。下書きはダイアログの中にだけあり、閉じると消える。
- **フォーカス**: 段が変わるたびに、その段の見出し（`tabindex="-1"`）にフォーカスを移す。見出しはマウントしたときに Effect で自分にフォーカスする（段ごとに `key` を変える）。
  子の Effect はダイアログの Effect より先に動くので、開いたときもダイアログ自身ではなく最初の段の見出しにフォーカスが残る。閉じると、開いたメニューのボタンにフォーカスが戻る。
- **理由**: `spam`・`legal`・`violation`・`other` を説明付きの `RadioGroup` で選ぶ（必須）。ルール違反は引用するルールが要るので、`GET /instance` の `rules` が空のときは出さない。ルールの段では 1 つ以上を選ぶまで進めず、`role="alert"` の文で知らせる。
- **投稿**: そのアカウントの最新のページ（`GET /accounts/{id}/statuses`、ブーストを除く）をチェックボックスの一覧にする。投稿から開いたときはその投稿を先頭に置いて選んでおき、プロフィールから開いたときは何も選ばない。
  チェックボックスの名前は投稿の日時で、本文（CW のある投稿は CW の文だけ）を `aria-describedby` の説明にする。本文の区画は `inert` にして、リンクを Tab の通り道にしない。投稿と引用するルールはそれぞれ 20 件まで（API の上限）。
- **コメントと転送**: コメントは任意で 1,000 文字まで（`TextArea` の文字数カウンタ）。ほかのサーバーのアカウントでは、そのサーバーにも名前を伏せて送る（`forward`）かを選ぶチェックボックスを出す（既定はオフ）。
- **送信**: 最後の段の「通報を送信する」が `POST /reports` を送る。失敗はダイアログの中に `role="alert"` で理由を出し、答えを残す（E2）。成功したら結果の段に、モデレーターが確認することと、通報者が相手に知らされないこと、転送したならその送り先を出す。
- **テスト**: `features/report/report.test.tsx` が、キーボードだけでの全段の操作と見出しへのフォーカス、理由とルールの必須、ルールのないサーバー、投稿の読み込みの失敗と再試行、送信の失敗、自分の投稿に出さないこと、プロフィールからのリモートのアカウントへの通報と転送を MSW で検査する。
  E2E（`e2e/report.spec.ts`）は、キーボードだけで投稿から通報を送り、各段の見出しへのフォーカス・axe・横スクロールと、投稿の段の ARIA スナップショットを検査する。
