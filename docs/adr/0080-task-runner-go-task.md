# ADR-0080: タスクの実行を Make から go-task に移す

- Status: Proposed
- Date: 2026-10-23
- Deciders: @rosylilly
- Issue: #1052

## Context（背景・課題）

検査・生成・ビルドは `Makefile`（418 行、説明付きのターゲット 68 個）にまとまっている。CI のワークフローも、ほとんどのステップで `make <target>` を呼ぶ（71 箇所）。
NFR-DEV-1 は「`make check` 一発で lint・型検査・テスト・ドキュメント検査が走る」と定めている。

今の `Makefile` には、並列にできる検査を直列に回してしまう形が残っている:

- `make check` は文脈（`docs-check`・`go-check` など）を並列に回す。ただし、文脈の中のステップは直列である（`check-run-%` が `MAKEFLAGS` を外して別の make で回す）。
  `go-check` の vet・lint・ビルド・テスト、`web-check` の `tsc`・Oxlint・knip・ビルド・Vitest は、互いに依存しないのに順に回る。
- ターゲットの多くは `.PHONY` で、入力のファイルを前提に持たない。そのため、生成（`i18n-gen`・`generate`・`sqlc`）が必要かどうかを make が判断できない。
  エージェントは衝突のたびに `regen` で全部を作り直している。
- make のファイルの新しさの判断は更新時刻で行う。`git checkout` や worktree の作成で時刻が揃うので、生成物の要否の判断には使いにくい。
- CI のジョブの中身は、`make a b c` の列やシェルのステップとして、ワークフローの YAML に散らばっている。手元で同じ組を回すには、YAML を読んで写す必要がある。

決定者の回答（チャット、原文）: タスクの実行は「go-task に移る」。

[ADR-0079](0079-ci-single-gate-merge-queue.md) は、CI のジョブをツールチェーンごとに数個にまとめ、ジョブの中を並列で埋める。そのためには、依存を表せて、並列に回せるタスクの実行器が要る。

## Decision Drivers（判断基準）

1. 互いに依存しない検査を、手元でも CI でも既定で並列に回せる
2. 生成のような入力から決まる作業を、入力が変わっていなければ飛ばせる。判断は更新時刻ではなく中身で行う
3. CI のジョブと手元のコマンドを同じ定義から回す（ワークフローにはツールの準備と 1 つのタスクの呼び出しだけを書く）
4. 依存とツールチェーンを増やしすぎない。AGPL-3.0-only と両立するライセンス（ADR-0011）、`CGO_ENABLED=0` のまま、Go と Bun のほかに新しい言語の実行環境を入れない
5. 並行して作業するエージェントが、同じファイルを編集して衝突しない（[ADR-0082](0082-conflict-free-file-layout.md)）
6. 移行の手間

## Considered Options（選択肢）

1. Make のまま。ターゲットに本当のファイルの前提を持たせ、文脈の中も `make -j` で並列にし、`mk/*.mk` に分ける
2. go-task（Taskfile）に移る
3. Bazel・Pants・moon・Nx などのビルドシステムに移る
4. agentkit（`tools/agentkit`）にタスクの実行を持たせる

## Decision（決定）

> 推奨案。決定者の回答は Context のとおり。

**選択肢 2（go-task）を推奨する。**

go-task は MIT ライセンスの Go 製の単一バイナリで、最新版は v3.54.0（2026-10-01）である（GitHub API で 2026-10-23 確認）。次の性質がこの ADR の判断基準に合う
（[Dependencies](https://taskfile.dev/docs/guide/dependencies)・[Up-to-date checks](https://taskfile.dev/docs/guide/up-to-date)・[Output](https://taskfile.dev/docs/guide/output)、2026-10-23 確認）:

- `deps` は既定で並列に動く。`--concurrency`（`-C`）で同時の数を絞れる。`--parallel` で、コマンドラインに並べたタスクも並列に回せる。
- `sources` と `generates` で入力と出力を宣言でき、既定の判断はチェックサム（`method: checksum`）で行う。指紋は `.task/` に置く（`.gitignore` に入れる）。
- 出力を `output: group` でコマンドごとにまとめられる。`error_only: true` にすると、失敗したコマンドの出力だけを出せる。
- `includes` で Taskfile を分け、名前空間を付けられる。

### 構成

```text
Taskfile.yml              ルート。includes と check・check:changed・generate・ci:* だけを書く
tasks/
  go.yml                  go:vet・go:lint・go:test・go:build・go:build-tags・go:licenses・go:mod・sqlc:*
  web.yml / ui.yml        web:*・ui:*（typecheck・lint・knip・build・test・ui-test・e2e）
  i18n.yml / api.yml      i18n:*・api:*（生成は sources / generates を宣言する）
  schema.yml / docker.yml / docs.yml / agentkit.yml
```

- 1 つのファイルに全部を書かず、領域ごとのファイルに分ける。新しい領域は新しいファイルとして足し、ルートには include を 1 行足すだけにする（ADR-0082）。
- `task check` は、すべての領域の検査を `deps` で並列に回す。NFR-DEV-1 は「`task check` 一発で〜」に書き換える。
- `task check:changed` は、今の `make check-changed` と同じく `scripts/changed-contexts.sh` で文脈を選び、その検査を並列に回す。
- `task generate` は、すべての生成（[ADR-0081](0081-generated-files-policy.md)）を回す。入力が変わっていない生成は飛ぶので、
  検査とビルドのタスクは、毎回 `generate` の該当のタスクを `deps` に持てる。衝突のあとに全部を作り直す手順（agentkit の `regen`）は要らなくなる。
- CI のジョブ（ADR-0079）は `task ci:<job>` を 1 つ呼ぶ。`ci:*` はルートに置き、各領域のタスクを `deps` で並べるだけにする。
- 同時の数: CI では制限しない（runner の 4 vCPU を使い切る）。手元では、`task check` の既定を CPU の数の半分にする（`TASK_CONCURRENCY` などの環境変数か Taskfile の変数で上書きする。要検証）。
  1 台のマシンで 4〜5 個のエージェントが同時に検査を回すからである。
- 出力: CI は `output: group` と `error_only: true` にし、落ちたタスクの出力だけを見せる。手元は `prefixed` にする。
- go-task の版は `tools/go.mod` の `tool` で固定し、`scripts/task.sh` が `bin/task` をビルドして呼ぶ（`scripts/agentkit.sh` と同じ形）。
  CI もこのスクリプトで導入する。GitHub の `go-task/setup-task` は使わない（版の固定が `tools/go.mod` と二重になり、手元では使えないから）。
- `Makefile` は消す。v0 は互換の層を残さない（[ADR-0012](0012-v0-breaking-change-policy.md)）。文書の `make <target>` は、同じ PR で `task <name>` に書き換える。

### 覆す条件

- go-task の開発が止まる、またはライセンスが変わるなら、選択肢 1 に戻す。Taskfile のタスクは名前と依存がそのまま make のターゲットに写せる。
- チェックサムによる飛ばしが原因で、古い生成物で検査が通る事故が起きるなら、生成のタスクを `method: none`（毎回実行）にする。

## Consequences（影響）

- 良い影響:
  - 文脈の中の検査も並列に回る。`go:vet`・`go:lint`・`go:build-tags` は同時に動き、`tsc`・Oxlint・knip・`vite build`・Vitest も同時に動く。
  - 生成は入力が変わったときだけ走る。衝突したあとの作り直しが、`task check:changed` の中で自然に済む。
  - CI のジョブと手元のコマンドが同じ定義から回る。ワークフローの YAML は薄くなる。
- 悪い影響・トレードオフ:
  - コントリビューターは make の代わりに go-task を使う。`scripts/task.sh` で Go から作れるが、慣れた `make` は使えなくなる。
  - 68 個のターゲット、ワークフローの 71 箇所、文書の約 50 ファイルを一度に書き換える。移行の PR は大きい。
  - `.task/` のチェックサムが手元の状態を持つ。おかしくなったら `task --force` で回し直す。
  - CPU を食う Go の検査（golangci-lint と `go test`）を並列に回しても、4 vCPU では短くなる幅が小さいことがある。CI のジョブは ADR-0079 で静的な検査とテストに分けたままにする。
- 後続の作業（issue にする）:
  - `Taskfile.yml` と `tasks/*.yml` を作り、`Makefile` を消す。`scripts/task.sh` と `tools/go.mod` の `tool` を足す
  - ワークフロー・AGENTS.md・CONTRIBUTING.md・[commands.md](../reference/commands.md)・workflow.md・`.claude/skills`・agentkit（`check` などの呼び出し）の `make` を書き換える
  - NFR-DEV-1 の文を書き換える
  - `tools/agentkit/context-map.yaml` に `Taskfile.yml` と `tasks/` を足す

## Pros and Cons of the Options（詳細比較・任意）

- 1（Make のまま）: 新しい依存がなく、誰でも知っている。`make -j` とファイルの前提で、並列と生成の要否はある程度表せる。
  一方、更新時刻による判断は checkout で崩れる。`.PHONY` と本物のファイルの前提が混ざるとわかりにくくなる。並列の出力のまとまりと失敗の扱い（`--output-sync`・`--keep-going`）を自分で組む必要があり、今の `check-run-%` のような仕組みが要る。
- 2（go-task）: 並列・チェックサム・まとまった出力・名前空間を最初から持つ。Go 製で、`tools/go.mod` で版を固定できる。YAML の中のシェルは、make の `$$` のような二重のエスケープが要らない。
- 3（Bazel など）: 完全な依存のグラフ、リモートキャッシュ、テストの選択ができる。一方、Go と Bun のルールの保守、学習、CI の準備の重さが、このリポジトリの規模に合わない。
- 4（agentkit）: すでに Go で書かれ、リポジトリの文脈を知っている。一方、エージェント向けの道具がビルドの要になる。並列・キャッシュ・出力を自前で作ることになる。

## Open Questions / 要検証

- 手元の既定の同時の数を、環境変数で切り替えられる形（Taskfile の変数で `-C` を組むか、ラッパーの `scripts/task.sh` で付けるか）。
- `sources` に大量のファイル（`app/**/*.ts` など）を宣言したときに、チェックサムの計算にかかる時間。
