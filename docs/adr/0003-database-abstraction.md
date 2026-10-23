# ADR-0003: RDB の抽象化と対応方言

- Status: Accepted（PostgreSQL のコントラクトテストの頻度を ADR-0041（PostgreSQL のテストを pgmem で動かす） が、PostgreSQL での ID の型を ADR-0078（PostgreSQL の ID の列を uuid 型にする） が置き換える）
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #3

## Context

運営者が「維持コストの安いもの、あるいは慣れたもの」から RDB を選べるようにしたい（G3）。
ただし方言を増やすほどテスト・保守コストが増える。

Cloudflare D1 は ADR-0001 により対象外である。候補のうち Aurora DSQL は楽観的同時実行制御（コミット時の競合で SQLSTATE 40001）、分離レベル Repeatable Read 固定、トランザクションあたり 3,000 行・10 MiB・5 分の上限、DDL と DML の分離を持つ。外部キーとシーケンス（`CACHE` は 1 か 65536 以上）はサポートされている（2026-10-16 時点の AWS 公式ドキュメント。出典は [deployment-targets.md](../architecture/deployment-targets.md)）。

## Decision Drivers

- 無料枠 / 低価格で使える選択肢をカバーする（SQLite、Turso、Neon、Supabase）
- 慣れた選択肢（PostgreSQL、将来 MySQL）
- 方言数とテストコストのバランス
- at-least-once のジョブ実行（ADR-0004）と両立する整合性モデル
- `CGO_ENABLED=0` の静的ビルドを維持できること（ADR-0001）

## Considered Options

- 方言: (a) PostgreSQL のみ / (b) **SQLite 系 + PostgreSQL 系** / (c) (b) + MySQL 系
- 抽象化: (i) ORM の方言吸収に任せる / (ii) **リポジトリポート + 方言別アダプタ + 共通コントラクトテスト** / (iii) 生 SQL を方言別に手書き
- トランザクション: (α) 書き込みはアトミックバッチのみ（対話的トランザクションを使わない）/ (β) **短い対話的トランザクションを許容**

## Decision

### 対応方言（v0）

| 方言ファミリ | 具体的な実体 | アダプタ |
| --- | --- | --- |
| SQLite 系 | ローカル SQLite ファイル、libSQL / Turso | `sqlite`, `libsql` |
| PostgreSQL 系 | セルフホスト PG、Neon、Supabase、Cloud SQL、RDS / Aurora、（検証次第で Aurora DSQL） | `postgres` |

MySQL 系（MySQL、TiDB Serverless、PlanetScale 等）は v0 では対象外とし、v1 以降にコミュニティアダプタとして受け入れられる設計にする。

### 抽象化

- アプリケーション層は **集約単位のリポジトリポート** と **`UnitOfWork`（`Tx(ctx, func(tx) error) error`）** にのみ依存する。
- トランザクションは β（短い対話的トランザクションを許容）とする。ただし:
  - **トランザクション内で外部 I/O（HTTP・キュー投入・Blob 操作）をしない**。キュー投入は Transactional Outbox 経由（ADR-0004）。
  - 直列化失敗・競合エラー（SQLSTATE 40001）は `UnitOfWork` が有限回リトライする（DSQL の OCC 競合 / PG の serializable 対策）。そのためトランザクション関数は再実行可能に書く。
  - 1 トランザクションで変更する行数は DSQL の上限（3,000 行）を超えないようにし、一括処理はジョブに分割する。
  - ジョブは at-least-once なので、書き込みは引き続き一意制約 + upsert（`INSERT ... ON CONFLICT`）・冪等キーで重複に強くする。
  - カウンタ（フォロワー数・お気に入り数など）は加算 SQL（`SET n = n + 1`）を使い、定期ジョブで再集計して補正する。
- SQL の扱い（Go）: **sqlc を方言ごとに生成**（`queries/sqlite/*.sql`, `queries/postgres/*.sql`）し、方言別アダプタがリポジトリポートを実装する。
  方言差（FTS、JSON 関数、日付関数、プレースホルダ）はアダプタ内に閉じ込める。根拠は [#17 の調査報告](https://github.com/mycroblog/my/issues/17#issuecomment-6209364365)。
- ドライバ: SQLite は `modernc.org/sqlite`（cgo 不要）、PostgreSQL は `pgx/v5`、libSQL / Turso は公式 Go クライアント（成熟度は Open Questions）。
- トランザクションの設定:
  - SQLite は `BEGIN IMMEDIATE`（`_txlock=immediate`）+ `busy_timeout` + WAL を必須とする。遅延ロックのままだと書き込みの競合でリトライが多発する。
  - PostgreSQL は READ COMMITTED を既定とし、直列化失敗・デッドロックは `UnitOfWork` のリトライで吸収する。
- スキーマは方言ごとに `schema/<dialect>.sql` を持つ。共通の論理スキーマ（`docs/architecture/data-model.md`）から乖離していないかを CI で検査する。
- **コントラクトテスト**: リポジトリポートごとに単一のテストスイートを書き、SQLite（インメモリ / 一時ファイル）・PostgreSQL（コンテナ）で毎 PR 実行する。
  libSQL / DSQL など実体が必要なものは nightly。

### 型・ID・時刻の共通規約

- ID: UUIDv7 を文字列（36 文字）で保存し、Go 1.27 標準ライブラリの `uuid.NewV7()` で生成する。時系列ソート可能で、クライアント API のキーセットページングに使える。
  照合順序に依存させないため、PostgreSQL では `COLLATE "C"` の `text` か `uuid` 型にする。
- 時刻: UTC・ミリ秒精度。SQLite は `INTEGER`（Unix ミリ秒、`STRICT` テーブル）、PostgreSQL は `timestamptz`。ドライバ既定の文字列化は、文字列の順序が時刻の順序と一致しないので使わない。
- 真偽値: 整数 0/1（SQLite）/ `boolean`（PG）をアダプタで吸収。JSON: SQLite は TEXT、PG は `jsonb`。
- 文字列比較・照合順序に依存したロジックを書かない（ユーザー名は保存時に正規化した小文字カラムを別に持つ）。

### KeyValueStore ポート

公開鍵・リモート文書のキャッシュ、レート制限、短命トークン用に KV ポートを用意する。
既定は RDB 上のテーブル実装（`kv-db`）。任意でメモリ / Redis。

## Consequences

- 運営者は SQLite（$0）から始め、規模に応じて PostgreSQL に移行できる（v0 期間はデータ移行ツールは提供しない）。
- 対話的トランザクションを使えるため、アトミックバッチ限定（α）より実装が素直になる。一方、再実行可能なトランザクション関数と
  トランザクション外での外部 I/O という規律が必要（AGENTS.md に明記）。
- sqlc を方言別に生成する場合、クエリが方言の数だけ重複する。コントラクトテストで挙動の一致を担保する。
- 方言 2 ファミリ × コントラクトテストの CI 時間が増える。

## Open Questions / 要検証

- [ ] PostgreSQL の ID 列を `COLLATE "C"` の `text` と `uuid` 型のどちらにするか
- [ ] 読み取り専用の経路（`BEGIN IMMEDIATE` を取らない読み取りトランザクション / 非トランザクション読み取り）の API
- [ ] libSQL / Turso の Go クライアントの成熟度（cgo 要否、リモート接続でのトランザクション）
- [ ] Aurora DSQL でコントラクトテストが通るか（`SKIP LOCKED` の可否は公式ドキュメントに記載がない。ホットなカウンタ行での OCC 競合率）
