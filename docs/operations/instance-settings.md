# インスタンス設定

運営者向けの手順です。要件は FR-001・FR-101・FR-705（[機能要件](../requirements/01-functional.md)）、
テーブルは [データモデル §4.9](../architecture/data-model.md#49-instance) です。管理画面からの変更は M5 で追加します。

## 設定（環境変数）とインスタンス設定（DB）の分担

| | 設定 | インスタンス設定 |
| --- | --- | --- |
| 内容 | デプロイの構成: 役割・アダプタ・接続先・ホスト名・秘密情報 | 運営の方針: 登録モード・文字数上限・既定のロケール・タイムラインの有効 / 無効、インスタンスの名前・説明・規約 |
| 置き場所 | 環境変数 `MY_*` と設定ファイル（[設定リファレンス](../reference/configuration.md)、ADR-0010） | DB。変更はすべて版として残る |
| 変更 | プロセスの再起動で反映 | `my instance set` で即時に反映 |
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
| `remote_media_policy` | `none` / `avatars` / `all` | `none` | リモートのメディアの複製を持つか（FR-409、ADR-0005）。`none` はメディアプロキシが毎回中継し、`avatars` はアバター・ヘッダー・カスタム絵文字、`all` はすべてを `remote_media_cache_ttl` の間 Blob にキャッシュする。キャッシュの合計の上限は設定 `remote_media_cache_max_bytes`（[federation.md §9](../architecture/federation.md#9-リモートメディア)） |
| `remote_media_cache_ttl` | 整数 60〜31536000（秒） | `86400` | 取得したリモートのメディアを持ってよい期間。メディアプロキシのレスポンスの `Cache-Control: max-age` で、CDN とブラウザがこの間キャッシュする（[federation.md §9](../architecture/federation.md#9-リモートメディア)） |
| `authorized_fetch` | `true` / `false` | `false` | Authorized Fetch（secure mode、FR-408）。`true` にすると、検証できる HTTP 署名の無い ActivityPub の GET に、オブジェクトとコレクションは `401` で、利用者の Actor は鍵の取得のための最小の文書で答える（[federation.md §10](../architecture/federation.md#10-コレクションとオブジェクトの-get)）。インスタンス Actor は署名なしでも文書全体を返すので、署名の検証のための鍵の取得はどのサーバーからもできる |
| `domain_actions_visibility` | `none` / `users` / `all` | `none` | 処分したドメインの一覧（`GET /api/v1/instance/domain_actions`）を誰に見せるか: 誰にも / ログインした利用者に / 誰にでも（FR-718、[ドメイン単位の処分と通報の対応](moderation.md#処分したドメインの公開)） |
| `domain_actions_rationale_visibility` | `none` / `users` / `all` | `none` | その一覧の公開コメントを誰に見せるか。一覧を見られない相手には見せない（FR-718） |
| `appeal_window` | 整数 20〜365（日） | `180` | 処分から何日のあいだ、本人が異議を申し立てられるか（FR-707、ADR-0038、[異議申し立て](moderation.md#異議申し立て)）。変えると、すでに出た処分の期限も変わる |

- 値は保存前に正規形にします（`TRUE` → `true`、`0750` → `750`、`ja_jp` → `ja-JP`、メディア ID は小文字）。不正な値は何も書き込まずに拒否します。
- `preview_image` は、存在する画像のメディア（処理中を含む）だけを受け付けます。処理が終わるまでと、処理に失敗した画像の代わりには既定画像が出ます。
  どの版でも一度設定したメディアは、未添付アップロードの消去（FR-803）で消えません。

### 文章（ロケール別）

| キー | 上限 | 説明 |
| --- | --- | --- |
| `name` | 100 文字・1 行 | インスタンスの名前 |
| `short_description` | 500 文字 | 短い説明 |
| `description` | 20000 文字 | 説明 |
| `terms` | 100000 文字 | 利用規約 |
| `privacy_policy` | 100000 文字 | プライバシーポリシー |
| `preview_image_alt` | 1500 文字 | `preview_image` の画像の代替テキスト（`og:image:alt`）。無ければ「〇〇のイメージ画像」 |

- 文章はロケールごとに持ちます。読むときは、読み手のロケールのフォールバックチェーン → `default_locale` のチェーン → `en` の順に探します
  （例: `ja-JP` → `ja` → 既定のロケール → `en`）。
- 空の文章を設定すると、そのロケールの文章を消したことになり、チェーンの次のロケールが使われます。

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
```

- フラグはキーと値より前に書きます。
- `-owner <ユーザー名>` を付けると、`preview_image` の値を画像ファイルのパスとして読み、そのローカルアカウントのメディアとしてアップロードします。
  画像はワーカーが処理してから使われます（[メディア](../architecture/data-model.md#44-content)）。推奨は 1200×630 の横長の画像です。
- 変更は 1 回のトランザクションで版（`instance_*_revisions`）を記録し、現在値（`instance_settings` / `instance_texts`）を更新します。
  CLI からの変更は変更者なし（履歴の `BY` が `cli`）として残ります。
- 同じ値を設定し直しても版は増えます。
- 終了コード: 0 は成功、1 は値の拒否や DB のエラー（何も変更しない）、2 は引数の誤り。

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
- 終了コードは設定と同じです（0 は成功、1 は値の拒否や DB のエラー、2 は引数の誤り）。
