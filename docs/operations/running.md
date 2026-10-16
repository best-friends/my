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
スキーマが未適用でもプロセスは起動し、readiness が 503 を返す。

| `role` | HTTP | ジョブワーカー |
| --- | --- | --- |
| `all`（既定） | すべてのルート | 動かす |
| `web` | すべてのルート | 動かさない |
| `worker` | ヘルスチェックだけ（プラットフォームのプローブ用） | 動かす |

ログは標準エラーに JSON Lines で出る。HTTP リクエストごとに 1 行のアクセスログ（メソッド・パス・ステータス・サイズ・所要時間）を出し、
クエリ文字列は記録しない。リクエスト中のログには `request_id` が付き、同じ ID をレスポンスの `X-Request-Id` で返す。
リクエストの `X-Request-Id` が 128 文字以内の英数字と `-_.:` だけなら、ロードバランサが振った ID としてそのまま使う。

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
