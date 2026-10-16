# アーキテクチャ概要（基礎設計）

> この文書は Proposed な ADR を前提にした **基礎設計のたたき台** です。ADR が確定したら追従して更新します。
> 実装言語に依存しない書き方をしており、言語別の対応は [ADR-0001](../adr/0001-server-language-and-runtime.md) を参照してください。

## 1. 全体像

```text
                ┌──────────────── Clients ────────────────┐
                │ Web (PWA) │ Mobile (Tauri 2) │ 3rd-party │
                │           │                  │           │
                └─────┬─────┴────────┬─────────┴─────┬─────┘
                      │  Client API (OpenAPI, 独自)    │
Remote servers ──AP──►│                               │
(Mastodon, Misskey…)  ▼                               ▼
        ┌──────────────────── Entrypoints (ターゲット別) ─────────────────────┐
        │ HTTP handler │ Queue consumer (pull / push) │ Scheduler (cron)       │
        └───────┬──────┴──────────────┬───────────────┴──────────┬────────────┘
                ▼                     ▼                          ▼
        ┌─────────────── Interface layer (inbound adapters) ──────────────────┐
        │ Client API │ ActivityPub (inbox/outbox/actor/webfinger) │ Admin       │
        └───────┬──────────────────────────────────────────────────────────────┘
                ▼
        ┌─────────────── Application layer (use cases) ───────────────────────┐
        │ PostStatus, Follow, ProcessInboxActivity, DeliverActivity, ...       │
        └───────┬──────────────────────────────────────────────────────────────┘
                ▼
        ┌─────────────── Domain layer (pure) ─────────────────────────────────┐
        │ Account, Status, Relationship, Visibility policy, Timeline rules ... │
        └──────────────────────────────────────────────────────────────────────┘
                ▲  ports (interfaces) — application 層が定義
        ┌───────┴─────────── Outbound adapters（設定で選択）────────────────────┐
        │ DB: sqlite │ libsql │ postgres                                        │
        │ Queue: db │ sqs │ cloud-tasks │ inline                                │
        │ Blob: fs │ s3-compatible(S3/R2/GCS/MinIO)                             │
        │ KV/Cache: db │ memory │ redis(任意)                                    │
        │ Mail: none │ smtp │ ses │ http-api   Push: webpush                    │
        │ Media: none │ native(pure Go) │ vips(cgo, 任意)                       │
        └──────────────────────────────────────────────────────────────────────┘
```

- **モジュラーモノリス + ヘキサゴナル（Ports & Adapters）**。[ADR-0002](../adr/0002-modular-monolith-hexagonal.md)
- 1 つのコードベースから、ターゲット別の **エントリポイント** が必要なアダプタだけを組み込んで起動する。
- Web とワーカーは同一成果物。役割（`web` / `worker` / `all`）は設定で切り替える。

## 2. モジュール（境界づけられたコンテキスト）

| モジュール | 責務 | 主な集約 / エンティティ |
| --- | --- | --- |
| `identity` | 利用者登録・認証・セッション・OAuth アプリ / トークン・2FA | User, Session, OAuthApp, AccessToken |
| `account` | ローカル / リモートのアカウント（Actor）とプロフィール | Account, ProfileField, ActorKey |
| `content` | 投稿・メディア・メンション・タグ・カスタム絵文字・投票 | Status, MediaAttachment, Tag, Emoji, Poll |
| `graph` | フォロー・フォローリクエスト・ブロック・ミュート | Follow, FollowRequest, Block, Mute |
| `timeline` | ホーム / ローカル / 連合 / タグ / プロフィールのタイムライン組み立て | （読み取りモデル） |
| `notification` | 通知の生成・取得・Web Push | Notification, PushSubscription |
| `federation` | AP の受信・配送・Actor/Object 取得、署名、ドメイン状態 | RemoteObjectCache, DeliveryAttempt, InstanceHealth |
| `moderation` | 通報・ドメインブロック・凍結・監査ログ | Report, DomainBlock, AuditLog |
| `instance` | インスタンス設定・規約・ルール・NodeInfo | InstanceSettings |
| `i18n` | ロケール解決・メッセージカタログ・整形 | （横断） |

### 依存ルール

- `domain` は他のどの層にも依存しない（純粋関数・値オブジェクト中心）。
- モジュール間はアプリケーション層の公開 API（ユースケース / イベント）経由でのみ参照する。テーブルの直接参照は禁止。
- インターフェイス層（Client API / AP）はアプリケーション層のユースケースを呼ぶだけで、ドメインロジックを持たない。
- **公開範囲の判定（`canView(viewer, status)`）はドメイン層の 1 箇所に集約**（NFR-SEC-4）。公開範囲は `public` / `local` / `unlisted` / `private` / `direct`（[ADR-0017](../adr/0017-visibility-local.md)）。

## 3. ポート一覧

詳細なシグネチャは実装開始時（M1）に `core` パッケージで定義し、ここからリンクします。

| ポート | 役割 | 既定アダプタ（self-hosted） | 関連 ADR |
| --- | --- | --- | --- |
| `Repositories` / `UnitOfWork` | 永続化。書き込みは **アトミックバッチ** 単位 | sqlite | [0003](../adr/0003-database-abstraction.md) |
| `JobQueue` | ジョブ投入（遅延・冪等キー付き） | db | [0004](../adr/0004-job-queue-abstraction.md) |
| `JobRunner` | ジョブ実行（pull ループ or push 受信） | db（pull） | [0004](../adr/0004-job-queue-abstraction.md) |
| `Scheduler` | 定期実行のトリガー | in-process cron | [0004](../adr/0004-job-queue-abstraction.md) |
| `BlobStore` | メディアの保存・署名付き URL | fs | [0005](../adr/0005-blob-storage-and-media.md) |
| `MediaProcessor` | サムネイル・リサイズ・EXIF 除去 | native | [0005](../adr/0005-blob-storage-and-media.md) |
| `KeyValueStore` | キャッシュ・レート制限・連合ライブラリ用 KV | db | [0003](../adr/0003-database-abstraction.md) |
| `HttpClient` | 外部 HTTP（SSRF ガード・タイムアウト・UA 付与） | 標準 fetch + guard | [0006](../adr/0006-federation-protocol-scope.md) |
| `Mailer` | メール送信 | none（ログ出力） | [0010](../adr/0010-configuration-and-packaging.md) |
| `PushSender` | Web Push 送信 | webpush | — |
| `Clock` / `IdGenerator` | 時刻・ID 生成（テスト容易性のため注入） | system / UUIDv7 | [0003](../adr/0003-database-abstraction.md) |
| `SearchIndex` | アカウント・タグ検索（将来全文） | db（FTS） | [0009](../adr/0009-internationalization.md) |
| `Logger` / `Tracer` | 観測性 | JSON stdout / OTel 任意 | — |

## 4. 主要フロー

### 4.1 ローカル投稿 → 連合配送

1. Client API の投稿作成 → `PostStatus` ユースケース
2. ドメインで検証（文字数・公開範囲・メンション解決）→ `Status` を生成
3. **1 回のアトミックバッチ** で `statuses` + 付随行 + `outbox_events`（配送予定）を書き込む（Transactional Outbox）
4. `JobQueue.enqueue(FanOutDelivery{statusId})`（失敗しても outbox から再投入できる）
5. ワーカー: 公開範囲に応じて宛先 inbox を算出し（`local` はリモートフォロワーのみ）、**shared inbox 単位** に `DeliverActivity{inbox, activityId}` を投入
6. `DeliverActivity` が署名付き POST。失敗時はバックオフで再投入、ドメインが連続失敗したら一時停止

### 4.2 Inbox 受信

1. `POST /inbox` or `/users/:id/inbox`
2. 同期処理: サイズ制限 → 署名検証（鍵は KV / DB キャッシュ、未取得なら取得）→ ドメインブロック判定 → `202 Accepted`
3. `JobQueue.enqueue(ProcessInbox{activity, verifiedActor})`
4. ワーカー: Activity 種別ごとのハンドラ（冪等: Activity ID で重複排除）→ 通知生成など

### 4.3 タイムライン取得

- v0 は **fan-out-on-read**（読み取り時にフォロー関係と JOIN してページング）を既定とする。
  Redis 等の常駐ミドルウェアを不要にするため。規模が大きくなったら `home_feed` テーブルへの fan-out-on-write に切り替えられるよう、
  `TimelineReader` を内部インターフェイスにしておく。→ [ADR-0013](../adr/0013-timeline-strategy.md)

## 5. データ設計の原則

テーブル・カラム・制約・インデックスの詳細は [論理データモデル](data-model.md) を参照してください。

- ID は時系列ソート可能な **UUIDv7**（文字列表現）。API ではそのまま文字列で返す。
- 時刻は UTC で保存、API では RFC 3339。
- 方言差を吸収するため、型は「共通部分集合」で設計（真偽値は整数、JSON は TEXT 保存も許容、等）。
- トランザクションは短く再実行可能にし、内部で外部 I/O をしない。重複実行への耐性は「一意制約 + upsert」「冪等キー」で担保（[ADR-0003](../adr/0003-database-abstraction.md)）。
- v0 期間はマイグレーション履歴を持たず、方言ごとの単一スキーマファイルから作り直す（[ADR-0012](../adr/0012-v0-breaking-change-policy.md)）。

## 6. リポジトリ構成

サーバーは Go（[ADR-0001](../adr/0001-server-language-and-runtime.md)）。詳細は [ADR-0010](../adr/0010-configuration-and-packaging.md) で確定させます。

```text
cmd/
  my/                # 単一バイナリ: serve / worker / cli（Self-hosted, Cloud Run, コンテナ）
  my-lambda/         # AWS Lambda エントリ（Lambda Web Adapter で my を流用できるなら不要）
internal/
  domain/<module>/   # 純粋なドメイン（identity, account, content, graph, ...）
  app/<module>/      # ユースケースとポート定義
  interface/
    clientapi/       # 独自クライアント API（api/openapi.yaml から生成したインターフェイスを実装）
    activitypub/     # inbox / outbox / actor / webfinger / nodeinfo
    admin/
  adapter/
    db/{sqlite,libsql,postgres}/
    queue/{db,inline,sqs,cloudtasks}/
    blob/{fs,s3}/
    media/{native,vips,none}/
    kv/{db,memory}/
    mail/{none,smtp,ses}/
  platform/          # 設定、composition root、ログ、HTTP サーバー
pkg/
  testkit/           # コントラクトテスト・フィクスチャ（独自アダプタ作者向けに公開）
api/                 # openapi.yaml（クライアント API の正）
schema/              # sqlite.sql, postgres.sql（v0 はマイグレーションなし）
queries/             # sqlc 用クエリ（方言別）
locales/             # ICU MessageFormat カタログ（en がソース。Go と TS で共有）
web/                 # Web クライアント（TypeScript + Bun）
mobile/              # Tauri 2
deploy/
  docker/ k8s/ gcp/ aws/
docs/
```
