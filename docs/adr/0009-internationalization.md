# ADR-0009: 国際化（i18n）と地域化（l10n）

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #5

## Context

多様な言語の利用者が使う（G5）。後付けの i18n は文字列の連結・日付整形・レイアウトの作り直しを招くため、最初期から仕組みに組み込む。
i18n の対象は UI 文字列だけでなく、サーバー生成文言（メール・API エラー・通知）、投稿の言語メタデータ、検索、書字方向まで及ぶ。

## Decision（推奨案）

### 1. メッセージカタログ

- 形式: **ICU MessageFormat**（MF1。MessageFormat 2 は成熟したら移行を検討）。複数形・select は CLDR 規則に従う。
- 配置: リポジトリ直下 `locales/<BCP47>/<namespace>.json`。**ソースロケールは `en`**（翻訳者人口のため）、`ja` はメンテナが同時に更新する。
- キーは意味ベース（`timeline.empty.home`）。文字列の連結・断片の組み合わせは禁止し、1 文 = 1 メッセージ。
- サーバー（Go）・Web・モバイル（TS）で **同一カタログファイルを共有** する（namespace でサーバー専用 / クライアント専用を分ける）。
  Go 側は ICU MessageFormat を解釈できるライブラリ（または自前の最小実装）を使い、カタログはバイナリに `embed` する。
- 型安全: キーとプレースホルダの型を生成し、存在しないキーや引数不足をコンパイル時に検出する（ライブラリは実装 Issue で選定。候補: FormatJS, Lingui, Paraglide 等。#20 の推奨は Open Questions を参照）。
- CI 検査: (a) `en` に対する未翻訳キーの一覧（失敗にはしない）、(b) プレースホルダの不一致・ICU 構文エラー（失敗にする）、(c) 未使用キー。
- **疑似ロケール**（`en-XA`: 文字列を 30〜40% 伸ばしアクセント付与、`ar-XB`: RTL 疑似）を開発・E2E テストで使う。

### 2. ロケール解決

- 優先順: 利用者設定 → `Accept-Language` → インスタンス既定 → `en`。
- フォールバックチェーン: `zh-Hant-TW` → `zh-Hant` → `zh`（※ 簡体字にフォールバックしない例外表を持つ）→ `en`。
- メール・Push 等の非同期処理は「受信者の」ロケールで生成する（ジョブペイロードにロケールを持たせず、実行時に受信者設定を引く）。

### 3. 整形

- 日付・時刻・数値・相対時刻・リスト・単位は `Intl.*` を使う。タイムゾーンは利用者設定（IANA）。サーバーは常に UTC で保持。
- ハードコードされた日付フォーマット文字列を禁止（lint）。

### 4. 投稿の言語

- 投稿は BCP 47 の言語タグを持つ（FR-110）。ActivityPub では `contentMap` で表現し、受信時は `contentMap` / `@language` から取得。
- 言語でタイムラインをフィルタ可能にする（FR-208）。クライアント側での言語自動推定は任意。

### 5. 書字方向・タイポグラフィ

- CSS は論理プロパティ（`margin-inline-start` 等）のみを使う（lint）。ルート要素の `dir` をロケールから設定。
- 利用者生成コンテンツ（表示名・投稿・プロフィール）は `dir="auto"` で個別に方向判定。
- フォントは OS フォントを優先し、CJK・アラビア文字等のフォールバックスタックを定義する。

### 6. 文字列と識別子

- ユーザー名（`@name`）は連合互換のため `[A-Za-z0-9_]` に限定し、比較は小文字正規化。表示名は任意の Unicode（NFC 正規化）。
- ドメインは IDN を Punycode で保存し、表示時に Unicode へ変換。
- ハッシュタグは NFKC + 小文字化で正規化して照合（表示は元の表記）。

### 7. 検索（CJK）

- v0 のアカウント・タグ検索は前方一致 / 部分一致。SQLite は FTS5 の trigram トークナイザ、PostgreSQL は `pg_trgm`
  （利用可能な環境では `pg_bigm`）を想定。分かち書きに依存しない方式を既定にする。

### 8. 翻訳ワークフロー

- OSS 向けホスティング（Weblate の Libre プラン / Crowdin の OSS プラン）を使い、翻訳者がコードを触らずに貢献できるようにする。選定は別 Issue。
- 翻訳の取り込みは自動 PR。

## Consequences

- 初期実装コストは増えるが、後付けより圧倒的に安い。
- エージェントには「UI 文字列を書いたら必ずカタログに追加し、`en` と `ja` を更新する」ルールを課す（AGENTS.md）。

## Open Questions / 要検証

- [x] #20 i18n ライブラリ選定（Go 側: ICU MessageFormat の複数形 / select 対応、TS 側: Web / Tauri で共有）
  → [spike レポート](../spikes/20-i18n-libraries.md)。推奨案:
  Go は `github.com/kaptinlin/messageformat-go/mf1`（MIT。FormatJS と共有カタログ 96/96・CLDR 12 ロケール 132/132 で出力一致）を自前パッケージの裏に隠して使う。
  TS は FormatJS `intl-messageformat`（BSD-3-Clause、AST 事前コンパイルでランタイム 2.6 KiB gzip）。
  カタログは Format.JS JSON 形式（`{"key": {"defaultMessage", "description"}}`）。型付きアクセサは 1 つの生成器から Go / TS の両方に生成し、生成物をコミットする。
  疑似ロケールは FormatJS の AST 変換で生成し、`en` の CLDR 規則で整形する。
- [ ] Go 側ライブラリの保守リスク（保守者が実質 1 名、v0.x）とバイナリ +5.1 MB / RSS +7 MB を受け入れるか。
  受け入れる場合も、FormatJS との出力一致テストと CLDR ドリフトテストを CI に常設し、フォーク / 自前実装（3〜5 人日の見積もり）へ切り替えられるようにする
- [ ] Go と TS で出力が一致する「共有サブセット」の採用: サーバーと共有する namespace ではリッチテキストタグ・数値スケルトン（`::`）・`date` / `time` 引数を禁止し、
  日付は整形済み文字列で渡す（lint で強制）。namespace の分け方（`server` / `shared` / `client`）もあわせて決める
- [ ] Weblate / Crowdin の選定（spike の推奨は Weblate の Libre プラン: GPL-3.0 でセルフホスト可、Format.JS JSON と ICU 検査をネイティブサポート）
- [ ] v0 で `en` / `ja` 以外に追加する言語（コミュニティ次第）
