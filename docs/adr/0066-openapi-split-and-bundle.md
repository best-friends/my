# ADR-0066: クライアント API の仕様をファイルに分けてバンドルする

- Status: Proposed
- Date: 2026-10-22
- Deciders: @rosylilly
- Issue: #868

## Context（背景・課題）

クライアント API は OpenAPI 3.1 のスキーマファーストで、[ADR-0016](0016-client-api-style.md) は「`api/openapi.yaml` を正とし、Go のサーバーインターフェイスと TS クライアントを生成する」、
[ADR-0029](0029-api-code-generation.md) は「API を変えるときは、`api/openapi.yaml` を先に編集する」と決めている。
[client-api.md §1](../architecture/client-api.md#1-ファイルの配置) は spec を分割せず 1 ファイルに書き、バンドルの工程を持たないとしている。
今の仕組みでは、次のものが `api/openapi.yaml` を読む。

- `scripts/generate.sh`（`make generate` / `make generate-check`）: oapi-codegen v2.8.0（`api.gen.go`）、`securitygen`（`security.gen.go`）、openapi-typescript 7.13.0（`app/web/src/lib/api/schema.gen.ts`）
- `make api-lint`: Redocly CLI 2.57.0 と `api/redocly.yaml`
- `api/api.go`: `//go:embed openapi.yaml` で埋め込み、`internal/interface/clientapi/apitest` が kin-openapi で読んで応答を照合する

このファイルは 10,697 行・約 409KB で、パスが 152、`components` が 319（`schemas` 268、`responses` 29、`parameters` 21、`headers` 1）ある。
2026-10-01 以降の main の 1,583 コミットのうち 100 がこのファイルを変えており、並行作業（[workflow.md §9](../agents/workflow.md#9-parallel-batches)）の競合の多くがここで起きる。
競合を減らすための `# --- ... (#621) ---` のような区切りのコメントが 224 行ある。エージェントも人もファイル全体を一度に読めず、検索と部分読みに頼っている。
決定者は「@api/openapi.yaml が1ファイルなのに巨大すぎるので分割するべきじゃないかと思えてきたんですがどうでしょうか」と提案し、「進めてください」と判断した（#868）。

スパイク（[#868 の報告](https://github.com/best-friends/my/issues/868#issuecomment-6319895238)、ブランチ `spike/868-openapi-split`）で、
今の spec を `redocly split` で分け、`redocly bundle` で 1 ファイルに戻し、生成物と検査が変わらないかを確かめた（2026-10-22、`origin/main` 876396d9）。

| 確かめたこと | 結果 |
| --- | --- |
| ルートが `components` を列挙しない分割（`redocly split` の出力そのまま）のバンドル | どの操作からも参照されない 4 つのスキーマ（`WebhookMessage` など、ADR-0060）が落ちる |
| ルートがすべての `components` を `$ref` で列挙する分割のバンドル | データとして元のファイルと一致する（パスと各 `components` の順序も同じ。参照は元の名前の `#/components/...` に戻る） |
| バンドルを `api/openapi.yaml` に置いた `make generate-check` | 通る（`api.gen.go`・`security.gen.go`・`schema.gen.ts` に差分なし） |
| バンドルを埋め込んだ `go test ./internal/interface/clientapi/...` | 通る |
| 分割したファイルへの `redocly lint`（今の `api/redocly.yaml`） | `rule/default-response-is-problem` だけが 191 件失敗する（参照が `../components/responses/Problem.yaml` になるため）。正規表現をファイルの参照に合わせると通る |
| oapi-codegen v2.8.0 を分割したルートに直接使う | `fatal error: stack overflow` で失敗する |
| openapi-typescript 7.13.0 を分割したルートに直接使う | コミット済みの `schema.gen.ts` と同じ出力 |
| kin-openapi v0.149.0 の `Loader`（`IsExternalRefsAllowed`）で分割したルートを読む | 読めて、検証も通る |
| `redocly bundle` の時間 | バンドル自体は約 0.1 秒（`bunx` の起動を除く） |

- `redocly split` は YAML のコメントをすべて捨て、`description: |` を `>` と空行の形に書き換える。`redocly bundle` もコメントを捨てる。
- oapi-codegen は、別ファイルへの `$ref` を `import-mapping` で別の Go パッケージ（または同じパッケージの別の spec）に対応させる方式を取る
  （[README「Splitting large OpenAPI specs across multiple packages」](https://github.com/oapi-codegen/oapi-codegen#splitting-large-openapi-specs-across-multiple-packages-aka-import-mapping-or-external-references)）。
  1 つの spec を 1 つのパッケージに生成する今の構成では、バンドルした 1 ファイルを渡す必要がある。
- OpenAPI 3.1 は Path Item Object に `$ref` を許し（[OpenAPI 3.1.0 §4.8.9](https://spec.openapis.org/oas/v3.1.0#path-item-object)）、
  `components` の各項目は Reference Object にできる。`redocly bundle` は外部の `$ref` をたどって 1 ファイルにまとめる（[Redocly CLI `bundle`](https://redocly.com/docs/cli/commands/bundle)）。

## Decision Drivers（判断基準）

1. 並行作業で spec の競合が起きにくい。起きても機械的に解ける
2. エージェントと人が、操作 1 つ・スキーマ 1 つを、その部分だけ読んで書ける。ファイルの名前から場所が決まる
3. Go と TS の生成物、`apitest` の照合、lint の検査が今と同じ結果になる（ADR-0029 の生成ツールを変えない）
4. 新しい依存やツールを増やさない。`go build` / `go test` が Bun なしで動く（Go の CI は Bun を入れない）
5. 第三者や文書が 1 ファイルの spec を `blob/main/api/openapi.yaml` で読める

## Considered Options（選択肢）

分け方の単位と、バンドルの扱いの 2 つの軸がある。

1. 分割しない。1 ファイルのまま、区切りのコメントや並び順の規則で競合を減らす
2. パス 1 つ（Path Item）・`components` の項目 1 つを 1 ファイルにし、Redocly CLI でバンドルした `api/openapi.yaml` をコミットする
3. タグ（モジュール）ごとに 1 ファイルにまとめ（`paths/statuses.yaml` に複数のパス、`schemas/statuses.yaml` に複数のスキーマ）、バンドルをコミットする
4. 2 と同じ単位で分け、バンドルはコミットせず生成のたびに作る。`api/api.go` は分割したファイルを `embed.FS` で埋め込み、kin-openapi が `$ref` をたどる

## Decision（決定）

> Proposed の間は「推奨案」として書く。Accepted にするのは人間の Decider のみ。

**推奨案は選択肢 2。** パス 1 つ・`components` の項目 1 つを 1 ファイルにした `api/spec/` を正とし、
Redocly CLI の `bundle` で作った `api/openapi.yaml` を生成物としてコミットする。生成器・`apitest`・第三者はバンドルを読む。

### ファイルの配置

```text
api/
  spec/
    openapi.yaml                        # ルート: info・servers・security・tags と、paths・components の $ref の一覧
    paths/<パス>.yaml                    # Path Item 1 つ。名前はパスの先頭の / を除き / を _ にしたもの（/statuses/{id}/favourite → statuses_{id}_favourite.yaml）
    components/schemas/<名前>.yaml       # スキーマ 1 つ。ファイル名は components の名前と同じ
    components/responses/<名前>.yaml
    components/parameters/<名前>.yaml
    components/headers/<名前>.yaml
  openapi.yaml                          # 生成物（redocly bundle api/spec/openapi.yaml）。手で編集しない
  redocly.yaml                          # lint の設定。api/spec/openapi.yaml を検査する
  api.go                                # openapi.yaml（バンドル）を埋め込む
```

- 名前の規則は `redocly split` の出力と同じにする。パスやスキーマの名前からファイルが 1 つに決まる。
- `securitySchemes` はルートに置く（1 つしかなく、変わらない）。
- 分割したファイルはそれぞれ `# SPDX-License-Identifier: AGPL-3.0-only` で始める。

### `$ref` の書き方

- ルートは、`paths` のパスごとに `$ref: ./paths/<パス>.yaml` を、`components` の種類ごとにすべての項目を `<名前>: {$ref: ./components/<種類>/<名前>.yaml}` で列挙する。
  列挙しない項目は、どこからも参照されないとバンドルから落ちる（スパイクの `WebhookMessage`）。
- パスと `components` のファイルは、`components` を相対パスのファイルで参照する（`../components/schemas/Status.yaml`、同じ種類の中では `./Status.yaml`）。
  ルート以外のファイルで `#/components/...` は書かない（そのファイル自身を指すため）。
- 1 ファイルは 1 つのオブジェクトで、JSON Pointer の断片（`file.yaml#/...`）で他のファイルの中を指さない。パスのファイルは他のパスのファイルを参照しない。
  リモートの URL への `$ref` は書かない。
- 2 か所以上で使う型は `components` の 1 ファイルにする（今の規約と同じ）。
- ルートの `paths` はタグのまとまりの順に並べ、新しいパスはそのタグのまとまりの末尾に足す。`components` は種類ごとに名前の順に並べる。
- YAML のコメントはバンドルで消える。クライアントに伝えることは `description` に書き、コメントは保守のための短い注記だけにする。区切りのコメントは置かない。

### 生成と検査

- `make generate`（`scripts/generate.sh`）は、まず `api/spec/openapi.yaml` を Redocly CLI（Makefile で固定した版、今は 2.57.0）で `api/openapi.yaml` にバンドルし、
  「生成物であり `api/spec/` を編集する」旨の SPDX 付きの見出しを付ける。続けて、今と同じ 3 つの生成をバンドルから行う。
- `make generate-check` は、バンドルも一時ディレクトリに作り、コミット済みの `api/openapi.yaml` との差分を他の生成物と同じように検査する。
- `make api-lint` は `api/spec/openapi.yaml` を lint し、問題の場所を分割したファイルで示す。
  `rule/default-response-is-problem` の正規表現は `../components/responses/Problem.yaml` への参照に合わせる。
  ルートが列挙しないファイルが `api/spec/components/` にあれば失敗させる検査を `make api-check` に足す（バンドルから黙って落ちるのを防ぐ）。
- `api/api.go`・`apitest`・`securitygen`・oapi-codegen と openapi-typescript の設定は変えない。新しい依存は足さない（Redocly CLI は lint ですでに使う）。
- `.gitattributes` で `api/openapi.yaml` に `linguist-generated=true` を付け、PR の差分ではたたむ。競合したときは手でマージせず `make generate` で作り直す（他の生成物と同じ）。

### ADR-0016・ADR-0029 との関係

この ADR は次の文だけを置き換える。両 ADR のその他の決定（REST + OpenAPI 3.1、生成ツール、CI での差分検査・lint・照合、使える 3.1 の部分集合）は変えない。

- ADR-0016 Decision「`api/openapi.yaml` を正とし、Go のサーバーインターフェイスと TS クライアントを生成する」のうち、正とするファイル:
  `api/spec/` の分割したファイルを正とし、バンドルした `api/openapi.yaml` から生成する。
- ADR-0016 Consequences「API の変更はまず `openapi.yaml` を編集する」と ADR-0029 Consequences「API を変えるときは、`api/openapi.yaml` を先に編集する」:
  `api/spec/` を先に編集し、`make generate` でバンドルと生成物を同じ PR で更新する。

両 ADR の本文は書き換えない。部分的な置き換えをどう記録するか（両 ADR を Accepted のまま本 ADR を参照するか、状態を変えるか）は決定者が決める。

**覆す条件**: oapi-codegen が分割したファイルを 1 つのパッケージに直接生成でき、kin-openapi と openapi-typescript も同じ結果になるなら、選択肢 4（バンドルをコミットしない）を再検討する。
バンドルが元の spec と食い違う不具合が Redocly CLI に見つかり、版を戻しても避けられないなら、選択肢 1 に戻す。

## Consequences（影響）

- 良い影響
  - 操作やスキーマを足す PR は、新しいファイルとルートの数行だけを変える。別のリソースを変える PR どうしは、ルートの隣り合う行を除いて競合しない。
  - ファイルの名前から場所が決まり、エージェントは必要なファイルだけを読める。
  - 生成物・`apitest`・第三者が読む `api/openapi.yaml` は今と同じ内容なので、生成ツールと Go のコードは変わらない。
- 悪い影響・トレードオフ
  - 生成物の `api/openapi.yaml` は今と同じように競合する。解き方は `make generate` での作り直しで機械的だが、なくなりはしない（`api.gen.go`・`schema.gen.ts` も同じ）。
  - PR の差分は、編集したファイルとバンドルの両方に出る。
  - 約 470 ファイルになり、ルートは 470 行ほどの `$ref` の一覧になる。spec 全体の検索は `api/openapi.yaml` で行える。
  - `make generate` が `bunx` で Redocly CLI を取得する（`make api-lint` と同じ）。オフラインでは、キャッシュがないと生成できない。
  - 分割への移行の時点で開いている、`api/openapi.yaml` を変える PR は、変更を分割したファイルに移し直す必要がある。
- 後続で必要になる作業（Issue 化する）
  - 移行: `redocly split` を基に `api/spec/` を作る。コメントのうち残すものを移し、ブロックスカラーの書き方を戻し、SPDX の見出しとルートの `components` の一覧を足す。
    元の `api/openapi.yaml` とデータとして一致し、`make generate-check` が差分なしで通ることを確かめる（パスと `components` の順序を保つ）。
    続く別のコミットで `components` を名前の順に並べ替え、`make generate` の差分（`schema.gen.ts` の順序だけ）をコミットする。並行のバッチの区切りで行う。
  - `scripts/generate.sh`・Makefile の `api-lint` と未列挙のファイルの検査・`api/redocly.yaml` の正規表現・`.gitattributes`。
  - 文書: [client-api.md](../architecture/client-api.md) の §1（配置と「分割しない」）と §3（追加の手順）、AGENTS.md の Commands の `make generate` の行、
    [CONTRIBUTING.md](../../CONTRIBUTING.md)、[overview.md §6](../architecture/overview.md#6-リポジトリ構成) の `api/` の行、
    [workflow.md](../agents/workflow.md) の生成物の源の一覧、[web-client.md](../architecture/web-client.md) の `api/openapi.yaml` への言及（バンドルを指すなら残す）。

## Pros and Cons of the Options（詳細比較・任意）

| 観点 | 1. 分割しない | 2. 項目ごと + バンドルをコミット（推奨） | 3. タグごと + バンドルをコミット | 4. 項目ごと + バンドルをコミットしない |
| --- | --- | --- | --- | --- |
| 競合 | 今のまま多い | 新しいファイルとルートの数行。バンドルは作り直し | 同じタグの PR どうしで競合する（`admin-server` は 54 操作） | 分割したファイルだけ。最も少ない |
| 読みやすさ | 10,697 行を部分読み | 1 ファイル 1 項目。名前から場所が決まる | タグごとに数百〜数千行。参照に `paths/x.yaml#/~1statuses~1{id}` の JSON Pointer が要る | 2 と同じ |
| 生成と照合 | 今のまま | 今のまま（バンドルを読む） | 今のまま | oapi-codegen にはバンドルが要る。`apitest` は kin-openapi が `$ref` をたどり、生成とは別の解決器で読む |
| `go test` と Bun | 不要 | 不要（バンドルはコミット済み） | 不要 | `apitest` は不要だが、生成は一時ファイルのバンドルに頼る |
| 第三者が読む 1 ファイル | ある | ある | ある | ない（リリースや配信で別に用意する） |
| 道具 | なし | Redocly CLI（既存） | Redocly CLI（既存） | Redocly CLI（既存）と `embed.FS` |

- 選択肢 1 の一番の利点は、仕組みが増えないことと、ADR-0016・ADR-0029 の文がそのまま使えること。ファイルの大きさと競合は解けない。
- 選択肢 3 はファイルの数が少なく（約 30）、関連する操作を並べて読める。同じタグに PR が集まる今の作り方（管理画面の作業など）では競合が残り、JSON Pointer の参照は読みにくい。
- 選択肢 4 の一番の利点は、生成物の競合もなくなること。ただし、バンドルの解決器（Redocly）と照合の解決器（kin-openapi）が別になり、食い違いを検査する手段がない。
  第三者や文書から 1 ファイルの spec も失う。生成物の Go と TS の競合は残るので、バンドルをコミットしないことで減る手間は小さい。

## Open Questions / 要検証

- ADR-0016・ADR-0029 の部分的な置き換えの記録の仕方（両 ADR を Accepted のまま本 ADR から参照する／状態に注記を足す）。決定者が決める。
- 分割したファイルを置くディレクトリ名（`api/spec/` と `api/src/` など）。
- 移行の時期。開いている PR が少ないバッチの区切りに行う前提で、移行の issue で決める。
