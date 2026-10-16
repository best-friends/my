# ADR-0003: RDB の抽象化と対応方言

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: <!-- 起票後に追記 -->

## Context

運営者が「維持コストの安いもの、あるいは慣れたもの」から RDB を選べるようにしたい（G3）。
ただし方言を増やすほどテスト・保守コストが増える。また Cloudflare D1 は **対話的トランザクション（BEGIN〜COMMIT の間にアプリのロジックを挟む）をサポートせず**、
アトミックなのは `batch()` のみ（ステートメント数に上限あり、要検証）。Aurora DSQL も楽観的同時実行制御と機能制約がある。

## Decision Drivers

- 無料枠 / 低価格で使える選択肢をカバーする（SQLite 系、D1、Turso、Neon、Supabase）
- 慣れた選択肢（PostgreSQL、将来 MySQL）
- 方言数とテストコストのバランス
- D1 等の制約下でも正しく動く整合性モデル

## Considered Options

- 方言: (a) PostgreSQL のみ / (b) **SQLite 系 + PostgreSQL 系** / (c) (b) + MySQL 系
- 抽象化: (i) ORM の方言吸収に任せる / (ii) **リポジトリポート + 方言別アダプタ + 共通コントラクトテスト** / (iii) 生 SQL を方言別に手書き

## Decision（推奨案）

### 対応方言（v0）

| 方言ファミリ | 具体的な実体 | アダプタ |
| --- | --- | --- |
| SQLite 系 | ローカル SQLite ファイル、libSQL / Turso、Cloudflare D1 | `db-sqlite`, `db-libsql`, `db-d1` |
| PostgreSQL 系 | セルフホスト PG、Neon、Supabase、Cloud SQL、RDS / Aurora、（検証次第で Aurora DSQL） | `db-postgres` |

MySQL 系（MySQL、TiDB Serverless、PlanetScale 等）は v0 では対象外とし、v1 以降にコミュニティアダプタとして受け入れられる設計にする。

### 抽象化

- アプリケーション層は **集約単位のリポジトリポート** と **`UnitOfWork`** にのみ依存する。
- `UnitOfWork` は「読み取りを先に行い、書き込みを **1 つのアトミックバッチ** として commit する」モデルとする。
  対話的トランザクション（読み取り結果に応じて同一トランザクション内で書き分ける）を **前提にしない**。
  - 競合は一意制約 + `INSERT ... ON CONFLICT`（upsert）、条件付き更新（`UPDATE ... WHERE version = ?`）、冪等キーで解決する。
  - カウンタ（フォロワー数・お気に入り数など）は加算 SQL（`SET n = n + 1`）を使い、定期ジョブで再集計して補正する。
  - バッチのステートメント数上限を超える一括処理はジョブに分割する。
- クエリビルダはランタイム非依存かつ複数方言対応のもの（TS: Kysely を第一候補、Drizzle を比較候補 / Go: sqlc を方言別に生成 or bob）。
  方言差（FTS、JSON 関数、日付関数）はアダプタ内に閉じ込める。
- スキーマは方言ごとに `schema/<dialect>.sql` を持つ。共通の論理スキーマ（テーブル・カラム・制約の一覧）を 1 つのドキュメント / 定義から管理し、
  方言間の乖離を CI で検査する。
- **コントラクトテスト**: リポジトリポートごとに単一のテストスイートを書き、SQLite（インメモリ）・PostgreSQL（コンテナ）・D1（ローカルエミュレータ）で実行する。

### 型・ID・時刻の共通規約

- ID: UUIDv7 を文字列（36 文字）で保存。時系列ソート可能で、Mastodon API のページング（`max_id` / `since_id` / `min_id`）に使える。
- 時刻: UTC。保存はミリ秒精度の整数 Unix 時刻 または ISO 8601 文字列のどちらかに統一（Kysely 等での扱いやすさで M1 で確定）。
- 真偽値: 整数 0/1。JSON: TEXT（PG では `jsonb` を使ってもよいが、アプリからは文字列として扱う）。
- 文字列比較・照合順序に依存したロジックを書かない（ユーザー名は保存時に正規化した小文字カラムを別に持つ）。

### KeyValueStore ポート

連合ライブラリのキャッシュ（公開鍵・リモート文書）、レート制限、短命トークン用に KV ポートを用意する。
既定は RDB 上のテーブル実装（`kv-db`）。任意で Workers KV / Redis / メモリ。

## Consequences

- 運営者は SQLite（$0）から始め、規模に応じて PostgreSQL に移行できる（v0 期間はデータ移行ツールは提供しない）。
- D1 / DSQL の制約に合わせたことで、全方言で同じ整合性モデルになりテストが単純化する。一方、対話的トランザクションで楽に書ける処理も
  冪等・upsert で書く必要があり、実装の注意点が増える（AGENTS.md に明記する）。
- 方言 2 ファミリ × コントラクトテストの CI 時間が増える。

## Open Questions / 要検証

- [ ] Spike: Kysely（または Drizzle）で SQLite / D1 / PG を同一リポジトリ実装から扱えるか、D1 の `batch()` を UnitOfWork に写像できるか
- [ ] D1 のバッチ上限・1 クエリの行数制限・無料枠の rows read 上限と、タイムライン取得のコスト試算
- [ ] Aurora DSQL のコントラクトテスト通過可否（外部キー・シーケンス・DDL 制約）
- [ ] 時刻の保存形式の確定
