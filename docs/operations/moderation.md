# アカウント・ドメインへの処分と通報の対応

運営者・モデレーター向けの手順です。要件は FR-701〜FR-703（[機能要件](../requirements/01-functional.md)）、
テーブルは [データモデル §4.3](../architecture/data-model.md#43-moderation)、連合への効き方は [federation.md §11](../architecture/federation.md#11-モデレーションとの連携) です。

どれも管理 API（`/api/v1/admin/...`、スコープ `admin:read` / `admin:write`）と CLI（`my admin account action` / `my admin domain` / `my admin report`）で行えます。
管理 API はロールの権限を要します。アカウントへの処分は `manage_accounts`、ドメイン単位の処分は `manage_domains`、通報は `manage_reports` です。
CLI はサーバーと同じ設定（`-config` か環境変数）で DB に直接つなぎ、操作者を記録しません（`issued_by_user_id` などは NULL）。

## アカウントへの処分

処分は記録したトランザクションで制限の投影（`account_restrictions`）に反映され、次の要求から効きます（[データモデル §5.2](../architecture/data-model.md#52-処分の種類と制限能力の剥奪)）。

| 種類 | ローカルのアカウントへの効果 |
| --- | --- |
| `warn` | 制限はない（本人に知らせるだけ） |
| `force_sensitive` | 添付をすべてセンシティブとして表示する |
| `limit` | 投稿（過去の投稿を含む）をフォロワー限定として扱い、公開タイムラインに出さない |
| `restrict_posting` | 投稿・返信・ブースト・編集を禁止する |
| `restrict_interaction` | フォロー・お気に入り・ブースト・返信を禁止する |
| `disable_login` | ログインと API の利用を禁止し、有効なセッションとトークンを失効させる |
| `suspend` | 上の制限をすべて課し、プロフィールと投稿を本人以外に見せない |

リモートのアカウントには `limit`・`suspend`・`force_sensitive` だけを出せます。`suspend` はそのアカウントからの受信を捨てます。
`ban`・`delete_statuses`・`mark_statuses_sensitive` はまだ出せません。

```sh
my admin account action issue -kind suspend -category spam -public-reason "spam" -duration 720h @alice
my admin account action issue -kind limit -report <report id> @bob@remote.example
my admin account action list -active @alice
my admin account action lift -reason "appealed" <action id>
```

管理 API では `POST /api/v1/admin/accounts/{id}/actions`（本文は `kind`・`reason_category`・`public_reason`・`private_note`・`report_id`・`notify_target`・`expires_in` 秒）、
`GET /api/v1/admin/accounts/{id}/actions`（`active` で絞り込み）、`GET /api/v1/admin/account_actions/{id}`、`POST .../{id}/lift`（本文 `{"reason": "..."}` は省略可）です。

- アカウントは CLI では `@username`（ローカル）、`@username@domain`（リモート）かアカウント ID で、管理 API ではアカウント ID で指定します。
- 複数の処分は重なります（制限の和集合）。解除はその処分の制限だけを消し、期限付きの処分は期限で自然に終わります（一覧では `expired`）。
- `disable_login` と `suspend` が失効させたセッションとトークンは、解除しても戻りません。本人はもう一度ログインします。
- `report_id`（`-report`）は、対象のアカウントの通報だけを指定できます。
- `public_reason` は本人に見せる理由、`private_note` はモデレーターだけのメモです。`notify_target`（既定は真、CLI では `-no-notify` で偽）は本人に知らせるかどうかです。

## ドメイン単位の処分

| 種類 | 効果 |
| --- | --- |
| `suspend` | 配送・取得・受信をすべて止める（受信は署名の検証より前に `403`）。既存のアカウントと投稿はタイムラインやプロフィールから隠れる |
| `limit` | 投稿を公開タイムライン・検索に出さず、フォローしていない利用者への通知を作らない |
| `reject_media` | メディアを取得しない |
| `reject_reports` | そのドメインからの通報（`Flag`）を捨てる |
| `force_sensitive` | メディアをセンシティブとして表示する |

```sh
my admin domain issue -kind suspend -public-comment "spam" spam.example
my admin domain issue -kind limit -duration 720h noisy.example
my admin domain list -active
my admin domain lift <action id>
```

管理 API では `POST /api/v1/admin/domain_actions`（本文は `domain`・`kind`・`public_comment`・`private_note`・`obfuscate`・`expires_in` 秒）、
`GET /api/v1/admin/domain_actions`（`domain`・`active` で絞り込み）、`GET .../{id}`、`POST .../{id}/lift` です。

- ドメインは IDNA の A-label（小文字、末尾のドットなし）に正規化して記録します。`例え.jp` と `xn--r8jz45g.jp` は同じドメインです。
- 処分はサブドメインにも効きます（`spam.example` の処分は `a.spam.example` にも効く）。
- 処分は記録したトランザクションで制限の投影（`domain_restrictions`）とタイムラインの制限に反映され、次の配送・取得・受信から効きます。
- 期限（`-duration` / `expires_in`）を付けた処分は期限で自然に終わります。期限切れは「解除」ではないので、一覧では `expired` と表示します。
- 解除はその処分の制限だけを消します。同じドメインへの別の処分は残ります。解除済みの処分をもう一度解除しても何も変わりません。
- 一覧は新しい順で、`-active`（`active=true`）を付けると効いているものだけを出します。

## 通報

他のサーバーから受信した通報（`Flag`）は、未解決（`open`）として通報の一覧に入ります。

```sh
my admin report list -state open
my admin report show <report id>
my admin report assign <report id> <username>
my admin report unassign <report id>
my admin report resolve -outcome dismissed <report id>
```

管理 API では `GET /api/v1/admin/reports`（`state` で絞り込み）、`GET .../{id}`、`POST .../{id}/assign_to_self`、`POST .../{id}/unassign`、
`POST .../{id}/resolve`（本文 `{"outcome": "actioned" | "dismissed"}`）です。

- 担当（`assign`）は、ロールが `manage_reports` を持つローカルの利用者にだけ割り当てられます。管理 API では自分を担当にします。
- 解決（`resolve`）の結果は、対象に処分をした `actioned` か、処分の必要がなかった `dismissed` です。解決済みの通報をもう一度解決しても結果は変わりません。
- 担当・解決はイベント（`report_assignments`・`report_resolutions`）として残り、現在の状態は `report_states` が持ちます。
