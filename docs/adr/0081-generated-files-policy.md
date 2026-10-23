# ADR-0081: Go が読む生成物だけをコミットする

- Status: Proposed
- Date: 2026-10-23
- Deciders: @rosylilly
- Issue: #1052

## Context（背景・課題）

ソースから作れるファイルの多くは、生成してコミットしている。CI はそれが古くないかを検査する（`i18n-check`・`generate-check`・`sqlc-check`・`web-routes-check`・`web-tokens`・`adr-index-check`）。
コミットする理由は、生成器を回さずにビルドと型検査ができること、PR の差分で生成の結果を見られることだった
（[ADR-0009](0009-internationalization.md)・[ADR-0059](0059-design-tokens-and-instance-theme.md)・[ADR-0066](0066-openapi-split-and-bundle.md)・[ADR-0000](0000-record-architecture-decisions.md) の索引）。

並行の作業（[workflow.md §9](../agents/workflow.md#9-parallel-batches)）では、生成物が衝突の主な原因の 1 つになっている。
直近 14 日で、ブランチに main をマージしたコミット 138 件のうち、次の生成物が衝突の解決で変わっていた:

| 生成物 | 衝突の回数 | 入力 | 読む側 |
| --- | --- | --- | --- |
| `tools/i18n/src/runtime/messages.gen.ts` | 42 | `locales/` | TS |
| `locales/en-XA/*.json`・`locales/ar-XB/*.json`（疑似ロケール） | 各 36 | `locales/en/` | TS、Go（開発と E2E だけ、`i18n.WithPseudoLocales`） |
| `docs/adr/README.md`（ADR の索引） | 16 | `docs/adr/*.md` | 人 |
| sqlc の生成物（`postgresgen`・`sqlitegen` など） | 各 7 | `schema/`・`queries/` | Go |
| `app/web/src/routeTree.gen.ts` | 7 | `app/web/src/routes/` | TS |
| `app/web/src/lib/api/schema.gen.ts` | 7 | `api/openapi.yaml` | TS |
| `api/openapi.yaml`（バンドル） | 7 | `api/spec/` | Go（埋め込みと照合）、生成器 |
| `internal/interface/clientapi/api.gen.go` | 6 | `api/openapi.yaml` | Go |

衝突した PR は Merge Queue から外れる。エージェントは main をマージし、生成し直し、PR と Merge Queue の CI が最初から回る（[ADR-0079](0079-ci-single-gate-merge-queue.md) の実測）。
GitHub は、サーバーでのマージにカスタムのマージドライバ（`.gitattributes` の `merge=`）を使わない。そのため、生成物の形式を工夫しても、行が近ければ衝突は避けられない。

決定者の回答（チャット、原文）: 生成物は「Go が読む物だけ残す」。
ADR-0066 の判断基準「`go build` / `go test` が Bun なしで動く（Go の CI は Bun を入れない）」は守る。

## Decision Drivers（判断基準）

1. 並行の PR が、生成物だけで衝突しない
2. `go build` と `go test` が、Go だけで（Bun なしで）動く（ADR-0066）
3. 生成物が入力に対して古いまま main に入らない
4. 手元と CI で、生成器を回す手間と時間が小さい（[ADR-0080](0080-task-runner-go-task.md) のチェックサムによる飛ばし）
5. 生成の結果（API の型やルートの木）が、レビューで追える

## Considered Options（選択肢）

1. 今のまま。すべてコミットし、出力の形式を衝突しにくくする
2. Go が読む生成物だけコミットし、TS だけが読む生成物・疑似ロケール・ADR の索引はコミットしない
3. 生成物を一切コミットしない。Go の生成器を Go に移すか、`go build` の前に Bun の生成を必ず挟む

## Decision（決定）

> 推奨案。決定者の回答は Context のとおり。

**選択肢 2 を推奨する。**

### 生成物の扱い

| 生成物 | 扱い | 作る時点 |
| --- | --- | --- |
| `api/openapi.yaml`、`clientapi/*.gen.go` | コミットする（Go が読む） | `task api:generate` |
| sqlc の生成物 | コミットする（Go が読む） | `task sqlc:generate` |
| `internal/i18n/messages_gen.go`、`internal/i18n/testdata/golden.json` | コミットする（Go が読む） | `task i18n:generate` |
| `internal/domain/instance/theme_tokens.gen.go` | コミットする（Go が読む） | `task ui:tokens` |
| `emoji_table.gen.go`、`ai_crawlers.gen.go` | コミットする（外のデータを固定した版で取り込む。入力がリポジトリにない） | 手動（今のまま） |
| `docs/reference/configuration.md` | コミットする（Go のテストが生成する人向けの文書。衝突は少ない） | `task go:config-docs` |
| `@my/ui` のスクリーンショットの基準画像 | コミットする（派生物ではなく期待値。ADR-0027） | CI のコンテナ |
| `tools/i18n/src/runtime/messages.gen.ts` | **コミットしない** | 型検査・ビルド・テストの前（`deps`） |
| `app/web/src/lib/api/schema.gen.ts` | **コミットしない** | 同上 |
| `app/web/src/routeTree.gen.ts` | **コミットしない** | 同上 |
| `app/ui/src/themeTokens.gen.ts` | **コミットしない** | 同上 |
| 疑似ロケール（`locales/en-XA`・`locales/ar-XB`） | **コミットしない** | 開発の起動・E2E・クライアントのビルドの前 |
| ADR の索引（`docs/adr/README.md` の表） | **なくす** | — |

- コミットしない生成物は `.gitignore` に入れる。生成のタスクは `sources` と `generates` を宣言する（ADR-0080）。
  使う側のタスク（`web:typecheck`・`web:build`・`web:test`・`i18n:test` など）は、生成のタスクを `deps` に持つ。入力が変わっていなければ、生成は飛ぶ。
- エディタの型検査のために、`task generate` を一度回す手順を CONTRIBUTING.md に書く。開発の起動（`task dev`）も生成を `deps` に持つ。
- コミットする生成物の鮮度は、Merge Queue の `server-static`（ADR-0079）で検査する。2 つの PR の入力を重ねると古くなりうるからである。
- **疑似ロケール**: Go がこれを読むのは、開発と E2E で `i18n.WithPseudoLocales` を付けたときだけである。本番のビルドには要らない。
  そのため、「Go が読む物」には入れずにコミットをやめる。疑似ロケールのファイルがないときに `WithPseudoLocales` を付けたら、生成の手順を示すエラーで起動を止める。
  E2E と開発の起動のタスクは、生成を `deps` に持つ。疑似ロケールを使う Go のテストは、`testdata` の小さなカタログに移す。
  こうして `go test ./...` を Bun なしで回せる状態を保つ。
- **ADR の索引**: README の表をなくし、運用の規則だけを残す。一覧は `scripts/agentkit.sh adr list`（番号・タイトル・Status。`--status Proposed` で絞る）で見る。
  GitHub では、ディレクトリの一覧とコード検索（`path:docs/adr "Status: Proposed"`）で見る。
- **ADR の番号**: `scripts/adr-new.sh` は、main と開いている PR（`gh pr list` と `gh pr diff --name-only`）にある ADR の番号の最大に 1 を足して番号を選ぶ。
  それでも重なったときに備え、Merge Queue の `repo` ジョブで番号の重複を検査する。重なったら、後から並んだ PR が番号を付け直す。

### 置き換える決定

- ADR-0009 の「生成物をコミットする」を、Go の型付きアクセサだけに狭める（TS のアクセサと疑似ロケールはコミットしない）。
- ADR-0059 の「生成物はコミットし、`make web-tokens` が古いと失敗する」を、Go の `theme_tokens.gen.go` だけに狭める。
- ADR-0000 の「一覧（README.md）は `make adr-index` で再生成し、CI で差分を検査する」を、上の索引と番号の決め方に置き換える。
- ADR-0066 は変えない（バンドルは Go が読むのでコミットを続ける）。

### 覆す条件

- Go が読む生成物（sqlc・API・i18n）の衝突が、選択肢 2 にしたあとも並行の作業を止めるほど多いなら、選択肢 3 を考え直す
  （生成器を Go に移す。ADR-0066 の覆す条件と合わせて判断する）。
- コミットしない生成物の作り忘れで、手元やエディタの混乱が続くなら、TS の生成物も `go:generate` のように固定の手順で作る仕組みを考える。

## Consequences（影響）

- 良い影響:
  - 衝突の原因のうち、TS の生成物・疑似ロケール・ADR の索引による衝突（14 日で約 140 回）がなくなる。
  - 古い生成物を検査する検査（`i18n` の TS 側・`web-routes-check`・トークンの TS 側・`adr-index-check`）が要らなくなる。
  - PR の差分から、生成物の大きな変更が消える。
- 悪い影響・トレードオフ:
  - clone した直後や `git pull` の後は、`task generate`（か、生成を `deps` に持つタスク）を回すまで、エディタの TS の型検査が通らない。
  - API の TS の型やルートの木の変化は、PR の差分では見えなくなる。レビューは入力（`api/spec/`・`app/web/src/routes/`）で行う。
  - Go が読む生成物（sqlc・API・i18n の Go 側）の衝突は残る。これは ADR-0082 のファイルの分割で減らす。
  - README を見ても ADR の Status の一覧が分からない。
  - 疑似ロケールを使う Go のテストを、`testdata` のカタログに書き直す手間がかかる。
- 後続の作業（issue にする）:
  - TS の生成物と疑似ロケールを `.gitignore` に入れ、生成のタスクと `deps` を整える（ADR-0080 の移行と一緒に行う）
  - `i18n.WithPseudoLocales` のエラーと、疑似ロケールを使う Go のテストの書き直し
  - `docs/adr/README.md` の索引と `scripts/adr-index.sh` を消し、`agentkit adr list` と `scripts/adr-new.sh` の番号の選び方、番号の重複の検査を足す
  - AGENTS.md の「After adding/renaming an ADR, run `make adr-index`」と workflow.md §7・§9 の生成物の衝突の手順を書き直す
  - `app/web/vite.config.ts` のコメント（`routeTree.gen.ts` をコミットする理由）を書き直す
  - ADR-0000・ADR-0009・ADR-0059 の状態の行に、置き換えた部分を書く（この ADR を Accepted にするときに同じ PR で行う）

## Pros and Cons of the Options（詳細比較・任意）

- 1（今のまま）: 生成器を回さずに何でも動く。PR で生成の結果が見える。一方、衝突は形式を工夫しても残る（GitHub はマージドライバを使わない）。
  衝突のたびに、Merge Queue から外れて CI が 2 回回り直す。
- 2（Go が読む物だけ）: `go build` を Go だけで動かす制約を守ったまま、衝突の多い生成物の大部分をなくせる。TS 側には、ビルドの前に生成する場所（Vite・`tsc` の前のタスク）がもともとある。
- 3（一切コミットしない）: 生成物の衝突がなくなる。一方、Go の生成器を Go に移す（Redocly のバンドル、FormatJS の疑似ロケール、i18n の Go のアクセサ）か、Go のビルドに Bun を要るようにする必要がある。
  ADR-0066 の判断基準と、`go install` などの Go の慣習に反する。

## Open Questions / 要検証

- TanStack Router のルートの木を、ビルドせずに生成する手段（`@tanstack/router-cli` の `tsr generate`、または `@tanstack/router-generator` を直接呼ぶ）。`tsc` の前に回すのに要る。
- `internal/i18n` のテストのうち、`locales.FS` の疑似ロケールに依るものの範囲（`parity_test.go` と `golden.json`）。
