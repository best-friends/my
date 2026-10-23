# ADR-0078: PostgreSQL の ID の列を uuid 型にする

- Status: Accepted
- Date: 2026-10-23
- Deciders: @rosylilly
- Issue: #1022

## Context（背景・課題）

ID は UUIDv7 をアプリで生成し、36 文字の小文字の正規形で保存している（[ADR-0003](0003-database-abstraction.md) の「型・ID・時刻の共通規約」）。
PostgreSQL では、照合順序に依らずバイト順で並ぶように `text COLLATE "C"` で持つ（`schema/postgres.sql` の 516 列、[data-model §1.2](../architecture/data-model.md#12-論理型)）。
ADR-0003 は `COLLATE "C"` の `text` と `uuid` 型のどちらでもよいとし、どちらにするかは data-model §11 の未決事項として残っている（spike #17）。

`uuid` 型なら 1 値が 37 バイト（可変長の頭の 1 バイトと 36 文字）から 16 バイトになり、表と索引が小さくなり、比較が速くなり、不正な値を DB が拒む。
型の変更は、実運用のデータが溜まった後だとデータの移し替えが要る。v0 は DB を作り直してよいが（[ADR-0012](0012-v0-breaking-change-policy.md)）、
実運用を始めた後のスキーマ変更の扱いは #1028 で見直している。そのため v0.x の公開の前に決める。表と索引の大きさは、従量課金の DB の保存量と読み取り量に効く（[vision](../requirements/00-vision.md) のほぼゼロ円の運用、[deployment-targets](../architecture/deployment-targets.md)）。

決定者の決定（#1043 のレビュー、原文）:

> ADR Accepted

## Decision Drivers（判断基準）

1. 外へ出す ID の形を変えないこと。client API・連合の URI（[ADR-0018](0018-federated-uris.md)）・表示用 URL は 36 文字の小文字の正規形のまま
2. 並び順と読み取り窓の意味を変えないこと（`ORDER BY id`、キーセットページング（ADR-0016）、`kernel.UUIDv7Floor` の下限、data-model §1.3）
3. 従量課金の DB での表と索引の大きさ・読み取り量（ほぼゼロ円の運用、[deployment-targets](../architecture/deployment-targets.md)）
4. 方言の差をアダプタに閉じ込め、アプリケーション層とドメインを変えないこと（ADR-0002、ADR-0003）
5. M6 の対象（Aurora DSQL、Neon、Supabase、Cloud SQL、RDS）と、テストの pgmem（[ADR-0041](0041-postgres-tests-on-pgmem.md)）で同じに動くこと
6. 実装の手間と、既存のクエリ・テストの書き換えの量

## Considered Options（選択肢）

1. 今のまま。PostgreSQL も `text COLLATE "C"`
2. PostgreSQL は `uuid` 型、SQLite は `TEXT` の 36 文字のまま。Go ではどちらも `string`
3. PostgreSQL は `uuid` 型、SQLite は `BLOB` の 16 バイト
4. 2 に加えて、ドメインに ID の型（`kernel.ID` など、`[16]byte`）を持つ

## Decision（決定）

**選択肢 2 を採る。** PostgreSQL の UUIDv7 の列を `uuid` 型にし、SQLite と Go の型は変えない。ADR-0003 の「型・ID・時刻の共通規約」の ID の項のうち、
PostgreSQL での型（`COLLATE "C"` の `text` か `uuid` 型）を、この ADR が `uuid` 型に決める。ほかの部分は ADR-0003 のまま。

### 1. 対象の列

- 対象は、**値がいつもこのサーバーが生成した UUID である列**。主キーの `id`、それを指す `*_id` の列、UUIDv7 だけを持つカーソルの列
- 次は `text` のまま（`COLLATE "C"` も今のまま）
  - UUID でない識別子: AP の URI、外部の事業者の ID（`provider_*_id`、決済の webhook の `event_id` / `object_id`）、OAuth の `client_id` と冪等キー（クライアントが決める値）
  - 種類の違う値が入る列: 監査ログの `target_id`（設定のキーなども入る）、リアクションの `emoji_key`（Unicode の絵文字かカスタム絵文字の ID）、
    ジョブの `run_key`、課金の取り込みのカーソル（`changes_cursor` など）
- 判断に迷う列は、値の出どころで決める。生成元が `kernel.IDGenerator` だけなら `uuid`、それ以外が 1 つでもあれば `text`
- `uuid` の列と `text` の列を比べるところ（`emoji_key` と `custom_emoji_id` の `CHECK` など）は `uuid_col::text` で比べる。`uuid` の文字列への変換は小文字の正規形なので、保存してある文字列と一致する

### 2. 並び順

- PostgreSQL の `uuid` の比較は 16 バイトの符号なしのバイト順（`memcmp`）。UUIDv7 は先頭の 48 ビットがミリ秒の時刻（ビッグエンディアン）なので、バイト順 = 時刻順 = 小文字の正規形の文字列のバイト順
- そのため `ORDER BY id`、`id < :cursor` のキーセットページング、`id >= UUIDv7Floor(t)` の読み取り窓は、意味も結果も変わらない
  - 確認: PostgreSQL 18.6 で 200 万行の UUIDv7 を `ORDER BY id` と `ORDER BY id::text COLLATE "C"` で並べ、順位の食い違いは 0 行（2026-10-23）
- PostgreSQL には `uuid` の `max` / `min` の集約関数が無い（18.6 で `function max(uuid) does not exist` を確認）。今の `MAX(id)` / `MIN(id)`（`queries/postgres` の 15 か所）は、
  `ORDER BY id DESC LIMIT 1`・`DISTINCT ON` などに書き換える。集約関数を自前で定義することはしない（[ADR-0023](0023-declarative-schema-management.md) の目標スキーマに書けないものを増やさない）

### 3. SQLite

- SQLite は `TEXT` の 36 文字のまま。SQLite は小さな構成向けで、大きさの差の効き目が小さい。`sqlite3` の CLI で読めること、libSQL / Turso でもそのまま動くことを優先する
- `BLOB` にしないので、SQLite の生成コードとクエリは型については変わらない

### 4. Go の型と sqlc

- アプリケーション層とドメインの ID は `string`（36 文字の小文字の正規形）のまま。ドメインに ID の型は持たない
- sqlc は PostgreSQL の `uuid` を `string` に対応させる（`sqlc.yaml` の `overrides` に `db_type: uuid`・`go_type: string`、NULL 可は `*string`）。
  pgx/v5 は `string` を `uuid` の引数に渡せ、`uuid` の列を `string` に読める（`pgtype.UUIDCodec` の text の経路。pgx v5 で確認、2026-10-23）。
  そのため `pgtype.UUID` も Go 1.27 の `uuid.UUID`（`driver.Valuer` / `sql.Scanner` を実装しない、#17）も、アダプタの外に出ない
- 方言の差は `schema/<dialect>.sql`・`queries/<dialect>/`・`sqlc.yaml` に閉じ込める。リポジトリのポートと契約テスト（`pkg/testkit`）のシグネチャは変わらない
- 「無し」を空文字 `''` で表す ID の引数・列は `uuid` に入らないので、次のように直す
  - クエリの任意の引数（`sqlc.arg(before)::text = ''` の形、72 か所）は、NULL 可の引数（`sqlc.narg`）にする
  - 列に `''` を保存しているもの（リモートのリンクカードの画像の `media_files.owner_account_id` など）は、任意の関連として NULL 可にする（[ADR-0020](0020-data-modeling-principles.md) 規約 4 の「任意の関連」）

### 5. 外へ出す形と、不正な ID の入力

- 外へ出す ID は 36 文字の小文字の正規形のまま。client API の `Id`（`format: uuid`）、AP の `id`、表示用 URL は変わらない。連合の URI は変えない（ADR-0012 の例外、ADR-0018）
- PostgreSQL の `uuid` は大文字・ハイフン無し・`{…}` の形も受け付けて同じ値にする（18.6 で確認）。これを通すと同じ投稿に複数の URL ができるので、
  **受け付ける形は 36 文字の小文字の正規形の UUIDv7 だけ** にする。判定は `kernel` に 1 つの関数（`kernel.ValidID` など）として置き、外から来た ID を DB に渡す前に使う
  - 今の `internal/interface/activitypub` の `validID`（同じ正規表現）をこの関数に移す
  - 使うのは、client API・AP の入口と、アプリケーション層で受信した文書から自サーバーの URI の ID を取り出すところ（`internal/app/federation` の引用の承認の `quoteRequestStatus` など）
  - client API の path の ID: 正規形でなければ、存在しない ID と同じ `404`（ADR-0018 の、存在しないときと権限が無いときを同じにする方針に合わせる）
  - client API のクエリ・本文の ID（カーソル、`ids` での絞り込み、返信先・添付の参照など）: 正規形でなければ、その欄に存在しない ID を渡したときと同じ応答（カーソルは今の壊れたカーソルと同じ `400` の `invalid_parameter`）
  - AP: 受信した文書の中の自サーバーの URI は、今のとおり `^/ap/(actors|objects|activities)/{uuid}` で厳密にパースし、合わなければ自サーバーの URI として扱わない。正規表現の `{uuid}` は小文字の正規形だけにする
- アダプタに不正な形の ID が届くのは、上の検査の漏れ（バグ）として扱う。PostgreSQL では SQLSTATE `22P02` のエラーになり `500` になる。漏れは契約テストと client API のテスト（不正な形の ID を入れたときの応答）で見つける

### 6. 大きさと速さ（簡単な計測）

PostgreSQL 18.6（`postgres:18-alpine`、WSL2 上の Docker）で、投稿に似た表（`id`・`account_id`・短い本文、200 万行、アカウント 1 万）を作り、`VACUUM ANALYZE` の後に測った（2026-10-23）。
速さは 1 台の手元の値で、傾向だけを見る。

| | `text COLLATE "C"` | `uuid` | 差 |
| --- | --- | --- | --- |
| 表 | 208 MB | 130 MB | −37% |
| 主キーの索引 | 113 MB | 60 MB | −47% |
| `(account_id, id)` の索引 | 321 MB | 137 MB | −57% |
| 200 万行の INSERT（索引あり） | 11.5 秒 | 9.6 秒 | −16% |
| 7 日の読み取り窓の件数（`id >= floor`） | 424 ms | 376 ms | −11% |
| 投稿とアカウントの結合（全件） | 79 ms | 62 ms | −22% |
| 主キーでの 1 行の取得 | 19.6 µs | 19.1 µs | ほぼ同じ |
| アカウントごとのキーセットのページ（41 行） | 232 µs | 222 µs | ほぼ同じ |

ID の列が多い表（関係・投影・イベント）ほど効く。読み取りの量で課金される DB（Aurora DSQL の DPU など）では、索引の大きさの差がそのまま読み取り量の差になる（要検証: 実際の課金への効き目は M6 の計測で確かめる）。

### 7. M6 の対象とテスト

- Aurora DSQL は `UUID` 型（16 バイト、索引に使える）に対応している（[Supported data types in Aurora DSQL](https://docs.aws.amazon.com/aurora-dsql/latest/userguide/working-with-postgresql-compatibility-supported-data-types.html)、2026-10-23 に確認）。
  `bytea` は DSQL で索引に使えないので、PostgreSQL でも `bytea` の 16 バイトにはしない
- Neon・Supabase・Cloud SQL・RDS / Aurora は PostgreSQL そのものなので、`uuid` は組み込みの型として使える
- pgmem（PGlite のフォーク、PostgreSQL 18.3）も組み込みの `uuid` をそのまま持つ。毎 PR の契約テストで新しいスキーマを通す

### 8. 移行

ADR-0012 のとおり移行は書かない。`schema/postgres.sql` を書き換えて、DB を作り直す（`db-reset`）。この変更は v0.x の公開と、#1028 で決める「実運用開始後のスキーマ変更の扱い」より前に入れる。

### この決定を見直す条件

- M6 の対象のどれかで、`uuid` 型の列の索引や `uuid` と `text` の比較が使えない・遅いと分かった場合
- 外部キーや結合で `uuid` と `text` が混ざる列が多く、キャストがクエリの大半に要ると分かった場合（そのときは選択肢 1 のまま）

## Consequences（影響）

- 良い影響
  - PostgreSQL の表と索引が 4〜6 割ほど小さくなり、結合と範囲の読み取りが 1〜2 割速くなる
  - DB が UUID でない値を拒み、ID の列に別の種類の値が混ざらない
  - 外から受け取る ID の形の検査が 1 か所にまとまり、`/statuses/0192F3A4-…` のような正規形でない ID の扱いが方言に依らず決まる
- 悪い影響・トレードオフ
  - 方言でスキーマの型が変わり、PostgreSQL のクエリだけに `::text` のキャスト、`MAX(id)` の書き換え、`sqlc.narg` が入る。SQLite と PostgreSQL のクエリの見た目の差が増える
  - `psql` で ID の列を文字列の関数（`LIKE` など）で扱うときにキャストが要る
  - テストの ID の文字列の多く（`pkg/testkit` などの `"mod-1"`・`"admin"` のような値、500 か所ほど）を、正規形の UUID に直す必要がある
- 後続の作業
  1. テストの ID を正規形の UUID に直す（`pkg/testkit` と各パッケージのテスト。スキーマを変える前に入れられる）
  2. `kernel.ValidID` と、client API・AP の入口での検査とそのテスト（§5。スキーマを変える前に入れられる）
  3. `schema/postgres.sql` の対象の列を `uuid` にし、`sqlc.yaml` の対応、`queries/postgres/` の書き換え（キャスト・`MAX` / `MIN`・`sqlc.narg`・`''` の列の NULL 可）、
     `make sqlc`、契約テストを pgmem とコンテナで通す。スキーマ・クエリ・生成コードは 1 つの PR にする
  4. data-model §1.2 の `id` の行と §11 の未決事項を、この ADR に合わせて直す（3 と同じ PR）

## Pros and Cons of the Options（詳細比較・任意）

| | 1. 今のまま | 2. PG は `uuid`、SQLite は `TEXT` | 3. PG は `uuid`、SQLite は `BLOB` | 4. 2 + ドメインの ID の型 |
| --- | --- | --- | --- | --- |
| PG の大きさ・速さ | 今のまま | 小さく速い | 小さく速い | 小さく速い |
| SQLite の大きさ | 今のまま | 今のまま | 小さい | 今のまま |
| 不正な値を DB が拒む | しない | PG だけ | 両方 | PG だけ（型で入口でも拒む） |
| アプリ・ドメインの変更 | 無し | 無し（入口の検査を足す） | 無し（SQLite のアダプタで変換） | 全モジュール |
| クエリの変更 | 無し | PG だけ | 両方 | PG だけ |
| CLI での読みやすさ | 文字列 | PG は文字列で表示、SQLite は文字列 | SQLite は 16 進で読む | 2 と同じ |

- 選択肢 1 の最も強い点: 変更が無く、2 方言のクエリがほとんど同じ形のまま保てる。ID の列はすでに `COLLATE "C"` で正しく並ぶので、正しさの問題は無い。
  大きさと速さの差は、小さな構成ではほとんど見えない
- 選択肢 3 は SQLite も小さくなるが、`sqlc` の生成型が `[]byte` になり、SQLite のアダプタで変換が要る。SQLite を使う小さな構成で得るものに比べて手間が大きい
- 選択肢 4 は型で正規形を保証できるが、`string` を前提にした全モジュールの書き換えになる。入口の検査（§5）で同じ保証が得られる

## Open Questions / 要検証

- `uuid` と `text` が混ざる比較・結合（§1 の `text` のままの列との結合）が、実際のクエリでどれだけあるか。後続の作業 3 で数え、多すぎれば「この決定を見直す条件」に当たる
- 読み取り量で課金される DB（Aurora DSQL など）での、実際の課金への効き目（M6 の計測）
