# デプロイターゲットと低コスト構成

> 価格・制限値は 2026-10 時点の調査に基づく **目安** で、多くは「要検証」です。M4 で実測し、本ファイルを更新します。
> 料金は変わるので、最終的には各社の公式ページを正としてください。

## サマリ

| ターゲット | コンピュート | RDB | キュー | Blob | スケジューラ | 想定固定費 |
| --- | --- | --- | --- | --- | --- | --- |
| **Google Cloud** | Cloud Run（min instances 0） | 外部 Neon / Turso、または Cloud SQL | Cloud Tasks（push） | GCS（または R2） | Cloud Scheduler | ほぼ $0（Cloud SQL を使うと数ドル〜） |
| **AWS** | Lambda（Function URL）+ Lambda Web Adapter | Aurora Serverless v2（0 ACU）/ Aurora DSQL / RDS / 外部 | SQS（event source） | S3 + CloudFront | EventBridge Scheduler | ほぼ $0（DB 次第） |
| **Self-hosted** | Docker Compose / microk8s | SQLite（+ Litestream）/ PostgreSQL | DB キュー | ローカル FS / MinIO / 外部 S3 互換 | プロセス内 cron | VM 代（$4〜6/月 クラス） |

## Cloudflare（実行環境としては対象外）

- [ADR-0001](../adr/0001-server-language-and-runtime.md) により、Cloudflare Workers（と D1 / Cloudflare Queues）は v0 の実行環境から除外した。
- ただし **R2（S3 互換 API）と CDN は他ターゲットと組み合わせて使える**。R2 は転送量課金が小さく、メディア配信のコストを下げる候補。
- Cloudflare Containers 上での実行は将来の検討事項（需要があれば別 ADR）。

## Google Cloud

- **構成**: Cloud Run サービス（Web）+ 同一イメージのワーカー用エンドポイント。Cloud Tasks が HTTP でワーカーを叩く（push 型）。
- **RDB**: Cloud SQL は最小構成でも常時課金のため、低コスト構成では Neon（Postgres）や Turso（libSQL）などの外部サーバーレス DB を推奨。
  Cloud Run + SQLite は永続ディスクがないため非推奨。
- **キュー**: 遅延実行・レート制御・タスク名による重複排除がある Cloud Tasks を第一候補。Pub/Sub は将来オプション。

## AWS

- **構成**: Lambda（Web 用関数 + SQS コンシューマ関数、同一コード）。Lambda Web Adapter で通常の HTTP サーバーとして書ける。
- **RDB**: Aurora Serverless v2 は 0 ACU までスケールダウン可能（復帰レイテンシあり、要検証）。Aurora DSQL は PostgreSQL 互換だが
  機能制約（外部キー・一部 DDL など）があるため、コントラクトテストで可否を判断する。
- **キュー**: SQS の遅延は最大 15 分。→ 再投入チェーンで長いバックオフを実現。

## Self-hosted（Docker / microk8s）

- **最小構成**: アプリ 1 コンテナ（`role=all`）+ SQLite ファイル（ボリューム）+ ローカル FS。リバースプロキシ（Caddy 等）で TLS。
- **拡張構成**: PostgreSQL、`role=web` と `role=worker` を分離、MinIO または外部 S3 互換ストレージ。
- **microk8s**: Kustomize ベースのマニフェスト（または Helm chart）を `deploy/k8s` に提供。SQLite 構成時はレプリカ 1 固定。
- **バックアップ**: SQLite は Litestream で S3 互換へ継続レプリケーション、Postgres は `pg_dump` / マネージドのスナップショット。

## コスト見積りの観点（M4 で埋める）

| 観点 | 計測方法 |
| --- | --- |
| 1 投稿あたりの配送リクエスト数 | フォロワーのドメイン分布 × shared inbox 利用率 |
| 受信アクティビティ量 | 連合タイムラインの流量（リレー非接続時） |
| DB 読み書き行数 | タイムライン 1 回取得あたりの 読み取り行数・コンピュート時間（Neon 等の課金単位） |
| ストレージ | ローカルメディア総量、リモートメディアのキャッシュ方針別 |
| 転送量 | メディア配信（CDN 経由 / 直接） |
