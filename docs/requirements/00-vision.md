# ビジョンとスコープ

## 一文で

**月額ほぼ $0〜$5 から運営できる、ActivityPub 互換で多言語対応のマイクロブログサーバー。
ホスティング先とバックエンドを自由に選べる OSS。**

## 背景・課題

- 既存の Fediverse サーバー（Mastodon など）は Ruby + PostgreSQL + Redis + Sidekiq + Elasticsearch などの構成が前提です。
  小規模インスタンスでも VM 1 台を常時動かす必要があり、月額 $20〜 かかることが珍しくありません。
- 運営コストの大部分は **常時起動のコンピュート**、**マネージド RDB の最低料金**、**リモートメディアのキャッシュ（ストレージ・転送量）** で決まります。
- サーバーレス（scale-to-zero）や無料枠の大きい DB（SQLite / Turso / Neon 等）を前提にした設計なら、小規模運営のコストを一桁下げられる見込みです。

## 目標（Goals）

| ID | 目標 |
| --- | --- |
| G1 | 低コスト: おひとりさま〜数十人規模なら、小さな VM 1 台（$4〜6/月）か、サーバーレス（Cloud Run / Lambda）の無料枠でほぼ収まる |
| G2 | 可搬性: Google Cloud / AWS / Self-hosted（Docker, microk8s）の 3 系統で同じコードベースが動く（Cloudflare Workers は対象外。[ADR-0001](../adr/0001-server-language-and-runtime.md)） |
| G3 | 差し替え可能性: RDB・キュー・ストレージ・メール等を設定で切り替えられ、利用者が独自アダプタを追加できる |
| G4 | 相互運用性: Mastodon / Misskey / GoToSocial / Pleroma(Akkoma) と問題なく連合できる |
| G5 | 多言語: UI・通知・投稿メタデータを含め、最初期から i18n / l10n を前提にする |
| G6 | マルチクライアント: Web(PWA) と、モバイルアプリ（Tauri 2 もしくはネイティブ）を提供する。クライアント API は独自（Mastodon クライアント互換はしない） |
| G7 | エージェント駆動開発: コーディングエージェントが Issue 単位で自律的に作業し、`make check` 一発で検証できる |

## 非目標（Non-goals, v0）

- 大規模インスタンス（数万ユーザー以上）向けの最適化
- ActivityPub C2S（Client-to-Server）プロトコル
- 全文検索（連合先コンテンツ全体）、リレー、グループ、アカウント移行（`Move`）
- E2EE DM
- 後方互換性の維持（v0 の期間中は DB 作り直しを含む破壊的変更を許容。[ADR-0012](../adr/0012-v0-breaking-change-policy.md)）

## 想定ユーザー（Personas）

| ペルソナ | 説明 | 主な関心 |
| --- | --- | --- |
| 運営者 (Operator) | インスタンスを立てて運営する人。おひとりさま〜コミュニティ運営者 | コスト、構築の容易さ、バックアップ、モデレーション |
| 利用者 (User) | 投稿・閲覧・フォローする人。多様な言語・地域 | 使いやすさ、母語 UI、プライバシー、モバイル |
| モデレーター | 通報対応・ドメインブロック | 管理画面、監査ログ |
| リモートサーバー | 連合先の Fediverse 実装 | 仕様準拠、署名、配送の安定性 |
| カスタマイザ / コントリビュータ | フォークや独自アダプタを書く開発者 | 明確な境界（ポート）、ドキュメント、テスト |
| コーディングエージェント | Issue を拾って実装する AI | 明確な受け入れ基準、単一コマンドの検証、ADR による決定事項 |

## マイルストーン概要

詳細は GitHub Milestones を正とします。

| Milestone | 内容 | 完了条件 |
| --- | --- | --- |
| **M0: Design & Harness** | 要件定義・基礎設計・ADR 確定・開発ハーネス整備 | 主要 ADR が Accepted、言語ツールチェーンと CI が動く |
| **M1: Walking Skeleton** | 1 プロセスで起動し、WebFinger / Actor を返し、SQLite に永続化。全レイヤを細く貫通 | Self-hosted で起動、Mastodon から検索・表示できる |
| **M2: Local Microblog** | アカウント・投稿・タイムライン・フォロー（ローカルのみ）・独自クライアント API・Web クライアント（PWA） | ローカルで SNS として使える |
| **M3: Federation** | 連合（Follow / Create / Announce / Like / Delete …）、配送キュー、リトライ | Mastodon / Misskey / GoToSocial と相互フォロー・投稿が往復 |
| **M4: Multi-target** | GCP / AWS / microk8s のデプロイテンプレートと各アダプタ | 3 系統すべてで E2E スモークテストが通る |
| **M5: v0 Release** | モデレーション最小機能、i18n（en / ja + α）、ドキュメント、Tauri 2 モバイルの検証版 | v0.x として公開し実運用を開始できる |
