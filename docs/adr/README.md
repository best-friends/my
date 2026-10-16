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
| [0002](0002-modular-monolith-hexagonal.md) | モジュラーモノリス + ヘキサゴナルアーキテクチャ | Proposed |
| [0003](0003-database-abstraction.md) | RDB の抽象化と対応方言 | Proposed |
| [0004](0004-job-queue-abstraction.md) | ジョブキューとスケジューラの抽象化 | Proposed |
| [0005](0005-blob-storage-and-media.md) | Blob ストレージとメディア処理 | Proposed |
| [0006](0006-federation-protocol-scope.md) | 連合プロトコルの対応範囲 | Proposed |
| [0007](0007-client-api.md) | クライアント API は Mastodon 互換を主とする | Proposed |
| [0008](0008-web-and-mobile-clients.md) | Web クライアントとモバイルアプリ | Proposed |
| [0009](0009-internationalization.md) | 国際化（i18n）と地域化（l10n） | Proposed |
| [0010](0010-configuration-and-packaging.md) | 設定・パッケージング・デプロイテンプレート | Proposed |
| [0011](0011-license.md) | ライセンスは AGPL-3.0 とする | Accepted |
| [0012](0012-v0-breaking-change-policy.md) | v0 期間中は破壊的変更を全面的に許容する | Accepted |
| [0013](0013-timeline-strategy.md) | タイムラインは fan-out-on-read を既定とする | Proposed |
| [0014](0014-task-management-and-agent-workflow.md) | タスク管理とコーディングエージェントの作業フロー | Accepted |
| [0015](0015-testing-strategy.md) | テスト戦略 | Proposed |
<!-- adr-index:end -->
