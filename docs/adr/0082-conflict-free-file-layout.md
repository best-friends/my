# ADR-0082: 並列の作業で衝突しないファイルの構成

- Status: Accepted
- Date: 2026-10-23
- Deciders: @rosylilly
- Issue: #1052

## Context（背景・課題）

並行の作業（[workflow.md §9](../agents/workflow.md#9-parallel-batches)）では、多くの機能が同じファイルに少しずつ追記する。
直近 14 日で PR のマージ 235 件に対し、ブランチに main をマージしたコミットが 138 件あった。衝突の解決で変わった手書きのファイルの上位は次のとおり:

| ファイル | 衝突の回数 | 大きさ | 追記の仕方 |
| --- | --- | --- | --- |
| `docs/architecture/web-client.md` | 46 | 1,180 行（約 9 万トークン）、19 節 | 画面ごとの節に追記する |
| `locales/{en,ja}/client.json` | 各 36 | `en` は 12,294 行、キーは約 3,000（うち `admin.*` が 1,816） | キーを辞書順の位置に足す |
| `internal/platform/app.go` | 23 | 1,217 行。`NewApp` だけで約 470 行 | 機能ごとの配線を `NewApp` に足す |
| `docs/architecture/data-model.md` | 22 | 1,814 行（約 9 万トークン）、11 節 | 表の節に追記する |
| `schema/sqlite.sql`・`schema/postgres.sql` | 各 14 | 各約 3,150 行 | 表を足す |
| `AGENTS.md` | 12 | — | 規則を足す |
| `Makefile` | 11 | 418 行 | ターゲットを足す |

ファイルが大きいほど、2 つの PR が近い行を変える確率が上がる。衝突は Merge Queue からの離脱と CI のやり直しを生む（[ADR-0079](0079-ci-single-gate-merge-queue.md)）。
生成物の衝突は [ADR-0081](0081-generated-files-policy.md) が、`Makefile` は [ADR-0080](0080-task-runner-go-task.md)（`tasks/*.yml` に分ける）が扱う。この ADR は手書きのファイルの構成を決める。

workflow.md §9 は、同じ共有のファイルに触る issue を同時に走らせないことで衝突を避けている。
ただ、`web-client.md`・`client.json`・`app.go` はほとんどの機能が触るので、この規則は並行の数を大きく減らす。

関係する要件は NFR-DEV-1・NFR-I18N（[ADR-0009](0009-internationalization.md) のカタログの形）。

決定者の決定（チャット、原文）:

> 全 ADR Accepted

## Decision Drivers（判断基準）

1. 別の機能を作る 2 つの PR が、同じファイルの近い行を変えない
2. 読む人とエージェントが、目的の部分を探しやすい（agentkit の `show`・`find`、文書の節の参照）
3. 既存の参照（文書の節へのリンク約 860 箇所、`data-model§4.4` の形の参照）を壊さずに移せる
4. ツール（Atlas・FormatJS・翻訳のプラットフォーム・`go:embed`）がそのまま使える
5. 移行の手間

## Considered Options（選択肢）

1. 今のまま。workflow.md §9 で、共有のファイルに触る issue を同時に走らせない
2. 追記の多いファイルを、機能や節ごとのファイルに分ける
3. 2 に加えて、追記型のファイルに「1 項目 1 ファイル」を一般の規則として課す（新しい共有のファイルを作るときの基準にする）

## Decision（決定）

**選択肢 3 を採る。**

### 規則

- 多くの機能が追記する一覧・カタログ・配線・設計文書は、機能（または節）ごとのファイルに分ける。1 つの PR が足すものは、新しいファイルか、その機能のファイルに入るようにする。
- 分けたファイルを束ねる側（索引・include・`embed`）は、ディレクトリを読む形にして、ファイルを足しても束ねる側を書き換えずに済むようにする。
  束ねる側にどうしても 1 行足す必要があるなら（Go の呼び出しなど）、その行は機能の名前の辞書順に並べる。
- 新しい共有のファイルを作るときは、この規則に合うかをレビューで確かめる。

### 対象

| 今 | 分けた後 | 束ねる側 |
| --- | --- | --- |
| `locales/<locale>/<namespace>.json` | `locales/<locale>/<namespace>/<area>.json`。`area` はキーの先頭の区切り（`settings`・`compose` など）。`admin` は 2 つ目の区切りまで（`admin.domain_actions` → `admin/domain_actions.json`） | `tools/i18n` のローダーと `locales.go` の `go:embed` がディレクトリを読む。キーは変えない |
| `docs/architecture/web-client.md` | `docs/architecture/web-client/<NN>-<slug>.md`（今の節ごと。`14-admin.md` など） | `web-client/README.md` に節の一覧と、節をまたぐ原則（今の §1〜§7）を置く |
| `docs/architecture/data-model.md` | `docs/architecture/data-model/<NN>-<slug>.md`（同上） | `data-model/README.md` |
| `internal/platform/app.go` の `NewApp` | `internal/platform/setup_<module>.go` ごとに、モジュールの配線を `(a *App) setUp<Module>(...)` にまとめる | `NewApp` はモジュールの `setUp*` を依存の順に呼ぶだけにする |
| `schema/<dialect>.sql` | `schema/<dialect>/<NN>-<module>.sql`（番号は外部キーと索引の依存の順） | Atlas にはディレクトリ（`file://schema/<dialect>`）をそのまま渡す。Go の埋め込みは同じ名前の順に連結する |

- カタログの分け方の実測: `en` の `client`（約 3,000 キー）をこの規則で分けると 70 ファイルになり、最大は `settings` の 352 キー、中央値は 30 キーだった。
  14 日間の衝突でカタログの中の変わった行は、`admin/report`・`admin/account`・`admin/accounts`・`account_status`・`admin/appeal` に集まっていた。
  そのため、`admin` は 2 つ目の区切りまで分ける。
- Atlas Community Edition 1.3.0 は、`file://` にディレクトリを渡すと、中の SQL ファイルを名前の順に dev データベースで実行して desired state を作る。
  今の `schema/sqlite.sql` と `schema/postgres.sql` を `CREATE TABLE` の区切りで 3 ファイルに分けて `schema inspect` で比べると、両方言とも 1 ファイルのときと同じ結果になった。
  名前の順を逆にすると、PostgreSQL では後のファイルの表を参照する索引で失敗した（2026-10-23）。番号の順は、`db-check` でそのまま検査できる。
- 翻訳のプラットフォームの候補（ADR-0009）は、どちらも分けたカタログを扱える。Weblate は Component discovery のアドオンが、
  名前付きのグループ（`language`・`component`）を持つ正規表現でファイルを探し、コンポーネントを自動で足し引きする（[Add-ons](https://docs.weblate.org/en/latest/admin/addons.html)）。
  Crowdin は `crowdin.yml` の `source` に `**` を、`translation` に `%locale%` を書ける（[Configuration file](https://support.crowdin.com/configuration-file/)）。いずれも 2026-10-23 確認。
- 節の参照: agentkit の `show web-client§14.2` と `data-model§4.4` は、節の番号から分けたファイルを引くようにする。
  文書の中のリンク（`web-client.md#14-管理画面` など）は、移行の PR でスクリプトを使って新しいファイルとアンカーに書き換える。
- `AGENTS.md`（12 回）と機能要件の表（`01-functional.md`、5 回）は分けない。規則や要件の追加は決定者の判断を伴い、並行して変わることが少ないからである。

### 覆す条件

- 分けた設計文書を読むのに、エージェントが読むファイルの数と手間がかえって増えるなら、節の大きさを見直す。

## Consequences（影響）

- 良い影響:
  - 衝突の多い手書きのファイル（14 日で約 190 回分）の大部分が、機能ごとのファイルに散る。workflow.md §9 の「共有のファイルに触る issue を同時に走らせない」の対象が減り、並行の数を保ちやすくなる。
  - 設計文書が節ごとのファイルになり、エージェントが 9 万トークンのファイルを部分的に読む工夫が要らなくなる。
  - カタログが機能ごとのファイルになり、機能を消すときにまとめて消せる。
- 悪い影響・トレードオフ:
  - ファイルの数が増え、機能をまたぐ変更では開くファイルが増える。
  - 文書の節へのリンク約 860 箇所を書き換える。外から（issue や PR のコメントから）張られたリンクは古くなる。
  - スキーマのファイルの番号の順を、外部キーの依存に合わせて保つ必要がある。番号のずれは `db-check` で見つかる。
  - `NewApp` の呼び出しの並びには、まだ 1 行ずつ追記が起きる（衝突は辞書順の並びで減るが、なくならない）。
- 後続の作業（issue にする。それぞれ独立の PR にし、並行の作業の少ない時間に入れる）:
  - ロケールのカタログの分割と、`tools/i18n`・`locales.go`・`internal/i18n` のローダーの対応
  - `web-client.md` と `data-model.md` の分割、リンクの書き換え、agentkit の節の参照の対応、`context-map.yaml` の更新
  - `internal/platform` の配線の分割
  - スキーマの分割と、`task schema:*` のディレクトリの指定・Go の埋め込みの連結
  - workflow.md §9 の「共有のファイル」の一覧を、分けた後の構成に合わせて書き直す

## Pros and Cons of the Options（詳細比較・任意）

- 1（今のまま）: 何も移さずに済む。一方、ほとんどの機能が触るファイルが共有のファイルなので、並行の数がそのファイルで決まる。
- 2（対象のファイルだけ分ける）: 今の衝突は減る。一方、新しい一覧やカタログが 1 本のファイルとして生まれ、同じ問題を繰り返す。
- 3（規則にする）: 2 の効果に加え、新しく作る共有のファイルにも同じ形を求められる。規則がレビューの観点として 1 つ増える。

## Open Questions / 要検証

- なし。
