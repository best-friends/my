# Spike #20: i18n ライブラリの選定（Go サーバーと TS クライアントでカタログを共有）

- Issue: [#20](https://github.com/best-friends/my/issues/20)
- 関連 ADR: [ADR-0001](../adr/0001-server-language-and-runtime.md)（Accepted）, [ADR-0008](../adr/0008-web-and-mobile-clients.md), [ADR-0009](../adr/0009-internationalization.md), [ADR-0011](../adr/0011-license.md)（Accepted）
- 検証コード: [`spikes/20-i18n-libraries/`](../../spikes/20-i18n-libraries/)（使い捨て。プロダクトコードから import しない）
- 調査日: 2026-10-16（バージョン・最終更新日はこの日時点）

## 結論（推奨）

| 領域 | 推奨 | 理由（要約） |
| --- | --- | --- |
| Go | **`github.com/kaptinlin/messageformat-go/mf1`**（MIT）を自前の薄い `i18n` パッケージの裏に隠して使う | 検証した Go 実装で唯一、FormatJS と **全ケースで同じ出力**（共有カタログ 96/96、12 ロケールの CLDR 複数形・数値 132/132）。plural / select / selectordinal / `offset` / `=N` / `#` / `{n, number}` に対応。`CGO_ENABLED=0` で動く |
| TS | **FormatJS `intl-messageformat`**（BSD-3-Clause）+ ビルド時 AST 事前コンパイル + **自前コード生成** | ICU MF1 の事実上の参照実装。事前コンパイル時のランタイムは 2.6 KiB（min+gzip）。`eval` / `new Function` を使わないので Tauri の CSP と両立。UI フレームワークに依存しない |
| カタログ | `locales/<BCP47>/<namespace>.json` を **Format.JS JSON 形式**（`{"key": {"defaultMessage", "description"}}`）で持つ | Weblate がネイティブに読み、ICU MessageFormat 検査を自動で有効にする。説明文（翻訳者向けコンテキスト）を持てる |
| 型生成 | **1 つの生成器（Bun + FormatJS パーサー）** で `messages.gen.ts` と `messages_gen.go` を同時に生成し、生成物をコミット | 2 言語の型が同じ AST から出るため食い違わない。`go build` に Bun は不要 |
| 疑似ロケール | `@formatjs/cli-lib` の `en-XA` / `en-XB` 変換（AST 上で変換）で `en-XA` / `ar-XB` を生成 | プレースホルダ・`#`・plural キーワードを壊さない |
| 翻訳基盤 | **Weblate**（Hosted の Libre プラン、だめなら自前ホスト） | 自身が GPL-3.0 の OSS でセルフホスト可能。Format.JS JSON と ICU 検査をサポート。Crowdin はプロプライエタリ SaaS で OSS ライセンスに審査条件あり |

主なリスクは Go 側ライブラリの **保守者が実質 1 人**（コミットの 9 割超が 1 名）で v0.x であること、と **バイナリ +5.1 MB / RSS +7 MB**。
自前 `i18n` パッケージで API を隠し、FormatJS との出力一致テスト（本 spike の `parity_test.go` 相当）を CI に置くことで、フォーク（MIT）や自前実装への切り替えを可能にしておく。

## 1. Go: ICU MessageFormat ライブラリ

### 1.1 候補一覧

| ライブラリ | ライセンス | 最終更新 | ICU MF1 構文 | 備考 |
| --- | --- | --- | --- | --- |
| **kaptinlin/messageformat-go/mf1** v0.8.6 | MIT | 2026-08（活発） | ○ plural / select / selectordinal / offset / `=N` / `#` / number・date・time スタイル | messageformat.js（`@messageformat/core`）の移植。CLDR は `agentable/go-intl`（MIT, ECMA-402 の Go 実装）。本体は MF2 実装で、MF1 は独立モジュールとして「非推奨ではない」と明記。保守者は実質 1 名 |
| kaptinlin/go-i18n | MIT | 2026-08 | ○（上記 mf1 を使用） | バンドル・フォールバック・`Accept-Language` 判定付き。ロケール解決は ADR-0009 の例外表を持てないため、mf1 を直接使う方が合う |
| gotnospirit/messageformat | BSD-2-Clause | 2023-01（実質停止） | △ `{n, number}` 非対応、ICU のアポストロフィ規則非対応（`\` エスケープ） | CLDR 規則は 2018 年の `makeplural`。フォークもすべて停止（最新 2023-04） |
| fullpipe/icu-mf v1.0.1 | MIT | 2025-12 | ○ 構文はほぼ対応 | `golang.org/x/text` ベース。`#` が桁区切りされない、アポストロフィ処理の差、古い CLDR（fr の `many` 等が無い）、ar の数字が FormatJS と異なる。YAML カタログ前提 |
| golang.org/x/text/message (+ `feature/plural`, `catalog`) | BSD-3-Clause | 活発 | ✕ ICU 構文を解釈しない | `plural.Selectf` による Go 独自のカタログ。`gotext` 抽出は Go コード起点で TS と共有できない。CLDR データも古い |
| nicksnyder/go-i18n v2.6.1 | MIT | 2026-10（活発） | ✕ | 複数形は `one` / `other` フィールド + `text/template`。select・selectordinal・ネストなし。ICU 文字列を TS と共有できない |
| modern-go/msgfmt | Apache-2.0 | 2018 | △ | 停止 |
| go-slim/icu | ライセンス表記なし | 2025-11 | ? | ライセンス不明のため対象外 |
| iawaknahc/gomessageformat | — | 2021 | ○ | icu4c を cgo で呼ぶ。`CGO_ENABLED=0` 方針に反するため対象外 |

### 1.2 実測（`go test -v ./...`）

同じカタログ（`en` / `ja` / `en-XA` / `ar-XB` × 24 ケース）と、12 ロケール（en, ja, fr, pt-PT, ru, pl, cy, ar, he, zh-Hant, de, hi）× 11 数値の CLDR 複数形・数値整形を、FormatJS（Bun 上の `intl-messageformat`）の出力と比較した。

| Go エンジン | 共有カタログ（96 出力） | CLDR 複数形 + 数値（132 出力） | `CGO_ENABLED=0` バイナリ増分 | RSS 増分 |
| --- | ---: | ---: | ---: | ---: |
| **kaptinlin/mf1** | **96/96** | **132/132** | +5.1 MB | +7.3 MB |
| fullpipe/icu-mf | 88/96 | 105/132 | +1.3 MB | +2.0 MB |
| gotnospirit/messageformat | 80/96 | 0/132（`{n, number}` 非対応） | +0.2 MB | +0.2 MB |

（`-trimpath -ldflags="-s -w"`、エンジンなしの基準バイナリ 1.5 MB / RSS 2.0 MB、linux/amd64。`spikes/20-i18n-libraries/cmd/sizeprobe` で再現）

kaptinlin/mf1 の追加の利点:

- `StrictPluralKeys`（既定 ON）で **そのロケールに存在しない複数形カテゴリ**（例: ja の `one`）を構文エラーにする。`RequireAllArguments` で引数不足をエラーにできる。起動時に全メッセージをコンパイルすれば、カタログの誤りが起動時 / テスト時に落ちる（`bundle.go`）。
- `NewParser(...).Parse()` で AST（トークン列）を取れるため、将来 Go 側に lint や生成器を持たせることも可能。

### 1.3 FormatJS と出力が一致しない構文（共有サブセットの外）

`subset_test.go` で固定した。サーバーとクライアントで共有する namespace ではこれらを **使わない**（`ts/check.ts` がエラーにする）。

| 構文 | Go（mf1） | FormatJS |
| --- | --- | --- |
| リッチテキストタグ `<b>{name}</b>` | タグを文字列のまま出力 | タグ名のコールバックを呼ぶ |
| 数値スケルトン `{n, number, ::percent}` / `::compact-short` / `::currency/JPY` | エラー | 対応 |
| `{d, date, medium}` | エラー | 対応 |
| `{d, date, short}` | `10/16/2026` | `10/16/26`（出力が違う） |

共有サブセット = 平文引数、`{n, number}`、`{n, number, integer|percent}`、`plural` / `selectordinal`（`offset`・`=N`・`#` 含む）、`select`、それらのネスト、ICU のアポストロフィエスケープ。
日付・時刻は ADR-0009 §3 のとおり `Intl.*`（Go ではサーバー側の整形関数）で整形した文字列を引数として渡す。

### 1.4 自前実装する場合の見積もり（不採用時の退路）

| 部品 | 規模の目安 | 備考 |
| --- | --- | --- |
| MF1 パーサー（上記共有サブセット + アポストロフィ規則） | 500〜800 行 | FormatJS パーサーのテストを移植して互換性を担保 |
| CLDR 複数形規則（cardinal / ordinal） | 生成器 200 行 + 生成コード | CLDR `plurals.json` / `ordinals.json` の規則式をコード生成。`x/text/feature/plural` は CLDR が古く不足 |
| 数値整形（桁区切り・小数点・数字体系） | 300〜500 行 + CLDR データ生成 | `x/text/number` も古い。ロケール数を絞ればデータは小さい |
| 合計 | 実装 3〜5 人日 + CLDR 更新の継続保守 | 上記の CLDR ドリフトテスト（`cldr_test.go`）を CI に置けば追従漏れを検出できる |

現時点では kaptinlin/mf1 の方が安く、品質も実測で上回るため自前実装は退路として残す。

## 2. TS: クライアント側ライブラリ

### 2.1 比較

| 候補 | ライセンス | ICU MF1 | 共有 JSON をそのまま読めるか | キー / 引数の型 | 抽出 | Tauri（WebView） |
| --- | --- | --- | --- | --- | --- | --- |
| **FormatJS `intl-messageformat`** 12.1.3 | BSD-3-Clause | ◎ 参照実装 | ◎ Format.JS JSON がそのまま入力。`formatjs compile --ast` で事前コンパイル | キーは型拡張で可、引数の型は無し → **自前生成で補う** | `@formatjs/cli`（コード起点。本プロジェクトはカタログ起点なので不要） | ◎ `Intl.PluralRules` / `NumberFormat` のみ使用、`eval` 不使用 |
| Lingui `@lingui/core` 6.9.1 | MIT | ○ | △ 既定は PO。JSON（minimal）も可だが、カタログはマクロで **コードから抽出** する前提 | キーは原文または明示 ID、引数型なし | `lingui extract`（Babel / SWC マクロ） | ○ |
| Paraglide JS 2.29 | MIT | △ `@inlang/plugin-icu1` で ICU1 を inlang のデータモデルに変換 | △ `project.inlang/settings.json` と plugin 設定が必要。plugin は CDN から読み込む方式。plugin の npm にライセンス表記がなく monorepo のライセンスも GitHub で判定されない（要確認） | ◎ メッセージごとの関数を生成（キーも引数も型付き、tree-shake 可） | 不要（カタログ起点） | ◎ |
| i18next 26 + i18next-icu 2.5 | MIT | ○（内部で intl-messageformat） | ○ ただし i18next 独自のネスト / 名前空間 / `_one` 接尾辞の慣習と混ざる | キーは型拡張で可、ICU 引数の型は無し | `i18next-parser` 等 | ○ |
| `@messageformat/core` 3.4 | MIT | ◎（Go 側 mf1 の元実装） | ○ | 無し | 無し | ✕ 実行時コンパイルが `new Function` を使い、CSP（`unsafe-eval` なし）と衝突。ビルド時コンパイルなら可 |

### 2.2 バンドルサイズ実測（`bun run size`、複数形メッセージ 1 件を整形する最小構成、Bun bundler・browser・minify）

| 候補 | min (KiB) | min+gzip (KiB) |
| --- | ---: | ---: |
| FormatJS intl-messageformat（実行時パース） | 33.1 | 9.7 |
| **FormatJS intl-messageformat（事前コンパイル AST、no-parser）** | **7.4** | **2.6** |
| Lingui @lingui/core（事前コンパイル） | 4.9 | 2.1 |
| Lingui @lingui/core + 実行時 compileMessage | 24.3 | 8.8 |
| i18next + i18next-icu | 82.2 | 25.3 |
| @messageformat/core（実行時コンパイル） | 71.7 | 20.1 |

Paraglide はメッセージ単位の関数を生成するためランタイムはほぼゼロ（メッセージ数に比例）。プロジェクト設定と plugin のライセンス確認が必要なため今回は実測していない。

### 2.3 推奨の理由

- 本プロジェクトは **カタログが正（source of truth）で、Go と TS が同じファイルを読む**。コードから抽出する Lingui / formatjs extract の流れとは向きが逆。
- FormatJS は ICU MF1 の参照実装で、Weblate の ICU 検査とも同じ方言。Go 側の一致テストの基準（golden）にもできる。
- 型安全はライブラリに頼らず、§3 の生成器で Go と同じ AST から作る。Paraglide は型生成の発想は最も近いが、ICU は plugin 経由の変換になり、Go との意味一致を別途保証する必要がある。
- UI フレームワークは未選定（ADR-0008）。`intl-messageformat` 単体なら React / Solid / Svelte のどれにも薄いラッパーで載る。

## 3. 共有カタログ・型生成・疑似ロケール・CI 検査

### 3.1 形式と配置

```text
locales/
  en/                 # ソースロケール（Weblate の monolingual base）
    server.json       # サーバー専用（メール・API エラー・通知）
    shared.json       # 両方で使う
    client.json       # クライアント専用。共有サブセット外（タグ・スケルトン等）を許す
  ja/ ...
  en-XA/ ar-XB/       # 生成物（Weblate の対象外）
```

各ファイルは Format.JS JSON 形式。キーは ADR-0009 のとおり意味ベース（`notifications.unread`）。ID は `<namespace>:<key>`。

```json
{
  "notifications.unread": {
    "defaultMessage": "{count, plural, =0 {No unread notifications} one {# unread notification} other {# unread notifications}}",
    "description": "Badge tooltip on the notifications tab."
  }
}
```

Go は `embed` で読み、起動時に全ロケール・全メッセージをコンパイルする（`catalog.go`, `bundle.go`）。
TS はビルド時に `formatjs compile --ast` 相当で AST にしてバンドルし、ランタイムは no-parser 版を使う。

### 3.2 型付きアクセサの生成（`ts/gen.ts`）

ソースロケール `en` の各メッセージを FormatJS パーサーで解析し、引数名と型を推論して両言語のコードを出す。

| ICU での使われ方 | TS の型 | Go の型 |
| --- | --- | --- |
| `{x}` | `string` | `string` |
| `{x, plural}` / `{x, selectordinal}` | `number` | `int` |
| `{x, number}`（plural と併用含む） | `number` | `float64` |
| `{x, select, a {...} b {...} other {...}}` | `"a" \| "b" \| (string & {})` | `string` |
| `{x, date}` / `{x, time}`（client のみ） | `Date \| number` | `time.Time` |
| `<tag>`（client のみ） | `(chunks) => string` | — |

生成例（Go）:

```go
// DemoNotificationsUnread formats "demo:notifications.unread".
func (l *Localizer) DemoNotificationsUnread(count int) string
```

生成例（TS）: `t("demo:notifications.unread", { count: 3 })`。存在しないキー・引数不足・型違いは `tsc` でエラーになる（`ts/usage.ts` の `@ts-expect-error` で検証）。
同じ引数が異なる型で使われた場合は生成器がエラーにする。

### 3.3 疑似ロケール（`ts/pseudo.ts`）

- `en-XA`: `@formatjs/cli-lib` の `en-XA`（アクセント付与・母音重複で約 40% 伸長・`[...]` で囲む）。
- `ar-XB`: 同 `en-XB`（文字の反転 + RLO / PDF 制御文字で RTL 表示）を `ar-XB` として出力。
- どちらも AST 上で変換し `printAST` で ICU 文字列に戻すため、引数・`#`・plural キーワードは保たれる。
- **整形ロケールは `en`**: 疑似ロケールは `en` の複数形カテゴリを持つので、`ar` の CLDR 規則で整形すると不正なカテゴリ（ar の序数は `other` のみ）になる。Go・TS とも `en-XA` / `ar-XB` は `en` の規則で整形し、`dir` だけを切り替える（`FormattingLocale` / `formattingLocale`）。
- 生成物は Weblate の対象外。本番ビルドに含めるかはビルドタグ等で切り替える（実装 Issue で決める）。

### 3.4 CI 検査（`ts/check.ts` で試作済み）

| 検査 | 判定 | 実装方法 |
| --- | --- | --- |
| ICU 構文エラー（`other` 欠落を含む） | 失敗 | FormatJS パーサー（`requiresOtherClause`）。Go 側は起動時コンパイルのテストでも検出 |
| プレースホルダの不一致（翻訳に未知の引数・型違い） | 失敗 | ソースとの引数集合・型の比較 |
| ロケールに存在しない複数形カテゴリ（ja の `one` 等） | 失敗 | `Intl.PluralRules#resolvedOptions().pluralCategories`。Go 側 mf1 の `StrictPluralKeys` でも検出 |
| ソースに無いキー（廃止キーの残骸） | 失敗 | キー集合の差分 |
| 共有 namespace での共有サブセット外の構文 | 失敗 | AST 走査（タグ・スケルトン・date / time） |
| 生成済みアクセサが古い | 失敗 | 再生成して差分比較 |
| 未翻訳キー | 警告（ADR-0009 のとおり失敗にしない） | キー集合の差分。実行時は `en` へフォールバック |
| 翻訳がソースの引数を使っていない | 警告 | ja で `gender` を使わない等、正当な場合がある |
| 未使用キー | 警告 | 型付きアクセサ経由に限定すれば機械的に追える: Go は `golang.org/x/tools/cmd/deadcode`（エクスポート済みメソッドも到達不能なら報告）、TS はキー文字列リテラルの検索。本 spike では未実装 |
| Go と TS の出力一致 | 失敗 | 本 spike の `parity_test.go`（golden を FormatJS で生成し Go で比較）と `cldr_test.go`（CLDR ドリフト）を常設する |

壊したカタログで実行した例:

```text
error: ja/demo.json greeting: unknown argument {nam}
error: ja/demo.json notifications.unread: cardinal category "one" does not exist in ja
error: ja/demo.json follow.rank: EXPECT_ARGUMENT_CLOSING_BRACE
error: ja/demo.json profile.followers: {count} is string, source has number
error: ja/demo.json removed.key: not in source locale (obsolete)
warn:  ja: untranslated demo:profile.moved
```

注意: クライアントの出力は **実行環境の `Intl`（ICU / CLDR）に依存する**。iOS の WKWebView、Android System WebView、WebView2、WebKitGTK はそれぞれ OS 更新に合わせて CLDR が変わるため、Go との「バイト単位の一致」は CI（Bun）上でのみ保証できる。
複数形カテゴリの選択は CLDR の版で変わりにくいが、数値の区切り文字（fr の U+202F など）は変わり得る。メールや通知本文などサーバー生成文言とクライアント表示を突き合わせる仕様は作らない。

## 4. 翻訳プラットフォーム

| 観点 | Weblate | Crowdin |
| --- | --- | --- |
| ソフトウェアのライセンス | GPL-3.0（セルフホスト可） | プロプライエタリ SaaS |
| OSS 向け無償枠 | Hosted Weblate の Libre プラン（Libre ライセンスの公開プロジェクト、申請・審査あり） | OSS ライセンス（無償）。条件: OSI 承認ライセンス、公開ソース、申請者がプロジェクトリード、3 か月以上の活動実績、活発なコミュニティ、定期リリース、ベータ参加と Global TM への訳の提供 |
| 本カタログ形式 | **Format.JS JSON**（Weblate 5.4+）をネイティブサポート。ICU MessageFormat 検査が自動で有効（構文・プレースホルダ欠落 / 過剰・型） | JSON（フラット / ネスト）と ICU 構文の編集・自動エラー検出に対応。Format.JS の `{defaultMessage, description}` 形をそのまま扱えるかは要検証 |
| namespace の扱い | Component discovery で `locales/(?P<language>[^/]*)/(?P<component>[^/]*)\.json` を 1 namespace = 1 コンポーネントにできる | ファイルごとに設定 |
| 取り込み | Git に直接コミット / GitHub に PR（ADR-0009 の「自動 PR」に合う） | GitHub 連携で l10n ブランチと PR を作る |
| AGPL / 理念との相性 | 高い（Fediverse 系 OSS の採用例が多い） | 中（ベンダーロックイン、条件付き無償） |

推奨は **Weblate**。Hosted の Libre プランが使えない場合も、同じ設定のまま自前ホストへ移行できる。
Libre プランの申請条件の詳細と、Format.JS JSON で `en` を monolingual base にした場合の運用（新規キーの自動追加・削除の扱い）は申請時に確認する。

## 5. ADR-0009 への反映

- Open Questions の #20 を解決済みにし、本レポートの推奨と追加の検討事項を記載した（ステータスは Proposed のまま）。
- 決定は人間の decider が行う。採用する場合の実装 Issue 候補:
  1. `internal/i18n`（仮）: kaptinlin/mf1 を隠す Go パッケージ、起動時コンパイル、ADR-0009 のフォールバック（例外表を含む）
  2. カタログ生成器（`gen` / `pseudo` / `check`）をリポジトリ直下のツールとして実装し `make check` に組み込む
  3. Go と FormatJS の一致テスト・CLDR ドリフトテストの常設
  4. Weblate（Libre プラン）の申請と Component discovery 設定

## 再現手順

```sh
cd spikes/20-i18n-libraries
bun install
bun run test      # FormatJS golden 検証 + bun test + tsc（型レベルテスト）+ go test
go test -v ./...  # エンジン別の一致表
bun run check     # カタログ lint
bun run size      # TS バンドルサイズ表
```
