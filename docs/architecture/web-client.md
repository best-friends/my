# Web クライアントの設計規約

Web クライアント（`app/web`）と共有 UI キット（`app/ui`）を書くときの規約です。
根拠は [ADR-0025](../adr/0025-web-client-stack.md)（技術スタック）、[ADR-0026](../adr/0026-client-workspace-and-component-architecture.md)（構成と責務分離）、[ADR-0027](../adr/0027-ui-quality-gates.md)（品質ゲート）、[ADR-0028](../adr/0028-html-shell-link-previews-and-pwa.md)（HTML の外枠と PWA）です。
主な出典は react.dev、WAI-ARIA Authoring Practices、WCAG 2.2、Tailwind CSS・TanStack・Testing Library の公式ドキュメントです。

「強制」の列は、lint・型検査・テストで機械的に検査するものを示します。「レビュー」はレビューで確認します。
lint は Oxlint で、`make web-lint`（`make web-check` と `make check` の一部）で実行します（[ADR-0049](../adr/0049-client-lint-oxlint.md)）。
設定は `app/.oxlintrc.json` です。型情報の規則（`typescript/*`）は tsgolint が、React Compiler とフックの規則（`react/*`）は Oxlint が動かし、
ほかのプラグインの規則は Oxlint の JS プラグインとして動きます。独自の制限は、`app/.oxlintrc.json` がセレクタを渡す JS プラグイン `app/oxlint/restrictions.js` の規則
（`restrict/syntax`・`restrict/svg`・`restrict/feature`・`restrict/test`）と、Oxlint の `no-restricted-imports` などの規則の設定です。
`app/oxlint/restrictions.test.js` が各制限の違反例と許可例を、`app/oxlint/violations.test.js` が下の表の lint の行ごとの違反を Oxlint が報告することを検査します。
Oxlint は警告でも失敗します（`options.denyWarnings`）。
ルールを無効化するコメント（`eslint-disable-next-line`）には理由（`-- 理由`）が必要です（`@eslint-community/eslint-comments/require-description`）。

## 1. React の基本

| # | 規約 | 強制 |
| --- | --- | --- |
| R1 | コンポーネントとフックは純粋にする。レンダリング中に副作用・変更・`Date.now()` / `Math.random()` を書かない | `react/purity`、`react/globals` |
| R2 | props・state・フックに渡した値・JSX に渡した値を変更しない（`push` / `sort` ではなく `toSorted` / スプレッド） | `react/immutability`（代入を検出する。`push` / `sort` などのメソッド呼び出しはレビュー） |
| R3 | フックはコンポーネントか `use*` 関数のトップレベルでだけ呼ぶ | `react/rules-of-hooks` |
| R4 | React Compiler を全コードに適用する。コンパイラが最適化を諦めるコードは lint とビルドで失敗させる。コンパイラから外す（`"use no memo"`）のは、コンパイルできないライブラリを包む 1 つのフックだけにし、理由を書く（`@my/ui` の `Feed` が TanStack Virtual の仮想化を読むフック） | Oxlint の React Compiler の規則（`react/*`。`react/incompatible-library`・`react/unsupported-syntax` も error）、`vite build`（`app/web/vite-plugins/react-compiler-check.ts`。コンパイラが関数を 1 つでも諦めると、場所と理由を出して失敗する。`"use no memo"` で外した関数は除く。1 つも関数を変換しなくても失敗）、`restrict/syntax`（`"use no memo"` / `"use no forget"`。必要なときは理由付きの無効化コメント。API クライアントのフック `api.query.useQuery` / `api.query.useMutation` の呼び出し、§10） |
| R5 | `useMemo` / `useCallback` / `memo` は書かない。必要なとき（Effect の依存に安定した参照が要るなど）は理由をコメントに書く | `no-restricted-imports`・`restrict/syntax`（`useMemo` / `useCallback` / `memo` の使用。必要なときは理由付きの無効化コメント）、`react/use-memo` |
| R6 | 他の値から計算できるものは state にせず、レンダリング中に計算する | `react/set-state-in-effect`、`react/no-deriving-state-in-effects` |
| R7 | 利用者の操作で起きる処理（送信・通知・計測）はイベントハンドラに書く。Effect に書かない | レビュー |
| R8 | Effect は外部のシステム（SSE、ブラウザ API、React 以外の部品）と同期するときだけ使い、必ず後始末を返す | レビュー |
| R9 | Effect の中で反応させたくない値は `useEffectEvent` で読む。`exhaustive-deps` を無効化しない | `react/exhaustive-deps` を error、`@eslint-community/eslint-comments/no-restricted-disable`（`react-hooks/*` と、Oxlint の `react/` の React Compiler とフックの規則の無効化を禁止。`Feed` のフックで Oxlint が `"use no memo"` の中でも報告する `react/incompatible-library` だけは除く） |
| R10 | ID が変わったら state を初期化したい場合は `key` を使う。Effect で初期化しない | レビュー |
| R11 | コンポーネントやフックを、ほかのコンポーネントの中で定義しない | `react/static-components`、`@eslint-react/no-nested-component-definitions` |
| R12 | リストの `key` には安定した ID を使う。動的なリストに配列の添字を使わない | `@eslint-react/no-array-index-key`、`@eslint-react/no-missing-key` |
| R13 | `forwardRef`・`<Ctx.Provider>`・関数コンポーネントの `defaultProps` を使わない（`ref` は普通の props、`<Ctx value>`、引数の既定値） | `@eslint-react/no-forward-ref`、`@eslint-react/no-context-provider`、`restrict/syntax`（`defaultProps`） |
| R14 | 数値を `&&` の条件にしない（`{count && <X />}` は `0` を表示する）。`count > 0 &&` と書く | レビュー |

## 2. 状態とデータ

| # | 規約 | 強制 |
| --- | --- | --- |
| S1 | サーバーのデータは TanStack Query だけで扱う。取得は `features/<x>/api` の `queryOptions`、書き込みは `useMutation` で行う。成功時に、応答で確かに分かる変化（投稿・閲覧者・関係・設定）は `setQueryData` / `updateCachedStatuses` でキャッシュに書き、並びや件数が変わり得る一覧・件数には `invalidateQueries`（既定の `refetchType: "active"`）で無効化の印を付ける。表示していないクエリは、次にその画面を開いたときに loader が取り直して待つ（S7）ので、取り直さない。書き込みの成功表示は、表示中のクエリの取り直しを待ってから出す（`await invalidateQueries(...)`）。書き込みが変えるクエリは、その書き込みの関数の隣（`features/<x>/api` のキャッシュ更新関数。例: `cachePostedStatus`）に書き、部品の `onSuccess` はそれを呼ぶ。データの種類ごとの方針は [ADR-0045](../adr/0045-client-server-cache-policy.md) | `features/*/api` と `lib/` の外で `no-restricted-imports`（`openapi-fetch`・`openapi-react-query`・`lib/api`）・`no-restricted-globals` / `no-restricted-properties`（`fetch`）、`features/*/hooks` と `features/*/components` で `restrict/feature`（キャッシュを書き換える `QueryClient` のメソッド `setQueryData`・`invalidateQueries` などの呼び出し）、`@tanstack/eslint-plugin-query` の `flat/recommended-strict`（`@tanstack/query/exhaustive-deps`・`@tanstack/query/prefer-query-options` など） |
| S2 | サーバーのデータを `useState`・Context・グローバルストアにコピーしない | レビュー |
| S3 | 共有・リロード・戻る / 進むで保ちたい状態（タイムラインの種類・タブ・検索語・開いているメディア）は URL に置き、Zod で検証する | TanStack Router の型 |
| S4 | 状態は使う場所の近くに置き、必要なときだけ最も近い共通の親に持ち上げる | レビュー |
| S5 | 状態は判別可能なユニオンで表す（`{ status: 'idle' } \| { status: 'error'; error: Problem }`）。矛盾し得る真偽値を並べない | レビュー |
| S6 | Context は、変化の少ない値（閲覧者・ロケール・テーマ・複合部品の内部）にだけ使い、更新頻度ごとに分ける | レビュー |
| S7 | ルートは薄くする。パラメータと検索条件を取り出し、loader で、キャッシュにないか無効化されていれば取得して待ち（`queryClient.query({ ...options, staleTime: Infinity })`。無限スクロールは `queryClient.infiniteQuery`。無効化の印を見ない `staleTime: "static"` と、非推奨の `ensureQueryData` は使わない）、それから機能の部品を表示する。ルーターは `defaultStaleReloadMode: "blocking"` で、前に訪れたルートでも loader の終わりを待って描画する（ADR-0045） | `@tanstack/eslint-plugin-router` の `flat/recommended`（`@tanstack/router/create-route-property-order` など）、`restrict/syntax`（`staleTime: "static"`・`ensureQueryData`）、レビュー |
| S8 | 楽観的更新は、まず保留中の mutation の `variables` を表示する方式にする。複数の画面に反映が要るときだけ、キャッシュを書き換えて失敗時に戻す | レビュー |
| S9 | API の型は OpenAPI からの生成物だけを使う。同じ型を手で書かない | 生成物の差分検査（ADR-0029） |
| S10 | SSE の購読は `lib/sse` の 1 つの Effect だけが行い、受け取った内容を Query のキャッシュに書き込む | レビュー |
| S11 | ブラウザの保存（localStorage・sessionStorage）は `lib/storage` で宣言したキーだけを通して読み書きする。新しく残すものは、そこにキーを宣言してから使う（下の「ブラウザの保存」） | `lib/storage` とテストの外で `restrict/storage`（`localStorage`・`sessionStorage` の参照） |

### ブラウザの保存

`lib/storage.ts` が、アプリがブラウザに残すものをすべて宣言する。キーごとに、置き場所（localStorage か sessionStorage）・値の型とその検査（Zod）・保存形式の版・何も残っていないときの既定値・利用者のデータか（セッションが終わると消すか）を決める。
宣言したキーのオブジェクトの `read()` / `write()` / `remove()` で読み書きし、React からは `useStorage(key)`（`useSyncExternalStore`）で読む。
値は `{"v":<版>,"value":<値>}` の JSON で残す。JSON でないもの・版の違うもの・検査を通らないものは、何もないのと同じく既定値として読む。既定値を書くとキーを消す。
同じタブでの書き込みはそのキーの購読者に、ほかのタブでの書き込みは `storage` イベントで伝わる。ストレージが使えない（ブロックされている、容量が足りない）ときは、書いた値をそのページの間だけメモリに置き、ページの動きは変えない。
セッションが終わると、`clearUserStorage()` が利用者のデータと宣言したキーを両方のストレージから消す（§10）。

| キー | 置き場所 | 内容 | 利用者のデータ |
| --- | --- | --- | --- |
| `my:display:theme` | localStorage | 表示のテーマ（§9）。`public/theme.js` も同じキーと形式を読む | いいえ |
| `my:display:locale` | localStorage | 最後に表示したロケール（§9） | いいえ |
| `my:display:autoplay` | localStorage | アニメーションの自動再生（§13） | いいえ |
| `my:instance:timelines` | localStorage | インスタンスが提供する公開タイムライン（§9） | いいえ |
| `my:emoji:recent` | localStorage | 最近選んだカスタム絵文字の shortcode（§12、ADR-0051 §4） | いいえ |
| `my:user:compose:<アカウント ID>` | sessionStorage | 投稿の下書きと送信中の投稿（§12） | はい |
| `my:oauth-pending` | sessionStorage | 進行中のサインインの verifier・`state`・戻り先（§10） | いいえ |
| `my:preload-error-reloaded-at` | sessionStorage | チャンクの読み込みに失敗して再読み込みした時刻（§11） | いいえ |

## 3. 構成と責務分離

| # | 規約 | 強制 |
| --- | --- | --- |
| A1 | 依存の向きは `routes → features → (@my/ui, lib)` だけにする | `boundaries/dependencies`（要素は `shell`（`web/src` 直下）・`route`・`feature`・`lib`・`ui`、テストだけが import できる `test`（`web/src/test`）と `feature-test`（`web/src/features/*/test`））、`boundaries/no-unknown-files` |
| A2 | `@my/ui` は React・React Aria Components・Tailwind のユーティリティと、ドメインを持たない振る舞いのライブラリ（許可リストは ADR-0031）にだけ依存する。API・Query・Router・機能・i18n のカタログを import しない（文言は props / children で受け取る） | `boundaries/dependencies`（`ui` が import できるのは `ui` 自身と React・React Aria・`@tanstack/react-virtual`・`tailwind-variants`・`tailwind-merge`、`Icons.tsx` だけは `react-icons` だけ）、パッケージの依存関係（Bun の isolated linker） |
| A3 | 機能は、ほかの機能を `index.ts` 経由でだけ使う | `boundaries/dependencies`（ほかの `feature` は `index.ts` だけ） |
| A4 | 機能のロジックは `features/<x>/hooks` のカスタムフックに置く。フックは JSX を返さない。機能の部品はフックの戻り値を `@my/ui` に渡すだけにする | レビュー |
| A5 | 操作できる要素は `@my/ui`（React Aria Components を包んだ部品）から使う。`onClick` を付けた `div` / `span` を書かない | `restrict/syntax`（`div` / `span` の `onClick`・`onKeyDown`・`onPointerDown` など） |
| A6 | `@my/ui` の部品は表示に専念し、状態はできるだけ外から渡す（`value` / `onChange`） | レビュー |
| A7 | props は型を明示する（`React.FC` を使わない）。ネイティブ要素の props は `ComponentPropsWithRef<'button'>` で拡張する。多態的な `as` props を使わない | `typescript/no-restricted-types`（`FC` / `FunctionComponent`）、レビュー |
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
| D5 | `100vh` を使わない（`svh` / `dvh`）。端に固定する要素には `env(safe-area-inset-*)` を足す | `restrict/syntax`（文字列）、`better-tailwindcss/no-restricted-classes`（`h-screen` など）、`make web-lint` の grep（CSS） |
| D6 | アニメーションは `motion-safe:` の下に置く | `make web-tokens`、`better-tailwindcss/no-restricted-classes`（`animate-*`）、レビュー |
| D7 | 方向は論理プロパティのユーティリティで書く。`ml-` / `mr-` / `pl-` / `pr-` / `left-` / `right-` / `text-left` / `text-right` / `rounded-l` / `rounded-r` / `border-l` / `border-r` / `mask-l-` / `mask-r-` を使わない（方向のあるアイコンの反転と、論理方向の版がない `mask-l-` / `mask-r-`（端のフェード）だけ、`rtl:` / `ltr:` の下で許可） | `better-tailwindcss/no-restricted-classes` |
| D8 | 色は `@my/ui/theme.css` の意味のトークン（`--color-surface`、`--color-fg-muted` など）だけを使う。生のパレット（`--palette-*`）や任意の色を部品に書かない（`--color-*: initial` で Tailwind の既定の色を消す）。トークンの一覧・コントラストの組・ライト / ダークの切り替えは [app/ui/README.md](../../app/ui/README.md) にある | Tailwind の設定、`make web-tokens`、`better-tailwindcss/no-unknown-classes` |
| D9 | 部品の見た目の違いは `tailwind-variants` の 1 部品 1 レシピで定義する。クラス名を動的に組み立てない。`className` は `tailwind-merge` で root / スロットにだけ合成する | `better-tailwindcss/no-concatenated-classes`、`better-tailwindcss/no-conflicting-classes`、レビュー |
| D10 | CSP を壊さない。`setAttribute('style')` や `cssText` を使わない（React の `style={}` は可） | `restrict/syntax` |

## 7. テスト

| # | 規約 | 強制 |
| --- | --- | --- |
| T1 | 利用者の使い方に近い形でテストする。フックの内部や state を直接検査しない | レビュー |
| T2 | 操作は `user-event` で再現する。`screen` を使い、`waitFor` + `getBy` ではなく `findBy` を使う | `testing-library/prefer-user-event`、`testing-library/prefer-screen-queries`、`testing-library/prefer-find-by` |
| T3 | ネットワークは MSW だけでモックする。`fetch` や API のモジュールをモックしない | テストの `restrict/test`（`api` を含むモジュールと `openapi-fetch` の `vi.mock` / `vi.doMock`、`fetch` の `vi.spyOn` / `vi.stubGlobal` / 代入）、レビュー |
| T4 | 既定は統合テスト（本物の QueryClient とルーター + MSW）にする。純粋なロジックは単体テスト、主要な流れは 4 ロケールの E2E にする | レビュー |
| T5 | `@my/ui` の部品は必ずストーリーを持つ。ストーリーはアクセシビリティ・見た目・RTL のテストの入力になる | レビュー、`knip`（使われないファイル・export・依存） |
| T6 | 部品のテストは jsdom かブラウザモードで動かす。React Compiler の preset はクライアント向けの変換にだけ効くので、Vitest の `node` 環境では本番と違うコードをテストすることになる | Vitest の設定 |
| T7 | 非同期の結果は Testing Library の `findBy` / `waitFor` で待つ。`vi.waitFor` は使わない。待ち時間の上限はテストごとに変えず、setup の `asyncUtilTimeout` と Vitest の `testTimeout` にまとめる（待ちは実時間なので、負荷の高い機械では既定の 1 秒・5 秒を超える）。React の描画の外で動くフォーカス（React Aria のフォーカスの復帰は `requestAnimationFrame`、ルート変更の見出しへのフォーカスはルーターの `onRendered`）は `waitFor` の中で `toHaveFocus()` を検証する。遷移の後は遷移先にしかないもの（タブなら名前で選んだ `tabpanel` とその中身）を待つ。読み込みの間は前のページが残り、`defaultPendingMs` を超えると pending component に置き換わるので、どのページにもある要素や「ないこと」の確認は遷移の前の画面でも通ってしまう | テストの `restrict/test`（`vi.waitFor`）、Vitest の設定、レビュー |

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
  - axe は、ページの終わりのあるアニメーションとトランジションがすべて終わってから検査を始める（`e2e/fixtures.ts` の `axeBuilder`。`document.getAnimations()` の `finished` を待つ）。
    種類の変わるボタン（「フォロー」のアクセントから「リクエスト中」の面へ）は `motion-safe:transition-colors` で色を移すので、途中で `color-contrast` が中間の文字色と背景色を読むと違反になるため。
    終わりのないアニメーション（スピナー、スケルトンの明滅）は待たない。
  - プロジェクトは画面幅 320 / 390 / 768 / 1024 / 1440 px × ロケール `en` / `ja` / `en-XA` / `ar-XB` の 20 個。768 px 未満はタッチと粗いポインタのスマートフォンとして動かす。
  - main への push では全ロケールの 320 / 1440 px だけを動かし（PR では動かさない、[ADR-0040](../adr/0040-e2e-on-main-only.md)。Web に触れる PR では作者が手元で `make web-e2e` を回すか、PR のブランチに対して `web-e2e.yml` を `workflow_dispatch` で手で回す）、nightly（`E2E_MATRIX=full`）は全体を動かす。
  - CI（`web-e2e.yml`）は `app/web/dist` と `bin/my` を 1 回だけビルドし、全シャードが同じビルドを検査する（[ADR-0047](../adr/0047-ci-protection-layers.md)）。シャードは main が 4、nightly が 8 で、各シャードの blob レポートを 1 つの HTML レポートにまとめる。`make web-e2e` は手元で全部を 1 回で回す。
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
  - `make check` はブラウザなしで動く検査だけを集める。`make ui-test` は CI の `ui-test` ジョブがそのイメージの中で、`app/ui` を変える PR と main への push で動かす（Merge Queue では動かさない、[ADR-0047](../adr/0047-ci-protection-layers.md)）。失敗したときの実際の画像と差分は成果物 `ui-test-screenshots` にある。
  - ワークフローのイメージと `PLAYWRIGHT_IMAGE`、`playwright` / `@playwright/test` の版が食い違うと、`make playwright-image-check`（`make check` の一部）が失敗する。

## 8. サーバーが描画する画面

ログイン（二要素認証を使う利用者に認証コードを求める 2 段目の画面 `/auth/sign_in/two_factor` を含む）・登録・メール確認・アカウントの状態・OAuth の認可同意の画面と、利用規約・プライバシーポリシーの公開ページ（`/auth/terms`・`/auth/privacy`）は、Go の `html/template`（`internal/interface/authpages`）が描画し、JavaScript なしで動く（ADR-0030）。CAPTCHA を有効にしたときだけ、ログインと登録の画面はサービスのウィジェットのスクリプトを読み、そのページの CSP にサービスのオリジンを足す（ADR-0042）。

- スタイルは Vite のビルドの別のエントリ `app/web/src/server-pages.css` にある。Tailwind と `@my/ui/theme.css` を読み込み、テンプレートが使う少数のクラス（`page`・`card`・`field`・`hint`・`field-error`・`alert`・`notice`・`button` など）を
  トークンのユーティリティの `@apply` で定義する。テンプレートからクラスを集めないので、Tailwind はソースを走査しない（`source(none)`）。
- `my serve` はビルドのマニフェストのキー `src/server-pages.css` からファイル名を引き（`web.Handler.Stylesheet`）、画面の `<link rel="stylesheet">` にする。SPA の外枠はこの CSS を読み込まない。
  サーバーのテーマ（§9 の「サーバーのテーマ」）があれば、その後にテーマのスタイルシートも読む（`authpages.Options.ThemeStylesheet`）。この CSS はユーティリティを持たないので、テーマのトークンは `@apply` で作った規則と `theme.css` の基本の規則を通して効く。
  Vite の開発サーバーを使うときはビルドがないので、画面にスタイルは付かない。
- CSP は `style-src 'self'` で、インラインのスタイルとスクリプトを置かない。
- ダークモードは `prefers-color-scheme` に従う（トークンの `light-dark()` と `<meta name="color-scheme" content="light dark">`）。フォーカスのリングは `theme.css` の `:focus-visible`、
  タップ領域は `pointer-coarse:` で 44 px 以上、方向は論理プロパティ（`px-`・`ps-`・`pe-`）で書く（D4・D7・Y4）。
- フォームは `<label for>`、`autocomplete`（`username`・`current-password`・`new-password`・`one-time-code`）、ヒントとエラーの `aria-describedby`、エラーの `aria-invalid` を持ち、ブラウザの制約検証（`required`・`minlength`）を使う。
- 利用規約とプライバシーポリシー（#489）: 管理者が Markdown で書いた文書を、サーバーが HTML にしてサニタイズし（`internal/adapter/markdown`）、`.document` の区画に、文書のロケールの `lang` と `dir="auto"` で出す。見出しはページの `h1` の下の `h2` から始まり、幅の広い表は区画の中で横にスクロールして、ページは横にスクロールしない（D3）。
  サーバーが公開している文書は、すべての画面の `main` の後のフッター（`nav`「規約とポリシー」）と、登録のフォームの作成のボタンの前の同意の文（ボタンの `aria-describedby`）からリンクする。公開していない文書はリンクせず、そのページは 404 にする。

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
- **起動**（`main.tsx`）: このブラウザで最後に表示したロケール（localStorage の `my:display:locale`）か、なければサーバーが外枠に書いた `<html lang>`（インスタンスのロケール）から、
  カタログのある最も近いロケールを決め（`ja-JP` → `ja`、ない言語は `en`）、`<html>` の `lang` / `dir` をそれに合わせる（`lib/locale.tsx` の `loadStartI18n`）。
  次に `<script type="application/json" id="config">` のランタイム設定を Zod で検証して読む（`lib/config`、`useConfig()`）。読めなければアプリの代わりに起動エラーの画面を出す。
  読めたら、表示のロケール・QueryClient・セッション・API クライアント（`lib/services.ts`）を作り、リフレッシュトークンの Cookie からセッションを再開し始める（§10）。
- **表示のロケールとタイムゾーン**（`lib/locale.tsx`、FR-007）: ページを開いている間に変わるので、React の外のストア（`AppLocale`、`useSyncExternalStore` で読む）に置く。
  サインインした利用者の設定（`GET /preferences`）を読むと、外枠（`AppLayout` が呼ぶ `features/settings` の `useUserDisplaySettings`）がその `locale` と `time_zone` をストアに渡す。
  ストアはそのロケールのカタログを読み込んでから `<html>` の `lang` / `dir` を変え、i18n のプロバイダを描画し直す。読み込みが前後したときは最後に頼んだロケールが勝つ。
  タイムゾーンは `react-intl` の `timeZone` になる。ロケールが変わると、サーバーが `Accept-Language` で訳した内容を取り直すために、Query をすべて無効にする。
  ストアは決めたロケールを `my:display:locale` に残す。設定を読むまでと、設定が読めないとき（429 を含む）とサインインしていないときは、最後に表示したロケール（初めてのブラウザではサーバーのロケール）のままになり、`<html lang>` も戻さない。
  このキーはこのブラウザでの表示のしかたで、利用者のデータではないので、セッションが終わっても消さない。次の利用者の設定が読めればそれに替わる。タイムゾーンは残さず、設定を読むまではブラウザのものになる。表示言語の選択肢は、カタログのあるロケール（疑似ロケールは、今それで表示しているときだけ）を、それぞれの言語の名前で並べる。
  タイムゾーンの選択肢は、表示中のロケールの CLDR の名前（`Intl.DisplayNames` にタイムゾーンの種類はないので、`Intl.DateTimeFormat` の `timeZoneName: "longGeneric"`。名前のないゾーンは UTC からの差だけ）に、
  画面を開いた時点の UTC からの差を付けて「（UTC+09:00）日本標準時」と出し、その差と名前の順に並べる。IANA の名前（`Asia/Tokyo`）は、同じ名前のゾーンを見分けるための選択肢の説明（`SelectItem` の `description`）にする。
- **端末のタイムゾーン**（FR-007）: タイムゾーンをまだ選んでいない利用者（`time_zone_chosen` が偽。新しいアカウント）の日時は端末（ブラウザ）のタイムゾーンで出し、
  `useUserDisplaySettings` が最初に開いた画面で、そのゾーン（`Intl.DateTimeFormat().resolvedOptions().timeZone`）を `PATCH /preferences` で保存する。
  外のシステム（ブラウザのゾーンとサーバーの設定）に合わせる処理なので Effect に置き、後始末で要求を中止する。失敗したら選ばないままにして、次に開いたときにまた試す。
  設定の画面は、選んでいるゾーンが端末のゾーンと違うとき（`Asia/Calcutta` と `Asia/Kolkata` のような別名は同じとみなす）、選択肢の下に端末のゾーンの名前を書いた注記と「この端末のタイムゾーンにする」ボタンを出す。
  ボタンを押すとフォーカスをタイムゾーンの選択肢に移してから保存する（注記はボタンごと消える）。
- **表示のテーマ**（`lib/theme.ts`）: 「システムに合わせる / ライト / ダーク」から選び、`<html data-theme>`（`light` / `dark`、システムに合わせるときは属性なし）にする。トークンの切り替えは [app/ui/README.md](../../app/ui/README.md) の「Light and dark」にある。
  表示言語の `my:display:locale` と同じく、このブラウザでの表示のしかたなので、利用者の設定（`GET /preferences`）ではなく localStorage の `my:display:theme` に残し、サインインしていなくても効き、セッションが終わっても消さない。
  外枠は `<head>` のスタイルシートより前で、同じオリジンのクラシックスクリプト `public/theme.js`（`/theme.js`、`defer` / `async` なし）を読み込み、最初の描画の前に属性を付ける。CSP がインラインのスクリプトを許さず、外枠はどの閲覧者にも同じもの（ADR-0028）なので、サーバーは選択を外枠に書けない。
  `my serve` はビルドにこのファイルがあるときだけタグを書き（`internal/interface/web`）、Service Worker はほかのビルドのファイルと一緒にプリキャッシュする。
  属性を状態とし、`useTheme()` が `useSyncExternalStore` で読む。選ぶとすぐに属性と保存先を変え、ほかのタブでの選択も `storage` イベントで追う。保存できないとき（ストレージが使えない）もそのページには効く。
  選ぶ場所は「表示と投稿」の設定とメニューのシート（ADR-0039）で、どちらも同じ `ThemeRadioGroup`（APG の radio group。矢印キーで移ると選ぶ）にする。サーバーが描画する画面（§8）は JavaScript を使わないので、OS の設定に従う。
- **サーバーのテーマ**（[ADR-0059](../adr/0059-design-tokens-and-instance-theme.md)、FR-605）: 管理者が保存したテーマ（トークンの上書きとカスタム CSS）は、`my serve` が版ごとの同じオリジンのスタイルシート `/theme/{版}.css`（`text/css`、`nosniff`、`immutable`）にして返す。
  外枠はビルドのスタイルシートの後にこれを `<link rel="stylesheet">` で読むので、最初の描画からテーマが当たる。規則は `@layer` の外にあり、Tailwind の `@layer theme` の既定値とユーティリティに勝つ。CSP（`style-src 'self'`）は変えない。
  外枠は閲覧者ごとに変わらず、ETag はテーマの版を含む。テーマがなければ `<link>` を出さない。
  `<meta name="theme-color">` は `media="(prefers-color-scheme: light)"` と `dark` の 2 つで、値はテーマ（なければ既定）の `color-surface`。Web App Manifest の `theme_color` / `background_color` はそのライトの値にする。
  `data-theme` で OS と違うテーマを選んだときのブラウザの枠の色は OS に従う。
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
  - プロフィールの下（`/@{acct}/…`）の第 1 区間は、次の予約語だけにする（[ADR-0053](../adr/0053-status-page-url.md)）。ID やタグ名などの可変の値は第 1 区間に置かず、語の下に置く。
    語は `[a-z][a-z0-9_-]*` で、1 件を指すページは単数、一覧は複数にする。ルートのファイルは `routes/@{$acct}_.<語>….tsx`。
    語を足すときはこの表を更新する（ADR は要らない）。サーバーが役割を持つのは `status` だけで（外枠の meta タグと、ActivityPub の `Accept` での `id` へのリダイレクト）、ほかの語のアドレスには汎用の外枠が返る。

    | 語 | ページ | 状態 |
    | --- | --- | --- |
    | `status` | 投稿の詳細（`status/{id}`、§13）と、投稿の下のページ（`status/{id}/history` の編集履歴など） | 実装済み |
    | `followers` | フォロワー一覧 | 実装済み |
    | `following` | フォロー一覧 | 実装済み |
    | `media` | メディア付きの投稿 | 予約 |
    | `favorite` | お気に入り | 予約 |
    | `with_replies` | 返信を含む投稿 | 予約 |
    | `tagged` | ハッシュタグで絞った投稿（`tagged/{tag}`） | 予約 |
    | `featured` | ピン留めした投稿 | 予約 |

- **リンク**: 画面の間のリンクは `@my/ui` の `Link` に `href` を渡す。React Aria の `RouterProvider` が同じオリジンの `href` をルーターの遷移に変える。`#id` の `href`（本文へ移動するリンク）は、その要素にフォーカスを移す。
  投稿の本文とプロフィール（`PostContent`）の中のリンクは、サーバーが描画した HTML のもので、自インスタンスのページも `https://{host}/...`（ADR-0018。連合でも同じ HTML を配る）を指す。
  これを同じタブで開くと（修飾キーのない左クリックか Enter）、`PostContent` の `onLinkClick` に渡した `lib/contentLinks.ts` の `useContentLinkClick()` が、
  ホストが `apiOrigin` のホストと同じ（スキームは問わない）プロフィール・投稿・ハッシュタグのリンク（`/@{acct}`・`/@{acct}/status/{id}`・`/tags/{tag}`）をルーターの遷移に変える。
  そのため平文 HTTP のローカルの構成（`apiOrigin` が `http://localhost:8080`）でも、TLS のない `https://` に移らずに該当の画面が開く。ほかのリンクと、新しいタブやウィンドウで開くことはブラウザに任せる。
  サーバー自身のパス（`/auth`・`/oauth`・`/api` などの予約パス、`lib/paths.ts` の `isServerPath`）へのリンクはアプリのルートではないので、外枠の `navigate`（`App.tsx`）がルーターに渡さず、`leaveApp` でページごと開く（利用規約とプライバシーポリシーの公開ページ）。
- **レイアウト**（`routes/-layout/AppLayout.tsx`）: モバイルファーストで、`md` 未満は下端のタブバー、`md` から横のレール、`lg` から文言付きのサイドバー、`xl` から投稿フォームとサーバーについての 3 列目を出す。
  `xl` 未満では、投稿フォームはタブバーの上（`md` からは画面の隅）に浮かぶボタンから開くシートになる（§12）。どちらにするかは外枠が `matchMedia` で決め（`useMediaQuery`）、ページに置くフォームは常に 1 つにする。
  浮かぶボタンは、ルートの静的なデータ（TanStack Router の `staticData`、`composeButton: true`）を持つ画面（タイムラインとプロフィール）にだけ出す（G4）。外枠は今のマッチ（`useMatches`）からそれを読む。
  ほかの画面でもシートは置いたままにし、返信のボタンはそこで開く。`md` 未満のボタンはアイコンだけで、「投稿を書く」をアクセシブルネーム（`aria-label`）と `Tooltip` にする（G12）。
  `main` は終わりに、タブバーとボタン（出ているとき、`data-compose-fab`）の高さぶんの余白を取り、`index.css` の `scroll-padding-block-end` もフォーカスした要素を同じだけ避ける。`md` 未満のトーストは、タブバーの上に、ボタンが出ているときはボタンの上に出す（`with-compose-fab:` の variant）。
  `md` 未満で高さが 32rem 以下の画面（横向きの電話、400% の拡大の 320 × 225 CSS px）は `short:` の variant（`index.css`）で、タブバーの項目を `min-h-12` にし、ボタンとの間を詰め、上端のオフラインの帯（G17）を粘着させずにページと一緒に流して、固定した要素を画面の高さの半分に収める（WCAG 1.4.10）。
  ナビゲーション（`routes/-layout/Navigation.tsx`）は 1 つの `nav` で、今いる区画のリンクに `aria-current="page"` を付ける。「プロフィール」の区画は自分のプロフィールとそのフォロワー / フォローの一覧で、
  同じ `/@{acct}` の下にある自分の投稿の詳細は含めない。画面の端に接する要素は `env(safe-area-inset-*)` を足し、高さは `svh` で指定する。
  - 項目は [ADR-0039](../adr/0039-mobile-navigation-and-compose-entry.md) に従い、ローカルと連合のタイムラインは 1 つの画面のタブにせず、どの幅でもナビゲーションの別の項目・別の画面にする（決定者の指示、[#660](https://github.com/best-friends/my/issues/660)）。
    `md` 未満のタブバーは、ホーム・ローカル・連合・通知・検索・メニューの 6 つ（サーバーが提供しないタイムラインの項目は出さないので 4〜6 つ）で、
    `md` からのレールとサイドバーは、ホーム・ローカル・連合・通知・検索・プロフィール・ブックマーク・設定の 8 つにする。どちらも 1 つのリストの同じ順番で、各項目はその幅で出ないときに `display: none`（アクセシビリティツリーからも外れる）にする。
    スタッフの画面は「モデレーション」と「サーバー管理」の 2 つの区分に分ける（[ADR-0058](../adr/0058-moderation-admin-separation.md)、§14）。ロールがモデレーションの画面の権限（`manage_reports`・`manage_accounts`・`manage_domains`・`manage_invites`・`view_audit`）のどれかを持つ閲覧者には「モデレーション」（`/moderation`）を、
    サーバー管理の画面の権限（`manage_settings`・`manage_rules`・`manage_custom_emojis`・`manage_jobs`）のどれかを持つ閲覧者には「管理」（`/admin`）を、設定の後にこの順で足す。どちらも、その区分に閲覧者の使える画面があるときだけ出すので、組み込みのモデレーターには「管理」が出ない。レールとサイドバーとメニューのシートに出し、タブバーには出さない。
    jsdom はスタイルを当てないので、統合テストでは両方の項目が文書にあり、どの幅で何が出るかは E2E（`e2e/shell.spec.ts`・`e2e/federated.spec.ts`）で確かめる。
  - `lg` からのサイドバーは、リストの上にサーバーの名前（下の「サーバーの名前」）を、リストの下（`nav` の末尾）にサインインしているアカウントのアバター・表示名・ハンドルを出す。
    アカウントはそのプロフィールへのリンクで、名前は表示名とハンドル（それぞれ 1 行に省略、G7）、説明（`aria-describedby`）は「サインインしているアカウント」にする。
    場所はリストの「プロフィール」が示すので、このリンクは `aria-current` を持たない。どちらもリストの外に置くので、リストのリンクとその順番はどの幅でも変わらない。`lg` 未満では出さず、サインインしていないときと読み込み中はアカウントを出さない。
  - タブバーの各項目は文言の幅を基本にし、残りの幅を等分する（`flex: 1 1 auto`）。長い文言（「Notifications」）が短い文言（「Menu」）の余りを使う。
    文言は省略せず、収まらない翻訳（6 つの項目の 320 px）では単語の途中でも折り返して、全体を読めるようにする。
  - 「ローカル」（`/local`）と「連合」（`/public`）はそれぞれの画面へのリンクで、その画面でだけ `aria-current="page"` になる。どちらの画面も見出しの下にそのタイムラインだけを置き、もう一方へのタブを持たない。
  - 「メニュー」は場所ではなくボタンなので `aria-current` を持たない。押すと `Sheet`（モーダルのダイアログ）を開き、タブバーにないレールの項目（プロフィール・ブックマーク・設定、閲覧者によってモデレーションと管理）を並べて、今いる区画のリンクに `aria-current="page"` を付ける。項目の下に表示のテーマの切り替え（横に並べた `ThemeRadioGroup`）を置く。
  - サーバーの利用規約とプライバシーポリシー（`GET /instance` の `terms_url` / `privacy_policy_url`、`features/documents` の `DocumentLinks`）は、`xl` からの 3 列目（`ServerInfo`）、メニューのシートの末尾（テーマの切り替えの下）、設定の画面の「規約とポリシー」の区画、サインインの画面（`/login`）にリンクする（`nav`「規約とポリシー」）。サーバーが公開していない文書は出さない。
  - 件数のバッジ（`@my/ui` の `Badge`）: 「通知」は未読数、「モデレーション」はモデレーターを待つ件数（`GET /admin/counts` の未解決の通報・承認待ちの登録・審査待ちの異議申し立ての合計、FR-712）を、アイコンの角に数字で出す。
    数字はアクセシビリティツリーから隠し、件数はリンクのアクセシブルネーム（見える名前で始まる「通知、未読 3 件」「モデレーション、未対応 3 件」、WCAG 2.5.3）で伝えるので、色だけで示さない（WCAG 1.4.1）。
    シートの「モデレーション」も同じ名前とバッジを持ち、タブバーの「メニュー」のボタンはシートの項目の件数を自分のバッジと名前（「メニュー、モデレーションに未対応 3 件」）で示す。「管理」には件数を出さない。
    件数は、その件数に要る権限（`manage_reports`・`manage_accounts`）を持つ閲覧者だけが読む。ストリームの `admin_report` / `admin_sign_up` / `admin_appeal` の通知と再接続、登録の決定と通報の処理で読み直す。読めないときはバッジを出さない。
    シートはパスの変わる遷移が始まったとき（ルーターの `onBeforeNavigate`）に閉じるので、新しい画面の `h1` がフォーカスを受け取れる。未保存の変更の確かめで止めた遷移は始まらないので、シートは開いたまま残る。
  - ローカル（`/local`）と連合（`/public`）のタイムラインは、インスタンスの設定で無効にでき、そのときは API が 404 を返す（FR-202）。タブバー・レール・サイドバーは `GET /instance` の `timelines` で、
    サーバーが提供するものだけを出す（`features/timeline` の `useTimelineAvailability`）。ローカルはサーバーが無効と答えるまで出し、連合はサーバーが有効と答えてから出すので、無効にしたサーバーでは読み込み中も失敗したときも出ない。
  - 最後に受け取った `timelines` は localStorage の `my:instance:timelines` に残し（インスタンスの設定で、利用者のデータではない）、ナビゲーションを置いたときに 1 回だけ読む。
    オフラインで開いたりリロードしたりして `GET /instance` が届かないときも、前と同じ項目を出す。サーバーの答えが届けばそちらを使う。
  - 無効にしたタイムラインのアドレスを直接開いたとき（古いリンクやブックマーク）は、「もう一度試す」ではなく無効であることを出す（`routes/local.tsx`・`routes/public.tsx`）。
  先頭に、フォーカスしたときだけ見える「本文へ移動」のリンクを置く。
- **フォーカスとタイトル**（`routes/-layout/useViewChange.ts`、Y5）: ルーターが画面を描画するたびに、`main` の `h1` の文字列から `document.title` を `client:shell.document_title`（画面名とサーバー名）で作る。サーバーの名前が変わったときも作り直す（フォーカスは動かさない）。
  パスが変わったときだけ、その `h1` にフォーカスを移す。最初の画面と、検索条件やハッシュだけの変化（タブ・絞り込み）ではフォーカスを動かさない。画面の見出しは `PageHeading`（`tabindex="-1"` の `h1`）で書く。
- **サーバーの名前**（`lib/instanceName.ts` の `useInstanceName`）: サイドバー・3 列目の「このサーバーについて」・タブのタイトル・ログインの画面は、サーバーを `GET /instance` の `name`（閲覧者の言語）で呼ぶ。
  読み込むまでと読めないときは、外枠のランタイム設定の `instanceName`（サーバーが既定の言語で書いた名前）にする。どちらも、名前がなければサーバーのドメインである。
  管理画面で名前を変えると、取り直した `GET /instance` で再読み込みなしに変わる。ハンドルとリンクは名前によらずドメイン（ランタイム設定の `accountDomain`）のままにする（[名前とドメイン](../operations/instance-settings.md#名前とドメイン)）。
- **エラー**（E1、G16）: 画面の失敗はルーターの `defaultErrorComponent`（`RouteError`）がレイアウトの中に出す。画面の `h1` はその画面の名前のままにし（ルートの `staticData.title` のメッセージ ID。名前がパラメータで決まる画面は自分の `errorComponent` から `heading` を渡す）、本文の位置に `RegionError` を出す。
  見出しは失敗の種類（`lib/errors.ts` の `errorKind`: 通信できない・サーバーの失敗・見つからない（404）・権限がない（403）・画面自体の不具合）ごとに分け、理由は `errorMessage(error)`（通信できないときはオフラインかどうかも言う。画面自体の不具合はカタログの文）にする。404 と 403 には「もう一度試す」を出さない。
  「もう一度試す」は、まずその失敗を起こした query だけをその場で取り直す。また失敗したら、エラーとボタンのフォーカスを残したまま新しい理由を `role="alert"` で出す。取れたら Query のエラー境界を戻して loader を再実行し、画面の `h1` にフォーカスを移す。
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
- **外枠の読み込み**: 外枠はどの画面でも、閲覧者（`GET /accounts/verify_credentials`、`useViewer`）・設定（`GET /preferences`、`useUserDisplaySettings`）・インスタンス（`GET /instance`、`useTimelineAvailability`）・
  通知の既読位置（`GET /notifications/marker`、`useUnreadNotificationCount`）を読む。どれが失敗しても画面全体のエラーにはせず、前に読めた値（Query のキャッシュ）か既定の値（閲覧者は読み込み中か `AccountSection` の区画のエラー、
  表示言語は上の「表示のロケール」、タイムラインは localStorage の値、未読数は 0）で表示を続ける。閲覧者は、前に読めたアカウントがあれば再取得が失敗してもサインイン中のままにする。
  これらの query の `retry` は `lib/query.ts` の `shellQueryRetry` にする（loader は再試行しないまま）。同じものを外枠と同時に読む投稿フォーム（`useComposeForm`）は `shellQueryReader` で、さらに `retryOnMount: false` にし、外枠がすでに知っている失敗をマウントのたびに取り直さない（429 の制限を使い増やさず、帯も消えて出直さない）。4xx（429 を含む）は再試行せず、それ以外（5xx・`NetworkError`）は 1 回だけ再試行するので、失敗は約 1 秒で区画に出る。
- **レート制限**（`lib/rateLimit.ts`）: サーバーのレート制限の 429 は `Retry-After`（秒数か HTTP の日付）を持つ。API クライアントはそれを `ApiError` の `retryAt`（時刻）にし、`rateLimitedUntil(error)` が 429 のときだけそれを返す。
  レイアウトの `useRateLimitRetry` が、画面で使っている query（observer のあるもの）のうち 429 で失敗して止まっているものを Query のキャッシュから探し、その最も遅い `retryAt` に、それらを取り直す（時刻が理由なので Effect のタイマー）。
  その間、オフラインの帯（`ConnectionBanner` が出す 1 つの `OfflineBanner`）に、リクエストが多すぎることと、読み込み直す時刻（秒まで）を出す。オフラインのときはオフラインの文言を優先する。
  `src/shell.test.tsx` が外枠の 4 つの要求の失敗（5xx と 429）・前の値の保持・再読み込み後の表示言語・429 の帯とその時刻での取り直し・アカウントの区画の再試行を MSW で検査し、
  `e2e/rate-limit.spec.ts` が 429 の帯の axe・横スクロール・ARIA スナップショットと、時刻での取り直しを検査する。

## 10. API クライアントと認証

API の呼び出しとトークンは `lib/api`・`lib/auth` が 1 か所で扱う（ADR-0029、ADR-0030）。機能はこれを作り直さない。

- **型**（S9）: `make generate` が openapi-typescript で `api/openapi.yaml` から `lib/api/schema.gen.ts` を生成する。生成物はコミットし、`make generate-check` が差分を検査する。
  機能は API の型を自分の `features/<x>/api` から export して使う（`lib/api` を直接 import できるのは `features/*/api` と `lib/` だけ。S1）。
- **クライアント**（`lib/api/client.ts`）: openapi-fetch のクライアントと、その上の openapi-react-query のクライアントを 1 つずつ作る（`Api` の `fetch` と `query`）。
  機能は `features/<x>/api` で `api.query.queryOptions(...)` から `queryOptions` を作る。キーは `[method, path, params]` になる。
  書き込みは、`features/<x>/api` が `api.fetch` で作る `mutationKey` / `mutationFn` を TanStack Query の `useMutation` に渡す。
  openapi-react-query のフック（`api.query.useQuery` / `api.query.useMutation` など）は呼ばない（`restrict/syntax`）。オブジェクトから読むフックは描画ごとに同じ関数だと示せないので、React Compiler が呼んだフックごと最適化を諦め、`vite build` が失敗する（R4、[Don't dynamically use Hooks](https://react.dev/reference/rules/react-calls-components-and-hooks#dont-dynamically-use-hooks)）。
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
    セッションが終わったときは、`lib/storage` で利用者のデータと宣言したキー（利用者が入力したもの。投稿の下書き、§12）も消す（§2「ブラウザの保存」）。サインインでは消さない（キーにアカウントの ID を含める）。
  - 状態（`unknown` / `signed-in` / `signed-out`）は `useSessionStatus()` で読む。サーバーのデータである閲覧者のアカウントは Context に置かず、`features/session` の `useViewer()` が Query で取る（S2、S6）。
- **ログイン**（`lib/auth/signIn.ts`、Authorization Code + PKCE）: `/login` のボタンが、新しい verifier（32 バイトの乱数）と `state` を作り、S256 の challenge を付けてサーバーの認可画面（`/oauth/authorize`）にページごと遷移する。
  scope は指定せず、Web クライアントに登録されたすべてのスコープを受け取る。
  - verifier・`state`・戻り先は、往復の間だけ sessionStorage（このタブだけ。`my:oauth-pending`）に置き、コールバックが読んだ時点で消す。どれも資格情報ではない（コードは 1 回限り・短命で、verifier に結び付く）。
  - コールバック（`/login/callback`、`routes/login_.callback.tsx`）は `beforeLoad` で `state` を照合し、コードを交換してから、戻り先に `replace` で遷移する。コードは履歴にもアドレスバーにも残らない。
    失敗（拒否・`state` の不一致・交換の失敗・通信の失敗）は、そのルートの `errorComponent` が理由とやり直しのリンクを出す。
  - 戻り先（`return_to`）は、1 つの `/` で始まるパスだけを受け付ける（`//example.com` や `javascript:` は捨てる）。
- **リアルタイム更新**（`lib/sse`、S10、FR-207）: レイアウトの `useLiveUpdates`（`routes/-layout`）が、サインイン中だけ `lib/sse` の `useEventStream` で `GET /streaming` を 1 本開く。
  `EventSource` はヘッダを送れないので、API クライアントの `fetch`（`parseAs: "stream"`）で読む。トークンの付与と 401 のあとの 1 度の更新は、ほかの要求と同じミドルウェアが行う。
  本文は `lib/sse/parse.ts` が HTML Standard の手順で解釈する（`id` と `retry` は使わない）。
  - ストリームが終わったら約 1 秒後に、失敗したら失敗のたびに倍の間隔（最大 1 分、後半の半分にばらす）で接続し直す。セッションが終わるか外枠が消えると、Effect の後始末が要求を中止する。
  - サーバーは何も再送しないので、2 本目以降のストリームが開いたら、ホームの最初のページを読み直して新着を数え、未読数を読み直し、通知の一覧を古いことにする（再取得はしない）。
  - `update` は表示中のタイムラインに差し込まず、Query のキャッシュの別の項目（`features/timeline/api/newStatuses.ts`）に待たせる。ホームの `NewItemsButton` が、読み込み済みの投稿にないものの数を出し、押されたら最初のページの先頭に差し込んで見出しにフォーカスを移す（Y7）。
    ホームを表示していない（ホームのクエリを使う部品がない）ときは誰も読んでいないので、待たせずにキャッシュの最初のページの先頭に差し込む。ほかの画面からホームを開くときも、ルートの loader が待っている投稿を同じように差し込む。`staleTime` の間に開き直した、取り直さないホームも、最初の描画から新着を出す。
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
  - **Service Worker が応えないパス**: `/api`・`/oauth`・`/auth`・`/.well-known`・`/nodeinfo`・`/ap`・`/media`・`/uploads`・`/theme`（サーバーの予約パス）と `/manifest.webmanifest` への遷移。
    サーバーが描画する画面（§8）はネットワークが要り、SPA の外枠に置き換えてはいけない。
  - **クライアント API**: `/api` への要求には応えない（ネットワークだけになり、何もキャッシュしない）。`Authorization` 付きの要求はどのパスでもキャッシュしない。
  - **公開メディア**: 同じオリジンの `/media/` の画像だけを、キャッシュ優先（最大 300 件、30 日、200 の応答だけ）にする。動画は範囲要求の部分応答を再生できないので対象にしない。
  - **サーバーのテーマ**: 同じオリジンの `/theme/` のスタイルシートを、キャッシュ優先（最大 5 件、200 の応答だけ）にする。URL が版を含み内容が変わらないので、オフラインの外枠も最後に読んだテーマで表示する。
- **登録と更新**（`src/lib/pwa.ts`）: ビルドしたアプリだけが `/sw.js` をスコープ `/` で登録する。外枠の CSP が Trusted Types を要求するので、URL は `/sw.js` だけを通すポリシーで作る。
  - 最初の Service Worker は有効になるとすぐページを制御する（`clientsClaim`）。新しい版は待機し、`UpdatePrompt` が「新しい版があります」のトースト（`timeout: null`、「更新する」の操作付き）を出す。
    利用者が押したときだけ `SKIP_WAITING` を送り、新しい版がページを制御したら読み直す。ページが表示に戻るたびに更新を確かめる。
  - デプロイで古いチャンクが消えたとき（`vite:preloadError`）は読み直す。10 秒以内に繰り返したら読み直さず、エラー境界に任せる。
- **ログアウト**: セッションが終わると、`lib/auth/userData.ts` が `clearCaches()` を呼ぶ（§10）。事前のキャッシュも含めてすべてのキャッシュと、メディアの期限のメタデータを消す（ADR-0028）。
  消したあともオンラインでは同じように動き、新しい版の Service Worker が事前にキャッシュし直すまでオフラインの外枠は使えない。
- **Web Push**（FR-206）: 設定の「表示と投稿」の「プッシュ通知」（`features/settings` の `usePushNotifications`）で、この端末で受け取るかと通知の種類を選ぶ。インスタンスが `push_server_key` を返さないときは区画を出さない。
  - 有効にするとき（スイッチの操作の中）だけ `Notification.requestPermission()` で許可を求め、Service Worker の登録の `PushManager.subscribe`（`userVisibleOnly`、`applicationServerKey` はサーバーの鍵）で購読し、`PUT /push/subscription` で種類と一緒に登録する。
    別の鍵で作った購読が残っていれば解除してから購読し直す。種類を変えると登録し直し、無効にすると `DELETE /push/subscription` とブラウザの購読の解除を行う。
  - Push API のないブラウザ（iOS / iPadOS ではホーム画面に追加していない Safari を含む）と、サイトの通知をブロックしたブラウザでは、スイッチの代わりに理由を出す。
  - Service Worker（`sw/push.ts`）は、サーバーが利用者の言語で書いた題を、通知の ID を `tag` にして表示する（同じ通知の再送は表示を置き換える）。押すと、開いているアプリの窓（フォーカスのある窓、なければどれか）をメッセージのパスに移し、窓がなければ新しく開く。
    パスは同じオリジンのものだけを開き、それ以外と読めないメッセージは通知の画面を開く。
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
- **道具**（`components/ComposeTools.tsx`）: 本文の下の 1 行に、画像・CW・カスタム絵文字・公開範囲・言語のボタンを同じ高さで並べ、同じ行の終わりに送信のボタンを置く（320 px のシートでも送信が最初の画面に入る）。
  アイコンだけのボタン（画像・CW・カスタム絵文字）は `aria-label` と同じ文言のツールチップを持ち（G12）、CW はトグル（`aria-pressed`）にする。どのボタンも今の値を伝える。
  画像は添付した枚数と上限・形式を説明（`aria-describedby`）にし、公開範囲は短い名前を見せて「公開範囲: <名前>」を名前、その範囲を見られる人を説明にし、言語は `Select` の `toolbar` の形で選んだ言語を見せる（名前は言語とラベル）。
  CW の入力欄は本文の上に、画像の上限と形式の文は添付した画像の下に出す。
- **カスタム絵文字のピッカー**（`features/emoji`、ADR-0051 §4）: `GET /custom_emojis` がピッカーに出す絵文字を返すときだけ、道具の行に絵文字のボタンを出す（投稿の編集のダイアログと、プロフィールの編集の表示名・自己紹介も同じ `EmojiPickerButton`）。
  ボタンはポップオーバー（APG の dialog）に `@my/ui` の `EmojiPicker` を開き、検索欄にフォーカスを移す。`EmojiPicker` は検索欄と、カテゴリごとの見出しで分けた絵文字のグリッド（React Aria の `GridList` の grid の配置。APG の grid のとおり、Tab で検索欄から入り、矢印キーで書字方向に沿って動き、Enter・Space・押下で選ぶ）で、
  検索は shortcode と別名の部分一致、各絵文字の名前は `:shortcode:` にする。選ぶと、対象の入力欄のカーソルの位置（選択していればその範囲）に `:shortcode:` を入れ（サーバーが前後に英数字とコロンのない shortcode だけを読むので、必要なら前後に空白を足す）、ポップオーバーを閉じて入力欄にフォーカスを戻す。
  最近使った絵文字は端末ごとに localStorage（`my:emoji:recent`、shortcode の配列）に 16 個まで持ち、検索していないときに「最近使った絵文字」として先頭に出す。動く絵文字は利用者の設定と `prefers-reduced-motion` に従って止める（`CustomEmoji` と同じ）。リアクション（ADR-0052）も同じ `EmojiPicker` を、Unicode 絵文字の群を足して使う（§13）。
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

`features/status` が投稿のカード（`StatusCard`）と操作、投稿の詳細と編集履歴の画面を受け持つ（FR-106〜109、FR-113、FR-116）。タイムラインなどの一覧は、このカードを `index.ts` から使う。

- **アドレス**: 投稿の詳細は `/@{acct}/status/{id}`（`routes/@{$acct}_.status.$id.tsx`、ADR-0053）。`acct` はローカルのアカウントではユーザー名、リモートでは `username@domain` で、読みやすさのためだけにあり、投稿は ID で引く。
  `_` で、プロフィールの画面（`/@{acct}`）のレイアウトの下に入らないようにする。カードの時刻はこのアドレスへのリンクになる（ブーストはブーストされた投稿へ）。
- **画面**: 見出しは `h1`「投稿」、その下に返信先の投稿（`h2`「スレッドの前の投稿」、古い順）、投稿そのもの（焦点の投稿）、返信（`h2`「返信」、深さ優先）を並べる。
  - 焦点の投稿は `PostCard` の `highlighted`（`surface-raised` の面と、行頭側の `accent` の線）で区別し、本文を `text-lg` にする。
    `article` の名前は、見えない `label`「開いている投稿」で始め、作者・時刻が続く（スクリーンリーダーと ARIA スナップショットで、スレッドのほかの投稿と名前で区別できる）。
    頭の時刻はほかのカードと同じ相対時刻（G8）で、自分へのリンクはない。本文と操作の列の間の `details` に、省略しない日時（`<time>`）と、0 でない操作の数の文言（「ブースト 3 件」「お気に入り 12 件」）を並べる。
  - 返信のつながりは、アバターの列を通る線（`PostCard` の `lineAbove` / `lineBelow`）で示す。並んだ 2 つの投稿の下の方が上の方への返信（`in_reply_to_id`）のときだけ線でつなぐ。
    つながる先が下にあるカードは、狭い幅でも本文をアバターの列の外に置き、線が本文と重ならないようにする。線は飾りで、スレッドの構造は見出しと順序が伝える。
  - スレッドの前の投稿が読み込まれて焦点の投稿の上に入ったら、焦点の投稿を 1 回だけ画面の上端（ページの下に残りが少なければ、届くところ）までスクロールする（描画の前、`useLayoutEffect`。`features/status` の `useRevealOnce`）。
    ルーターのスクロールの復元は同じコミットの後（`onRendered`）で新しい画面を上端に戻すので、そのときはその後にもう一度スクロールする。戻るで復元した位置は動かさない。
    フォーカスは `h1` のまま動かさない（外枠は `h1` に `preventScroll` でフォーカスを移すので、このスクロールを戻さない）。
  ローダーは投稿を待ち、スレッド（`GET /statuses/{id}/context`）は待たずに読み込み始める。スレッドの失敗は返信の区画に理由と「もう一度試す」を出し、投稿は表示したままにする（E1）。
  見えない投稿と存在しない投稿はどちらも 404 で、ルートの `errorComponent` が「投稿が見つかりません」を出す。
- **操作**: 返信（`useReply()`、§12）、ブースト・お気に入り・ブックマークのトグル（`@my/ui` の `ToggleButton`。名前は状態で変えず `aria-pressed` で示し、数は説明にする。オンのアイコンは塗りつぶしで、色だけに頼らない）、
  リアクションの選択（後述）、「その他の操作」のメニューの「リンクをコピー」と、自分の投稿だけの「編集」「プロフィールに固定」「削除」。サインインしていない間は操作を出さない。公開範囲がブーストを許さない投稿（他人のフォロワー限定・ダイレクト）は、ブーストのボタンを無効にする。
- **絵文字リアクション**（FR-116、ADR-0052 §5）: 操作の列の「リアクションする」（アイコンのボタンとツールチップ）が、ポップオーバーに `@my/ui` の `EmojiPicker` を開く（`features/emoji` の `ReactionPickerButton`）。
  よく使う Unicode 絵文字の群（`EmojiPicker` の `unicode`。名前は絵文字そのもの）と、投稿フォームと同じこのサーバーのカスタム絵文字を出し、選ぶとポップオーバーを閉じてその絵文字でリアクションする。
  投稿の下（`PostCard` の `reactions`、本文と操作の列の間）に、絵文字ごとの数を「リアクション」の `group` に並べる。Unicode とこのサーバーのカスタム絵文字は `ToggleButton`（名前は「{emoji} でリアクション」、数は説明、自分のものは `aria-pressed`）で、押すとその絵文字で加わり、もう一度押すと外す。
  ほかのサーバーのカスタム絵文字は、ボタンにせず絵文字と数だけを出す（ADR-0052 §2）。列の終わりの「リアクションした人」は、付けた人の一覧（`GET /statuses/{id}/reactions`、新しい順、各行はプロフィールへのリンクと絵文字。続きは「さらに表示」のボタン）をダイアログで開く。
  お気に入りとリアクションは 1 つの枠（ADR-0052 §1）なので、どちらの操作も `useLikeSlot` の 1 つの mutation の `scope` で順に送り、押した時点でお気に入り・自分のリアクション・それぞれの数をすべての写しで入れ替え、失敗したら押す前の枠に戻してトーストで理由を知らせる。
- **ハンドル**: アカウントのハンドルは `@{acct}`（`lib/handle.ts` の `handleOf`）で出す。`acct` はこのサーバーのアカウントではユーザー名なので `@username`、ほかのサーバーのアカウントでは `@username@domain` になる。
  ローカルかどうかは API の `acct` で判断し（`isRemoteAcct`）、ドメインを文字列で比べない。タイムライン・投稿の詳細・通知・プロフィール・フォローの一覧・設定・サイドバーで同じにする。
- **ほかのサーバーのアカウントと投稿**（`acct` にドメインがあるもの）: 名前の横のハンドルは `@username@domain` で出す。カードの操作の列の末尾に、元のサーバーの投稿のページ（`url`、なければ `uri`）へのリンクを置く。
  リンクはホスト名だけを見せ、アクセシブルネームはそのホスト名を含む文（「{host} で元の投稿を開く」）にする。サインインしていない間も出す。
  プロフィールは各タブの投稿の一覧の上に「このサーバーが受け取った投稿だけを表示しています」と、元のサーバーのプロフィール（`Account.url`）へのリンク「{host} で見る」を置き、投稿が無いタブでは空の表示の説明と操作に同じものを出す（`TimelineFeed` の `origin`、ADR-0044）。
  どちらのリンクも外部へのリンクのアイコン（`@my/ui` の `ExternalLinkIcon`）を付け、`rel="noreferrer"` で開く。
  タブの最初のページが `remote_fetch_started` を返したら（client-api.md）、一覧の上に「元のサーバーから最近の投稿を取得しています」を出して `useAnnouncer` の polite の領域で 1 回知らせ（Y7。表示はページと同時に現れるので、それ自体はライブリージョンにしない）、
  4 秒後にそのアカウントの投稿の一覧に無効化の印を付けて表示中のタブを 1 回だけ取り直し、取り直しの成否にかかわらずキャッシュの最初のページから `remote_fetch_started` を外して表示を消す（`timeline/hooks/useRemoteFetch`）。失敗は知らせない。
  画像は `preview_url` で表示する。アップロードでは `small` の静止版、リモートの添付ではメディアプロキシの URL で、リモートの添付は `variants` を持たない。
  アップロードの画像・アバター・ヘッダーは `variants` を `@my/ui` の `Picture`（`lib/images` の `imageSources`）に渡し、表示する大きさに合う版をブラウザーが `srcset` / `sizes` で選ぶ。
  アニメーションの動く版は `<source media="(prefers-reduced-motion: no-preference)">` にだけ置き、設定「アニメーションを自動で再生する」（このブラウザーの localStorage `my:display:autoplay`、`lib/autoplay`、既定はオン）がオフなら、ポインタを重ねるか触れている間だけ動く版にする（ADR-0054 §3）。説明のない画像には「説明なし」のバッジが付く（Y9）。CW はローカルの投稿と同じ disclosure で出す。
  センシティブな画像を隠している間と、画像の読み込み中は、添付の `blurhash` のぼかし（`@my/ui` の `Blurhash`。自前の復号器が 32 × 32 ピクセルに復号して `<canvas>` に描き、飾りとして `aria-hidden`）をそのマスに出し、読み込み終えたら外す。`blurhash` のない添付と復号できないものは、マスの地の面（`bg-surface-sunken`）が代わりになる。
- **楽観的更新**（S8）: 同じ投稿がタイムラインのページ・ブースト・スレッド・投稿の query に同時にあるので、`api/cache.ts` がキャッシュの答えを形（`Status`・`items`・`pages`・`ancestors` / `descendants`）で見分け、すべての写しを書き換える。
  トグルは押した時点で状態と数を変え、失敗したらその 1 回分だけ戻してトーストで理由を知らせる。同じ投稿の同じ操作は TanStack Query の mutation の `scope` で 1 つずつ順に送り、最後に押したものが残る。
  成功の応答の数（他の人の操作を含む）は、その操作の最後の要求の応答からだけ取り込む。結果は `useAnnouncer` で知らせる。
- **ブックマークの一覧**（`/bookmarks`、ナビゲーションの「ブックマーク」、FR-113）: サインインが要る画面で、ブックマークした新しい順の投稿をタイムラインと同じ feed（`features/timeline` の `TimelineFeed`）で出す。
  ブックマークの切り替えは一覧に無効化の印を付けるだけで（`refetchType: "none"`）、表示中の一覧は取り直さない。一覧で外した投稿は外した状態のまま残ってフォーカスを保ち、押し直せば戻せる。次に一覧を開いたときに消える。
- **削除**: `alertdialog` で確かめ（最初のフォーカスは「キャンセル」）、サーバーが消してから、投稿とそのブーストをすべての一覧から除き、投稿とスレッドの query を消し、返信先の返信数を 1 減らす。
  詳細の画面で消したときは前の画面に戻る（戻る先がなければホーム）。失敗したらダイアログを残してトーストで理由を知らせる。
- **編集**（FR-109）: モーダルのダイアログ（`components/EditStatusDialog.tsx`）で、投稿フォーム（§12）の部品（本文の `TextArea` と文字数、CW のトグルと入力欄、画像の代替テキストの一覧 `AttachmentList`、センシティブのチェックボックス、言語の `Select`）を使う。
  投稿フォームの下書きは共有して保存されるので、編集はそこに入れず、ダイアログの中だけの state にする（閉じると消え、書きかけの新しい投稿を上書きしない）。
  開くたびに `GET /statuses/{id}/source` で作者が書いた文を読み直し、本文にフォーカスを移す。読めなければ理由と「もう一度試す」を出す。
  公開範囲と返信先は変えられない（API）ので、その旨を書く。画像は足し外しせず、代替テキストだけを変え、送るときはすべての画像を順に送る（省くと外れる）。
  「保存する」は `PUT /statuses/{id}` を送り、サーバーの答えを待つ。成功したら答えですべての写しを置き換え（`cacheEditedStatus`。一覧の中の位置は変えない）、履歴に無効化の印を付け、トーストで知らせて閉じる。失敗はダイアログの中に `role="alert"` で出し、入力を残す（E2）。
- **編集済みと編集履歴**: `edited_at` のある投稿は、本文と操作の列の間（`details`）に「編集済み」のリンクを出し、編集履歴の画面（`/@{acct}/status/{id}/history`、`routes/@{$acct}_.status.$id_.history.tsx`）へ移る。
  画面は `h1`「編集履歴」、投稿に戻るリンク、版の一覧（`GET /statuses/{id}/history` を新しい順）にする。各版は、作った日時の `h2` が名前を与える `article` で、現在の版か最初の投稿か、前の版から変わったもの（本文・CW・センシティブの指定・添付）、CW と本文と画像をカードと同じ部品で出す。
- **固定**（FR-403）: 自分のダイレクト以外の投稿（サーバーが固定を受け付けるもの）の「その他の操作」のメニューに、「プロフィールに固定」か、固定済みなら「プロフィールへの固定を外す」を出す。
  確かめずに送り、サーバーが答えてから、投稿のすべての写しの `pinned` を応答のものにし、閲覧者の固定された投稿の一覧（`GET /accounts/{id}/statuses?pinned=true`）を無効にして、表示中なら取り直してから結果をトーストで知らせる。
  失敗（固定できる件数の上限の 422 など）はサーバーの理由をトーストで知らせ、何も変えない（E2）。同じ投稿の固定と外すは mutation の `scope` で 1 つずつ順に送る。
  プロフィールの「投稿」のタブは、ルートの loader がそのアカウントの固定された投稿も読み、feed の先頭に固定された新しい順で並べる（同じ投稿は下のタイムラインにも出る）。固定された投稿のカードは、行頭の文脈の行「固定された投稿」を持ち、`article` の名前もそれで始まる。
  ほかのタブには出さない。ほかのサーバーのアカウントでは、このサーバーが知っている固定された投稿だけが出る。
- **テスト**: `features/status/status.test.tsx` が画面・楽観的更新と失敗時の巻き戻し・順に送ること・複数の写しへの反映・削除を MSW で検査し、`api/cache.test.ts` がキャッシュの書き換えを単体で検査する。
  `features/status/edit.test.tsx` は、自分の投稿だけに「編集」があること、作者が書いた文から始まり、変えたものを送って、一覧の位置のまま「編集済み」になること、保存と読み込みの失敗、空の本文、編集履歴の版の並び・見出し・変わったもの・404 を検査する。
  E2E（`e2e/status-edit.spec.ts`）は、編集のダイアログと編集履歴の画面の axe・横スクロール・ARIA スナップショットと、ダイアログで保存した編集が投稿に出るまでを検査する。
  `features/status/reactions.test.tsx` がリアクションの列・押して加わる / 外すこと・お気に入りとの入れ替え・ピッカー・失敗時の巻き戻し・付けた人の一覧を MSW で検査し、E2E（`e2e/reactions.spec.ts`）はリアクションの列・ピッカー・付けた人のダイアログの axe・横スクロールを検査する。
  `features/status/pin.test.tsx` が固定と外すこと・上限の失敗・出す投稿の範囲・プロフィールの先頭と出すタブを MSW で検査し、E2E（`e2e/pins.spec.ts`）はプロフィールの先頭の固定された投稿の axe・横スクロールと、キーボードでメニューから固定するまでを検査する。
  E2E（`e2e/status.spec.ts`）は、サインイン済みの状態でスレッドの ARIA スナップショット・焦点の投稿が最初の画面に入ること・axe・横スクロール・失敗したお気に入りの巻き戻し・メニューと削除の確認を検査する。

## 14. 管理画面

スタッフの画面は、モデレーションとサーバー管理の 2 つの区分に分ける（[ADR-0058](../adr/0058-moderation-admin-separation.md)）。1 つの画面は 1 つの区分の経路だけを持つ。

- **モデレーション**（`/moderation/*`、`features/moderation`）: 通報・異議申し立ての審査・アカウント・登録の審査・登録の拒否リスト・招待・ドメインへの処分・連合先のサーバーの状態（[federation.md §7.3](federation.md)）・監査ログ。
  `features/moderation` は、利用者が自分への処分を確かめる画面（§18）も受け持ち、サーバー管理の画面が使う共通の部品（下の「共通の部品」）と権限の型・一覧のページングも `index.ts` から出す。
- **サーバー管理**（`/admin/*`、`features/admin`）: サーバーの設定・ルール・テーマ・規約とポリシー・カスタム絵文字・dead job。これから足すサーバー管理の画面（お知らせ・webhook・リレー）は、
  `routes/admin.<画面>.tsx` のルートと、`features/admin/components/AdminMenu.tsx` の一覧の 1 行を足してここに置く。

API の経路はどちらも `/admin/*` で（[client-api.md](client-api.md) の管理の操作、タグ `admin-moderation` / `admin-server`）、Web の経路とは一致しない。

- **出し分け**: 閲覧者の `CredentialAccount.permissions` で、ナビゲーションの「モデレーション」と「管理」（§9）・各区分の画面（`/moderation`・`/admin`）・設定の画面の「モデレーション」と「管理」の区画と各画面を出す。`manage_domains` が連合先とドメインへの処分、`manage_jobs` が dead job、`view_audit` が監査ログ、`manage_accounts` が登録の審査と登録の拒否リストと異議申し立ての審査とアカウント、`manage_invites` が招待、`manage_reports` が通報の画面、`manage_rules` がルールの画面、`manage_roles` がアカウントの管理画面のロールの区画、`manage_settings` が規約とポリシーの画面とサーバーの設定とテーマの画面、`manage_custom_emojis` がカスタム絵文字の画面に要る。
  ルートは `requireSignIn` のあと、loader で閲覧者を読み、権限があるときだけ一覧を読み込む（`routes/-admin/adminRoute.tsx`）。権限がなければ、要求を送らずに理由（権限の名前）を出す。
  守るのはサーバーで、権限が途中で外れて 403 `missing-permission` が返ったときは、ルートの `errorComponent` が同じ理由を出す。
- **モデレーターへの通知のメール**（設定の「表示と投稿」、`features/settings` の `useModeratorEmail`）: モデレーターへの通知が届く権限（`manage_reports` か `manage_accounts`）を持つ閲覧者にだけ、
  「モデレーターへの通知」の区画（`h2` の `section`）を出し、`email_moderator_notifications` の `Switch` を置く。ほかの設定と同じく、変えたとたんに保存する。
  `GET /instance` の `sends_email` が `false`（サーバーの `mail = none`）なら、切り替えの代わりに、通知がアプリの中にだけ届く理由を出す。
- **区分の画面**（`/moderation` はナビゲーションの「モデレーション」、`/admin` は「管理」）: 閲覧者のロールが使えるその区分の画面へのリンクを、設定の画面のその区分の区画と同じ一覧で並べる
  （モデレーションは `ModerationMenu` / `ModerationPages`、サーバー管理は `AdminMenu` / `AdminPages`。どちらも `features/moderation` の `StaffMenuSection` / `StaffPages` で組む）。
  設定の画面は「モデレーション」「管理」の順に 2 つの区画（`h2` の `section`）を出し、使える画面のない区分の区画は出さない。
  通報（`/moderation/reports`）・登録の審査・異議申し立て（`/moderation/appeals`）のリンクは、待っている件数（未解決の通報・承認待ちの登録・審査待ちの異議申し立て）の `Badge` を持ち、その文言（「承認待ち 1 件」）はリンクの名前に入る。使える画面がないロールには、その理由を `EmptyState` で出す。
- **連合先**（`/moderation/instances`）: 状態のタブ（`?state=unavailable` / `available`、URL に置く。S3）と、ドメイン順の一覧。各サーバーは見出し（ドメインへのリンク）と、状態・その状態になった日時・失敗した日数・最後に失敗した日（UTC の日付）の説明リストにする。状態は文言で示し、色はそれを繰り返すだけにする。
- **連合先の詳細**（`/moderation/instances/{domain}`）: 状態と、最新の `instance_availability_events` の理由、最後に配送を再開した日時（最新の `marked_available`）、新しい順の履歴。
  データモデルは成功した配送の日時を持たない（`instance_health` は失敗の間だけ更新する。[data-model §4.8](data-model.md#48-federation)）ので、「最後の成功」は配送の再開の日時で示す。
  失敗の日数があるか停止中なら「配送を再開する」を出し、成功したら詳細の答えを置き換え、一覧と履歴を無効にして結果を知らせる。失敗はトーストで知らせ、状態とボタンを残す（E2）。
  「モデレーション」の区画から、そのドメインのドメインへの処分の画面（`/moderation/domain-actions?domain=…`）へリンクする。
- **ドメインへの処分**（`/moderation/domain-actions`、FR-702）: 新しい処分のフォームと、新しい順の処分の一覧の 2 つの区画。一覧は「有効」（`active=true`）と「すべて」のタブ（`?filter=all`、URL に置く。S3）で、
  `?domain=` があればそのドメインの処分だけを出し、フォームのドメインにも入れ、「すべてのドメインを表示」のリンクで外せる。各処分は見出し（ドメイン）と、種類・状態・処分した日時・終了（なければ「解除するまで」）・解除した日時・公開の一覧での伏せ字の説明リスト、公開コメントと非公開のメモにする。
  - **ドメインの 2 つの形**: サーバーはドメインを IDNA の A-label（`xn--r8jz45g.jp`）で記録して返す。画面は `lib/idn.ts`（RFC 3492 の Punycode の復号。表示専用）で U-label（`例え.jp`）に戻し、違うときは A-label を括弧で後に添える（`features/admin` の `DomainName`、連合先の一覧と詳細の見出しも同じ）。
    似た字形の別の文字のドメインを見分けられるようにするためである。文の中では U+2066 / U+2069 で囲む。フォームは U-label のまま送り、A-label への変換はサーバーが行う。
  - **フォーム**: ドメイン（必須）、種類（`RadioGroup`、必須、既定の選択なし、軽いものから `suspend` まで、それぞれの効果の説明付き）、期間（`DurationSelect`、1・7・30・90・180・365 日と「解除するまで」）、公開コメントと非公開のメモ（`ReasonField`、5,000 文字）、伏せ字（`Checkbox`）。
    ブラウザの制約検証が通ってから送る。`suspend` だけは送る前に `ConfirmDialog` で確かめ（NFR-MOD-7）、失敗はダイアログの中に出す。ほかの種類はすぐに送り、失敗はフォームの下（`role="alert"`）に出して入力を残す。
    成功したらドメインを残して入力を空にし、一覧を無効にして、結果をフォームの `StatusMessage` に出す。
  - **解除**: 有効な処分の「解除」（アクセシブルネームに種類・ドメイン・処分した日時を含める）は確かめずに送る。成功したら、キャッシュの「有効」の一覧から除いてフォーカスを次の行のボタンに移し（`lib/useFocusAfterRemoval.ts`）、
    「すべて」の一覧では解除済みに置き換えてフォーカスをその行の見出しに移す。結果は一覧の区画の `StatusMessage` に出し、失敗はトーストで知らせる（E2）。
- **登録の拒否リスト**（`/moderation/signup-blocks`、FR-719）: 新しい拒否のフォーム、一覧からの追加、新しい順の拒否の一覧の 3 つの区画。一覧は「有効」（`active=true`）と「すべて」のタブ（`?filter=all`、URL に置く。S3）で、
  各拒否は見出し（対象の値）と、種類・効果・状態・追加した日時・終了（なければ「解除するまで」）・解除した日時・追加のしかた（単独か BAN とともにか）の説明リスト、メモにする。
  メールのドメインはドメインへの処分と同じく `DomainName` で 2 つの形を出し、フォームは U-label のまま送る。メールアドレスの拒否はサーバーがハッシュだけを持つので、見出しはそのハッシュ（16 進の SHA-256）になる。
  - **フォーム**: 種類（`RadioGroup`、既定はメールのドメイン。メールのドメイン・メールアドレス・IP アドレスの範囲）、値（必須。名前・説明・空のときの文言は種類で変わる）、効果（`RadioGroup`、必須、既定の選択なし。承認制にする・拒否する）、
    期間（`DurationSelect`、1・7・30・90・180・365 日と「解除するまで」）、メモ（`ReasonField`、1,000 文字）。拒否はすぐに効き、同じ画面で解除できるので確かめずに送る。
    値の形式（ドメイン・メールアドレス・CIDR）はサーバーが確かめ、断られたら理由をフォームの下（`role="alert"`）に出して入力を残す。成功したら種類を残して入力を空にし、一覧と監査ログを取り直して、結果をフォームの `StatusMessage` に出す。
  - **解除**: ドメインへの処分の解除と同じ（アクセシブルネームに種類・値・追加した日時を含め、確かめずに送り、「有効」では次の行のボタン、「すべて」では行の見出しにフォーカスを移す。結果は一覧の区画の `StatusMessage`、失敗はトースト）。
  - **一覧からの追加**（`POST /admin/signup_blocks/import`）: 使い捨てのメールアドレスのドメインの公開リストのような一覧の値ごとに、同じ種類・効果・期間・メモの拒否を追加する。フォームは種類・一覧・効果・期間・メモ（新しい拒否のフォームと同じ選択肢）で、
    一覧は `TextArea` に貼り付けるか、「テキストファイルを選ぶ」（React Aria の `FileTrigger`、テキストと CSV）で読み込む。読み込んだファイルの内容は一覧を置き換え、ファイル名と値の数を `StatusMessage` に出す。1 MB を超えるファイルは読まずに `role="alert"` で断る。
    値の数はサーバーと同じ数え方（空の行と `#` で始まる行を除く）で求め、値がない・10,000 件を超える・1 MB を超える一覧は、`validate` で一覧の欄のエラーにして送らない（サーバーが読む本文は 1 MiB まで）。拒否はすぐに効き、下の一覧で解除できるので確かめずに送る。
    サーバーの失敗はフォームの下（`role="alert"`）に出して入力を残す。成功したら種類・効果・期間を残して一覧とメモを空にし、拒否の一覧と監査ログを取り直してから、追加した件数と追加しなかった行の数をフォームの `StatusMessage` に出す。
    追加しなかった行は、見出し（`h3`「追加しなかった行」）・理由ごとの行数の説明リスト・その見出しが名前を与える表（行番号・一覧に書かれたままの値（`dir="auto"`）・理由（`invalid`・`duplicate`・`already_blocked` を文言にしたもの））で、行の順に出す。
    表は最初の 100 行だけを出し、残りは「すべて表示する」で出す。ボタンは消えるので、フォーカスを表の見出しに移す。次の追加では表を最初の 100 行に戻す（`key`、R10）。
- **カスタム絵文字**（`/admin/custom-emojis`、`manage_custom_emojis`、FR-111、[ADR-0051](../adr/0051-custom-emoji.md)）: 「カスタム絵文字を登録する」と「登録されたカスタム絵文字」の 2 つの区画（`h2` の `section`）。
  - **登録**: 画像（React Aria の `FileTrigger`、PNG・GIF・WebP）・shortcode（必須、`[A-Za-z0-9_]{2,30}` をブラウザの制約検証で確かめる）・カテゴリ（64 文字まで）・別名（空白かカンマで区切り、10 個まで）・ピッカーに出すか（`Checkbox`、既定は出す）のフォーム。
    100 KB を超える画像には、GoToSocial などが表示しないことがある旨を出す（ADR-0051 §2。登録は止めない）。送ると、画像を `POST /media`（`purpose=emoji`）で送って処理が終わるまで `GET /media/{id}` をポーリングし、`POST /admin/custom_emojis` で登録する。
    サーバーの拒否（422 `invalid-custom-emoji`、`reason` に理由）は `detail` をフォームの下（`role="alert"`）に出して入力を残す。成功したら入力を空にし、一覧・ピッカーの絵文字（`GET /custom_emojis`）・監査ログに無効化の印を付けてから、結果をフォームの `StatusMessage` に出す。
  - **一覧**: 「このサーバー」と「ほかのサーバー」のタブ（`?origin=remote`、URL に置く。S3）と、shortcode（ほかのサーバーはドメインも）の検索（`?shortcode=`・`?domain=`、送ると URL を変える）で、新しい順。各絵文字は画像と見出し（`:shortcode:`、`dir="ltr"`）と、ドメイン・状態・ピッカー・カテゴリ・別名・画像を設定した日時の説明リストにする。
    このサーバーの絵文字は「編集する」（カテゴリ・別名・ピッカーに出すか・有効にするかのダイアログ。保存したら閉じ、失敗はダイアログの中に出す）・「画像を差し替える」（`FileTrigger` で選ぶとすぐに送る）・「削除する」（元に戻せないので `ConfirmDialog` で確かめる）、ほかのサーバーの絵文字は「無効にする」「有効にする」を持つ。
    ボタンのアクセシブルネームは絵文字（`:shortcode:`、ほかのサーバーは `@domain` を付け、U+2066 / U+2069 で囲む）を含める。変えた絵文字は応答で行を置き換え、消した絵文字は一覧から除いてフォーカスを次の行のボタンに移す（`lib/useFocusAfterRemoval.ts`）。
    結果は一覧の上の `StatusMessage` に出し、失敗は同じ場所（`role="alert"`）に出す。どの変更も、ピッカーの絵文字と監査ログに無効化の印を付ける。
- **dead job**（`/admin/dead-jobs`）: 失敗した新しい順の一覧。各ジョブは型の見出し、失敗と作成の日時、試行回数、冪等キーと順序キー、最後のエラー、ペイロード（JSON の文字列のまま）を出す。型・キー・ペイロードは `dir="ltr"` で表示する。
  「再実行」はジョブの型と失敗の日時をアクセシブルネームに含める。成功したジョブはキャッシュの一覧から除き、フォーカスを次の行のボタン（なければ見出し）に移す（`lib/useFocusAfterRemoval.ts`）。
  複数のジョブを同時に再実行できるので、成功後の処理は `mutate` ではなく `useMutation` 自身のコールバックに書き、送信中のジョブは `useMutationState` で求める。
- **監査ログ**（`/moderation/audit-log`、FR-704）: 「絞り込み」と「操作」の 2 つの区画。操作は新しい順で、各行は操作の種類の見出しと、操作者・対象・日時の説明リストにする。
  - **絞り込み**: 操作の種類（`?action=`）と対象の種類（`?target_kind=`）は `Select`（APG の select-only combobox）で選び、変えるとすぐに URL を変える（S3）。対象の種類を変えると 1 つの対象の絞り込みは外れる。
    操作者（`?actor=`、アカウントの ID）と 1 つの対象（`?target=`、`target_kind` と組でだけ効く）は、行のリンク（「{名前}の操作だけを表示」「{対象}への操作だけを表示」）で選ぶ。
    絞り込んだ行はそのまま残るので、同じ位置のリンクを「すべての操作者の操作を表示」「すべての対象への操作を表示」に変えてフォーカスを保つ。
    絞り込みの区画に、選んだ操作者と対象（読み込んだ行にある名前）とそれを外すリンク、「絞り込みを外す」を出す。外すリンクが消えたときは、区画の最初の `Select` にフォーカスを移す。
  - **ページの読み足し**: 読み込んだページ数を `?pages=`（2〜25）に置く。「さらに表示」で次のページが届いたら、履歴を増やさずスクロールも動かさずに（`replace`・`resetScroll: false`）URL を変える。
    ルートの loader は TanStack Query の `infiniteQuery` の `pages` で、そのページ数を新しい方から順に読む。キャッシュに読み込み済みのページはそのまま使う。絞り込みを変えると 1 ページ目からになる。
    次のページの失敗は、一覧の下に `RegionError` で理由を出し、読み込み済みの行を残す。もう一度「さらに表示」を押すと読み直す。
  - **操作者**: アカウントの表示名（`dir="auto"`）とハンドル。アカウントが消えた利用者は「削除されたアカウント」、CLI・サーバー自身・ほかのサーバーはそれぞれの名前にする。
  - **対象**: 項目名を対象の種類にし、値は記録時の `target_label` にする。対象の種類に管理画面があるものはそこへリンクする（アカウントは `/moderation/accounts/{id}`、通報は `/moderation/reports/{id}`、ドメインは `/moderation/domain-actions?filter=all&domain=…`、登録の拒否は `/moderation/signup-blocks?filter=all`、カスタム絵文字は `target_label`（`:shortcode:` か `:shortcode:@domain`）の shortcode とドメインで検索した `/admin/custom-emojis`、ドメインは `DomainName` で 2 つの形を出す）。
    設定・ルール・登録など画面のない種類と、消えた対象（API の `target_exists` が `false`）は、リンクにせず `target_label` だけを出す。
- **招待**（`/moderation/invites`、`manage_invites`、FR-001）: 「招待を作成する」と「招待の一覧」の区画。登録モードが `invite` でない（`GET /instance` の `registrations.mode`）ときは、招待では誰も登録できず、登録モードは管理者がサーバーの設定で変えることを、区画の前に書く。
  - **作成**: 登録できる人数（`Select`、1・5・10・25・50・100 人と制限なし。既定は 1 人）と有効期間（`DurationSelect`、30 分〜30 日と無期限。既定は 7 日）。CLI の `my invite create` と同じ既定値にする。確かめずに送り、成功したら一覧を取り直してから、
    コードを `StatusMessage` に、登録画面のリンク（`dir="ltr"`）をその下に出す。失敗はフォームの下（`role="alert"`）に出す。
  - **一覧**: 新しい順で、各招待はコードの見出し（`h3`、`dir="ltr"`）と、状態（使える・失効・期限切れ・使い切り）・登録した人数（上限があれば「{上限} 人中 {人数} 人」）・期限・作成日時の説明リスト。
    使える招待には「リンクをコピー」（クリップボードに書き、結果をトーストで知らせる）と「失効」（アクセシブルネームにコードと作成日時）を出す。失効は確かめずに送り、応答で行を置き換えて、ボタンが消えるのでフォーカスを行の見出しに移し（`useFocusHeadingAfterLift`）、結果を一覧の上の `StatusMessage` に出す。失敗はトーストで知らせる。
- **登録の審査**（`/moderation/registrations`、FR-721）: 承認を待つ登録の、待っている長い順の一覧。各登録は見出し（`@username`。アカウントがなければその旨）と、登録日時・メールアドレス・メールの確認・招待の説明リスト、登録の理由にする。
  理由は申請者が書いた文なので `dir="auto"` で表示する（Y10）。「承認する」「却下する」はアクセシブルネームに `@username` を含める。
  決定は後から変えられないので、どちらも `ConfirmDialog` で確かめる（NFR-MOD-7。却下は `danger`、承認は `primary`）。ダイアログはモデレーター向けのメモ（`ReasonField`、任意、1,000 文字）を持ち、開くとメモにフォーカスが移る。失敗はダイアログの中に出す。
  成功したらダイアログを閉じ、キャッシュの一覧から除いてフォーカスを次の行のボタン（なければ見出し）に移し（`lib/useFocusAfterRemoval.ts`）、結果を一覧の上の `StatusMessage` に出す。
  ほかのモデレーターが先に決めた登録は、サーバーがその決定のまま返すので、一覧から除き、どちらの決定が残ったかを知らせる。
- **異議申し立ての一覧**（`/moderation/appeals`、FR-707）: 「審査待ち」と「審査済み」のタブ（`?state=decided`、URL に置く。S3）で、新しい順。各申し立ては見出し（「{処分の種類}への異議申し立て」、詳細へのリンク）と、申し立てたアカウント・申し立てた日時・処分の状態（審査済みは決定と審査した日時も）の説明リスト、本人が書いた文（`dir="auto"`、2 行に省略）にする。
  API の絞り込みは 1 つの状態だけなので、「審査済み」は状態を指定せずに読んで審査待ちを除く。審査待ちだけのページは、審査済みが 1 件見つかるか一覧が終わるまで次のページを続けて読み、続きがあるのに空のページを出さない。
  申し立ては `account_id` しか持たないので、アカウントの名前は `GET /accounts/{id}` で出す。
- **異議申し立ての審査**（`/moderation/appeals/{id}`、FR-707）: 申し立て（状態・アカウントと管理画面へのリンク・日時・本人が書いた文）と、その処分（種類・状態・理由の種類・日時・期限・解除した日時・投稿の数・通報へのリンク・公開の理由と非公開のメモ）と、審査の区画を `h2` の `section` で並べる。
  審査待ちなら「維持する」「撤回する」を出す。決定は後から変えられないので、どちらも `ConfirmDialog` で確かめ（NFR-MOD-7。撤回は処分を解除するので `danger`、BAN の撤回は削除の取り消しも本文で示す）、ダイアログは本人に見せる説明（`ReasonField`、任意、2,000 文字）を持つ。失敗はダイアログの中に出す。
  成功したら応答の申し立て（撤回なら解除された処分を含む）でキャッシュを置き換え、一覧・件数・アカウントの処分の履歴を無効にする。処分の区画の状態は「解除」に変わり、審査の区画はボタンの代わりに決定・日時・審査した人・本人への説明を出して、その見出しにフォーカスを移し、結果を区画の `StatusMessage` に出す。
  ほかのモデレーターが先に逆の決定をした（409 `appeal-decided`）ときは、ダイアログを閉じて申し立てを読み直し、その決定が残ったことを知らせる。
- **サーバーの設定**（`/admin/settings`、`manage_settings`、FR-705）: 「名前」「説明」の区画と、設定のフォーム（「登録」「言語」「タイムライン」「連合」「制限・凍結したサーバー」の区画と保存のボタン）を `h2` の `section` で並べる。
  「名前」の区画に、アプリの言語（疑似ロケールを除くカタログのある言語）と、名前が保存されているほかの言語ごとに、名前の `TextField`（「{言語}での名前」、言語名は `Intl.DisplayNames`）を並べる。
  説明に、名前がドメインの代わりに出る場所と、名前のない言語・どの言語にも名前がないときに何が出るか、ハンドルとリンクはドメインのままであることを書く（[名前とドメイン](../operations/instance-settings.md#名前とドメイン)）。
  101 文字以上と空白だけの名前はフォームの中で無効にし、送らない。「保存する」は変えた言語だけを `PUT /admin/instance_texts/name/{locale}` で送り（空は消す）、規約とポリシーと同じく `GET /admin/instance_texts`・`GET /instance`・監査ログに無効化の印を付け、表示中の `GET /instance` を取り直してから、結果を区画の `StatusMessage` に出す。
  外枠の名前（「サーバーの名前」）はこの取り直しで変わる。変えた言語がなければ何も送らずにその旨を出す。失敗はボタンの上（`role="alert"`）に出し、入力を残す（E2）。
  - **説明**: 「名前」と同じ言語ごとに、短い説明（500 文字）と説明（20,000 文字）の `TextArea`（文字数の表示付き。「{言語}での短い説明」「{言語}での説明」）を並べる。上限を超えた欄があれば送らず、
    「説明を保存する」は変えた言語とキーだけを `PUT /admin/instance_texts/{key}/{locale}` で送り、名前と同じく取り直してから区画の `StatusMessage` に出す。失敗はボタンの上（`role="alert"`）に出し、入力を残す。
  - **設定**: `GET /admin/settings` の値で、登録モード（`RadioGroup`、各モードの説明付き）、既定の言語（`Select`。カタログのある言語と保存されている言語）、ローカルと連合のタイムライン（`Checkbox`）、
    Authorized Fetch（`Checkbox`）、リモートのメディアの複製（`RadioGroup`）と持つ期間（`Select`。1 時間〜365 日と保存されている値）、処分したドメインの一覧と公開コメントを見られる人（`RadioGroup`、誰にも・ログインした利用者・誰にでも）を出す。
    1 つのフォームで、「設定を保存する」は変えた設定だけを `PATCH /admin/settings` でまとめて送る（サーバーはすべてか何も変えない）。成功したら応答で設定のキャッシュを置き換え、`GET /instance`（登録モードとタイムライン）と監査ログを取り直してから
    結果をボタンの下の `StatusMessage` に出す。変えた設定がなければ何も送らずにその旨を出し、失敗はボタンの上（`role="alert"`）に出して選択を残す（E2）。
- **ルール**（`/admin/rules`、`manage_rules`、FR-705）: 「ルールを追加する」と「有効なルール」の区画（`h2` の `section`）。
  - **追加**: アプリの言語ごとに、ルール（`TextField`、1 行、300 文字）と補足（`TextArea`、1,000 文字）を並べる。文も補足も空の言語は送らず、補足だけの言語は文の欄のエラー、どの言語にも文がなければフォームの下（`role="alert"`）のエラーにして送らない。
    成功したらルールの一覧・`GET /instance`（ルール）・監査ログを取り直し、入力を空にして結果をフォームの `StatusMessage` に出す。
  - **一覧**: 有効なルールを順に `ol` で並べ、各ルールは見出し（`h3`、「ルール {番目}：{閲覧者の言語の文}」。文は U+2068 / U+2069 で囲む）と、言語ごとの文と補足（`dir="auto"` と `lang`）の説明リストにする。
    「上へ」「下へ」（最初のルールに「上へ」、最後のルールに「下へ」は出さない）は `POST /admin/rules/{id}/move` で 1 つずらし、一覧を取り直してから、動いたルールの見出しにフォーカスを移す（`lib/useFocusAfterRemoval.ts` の `useFocusHeadingAfterMove`）。ドラッグでは並べ替えない。
    「編集」はダイアログに追加と同じ欄（保存されている言語を含む）を出し、保存したら閉じる。「廃止」は元に戻せないので `ConfirmDialog`（`danger`）で確かめ、成功したら一覧から除いてフォーカスを次の行のボタンに移す。
    ボタンのアクセシブルネームはルールの文を含める。結果は一覧の上の `StatusMessage`、失敗は同じ場所（`role="alert"`。編集と廃止はダイアログの中）に出す。
- **テーマ**（`/admin/theme`、`manage_settings`、FR-605、[ADR-0059](../adr/0059-design-tokens-and-instance-theme.md)）: 「色」「コントラスト」「文字・余白・動き」「カスタム CSS」「プレビュー」の区画（`h2` の `section`）と、保存のボタンの 1 つの `form`。
  - 色は上書きできる `color-*` のトークンごとに、ライトとダークの `TextField`（名前は「{トークン}、ライト」、説明に形と既定値）。空の欄は既定値のままで、片方だけ入れた色はもう片方を既定値にして送る。ほかのトークンは 1 つずつの `TextField`（名前はトークン、説明に既定値と範囲）。トークンの名前は訳さない。
  - 「コントラスト」は入力のたびに、`@my/ui` の `themeContrast`（サーバーと同じ組と式）で最小値を下回る組と、形の誤った色を文で並べる（保存のボタンの前）。保存できるかを決めるのはサーバーで、422（`invalid-theme`）の `theme_errors` を 1 つずつ訳した文の一覧を、ボタンの上の `role="alert"` に出し、入力を残す（E2）。
  - 「カスタム CSS」は `TextArea` で、サーバーが結果を確かめないこと（コントラスト・レイアウト・フォーカス・RTL は管理者の責任）、CSP の下で同じオリジンの資源しか使えないこと、クラス名と構造は版で変わることを説明に書く。
  - 「プレビュー」は入力中のトークンを、区画の要素の CSS 変数（React の `style`、D10）にして当てた見本（面・文字・補足の文字・リンクとボタンに似せた表示・エラーの文字）。操作できる要素は置かない。カスタム CSS はプレビューしない。
  - 保存に成功したら、応答でテーマのキャッシュを置き換え、`GET /instance`（`theme`）と監査ログに無効化の印を付け、区画の `StatusMessage` に「数分以内に全員に届く」ことを出す。開いているページの見た目は読み込み直すまで変わらない。
- **通報の一覧**（`/moderation/reports`、FR-701・FR-710）: 「未解決」と「解決済み」のタブ（`?state=resolved`、URL に置く。S3）で、新しい順。各通報は見出し（対象のハンドル、詳細へのリンク）と、カテゴリ・通報者・通報日時・担当・投稿の数（解決済みは結果と解決日時も）の説明リスト、通報者のコメント（`dir="auto"`、2 行に省略）にする。
  担当はアカウントの ID しか返らないので、閲覧者なら「あなた」、ほかは `GET /accounts/{id}` で名前を出す。
- **通報の詳細**（`/moderation/reports/{id}`、NFR-MOD-4）: 1 つの画面に、通報の内容（状態・カテゴリ・日時・通報者・通報元・転送・引用するルール・コメント）、通報された投稿、通報されたアカウントとその履歴（ほかの通報と処分の件数、`manage_accounts` があれば処分の履歴）、担当、メモ、対応の区画を `h2` の `section` で並べる。
  - **投稿**: `GET /statuses/{id}` で読み、`StatusCard` を操作の列なし（`withActions={false}`）で出す。モデレーターにも公開範囲が効くので、読めない投稿（削除済み・公開範囲の外）はその旨を出す。
  - **ルール**: `GET /instance` の `rules` から文を引き、今のルールにないもの（廃止されたもの）はその旨を出す。
  - **担当とメモ**: 「自分が担当する」（担当がほかの人なら「担当を引き継ぐ」）と「担当を外す」は位置を変えずに名前だけを変え、押したボタンにフォーカスが残る。メモは古い順で、新しいメモは `ReasonField`（必須、2,000 文字）から追加する。空白だけのメモと失敗は入力欄のエラーにして、文を残す。
  - **対応**: 未解決なら、対象のアカウントへの処分のフォーム（`components/AccountActionForm.tsx`。`manage_accounts` がなければ理由を出す）と、「処分せずに解決する」「処分済みとして解決する」を出す。解決済みなら結果と解決日時と「再開する」を出す。
    処分は `report_id` 付きで送り、サーバーが同じ操作で通報を処分済みにするので、通報・一覧・件数・処分の履歴を無効にする（一覧の「未解決」から消える）。解決・再開・処分で区画の中身が入れ替わったら、区画の見出しにフォーカスを移し、結果を区画の `StatusMessage` に出す。担当・解決・再開の失敗はトーストで知らせる（E2）。
  - **処分のフォーム**: 種類（`RadioGroup`、軽いものから BAN まで、効果の説明付き）。ほかのサーバーのアカウントは `force_sensitive`・`limit`・`suspend` だけで、知らせるかのチェックボックスを出さない。投稿への処分（`mark_statuses_sensitive`・`delete_statuses`）は通報の投稿が読めたときだけ出し、その投稿をチェックボックスで選ぶ（最初はすべて選び、1 つもなければ送らずに `role="alert"` で知らせる）。
    理由の種類（`Select`、最初は通報のカテゴリ）、期間（`DurationSelect`。BAN・投稿への処分・警告は持たない）、公開の理由と非公開のメモ（`ReasonField`、5,000 文字）、アカウントに知らせるか（既定はオン）。取り消せない BAN と `delete_statuses` は `ConfirmDialog` で確かめ、失敗はダイアログの中に、ほかの種類の失敗はフォームの下に出す。
    詳細の画面は、対象のアカウントの管理画面（`/moderation/accounts/{id}`）にリンクする。
- **アカウントの検索**（`/moderation/accounts`、FR-714）: 「検索」と「検索結果」の 2 つの区画。検索の条件は API の名前のまま URL に置き（`?username=`・`domain`・`origin`・`state`・`email` / `email_domain` / `ip`。S3）、
  フォームを送ると URL を変える。URL が変わる（戻る / 進む・リンク）とフォームはその条件から作り直す（`key`、R10）。
  - **フォーム**: ユーザー名（前方一致。ハンドル `@name@domain` を入れるとユーザー名とドメインに分ける）、サーバー（ドメイン。`lib/idn.ts` の `domainToAscii`（URL Standard のホストの解析）で A-label にして送る）、場所（このサーバー / ほかのサーバー）と状態の `Select`、
    「メールアドレスか IP アドレス」の `Select`（使わない・メールアドレス・メールアドレスのドメイン・IP アドレス）。API は 3 つのうち 1 つしか受け付けないので、選んだ 1 つの入力欄（必須）だけを出す。URL に 2 つ以上あれば最初の 1 つだけを使う。
  - **結果**: 新しい順の一覧。各アカウントは見出し（アバターと表示名・ハンドル、詳細へのリンク）と、状態・サーバー（ローカルは「このサーバー」、リモートは `DomainName`）・登録の審査（ローカルだけ）・作成日時・いま効いている制限（あれば）の説明リストにする。結果はメールアドレスと IP アドレスを含まない。
    API は 1 ページで読むアカウントの数に上限があり、空のページでも続きがあり得るので、見つからないまま続きがあるときは、空の状態に「続きを探す」を出す。
    サーバーが断った検索（IP アドレスでない `ip` など）と通信の失敗は、ローダーで画面の失敗にせず、結果の区画に `RegionError`（理由と「もう一度試す」）で出し、フォームは送った条件のまま残す。権限の 403 だけは画面の失敗にする。
- **アカウントの管理画面**（`/moderation/accounts/{id}`、FR-714・FR-703・FR-713）: 「アカウント」「制限」「ロール」（ローカルで `manage_roles` があるときだけ）「メールアドレスと IP アドレス」（ローカルだけ）「アカウントへの処分」「アカウントへの通報」「処分する」の区画。
  - **アカウント**: アバターと表示名・ハンドル、プロフィールと（`view_audit` があれば）監査ログへのリンク、状態・サーバー・作成日時と、ローカルのアカウントでは登録の審査・申請と審査の日時・招待・メールアドレスの確認・ロール（キーを `dir="ltr"`）、登録の理由（`dir="auto"`）。制限の区画は、いま効いている処分が取り上げているもの（`restrictions`）を文で並べる。
  - **ロール**（FR-720）: `GET /admin/roles` のすべてのロール（位置の高い順）を 1 行ずつ、キーの見出し（`h3`、`dir="ltr"`）と、このアカウントが持つか・権限（`dir="ltr"`）の説明リストにする。
    持っているかは `GET /admin/accounts/{id}/roles`、閲覧者のいちばん高いロールは `GET /admin/accounts/{閲覧者の ID}/roles` で読む。閲覧者のいちばん高いロール以下のロールには「付与する」か「剥奪する」（アクセシブルネームにロールのキー）を出し、それより上のロールはボタンの代わりに、閲覧者のいちばん高いロールを挙げて変えられない理由を書く。
    どちらも `ConfirmDialog` で確かめる（付与は `primary`、剥奪は `danger`。`admin` の剥奪の本文は、ただ 1 人の管理者からは剥奪できないことを書く）。失敗はダイアログの中に出し、上位のロール（403）と最後の管理者（409）はクライアントのカタログの文言で理由を書く。403 のときは閲覧者のロールを読み直す。
    成功したら応答のロールでキャッシュを置き換え、押したボタンが反対のボタンに替わるので区画の見出しにフォーカスを移し、監査ログに記録したことを区画の `StatusMessage` に出して、アカウント・検索・監査ログを読み直す（閲覧者自身のロールなら閲覧者の権限も）。
  - **二要素認証**（FR-005、ローカルのアカウントだけ）: 利用者が二要素認証を使っているか（`two_factor_enabled`）を出す。使っていれば「二要素認証を解除」を出し、`ConfirmDialog`（`danger`）で確かめてから `POST /admin/accounts/{id}/two_factor/disable` を送る。
    失敗はダイアログの中に出す。成功したらアカウントと監査ログを読み直し、ボタンが消えるので区画の見出しにフォーカスを移して、結果を区画の `StatusMessage` に出す。
  - **メールアドレスと IP アドレス**（ADR-0038）: 画面を開いただけでは要求しない。「メールアドレスと IP アドレスを表示する」を押したときだけ `POST /admin/accounts/{id}/reveal` を送る（サーバーは送るたびに監査ログに `revealed_private_data` を記録するので、query にせず mutation にする。R7）。
    答えは mutation の中にだけ置き、Query のキャッシュに入れないので、画面を離れて戻ると隠れ、見るにはもう一度押す。表示したら区画の見出しにフォーカスを移し、監査ログに記録したことを区画の `StatusMessage` に出す。失敗はトーストで知らせ、ボタンを残す（E2）。
    メールアドレス（確認の有無、そのドメインのアカウントの検索へのリンク）、登録元の IP と日時、最近のログイン（日時・IP・User-Agent。IP・User-Agent・メールアドレスは `dir="ltr"`）、同じ IP アドレスのほかのアカウント（IP ごとに、各アカウントの管理画面とその IP の検索へのリンク）を出す。残っていないものは「不明」にする。
  - **処分の履歴**: `GET /admin/accounts/{id}/actions` の新しい順の一覧（「さらに表示」）。各処分は種類の見出し（`h3`、`tabindex="-1"`）と、状態・日時・終了・解除した日時・理由の種類・処分した人・アカウントへの通知（ローカルだけ）・投稿の数・通報へのリンクの説明リスト、公開の理由・非公開のメモ・解除の理由（`dir="auto"`）にする。
    有効な処分には「解除」（アクセシブルネームに種類と処分した日時を含める）を出す。BAN は解除できない（API が断る）ので出さない。解除は `ConfirmDialog`（`primary`）で確かめ、ダイアログは任意の解除の理由（`ReasonField`、5,000 文字）を持ち、開くとそこにフォーカスが移る。
    失敗はダイアログの中に出す。成功したらダイアログを閉じ、キャッシュの一覧の処分を解除済みに置き換え、フォーカスをその行の見出しに移し（`lib/useFocusAfterRemoval.ts` の `useFocusHeadingAfterLift`）、結果を区画の `StatusMessage` に出して、アカウントの状態と制限と検索を読み直す。
  - **通報**: 新しい 20 件の通報の、通報の日時の見出し（`manage_reports` があれば通報の詳細へのリンク）と、状態（解決済みなら結果）・カテゴリ・通報者の説明リスト、コメント（`dir="auto"`、2 行に省略）。
  - **処分する**: 通報の詳細と同じ処分のフォーム（`AccountActionForm`。通報なし、理由の種類の最初は `other`）。リモートのアカウントには `force_sensitive`・`limit`・`suspend` だけを出し、知らせるかのチェックボックスを出さない。
    投稿への処分は投稿を選ぶ通報の詳細から行うので、この画面では出さず、その旨をフォームの上に書く。成功したら区画の `StatusMessage` に結果を出し、処分の履歴・アカウント・検索・通報を無効にする。削除済みのアカウントにはフォームを出さない。
- **規約とポリシー**（`/admin/documents`、FR-705、#489）: 言語の `Select`（`?locale=`、URL に置く。S3。カタログのある言語と、文書がすでにある言語）と、利用規約・プライバシーポリシーの 2 つの区画（`h2` の `section`）。
  各区画は、書かれている言語・選んだ言語で最後に保存した日時の説明リスト、公開ページへのリンク（閲覧者が読めるときだけ）、選んだ言語の本文（Markdown の `TextArea`、100,000 文字）と「保存する」の `form` で、言語ごとに別に保存する。
  成功したら本文・`GET /instance`（リンク）・監査ログを取り直してから、監査ログに記録したことを区画の `StatusMessage` に出す。失敗はフォームの下（`role="alert"`）に出して入力を残す。保存してある本文を空にして保存すると削除になるので、`ConfirmDialog`（`danger`）で確かめる。
  下書きは文書と言語ごとに持ち、言語を変えても残る。下書きがあるまま別の画面へ移るときは、`features/settings` の `useUnsavedChangesGuard` で確かめる（§9 の未保存の変更）。
- **共通の部品**: 管理画面の一覧と詳細は同じ型で組む。権限の出し分けは `RequirePermission`、記録の一覧（区切り線の行・空の状態・「さらに読み込む」、行の `data-row-id`）は `RecordList`、詳細の画面の区画（`h2` で名前を付けた `section`）は `DetailSection`、区画の中身が入れ替わって押したボタンが消えるときに区画の見出しへフォーカスを移すのは `useFocusAfterChange` にし（`features/moderation/components/parts.tsx`）、
  記録の項目と値は `@my/ui` の `DescriptionList` / `DescriptionItem` にする。
- **取り消せない操作**（BAN・ドメインの凍結・投稿の削除。NFR-MOD-7）: `@my/ui` の `ConfirmDialog`（APG の alertdialog）を挟む。理由は `ReasonField`、期限は `DurationSelect` をダイアログの中に置く。
  開くと最初の入力欄（なければ「キャンセル」）にフォーカスが移り、Esc で閉じ、閉じると開いたボタンにフォーカスが戻る。失敗はダイアログの中（`errorMessage`）に出して開いたままにし、成功したら閉じて、結果を画面の `StatusMessage`（`role="status"`）に出す。
  操作で開いたボタンが消えるとき（一覧から行が消えるなど）は、`lib/useFocusAfterRemoval.ts` でフォーカスの行き先を決める。
- **テスト**: `features/admin/admin.test.tsx` が設定の画面の 2 つの区画の出し分けと dead job の再実行・失敗・権限を、`features/moderation/instances.test.tsx` が連合先の一覧と絞り込み・ページの読み足し・再開・失敗・403 を MSW で検査する。
  既定の閲覧者は権限を持たないので、テストは `test/msw/http.ts` の `operatorHandler()` で権限を与える。
  `features/moderation/domainActions.test.tsx` は、IDN のドメインの処分（A-label で記録され、両方の形で出る）、必須の入力、`suspend` の確認と取り消し、ダイアログの中とフォームの下の失敗、解除とフォーカス、連合先の詳細からのリンクとドメインの絞り込みを検査する。
  `features/moderation/signupBlocks.test.tsx` は、IDN のメールのドメインの拒否（A-label で記録され、両方の形で出る）、種類で変わる値の欄、必須の入力、フォームの下の失敗、解除とフォーカス、解除の失敗、管理の画面のリンク、権限を検査する。
  `features/moderation/signupBlockImport.test.tsx` は、貼り付けた一覧の追加（送る内容、追加しなかった行の表と理由ごとの行数、一覧の取り直し、次の一覧のためのフォーム）、テキストファイルからの読み込み、大きすぎるファイル、値のない一覧と効果の必須、10,000 件を超える一覧、長い表の「すべて表示する」とフォーカス、サーバーの失敗を検査する。
  `features/moderation/auditLog.test.tsx` は、監査ログの行と対象へのリンク、`Select` と行のリンクでの絞り込みと URL、絞り込みのあとのフォーカス、`?pages=` の書き込みと読み込み、URL の不正な値、次のページの失敗、空の状態、権限のない閲覧者と 403 を検査する。
  `features/moderation/registrations.test.tsx` は、登録の理由の `dir="auto"`、ページの読み足し、メモ付きの承認と却下・フォーカス・`role="status"` の結果、キャンセルと Esc、メモの上限、ダイアログの中の失敗、先に決められた登録、権限を検査する。
  `features/moderation/accounts.test.tsx` は、検索の結果（メールアドレスと IP アドレスを出さない）、キーボードでの検索と URL（ハンドルの分割・A-label のドメイン・IP）、必須の入力欄、断られた検索と再試行、続きを探す空の状態、
  開いただけではメールアドレスと IP アドレスを要求しないことと、押したときに 1 回だけ要求してフォーカスと `role="status"` で知らせること、その失敗、リモートのアカウントの種類、処分の発行と読み直し、確認と理由付きの解除とフォーカス、BAN に解除がないこと、解除の失敗と取り消し、権限を検査する。
  `features/moderation/reports.test.tsx` は、通報の一覧とタブ、詳細の各区画（読めない投稿・廃止されたルール・処分の履歴）、担当とメモ、処分による解決と一覧から消えること、BAN と投稿の削除の確認、ほかのサーバーのアカウントの種類、処分せずに解決と再開とフォーカス、失敗、権限と 403 を検査する。
  `features/moderation/appeals.test.tsx` は、異議申し立ての一覧とタブと URL、審査済みのタブが審査待ちだけのページを読み進めること、本文の `dir="auto"`、説明付きの撤回で処分が解除と出ることとフォーカス、説明なしの維持、キャンセルと Esc、ダイアログの中の失敗、先に逆の決定をされた申し立て、権限と 403 を検査する。
  `features/admin/settings.test.tsx` は、名前の保存と外枠の名前、説明の言語ごとの保存、設定の表示と変えた設定だけをまとめて送ることと `GET /instance` の取り直し、既定の言語の選択、保存の失敗、権限を、
  `features/admin/rules.test.tsx` は、ルールの一覧と言語ごとの文、追加と入力の検査、下へ動かしたルールの見出しへのフォーカス、ダイアログでの編集、確認を経た廃止、権限を、
  `features/moderation/invites.test.tsx` は、招待の一覧の状態と人数、既定の値と制限なしでの作成とコードとリンク、失効と見出しへのフォーカス、登録モードが `invite` でないときの説明、権限を検査する。
  `features/admin/documents.test.tsx` は、表示言語での本文と書かれている言語、公開ページをページごと開くこと、保存と `role="status"` の結果、URL での言語の切り替えと言語ごとの下書き、削除の確認と取り消し、保存の失敗、未保存の変更の確認、権限を検査し、
  `features/documents/documents.test.tsx` は、サインインの画面・設定・メニューのシートの文書へのリンクと、公開していない文書を出さないことを検査する。
  `features/moderation/roles.test.tsx` は、ロールの一覧と持っているか、確認を経た付与と剥奪・`role="status"` の結果・見出しへのフォーカス、キャンセルと Esc、閲覧者より上のロールに操作がなく理由が出ること、403 と 409（最後の管理者）の理由をダイアログの中に出すこと、読み込みの失敗と再試行、`manage_roles` のない閲覧者にロールの区画も要求もないことを検査する。
  E2E（`e2e/admin.spec.ts`、`e2e/domain-actions.spec.ts`、`e2e/audit-log.spec.ts`、`e2e/reports.spec.ts`、`e2e/accounts.spec.ts`）は、権限のある閲覧者として各画面の axe・横スクロール・ARIA スナップショット・配送の再開・再実行・`suspend` の確認のダイアログ・解除とフォーカスの移動・監査ログのキーボードでの絞り込みとページの読み足しの URL・キーボードで通報を開き、BAN の確認のダイアログを経て、処分で通報を解決して一覧から消えるまで・キーボードで検索からアカウントを開き、押したときだけメールアドレスと IP アドレスを出し、解除の確認のダイアログを経てフォーカスが行の見出しに移るまで・リモートのアカウントの処分の種類を検査する。
  `e2e/registrations.spec.ts` は、登録の審査の画面の axe・横スクロール・ARIA スナップショット、承認の確認のダイアログ、キーボードでの却下とフォーカスの移動を検査する。
  `e2e/signup-blocks.spec.ts` は、登録の拒否リストの axe・横スクロール・ARIA スナップショット（両方の形のドメインと長いハッシュ）、キーボードでのメールのドメインの拒否の追加、キーボードでの一覧からの追加と追加しなかった行の表の ARIA スナップショット、解除とフォーカスの移動を検査する。
  `e2e/server-settings.spec.ts` は、名前の区画の ARIA スナップショットと名前の変更で外枠の名前が変わることと、キーボードで登録モードを招待制にして保存するまでを、各幅とロケールの axe・横スクロールとともに検査する。
  `e2e/roles.spec.ts` は、ロールの区画の ARIA スナップショット（閲覧者より上のロールの理由）・axe・横スクロールと、キーボードで付与の確認のダイアログを経て、区画の見出しにフォーカスが移るまでを検査する。
  `e2e/documents.spec.ts` は、本物の `my serve` で登録の画面の同意の文とフッターから文書の公開ページを開き、Markdown の見出し・箇条書き・表の ARIA スナップショット・両方のテーマの axe・横スクロールを、`vite preview` で規約とポリシーの画面の ARIA スナップショット・axe・横スクロールと、キーボードでプライバシーポリシーを書いて保存するまでを検査する。
  `e2e/appeals.spec.ts` は、異議申し立ての一覧の axe・横スクロール・ARIA スナップショットと、キーボードで申し立てを開き、撤回の確認のダイアログを経て処分が解除と出て、審査の見出しにフォーカスが移るまでを検査する。
  `features/moderation/staffEntry.test.tsx` は、ナビゲーションの「モデレーション」と「管理」の出し分け（モデレーターに「管理」が出ないこと、両方を持つ閲覧者での順番）・レールとメニューのボタンとシートの件数の名前・各区分の画面と `aria-current`・使える画面のない区分の画面・ストリームの通知での件数の読み直しを検査し、
  `e2e/admin-entry.spec.ts` は、各幅で「モデレーション」（`md` 未満はメニューのボタンとシート）の件数の名前と数字、モデレーターに「管理」がないこと、モデレーションの画面の ARIA スナップショット、サーバーからの通知の行とそのリンク（ARIA スナップショット）・axe・横スクロールを検査する。
  `e2e/admin-a11y.spec.ts` はスタッフの画面のアクセシビリティの回帰テストで、すべての権限を持つモデレーターとして、すべての画面（`src/routes` の `moderation.*.tsx` と `admin.*.tsx`）の axe と横スクロールを、各プロジェクトで検査する。
  画面の一覧はルートのファイルと突き合わせるので、どちらかの区分のルートを足したら一覧にも足さないと失敗する。
  キーボードだけ（Tab・Shift+Tab・Enter・矢印キー）で、モデレーションの画面から通報の一覧、通報、対象のアカウントの管理画面（処分と通報の履歴）、通報に戻って処分を選び、処分で通報を解決して一覧に戻るまでを通し、画面が変わるたびに `h1` にフォーカスが移ることと axe・横スクロールを検査する。
  E2E の spec はすべてのプロジェクトで動くので、管理画面は nightly（`E2E_MATRIX=full`）の全幅 × 全ロケールに入る。
- **リリースごとの手動確認**（[ADR-0027](../adr/0027-ui-quality-gates.md) の手動の層）: 自動の検査では分からない読み上げと拡大の具合を、すべての権限を持つアカウントで、通報 1 件・承認待ちの登録 1 件・審査待ちの異議申し立て 1 件がある状態から確かめる。
  1. **VoiceOver**（macOS の Safari、iOS の Safari）と **NVDA**（Windows の Firefox か Chrome）: モデレーションの画面から通報を開き、対象のアカウントの管理画面で履歴を読み、通報に戻って処分を出して解決するまでを、読み上げだけで行う。
     画面が変わるたびに新しい画面の見出しが読まれること、リンクの名前に待っている件数が入ること（「通報 未解決の通報 1 件」）、区画が見出しの名前のランドマークとして並ぶこと、処分の種類のラジオボタンが説明付きで読まれること、
     BAN・投稿の削除・ドメインの凍結の確認のダイアログが開くとその問いが読まれて Esc で閉じ、開いたボタンへ戻ること、処分や解決の結果が `role="status"` で読まれ、フォーカスが区画の見出しに移ることを確かめる。
     登録の審査（承認と却下）と異議申し立ての審査（撤回と維持）も、確認のダイアログと結果の読み上げまで同じように通す。
  2. **200% / 400% 拡大**（ブラウザの拡大。400% は幅 1280 px の窓で 320 CSS px 相当）: すべての管理画面で、横スクロールなしに読めて操作できること、説明リスト・長いドメインやハンドル・dead job の本文が折り返すこと、
     確認のダイアログとフォームの選択肢（期間・理由の区分）が画面に収まり、フォーカスのある要素が固定ヘッダの下に隠れないことを確かめる。
  3. **強制カラーモード**（Windows のハイコントラスト、または Chrome の DevTools の `forced-colors: active` の模擬）: すべての管理画面で、タブの選択中・ラジオボタンとチェックボックスの状態・件数の `Badge`・状態の文言・フォーカスのリング・ボタンの境界が見え、色だけで示しているものがないことを確かめる。
  見つかった問題は Issue にし、自動の検査で捕まえられるものは E2E か部品のテストに足す。

- **サーバーからの通知**（`features/notifications`）: 行為者のいない通知とモデレーターへの通知は、行の操作の列に、その後を確かめる画面へのリンクを置く。
  `moderation_notice` と `appeal_decided` は処分の種類と理由の区分（サーバーが訳した `kind_label` / `reason_category_label`）・モデレーターの説明（`dir="auto"`）・終わる日時（`appeal_decided` は異議申し立てへの決定とその理由も）を本文に出し、アカウントの状態の画面（`/settings/account-status`、FR-715）へ、
  `report_resolved` は通報したアカウントの名前（そのプロフィールへのリンク、アバターなし）を文に入れ、`admin_report` は通報（`/moderation/reports/{id}`）へ、`admin_sign_up` は登録の審査（`/moderation/registrations`）へ、`admin_appeal` は異議申し立て（`/moderation/appeals/{id}`）へ、
  `relationships_severed` は自分のフォロワーとフォローの一覧へリンクする。

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
| G10 | アイコンは `@my/ui` のアイコン集（`Icons.tsx`）から使う。機能やルートに SVG のパスを書かない。形は Phosphor（`react-icons/pi`、[ADR-0043](../adr/0043-icon-set.md)）で、`Icons.tsx` の表が意味の名前（`ReplyIcon`）を Phosphor のアイコン（オフ・オンの形、向きの有無）に対応させる。太さは Regular を基本にする。新しいアイコンは描かずに表に 1 行足し、`react-icons` を import するのは `Icons.tsx` だけ、`react-icons/pi` だけにする。部品も SVG を描かない。例外は部品の形の一部で状態とともに変わる印（`Checkbox` のチェック、`Select` と `ContentWarning` の山形、`Tooltip` の矢印、`Avatar` の人影）と、サーバーが符号化した QR コードを描く `QrCode`（アイコンではなくデータの絵）と、テストやストーリーの差し替えの中身だけで、`app/.oxlintrc.json` に理由とともに挙げる | `restrict/svg`（JSX の `<svg>`・`<path>` などの SVG の要素）、`no-restricted-imports`、`boundaries/dependencies`、`knip` |
| G11 | アイコンの大きさは置き場所で決める: 文と並ぶものは `size-4`、ボタンの中は `size-5`、ナビゲーションは `size-6`。`Button`・`ToggleButton`・`Link` が子の SVG の大きさを決め、アイコンの側では大きさを指定しない（react-icons が書く `1em` より `[&>svg]:size-*` が優先し、置き場所が決めない所では文字の大きさで描かれる） | 部品のテスト（ボタンの中の SVG が 0 × 0 でない）、スクリーンショット |
| G12 | アイコンは飾り（`aria-hidden="true"`、`focusable="false"`、`Icons.tsx` がどのアイコンにも付ける）にし、意味は見える文言かアクセシブルネームで伝える。アイコンだけのボタンには `aria-label` と、同じ文言の `Tooltip` を付ける | `@my/ui` の部品のテスト（`Foundations/Icons` の play）、レビュー |
| G13 | オン / オフはアイコンの形でも示し、色だけに頼らない（WCAG 1.4.1）: オンは、形が閉じるもの（星・ブックマーク・警告）を Phosphor の Fill、閉じないもの（ブースト）を Bold にする。向きのあるアイコン（返信・戻る・外部へのリンク）は `rtl:-scale-x-100` で反転し（Phosphor の `mirrored` は書字方向を見ないので使わない）、時計回りの意味を持つもの（再読み込み）や輪（ブースト）は反転しない | `Foundations/Icons` の play（`ar-XB` で向きのあるものだけが反転する）、スクリーンショット（`ar-XB`）、レビュー |

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

## 18. アカウントの状態と異議申し立て

`features/moderation` が、利用者が自分のアカウントへの処分を確かめ、異議を申し立てる画面を受け持つ（FR-715、FR-707）。同じ機能がモデレーターの画面（§14）も受け持つ。

- **画面**（`/settings/account-status`、設定のメニューの「アカウントの状態」）: `moderation_notice` の通知もここへ移る。「効いている処分」（`GET /moderation/actions?active=true`）と「過去の処分」（すべての処分から効いているものを除いたもの。API は効いているものでしか絞り込めない）の 2 つの区画（`h2` の `section`）を、どちらも新しい順に並べる。
  ルートの loader が両方の最初のページを読む。過去の処分は除いたあとのページが空でも「さらに表示」を出し、続きがなく空のときだけ `EmptyState` にする。
- **処分**: 各処分は種類の見出し（`h3`、サーバーが利用者の言語で返す `kind_label`）の `article` で、状態・理由の区分（`reason_category_label`）・処分した日時・期限・解除した日時の説明リストと、モデレーターからの説明（`dir="auto"`）を出す。
  期限のない処分は、期間を持てる種類が効いている間だけ「解除されるまで」と出し、警告・投稿への処分・BAN と、終わった処分には期限の項目を出さない。日時はすべて利用者の設定のタイムゾーンとロケールで書く（`react-intl` の `timeZone`、§9）。
  投稿への処分は対象の投稿（`h4`）を `StatusCard` の操作の列なしで並べ、削除された投稿（404）はその旨を出す。
- **異議申し立て**（`h4`「異議申し立て」）: 申し立て済みなら、その状態（審査中・維持・撤回）・申し立てと決定の日時・書いた内容と決定の説明（`dir="auto"`）を出し、フォームは出さない。
  申し立てられる（`appealable`）なら期限（`appeal_deadline`）と「異議を申し立てる」を出す。ボタンのアクセシブルネームは処分した日時を含める。どちらでもなければ、期限が過ぎたこと（期限がなければ申し立てられないこと）を出す。
  申し立ては 1 つの処分に 1 回だけで後から変えられないので、`ConfirmDialog`（`primary`）の中の `ReasonField`（必須、2,000 文字）で書く。開くと理由の欄にフォーカスが移る。空白だけの理由と上限を超えた理由は `validate` で入力欄のエラーにし、送らない。
  送信が失敗したらダイアログの中に理由を出して入力を残す。処分の状態が理由の失敗（`already-appealed`・`appeal-window-closed`・`not-appealable`）では一覧を読み直し、フォームが消える。
  成功したらダイアログを閉じ、応答の処分で両方の一覧のキャッシュを置き換え、結果を画面の上の `StatusMessage` に出す。開いたボタンはフォームと一緒に消えるので、フォーカスはその処分の「異議申し立て」の見出し（`tabindex="-1"`）に移す。
- **テスト**: `features/moderation/accountStatus.test.tsx` が、設定のメニューからの移動、効いている処分と過去の処分の項目、対象の投稿と削除された投稿、利用者のタイムゾーンとロケールでの日時、空の状態、ページの読み足し、期限が過ぎた処分と申し立て済みの処分にフォームを出さないこと、
  理由の必須・送信・フォーカス・`role="status"` の結果、キャンセルと Esc、期限が過ぎて断られたときの読み直しを MSW で検査する。E2E（`e2e/account-status.spec.ts`）は、画面とダイアログの axe・横スクロール・ARIA スナップショットと、キーボードでの申し立てを検査する。
