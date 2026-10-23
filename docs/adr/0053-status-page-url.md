# ADR-0053: 投稿の表示用 URL を `/@{username}/status/{id}` にし、プロフィールの下を予約語の名前空間にする

- Status: Accepted
- Date: 2026-10-20
- Deciders: @rosylilly
- Issue: #643

## Context（背景・課題）

[ADR-0018](0018-federated-uris.md) は、AP の `id` を `/ap/` 名前空間の ID ベースにし、人間向けの表示用 URL（`url`）を別に持つと決めた。
投稿の表示用 URL は `/@{username}/{uuid}` である。[ADR-0028](0028-html-shell-link-previews-and-pwa.md) は `GET /@{acct}/{id}` で OGP 付きの外枠を返すと決めた。

main の実装:

- `internal/domain/content/uri.go` の `StatusURL` が `https://{host}/@{username}/{id}` を作る。`internal/app/content/service.go` が投稿の作成時にこれを `statuses.url` に保存し、
  AP の Note（`internal/app/federation/document.go`）とクライアント API が保存した値を返す。
- `internal/interface/web/preview.go` の `parsePreviewRoute` は `/@{name}/` の次の区間が UUIDv7 なら投稿、そうでなければ Web クライアントの別のページとみなす。
  名前が現在のユーザー名と違えば `StatusURL` へ 301 でリダイレクトする。
- AP の `Accept` で `/@{username}` に来たら Actor の `id` へ 302 でリダイレクトする（`internal/interface/activitypub/actor.go`）。
  投稿のページに AP の `Accept` で来たときは `internal/platform/app.go` が 404 を返し、ADR-0018 の「`id` へ 302」はまだ実装されていない。
- Web クライアントの経路は `app/web/src/routes/@{$acct}_.$id.tsx`（投稿）、`@{$acct}_.followers.tsx`、`@{$acct}_.following.tsx`。
  投稿 ID と `followers` / `following` が `/@{acct}/` の下の同じ階層に並んでいる。

決定者の指示（原文）: 「単一投稿の URL を `/@user/:status-id` から `/@user/status/:id` に変更。 `/@user/favorite` や  `/@user/media` などが今後できうるため」

プロフィールの下にお気に入り・メディア・返信付きなどのページ（FR-204 プロフィールタイムラインの派生）を足すと、
投稿 ID とページ名が同じ階層で区別されることになる。いまは UUIDv7 の正規表現で区別できるが、ID の形式とページ名の両方に暗黙の制約がかかり、
Web クライアント（TanStack Router）と Go（外枠の振り分け）の 2 か所で同じ規則を持ち続ける必要がある。
M4 で投稿の詳細（FR-106 スレッド表示）とプロフィールの下のページを足す前に、URL の形と名前空間を決める。

関連: FR-106（返信・スレッド表示）、FR-204（プロフィールタイムライン）、FR-305（URL 指定での解決）、[ADR-0012](0012-v0-breaking-change-policy.md)、ADR-0018、ADR-0028

### 主要実装の投稿 URL

2026-10-20 に各実装のソースで確認した（出典は末尾）。

| 実装 | 投稿の表示用 URL | プロフィールの下のページ | 衝突の避け方 |
| --- | --- | --- | --- |
| Mastodon | `/@{username}/{id}` | `/@{username}/featured`・`collections`・`with_replies`・`media`・`tagged/{tag}`・`following`・`followers` | 経路の定義順で固定のページ名を先に照合する。ID は数値（snowflake）なので語と重ならない |
| GoToSocial | `/@{username}/statuses/{id}` | `/@{username}/statuses` を投稿の親にする | 投稿を `statuses` の下に置く |
| Misskey | `/notes/{id}` | `/@{username}/…` はプロフィールだけ | 投稿をプロフィールの外に置く |
| Pleroma / Akkoma | `/notice/{id}` | 同上 | 同上 |
| （参考）X | `/{username}/status/{id}` | `/{username}/media`・`likes`・`with_replies` など | 投稿を単数形 `status` の下に置く |

分かったこと:

- 投稿をプロフィールの下に置く実装のうち、ID を直下に置く Mastodon は、ID が数値であることと経路の順序に頼ってページ名と区別している。
  GoToSocial は投稿を固定の語の下に置き、衝突そのものを無くしている。
- AP の相互運用には影響しない。`url` は表示用の値で、連合先は投稿を `id` で識別する（ADR-0018 の比較を参照）。
- Mastodon は URL を貼った検索（`ResolveURLService`）で、ローカルに無い URL を取得しに行く。取得が 404 のときは、保存済みの投稿を `uri` か、推測した `uri` と `url` の組で探す。
  連合先に保存された古い `url` は、投稿の識別には使われないが、「元のページを開く」リンクとして残る。

## Decision Drivers（判断基準）

1. **名前空間の衝突が無いこと**: プロフィールの下にページを足しても、投稿 ID の形式に頼らずに経路が決まる
2. **`id` の不変性**: AP の `id`（`/ap/objects/{uuid}`）は変えない（ADR-0012、ADR-0018）
3. **規則を 1 か所で持つこと**: Go の外枠と Web クライアントが、同じ単純な規則で経路を振り分けられる
4. **読みやすさ**: 共有された URL から、それが投稿だと分かる
5. **移行の費用**: v0 は互換を持たない（ADR-0012）。既に配った `url` が切れることの影響が小さい

## Considered Options（選択肢）

1. **`/@{username}/status/{id}`**（決定者の指示）
2. **`/@{username}/statuses/{id}`**（GoToSocial と同じ）
3. **`/@{username}/{id}` のまま、予約語の一覧で区別する**（Mastodon と同じ）
4. **プロフィールの外に置く `/status/{id}`**（Misskey・Akkoma に近い）

## Decision（決定）

**選択肢 1 を採用する。** 投稿の表示用 URL を `/@{username}/status/{id}` にし、`/@{username}/` の下の第 1 区間をサーバーが決める固定の語（予約語）だけにする。

### 投稿の表示用 URL

| 対象 | URL | 例 |
| --- | --- | --- |
| ローカルの投稿（HTML、Note の `url`） | `https://{host}/@{username}/status/{uuid}` | `https://social.example.org/@alice/status/0192f3b0-2c3d-7e4f-8a00-112233445566` |
| リモートの投稿（Web クライアントの経路） | `/@{username@domain}/status/{id}` | `/@bob@remote.example/status/0192f3b0-…` |

- 投稿は ID だけで引く。`{username}` は読みやすさのためだけにあり、現在のユーザー名と違えば、正しい URL へ 301 でリダイレクトする（改名後も古いリンクが切れない）。
- AP の `Accept`（ADR-0018 の判定）で `/@{username}/status/{uuid}` に来たら、投稿の `id`（`/ap/objects/{uuid}`）へ 302 でリダイレクトする（FR-305。Mastodon の検索に URL を貼ると投稿が引ける）。
  リダイレクトの前に `canView` で匿名の閲覧者が見られるかを確かめ、見られなければ外枠の 404 と同じに扱う（ADR-0017）。
- ローカルの投稿の `url` は保存せず、読み出すときに作者の現在のユーザー名と投稿 ID から `StatusURL` で作る。
  改名と URL の形の変更が、保存済みの値に残らないようにするため。`statuses.url` はリモートの投稿の `url` だけを持つ。
- AP の `id` は `https://{host}/ap/objects/{uuid}` のままで変わらない。

### `status` と `statuses`

- `status` を使う。「この投稿」を指す 1 件のページで、X と同じ語順なので、共有された URL から投稿だと読み取りやすい。
- `statuses` は REST の「コレクションの下の要素」の形で、GoToSocial と Mastodon のクライアント API（`/api/v1/statuses/{id}`）と同じ。
  `/@{username}/statuses` を投稿一覧のページにする余地が残るが、投稿一覧はプロフィール（`/@{username}`）そのものなので、使い道が無い。
- どちらも AP の相互運用には影響しない。決定者の指示に合わせて `status` にする。

### `/@{username}/` の下の名前空間

- `/@{acct}/` の直下の第 1 区間は、次の規則の固定の語だけにする。可変の値（ID、タグ名など）を第 1 区間に置かない。
  可変の値は語の下（`/@{acct}/status/{id}`、`/@{acct}/tagged/{tag}` など）に置く。
- 語は小文字の ASCII 英字・数字・ハイフン・アンダースコア（`[a-z][a-z0-9_-]*`）。単数か複数かは、そのページが 1 件を指すか一覧かで決める。
- 予約語の一覧（現在のページと、将来のページ）:

  | 語 | ページ | 状態 |
  | --- | --- | --- |
  | `status` | 投稿の詳細（`status/{id}`）。`status/{id}/…` も投稿の下のページ用に予約する | この ADR で移す |
  | `followers` | フォロワー一覧 | 実装済み |
  | `following` | フォロー一覧 | 実装済み |
  | `media` | メディア付きの投稿 | 予約 |
  | `favorite` | お気に入り | 予約 |
  | `with_replies` | 返信を含む投稿 | 予約 |
  | `tagged` | ハッシュタグで絞った投稿（`tagged/{tag}`） | 予約 |
  | `featured` | ピン留めした投稿 | 予約 |

- 一覧の置き場所: Web クライアントの経路のファイル（`app/web/src/routes/@{$acct}_.*.tsx`）が実体で、
  語を足すときは [docs/architecture/web-client.md](../architecture/web-client/README.md) の経路の説明に一覧を書く（#653 で節を足す）。
  Go が知るのは、サーバーが役割を持つ語（外枠の meta タグと AP のリダイレクトを返す `status`）だけで、それ以外の語は汎用の外枠を返す。
  語を足すのに ADR は要らないが、第 1 区間に可変の値を置く変更はこの ADR を置き換える ADR を要する。

### 既存の `/@{username}/{uuid}`

- 互換のためのリダイレクトは持たない（ADR-0012）。`/@{username}/{uuid}` は予約語でない第 1 区間なので、Web クライアントの「ページが無い」になる。
- 連合先が保存した古い `url` と、外部に貼られた古いリンクは切れる。投稿の識別（`id`）は変わらないので、連合先での返信・ブースト・お気に入りは影響を受けない。
  公開済みのインスタンスがあれば、利用者に URL の変更を告知する（v0 の試験運用の前提、ADR-0012）。

### ADR-0018・ADR-0028 のうち、この ADR が置き換える部分

次の部分を置き換える（両 ADR のファイルの扱いは決定者が決める）。

- ADR-0018「URI 一覧」の行「投稿（HTML、`url`）」: `https://{host}/@{username}/{uuid}` を `https://{host}/@{username}/status/{uuid}` にする。
- ADR-0018「HTML と AP JSON の振り分け」の 2 つの規則:
  - 「表示用 URL（`/@alice`、`/@alice/{uuid}`）に AP の `Accept` で来たら、対応する `id` へ `302`」の投稿の経路を `/@alice/status/{uuid}` にする。
  - 「`/@{任意の名前}/{uuid}` は投稿 ID だけで引く。名前が現在のユーザー名と違えば正しい URL へリダイレクトする」の経路を `/@{任意の名前}/status/{uuid}` にし、リダイレクトを 301 と明記する。
- ADR-0028「HTML の外枠」の `GET /@{acct}/{id}`（と Context の `/@alice/{id}`）: `GET /@{acct}/status/{id}` にする。

ADR-0018 の `id` の形式、`/ap/` 名前空間、プロフィールの URL（`/@{username}`）、アクセス制御、ADR-0028 のそれ以外の部分は変えない。

### この決定を見直す条件

- 主要実装のいずれかが、`url` のパスの形を前提にした処理（例: `/@{username}/{id}` を Mastodon の形とみなす推測）で、この形の URL を解決できないと分かった場合
- 公開済みのインスタンスが増え、v0 の間でも古い `url` のリダイレクトを持つべきだと決定者が判断した場合

## Consequences（影響）

- 良い影響
  - プロフィールの下にページを足しても、投稿 ID と衝突しない。Go の外枠の振り分けが UUIDv7 の正規表現に頼らず、固定の語の照合で済む。
  - 共有された URL から投稿だと読み取れる。
  - ローカルの投稿の `url` を保存しないので、改名や URL の形の変更で保存済みの値を書き換える必要が無い。
- 悪い影響・トレードオフ
  - URL が `/status` の 7 文字だけ長くなる。
  - 既に配った `url` と外部に貼られたリンクが切れる。連合先の「元のページを開く」が 404 になる。
  - Mastodon とは URL の形が違う。Mastodon の `ResolveURLService` は取得が 404 のときだけ URL の形から `uri` を推測するので、取得が成功する新しい URL ではこの推測に頼らない。
- 後続の作業（Issue 化する）
  - #653: `StatusURL`・外枠の経路と 301・AP の `Accept` での 302・Web クライアントの経路とリンクの生成・`statuses.url` をリモートだけにする変更、
    docs（[docs/architecture/web-client.md](../architecture/web-client/README.md)、[docs/operations/link-previews.md](../operations/link-previews.md)、[docs/operations/running.md](../operations/running.md)）の更新
  - 予約した語のページ（FR-204 の派生）を足すときに、web-client.md の一覧の状態を更新する

## Pros and Cons of the Options（詳細比較）

### 1. `/@{username}/status/{id}`（採用）

- 良い: 判断基準 1・3・4 を満たす。X と同じ語順で読みやすい。
- 悪い: 既存の URL が切れる。Mastodon と形が違う。

### 2. `/@{username}/statuses/{id}`

- 良い: 判断基準 1・3 を満たす。GoToSocial・Mastodon のクライアント API と同じ語で、REST の形として一貫する。
- 悪い: 1 件のページに複数形を使う。`/@{username}/statuses` に意味のあるページを置けない（プロフィールと重なる）。決定者の指示と違う。

### 3. `/@{username}/{id}` のまま予約語で区別する

- 良い: URL が最も短く、Mastodon と同じ形。既存の URL が切れない。
- 悪い: 投稿 ID の形式（UUIDv7）とページ名の両方に「重ならない」という暗黙の制約がかかる。ID の形式を変える（ADR-0018 の Open Question にある短い表記など）と、予約語と衝突しうる。
  Go と Web クライアントの両方が UUIDv7 の正規表現を持ち続ける。

### 4. `/status/{id}`（プロフィールの外）

- 良い: 名前空間の衝突が無く、改名でリダイレクトが要らない。Misskey・Akkoma と同じ考え方。
- 悪い: URL から作者が読めない。`/status` が Web クライアントのトップレベルの経路を 1 つ消費し、`/@{username}` の下にまとめた Web クライアントのページ構成（ADR-0028 の外枠の経路）と揃わない。

## Open Questions / 要検証

- 無し。連合先が `Update(Note)` で保存済みの `url` を更新するかは、外部のサーバーと接続していない現時点では検証しない

### 出典（2026-10-20 確認）

- Mastodon: [config/routes.rb](https://github.com/mastodon/mastodon/blob/main/config/routes.rb)（`/@:account_username/:id`、`/@:username/media` など）、
  [app/services/resolve_url_service.rb](https://github.com/mastodon/mastodon/blob/main/app/services/resolve_url_service.rb)
- GoToSocial: [internal/uris/uri.go](https://codeberg.org/superseriousbusiness/gotosocial/src/branch/main/internal/uris/uri.go)（`StatusesPath`、`statusesURL := userURL + "/" + StatusesPath`）
- Misskey: [ActivityPubServerService.ts](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/server/ActivityPubServerService.ts)
- Akkoma: [router.ex](https://akkoma.dev/AkkomaGang/akkoma/src/branch/develop/lib/pleroma/web/router.ex)
