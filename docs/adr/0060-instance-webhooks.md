# ADR-0060: サーバー単位の webhook

- Status: Accepted
- Date: 2026-10-20
- Deciders: @rosylilly
- Issue: #650

## Context（背景・課題）

決定者の指示は「管理者によって設定されるサーバー単位 webhook の仕組み」である。
通報（FR-701）・承認待ちの登録（FR-721）・処分（FR-703, FR-702）が起きたとき、いまの運営者が知る手段は
アプリ内の通知とメール（FR-712）だけで、Slack・Discord・チケット管理・自動化ツール（n8n など）へ流す口がない。
webhook の仕組みも FR もないので、FR-722 を足し、送るイベント・ペイロード・署名・再送・宛先の HTTP・データモデル・権限を決める。

この決定は、既存の Accepted ADR の次の規約の上に立つ。

- ジョブは at-least-once で、ハンドラは冪等、トランザクション内では Outbox で投入する（ADR-0004、NFR-REL-1〜3）。
- 外部への HTTP は SSRF ガード付きの `kernel.HTTPClient` を通す（ADR-0019 §1、NFR-SEC-1）。
  いまのポートは用途（`HTTPPurpose`）ごとに応答の上限を持ち、リダイレクトを最大 5 回たどる（`internal/app/kernel/httpclient.go`）。
- テーブルはリソース・イベント・投影に分け、状態を NULL 許容の日時で表さない（ADR-0020）。
- 管理操作は監査ログに残す（ADR-0037、FR-704）。秘密情報は `kernel.Sealer` で封をして DB に置く（ADR-0019 §6、NFR-SEC-7）。
- 管理の権限は `internal/domain/identity` の `Permission`（`manage_reports` など）で表す。モデレーションとサーバー管理の権限の分け方は ADR-0058（#648）で並行して検討している。

主要実装（2026-10-20 にソースと公式文書で確認）:

| | Mastodon | Misskey（SystemWebhook） | Standard Webhooks |
| --- | --- | --- | --- |
| イベント | `account.approved` / `account.created` / `account.updated` / `report.created` / `report.updated` / `status.created` / `status.updated` | `abuseReport` / `abuseReportResolved` / `userCreated` / `inactiveModeratorsWarning` / `inactiveModeratorsInvitationOnlyChanged` | 名前は自由（ドット区切りを推奨） |
| ペイロード | `{event, created_at, object}`。テンプレートで本文を差し替えられる | 独自の JSON | `{type, timestamp, data}` を推奨、20 KB 未満を推奨 |
| 署名 | `X-Hub-Signature: sha256=<本文の HMAC-SHA256 の hex>`（WebSub 由来）。時刻・ID を署名しない | 署名しない。秘密をそのまま `X-Misskey-Hook-Secret` ヘッダで送る | `webhook-id` / `webhook-timestamp` / `webhook-signature`。`id.timestamp.body` の HMAC-SHA256（`v1`）か Ed25519（`v1a`）。署名を空白区切りで複数並べて鍵を回転する |
| 再送 | Sidekiq の retry 16 回、dead に送らない | 4xx（再送可能なもの以外）で打ち切り、それ以外は再送 | 数日にわたる指数バックオフとジッタ。リダイレクトは失敗として扱う。`410 Gone` で宛先を無効にする。タイムアウトは 15〜30 秒 |
| 失敗の記録・無効化 | 自動の無効化なし | 最後の送信時刻と状態コードだけを持つ | 失敗が続けば別の経路で知らせ、無効化を検討する |
| 宛先の検査 | `allow_local: true` で内部のアドレスへも送る | 共通の HTTP サービスを使う | 内部のアドレスを塞ぐプロキシか、内部に届かないサブネットから送ることを推奨 |
| 権限 | 購読するイベントごとに要る権限（`account.*` は `manage_users`、`report.*` は `manage_reports`、`status.*` は `view_devops`） | 管理者 | — |

出典: Mastodon の [`Webhook`](https://github.com/mastodon/mastodon/blob/main/app/models/webhook.rb)・
[`Webhooks::DeliveryWorker`](https://github.com/mastodon/mastodon/blob/main/app/workers/webhooks/delivery_worker.rb)・
[管理者向け文書](https://docs.joinmastodon.org/admin/webhooks/)、
Misskey の [`SystemWebhook`](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/models/SystemWebhook.ts)・
[`SystemWebhookDeliverProcessorService`](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/queue/processors/SystemWebhookDeliverProcessorService.ts)、
[Standard Webhooks の仕様](https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md)。

## Decision Drivers（判断基準）

1. 安全であること: 管理の権限を持つ人やその乗っ取られたアカウントが、webhook を内部ネットワークへの踏み台（SSRF）や、権限外のデータの持ち出しに使えないこと
2. 受け手が検証しやすいこと: 署名・再送・重複排除が既存のライブラリで扱える公開された形であること
3. 既存の規約（ADR-0004 の Outbox とジョブ、ADR-0020 のデータモデル、ADR-0037 の監査ログ）にそのまま乗ること
4. webhook を設定しないサーバーに費用がかからないこと（NFR-COST）
5. 1 人で運用するサーバーでも設定が少なく済むこと

## Considered Options（選択肢）

1. **Standard Webhooks の形で送り、宛先は `kernel.HTTPClient` を通し、内部の宛先は設定ファイルの許可リストでだけ許す**
2. **Mastodon の admin webhook に合わせる**: `X-Hub-Signature`・`{event, created_at, object}`・Mastodon のイベント名・内部の宛先も許す
3. **webhook を持たない**: 管理 API のポーリング、ストリーミング、構造化ログを外部の仕組みで拾ってもらう

宛先の HTTP については、さらに次の 3 つを比べる。

- (a) SSRF ガードを常にかける
- (b) webhook ではガードを外す（Mastodon の `allow_local`）
- (c) ガードをかけ、デプロイの設定（ADR-0010）に書いたホスト・CIDR だけを例外にする

## Decision（決定）

**選択肢 1 と、宛先の扱い (c) を採用する。**

### イベントとペイロード

- 本文は Standard Webhooks の推奨形 `{"type": "<イベント>", "timestamp": "<RFC 3339>", "data": {...}}` の JSON にする。
  `data` は、同じ対象を返す管理 API（`/api/v1/admin/...`）の表現を、イベントが起きたトランザクションの中で写したものにする。
  型は `api/openapi.yaml` の `components/schemas` に置く（`webhooks` キーワードは ADR-0029 の部分集合の外なので使わない）。イベントの一覧と `data` の型は参照文書 `docs/reference/webhooks.md` に書く。
- 版: v0 のあいだは版を持たず、`data` は管理 API の `/api/v1` の表現に従う（ADR-0012）。
  v1 以降に `data` の互換を壊すときは、宛先ごとに API の版を選ぶ列を足す ADR を書く。
- イベントと、購読に要る権限（Mastodon と同じく、購読するイベントのすべてについて、設定する人がその権限を持つ必要がある）:

| `type` | 起きるとき | 要る権限 |
| --- | --- | --- |
| `report.created` | 通報の作成（ローカル・リモートの `Flag`。FR-701） | `manage_reports` |
| `report.resolved` / `report.reopened` | 通報の解決・再開（FR-710） | `manage_reports` |
| `registration.pending` | 承認制での登録の申請（FR-721） | `manage_accounts` |
| `account.created` | ローカルのアカウントの作成（登録が有効になったとき） | `manage_accounts` |
| `moderation_action.created` / `moderation_action.lifted` | アカウントへの処分と解除（FR-703, FR-713） | `manage_accounts` |
| `appeal.created` | 異議申し立て（FR-707） | `manage_accounts` |
| `domain_moderation_action.created` / `domain_moderation_action.lifted` | ドメイン単位の処分と解除（FR-702） | `manage_domains` |
| `account.updated` | ローカルのアカウントのプロフィールの変更（新しい版の記録） | `manage_accounts` |
| `status.created` | ローカルの利用者の公開範囲 `public` の投稿 | `manage_webhooks`（ほかに要らない） |
| `custom_emoji.created` / `custom_emoji.updated` / `custom_emoji.deleted` | カスタム絵文字の管理操作（ADR-0051、#618 の §5 で監査ログに記録する操作）。作成とリモートの絵文字の複製は `created`、画像の差し替え・設定と別名の変更・無効化と有効化は `updated`、削除は `deleted` | `manage_custom_emojis`（ADR-0051、#618） |
| `webhook.test` | 管理画面の「テスト送信」（購読の設定に関係なく、その宛先にだけ送る） | — |

- `status.created` は `public` だけを送る。`unlisted` 以下と `local`（ADR-0017）の投稿は、サーバーの外の宛先に出さない。リモートの投稿は量が読めないので送らない。
- `account.updated` はローカルのアカウントだけを送る。リモートのプロフィールの取り込みは送らない。
- `custom_emoji.*` は管理操作だけで起きる。リモートの絵文字を投稿やプロフィールの受信で記録したときは送らない。
  イベント名と `data` の表現（管理 API `/admin/custom_emojis` の表現）は ADR-0051（#618）の名前に合わせる。
- 権限の検査は設定・変更のときに行う。設定した人が後で権限を失っても、宛先は止めない（Mastodon と同じ。止めたいときは `manage_webhooks` を持つ人が無効にする）。

### 署名と秘密

- [Standard Webhooks](https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md) の `v1`（HMAC-SHA256）で署名し、
  `webhook-id`（メッセージの UUIDv7。再送しても同じ）・`webhook-timestamp`（送信の試行ごとの Unix 秒）・`webhook-signature`（`v1,<base64>`）を付ける。
  署名する文字列は `webhook-id.webhook-timestamp.body` で、body は DB に保存したバイト列をそのまま送る。
- 秘密は宛先ごとに 32 バイトの乱数で、`whsec_<base64>` の形で 1 度だけ見せる（作成時と回転時）。DB には `kernel.Sealer` で封をして置く。
- 回転は新しい秘密を記録するだけで、古い秘密は 24 時間のあいだ有効なまま残し、そのあいだは 2 つの署名を空白区切りで並べる。
- 非対称の `v1a`（Ed25519）は持たない。受け手が公開鍵だけで検証できる利点はあるが、宛先ごとの秘密で足りる。

### 配送と再送

- webhook を所有する新しいモジュール `webhook`（`internal/domain/webhook`・`internal/app/webhook`）を置く。
  各モジュールのユースケースが元のイベントと同じトランザクションで発行するドメインイベント（監査ログ・FR-712 の通知と同じ経路）を、`webhook` の同期の購読者が受け、
  購読している有効な宛先ごとに配送の行を書き、`webhook.deliver` ジョブを `queue.Outbox` で投入する（冪等キーは配送の ID）。
  購読する宛先が無ければ何も書かないので、webhook を使わないサーバーの費用は増えない。
- `webhook.deliver` のハンドラは、トランザクションの外で 1 回送り、結果を試行のイベントとして書く。
  - `2xx` は成功。`410 Gone` は宛先を無効にして終わる。それ以外の状態コード・タイムアウト・接続の失敗は再送する。`Retry-After` があれば従う（上限 1 時間）。
  - 再送の間隔は Standard Webhooks の例に合わせ、すぐ・5 秒・5 分・30 分・2 時間・5 時間・10 時間・10 時間（合計 約 27 時間、8 回）とする（`JobHandlerRegistry.RegisterWithPolicy`）。
    使い切ったら ADR-0004 のとおり `dead_jobs` に移し、管理画面からその配送を再送できる（同じ `webhook-id`）。
  - 宛先が無効なら送らずに終わる（再び有効にしても、無効だったあいだの配送は送り直さない）。
- **自動の無効化**: 宛先への配送が 1 度も成功しないまま、最初の失敗から 72 時間がたち、かつ連続して 20 回以上失敗したら、宛先を無効にする。
  無効にしたことは監査ログ（`actor_kind = system`）に残し、`manage_webhooks` を持つ利用者にアプリ内で通知する（FR-712 と同じ仕組みの新しい種類）。

### 宛先の HTTP

- 送信は `kernel.HTTPClient` を通す。新しい用途 `HTTPPurposeWebhook`（応答の本文の上限 64 KiB、全体のタイムアウト 15 秒）を足し、この用途ではリダイレクトをたどらない（`3xx` は失敗。Standard Webhooks の推奨）。
- 内部ネットワークの宛先（同じホストの n8n、社内のチャットなど）は、デプロイの設定 `webhook_allowed_destinations`（ホスト名か CIDR の一覧。環境変数 `MY_WEBHOOK_ALLOWED_DESTINATIONS`）に書いたものだけ、この用途に限ってガードの例外にする。
  例外の宛先に限り `http` のスキームと任意のポートも許す。設定の変更にはサーバーへの権限が要るので、管理画面の権限だけでは内部に届かない。
- 開発用の `http_allow_private_networks` は、webhook を含むすべての用途のガードを外す。

### データモデル（ADR-0020）

`webhook` モジュールが所有する。

| テーブル | 分類 | 内容 |
| --- | --- | --- |
| `webhook_endpoints` | リソース | `id`, `created_by_user_id`（NULL 可 = CLI）, `created_at` |
| `webhook_endpoint_revisions` | イベント（版） | `id`, `endpoint_id`, `url`, `description`, `revised_by_user_id`（NULL 可）, `revised_at` |
| `webhook_endpoint_revision_events` | イベント（版の子） | PK `(revision_id, event_type)`。`event_type` は enum |
| `webhook_secrets` | イベント | `id`, `endpoint_id`, `sealed_secret`（`bytes`）, `created_by_user_id`（NULL 可）, `created_at`。回転は新しい行。古い行は次の行の `created_at` から 24 時間で使わなくなる |
| `webhook_endpoint_state_changes` | イベント | `id`, `endpoint_id`, `state`（`enum(enabled, disabled, deleted)`）, `reason`（`enum(admin, gone, failing)`）, `changed_by_user_id`（NULL 可）, `changed_at` |
| `webhook_messages` | リソース | `id`（= `webhook-id`）, `event_type`, `body`（`bytes`。送る本文そのもの）, `created_at` |
| `webhook_deliveries` | リソース | `id`, `message_id`, `endpoint_id`, `created_at`。一意 `(message_id, endpoint_id)` |
| `webhook_delivery_attempts` | イベント | `id`, `delivery_id`, `outcome`（`enum(succeeded, http_error, timeout, connection_error, forbidden_destination)`）, `status_code`（`outcome = http_error` と `succeeded` のときだけ。`CHECK`）, `duration_ms`, `attempted_at` |
| `webhook_endpoint_heads` | 投影 | `endpoint_id`, `revision_id`, `state`, `secret_id` |
| `webhook_endpoint_failures` | 投影 | `endpoint_id`（PK）, `first_failure_attempt_id`, `consecutive_failures`。行がある = 成功せずに失敗が続いている。成功した試行で行を消す |

- `body` は「解釈しない文書」（規約 4）で、通報のコメントなどの個人データを含む。`webhook_messages` と配送・試行の行は、作成から 14 日で消す定期ジョブを持つ（FR-803）。
  14 日は固定の値で、ADR-0038 の保持期間の設定（`retention_*`）には足さない。
- URL は、Discord や Slack のように経路やクエリに秘密を含むことがあるので、ログと監査ログにはホスト名だけを出す。

### 管理 API・画面・監査ログ・権限

- 新しい権限 `manage_webhooks` を足す。webhook の一覧・作成・変更・削除・有効化・無効化・秘密の回転・テスト送信・配送の履歴と再送に要る。
  ADR-0058（#648）の分け方では、サーバー管理の側に置く。
- 管理 API: `/api/v1/admin/webhooks`（一覧・作成）、`/{id}`（取得・変更・削除）、`/{id}/enable`・`/{id}/disable`・`/{id}/rotate_secret`・`/{id}/test`、
  `/{id}/deliveries`（キーセットページングの履歴）と `/{id}/deliveries/{delivery_id}/redeliver`。管理画面に同じ操作を置く。
- 監査ログ（ADR-0037）: 作成・変更・削除・有効化・無効化（自動を含む）・秘密の回転を記録する。`target_kind` に `webhook` を足し、`target_label` は説明とホスト名にする。テスト送信と再送は記録しない。

### この決定を覆す条件

- 受け手の多くが Mastodon の `X-Hub-Signature` しか検証できず、利用者から互換を求められた場合は、宛先ごとに署名の形を選べるようにする。
- 内部の宛先を設定ファイルで許す形が、マネージドの実行環境（Cloud Run・Lambda）で VPC 内の宛先に届かず使えない場合は、許可リストの書き方を見直す。

## Consequences（影響）

- 良い影響
  - 受け手は Standard Webhooks の公式ライブラリ（各言語）でそのまま検証でき、`webhook-id` で重複を捨てられる。時刻を署名するので再生攻撃も防げる。
  - 管理画面の権限だけでは内部ネットワークに届かず、権限外のイベントも購読できない。
  - 配送は既存のジョブ・Outbox・`dead_jobs` に乗り、新しいキューを持たない。webhook を設定しないサーバーには行もジョブも増えない。
- 悪い影響・トレードオフ
  - Mastodon の webhook 向けに作った受け手（`X-Hub-Signature` を検証するもの）はそのままでは使えない。
  - Mastodon にあるテンプレート（Discord・Slack の形への変換）は持たない。チャットに流すには中継（n8n など）が要る。
  - 本文をトランザクションの中で作るので、イベントを発行するユースケースの書き込みに `data` の組み立てが加わる（購読する宛先があるときだけ）。
  - モジュールが 1 つ増え、overview §2・data-model §4 の表と depguard の設定を更新する必要がある。
- 後続で必要になる作業（#666 の実装で Issue に分ける）
  - `webhook` モジュール、スキーマ、`webhook.deliver` ジョブと消去ジョブ
  - `kernel.HTTPPurposeWebhook`・リダイレクトをたどらない指定・`webhook_allowed_destinations` の設定とコントラクトテスト
  - 各モジュールからのドメインイベントの発行と `data` の組み立て、`api/openapi.yaml` の管理 API と `docs/reference/webhooks.md`
  - `manage_webhooks` の権限、監査ログの `action` と `target_kind`、自動の無効化の通知の種類
  - 管理画面（一覧・作成・秘密の表示・配送の履歴・再送・テスト送信）

## Pros and Cons of the Options（詳細比較・任意）

### 1. Standard Webhooks の形（採用）

- 良い: 署名・ID・時刻・回転・再送・`410` の扱いが仕様として公開され、受け手のライブラリがある（Driver 2）。時刻を署名に含むので、Mastodon の形より再生攻撃に強い。
- 悪い: Fediverse の既存の受け手との互換はない。

### 2. Mastodon の admin webhook に合わせる

- 良い（最も強い論点）: Mastodon 向けの受け手やボット、テンプレートの書き方をそのまま使え、運営者が移ってきたときの学習が要らない。
- 悪い: 署名が本文だけで時刻と ID を含まず、同じ要求を再生されても受け手は見分けられない。Mastodon の Client API との互換はしない方針（ADR-0007）で、`object` の形も Mastodon の Admin API の表現なので、形だけ合わせても互換にならない。
  内部の宛先を無条件に許すのは Driver 1 に反する。

### 3. webhook を持たない

- 良い: 実装が要らない。管理 API のポーリングやストリーミングで同じ情報は取れる。
- 悪い: 受け手が常駐してポーリングする必要があり、チャットや自動化ツールに流すという決定者の指示を満たさない。

### 宛先の HTTP の (a)・(b)・(c)

- (a) 常にガード: 最も安全だが、同じホストや VPC 内の自動化ツールに送れず、self-hosted の典型的な使い方を塞ぐ。
- (b) ガードを外す: 最も手軽で Mastodon と同じだが、`manage_webhooks` を持つ人（乗っ取りを含む）がクラウドのメタデータエンドポイントや内部の管理画面に要求を送れる。応答の本文は見せなくても、状態コードと所要時間が配送の履歴に出るので、内部の探索に使える。
- (c) 設定の許可リスト（採用）: 内部に届くかを決めるのはサーバーに権限を持つ運営者だけになり、(a) の安全さを保ったまま (b) の使い方ができる。

## Open Questions / 要検証

- なし
