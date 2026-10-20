# リンクプレビュー（OGP / X Card / fediverse:creator）

運営者・開発者向けの説明と手動確認の手順です。設計は [ADR-0028](../adr/0028-html-shell-link-previews-and-pwa.md)、
閲覧判定は [公開範囲の閲覧判定](../architecture/visibility.md)、URL は [ADR-0018](../adr/0018-federated-uris.md) と [ADR-0053](../adr/0053-status-page-url.md) です。

## サーバーが返すもの

ランディングページ（`https://<host>/`）とインスタンスの情報ページ（`https://<host>/about`）、プロフィール（`https://<host>/@alice`）と投稿（`https://<host>/@alice/status/<投稿 ID>`）の HTML は、`my serve` が Web クライアントの外枠に meta タグを埋め込んで返します。
HTML のリクエストは常に未ログインとして扱うので、プレビューに出るのは未ログインでも見られるものだけです。

| ページ | 応答 |
| --- | --- |
| ランディングページ（`/`）、インスタンスの情報ページ（`/about`） | 200。インスタンスを紹介する meta タグ付きの外枠（下の「ランディングページ」） |
| `public` の投稿、有効なローカルアカウントのプロフィール | 200。meta タグ付きの外枠 |
| `unlisted` の投稿 | 200。meta タグ付きの外枠に `<meta name="robots" content="noindex, noarchive">` と `X-Robots-Tag: noindex, noarchive` |
| `local` / `private` / `direct` / 削除済み / 存在しない投稿、停止・削除されたアカウントや制限のあるアカウントのプロフィールと投稿 | 404。汎用の外枠と `X-Robots-Tag: noindex, noarchive`。本文とヘッダーは理由によらず同じ |
| `/@<旧ユーザー名>/status/<投稿 ID>` | 301。現在のユーザー名の URL へ |

どのページにも、検索エンジンと AI クローラーへの表明（`noindex`・`noai` などの meta タグと `X-Robots-Tag`・`Content-Usage`・`tdm-reservation`）が、
インスタンス設定と持ち主の設定に応じて付きます（[検索エンジンと AI クローラー](crawlers.md)）。上表の `noindex` は設定によらず付くものです。
`noindex` はリンクプレビューを止めません。

ActivityPub の `Accept` で来たリクエストには外枠を返さず、ActivityPub 側が答えます（プロフィールはアクターの `id`、未ログインで見られる投稿はその `id` への 302。ADR-0018、ADR-0053）。どの応答にも `Vary: Accept` が付きます。

### meta タグ

| タグ | 中身 |
| --- | --- |
| `og:title`、`<title>` | 「表示名 (@ユーザー名@アカウントドメイン)」。表示名が空ならユーザー名 |
| `og:type` | プロフィールは `profile`、投稿は `article` |
| `og:url`、`<link rel="canonical">` | 正規 URL（現在のユーザー名の URL） |
| `og:description` | 投稿の本文（プロフィールは自己紹介）からタグを除いた平文を 200 書記素で切り詰めたもの。CW 付きの投稿は CW の文だけ |
| `og:image`、`og:image:alt` / `width` / `height` | 下記の順で選んだ画像 |
| `og:site_name` | インスタンスの名前（文章 `name`。どの言語でも同じ値。無ければ `host`、[名前とドメイン](instance-settings.md#名前とドメイン)） |
| `og:locale`、`<html lang dir>` | 投稿の言語。未設定（`und`）なら投稿者の既定の投稿言語、それもなければインスタンスの `default_locale`。`Accept-Language` では変わらない |
| `article:published_time`、`article:author` | 投稿だけ。投稿日時（UTC）と投稿者のプロフィール URL |
| `profile:username` | プロフィールだけ。`ユーザー名@アカウントドメイン` |
| `twitter:card` | 画像が約 2:1 より横長（幅 / 高さ ≥ 1.7）で 300×157 以上なら `summary_large_image`、それ以外は `summary` |
| `fediverse:creator` | `@ユーザー名@アカウントドメイン`。Mastodon が「作成者」として表示する |
| `<link rel="alternate" type="application/activity+json">` | AP の `id`（アクターまたは投稿のオブジェクト） |

ランディングページの meta タグは、インスタンスそのものを紹介します。

- `og:type` は `website`、`og:title`・`<title>`・`og:site_name` はインスタンスの名前、`og:url` と `<link rel="canonical">` は `https://<host>/` です。
- `og:description` はインスタンスの文章 `short_description` を、200 書記素で切り詰めたものです。無ければ出しません。
- 言語（`og:locale`、`<html lang dir>`）は、インスタンスの `default_locale` です。名前と説明はどの言語でも同じ値です。
- `og:image` は下の 3（インスタンスのプレビュー画像）、なければ 4 です。
- アカウントのものではないので、`fediverse:creator` と ActivityPub の `alternate` は出しません。
- インスタンスの情報ページ（`/about`）も同じ meta タグで、`og:title` と `<title>` は「{インスタンスの名前} について」（`server:preview.about_title`）、`og:url` と `<link rel="canonical">` は `https://<host>/about` です。

`og:image` は次の順で選びます。

1. 投稿の最初の添付画像（代替テキストを `og:image:alt` に入れる）。センシティブ指定または CW 付きの投稿では使わない
2. プロフィールのヘッダー画像、なければアバター（`og:image:alt` は「〇〇のヘッダー画像」「〇〇のアバター」）
3. インスタンスのプレビュー画像（インスタンス設定 `preview_image`。`og:image:alt` は文章 `preview_image_alt`、無ければ「〇〇のイメージ画像」）
4. 同梱の既定画像（`https://<host>/preview-image.png`、1200×630 の PNG。インスタンス名は描かない汎用の絵で、`og:image:alt` は絵の説明）

そのため、プレビューの出るページには必ず `og:image` と `og:image:alt` があります。
1〜3 の画像は処理済みのメディアの `full` 版で、幅と高さは処理結果の値です。`preview_image` が処理中・処理失敗・画像以外なら 4 を使います。
インスタンス設定はリクエストごとに読むので、`my instance set preview_image ...` は再起動なしに反映します（設定の手順は [インスタンス設定](instance-settings.md)）。

既定画像は起動後の最初のリクエストで標準ライブラリで描いて、プロセス内に持ちます（約 10 KB。リポジトリにバイナリを置かず、寸法を定数として `og:image:width` / `height` に書けるため）。
`Cache-Control: no-cache` と ETag で配信します。

### キャッシュ

- `Cache-Control: public, max-age=60, stale-while-revalidate=300` を返します。削除や公開範囲の変更は、最長でこの時間だけ共有キャッシュに残ります。
- ETag はテンプレート・アセット・インスタンスの設定の版、言語、投稿の版（リビジョン）、実効公開範囲、プロフィールの版、選んだ画像（ランディングページと情報ページは、ページのアドレス・タイトル・インスタンスの説明と選んだ画像）から作ります。`If-None-Match` には `304` を返します。
- CDN を前に置く場合は、`Accept` をキャッシュキーに含めてください（AP の JSON と HTML を取り違えないため）。

## 手動確認の手順

各サービスの実機での確認は、リリースごとに行います。HTTPS で外部から到達できるインスタンスが必要です（プレビューのボットは `localhost` に届きません）。

### 準備

1. 確認用のアカウントで、次の投稿を用意します。
   - `public` の投稿で、横長（例: 1200×630）の画像を添付し、代替テキストを付けたもの
   - `public` の投稿で、画像なしのもの（ヘッダー画像を設定したアカウント）
   - `public` の投稿で、本文が 200 文字を超え、絵文字（👨‍👩‍👧‍👦 など）を含むもの
   - `unlisted` の投稿
   - `private` の投稿
   - 表示名に `"><script>alert(1)</script>` を含むアカウントの投稿
2. 各 URL の HTML を確認します。

   ```sh
   curl -sS -D - -H 'Accept: text/html' https://<host>/@alice/status/<投稿 ID>
   ```

   - 200 で、上表の meta タグがあること。`unlisted` は `noindex` が付くこと。
   - `private` の投稿と存在しない ID が、同じ本文の 404 で `X-Robots-Tag: noindex, noarchive`（AI を拒否する設定では `noai, noimageai` も）を返すこと。

     ```sh
     diff <(curl -sS https://<host>/@alice/status/<private の投稿 ID>) <(curl -sS https://<host>/@alice/status/0192f3a4-7b1c-7d2e-9f00-1a2b3c4d5e6f)
     ```

   - 表示名の `<script>` が `&lt;script&gt;` とエスケープされていること。

### X

1. 投稿画面（または DM）に、画像付きの投稿の URL を貼ります。
2. 大きい画像のカード（`summary_large_image`）に、タイトル・ドメイン・画像が出ることを確かめます。
3. 画像なしの投稿の URL では、ヘッダー画像のカードが出ることを確かめます。ヘッダー画像もアバターもないアカウントでは、既定画像（`preview_image`、未設定なら同梱の画像）の大きいカードが出ることを確かめます。
4. 正方形のアバターだけのアカウントでは、小さい画像のカード（`summary`）になることを確かめます。

X はカードをキャッシュします。変更を確かめるときは、URL にクエリ（`?v=2` など）を付けて別の URL として貼ります。

### Mastodon

1. 別のインスタンスのアカウントで、画像付きの投稿の URL を含む投稿を作ります。
2. 投稿の下にプレビューカードが出て、タイトル・説明・画像が出ることを確かめます。
3. カードの下に「作成者」として確認用アカウント（`fediverse:creator`）が表示され、そのプロフィールに移動できることを確かめます。

Mastodon はカードを取得したサーバーごとにキャッシュします。作り直すには、新しい URL（クエリ付き）で投稿します。

### Discord

1. チャンネルに画像付きの投稿の URL を貼ります。
2. 埋め込みに、サイト名（インスタンス名）・タイトル・説明・大きい画像が出ることを確かめます。
3. 200 文字を超える投稿で、説明が `…` で終わり、絵文字が途中で切れていないことを確かめます。

### Slack

1. チャンネルに画像付きの投稿の URL を貼ります。
2. 展開されたプレビューに、サイト名・タイトル・説明・画像が出ることを確かめます。
3. `unlisted` の投稿の URL でもプレビューが出ることを確かめます（`noindex` は検索エンジン向けで、プレビューは止めません）。

Slack もプレビューをキャッシュします。変更を確かめるときは、URL にクエリを付けます。

### LINE

1. トークに画像付きの投稿の URL を送ります。
2. タイトル・説明・画像のプレビューが出ることを確かめます。

LINE もプレビューをキャッシュします。変更を確かめるときは、URL にクエリを付けます。

### 見えてはいけないもの

どのサービスでも、`private` の投稿の URL と存在しない投稿の URL が、どちらも同じ汎用のプレビュー（インスタンス名だけ、または何も出ない）になり、本文・表示名・画像が出ないことを確かめます。
