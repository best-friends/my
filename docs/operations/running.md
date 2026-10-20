# サーバーの起動・停止とヘルスチェック

運営者向けの手順です。設定項目の一覧は [設定リファレンス](../reference/configuration.md)、
パッケージングの方針は [ADR-0010](../adr/0010-configuration-and-packaging.md) を参照してください。

## 起動する

```sh
my config check   # 設定の問題をまとめて表示する
my serve          # 設定の role に従って起動する（-config <file> で TOML を指定できる）
```

`my serve` は設定を検証してから DB・ジョブキュー・鍵管理のアダプタを組み立て、`listen` で HTTP を受け付ける。
スキーマは適用しない。起動前に Atlas で適用する（`make db-apply`、[ADR-0023](../adr/0023-declarative-schema-management.md)）。
Docker Compose の構成（`deploy/docker/compose.yaml`、手順は [README](../../README.md#クイックスタートdocker-compose)）では、一回限りの `schema` サービスが適用してからサーバーが起動する。
スキーマが未適用でもプロセスは起動し、readiness が 503 を返す。

| `role` | HTTP | ジョブワーカー |
| --- | --- | --- |
| `all`（既定） | すべてのルート | 動かす |
| `web` | すべてのルート | 動かさない |
| `worker` | ヘルスチェックだけ（プラットフォームのプローブ用） | 動かす |

ログは標準エラーに JSON Lines で出る。リクエスト中のログには `request_id` が付き、同じ ID をレスポンスの `X-Request-Id` で返す。
リクエストの `X-Request-Id` が 128 文字以内の英数字と `-_.:` だけなら、ロードバランサが振った ID としてそのまま使う。

HTTP リクエストとジョブの実行は、終わりにそれぞれ 1 行を出す。その行は下の [指標](#指標メトリクス) のラベルを同じ名前と値で持つので（[ADR-0055](../adr/0055-service-observability.md) §5）、
`metrics_exporter = none` の Cloud Run では、この行から Cloud Logging のログベースの指標（カウンタと分布）を作れる。

| 行（`msg`） | 項目 |
| --- | --- |
| `http request`（`INFO`。ヘルスチェックは `DEBUG`） | `http.request.method`、`http.route`、`my.surface`、`http.response.status_code`（数値）は `http.server.request.duration` のラベルと同じ値。`duration_seconds`（秒の数値）、`url.path`（クエリ文字列を除いたパス）、`http.response.body.size`、`request_id`。`http.route` と `my.surface` は、ルートに届かなかったリクエスト（ヘルスチェック、ルートの前で断ったもの）にはなく、指標も数えない |
| `job`（成功は `INFO`、`retry` は `WARN`、`dead` は `ERROR`） | `my.job.type`、`my.outcome` は `my.job.duration` のラベルと同じ値。`duration_seconds`（`my.job.duration`）、`wait_seconds`（`my.job.wait`）、`job_id`、`attempt`、失敗のときの `error` |

項目名は `.` を含む 1 つのキーで、入れ子ではない。Cloud Logging のフィルタと抽出では引用符で囲む（例: `jsonPayload."http.route"`、分布の値は `EXTRACT(jsonPayload.duration_seconds)`）。
ラベルに使う項目の値は有限の集合だけで（ADR-0055 §6）、`url.path`・`request_id`・`job_id` はラベルにしない。

## 指標（メトリクス）

設定 `metrics_exporter`（既定 `none`）でサービスの指標を出す（[ADR-0055](../adr/0055-service-observability.md)、設定は [configuration.md](../reference/configuration.md)）。

| `metrics_exporter` | 経路 | 向くターゲット |
| --- | --- | --- |
| `none` | 出さない。計測はほぼ費用がない | Cloud Run（基盤の指標とログベースの指標で代える） |
| `prometheus` | `metrics_listen`（既定 `127.0.0.1:9464`）の `GET /metrics`。公開の `listen` とは別のリスナーで、認証はない。どの `role` でも出す | Self-hosted（Prometheus 互換のサーバー・KEDA が scrape する） |
| `otlp` | OTLP/HTTP で OpenTelemetry Collector へ送る。宛先は `OTEL_EXPORTER_OTLP_ENDPOINT` など OTel の標準の環境変数 | コレクターを動かせる構成 |
| `emf` | 標準出力に CloudWatch Embedded Metric Format の JSON 行。リクエスト・ジョブの終わりに、ラベルの組ごとに 1 行 | AWS Lambda |

出す指標は次のとおり。Prometheus では名前の `.` が `_` になり、単位の接尾辞（`_seconds`、Counter の `_total`）が付く。

| 指標 | 種類 | ラベル |
| --- | --- | --- |
| `http.server.request.duration` | Histogram・秒 | `http.request.method`、`http.route`（ルートのパターンのパス。一致しないものは `other`）、`http.response.status_code`、`my.surface`（`client_api` / `admin` / `activitypub` / `oauth` / `media` / `web`） |
| `http.server.active_requests` | UpDownCounter | `my.surface` |
| `my.inbox.activities` | Counter | `activity.type`（既知の Activity の型か `other`）、`my.outcome`（`accepted` / `duplicate` / `rejected` / `ignored` / `error`） |
| `my.delivery.attempts` | Counter | `my.outcome`（`success` / `retry` / `gone` / `failed`。`gone` は 410 と、利用できない・停止したサーバーで送らなかったもの） |
| `my.delivery.duration` | Histogram・秒 | `my.outcome` |
| `my.job.duration` | Histogram・秒 | `my.job.type`（登録済みのジョブの型か `other`）、`my.outcome`（`success` / `retry` / `dead`） |
| `my.job.wait` | Histogram・秒 | `my.job.type` |
| `my.timeline.delivery.lag` | Histogram・秒 | なし（ローカルの投稿の作成から、ホームタイムラインへの書き込みとストリームへの知らせまで） |
| `my.stream.connections` | UpDownCounter | `my.stream.kind`（`user`） |
| `my.accounts.registered` | Counter | なし |
| `my.statuses.created` | Counter | `my.visibility` |
| `my.queue.ready` | Gauge | `my.job.type`（DB キューの実行可能なジョブの数） |
| `my.queue.oldest_ready_age` | Gauge・秒 | なし（DB キューの最も古い実行可能なジョブの待ち） |
| `my.queue.dead` | Gauge | なし |
| `my.instance.users`・`my.instance.statuses` | Gauge | なし（NodeInfo と同じローカルの利用者数・投稿数） |
| `my.media.stored_bytes` | Gauge・バイト | `my.media.origin`（`local` / `remote_cache`） |
| `my.metrics.snapshot.age` | Gauge・秒 | `my.metrics.snapshot.part`（`queue` / `totals`） |

ヘルスチェック（`/healthz`・`/readyz`）は計測しない。ラベルに連合先のドメイン・アカウント・URL・クライアント IP は入らない。
`my.queue.*`・`my.instance.*`・`my.media.stored_bytes` はインスタンス全体の値で、KV に置く 1 つのスナップショットから全プロセスが同じ値を出す（ADR-0055 §4）。

### システムの指標

プロセス・Go のランタイム・資源への接続の指標（[ADR-0056](../adr/0056-system-observability.md)）も、同じ `metrics_exporter` で出す。
コンテナの CPU・メモリ・台数・入口のリクエストは基盤の指標に任せ、ここにあるのは基盤から見えないプロセスの中の飽和である。
スケールの契機には使わず、警報と台数の上限の見積もりに使う。

| 指標 | 種類 | ラベル |
| --- | --- | --- |
| `go.memory.used`・`go.memory.limit`・`go.memory.gc.goal` | Gauge・バイト | `go.memory.used` だけ `go.memory.type`（`stack` / `other`）。`go.memory.limit` は `GOMEMLIMIT` があるときだけ |
| `go.memory.allocated`・`go.memory.allocations` | Counter | なし |
| `go.goroutine.count`・`go.processor.limit`・`go.config.gogc` | Gauge | なし |
| `go.schedule.duration`・`go.memory.gc.pause.duration` | Histogram・秒 | なし（実行可能になってから動き出すまでの待ち、GC の停止） |
| `go.memory.gc.cycles` | Counter | なし |
| `process.cpu.time` | Gauge・秒（起動からの合計） | `cpu.mode`（`user` / `system`） |
| `process.memory.usage` | Gauge・バイト | なし（常駐メモリ、RSS） |
| `process.unix.file_descriptor.count`・`my.process.file_descriptor.limit` | Gauge | なし（開いたファイル記述子の数と上限） |
| `process.uptime` | Gauge・秒 | なし |
| `db.client.connection.count` | Gauge | `db.client.connection.pool.name`（`postgres` / `sqlite`）、`db.client.connection.state`（`idle` / `used`） |
| `db.client.connection.max` | Gauge | `db.client.connection.pool.name` |
| `db.client.connection.wait_time`・`my.db.client.connection.waits`・`db.client.connection.timeouts` | Gauge（起動からの合計。秒・回） | `db.client.connection.pool.name`。`timeouts` は PostgreSQL だけ |
| `my.adapter.operation.duration` | Histogram・秒 | `my.adapter`（`queue.db` / `kv.db` / `kv.memory` / `kv.valkey` / `stream.<stream>` / `blob.fs`）、`my.operation`、`my.outcome`（`success` / `error`） |
| `my.adapter.connections` | UpDownCounter | `my.adapter`（`valkey` の接続、`stream.<stream>` の受信） |
| `my.adapter.reconnects` | Counter | `my.adapter` |
| `http.client.request.duration` | Histogram・秒 | `my.http.purpose`（`json` / `html` / `image` / `video` / `human_check` / `web_push` / `webhook`）、`http.request.method`、`my.http.status_class`（`2xx` など）、`error.type`（`forbidden_destination` は SSRF ガードの拒否） |
| `http.client.open_connections` | UpDownCounter | なし |
| `my.readiness.failures` | Counter | なし |

- Go のランタイムの指標（`go.*`）は `prometheus` と `otlp` だけが出す。プロセスと接続のプールの Gauge は、`emf` ではスナップショットの定期ジョブが書く
- プールの待ちと CPU 時間は、プールとランタイムが持つ起動からの合計なので Gauge で出す。PromQL の `rate()` で増え方を見る
- キューの取得（`claim`）は、ジョブを取れたときと失敗したときだけ計測する。待機中のワーカーのポーリングは何も記録しない
- `db.client.connection.pending_requests` は出さない。pgx と `database/sql` の統計に待っている数がないので、待ちの回数と時間で代える
- 警報の目安: `go.memory.used` の合計と `go.memory.limit` の比、`db.client.connection.wait_time` の増え方（プールの上限に当たっている）

### メモリの上限（`GOMEMLIMIT`）

`GOMEMLIMIT` は運営者が設定する Go のランタイムの環境変数で、アプリは自動では決めない。
設定がなければ Go の既定（上限なし）で動き、ヒープはコンテナのメモリの上限に近づいても GC を急がない。
コンテナのメモリの上限がある構成（Cloud Run・Lambda・ECS・Kubernetes）では、その約 90% を目安に設定する（例: 512 MiB のコンテナで `GOMEMLIMIT=460MiB`）。
`GOMAXPROCS` は設定しない。Go の既定がコンテナの CPU の上限に従う。

## 停止する

SIGTERM または SIGINT で graceful shutdown する。

1. 新しい接続とジョブの取得をやめる。
2. 処理中のリクエストと実行中のジョブの終了を最大 25 秒待つ（Kubernetes・ECS の既定の猶予 30 秒に収まる）。
3. 25 秒を過ぎたら、残ったリクエストとジョブのコンテキストを取り消して接続を閉じ、終了コード 1 で終わる。
   取り消されたジョブは再試行され、途中で強制終了されたジョブもリースが切れると別のワーカーが拾い直す（[ADR-0004](../adr/0004-job-queue-abstraction.md)）。

## ヘルスチェック（FR-802）

| パス | 種類 | 200 の条件 |
| --- | --- | --- |
| `GET /healthz` | liveness | プロセスが HTTP に応答できる。DB などの依存は見ない（DB の遅延でプロセスを再起動させないため） |
| `GET /readyz` | readiness | DB に接続でき、スキーマが適用済み |

- 200 の本文は `{"status":"ok"}`。
- readiness が満たされないときは 503 と RFC 9457 の `application/problem+json`（[ADR-0016](../adr/0016-client-api-style.md)）を返す。
  `title` は `Accept-Language` に合わせて翻訳され、内部の詳細は含めない。原因（接続エラー、足りないテーブル名）はログに出る。
- 1 回の readiness 判定は 3 秒で打ち切る。
- **スキーマが適用済み** とは、`schema/<dialect>.sql` が `CREATE TABLE` で宣言するテーブルがすべて存在すること
  （SQLite は `sqlite_schema`、PostgreSQL は接続の `current_schema()` で調べる）。Atlas はファイル全体を適用するので、
  テーブルが欠けていれば未適用か、バイナリより古いスキーマである。列や索引の差分までは見ない。差分の検出は `make db-check` の役割。

## セキュリティヘッダ

すべてのレスポンスに [ADR-0019 §9](../adr/0019-security-baseline.md) のヘッダ（`Strict-Transport-Security`、`X-Content-Type-Options`、
`Referrer-Policy`、`Permissions-Policy`、`Cross-Origin-Opener-Policy`、`Cross-Origin-Resource-Policy: same-site`、`X-Frame-Options`）と、
文書ではないレスポンス向けの `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'` が付く。
ActivityPub の JSON や公開メディアなど、別の値が必要なレスポンスはハンドラが上書きする。
Web クライアントの HTML には、nonce を使わない文書用の CSP（`script-src 'self'` など ADR-0019 §9 のポリシー）が付く。
Trusted Types を必須にし（`require-trusted-types-for 'script'`）、作れるポリシーをクライアントの 2 つ（`trusted-types my-server-sanitized-html my-service-worker`）に限る。

## Web クライアント

`role` が `web` か `all` のプロセスは、バイナリに埋め込んだ Web クライアントを配る（[ADR-0028](../adr/0028-html-shell-link-previews-and-pwa.md)）。

| パス | 内容 | `Cache-Control` |
| --- | --- | --- |
| ほかのルートに当たらない `GET`（`/`、`/@alice`、`/@alice/status/{id}`、`/settings` など） | SPA の HTML の外枠。インスタンスの設定は `<script type="application/json" id="config">` で渡す | `public, max-age=60, stale-while-revalidate=300`、ETag |
| `GET /assets/*` | ビルドのハッシュ付きファイル | `public, max-age=31536000, immutable`、ETag |
| `GET /manifest.webmanifest` | インスタンスの設定から作る Web アプリマニフェスト | `public, max-age=60, stale-while-revalidate=300`、ETag |
| `GET /sw.js`、`/icon.svg` などビルドの直下のファイル | そのまま配る | `no-cache`、ETag |

- `/api`、`/oauth`、`/auth`、`/.well-known`、`/nodeinfo`、`/ap`、`/media`、`/uploads`、`/theme`（サーバーのテーマのスタイルシート）の下は外枠を返さず、ほかのルートに当たらなければ 404 の problem になる。
- 外枠は ActivityPub のコンテントネゴシエーションの後に返す。ActivityPub の `Accept` で外枠のパスを取ると 404 になる（投稿のページは下記のリダイレクト）。
  外枠には常に `Vary: Accept` が付く。CDN を前に置くときは `Accept` の種類（ActivityPub か HTML か）をキャッシュキーに含めるか、
  外枠と `/ap/*`、`/@*` をキャッシュ対象から外す。CDN から配るのは `/assets/*` だけにする。
- Web クライアントを含めずにビルドしたバイナリ（`make build` を通さない `go build`）は、起動時に警告を出し、スクリプトのない外枠を返す。

## 連合のエンドポイント（FR-401〜403）

`role` が `web` か `all` のプロセスは、他の Fediverse サーバーがアカウントを見つけて表示するための次のエンドポイントに答える。
URI の形式は [ADR-0018](../adr/0018-federated-uris.md) に従う。

| パス | 内容 |
| --- | --- |
| `GET /.well-known/webfinger` | WebFinger（RFC 7033）。`resource` は `acct:alice@{account_domain}`（`acct:alice@{host}` も可）、Actor の `id`、`/@alice` の URL。`subject` は常に `account_domain` 側のハンドル |
| `GET /.well-known/host-meta`、`/.well-known/host-meta.json` | WebFinger の URI テンプレート（XRD / JRD） |
| `GET /.well-known/nodeinfo`、`GET /nodeinfo/2.1` | NodeInfo 2.1。`software.repository` は設定 `source_url`（AGPL 第 13 条、[ADR-0011](../adr/0011-license.md)） |
| `GET /ap/actors/{uuid}` | Actor（`publicKey` は RSA の `#main-key`）。HTML を優先するリクエストは `/@{username}` へ 303 |
| `GET /ap/actors/{uuid}/{outbox,followers,following,featured,featured-tags}` | コレクション。投稿・フォローを実装するまでは空 |
| `GET /@{username}` | ActivityPub の `Accept` なら Actor の `id` へ 302。それ以外は Web クライアントの HTML（下記） |
| `GET /@{username}/status/{uuid}` | ActivityPub の `Accept` なら、未ログインで見られる投稿の `id`（`/ap/objects/{uuid}`）へ 302、見られなければ 404（[ADR-0053](../adr/0053-status-page-url.md)）。それ以外は Web クライアントの HTML |

- インスタンス Actor のハンドルは `{account_domain}@{account_domain}`（Mastodon と同じ形）。
- 存在しない、または見せられない（停止中・削除済み・リモートの）アカウントは、どれも同じ 404（`Cache-Control: no-store`）になる。
- 匿名で取れる応答には `Cache-Control: public, max-age=60` を付ける。`Accept` で出し分ける応答には `Vary: Accept` が付く。
  `Vary: Accept` を扱えない CDN では `/ap/*` と `/@*` をキャッシュ対象から外す。
- inbox（`{actor}/inbox`）と shared inbox（`/ap/inbox`）は Actor に載っているが、まだ受け付けない。

### ハンドルと配信のドメインを分ける場合

`account_domain` を `host` と別にしたときは、`account_domain` 側の `/.well-known/webfinger`、`/.well-known/host-meta`、
`/.well-known/nodeinfo` を `host` へ 301 でリダイレクトする（`Access-Control-Allow-Origin: *` を付ける）。
`account_domain` へのリクエストをこのサーバーに届ければ、サーバー自身がそのリダイレクトを返す。
別のサーバーやリバースプロキシで返す場合は、パスとクエリ文字列を保ったまま `https://{host}` へリダイレクトする。
