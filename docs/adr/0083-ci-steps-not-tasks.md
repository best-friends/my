# ADR-0083: CI のジョブはワークフローのステップで組み、go-task は手元の入口にする

- Status: Accepted
- Date: 2026-10-23
- Deciders: @rosylilly
- Issue: #1064

## Context（背景・課題）

[ADR-0080](0080-task-runner-go-task.md) は、CI のジョブの中身を `task ci:<job>` の 1 つの呼び出しにすると決めた。
[ADR-0079](0079-ci-single-gate-merge-queue.md) も「ジョブの中身は、ADR-0080 のタスク（`task ci:<ジョブ>`）1 つにする」としている。
ねらいは、手元と CI が同じ定義から同じ組を回すことだった。

PR #1069 で、Merge Queue の本検査を PR の上で初めて回した（`ci:full`、2026-10-23）。`server-static` は 754 秒かかり、ADR-0079 の見込み（4〜5 分）を大きく超えた。
ところが、ジョブのステップは `scripts/task.sh ci:server-static` の 1 つなので、Actions の画面からはどの検査が遅いのかが分からない。
CI では go-task の出力を `group` と `error_only` にしているので、成功した検査はログにも何も残さない。
検査ごとの所要時間と失敗の場所が見えないと、ADR-0079 の「覆す条件」（ジョブが 5 分を超えたら分ける）も判断できない。

決定者の指摘（チャット、原文）:

> `task ci:server-static` とか、 task にするんじゃなくて GitHub Actions の parallel step にしたほうがどのタスクでどれくらい時間かかってるかわかりやすいんじゃないでしょうか。 `task` はあくまでローカルでの開発支援に徹して、 CI はちゃんとそれように設計した方が賢いと思うんですよね。どうですか？

GitHub Actions は、1 つのジョブの中でステップを並列に動かせる（2026-06-25 から。`parallel`・`background`・`wait`・`wait-all`・`cancel`）。
並列のステップは、それぞれのログと実行を持つ。1 つのジョブで同時に動く background のステップは 10 まで。composite action の中では使えない
（[changelog](https://github.blog/changelog/2026-06-25-actions-steps-can-now-be-run-in-parallel)、[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)、2026-10-23 確認）。

決定者の決定（チャット、原文）:

> Accepted

## Decision Drivers（判断基準）

1. CI の画面で、検査ごとの所要時間・ログ・失敗の場所が分かる
2. 1 つの検査のコマンドは 1 か所で定義し、手元と CI で中身がずれない
3. CI はジョブの分け方・並列・キャッシュ・注釈（golangci-lint の PR への注釈など）を CI のために最適化できる
4. 手元の開発は 1 つのコマンド（`task check`・`task check:changed`）で済む（NFR-DEV-1）

## Considered Options（選択肢）

1. 今のまま（ADR-0080）。ジョブは `task ci:<job>` を 1 つ呼ぶ
2. ジョブの中身をワークフローの `parallel` のステップで組み、各ステップは末端のタスク（`go:vet` などの 1 つの検査）を 1 つ呼ぶ
3. ワークフローのステップに、タスクを通さずコマンドを直接書く

## Decision（決定）

**選択肢 2 を採る。**

- go-task は、各検査のコマンドの唯一の定義と、手元の入口（`check`・`check:changed`・領域ごとの `*:check`・生成・開発の起動）に徹する。
- CI のジョブは、ワークフローで組む。準備のステップ（checkout・`setup-go`・`setup-bun`・`setup-task`・生成）の後に、互いに依存しない検査を `parallel` のステップにする。
  各ステップは名前を検査の名前にして、末端のタスクを 1 つ呼ぶ（`scripts/task.sh go:vet`）。
- 公式の action が注釈やキャッシュで価値を足す検査は、その action をステップに使う（golangci-lint-action）。コマンドの版は `.tool-versions` から読み、タスクと同じ版にする。
- `ci:*` の集約タスクはなくす。CI 向けの `output: group` と `error_only` の設定もなくす（ステップごとにログが分かれる）。
- go-task の `deps` が保証していた順序は、ワークフローでも保つ。生成は並列のまとまりの前のステップにする。互いに干渉する検査は、同じまとまりに入れないか、`deps` で順序を持つ 1 つのタスクとして呼ぶ。
  干渉する検査の例: `web:lint-rules` は `app/web/src` に一時的なファイルを置く。golangci-lint は同時に 1 つしか動かない。
- 手元と CI の組み合わせのずれは、`repo` ジョブで検査する。`check` が含む末端のタスクがすべて、CI のどこかのステップで呼ばれているか、CI で回さない理由とともに除外の一覧にあるかを確かめる。
- ジョブの分け方（PR の preflight、Merge Queue の `server-static`・`server-test`・`client-static`・`client-test`・`repo`・`docker`）と、ジョブの数の上限（1 グループ 7）は ADR-0079 のまま。

置き換える決定: ADR-0080 の「CI のジョブ（ADR-0079）は `task ci:<job>` を 1 つ呼ぶ。`ci:*` はルートに置き、各領域のタスクを `deps` で並べるだけにする」と「出力: CI は `output: group` と `error_only: true` にし」、
ADR-0079 の「ジョブの中身は、ADR-0080 のタスク（`task ci:<ジョブ>`）1 つにする」。

### 覆す条件

- `parallel` のステップが GitHub-hosted runner で期待どおりに動かない（ログが分かれない、失敗が拾えない）なら、ジョブを検査ごとに分けるか、選択肢 1 に戻す。
- 組み合わせのずれの検査が保守の負担になるなら、検査をやめ、ずれはレビューで見る。

## Consequences（影響）

- 良い影響:
  - Actions の画面で、検査ごとの所要時間とログが見える。遅い検査を見つけ、ADR-0079 の「5 分を超えたら分ける」を数字で判断できる。
  - 失敗したステップの名前が、落ちた検査の名前になる。
  - golangci-lint の注釈など、CI のための action を素直に使える。
- 悪い影響・トレードオフ:
  - CI の組み方を、タスクの `deps` とワークフローの 2 か所で持つ。検査のコマンドは 1 か所のままだが、どれを並べるかは別々に保守する。
  - 手元で CI のジョブと同じ組を 1 つのコマンドで回す手段がなくなる。手元は `task check` か `task check:changed` を使う。
  - `parallel` は 2026-06 からの機能で、使われた実績が少ない。
- 後続の作業: #1069（epic #1064）の中で、`ci.yml`・`slow-suite.yml` のジョブをステップで組み直し、`ci:*` タスクを消し、組み合わせのずれの検査を `repo` ジョブに足す。ADR-0080 と ADR-0079 の状態の行に、置き換えた部分を書く。

## Open Questions / 要検証

- `parallel` のステップが Free プランの GitHub-hosted runner で使えること。#1069 の `ci:full` の実行で確かめる。
