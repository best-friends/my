# ADR-0003: RDB の抽象化と対応方言

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #3

## Context

運営者が「維持コストの安いもの、あるいは慣れたもの」から RDB を選べるようにしたい（G3）。
ただし方言を増やすほどテスト・保守コストが増える。

当初は Cloudflare D1（対話的トランザクション非対応）を対象に含めていたが、ADR-0001 で Cloudflare Workers を対象外としたため D1 も対象外になった。
残る候補のうち Aurora DSQL は楽観的同時実行制御（コミット時の競合で SQLSTATE 40001）、分離レベル Repeatable Read 固定、トランザクションあたり 3,000 行・10 MiB・5 分の上限、DDL と DML の分離を持つ。外部キーとシーケンス（`CACHE` は 1 か 65536 以上）はサポートされている（2026-10-16 に AWS 公式ドキュメントで確認、#18。詳細と出典は [deployment-targets.md](../architecture/deployment-targets.md)）。

## Decision Drivers

- 無料枠 / 低価格で使える選択肢をカバーする（SQLite、Turso、Neon、Supabase）
- 慣れた選択肢（PostgreSQL、将来 MySQL）
- 方言数とテストコストのバランス
- at-least-once のジョブ実行（ADR-0004）と両立する整合性モデル
- `CGO_ENABLED=0` の静的ビルドを維持できること（ADR-0001）

## Considered Options

- 方言: (a) PostgreSQL のみ / (b) **SQLite 系 + PostgreSQL 系** / (c) (b) + MySQL 系
- 抽象化: (i) ORM の方言吸収に任せる / (ii) **リポジトリポート + 方言別アダプタ + 共通コントラクトテスト** / (iii) 生 SQL を方言別に手書き
- トランザクション: (α) 書き込みはアトミックバッチのみ（D1 前提の旧案）/ (β) **短い対話的トランザクションを許容**

## Decision（推奨案）

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
- SQL の扱い（Go）: 第一候補は **sqlc を方言ごとに生成**（`queries/sqlite/*.sql`, `queries/postgres/*.sql`）し、
  方言別アダプタがリポジトリポートを実装する。比較候補は bob、`database/sql` + 手書き。spike で決める。
  方言差（FTS、JSON 関数、日付関数、プレースホルダ）はアダプタ内に閉じ込める。
- ドライバ候補（要検証）: SQLite は cgo 不要のもの（`modernc.org/sqlite` / `ncruces/go-sqlite3`）、PostgreSQL は `pgx`、libSQL / Turso は公式 Go クライアント。
- スキーマは方言ごとに `schema/<dialect>.sql` を持つ。共通の論理スキーマ（`docs/architecture/data-model.md`）から乖離していないかを CI で検査する。
- **コントラクトテスト**: リポジトリポートごとに単一のテストスイートを書き、SQLite（インメモリ / 一時ファイル）・PostgreSQL（コンテナ）で毎 PR 実行する。
  libSQL / DSQL など実体が必要なものは nightly。

### 型・ID・時刻の共通規約

- ID: UUIDv7 を文字列（36 文字）で保存。時系列ソート可能で、クライアント API のキーセットページングに使える。
- 時刻: UTC。保存形式（ミリ秒精度の整数 Unix 時刻 / ISO 8601 文字列）は spike で確定。
- 真偽値: 整数 0/1（SQLite）/ `boolean`（PG）をアダプタで吸収。JSON: SQLite は TEXT、PG は `jsonb`。
- 文字列比較・照合順序に依存したロジックを書かない（ユーザー名は保存時に正規化した小文字カラムを別に持つ）。

### KeyValueStore ポート

公開鍵・リモート文書のキャッシュ、レート制限、短命トークン用に KV ポートを用意する。
既定は RDB 上のテーブル実装（`kv-db`）。任意でメモリ / Redis。

## Consequences

- 運営者は SQLite（$0）から始め、規模に応じて PostgreSQL に移行できる（v0 期間はデータ移行ツールは提供しない）。
- 対話的トランザクションを使えるため、アトミックバッチ限定の旧案より実装が素直になる。一方、再実行可能なトランザクション関数と
  トランザクション外での外部 I/O という規律が必要（AGENTS.md に明記）。
- sqlc を方言別に生成する場合、クエリが方言の数だけ重複する。コントラクトテストで挙動の一致を担保する。
- 方言 2 ファミリ × コントラクトテストの CI 時間が増える。

## Open Questions / 要検証

- [x] #17 Spike: sqlc（比較: bob）で SQLite / PostgreSQL のリポジトリ実装と UnitOfWork を作り、同一コントラクトテストを通す。cgo 不要の SQLite ドライバを選ぶ
  → [報告](../spikes/17-sqlc-dialects.md)。同一コントラクトテストが SQLite（modernc / ncruces × ファイル / インメモリ）と PostgreSQL 18（SERIALIZABLE / READ COMMITTED）で通過。
  推奨は **sqlc（方言別生成）+ `pgx/v5` + `modernc.org/sqlite`**（`CGO_ENABLED=0` で可。ncruces も同等に動き、差はアダプタの接続部のみ）。
  SQLite は `BEGIN IMMEDIATE`（`_txlock=immediate`）+ `busy_timeout` + WAL が必須。PG は READ COMMITTED を既定とし、直列化失敗・デッドロックはリトライで吸収する案。
- [ ] 時刻の保存形式の確定 — spike の推奨案: SQLite は `INTEGER`（UTC Unix ミリ秒、`STRICT` テーブル）、PG は `timestamptz`、ドメインでは UTC・ミリ秒に丸めた `time.Time`。
  ドライバ既定の TEXT 化は形式がドライバごとに違い、文字列順が時刻順にならないことを確認した（[報告](../spikes/17-sqlc-dialects.md#時刻の保存形式)）。決定待ち
- [ ] UUIDv7 の生成は Go 1.27 標準ライブラリ `uuid.NewV7()` を使う案（単調増加を確認、依存追加なし）。PG の ID 列は `COLLATE "C"` または `uuid` 型にする（照合順序に依存させない）。決定待ち
- [ ] ncruces/go-sqlite3 を採る場合のライセンス検査: 依存 `go-sqlite3-wasm`（MIT-0）を go-licenses が識別できない。許可リストの扱いを決める
- [ ] 読み取り専用の経路（`BEGIN IMMEDIATE` を取らない読み取りトランザクション / 非トランザクション読み取り）の API
- [ ] libSQL / Turso の Go クライアントの成熟度（cgo 要否、リモート接続でのトランザクション）
- [x] #18 Aurora DSQL の制約の確認（2026-10-16）: 外部キー・シーケンスは可。トランザクション 3,000 行 / 10 MiB / 5 分、1 トランザクション 1 DDL・DDL と DML 分離、Repeatable Read 固定、一時テーブル・PL/pgSQL・`TRUNCATE` 不可
- [ ] Aurora DSQL のコントラクトテスト通過可否（上記制約下での実測。`SKIP LOCKED` の可否は公式ドキュメントで未確認のため要検証。ホットなカウンタ行の OCC 競合率）
