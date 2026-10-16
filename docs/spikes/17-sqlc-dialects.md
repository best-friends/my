# Spike #17: sqlc で SQLite / PostgreSQL のリポジトリと UnitOfWork を作る

- Issue: #17
- 関連 ADR: [ADR-0001](../adr/0001-server-language-and-runtime.md), [ADR-0003](../adr/0003-database-abstraction.md), [ADR-0011](../adr/0011-license.md)
- コード: [`spikes/17-sqlc-dialects/`](../../spikes/17-sqlc-dialects/)（独立モジュール。プロダクトコードから import しない）
- 実施日: 2026-10-16 / Go 1.27.2 / sqlc v1.31.1

## 結論

- **ADR-0003 のモデル（リポジトリポート + 方言別アダプタ + 共通コントラクトテスト + 短い再実行可能トランザクション）は Go で無理なく書ける。**
  `accounts` / `follows` の 2 テーブルと Follow ユースケース（一意制約 + upsert + カウンタ加算）で、同一のコントラクトテスト 13 件が
  SQLite 2 ドライバ × {一時ファイル, インメモリ} と PostgreSQL 18 × {SERIALIZABLE, READ COMMITTED} の計 6 構成すべてで通った。
- 推奨ツール: **sqlc（方言ごとに生成）+ `pgx/v5`（PostgreSQL）+ `modernc.org/sqlite`（SQLite）**。`ncruces/go-sqlite3` も同等に動くので、
  アダプタはドライバ非依存に書き、ドライバ差は接続オープンとエラー分類（約 30 行）に閉じ込める。
- 時刻: **SQLite は INTEGER（UTC の Unix ミリ秒）、PostgreSQL は `timestamptz`、ドメインでは `time.Time` を UTC・ミリ秒に丸めて扱う。**
- UUIDv7: **Go 1.27 標準ライブラリの `uuid` パッケージ（`uuid.NewV7()`）** で足りる。サードパーティ依存は不要。
- 全構成で `CGO_ENABLED=0` のビルド・テストが通る。依存ライセンスはすべて AGPL-3.0 と両立する（ただし後述の go-licenses の検出漏れに注意）。

## 実験の構成

```text
spikes/17-sqlc-dialects/
  schema/{sqlite,postgres}.sql      方言別 DDL（sqlc の入力 + go:embed）
  queries/{sqlite,postgres}/*.sql   方言別クエリ（sqlc の入力）
  store/                            ポート（Account / Follow / リポジトリ / Tx / UnitOfWork / エラー）
  app/                              Follow / Unfollow ユースケース（store にのみ依存）
  internal/txretry/                 有限回リトライのループ（全アダプタ共通）
  adapter/sqlitedb/                 SQLite アダプタ（database/sql、modernc / ncruces を切替）
  adapter/pgdb/                     PostgreSQL アダプタ（pgx/v5 ネイティブ API）
  contract/                         共通コントラクトテスト（Factory を受け取る単一スイート）
  sketches/{bob,handwritten}/       比較用の最小スケッチ（コンパイル + SQL 生成のみ）
  cmd/size-*/                       バイナリサイズ計測用
```

実行方法:

```sh
cd spikes/17-sqlc-dialects
CGO_ENABLED=0 go test ./...                 # PG は docker があれば testcontainers-go で自動起動
TEST_POSTGRES_URL=postgres://... go test ./... # 既存の PG を使う
TEST_POSTGRES_DOCKER=0 go test ./...         # docker を使わない（PG テストは Skip）
go test -race ./...                          # race detector は cgo が必要（ドライバ自体は cgo 不要）
```

コントラクトテストの内容: upsert（URI 一意キーで ID・`created_at` を保持し可変フィールドだけ更新）、NotFound、時刻の往復（ナノ秒・JST 入力 → UTC ミリ秒で返る）、
Follow の冪等性（2 回目以降は `created=false`・カウンタ不変）、存在しない相手へのフォロー、エラー時ロールバック、Unfollow の冪等性、
UUIDv7 カーソルによる `max_id` ページング、カウンタ再集計、リトライの上限・非リトライエラー・再試行後成功、
同時実行（16 並列で別々のフォロワー / 同じペア / 同じ URI の upsert）。

## 結果

### コントラクトテスト

| 構成 | 結果 | 同時実行テストで観測したリトライ回数（別フォロワー / 同一ペア / 同一 URI） |
| --- | --- | --- |
| modernc.org/sqlite v1.60.1・一時ファイル（WAL） | PASS | 0 / 0 / 0 |
| modernc.org/sqlite・`:memory:` | PASS | 0 / 0 / 0 |
| ncruces/go-sqlite3 v0.35.6・一時ファイル（WAL） | PASS | 0 / 0 / 0 |
| ncruces/go-sqlite3・`:memory:` | PASS | 0 / 0 / 0 |
| PostgreSQL 18（testcontainers-go）・SERIALIZABLE | PASS | 54 / 1 / 34 |
| PostgreSQL 18（testcontainers-go）・READ COMMITTED | PASS | 0 / 0 / 0 |

- **PostgreSQL のテストは実際に実行した**（ローカルの Docker 29.8 上で `postgres:18-alpine` を testcontainers-go で起動）。
  初回はイメージ取得込みで約 24 秒、2 回目以降はスイート全体で約 4 秒。
- SQLite は `BEGIN IMMEDIATE`（`_txlock=immediate`）+ `busy_timeout` で書き込みが直列化されるため、リトライは発生しない。
  比較のため `_txlock=deferred` で同じ 16 並列フォローを流すと、modernc で 20 回、ncruces で 15 回のリトライが発生した（結果はリトライで正しく収束）。
- PostgreSQL の SERIALIZABLE はホットな行（人気アカウントのカウンタ）で直列化失敗が多発する。READ COMMITTED でも加算 SQL と `ON CONFLICT` により結果は正しい。

### SQLite ドライバ（`CGO_ENABLED=0`）

| 観点 | modernc.org/sqlite v1.60.1 | ncruces/go-sqlite3 v0.35.6 |
| --- | --- | --- |
| 方式 | C の SQLite を Go にトランスパイル | SQLite の Wasm ビルドを wasm2go で Go に変換（wazero 依存はなくなった） |
| 同梱 SQLite | 3.53.4 | 3.53.4 |
| `CGO_ENABLED=0` ビルド / テスト | OK / OK | OK / OK |
| バイナリサイズ（`-s -w`、ドライバのみ。基準 1.7 MiB） | 6.4 MiB | 7.8 MiB |
| Follow 1 件（読み 2 + 書き 3、WAL ファイル） | 約 130 µs | 約 115 µs |
| DSN | `_pragma=...` / `_txlock=immediate`（ncruces と同じキー） | `_pragma=...` / `_txlock=immediate` |
| BUSY の判定 | `*sqlite.Error` の `Code() & 0xff` | `errors.Is(err, sqlite3.BUSY)` |
| `time.Time` を直接バインドした場合 | Go の `Time.String()` 形式の TEXT（SQLite の日付関数で解釈不可、`time.Time` に Scan できない） | RFC 3339 の TEXT（オフセット保持・小数部可変長） |
| ライセンス | BSD-3-Clause（libc 等も BSD-3） | MIT + MIT-0（`go-sqlite3-wasm`）+ SQLite（public domain） |
| go-licenses の判定 | 全て識別される | `go-sqlite3-wasm/v6` が **Unknown**（MIT-0 を識別できない）→ 現行の `make go-licenses` では失敗する |

推奨は **modernc.org/sqlite**。決め手は (1) 現行のライセンス検査をそのまま通る、(2) バイナリが小さい、(3) 実績が長い。
性能差（約 1 割）は v0 の要件では問題にならない。ドライバ差はアダプタの `drivers.go` だけに閉じているので、後から ncruces へ切り替えるコストは小さい
（ncruces を採る場合は `GO_ALLOWED_LICENSES` に `MIT-0` を足すか、当該モジュールを手動確認の上で除外する）。

### SQL ツール比較

| 観点 | **sqlc（方言別生成）** | bob（クエリビルダ / bobgen） | `database/sql` + 手書き |
| --- | --- | --- | --- |
| 方言対応 | engine: `sqlite` / `postgresql`（MySQL も可）。パーサが方言ごとに SQL を検査 | `dialect/psql`・`dialect/sqlite`・`dialect/mysql` を別パッケージで提供 | 自前（プレースホルダ `?` / `$1` の書き換え、値のエンコードを方言ごとに分岐） |
| 型安全 | 高い。スキーマとクエリから引数・結果の構造体を生成し、列の誤りは生成時に失敗 | ビルダは列名が文字列で実行時まで誤りに気付かない。bobgen を使えば型付きになるが **生きた DB から** 方言ごとに生成 | 低い。Scan の列順を手で合わせる |
| pgx ネイティブ API | `sql_package: pgx/v5` で使える | database/sql 経由のみ | `pgx/v5/stdlib` 経由のみ |
| 生成コード量（本 spike） | 方言あたり約 270 行（クエリ 9 本）。手書きアダプタはマッピング中心で約 180 行 / 方言 | 生成なし（ビルダ）または bobgen のモデル一式（大きい） | 生成なし。sqlc の生成コード相当を手で書く |
| クエリ重複の扱い | 方言ごとに `.sql` を持つ。本 spike の 2 方言のクエリ差分は **1 箇所（`::bigint` キャスト）だけ**。共通部分は 1 ファイルを両 engine に読ませることもでき、`CAST(? AS bigint)` に書き換えた共通ファイルから両方言の生成が通ることを確認した | Go コードが方言ごとに必要（`sqlite.Insert` / `psql.Insert`）。ロジックは書けるが SQL 文字列の重複が Go コードの重複に置き換わるだけ | 共通サブセットは 1 文字列で共有できるが、方言差が各メソッドに漏れる |
| 学習コスト / レビュー性 | SQL をそのまま読める。生成物は素直な Go | DSL の習得が必要 | 最小。ただし定型コードが多い |
| ライセンス | MIT（ツールのみ。生成コードはランタイム依存なし） | MIT | — |

**sqlc を推奨する。** 方言差の大半はスキーマ（型）とアダプタのマッピングに出て、クエリ本文の差は小さかった。
bob は「方言を跨いで 1 つのコードで書ける」わけではなく（ビルダが方言別パッケージ）、型安全を得るには方言ごとに生きた DB から生成する必要がある。
手書きは方言分岐とスキャン定型が全メソッドに散る。sqlc の弱点（動的 WHERE を書きにくい）は、タイムライン等で必要になった時点で
アダプタ内にその 1 クエリだけ手書き / ビルダを使う逃げ道で足りる。

### 時刻の保存形式

| 観察 | 内容 |
| --- | --- |
| ドライバ既定の TEXT 化はドライバごとに違う | modernc は `2026-10-16 20:00:00.1 +0900 JST`（Go の `String()` 形式）、ncruces は `2026-10-16T20:00:00.1+09:00`（RFC 3339）を書く。同じ DB を別ドライバで読むと解釈が変わる |
| 文字列順 ≠ 時刻順 | 3 つの時刻を TEXT 列で `ORDER BY` すると、modernc は `0,1,2`、ncruces は `1,0,2`（正しくは `2,0,1`）。原因はタイムゾーンの混在と小数部の可変長（`…00.5Z` < `…00Z`、`.` < `Z`） |
| INTEGER 列でも防げない | `time.Time` を INTEGER 列にバインドしても両ドライバとも TEXT で格納される（`typeof` = `text`）。STRICT テーブルならエラーになる |
| 整数ミリ秒なら一意 | `t.UnixMilli()` を INTEGER に入れれば比較・索引・範囲検索が単純。現在時刻で 6 バイト。SQLite 側でも `datetime(created_at / 1000, 'unixepoch')` で読める |
| PostgreSQL | `timestamptz` はマイクロ秒精度。アダプタで `store.Timestamp()`（UTC・ミリ秒切り捨て）を通せば SQLite と往復結果が一致する（コントラクトテストで確認） |

**推奨:** SQLite は `INTEGER NOT NULL`（UTC Unix ミリ秒）+ `STRICT` テーブル、PostgreSQL は `timestamptz`。ドメイン型は `time.Time` のまま、
書き込み前に `UTC().Truncate(time.Millisecond)` へ正規化する（ポート側の関数 1 つ）。ISO 8601 文字列は固定幅・UTC・`Z` 終端を全経路で強制できれば成り立つが、
上の通りドライバ既定の挙動に任せると壊れやすく、規律の維持コストに見合わない。PostgreSQL も `bigint` に揃える案は挙動が完全に一致する利点があるが、
運用者が psql で読む・日付関数や将来のパーティショニングを使う利便を優先して `timestamptz` とする。

### UUIDv7

- Go 1.27 で標準ライブラリに `uuid` パッケージが入った（RFC 9562、`NewV7()` は「時計が戻らない限り単調増加」を保証）。
  20 万件を連続生成して文字列比較で厳密に増加することを確認（約 105 ns / 件）。**サードパーティの UUID ライブラリは不要。**
- 保存は ADR-0003 どおり 36 文字の小文字文字列。`max_id` ページングは `id < ?` の文字列比較で行い、コントラクトテストで新しい順に正しく辿れることを確認した。
- 標準 `uuid.UUID` は `driver.Valuer` / `sql.Scanner` を実装しないので、アダプタ境界で `String()` / `uuid.Parse` する（本 spike は文字列のまま扱った）。

## UnitOfWork とリポジトリポートの提案

```go
// package store — ドライバを import しない
var (
    ErrNotFound  = errors.New("store: not found")
    ErrRetryable = errors.New("store: retryable transaction conflict") // アダプタがドライバのエラーを包む
)

type AccountRepository interface {
    Upsert(ctx context.Context, a Account) (Account, error) // URI で一意。返り値の ID が既存行のものになり得る
    Get(ctx context.Context, id string) (Account, error)
    AddFollowCounts(ctx context.Context, id string, followersDelta, followingDelta int64) error // SET n = n + ?
    RecountFollowCounts(ctx context.Context, id string) error                                // 補正ジョブ用
}

type FollowRepository interface {
    Insert(ctx context.Context, f Follow) (created bool, err error) // ON CONFLICT DO NOTHING。重複はエラーではない
    Delete(ctx context.Context, followerID, targetID string) (deleted bool, err error)
    Get(ctx context.Context, followerID, targetID string) (Follow, error)
    ListFollowers(ctx context.Context, targetID, maxID string, limit int) ([]Follow, error)
}

// Tx はトランザクションに束縛されたリポジトリの集合。集約が増えたらメソッドを足す。
type Tx interface {
    Accounts() AccountRepository
    Follows() FollowRepository
}

// fn はリトライで複数回呼ばれ得る。外部 I/O をせず、捕捉した結果変数は冒頭でリセットする。
type UnitOfWork interface {
    Tx(ctx context.Context, fn func(tx Tx) error) error
}

// ユースケース側（抜粋）
err = s.UoW.Tx(ctx, func(tx store.Tx) error {
    created = false
    ok, err := tx.Follows().Insert(ctx, store.Follow{ID: uuid.NewV7().String(), ...})
    if err != nil || !ok {
        return err
    }
    if err := tx.Accounts().AddFollowCounts(ctx, targetID, +1, 0); err != nil {
        return err
    }
    created = true
    return tx.Accounts().AddFollowCounts(ctx, followerID, 0, +1)
})
```

- リトライは共通の `txretry.Run(ctx, Policy, once)` に置く。既定は最大 5 回・指数バックオフ（2 ms から 100 ms 上限、フルジッタ）、`OnRetry` フックでメトリクスを出す。
  リトライ対象は `errors.Is(err, store.ErrRetryable)` のみ。アダプタが PG の `40001`（serialization_failure）/ `40P01`（deadlock_detected）、
  SQLite の `SQLITE_BUSY` / `SQLITE_LOCKED` を `ErrRetryable` に包む（`errors.As` で元のドライバエラーにも届くように多重 Unwrap で包む）。
- アダプタはトランザクションごとに sqlc の `Queries` を `New(tx)` で作り、`Tx` 実装に渡すだけ。マッピング（`int64` ミリ秒 ⇔ `time.Time`、`sql.ErrNoRows` / `pgx.ErrNoRows` ⇔ `ErrNotFound`）はアダプタ内。
- 読み取り専用のクエリも当面は `Tx` を通してよい（SQLite では `BEGIN IMMEDIATE` が書き込みロックを取るため、読み取り専用の経路は M1 で別途 `ReadOnly` 等を検討）。

## 落とし穴

1. **SQLite の deferred トランザクション**: 読み取り後に書き込みへ昇格するとき `SQLITE_BUSY` が `busy_timeout` を待たずに返る。`_txlock=immediate` を必須にする（上記の計測で deferred は 15〜20 回リトライ、immediate は 0 回）。
2. **SQLite の `:memory:`** は接続ごとに別 DB になる。`SetMaxOpenConns(1)` が必要で、その状態で `fn` の中からトランザクション外のクエリを投げるとデッドロックする（ポート設計上 `Tx` 経由しか持たせないことで防ぐ）。
3. **PostgreSQL の SERIALIZABLE** はホットな行で直列化失敗が多発する（16 並列で 54 回）。既定の 5 回では足りない場面がある。既定は READ COMMITTED + 加算 SQL + `ON CONFLICT` とし、リトライは deadlock・DSQL 用の保険と位置付けるのがよい。
4. **トランザクション関数の再実行**: 外側の変数に結果を入れる書き方は、試行ごとにリセットしないと前回の値が残る。ID・時刻を `fn` 内で生成すると試行ごとに変わる（UUIDv7 なら問題ないが、冪等キーは必ず `fn` の外から渡す）。
5. **`ON CONFLICT DO NOTHING ... RETURNING`** は衝突時に行を返さない。作成有無は sqlc の `:execrows`（RowsAffected）で判定する。逆に upsert で既存行を返したいときは `DO UPDATE ... RETURNING *` を使う。
6. **PostgreSQL の照合順序**: `text` の ID を `id < $1` で比較するとき、DB 既定の照合順序（例: `en_US.utf8`）に依存する。ID 列には `COLLATE "C"` を付けるか `uuid` 型を使う（SQLite は既定で BINARY）。
7. **sqlc の細部**: 相関サブクエリで `id` が曖昧だとエラー（`accounts.id` と修飾）。クエリファイル先頭のコメント（SPDX ヘッダ）が最初のクエリの doc コメントとして生成コードに入る。
   生成ファイルには SPDX ヘッダが付かない（生成物はヘッダ検査の対象外にする）。`timestamptz` の既定 Go 型は `pgtype.Timestamptz` なので `overrides` で `time.Time` にする。
   `LIMIT` の引数型が SQLite は `int64`、PG は `int32` など、方言ごとに Params 構造体が微妙に違うため、アダプタのマッピングは共通化しにくい（方言あたり約 180 行の重複を許容する）。
8. **go-licenses** は ncruces の `go-sqlite3-wasm`（MIT-0）を Unknown と判定する。ncruces を採るなら許可リストの調整が要る。
9. **race detector** は cgo が必要。`CGO_ENABLED=0` のビルド検査とは別に、CI のテストは cgo 有効で `-race` を回す（ドライバ自体は cgo 不要なので両立する。本 spike で `-race` も通過）。
10. **testcontainers-go** は ryuk コンテナも起動する。GitHub Actions の `ubuntu-latest` は Docker が使えるので毎 PR で実行できる見込み。Docker がない環境向けに `TEST_POSTGRES_URL` / `TEST_POSTGRES_DOCKER=0` で切り替えられるようにした。

## 未検証 / 後続

- libSQL / Turso（Go クライアントの cgo 要否、リモート接続でのトランザクション）と Aurora DSQL は本 spike の範囲外（ADR-0003 の Open Questions に残す）。
- スキーマと論理データモデルの乖離検査、マイグレーションツールの選定（v0 は破壊的変更を許容するので優先度は低い）。
- 読み取り専用トランザクション / 読み取り経路の API。
