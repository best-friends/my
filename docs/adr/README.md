# Architecture Decision Records

設計判断の記録です。運用ルールは [ADR-0000](0000-record-architecture-decisions.md) を参照してください。

- 新規作成: `make adr-new TITLE="短いタイトル"`
- 一覧の再生成: `make adr-index`（CI で差分を検査します）
- `Proposed` は推奨案であり未確定です。`Accepted` にできるのは人間の Decider のみです。

## 一覧

<!-- adr-index:start -->
| ADR | タイトル | Status |
| --- | --- | --- |
| [0000](0000-record-architecture-decisions.md) | 設計判断を ADR で記録する | Accepted |
| [0001](0001-server-language-and-runtime.md) | サーバーの実装言語は Go とし、Cloudflare Workers は対象外とする | Accepted |
| [0002](0002-modular-monolith-hexagonal.md) | モジュラーモノリス + ヘキサゴナルアーキテクチャ | Accepted |
| [0003](0003-database-abstraction.md) | RDB の抽象化と対応方言 | Accepted |
| [0004](0004-job-queue-abstraction.md) | ジョブキューとスケジューラの抽象化 | Accepted |
| [0005](0005-blob-storage-and-media.md) | Blob ストレージとメディア処理 | Accepted |
| [0006](0006-federation-protocol-scope.md) | 連合プロトコルの対応範囲 | Accepted |
| [0007](0007-client-api.md) | Mastodon Client API とは互換させず、独自のクライアント API を提供する | Accepted |
| [0008](0008-web-and-mobile-clients.md) | Web クライアントとモバイルアプリ | Accepted |
| [0009](0009-internationalization.md) | 国際化（i18n）と地域化（l10n） | Accepted |
| [0010](0010-configuration-and-packaging.md) | 設定・パッケージング・デプロイテンプレート | Accepted |
| [0011](0011-license.md) | ライセンスは AGPL-3.0 とする | Accepted |
| [0012](0012-v0-breaking-change-policy.md) | v0 期間中は破壊的変更を全面的に許容する | Accepted |
| [0013](0013-timeline-strategy.md) | タイムラインは fan-out-on-read を既定とする | Accepted |
| [0014](0014-task-management-and-agent-workflow.md) | タスク管理とコーディングエージェントの作業フロー | Accepted |
| [0015](0015-testing-strategy.md) | テスト戦略 | Accepted |
| [0016](0016-client-api-style.md) | クライアント API の様式 | Accepted |
| [0017](0017-visibility-local.md) | 公開範囲 `local`（サーバーローカル）を追加する | Accepted |
| [0018](0018-federated-uris.md) | 連合 URI は ID ベースで `/ap/` 名前空間に置き、表示用 URL と分離する | Accepted |
| [0019](0019-security-baseline.md) | セキュリティベースライン | Accepted |
| [0020](0020-data-modeling-principles.md) | データモデリング方針（リソース・イベント・投影の分離と状態の直交分解） | Accepted |
| [0021](0021-documents-describe-present-state.md) | リポジトリ内の文書は現在の状態だけを記述し、履歴は git に任せる | Accepted |
| [0022](0022-spike-outputs-outside-repository.md) | 調査（spike）の報告と試作コードはリポジトリに置かない | Accepted |
| [0023](0023-declarative-schema-management.md) | DB スキーマは宣言的に管理し、Atlas（Community Edition）で適用する | Accepted |
| [0024](0024-boy-scout-rule.md) | ボーイスカウトルールを採用し、担当外の壊れた箇所も放置しない | Accepted |
<!-- adr-index:end -->
