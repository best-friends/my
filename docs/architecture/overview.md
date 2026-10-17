# アーキテクチャ概要（基礎設計）

> この文書はサーバーの基礎設計で、Accepted な ADR に従います。判断の根拠は各 ADR を参照してください。
> サーバーの実装言語は Go です（[ADR-0001](../adr/0001-server-language-and-runtime.md)）。

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
| `i18n` | ロケール解決・メッセージカタログ・整形 | （横断。`internal/i18n` の 1 パッケージで、`domain` / `app` の層を持たない） |

### 依存ルール

- `domain` は他のどの層にも依存しない（純粋関数・値オブジェクト中心）。
- モジュール間はアプリケーション層の公開 API（ユースケース / イベント）経由でのみ参照する。テーブルの直接参照は禁止。
- インターフェイス層（Client API / AP）はアプリケーション層のユースケースを呼ぶだけで、ドメインロジックを持たない。
- **公開範囲の判定（`canView(viewer, status)`）はドメイン層の 1 箇所に集約**（NFR-SEC-4、[公開範囲の閲覧判定](visibility.md)）。公開範囲は `public` / `local` / `unlisted` / `private` / `direct`（[ADR-0017](../adr/0017-visibility-local.md)）。
- import の方向は golangci-lint の depguard（`.golangci.yml`）で `make check` と CI が検査する。§6 の各ディレクトリについて:

  | パッケージ | import してよいもの |
  | --- | --- |
  | `internal/domain/<module>` | 標準ライブラリと自モジュールの `domain` だけ |
  | `internal/app/<module>` | 標準ライブラリ、自モジュールの `domain`、各モジュールの `app`（モジュール間の唯一の経路）、`internal/i18n` |
  | `internal/app/kernel` | 標準ライブラリだけ（全モジュール共通のポート） |
  | `internal/app/uow` | 標準ライブラリだけ（`UnitOfWork` ポート・リトライ・同期 projector の登録先） |
  | `internal/app/blob`, `internal/app/media` | 標準ライブラリだけ（`BlobStore` / `MediaProcessor` ポート） |
  | `internal/app/stream` | 標準ライブラリだけ（リアルタイム更新の `Publisher` / `Subscriber` ポートとプロセス内ブローカー） |
  | `internal/i18n` | 標準ライブラリ、`locales`（埋め込みカタログ）、MessageFormat エンジンだけ（`app` と `interface` から使う末端のライブラリ） |
  | `internal/interface/*` | `app` と `domain`（`adapter` / `platform` は不可。`pkg/testkit` はテストからだけ可） |
  | `internal/adapter/*` | `app` のポートと `domain`（`interface` と `platform` は不可） |
  | `pkg/testkit` | `app` と `domain`（`interface` / `adapter` / `platform` は不可） |
  | `internal/platform`, `cmd/*` | すべて（composition root）。`cmd/` はどこからも import しない |

  `domain` と `app` で外部ライブラリを使うときは、`.golangci.yml` の allow に明示的に追加する。

## 3. ポート一覧

各ポートは、それを使うモジュールの `internal/app/<module>` に Go のインターフェイスとして定義する（§6）。
全モジュールが使う `Clock` / `IdGenerator`（Go 名 `IDGenerator`）と `KeyManager` は `internal/app/kernel` に置き、ドメインには `time.Time` と ID 文字列を値で渡す。
`UnitOfWork` も全モジュールと全 DB アダプタが共有するので `internal/app/uow` に置く。リポジトリポートは各モジュールの `app` に置き、
DB アダプタの実装は受け取った `uow.Tx` から自分のトランザクションを取り出す。
ジョブキューのポート（`JobQueue` / `Outbox` / `JobHandlerRegistry` / `JobRunner`）も全モジュールと全キューアダプタが共有するので `internal/app/queue` に置く。
ここにはリトライ方針（指数バックオフ + ジッタ）と、キューアダプタの `queue.Source` を駆動する pull runner、プロセス内の `Scheduler` も置く。
`Scheduler` は定期ジョブを周期ごとの冪等キー付きで enqueue するだけで、worker と all のロールで動く。
ファイルを持つモジュールとそのアダプタが共有する `BlobStore` は `internal/app/blob`（Go 名 `blob.Store`）、`MediaProcessor` は `internal/app/media`（Go 名 `media.Processor`）に置く。
リアルタイム更新（§4.4）の `Publisher` / `Subscriber` は、通知する側のモジュールとストリーミングのエンドポイントが共有するので `internal/app/stream` に置く。
Blob のキーはサーバーが生成し、`blob.ValidateKey` の構文（`[A-Za-z0-9._-]` のセグメントを `/` でつなぐ）に限る。`media/` で始まるキーだけが公開で、`/media/` で配信する。

| ポート | 役割 | 既定アダプタ（self-hosted） | 関連 ADR |
| --- | --- | --- | --- |
| `Repositories` / `UnitOfWork` | 永続化。書き込みは **短く再実行可能なトランザクション** 単位 | sqlite / postgres | [0003](../adr/0003-database-abstraction.md) |
| `JobQueue` | ジョブ投入（遅延・冪等キー付き） | db | [0004](../adr/0004-job-queue-abstraction.md) |
| `JobRunner` | ジョブ実行（pull ループ or push 受信） | db（pull） | [0004](../adr/0004-job-queue-abstraction.md) |
| `Scheduler` | 定期実行のトリガー | in-process cron | [0004](../adr/0004-job-queue-abstraction.md) |
| `BlobStore` | メディアの保存・署名付き URL | fs | [0005](../adr/0005-blob-storage-and-media.md) |
| `MediaProcessor` | サムネイル・リサイズ・EXIF 除去 | native | [0005](../adr/0005-blob-storage-and-media.md) |
| `KeyValueStore` | キャッシュ・レート制限・連合ライブラリ用 KV | db | [0003](../adr/0003-database-abstraction.md) |
| `HTTPClient` | 外部 HTTP（SSRF ガード・用途別の本文上限とタイムアウト・UA 付与） | httpclient（`net/http` + SSRF ガード） | [0006](../adr/0006-federation-protocol-scope.md), [0019](../adr/0019-security-baseline.md) |
| `Mailer` | メール送信 | none（ログ出力） | [0010](../adr/0010-configuration-and-packaging.md) |
| `PushSender` | Web Push 送信 | webpush | — |
| `Publisher` / `Subscriber` | リアルタイム更新の通知と購読（§4.4） | in-process（`stream.Hub`） | [0016](../adr/0016-client-api-style.md) |
| `Clock` / `IdGenerator` | 時刻（UTC・ミリ秒）・ID 生成（テスト容易性のため注入） | system（システム時計 / `uuid.NewV7()`） | [0003](../adr/0003-database-abstraction.md) |
| `KeyManager` | 秘密（アクター秘密鍵など）を包む DEK の wrap / unwrap。暗号化は `kernel.Sealer` が行う | local（設定の KEK） | [0019](../adr/0019-security-baseline.md) |
| `SearchIndex` | アカウント・タグ検索（将来全文） | db（FTS） | [0009](../adr/0009-internationalization.md) |
| `Logger` / `Tracer` | 観測性 | JSON stdout / OTel 任意 | — |

## 4. 主要フロー

### 4.1 ローカル投稿 → 連合配送

連合の受信・配送・取得の詳細は [連合の設計](federation.md) にあります。

1. Client API の投稿作成 → `PostStatus` ユースケース
2. ドメインで検証（文字数・公開範囲・メンション解決）→ `Status` を生成
3. **1 回のトランザクション**（`UnitOfWork.Tx`）で `statuses` + 付随行 + `outbox_events`（配送予定）を書き込む（Transactional Outbox）
4. `JobQueue.enqueue(FanOutDelivery{statusId})`（失敗しても outbox から再投入できる）
5. ワーカー: 公開範囲に応じて宛先 inbox を算出し（`local` はリモートフォロワーのみ）、**shared inbox 単位** に `DeliverActivity{inbox, activityId}` を投入
6. `DeliverActivity` が署名付き POST。失敗時はバックオフで再投入、ドメインが連続失敗したら一時停止

### 4.2 Inbox 受信

1. `POST /ap/inbox`（shared inbox）または `/ap/actors/{uuid}/inbox`（[ADR-0018](../adr/0018-federated-uris.md)）
2. 同期処理: サイズ制限 → 署名検証（鍵は KV / DB キャッシュ、未取得なら取得）→ ドメインブロック判定 → `202 Accepted`
3. `JobQueue.enqueue(ProcessInbox{activity, verifiedActor})`
4. ワーカー: Activity 種別ごとのハンドラ（冪等: Activity ID で重複排除）→ 通知生成など

### 4.3 タイムライン取得

- v0 は **fan-out-on-read**（読み取り時にフォロー関係と JOIN してページング）を既定とする。
  Redis 等の常駐ミドルウェアを不要にするため。規模が大きくなったら `home_feed` テーブルへの fan-out-on-write に切り替えられるよう、
  `TimelineReader` を内部インターフェイスにしておく。→ [ADR-0013](../adr/0013-timeline-strategy.md)

### 4.4 リアルタイム更新（SSE）

1. コマンドのトランザクションがコミットされると、`uow.Projectors` に登録したコミットフックが、そのトランザクションで発行したイベントを受け取る（[data-model §2.1](data-model.md#21-トランザクションの規律)）
2. `timeline` のフックは新しい投稿・ブーストの ID を、`notification` のフックは通知を受け取りうるアカウントを、`stream.Publisher` に知らせる。知らせるのは「何が変わったか」だけで、内容は載せない
3. `GET /api/v1/streaming` の各接続は `stream.Subscriber` で自分のアカウントを購読し、知らせを受けるたびにユースケースで読み直す。
   ホームに載るかは `timeline.Service.HomeEntry`（ホームのタイムラインと同じ条件と `CanView`）、通知は前回送ったものより新しい通知の一覧で決める
4. 結果を SSE のイベント（`update` は `Status`、`notification` は `Notification`）として送る。手順は [client-api.md §5](client-api.md#5-ストリーミング)

- 既定のブローカー `stream.Hub` は **1 つのプロセスの中だけ** で知らせを配る。web のプロセスが複数ある構成、web と worker を分けた構成（Inbox の処理で作られた通知は worker で生まれる）では、
  別のプロセスで起きた変更がストリームに届かない。複数プロセスでの配信（DB の LISTEN / NOTIFY や外部のブローカーで `Publisher` / `Subscriber` を実装する）は M4 で扱う（#196）
- 配信は best effort で、再送しない。クライアントは再接続したらホームと通知を読み直す
- ホームの新着は、接続中の利用者ごとに 1 回 `HomeEntry` を読む。読み取りの回数は「新しい投稿数 × 接続数」に比例する

## 5. データ設計の原則

テーブル・カラム・制約・インデックスの詳細は [論理データモデル](data-model.md) を参照してください。
モデリングの規約（リソース・イベント・投影の分離、状態の直交分解、正規化）は [ADR-0020](../adr/0020-data-modeling-principles.md) です。

- ID は時系列ソート可能な **UUIDv7**（文字列表現）。API ではそのまま文字列で返す。
- 時刻は UTC で保存、API では RFC 3339。
- 方言差を吸収するため、型は「共通部分集合」で設計（真偽値は整数、JSON は TEXT 保存も許容、等）。
- トランザクションは短く再実行可能にし、内部で外部 I/O をしない。重複実行への耐性は「一意制約 + upsert」「冪等キー」で担保（[ADR-0003](../adr/0003-database-abstraction.md)）。
- v0 期間はマイグレーション履歴を持たず、方言ごとの単一スキーマファイルから作り直す（[ADR-0012](../adr/0012-v0-breaking-change-policy.md)）。

## 6. リポジトリ構成

サーバーは Go（[ADR-0001](../adr/0001-server-language-and-runtime.md)）、構成は [ADR-0010](../adr/0010-configuration-and-packaging.md) に従う。

```text
cmd/
  my/                # 単一バイナリ: serve / worker / cli（Self-hosted, Cloud Run, コンテナ）
  my-lambda/         # AWS Lambda エントリ（Lambda Web Adapter で my を流用できるなら不要）
internal/
  domain/<module>/   # 純粋なドメイン（identity, account, content, graph, ...）
  app/<module>/      # ユースケースとポート定義
  app/kernel/        # 全モジュール共通のポート（Clock / IDGenerator / KeyManager / KeyValueStore / Mailer）、UUIDv7 ヘルパー、Sealer、RateLimiter
  app/uow/           # UnitOfWork ポート、トランザクションのリトライ、同期 projector の登録先
  app/queue/         # ジョブキューのポート、ハンドラの登録先、リトライ方針、pull runner、プロセス内 Scheduler
  app/blob/          # BlobStore ポートとキーの構文
  app/media/         # MediaProcessor ポート、形式の判定、上限の既定値
  app/stream/        # リアルタイム更新の Publisher / Subscriber ポートとプロセス内ブローカー Hub
  interface/
    clientapi/       # 独自クライアント API（api/openapi.yaml から生成したインターフェイスを実装）
    activitypub/     # inbox / outbox / actor / webfinger / nodeinfo
    media/           # 公開メディア（Blob のキー media/...）の配信 /media/
    authpages/       # ログイン・ログアウト・登録・メール確認の画面（html/template、JavaScript なし。ADR-0030）
    admin/
    web/             # Web クライアントの HTML の外枠（プロフィールと投稿のリンクプレビューを含む）・ビルド成果物（dist/ を埋め込む）・Web アプリマニフェスト（ADR-0028）
  adapter/
    db/{sqlite,libsql,postgres}/
    queue/{db,inline,sqs,cloudtasks}/
    blob/{fs,s3}/
    media/{native,vips,none}/
    kv/{db,memory}/
    mail/{none,smtp,ses}/
    system/          # Clock / IDGenerator の本番実装（システム時計・UUIDv7）
    keymanager/local/ # KeyManager の実装（設定の KEK。KMS アダプタは後続）
  platform/          # 設定、composition root、ログ、HTTP サーバー
  i18n/              # メッセージ整形・ロケール解決（型付きアクセサは tools/i18n が生成）
pkg/
  testkit/           # コントラクトテスト・フィクスチャ・決定的なフェイク（独自アダプタ作者向けに公開）
api/                 # openapi.yaml（クライアント API の正）と lint 設定 redocly.yaml。規約は client-api.md
schema/              # 目標スキーマ sqlite.sql / postgres.sql（Atlas で宣言的に適用）と準備 SQL postgres.bootstrap.sql（ADR-0023）
queries/             # sqlc 用クエリ（方言別。設定は sqlc.yaml、生成先は internal/adapter/db/<dialect>/。queue/ は internal/adapter/queue/db 用）
locales/             # ICU MessageFormat カタログ（en がソース。Go と TS で共有。en-XA / ar-XB は生成物）
tools/               # 開発ツール（Go ツールの go.mod、i18n の生成器・検査を持つ Bun パッケージ tools/i18n）
app/                 # クライアントの Bun ワークスペース（アプリと共有パッケージ。ADR-0026）
  ui/                # 共有 UI キット @my/ui（React Aria Components を包む部品、デザイントークン、Storybook）
  web/               # Web クライアント（PWA、TypeScript）
  mobile/            # モバイルアプリ（Tauri 2）
deploy/
  docker/ k8s/ gcp/ aws/
docs/
```
