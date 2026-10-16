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
| A1 | 依存の向きは `routes → features → (@my/ui, lib)` だけにする | `boundaries/dependencies`（要素は `shell`（`web/src` 直下）・`route`・`feature`・`lib`・`ui`、テストだけが import できる `test`（`web/src/test`））、`boundaries/no-unknown-files` |
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
| Y7 | 新しい投稿を自動で差し込まない。「N 件の新しい投稿」は `@my/ui` の `NewItemsButton`（`role="status"` の中のボタン）で知らせ、押されたときだけ差し込む。それ以外の件数・完了・エラーは、共通の `role="status"` / `role="alert"` の通知部品（`@my/ui` の `AnnouncerProvider` / `useAnnouncer`、見える通知は `ToastProvider` / `useToast`）だけで知らせる。フォーカスを奪わない | ARIA スナップショット、部品のテスト（Announcer・NewItemsButton のテストとストーリー）、レビュー |
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
  - 部品は `src/test/render.tsx` の `renderWithProviders`（ランタイム設定と i18n のプロバイダ）で描画する。`user-event` は描画の前に `setup()` する。
  - 統合テストは同じファイルの `renderApp` でアプリ全体（本物のルーターをメモリ上の履歴で、新しい QueryClient）を描画する。`path` で開くアドレス、`locale` でロケールを指定する。
    サインインした状態は `server.use(signedInHandler)`（リフレッシュがハンドラの受け付けるアクセストークンを返す）で作る。
  - jsdom の `FormData` と `File` の本文は、MSW が相手にする Node.js の `fetch` で送れない。ファイルを送るテストは `src/test/multipart.ts` の `nodeUploadFile()` で
    Node.js の `File` を作り、そのテストの間だけ `FormData` を Node.js のものにする。
  - `@my/ui` の部品のテストも `app/ui/src` の部品の隣に置き、`make web-test` で動かす（`app/ui/vitest.config.ts` の `unit` プロジェクト、jsdom）。
    `app/ui/test/setup.ts` が jest-dom のマッチャーを読み込む。キットは通信しないので MSW もプロバイダもなく、Testing Library の `render` で描画する。
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
  - サーバーのレート制限（登録は 1 つのアドレスから 30 分に 5 回、ログインの失敗は 1 分に 5 回）を使い切らないように、アカウントを作るテストは 1 つのプロジェクト（`en-320`）だけで動かし、
    エラーの表示は制限を数える前に断られる入力（期限切れのフォーム、形式の誤り）で確かめる。
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
- **プロバイダ**（`App.tsx`）: 外側から、ランタイム設定、表示のロケール、i18n（`react-intl` と React Aria の `I18nProvider`）、QueryClient（`lib/query`）、セッションと API クライアント（`lib/auth/SessionProvider`、§10）、`AnnouncerProvider` / `ToastProvider`、React Aria の `RouterProvider`、ルーターの順に置く。
- **文言**（`lib/i18n`）: クライアントの文言は `locales/<locale>/client.json` にあり、ロケールごとに別のチャンクで読み込む。足りない文言は `ja-JP` → `ja` → `en` の順に補う。
  ID の型は `tools/i18n` が生成する `MessageArgs` から作る（`ClientMessageKey`）。文字列にするときは `useT()` の `t("client:…", 引数)` を使い、ID と引数の型を検査する。
- **ルーティング**（`router.ts`）: TanStack Router のファイルルート。Vite のプラグインが `routes/` から `routeTree.gen.ts` を生成する（自動コード分割）。生成物はコミットし、`tsc` がビルドの前に使う。
  - ルーターは遷移を `startTransition` の中で確定するので、次の画面を読み込む間も今の画面が残る（E4）。読み込みが `defaultPendingMs` を超えたときだけ `RoutePending` を出す。
  - `-` で始まるファイルとディレクトリはルートにならない。外枠の部品は `routes/-layout/` に置く。
- **リンク**: 画面の間のリンクは `@my/ui` の `Link` に `href` を渡す。React Aria の `RouterProvider` が同じオリジンの `href` をルーターの遷移に変える。`#id` の `href`（本文へ移動するリンク）は、その要素にフォーカスを移す。
- **レイアウト**（`routes/-layout/AppLayout.tsx`）: モバイルファーストで、`md` 未満は下端のタブバー、`md` から横のレール、`lg` から文言付きのサイドバー、`xl` から投稿フォームとサーバーについての 3 列目を出す。
  `xl` 未満では、投稿フォームはタブバーの上（`md` からは画面の隅）に浮かぶボタンから開くシートになる（§12）。どちらにするかは外枠が `matchMedia` で決め（`useMediaQuery`）、ページに置くフォームは常に 1 つにする。
  ナビゲーションは 1 つの `nav` で、今いる区画のリンクに `aria-current="page"` を付ける。画面の端に接する要素は `env(safe-area-inset-*)` を足し、高さは `svh` で指定する。
  先頭に、フォーカスしたときだけ見える「本文へ移動」のリンクを置く。
- **フォーカスとタイトル**（`routes/-layout/useViewChange.ts`、Y5）: ルーターが画面を描画するたびに、`main` の `h1` の文字列から `document.title` を `client:shell.document_title`（画面名とサーバー名）で作る。
  パスが変わったときだけ、その `h1` にフォーカスを移す。最初の画面と、検索条件やハッシュだけの変化（タブ・絞り込み）ではフォーカスを動かさない。画面の見出しは `PageHeading`（`tabindex="-1"` の `h1`）で書く。
- **エラー**（E1）: 画面の失敗はルーターの `defaultErrorComponent`（`RouteError`）がレイアウトの中に出し、「もう一度試す」で Query のエラー境界を戻してから loader を再実行する。
  レイアウト自体の失敗はルートの `errorComponent`（`RootError`）が画面全体に出す。合うルートのないアドレスは `NotFound` を出す。独立した区画（3 列目など）は `react-error-boundary` で囲む。

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
    どちらも `message` がローカライズ済みで、そのまま表示できる。problem でない本文（プロキシの HTML など）は、クライアントのカタログの文言（`client:api.error.status`）にする。
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
- **ログアウト**（設定画面の「ログアウト」）: アクセストークンを `POST /oauth/revoke` で失効させ（サーバーはトークン系列ごと失効させ、Cookie を消し、その系列を認可した認可サーバーの画面のセッションも終わらせる）、メモリのトークンを消して他のタブに伝え、`/login` を開く。
  サーバーに届かなかったときも、このタブはログアウトし、セッションが残り得ることをトーストで知らせる。
  画面のセッションが終わるので、続けて「ログインに進む」を押すとパスワードを聞かれる（共用端末で次の人がそのままログインできない）。
- **サインインが要る画面**: ルートの `beforeLoad` に `lib/auth/requireSignIn.ts` の `requireSignIn` を置く。セッションの再開を待ち、サインインしていなければ、戻り先をその画面にして `/login` に送る（TanStack Router の Authenticated Routes）。
  画面を守るのは API で、これはサインインを先に促すだけである。
- **テスト**: `src/test/msw/handlers.ts` が OAuth のエンドポイント（既定ではリフレッシュ Cookie がない状態）と、API の操作（Bearer が必要）の既定のハンドラを持つ。
  `renderApp` は main.tsx と同じくセッションを再開し、ページの外への遷移（認可画面）を `leftTo` に記録する（jsdom は別の文書を読み込めない）。
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
  ほかの機能の返信ボタンは `useReply()` の関数に投稿を渡す。フォームが返信先を示し、返信先の作者とメンション（閲覧者を除く）を本文の先頭に入れ、シートを開くか本文にフォーカスを移す。サインインしていない間はどちらも出さない（`useReply()` は `null`）。
- **下書き**（`hooks/draftStore.ts`）: フォームの外のストア（`useSyncExternalStore`）に置き、パネル・シート・返信ボタンが同じ下書きを共有する。
  リロードやタブの異常終了で失わないように、アカウントごとのキー（`my:user:compose:<アカウント ID>`）で sessionStorage に保存する。このタブだけのもので、タブを閉じるかセッションが終わると消える（§10）。
  アップロードの途中のファイルは保存しない（再開できない）。アップロード済みの画像は ID と説明を保存し、リロード後はサーバーのサムネイルを表示する。
- **既定値**: 公開範囲と言語は、利用者が選ぶまで `GET /preferences` の既定値に従う（state にコピーせず、表示のたびに求める）。返信の公開範囲の既定は、既定値と返信先のうち狭いほう（ADR-0017。`local` への返信は `local`）。
  公開範囲は 5 段階を説明付きのラジオグループ（ポップオーバーの中）で選ぶ。
  制限で公開範囲に上限があるとき（data-model.md §5.4）は、閲覧者の `CredentialAccount` の `posting`（サーバーがドメインの規則から求めたもの）の `visibilities` にない選択肢を無効にし、ラジオグループの説明で上限を示す。
  上限より広い選択（制限の前に選んだもの、既定値）は、上限（`visibility_ceiling`）として表示して送る。上限は投稿の間にも変わり得るので、結果の公開範囲が送ったものより狭いときは、投稿の後にトーストで知らせる。
  `local` の投稿がほかのサーバーのアカウントにメンションするときは警告する（ADR-0017）。本文が 400 ms 変わらなくなってから、ドメイン付きのメンションを `GET /accounts/lookup` で引き、
  見つかったほかのサーバーのアカウント（`acct` にドメインがある）を `GET /accounts/relationships` で調べ、閲覧者をフォローしていない相手に投稿が届かないことを本文の下に示す。サーバーが知らないアカウントは、サーバーも解決しないので示さない。
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
