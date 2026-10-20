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
| [0025](0025-web-client-stack.md) | Web クライアントの技術スタック | Accepted |
| [0026](0026-client-workspace-and-component-architecture.md) | クライアントのワークスペース構成と、ロジックと UI の責務分離 | Accepted |
| [0027](0027-ui-quality-gates.md) | UI のアクセシビリティ・レスポンシブ・見た目の品質ゲート | Accepted |
| [0028](0028-html-shell-link-previews-and-pwa.md) | Go が HTML の外枠を返し、リンクプレビュー（OGP / X Card）と PWA を両立する | Accepted |
| [0029](0029-api-code-generation.md) | クライアント API のコード生成 | Accepted |
| [0030](0030-web-client-token-handling.md) | Web クライアントのトークンの扱いとログイン画面 | Accepted |
| [0031](0031-ui-kit-behavior-dependencies.md) | 共有 UI キットは、ドメインを持たない振る舞いのライブラリにも依存してよい | Accepted |
| [0032](0032-http-signature-profile.md) | HTTP 署名の受信・送信プロファイル | Accepted |
| [0033](0033-inbox-processing-model.md) | Inbox の受信処理モデル（同期・非同期の境界、重複排除、順序） | Accepted |
| [0034](0034-remote-content-scope-and-retention.md) | リモートコンテンツの取り込み範囲・リフレッシュ・保持 | Accepted |
| [0035](0035-local-visibility-mention-tags.md) | 公開範囲 local の投稿は、フォロワーでないリモートのメンション先を tag に入れない | Accepted |
| [0036](0036-moderation-federation-propagation.md) | モデレーションの処分を連合先へ伝える方法 | Accepted |
| [0037](0037-audit-log.md) | 監査ログを処分のイベントとは別の追記専用の記録にする | Accepted |
| [0038](0038-moderation-personal-data-retention.md) | モデレーションに関わる個人データの保持期間と開示の範囲 | Accepted |
| [0039](0039-mobile-navigation-and-compose-entry.md) | モバイルのナビゲーションの項目と投稿の入口 | Accepted |
| [0040](0040-e2e-on-main-only.md) | Web の E2E を main への push でだけ回す | Accepted |
| [0041](0041-postgres-tests-on-pgmem.md) | PostgreSQL のテストを pgmem で動かす | Accepted |
| [0042](0042-captcha-human-verifier.md) | サインインと登録のボット対策に CAPTCHA を設定で使えるようにする | Accepted |
| [0043](0043-icon-set.md) | UI キットのアイコンを既存のアイコンセット（Phosphor）から使う | Accepted |
| [0044](0044-remote-profile-recent-posts.md) | リモートのプロフィールを開いたときに最近の投稿を取得する | Accepted |
| [0045](0045-client-server-cache-policy.md) | キャッシュの方針（クライアントとサーバー） | Accepted |
| [0046](0046-dependabot-vulnerability-detection.md) | 依存関係の脆弱性検知を Dependabot に移す | Accepted |
| [0047](0047-ci-protection-layers.md) | CI の検査を PR・Merge Queue・main・nightly に振り分け、文脈ごとのワークフローに分ける | Accepted |
| [0048](0048-typescript-7-with-typescript-6-api.md) | 型検査を TypeScript 7 にし、コンパイラ API を使うツールには TypeScript 6 を並べる | Accepted |
| [0049](0049-client-lint-oxlint.md) | クライアントの lint を ESLint から Oxlint に移す | Accepted |
| [0050](0050-stream-pubsub-across-processes.md) | リアルタイム更新のプロセス間 Pub/Sub | Accepted |
| [0052](0052-emoji-reactions.md) | 絵文字リアクション（Misskey / Pleroma / Akkoma 互換） | Accepted |
| [0054](0054-media-variants-video-audio.md) | メディアの派生と動画・音声の扱い | Accepted |
| [0056](0056-system-observability.md) | システムメトリクスの可観測性 | Accepted |
| [0060](0060-instance-webhooks.md) | サーバー単位の webhook | Accepted |
| [0062](0062-link-preview-cards.md) | 投稿中のリンクのカード | Accepted |
<!-- adr-index:end -->
