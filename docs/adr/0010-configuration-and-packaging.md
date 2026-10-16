# ADR-0010: 設定・パッケージング・デプロイテンプレート

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: <!-- 起票後に追記 -->

## Context

利用者が自由にカスタマイズでき（G3）、4 系統のターゲットに配れる（G2, NFR-DIST-2）必要がある。

## Decision（推奨案）

### 設定

- 単一の型付き設定スキーマを `core` に定義し、起動時に検証（NFR-PORT-5）。
- ソース: 環境変数（`MY_` プレフィックス、名称はプロダクト名確定時に変更）→ 任意の設定ファイル（TOML / JSON）→ 既定値。
  Workers ではバインディング（`env`）と `vars` から同じスキーマに写像する。
- アダプタ選択は明示的なキー（例: `MY_DB=postgres`, `MY_DATABASE_URL=...`, `MY_QUEUE=sqs`, `MY_BLOB=s3`）。URL スキームからの推測はしない。
- 秘密情報は環境変数 / シークレットストア（Workers Secrets, Secret Manager, Secrets Manager / SSM, Kubernetes Secret）から。

### パッケージング

- モノレポ（ワークスペース）。`core` と各アダプタを別パッケージにし、エントリポイントが必要なものだけ import する（NFR-PORT-4）。
- エントリポイント: `apps/server`（Bun / コンテナ）、`apps/server-workers`、`apps/server-lambda`。
- コンテナイメージ: multi-arch（amd64 / arm64）、非 root、distroless 系。GHCR に公開。
- 利用者による拡張: 独自アダプタは「ポートの型を実装し、コントラクトテストを通し、独自エントリポイントで組み立てる」方式。
  プラグインの動的ロードは v0 では行わない（Workers 等で不可能なため）。

### デプロイテンプレート（`deploy/`）

| ターゲット | 形式 |
| --- | --- |
| Docker | `compose.yaml`（SQLite 最小構成 / PostgreSQL + MinIO 構成） |
| microk8s / k8s | Kustomize（base + overlays）。Helm chart は需要次第 |
| Cloudflare | `wrangler.jsonc` テンプレート + セットアップスクリプト（D1 / Queues / R2 作成） |
| Google Cloud | OpenTofu モジュール（Cloud Run, Cloud Tasks, GCS, Scheduler） |
| AWS | OpenTofu モジュール（Lambda, SQS, S3, CloudFront, EventBridge Scheduler） |

IaC は OSS ライセンスの OpenTofu を採用（Terraform 互換）。

## Consequences

- 設定スキーマが「どのアダプタが何を要求するか」のドキュメントを兼ねる（スキーマからドキュメント生成）。
- テンプレート 5 種の保守負荷。M4 で各ターゲットの E2E スモークテストを nightly 化する。
