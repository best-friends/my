# インスタンス設定

運営者向けの手順です。要件は FR-001・FR-101・FR-202・FR-408・FR-704・FR-705・FR-718（[機能要件](../requirements/01-functional.md)）、
テーブルは [データモデル §4.9](../architecture/data-model.md#49-instance) です。

## 設定（環境変数）とインスタンス設定（DB）の分担

| | 設定 | インスタンス設定 |
| --- | --- | --- |
| 内容 | デプロイの構成: 役割・アダプタ・接続先・ホスト名・秘密情報 | 運営の方針: 登録モード・文字数上限・既定のロケール・タイムラインの有効 / 無効、インスタンスの名前・説明・規約 |
| 置き場所 | 環境変数 `MY_*` と設定ファイル（[設定リファレンス](../reference/configuration.md)、ADR-0010） | DB。変更はすべて版として残る |
| 変更 | プロセスの再起動で反映 | `my instance set`・Web クライアントの管理画面・クライアント API で即時に反映（[管理画面とクライアント API](#管理画面とクライアント-api)） |
| 未設定のとき | スキーマの既定値（必須なら起動しない） | コードで定義した既定値 |

- **同じキーが両方にあることはありません。** どちらかがもう一方を上書きする優先順位は存在しません。
  環境変数や設定ファイルでインスタンス設定を与えることはできず、DB に設定（環境変数）の値を置くこともできません。
  両方のキーが重ならないことはテスト（`TestConfigAndInstanceSettingsShareNoKey`）で保証します。
- 新しい値をどちらに置くかは、「再起動なしに運営者が変えるものか」で決めます。デプロイごとに決まり、変えるとプロセスの構成が変わるものは設定、
  運営中に変えるものはインスタンス設定です。

## 前提

- 設定が [設定リファレンス](../reference/configuration.md) どおりに与えられている（`my config check`）。
- DB にスキーマが適用済み（Atlas で `schema/<dialect>.sql` を適用する。[ADR-0023](../adr/0023-declarative-schema-management.md)）。

## キー

### 設定値（1 キー 1 値）

| キー | 型 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `registration_mode` | `open` / `approval` / `invite` / `closed` | `approval` | 登録モード（FR-001、[利用者登録](registration.md)） |
| `max_status_length` | 整数 1〜100000 | `500` | 投稿の文字数上限（FR-101） |
| `default_locale` | BCP 47 言語タグ | `en` | 既定のロケール。新しい利用者の UI ロケールの初期値、利用者の希望に合うカタログが無いとき、インスタンスの文章がその言語に無いときに使う（ADR-0009） |
| `local_timeline_enabled` | `true` / `false` | `true` | ローカルタイムラインを提供するか（FR-202） |
| `federated_timeline_enabled` | `true` / `false` | `true` | 連合タイムラインを提供するか（FR-202） |
| `preview_image` | メディア ID（空 = 未設定） | 空 | 画像のない投稿・プロフィールのリンクプレビュー画像（`og:image`）。未設定なら同梱の既定画像（[リンクプレビュー](link-previews.md)） |
| `contact_account` | ローカルアカウントのユーザー名（空 = 未設定） | 空 | 運営への連絡先のアカウント。[連絡先とサムネイル](#連絡先とサムネイル) |
| `contact_email` | メールアドレス（空 = 未設定） | 空 | 運営への連絡先のメールアドレス |
| `thumbnail` | メディア ID（空 = 未設定） | 空 | インスタンスを表す画像（サムネイル） |
| `remote_media_policy` | `none` / `avatars` / `all` | `none` | リモートのメディアの複製を持つか（FR-409、ADR-0005）。`none` はメディアプロキシが毎回中継し、`avatars` はアバター・ヘッダー・カスタム絵文字、`all` はすべてを `remote_media_cache_ttl` の間 Blob にキャッシュする。キャッシュの合計の上限は設定 `remote_media_cache_max_bytes`（[federation.md §9](../architecture/federation.md#9-リモートメディア)） |
| `remote_media_cache_ttl` | 整数 60〜31536000（秒） | `86400` | 取得したリモートのメディアを持ってよい期間。メディアプロキシのレスポンスの `Cache-Control: max-age` で、CDN とブラウザがこの間キャッシュする（[federation.md §9](../architecture/federation.md#9-リモートメディア)） |
| `authorized_fetch` | `true` / `false` | `false` | Authorized Fetch（secure mode、FR-408）。`true` にすると、検証できる HTTP 署名の無い ActivityPub の GET に、オブジェクトとコレクションは `401` で、利用者の Actor は鍵の取得のための最小の文書で答える（[federation.md §10](../architecture/federation.md#10-コレクションとオブジェクトの-get)）。インスタンス Actor は署名なしでも文書全体を返すので、署名の検証のための鍵の取得はどのサーバーからもできる |
| `domain_actions_visibility` | `none` / `users` / `all` | `none` | 処分したドメインの一覧（`GET /api/v1/instance/domain_actions`）を誰に見せるか: 誰にも / ログインした利用者に / 誰にでも（FR-718、[ドメイン単位の処分と通報の対応](moderation.md#処分したドメインの公開)） |
| `domain_actions_rationale_visibility` | `none` / `users` / `all` | `none` | その一覧の公開コメントを誰に見せるか。一覧を見られない相手には見せない（FR-718） |
| `appeal_window` | 整数 20〜365（日） | `180` | 処分から何日のあいだ、本人が異議を申し立てられるか（FR-707、ADR-0038、[異議申し立て](moderation.md#異議申し立て)）。変えると、すでに出た処分の期限も変わる |
| `retention_session_origins` | 整数 90〜3650（日） | `180` | ログイン元の IP アドレスと User-Agent を、ログインから何日残すか（ADR-0038、[記録の保持期間と保全](moderation.md#記録の保持期間と保全)） |
| `retention_registration_origins` | 整数 90〜3650（日） | `180` | 登録元の IP アドレスを、登録から何日残すか |
| `retention_reports` | 整数 180〜3650（日） | `365` | 通報を、最後の解決から何日残すか。処分が参照する通報は処分と同じだけ残る |
| `retention_moderation_actions` | 整数 180〜3650（日） | `365` | アカウントへの処分と異議申し立てを、処分の終わり（解除・期限切れ・異議の審査の遅い方）から何日残すか |
| `retention_audit_log` | 整数 365〜3650（日） | `730` | 監査ログの行を、記録から何日残すか |

- 値は保存前に正規形にします（`TRUE` → `true`、`0750` → `750`、`ja_jp` → `ja-JP`、メディア ID は小文字）。不正な値は何も書き込まずに拒否します。
- `preview_image` と `thumbnail` は、存在する画像のメディア（処理中を含む）だけを受け付けます。`preview_image` は、処理が終わるまでと、処理に失敗した画像の代わりには既定画像が出ます。
  どの版でも一度設定したメディアは、未添付アップロードの消去（FR-803）で消えません。
- `contact_account` は、削除されていないローカルの利用者のユーザー名だけを受け付けます（小文字にして保存）。`contact_email` は `alice@example.com` の形のアドレスだけを受け付けます。

### 文章（ロケール別）

| キー | 上限 | 説明 |
| --- | --- | --- |
| `name` | 100 文字・1 行（空白だけは不可） | インスタンスの名前（表示名）。[名前とドメイン](#名前とドメイン) |
| `short_description` | 500 文字 | 短い説明 |
| `description` | 20000 文字 | 説明 |
| `terms` | 100000 文字 | 利用規約（Markdown） |
| `privacy_policy` | 100000 文字 | プライバシーポリシー（Markdown） |
| `preview_image_alt` | 1500 文字 | `preview_image` の画像の代替テキスト（`og:image:alt`）。無ければ「〇〇のイメージ画像」 |
| `thumbnail_alt` | 1500 文字 | `thumbnail` の画像の代替テキスト |

- 文章はロケールごとに持ちます。読むときは、読み手のロケールのフォールバックチェーン → `default_locale` のチェーン → `en` の順に探します
  （例: `ja-JP` → `ja` → 既定のロケール → `en`）。
  読み手のロケールは、画面の文言のロケールを選んだ要求のロケール（利用者の設定、または `Accept-Language` のタグ）を地域や文字体系ごと使います。
  `ja-JP` の読み手は文言は `ja` のカタログで読みますが、文章は `ja-JP` から探します。
- 空の文章を設定すると、そのロケールの文章を消したことになり、チェーンの次のロケールが使われます。
- 文章は CLI のほか、Web クライアントの管理画面（名前・短い説明・説明は「サーバーの設定」、`/admin/settings`。利用規約とプライバシーポリシーは「規約とポリシー」、`/admin/documents`）とクライアント API（`GET /api/v1/admin/instance_texts`・`PUT /api/v1/admin/instance_texts/{key}/{locale}`）で変えられます。どちらもロールの権限 `manage_settings` が要り、変更は監査ログ（`text_changed`）に残ります。

### 利用規約とプライバシーポリシー

- `terms` と `privacy_policy` は Markdown で書きます（CommonMark と、GitHub Flavored Markdown の表・取り消し線・URL の自動リンク）。段落の中の改行はそのまま改行になり、`#` の見出しはページの題の下の段（`h2`）から始まります。
- サーバーが公開ページ `/auth/terms`・`/auth/privacy` で HTML にしてサニタイズします。生の HTML・画像・`http` / `https` / `mailto` 以外のリンクは出ません。ページはサインインも JavaScript も要りません。
- 書いた文書は、登録の画面（作成のボタンの前の同意の文とリンク）、サーバーが描画するすべての画面のフッター、Web クライアント（3 列目・メニュー・設定・サインインの画面）からリンクされます。読み手のチェーンのどのロケールにも無い文書はリンクされず、ページは 404 です。

### 名前とドメイン

インスタンスはドメインと名前を別々に持ちます。

- **ドメイン**は設定 `host`（とハンドルのドメイン `account_domain`）で、運営中は変えられません。ハンドル（`@alice@example.com`）・URL・ActivityPub の ID・`GET /api/v1/instance` の `domain` は、名前があってもドメインのままです。
- **名前**は文章 `name` で、人に向けてインスタンスを名指す場所に出ます: Web クライアントのサイドバーと「このサーバーについて」、ブラウザのタブのタイトル、
  サーバーが描画するログイン・登録・メール確認・アカウントの状態・認可同意の画面、メール（メールアドレスの確認・モデレーターへの通知）、
  リンクプレビューの `og:site_name`、Web App Manifest の `name` / `short_name`、NodeInfo の `metadata.nodeName`、`GET /api/v1/instance` の `name`。
- 名前は読み手のロケールで選びます（利用者・メールの宛先のロケール、リンクプレビューは投稿の言語）。HTML の外枠・Web App Manifest・NodeInfo は `default_locale` の名前です。
  どのロケールのチェーンにも名前が無ければ、名前の代わりにドメイン（`host`）を出します。
- 変更は再起動なしで、次の要求から反映されます。開いている Web クライアントのタブのタイトルとサイドバーは、管理画面で保存したときに更新されます。

### 連絡先とサムネイル

運営への連絡先（`contact_account`・`contact_email`）とインスタンスを表す画像（`thumbnail` と文章 `thumbnail_alt`）は、次の場所に出ます。
どれも未設定なら出ません。

- Web クライアントの「このサーバーについて」（`/about`）。サムネイルはサインインしていない人の最初の画面（`/`）にも出ます。
- `GET /api/v1/instance` の `contact`（`account` と `email`）と `thumbnail`（`url`・`width`・`height`・`alt`）。代替テキストは要求のロケールで選びます。
- NodeInfo の `metadata`（`default_locale` で選ぶ）。Misskey の慣行の `maintainer`（`name` は連絡先のアカウントの表示名、無ければユーザー名。`email` は `contact_email`）、
  Pleroma・Akkoma の慣行の `staffAccounts`（連絡先のアカウントの Actor の ID）、`thumbnail`（`url` と `alt`）、短い説明 `short_description` の `nodeDescription`。

連絡先のアカウントが凍結・削除されたときは、アカウントは出さずにメールアドレスだけを出します。
変更は Web クライアントの管理画面（「サーバーの設定」、`/admin/settings`）か、CLI・クライアント API で行います。

## 使い方

```sh
my instance list                                      # すべての設定値（既定値かどうか）と文章
my instance get registration_mode
my instance set registration_mode open
my instance set name "Example Social"                 # default_locale のロケールに設定
my instance set -locale ja name "例のソーシャル"
my instance set -locale ja description - < description.ja.txt   # 値 "-" は標準入力から読む
my instance get -locale ja-JP name                    # フォールバックチェーンで探す
my instance set -locale ja name ""                    # ja の名前を消す
my instance history registration_mode                 # 変更履歴（新しい順、-limit で件数）
my instance history -locale ja description
my instance set -owner admin preview_image ./preview.png   # 画像を admin のメディアとしてアップロードし、プレビュー画像にする
my instance set preview_image 0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e6f   # アップロード済みのメディア ID を指定
my instance set -locale ja preview_image_alt "青い鳥のマスコット"
my instance set preview_image ""                      # 同梱の既定画像に戻す
my instance set contact_account admin                 # 連絡先のアカウント
my instance set contact_email admin@example.com
my instance set -owner admin thumbnail ./thumbnail.png  # サムネイル
my instance set -locale ja thumbnail_alt "海辺の町の写真"
```

- フラグはキーと値より前に書きます。
- `-owner <ユーザー名>` を付けると、`preview_image`・`thumbnail` の値を画像ファイルのパスとして読み、そのローカルアカウントのメディアとしてアップロードします。
  画像はワーカーが処理してから使われます（[メディア](../architecture/data-model.md#44-content)）。推奨は 1200×630 の横長の画像です。
- 変更は 1 回のトランザクションで版（`instance_*_revisions`）を記録し、現在値（`instance_settings` / `instance_texts`）を更新します。
  CLI からの変更は変更者なし（履歴の `BY` が `cli`）として残ります。
- 同じ値を設定し直しても版は増えます。
- 終了コード: 0 は成功、1 は値の拒否や DB のエラー（何も変更しない）、2 は引数の誤り。

## 管理画面とクライアント API

CLI で変えられる設定値は、Web クライアントの管理画面とクライアント API でも変えられます。どちらもロールの権限 `manage_settings` が要り（組み込みの `admin` だけが持つ）、
変更は CLI と同じく版として残り、キーごとに監査ログ（`setting_changed`、対象は設定、操作者はその利用者）に記録されます。監査ログはモデレーションの画面（「監査ログ」、`/moderation/audit-log`）で引けます。

- **Web クライアント**（「サーバーの設定」、`/admin/settings`、[ADR-0058](../adr/0058-moderation-admin-separation.md) のサーバー管理）: 名前・説明（言語ごと）と、次の設定を変えられます。
  設定はまとめて 1 つのフォームで保存し、変えたものだけを送ります。

  | 区画 | 設定 |
  | --- | --- |
  | 登録 | `registration_mode` |
  | 言語 | `default_locale`（アプリのある言語から選ぶ） |
  | タイムライン | `local_timeline_enabled`・`federated_timeline_enabled` |
  | 連合 | `authorized_fetch`・`remote_media_policy`・`remote_media_cache_ttl`（1 時間・1 日・7 日・30 日・365 日から選ぶ） |
  | 制限・凍結したサーバー | `domain_actions_visibility`・`domain_actions_rationale_visibility` |

  `max_status_length`・`preview_image`・`appeal_window`・保持期間（`retention_*`）は、画面には出ず、CLI とクライアント API で変えます。
- **クライアント API**: `GET /api/v1/admin/settings` がすべての設定値を、現在値・既定値・設定済みか・型と受け付ける値（選択肢・範囲）とともに返します。
  `PATCH /api/v1/admin/settings` は `{"values": {"<キー>": "<値>", ...}}` の設定値をまとめて変えます。値はすべて文字列で、CLI と同じく正規形にして保存します。
  1 つでも知らないキーか不正な値があれば、そのキーを挙げた 422 で、何も変えません。変えたキーは同じトランザクションで版と監査ログに残ります。
- 登録モードを `invite` にしたときの招待は、モデレーションの画面（「招待」、`/moderation/invites`、権限 `manage_invites`）で作ります（[利用者登録](registration.md#招待コード)）。

## ルール

インスタンスのルール（FR-705）は、`GET /api/v1/instance` の `rules` が並べ、通報が ID で引用します。

- ルールはロケールごとに 1 行の文（300 文字まで）と任意の補足（1,000 文字まで）を持ちます。1 つのルールのロケールは 50 個まで、有効なルールは 100 個までです。
- 変更（文の改訂・並べ替え）はすべて版（`instance_rule_revisions`）として残り、ルールの ID は変わりません。
- 廃止したルールは有効なルールの一覧から消えます。元には戻せませんが、通報からは引き続き参照できます。
- 公開の一覧では、各ルールの文を読み手のロケールのフォールバックチェーン → `default_locale` のチェーン → `en` の順に選びます。
  どのロケールにも文が無いルールは、ロケール順で最初の文を出します（ルールが読み手によって消えることはありません）。

```sh
my instance rule add "Be kind"                        # default_locale のロケールで、有効なルールの最後に追加
my instance rule add -locale ja -hint "嫌がらせをしない" "親切に"
my instance rule edit -locale ja <rule> "いつも親切に"     # ja の文を変える（無ければ足す）。補足は -hint を付けたときだけ変わる
my instance rule edit -locale ja <rule> ""            # ja の文を消す（最後のロケールは消せない）
my instance rule move <rule> 1                        # 1 番目に移し、有効なルールを 1, 2, ... と振り直す
my instance rule retire <rule>                        # 廃止する
my instance rule list                                 # 有効なルール（-all で廃止したものも）
```

- 管理 API は `/api/v1/admin/rules`（一覧・追加・取得・改訂・移動・廃止）で、ロールの権限 `manage_rules` を要します。
- Web クライアントの管理画面（「ルール」、`/admin/rules`、`manage_rules`）で、言語ごとの文と補足でのルールの追加、編集、「上へ」「下へ」での並べ替え、廃止（確認のダイアログを経る）ができます。
  どの変更も監査ログ（`rule_added`・`rule_revised`・`rule_moved`・`rule_retired`）に残ります。
- 終了コードは設定と同じです（0 は成功、1 は値の拒否や DB のエラー、2 は引数の誤り）。

## テーマ

サーバー単位のテーマ（FR-605、[ADR-0059](../adr/0059-design-tokens-and-instance-theme.md)）は、Web クライアントのデザイントークンの上書きと、任意の CSS（カスタム CSS）1 つです。
すべての閲覧者の Web クライアントとサーバーが描画する画面（ログイン・登録・規約など）に、ライトとダークの両方で当たります。

- 上書きできるトークンは `app/ui/src/theme.css` の原色（`--palette-*`）以外で、一覧・既定値・範囲は [app/ui/README.md](../../app/ui/README.md#server-themes) にあります。
  色はライト / ダークそれぞれ `#rrggbb`（`color-overlay` は `#rrggbbaa` も可）で、ほかは種類ごとの形（長さ・数・時間・緩急・フォント名の並び・影）と範囲に収まる値です。
- 保存のときにサーバーが、値の形と範囲、色のトークンのすべての組のコントラスト（上書きしないトークンは既定値で、ライト / ダークの両方で WCAG 2.2 AA）、カスタム CSS の長さ（64 KiB）を検査し、
  1 つでも満たさなければ何も保存しません。
- カスタム CSS は、サーバーは中身を読まずに、トークンの規則の後に `@layer` の外で同じスタイルシートに入れて返します。Web クライアントと同じ CSP の下で動くので、別のオリジンの画像・フォント・`@import` は読めません。
  **カスタム CSS の結果（コントラスト・横スクロール・タップ領域・フォーカスの表示・右から左の言語）はサーバーが確かめず、管理者の責任です。** クラス名とページの構造は版で変わるので、安定して使えるのはトークンだけです。
- テーマは版（`instance_theme_revisions`）として残り、現在の版のスタイルシートを `/theme/{版の ID}.css`（`immutable` で長期キャッシュ）で配ります。
  HTML の外枠は共有キャッシュ（60 秒と再検証の 300 秒）を持つので、保存してから全員に届くまで数分かかります。開いているページは読み込み直すまで前の見た目のままです。
- 外枠の `<meta name="theme-color">`（ライト / ダーク）と Web App Manifest の `theme_color` / `background_color`（ライト）は、テーマの `color-surface` の値になります。
- トークンもカスタム CSS も無い版を保存すると、テーマは既定に戻り、スタイルシートを読み込まなくなります。
- Web クライアントの管理画面（「テーマ」、`/admin/theme`）とクライアント API（`GET` / `PUT /api/v1/admin/theme`）と CLI で変えられます。
  管理画面とクライアント API はロールの権限 `manage_settings` が要り、変更は監査ログ（`setting_changed`、対象 `theme`）に残ります。

```sh
my instance theme get > theme.json                    # 今のテーマを管理 API と同じ JSON で出す
my instance theme set < theme.json                    # テーマ全体を置き換える（検査に通らなければ理由を出して何もしない）
echo '{"tokens":[],"custom_css":""}' | my instance theme set   # 既定に戻す
```

```json
{
  "tokens": [
    { "token": "color-accent", "light": "#1447e6", "dark": "#92c5fe" },
    { "token": "radius-md", "value": "0.25rem" }
  ],
  "custom_css": ".brand { letter-spacing: 0.02em; }"
}
```

## カスタム絵文字

ローカルのカスタム絵文字（FR-111、[ADR-0051](../adr/0051-custom-emoji.md)）は、投稿の本文・CW とプロフィールの表示名・自己紹介・補足情報で `:shortcode:` と書くと画像で表示され、
送る Note と Actor の `Emoji` タグで Mastodon・Misskey・GoToSocial・Akkoma にも届きます。

- shortcode は 2〜30 文字の英字・数字・`_` で、大文字と小文字を区別し、登録した後は変えられません。別名（10 個まで）とカテゴリ（64 文字まで）は絵文字ピッカーの検索と分類に使います。
- 画像は PNG・GIF・WebP で、設定 `custom_emoji_max_bytes`（既定 256 KiB）以下、縦横 1,024 px 以下です。64 / 128 / 256 px の箱に収めた派生を作り、元の画像は残しません。
  APNG は最初のフレームだけの静止画になります。GoToSocial は既定で 100 KiB を超える絵文字を取得しないので、小さい画像を勧めます。
- 無効にした絵文字は、投稿・プロフィール・連合・ピッカーのどこにも出ず、`:shortcode:` の文字に戻ります。ピッカーに出さない設定にした絵文字は、ピッカーに出ないだけで、書けば使えます。
  ほかのサーバーから届いた絵文字は、無効にすることだけができます。
- 削除は取り消せません。過去の投稿の絵文字も文字に戻り、同じ shortcode で登録し直しても別の絵文字です。連合先が複製した画像は消せません。
- 管理画面（「カスタム絵文字」、`/admin/custom-emojis`）とクライアント API（`/api/v1/admin/custom_emojis`）と CLI で管理できます。
  管理画面とクライアント API はロールの権限 `manage_custom_emojis`（組み込みの `admin` だけが持つ）が要り、登録・画像の差し替え・設定と別名の変更・有効化と無効化・削除が監査ログ（対象 `custom_emoji`）に残ります。

```sh
my admin emoji create -shortcode blobcat -category Cats -alias neko blobcat.png   # 登録する
my admin emoji list -local                                                        # ローカルの絵文字を新しい順に出す
my admin emoji disable <emoji id>                                                 # 無効にする（enable で戻す）
my admin emoji delete <emoji id>                                                  # 削除する
```
