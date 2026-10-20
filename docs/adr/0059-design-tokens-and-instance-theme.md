# ADR-0059: デザイントークンの体系とサーバー単位のテーマ

- Status: Proposed
- Date: 2026-10-20
- Deciders: @rosylilly
- Issue: #649

## Context（背景・課題）

決定者の指示は「デザイントークンの管理を強化したうえで、トークン上書きによる独自テーマをサーバーで適用できる仕組み」である。
Web クライアント（FR-601）と管理画面（FR-602）に関わり、実装は #664 が待っている。

トークンは `app/ui/src/theme.css` が Tailwind CSS v4（ADR-0025）の `@theme` で持つ。

- 色は 2 層: `--palette-*`（OKLCH の原色。ユーティリティを作らない）と、`light-dark(<ライト>, <ダーク>)` で原色を指す意味の色 `--color-*`（`@theme static` なので必ず出力される）。
- 色以外（文字・行の高さ・余白の単位・角丸・影・動きの時間と緩急）も同じファイルの `@theme` にあり、Tailwind の既定の値の多くは `initial` で消してある。
- `app/ui/src/tokens.ts` が前景と背景の組と用途（文字 4.5:1、UI 3:1）を列挙し、`make web-tokens`（`app/ui/scripts/check-tokens.ts` と `scripts/color.ts`）が
  両方のテーマで WCAG 2.2 AA のコントラスト、原色の sRGB の色域、組にも装飾の一覧にもない意味の色、既定のパレット・任意の色・`motion-safe:` の外の動きの使用を検査する。
- テーマの切り替えは `<html data-theme>`（ライト / ダーク / なしで OS）だけで、`public/theme.js` が最初の描画の前に付ける（web-client.md §9）。

サーバーごとの見た目の違いは、Web App Manifest と外枠の `theme-color`（`internal/interface/web` の `Options.ThemeColor` / `BackgroundColor`）しかなく、
それも設定から渡す経路がなく既定の `#ffffff` のままである。

制約は次のとおり。

- 外枠の HTML はすべての閲覧者に同じもので、60 秒の共有キャッシュがあり、CSP は nonce のない `style-src 'self'` と Trusted Types である（ADR-0028、ADR-0019 §9、web-client.md §9）。
  部品は `setAttribute('style')` や `cssText` を使わない（D10）。
- サーバーが描画する画面（web-client.md §8）は JavaScript なしで動き、`server-pages.css` で同じトークンを使う。
- 品質ゲート（ADR-0027）は、ライト / ダークの両方でコントラスト AA と axe を求め、`@my/ui` のスクリーンショットを比べる。
- 設定はイベント（リビジョン）と投影に分け、問い合わせや制約のあるデータを JSON に入れない（ADR-0020、data-model.md §4.9）。

## Decision Drivers（判断基準）

1. **上書きしてもアクセシビリティを壊さない**: どの上書きでも、ライト / ダークの両方でコントラスト AA を満たす（ADR-0027、Y3）。保存の時点で拒めること。
2. **CSP とキャッシュを緩めない**: インラインのスタイル・nonce・閲覧者ごとの外枠を足さない（ADR-0028、ADR-0019）。
3. **最初の描画からテーマが当たる**: 既定の色が一瞬見えない。JavaScript のないサーバーの画面にも当たる。
4. **トークンの定義が 1 か所**: 上書きできるトークンの一覧・既定値・コントラストの組を、CSS・TypeScript・Go で別々に書かない。
5. **アップグレードで壊れない**: 上書きが部品の内部構造（クラス名や DOM）に依存せず、版を上げても意味が保たれる。
6. **依存を増やさない**: Accepted ADR が決めたライブラリ（Tailwind CSS v4、Storybook 10 など）の範囲で作る。

## Considered Options（選択肢）

### A. トークンの体系（定義の元）

1. **A1: `theme.css` を元のままにし、上書きの対象と検査の組をそこから生成する**（推奨）。
   Tailwind v4 は CSS を設定の元にする（[Theme variables](https://tailwindcss.com/docs/theme)。テーマの変数は通常の CSS 変数として出力され、ユーティリティもそれを参照する）。
2. **A2: JSON / TypeScript の 1 つの定義から CSS と Go を生成する**（Style Dictionary のような変換の仕組み）。定義を言語に依らない形にできるが、
   Tailwind の `@theme` の書き方（`--color-*: initial`、`static`）を生成器で再現することになり、新しい依存か自作の変換器が要る。

### B. 上書きできる範囲

1. **B1: アクセントの色だけ**（`accent` / `accent-hover` / `accent-fg` / `focus`）。最小で、壊れにくい。
2. **B2: 意味の色（`--color-*`）を、ライトとダークそれぞれに**（推奨）。`overlay` は除く。
3. **B3: 原色（`--palette-*`）まで**。原色は意味の色の実装の詳細で、名前や段階を変えると上書きが別の色を指すようになる。
4. **B4: 色に加えて、角丸・文字・余白も**。

### C. 検証

1. **C1: 保存のときにサーバー（Go）が、`make web-tokens` と同じ組でコントラストを検査して拒む**（推奨）。管理画面は同じ計算で入力中に結果を示す。
2. **C2: 管理画面（クライアント）だけで検査する**。API を直接呼べば、AA を満たさないテーマを保存できる。
3. **C3: 検査せず、警告だけにする**（Mastodon のカスタム CSS や Misskey のテーマと同じく、管理者の責任にする）。

### D. 配信

1. **D1: サーバーが同じオリジンのスタイルシートを返し、外枠とサーバーの画面が `<link rel="stylesheet">` で読む**（推奨）。
2. **D2: `GET /instance` で値を返し、クライアントが起動後に当てる**。最初の描画では既定の色が見え、JavaScript のないサーバーの画面には当たらない。
3. **D3: 外枠の `<style>` に埋め込み、CSP に `'sha256-…'` を足す**。テーマの版ごとに CSP が変わり、`style-src` を緩める。
4. **D4: 任意の CSS を許す**（Mastodon の管理画面の「カスタム CSS」。
   [`CustomCssController`](https://github.com/mastodon/mastodon/blob/main/app/controllers/custom_css_controller.rb) が設定の CSS をそのまま `text/css` で 1 か月の公開キャッシュ付きで返す。
   GoToSocial はアカウントごとのカスタム CSS を既定で無効にし、公開登録のサーバーでは無効のままにするよう[強く勧めている](https://docs.gotosocial.org/en/latest/configuration/accounts/)）。

他のサーバーの仕組み（2026-10-20 に一次資料で確認）:

- **Mastodon**: 任意の CSS を 1 つの設定に持ち、`/custom.css` のような同じオリジンのスタイルシートで配る（D4）。検査はない。
- **Misskey**: テーマは `props` のキーが CSS 変数になる JSON5 で（[テーマの説明](https://misskey-hub.net/en/docs/for-users/features/theme/)）、
  管理者はブランディングの画面で `themeColor` と、サーバーの既定のライト / ダークのテーマ（`defaultLightTheme` / `defaultDarkTheme`）を設定できる
  （[`Meta.ts`](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/models/Meta.ts)、
  [`branding.vue`](https://github.com/misskey-dev/misskey/blob/develop/packages/frontend/src/pages/admin/branding.vue)）。変数による上書きは B2 に近いが、コントラストの検査はない。
- **Pleroma / Akkoma**: Pleroma-FE の既定のテーマ名を `frontend_configurations` の `pleroma_fe` に書き、テーマの JSON を静的ファイルとして置く
  （[Theming your instance](https://docs.pleroma.social/backend/configuration/howto_theming_your_instance/)）。設定の変更には再起動が要る。
- **GoToSocial**: 管理者は `web/assets/themes` に CSS のテーマのファイルを置き、再起動する（[Themes](https://docs.gotosocial.org/en/latest/admin/themes/)）。

## Decision（決定）

> Proposed の間は推奨案である。Accepted にするのは人間の Decider のみ。

**A1・B2・C1・D1 を推奨する。任意の CSS（D4）は許さない。**

### 1. トークンの体系の強化

- `theme.css` をトークンの唯一の定義にする（A1）。上書きできるトークンは `@theme`（`inline` でない）に置き、ユーティリティが `var(--color-*)` を参照する状態を保つ。
  上書きは CSS 変数の値を替えるだけで効く。上書きできるトークンを `@theme inline` にしないことを `theme.css` の注記と検査で守る。
- `app/ui/scripts` に生成器（Bun）を足し、`theme.css` と `tokens.ts` から次を作る。生成物はコミットし、`make web-tokens` が古いと失敗する（`make i18n-gen` と同じ扱い）。
  - Go の `internal/domain/instance` の生成ファイル: 上書きできるトークンの一覧、それぞれのライト / ダークの既定値（`#rrggbb` に解決したもの）、コントラストの組と最小値。
    ドメインは標準ライブラリだけで、WCAG 2.2 の相対輝度とコントラスト比（`scripts/color.ts` と同じ式）を計算する。
  - クライアントの `@my/ui` の同じ一覧（管理画面のプレビューと入力の検査が使う）。`scripts/color.ts` のコントラストの計算は `src` へ移し、検査スクリプトと管理画面が共有する。
- 色以外のトークン（文字・余白・角丸・影・動き）は今の `theme.css` の値と app/ui/README.md の役割の表を正とし、検査を強める:
  `make web-tokens` は、任意の値の角丸・影・動きの時間（`rounded-[3px]`、`shadow-[…]`、`duration-[…]`）と、README にない余白の段階も拒む。
- Storybook の `Foundations/Tokens` に、上書きの例（検査を通る見本のテーマ）を当てた表示を足し、`make ui-test` がそれも両方のテーマで axe にかける。

### 2. 上書きできる範囲

- 意味の色 `--color-*` のうち `overlay` を除くすべてを、ライトとダークそれぞれに上書きできる（B2）。指定しないトークン・テーマは既定値のまま。
- 値は `#rrggbb` だけにする。sRGB の中にあることが書式で決まり、色域の検査が要らず、管理画面の色の入力（`<input type="color">` の値）と同じ形になる。不透明に限る（背景の組が半透明だと比が決まらないため）。
- 原色（B3）、角丸・文字・余白（B4）は上書きできない。原色は意味の色の実装の詳細で、文字・余白はロケールごとの行の高さ・320 px の横スクロール（D3）・スクリーンショットの検査に直接効くため。
- ロゴ・ファビコン・PWA のアイコンの差し替えはこの ADR に含めない（画像の扱いはメディアの ADR と別の Issue で決める）。

### 3. 検証（C1）

- 管理 API（`PUT` で上書きの全体を置き換える）が受け取った上書きを既定値と合わせ、生成した組のすべてをライトとダークで検査する。1 つでも最小値を下回れば
  保存せず、422 の Problem に下回った組（前景・背景・テーマ・比・最小値）を並べて返す（ADR-0016）。上書きしたトークンが既定のトークンとの組で落ちる場合も拒む。
- 管理画面は同じ一覧と計算で、入力中に組ごとの比を示し、下回る組があるうちは保存のボタンの前に理由を示す（保存の判定はサーバーが正）。
- 保存は ADR-0020 に従い、テーマのリビジョン（イベント）と、そのリビジョンの色の行（トークン・テーマ・値。トークンとテーマは CHECK で限る）と、現在のリビジョンの投影に分ける。
  色を JSON の列に入れない。テーブルの形は実装（#664）で data-model.md に書く。権限は既存の `manage_settings`（ADR-0058（#648）がモデレーションとサーバー管理を分けるときはサーバー管理の側）、変更は監査ログ（ADR-0037）に残す。

### 4. 配信（D1）

- `my serve` が、現在のテーマを `:root { --color-…: light-dark(#…, #…); … }` の 1 つの規則にしたスタイルシートを、リビジョンを含むパス（例 `/theme/{revision}.css`）で返す。
  内容はリビジョンで決まるので `immutable` で長期キャッシュする。上書きがないときは `<link>` を出さない。
  規則は `@layer` の外に置くので、Tailwind が `@layer theme` に出す既定値に勝つ。値は生成した一覧の名前と検査済みの `#rrggbb` だけから組み立て、利用者の文字列を CSS に入れない。
- SPA の外枠と、サーバーが描画する画面は、ビルドのスタイルシートの後に `<link rel="stylesheet">` で読む。スタイルシートは描画を止めるので最初の描画から当たり、`data-theme` と `light-dark()` の仕組みはそのまま効く。
  CSP（`style-src 'self'`）は変えない。外枠は閲覧者ごとに変わらない（インスタンスごとに 1 つ）ので、共有キャッシュも ETag（テーマのリビジョンを足す）もそのまま使える。
  保存から全員の外枠に届くまでは外枠のキャッシュ（最大 60 秒と再検証の 300 秒）の分だけ遅れる。
- Service Worker は `/theme/` のスタイルシートを実行時にキャッシュし（版付きなのでキャッシュ優先）、オフラインの外枠でもテーマが当たる。
- `theme-color`: 外枠は `media="(prefers-color-scheme: light)"` と `dark` の 2 つの `<meta name="theme-color">` に、テーマの `surface` のライト / ダークの値を書く。
  Web App Manifest の `theme_color` / `background_color` はテーマを 1 つしか持てないので、ライトの `surface` にする。`Options.ThemeColor` / `BackgroundColor` の既定値もトークンの既定の `surface` に合わせる。
  `data-theme` で OS と違うテーマを選んだときのブラウザの枠の色は OS の設定に従う（受け入れる制限）。
- `GET /instance` は、テーマのリビジョンと上書きの値（トークン・ライト・ダーク）も返す。管理画面の初期値と、スタイルシートを読めないクライアント（下記の Tauri）のためである。

### 5. Tauri のアプリ

- Tauri のアプリ（ADR-0008、FR-603）はビルドした UI を同梱し、接続するサーバーが実行時に変わるので、外枠の `<link>` は使えない。
  `GET /instance` の値から同じ規則を作り、構築できるスタイルシート（`new CSSStyleSheet()` と `document.adoptedStyleSheets`）で当てる。
  CSSOM による変更は D10 の禁止（`style` 属性・`cssText`）に当たらない。最初の描画の前に当てるため、前回の値をアプリの保存領域に残して起動時に先に当てる。
  詳細は Tauri の作業（#31 の検証の結果）で決める。

### この推奨を覆す条件

- 決定者が任意の CSS を必須とする場合（D4 を足し、コントラストの保証を外すことを受け入れる）。
- 管理者の多くがアクセントの色しか変えないと分かり、B2 の入力の多さ（17 トークン × 2 テーマ）が使いにくさになる場合は、B1 に絞り、アクセントから他の色を導く。
- Tailwind の版の変更で、テーマの変数がユーティリティから参照されなくなる場合（D1 の前提が崩れる）。

### 既存の ADR との関係

Accepted ADR の決定は置き換えない。ADR-0028 の「外枠を閲覧者ごとに変えない・CSP の nonce を使わない」と ADR-0019 §9 の CSP の範囲で、外枠に同じオリジンのスタイルシートを 1 つ足すだけである。

## Consequences（影響）

- 良い影響
  - 管理者はコードを変えずにブランドの色を当てられ、どの上書きでもライト / ダークの両方で AA が保たれる。
  - CSP・外枠のキャッシュ・JavaScript なしのサーバーの画面がそのまま動き、最初の描画からテーマが当たる。
  - 上書きはトークンの名前にだけ依存するので、部品の作り替えで壊れない。コントラストの組を足せば、サーバーの検査も同じ組で増える。
- 悪い影響・トレードオフ
  - 任意の CSS（Mastodon のカスタム CSS）に慣れた管理者には自由度が低い。フォント・角丸・レイアウトは変えられない。
  - `app/ui` の変更が Go の生成ファイルを古くする。`scripts/changed-contexts.sh` で `app/ui` の変更が `go-check` も選ぶか、生成物の検査を web 側に置くかを実装で決める。
  - 1 つの色を変えると別の組が落ちることがあり（`accent` を明るくすると `accent-fg` も替える必要がある）、管理画面にそれを分かりやすく示す作業が要る。
  - 外枠のキャッシュの分だけ、保存してから全員に届くまで数分遅れる。
  - ブラウザの枠の色（`theme-color`）は `data-theme` の選択に追従しない。
- 後続で必要になる作業（Issue 化する）
  - #664: 生成器と生成物の検査、Go のドメインの検査、管理 API と管理画面、スタイルシートの配信、外枠・サーバーの画面・Service Worker・マニフェストの変更、
    app/ui/README.md と web-client.md（§8・§9・§11・§14）・data-model.md・api/openapi.yaml の更新。
  - `make web-tokens` の色以外のトークンの検査の強化と、Storybook の上書きの見本（#664 に含めるか別の Issue にする）。
  - 見本のテーマを当てた E2E（axe を両方のテーマで）。
  - Tauri のアプリでの適用（Tauri の作業の Issue に含める）。

## Pros and Cons of the Options（詳細比較・任意）

| | 長所 | 短所 |
| --- | --- | --- |
| A1 | Tailwind v4 の書き方のまま、生成器は読むだけ。依存が増えない | CSS を正規表現で読む生成器（今の `check-tokens.ts` と同じ）に依存する |
| A2 | 言語に依らない定義。Go からも直接読める | `@theme` の書き方の再現が要り、新しい依存（ADR が要る）か自作の変換器が増える |
| B1 | 入力が少なく、壊れにくい | 背景や文字の色を変えたいサーバーに足りない。決定者の「トークン上書き」より狭い |
| B2 | 意味のある単位で変えられ、組の検査がそのまま使える | 入力が多く、組の依存を管理画面で示す必要がある |
| B3 | 原色の段階ごと替えられる | 原色の名前と段階が実装の詳細で、版の変更で上書きの意味が変わる |
| B4 | 見た目を大きく変えられる | ロケールの行の高さ・横スクロール・スクリーンショットの検査を上書きのたびに保証できない |
| C1 | API を直接呼んでも AA が保たれる。クライアントと同じ組 | Go にコントラストの計算と生成物が要る |
| C2 | サーバーの実装が少ない | API から不正なテーマを保存できる |
| C3 | 最も自由 | ADR-0027 のゲートを管理者の入力が破れる |
| D1 | CSP・キャッシュを変えず、最初の描画から当たり、JavaScript なしの画面にも当たる | 外枠のキャッシュの分だけ反映が遅れる。リクエストが 1 つ増える |
| D2 | サーバーの変更が少ない | 最初の描画で既定の色が見え、サーバーの画面には当たらない |
| D3 | リクエストが増えない | CSP をテーマの版ごとに変え、`style-src` を緩める |
| D4 | 最も自由（Mastodon と同じ） | コントラスト・axe・RTL・スクリーンショットのゲートを保証できず、版の変更でセレクタが壊れ、偽の表示（フィッシング）にも使える |

## Open Questions / 要検証

- 上書きできる範囲を B2（`overlay` を除く意味の色すべて）にするか、B1（アクセントだけ）から始めるか。
- 状態の色（`danger` / `success` / `warning`）を上書きの対象に含めるか。意味が広く共有された色で、変えると利用者が誤解するおそれがある。
- Tailwind v4 のビルドで、`@theme static` の意味の色の `:root` の値を、`@layer` の外の `:root` の規則で上書きするとユーティリティの色が替わることを、実装の最初に E2E で確かめる（要検証。
  [Theme variables](https://tailwindcss.com/docs/theme) はテーマの変数が通常の CSS 変数として出力されることだけを述べている）。
- Tauri の WebView（WKWebView・Android WebView）で、CSP の下で `adoptedStyleSheets` が使えること（要検証。#31 の検証で確かめる）。
- テーマの有無を要件（`01-functional.md`）に新しい FR として足すか。
