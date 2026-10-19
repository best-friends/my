# ADR-0047: CI の検査を PR・Merge Queue・main・nightly に振り分け、文脈ごとのワークフローに分ける

- Status: Accepted
- Date: 2026-10-19
- Deciders: @rosylilly
- Issue: #553

## Context（背景・課題）

CI は `ci.yml` の 1 本で、PR（`pull_request`）・Merge Queue（`merge_group`）・main への push のどこでも、ほとんど同じジョブを回している。
`changes` ジョブ（`scripts/changed-contexts.sh`）が PR と Merge Queue では変更のない文脈のジョブを飛ばすが、main への push ではすべての文脈を回す。
必須チェックは `check` の 1 つで、`docs`・`go`・`go-test`・`i18n`・`api`・`ui`・`web`・`docker`・`shell` の結果を集める。
`schema`・`ui-test`・`docker-smoke` も PR と Merge Queue で動くが、`check` に入っていないので、落ちてもマージを止めない。

決定者の要望（#553、原文）:

> PR, MergeQueue, Main で実行されるべき CI の内容を精査したい
>
> 今はどこでもほとんど同じモノが実行されているが、ほぼすべてを実行するのはリソースの無駄だし、なにより時間がかかる。ci.yml 単体からの脱却も踏まえて files などの絞り込みなども活用する前提で各ステップにおいて何を守る必要があり、どこで防御されるのかを明確にしたい。特に E2E などは実行時間も長く、現在の v0 運用において毎マージで実行しているのではどれだけリソースがあっても足りないし、もっと並列化して実行できるようにしたほうが効率がいいはず。

関係する要件は NFR-DEV-1（`make check` 一発で検査が走る）と NFR-DEV-2（外部サービスなしで検査が完結する）。
CI の構成は ADR-0015・ADR-0027・ADR-0040・ADR-0041・ADR-0046 が決めている。

### 実測（2026-10-19）

`gh run list` と `gh run view --json jobs` の `startedAt` / `completedAt` から測った。
対象は、pgmem（ADR-0041）と文脈ごとのジョブ分けが入ったあとの 2026-10-19 06:10〜08:50 UTC の実行（PR 53 回、merge group 7 回、main への push 23 回）。
main の取り消しの割合と main の E2E は、文脈ごとのジョブ分けが入った 02:10 UTC からの 49 回の push で数えた。
秒の値は「中央値 / p90」。PR の列の括弧は、53 回のうちそのジョブが動いた回数。

| ワークフロー / ジョブ | 起動 | 絞り込み | `check` | PR | Merge Queue | main への push |
| --- | --- | --- | --- | --- | --- | --- |
| `ci.yml` `changes` | PR・MQ・push | なし | ○ | 6 / 7 | 8 / 8 | 5 / 6 |
| `docs` | PR・MQ・push | docs | ○ | 15 / 18（48） | 17 / 17 | 14 / 16 |
| `go`（go-test 以外の go-check） | PR・MQ・push | go | ○ | 151 / 192（37） | 177 / 192 | 93 / 344 |
| `go-test`（pgmem） | PR・MQ・push | go | ○ | 103 / 134（37） | 106 / 114 | 40 / 155 |
| `i18n` | PR・MQ・push | i18n | ○ | 19 / 24（35） | 17 / 20 | 22 / 25 |
| `api` | PR・MQ・push | api | ○ | 60 / 72（34） | 61 / 63 | 59 / 62 |
| `ui`（ui-check） | PR・MQ・push | ui | ○ | 48 / 54（35） | 48 / 49 | 46 / 50 |
| `web`（web-check） | PR・MQ・push | web | ○ | 220 / 244（40） | 247 / 260 | 185 / 249 |
| `docker`（docker-check） | PR・MQ・push | docker | ○ | 6 / 7（34） | 6 / 8 | 6 / 7 |
| `shell` | PR・MQ・push | shell | ○ | 7 / 8（34） | 7 / 8 | 6 / 8 |
| `check` | PR・MQ・push | `always()` | 必須 | 3 / 4 | 3 / 3 | 3 / 3 |
| `schema (postgres)` / `schema (sqlite)` | PR・MQ・push | schema | × | 21 / 27・7 / 9（34） | 23 / 27・7 / 8 | 22 / 27・8 / 9 |
| `ui-test` | PR・MQ・push | ui | × | 204 / 229（35） | 204 / 229 | 190 / 216 |
| `docker-smoke` | PR・MQ・push | go・web・i18n・docker | × | 172 / 186（43） | 179 / 183 | 176 / 182 |
| `postgres-contract`（`postgres-contract.yml`） | push | なし | × | — | — | 130 / 210 |
| `web-e2e`（`web-e2e.yml`、`matrix: pr`） | push | なし | × | — | — | 1168 / 1389（18 回） |
| `nightly.yml` `web-e2e`（`matrix: full`） | schedule | なし | — | — | — | 2239（10-18 の 1 回） |
| `nightly.yml` `federation-live` | schedule・手動 | なし | — | — | — | 570〜592（2 回） |
| `nightly.yml` `postgres-contract` | schedule | なし | — | — | — | nightly での実行はまだない |

イベントごとの所要時間（run の作成から）:

| イベント | `check` が出るまで | 最後のジョブの終わりまで | runner の消費（全ジョブの合計） |
| --- | --- | --- | --- |
| PR（全体） | 243 / 814 秒 | 247 / 846 秒 | 953 / 1117 秒 |
| PR（go と web の両方が動いたもの、34 回） | 346 / 861 秒 | — | — |
| PR（docs だけ、10 回） | 30 / 34 秒 | — | — |
| Merge Queue | 271 / 434 秒 | 271 / 434 秒 | 1117 / 1141 秒 |
| main への push（最後まで動いたもの） | 830 / 933 秒 | 1869 / 1975 秒 | 2458 / 2971 秒 |

ここから分かること:

- **main への push の 63%（49 回中 31 回）は、ジョブが 1 つも動かずに取り消されている。** `ci.yml` の `concurrency`（`ci-refs/heads/main`、main では取り消さない）は、
  同じグループで 1 つが走っている間は次の 1 つだけを待たせ、さらに新しい push が来ると待っている方を取り消す。
  main の run は E2E のため約 30 分かかるので、そのあいだのコミットの CI は丸ごと消える。`web-e2e` の `cancel-in-progress: true` は走っている E2E を一度も取り消していない（取り消されるのは待っている run の側）。
- main のコミットは Merge Queue で検査したマージグループの head と同じコミットである（push の `headSha` が merge group の `headSha` と一致する）。main への push で `check` の文脈を回し直しても、同じ木を 2 度検査しているだけである。
- PR の p90 は runner の待ちで延びる。`changes` のあと最後に始まるジョブの待ちは、中央値 75 秒、p90 598 秒。
  Free プランの同時実行は 20 ジョブ（[Actions の上限](https://docs.github.com/en/actions/reference/limits)、2026-10-19 確認）で、PR の run は 1 回で 15 ジョブ前後、測った期間の Merge Queue は最大 5 グループを同時に組む設定（`max_entries_to_build: 5`）だった。
- `web-check` の内訳（merge group の 1 回）は tsc 13 秒、ESLint 76 秒、knip 3 秒、Vite のビルド 10 秒、Vitest 129 秒。直列なので `web` が PR と Merge Queue の最長のジョブになる。
- main の E2E（`matrix: pr`）の最新の 1 回は 1792 テストを 2 worker で 18.0 分。`make web-e2e` のうちビルド（Vite と `bin/my`）は約 16 秒で、Playwright の前のセットアップ（コンテナ、make、Bun、Node.js、Go）が約 74 秒。
  nightly の全体（`matrix: full`）は 10-18 に 3280 テストで 35.8 分だった。いまのテスト数（1 プロジェクト約 224 テスト × 20 プロジェクト、約 4480 テスト）では、1 ジョブで約 45〜49 分になる。
- `ubuntu-latest` は公開リポジトリで 4 vCPU・16 GB（[GitHub-hosted runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)、2026-10-19 確認）。E2E は 2 worker で回している。

### 必須チェックとスキップ（GitHub のドキュメント、2026-10-19 確認）

- パス・ブランチ・コミットメッセージでスキップされた**ワークフロー**のチェックは "Pending" のままで、必須ならマージを止める。
  条件（`if`）でスキップされた**ジョブ**は "Success" を報告する。スキップされうるワークフローを必須にしない
  （[Troubleshooting required status checks](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/troubleshooting-required-status-checks)）。
- Merge Queue が必須にする Actions のチェックは `merge_group` で起動しなければならず、起動しないと報告されずにマージが失敗する（同上、[Managing a merge queue](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue)）。
- `paths` / `paths-ignore` は `push` と `pull_request` について説明されており、`merge_group` のフィルタとして書かれているのは `branches` / `branches-ignore` だけ
  （[Events that trigger workflows](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows)、[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)）。
  Merge Queue で変更範囲によって検査を絞るには、いまの `changes` ジョブのように、ワークフローの中で差分を判定するしかない。

## Decision Drivers（判断基準）

1. main を壊さない: Merge Queue は、PR を main（と先に並んだ PR）に重ねて初めて壊れうるものを、マージの前に必ず止める
2. マージまでの待ち時間（PR と Merge Queue の `check` が出るまで）を短くする
3. runner の消費を減らす（Free プランの同時 20 ジョブを、結果が使われない実行に使わない）
4. 遅い検査（E2E、実 PostgreSQL、連合）の退行を、main に入ったあと遅くとも 1 日で見つける
5. 必須チェックがスキップで Pending のまま残らない
6. ワークフローを文脈ごとに読め、手で 1 文脈だけ回せる

## Considered Options（選択肢）

振り分け:

- A. 今のまま（PR・Merge Queue・main のどこでもほぼ同じものを回し、E2E は main、全体は nightly）
- B. 守るものごとに防ぐ場所を決める: PR は変わった文脈を全部、Merge Queue は「重ねて壊れうるもの」だけ、main はマージ後にしか回さない遅い検査だけ、nightly は全体

ワークフローの分け方:

1. `ci.yml` 1 本のまま（`changes` とジョブの `if`）
2. 文脈ごとの独立したワークフローに `on.pull_request.paths` を付け、それぞれを必須チェックにする
3. 文脈ごとの再利用ワークフロー（`workflow_call`）に分け、PR と Merge Queue の入口の `ci.yml` が `changes` の結果で呼び分け、`check` が結果を集める

E2E:

- E1. 1 ジョブのまま
- E2. ビルドを 1 回だけして成果物を配り、Playwright の `--shard` で N 個のジョブに分ける

## Decision（決定）

**B・3・E2 を採用する。** 決定者の回答（#556 のレビュー、原文）: 「ADR Accepted」

### 守るものと防ぐ場所

○ は必ず回す、（文脈）はその文脈が変わったときだけ回す、— は回さない。「必須」は PR と Merge Queue の `check` に入れることを表す。

| 守るもの | 検査 | PR | Merge Queue | main への push | nightly | 理由 |
| --- | --- | --- | --- | --- | --- | --- |
| Markdown の書式 | markdownlint | （docs）必須 | — | — | — | ファイル単位で決まり、2 つの PR を重ねても変わらない（重なりは Git の衝突になる） |
| ADR の索引 | `adr-index-check` | （docs）必須 | （docs）必須 | — | — | 2 つの PR が ADR を足すと、それぞれは通っても重ねた索引が古くなる・番号がぶつかる |
| Go のモジュール・vet・lint・`CGO_ENABLED=0` ビルド | `go-mod-check`・`go-vet`・`go-lint`・`go-build` | （go）必須 | （go）必須 | — | — | 名前の変更と呼び出しの追加、最後の呼び出しの削除（`unused`）など、重ねて初めて壊れる |
| sqlc の生成物 | `sqlc-check` | （go）必須 | （go）必須 | — | — | クエリとスキーマを別々の PR が変えると、重ねた生成物が古くなる |
| Go の依存のライセンス | `go-licenses` | （go）必須 | — | — | — | `go.sum` だけで決まる。2 つの PR の依存の和は、それぞれを通った依存の和 |
| Go のテスト（pgmem） | `go-test` | （go）必須 | （go）必須 | — | — | 振る舞いの組み合わせで壊れる |
| 実 PostgreSQL との互換 | `go-test-postgres` | — | — | DB に関わるパスが変わったときだけ | ○ | pgmem との差は稀で、壊れても main に入れたまま 1 日以内に見つければ足りる（ADR-0041 の悪い影響） |
| スキーマの desired-state | `db-check`（sqlite・postgres） | （schema）必須 | （schema）必須 | — | — | 2 つのスキーマの変更を重ねると、名前の重複などで適用が壊れうる。安い（30 秒以内） |
| i18n のカタログと生成物 | `i18n-check` | （i18n）必須 | （i18n）必須 | — | — | 2 つの PR がメッセージを足すと、重ねた生成物が古くなる |
| API の仕様と生成物 | `api-check` | （api）必須 | （api）必須 | — | — | 同上。operationId の重複など、仕様の lint も重ねて壊れる |
| `@my/ui` の型と単体テスト | `ui-check` | （ui）必須 | （ui）必須 | — | — | 型とテストは重ねて壊れる |
| `app/web` の型・lint・knip・ビルド・ルートの木・テスト | `web-check` のうち typecheck・lint・knip・dist・routes・test | （web）必須 | （web）必須 | — | — | 型・未使用の export・生成されるルートの木は重ねて壊れる |
| npm のライセンス・デザイントークン・Playwright の版・`100vh` | `web-licenses`・`web-tokens`・`playwright-image-check`・CSS の検査 | （web）必須 | — | — | — | ファイル単位または lockfile だけで決まる |
| Dockerfile と `.tool-versions` の一致 | `docker-check` | （docker）必須 | （docker）必須 | — | — | `.tool-versions` と `Dockerfile` を別々の PR が変えると重ねて壊れる。6 秒 |
| シェルスクリプト | shellcheck | （shell）必須 | — | — | — | ファイル単位 |
| `@my/ui` のスクリーンショットと axe | `ui-test` | （ui）必須 | — | （ui） | — | 基準画像は変えた PR の中で直す必要がある。重ねて画像が変わるのは稀で、main で見つければ足りる |
| イメージのビルドと Compose の起動（amd64） | `docker-smoke` | （docker、`cmd/`・`internal/platform`）必須 | — | ○（docs だけの push を除く） | ○ | Dockerfile・`deploy/` の変更と、設定の読み込みや起動を組み立てる `cmd/`・`internal/platform` の変更は PR で止める（起動の失敗は Compose で初めて見つかることがある）。ほかの Go や Web のコードの変更でイメージが壊れることは稀 |
| arm64 のイメージ（QEMU） | `docker build --platform linux/arm64` | （docker）必須 | — | — | ○ | Go の arm64 のクロスビルドは `CGO_ENABLED=0` で決まり、イメージの差は Dockerfile の変更でしか生まれない |
| Web の主要な流れ（E2E、部分集合） | `web-e2e`（`matrix: pr`） | 手動（`workflow_dispatch`） | — | ○（シャード分割） | — | ADR-0040 のとおり。待ちの長い検査を毎マージに入れない |
| Web の全体（E2E、全幅 × 全ロケール） | `web-e2e`（`matrix: full`） | — | — | — | ○（シャード分割） | ADR-0015・ADR-0027 のとおり |
| 連合の相互運用 | `federation-live` | 手動 | — | — | ○ | 外部のイメージを使い 10 分かかる。ADR-0015 のとおり |
| PR が使う Go と Bun のキャッシュ | キャッシュの保存 | — | — | ○ | — | PR はキャッシュを復元するだけで、保存は main で行う |

- `schema`・`ui-test`・`docker-smoke` は、PR で動くときは `check` に入れる（いまは入っておらず、落ちてもマージできる）。
- E2E は PR で自動では回さない（パスやラベルによる起動も持たない）。PR のブランチで確かめたいときは `workflow_dispatch` で手で回す。
- `docker-smoke` を PR で回す範囲は、Docker の文脈に `cmd/` と `internal/platform` の変更を加えたものにする。`scripts/changed-contexts.sh` はこの 2 つを go の文脈に入れるので、`docker-smoke` の起動はパスで別に判定する。arm64 のイメージのビルドは Docker の文脈だけのまま。
- Merge Queue が同時に組むグループは最大 2（`max_entries_to_build: 2`）、1 回にマージするのは最大 5（`max_entries_to_merge: 5`）にする。グループが増えると同時 20 ジョブを使い切り、PR の待ちが延びるからである。ルールセットは `.github/rulesets/protect-main.json` に置く。
- nightly の schedule は 18:00 UTC のままにする。GitHub の混雑で 2 時間ほど遅れて起動することがある（19:47〜20:10 UTC に起動していた）が、main に入ったあと 1 日以内に退行を見つける（判断基準 4）には足りる。
- Merge Queue で回す範囲は PR より狭いが、Merge Queue に入るのは PR の `check` を通ったものだけなので、PR だけで回す検査も main に入る前に一度は通っている。
- main への push では Merge Queue で通した検査を回し直さない。main のコミットは Merge Queue で検査したマージグループの head そのものだからである。
- main の run は新しい push が来たら取り消す（ADR-0040 の意図のとおり、最後のコミットを検査する）。E2E が main で落ちたときの扱いは ADR-0040 のまま。

### ワークフローの分け方

```text
.github/workflows/
  ci.yml              pull_request と merge_group の入口。changes → 文脈ごとの再利用ワークフローを if で呼ぶ → check
  main.yml            main への push。キャッシュの保存、web-e2e（シャード）、docker-smoke、ui-test、DB のパスが変わったときの postgres-contract
  nightly.yml         schedule と手動。web-e2e（full、シャード）、federation-live、postgres-contract、arm64 のイメージ
  ctx-docs.yml        workflow_call（と workflow_dispatch）。入力 scope: pr | queue
  ctx-go.yml          同上。go と go-test のジョブ
  ctx-web.yml         同上。ui・web（静的検査とテストを別ジョブにする）・ui-test
  ctx-i18n.yml / ctx-api.yml / ctx-schema.yml / ctx-docker.yml / ctx-shell.yml
  web-e2e.yml         workflow_call と workflow_dispatch。build → shard × N → report
  postgres-contract.yml / federation-live.yml   今のまま
```

- 必須チェックは今までどおり `ci.yml` の `check` の 1 つにする。`ci.yml` は `pull_request` と `merge_group` で**必ず起動し**、ワークフローにパスのフィルタを付けない。
  文脈を絞るのは、呼び出すジョブの `if`（`changes` の出力）だけにする。`if` でスキップしたジョブは "Success" を報告し、`needs` の結果は `skipped` になるので、
  `check`（`if: always()`）はスキップを成功として扱える。スキップでワークフローごと Pending になることはない。
- 再利用ワークフローの入力 `scope`（`pr` / `queue`）で、PR だけの検査（ライセンス、markdownlint、shellcheck など）を Merge Queue で飛ばす。
- `web-check` は Merge Queue と PR で 2 ジョブに分ける（tsc・ESLint・knip・ビルド・ルートの木で約 110 秒、Vitest で約 130 秒）。`make web-check` は手元では今までどおり 1 つのターゲットのまま。
- `main.yml` の `concurrency` は main で 1 グループ、`cancel-in-progress: true` にする。`main.yml` の変更範囲の判定は、push の `before` と `after` の差分を `scripts/changed-contexts.sh` に渡す。
- 選択肢 2 は採らない。PR のワークフローがパスでスキップされると必須チェックが Pending のまま残り、`merge_group` にはパスのフィルタがないので Merge Queue では毎回すべて動くからである。

### E2E の並列化

```text
build（ubuntu-latest、1 ジョブ）: make web-build と bin/my のビルド → actions/upload-artifact（app/web/dist と bin/my）
shard 1..N（Playwright のイメージ）: bun install → download-artifact → playwright test --shard=i/N --reporter=blob
report（1 ジョブ、if: always()）: blob を集めて playwright merge-reports で 1 つの HTML にする
```

- シャードは `fullyParallel: true` のテスト単位で分かれる。シャードごとに自分の `vite preview` と `my serve`（SQLite、毎回新しい DB）を起動するので、シードとレート制限は今までどおりシャードの中で閉じる。
- 成果物を配る主な理由は、時間よりも全シャードが同じビルドを検査することである（ビルドそのものは約 16 秒）。シャードは Go のセットアップを省ける。
- 見込み（実測の 1 テスト約 0.6 秒〈2 worker の壁時計〉、シャードの固定の準備約 90 秒、ビルドのジョブ約 90 秒、レポートの結合約 30 秒。幅はシャードの偏りの分）:

| 範囲 | 今 | N=4 | N=6 | N=8 |
| --- | --- | --- | --- | --- |
| main（`matrix: pr`、1792 テスト、18.0 分） | 約 19.5 分（ジョブ） | 約 8〜9 分 | 約 7 分 | 約 6 分 |
| nightly（`matrix: full`、約 4480 テスト） | 約 45〜49 分（推定） | 約 15〜17 分 | 約 11〜12 分 | 約 9〜10 分 |

- シャードの数は main が N=4、nightly が N=8。main は同時 20 ジョブを PR と Merge Queue と分け合うので、シャードを増やしすぎない。runner の消費は 1 ジョブのときより、シャードの準備の分だけ 3〜4 割増える（main の N=4 で約 1170 秒が約 1560 秒）。
- シャードの worker 数（いまは 2）を 3 にして速くなるかは、実装のときに測って決める。

### 見込みの効果

- Merge Queue: `ui-test`（204 秒）・`docker-smoke`（179 秒）・PR だけの検査がなくなり、`web` を 2 つに分けるので、最長のジョブは `go`（約 180 秒）になる。`check` まで約 271 秒が約 200 秒に、runner の消費は約 1117 秒が約 750 秒になる。
- PR: `docker-smoke` は Docker の文脈か `cmd/`・`internal/platform` を変えたときだけになり、`web` の分割で最長のジョブが約 220 秒から約 150〜180 秒になる。
- main: `check` の文脈を回し直さないので、runner の消費は E2E（シャード込みで約 1560 秒）と `docker-smoke`・キャッシュの保存が中心になる。E2E は約 8〜9 分で終わり、新しい push で取り消されるので、待っている run がジョブなしで消えることはなくなる。

### 覆す条件

- 重ねて初めて壊れる種類の失敗が、Merge Queue で回さない検査（`ui-test`・`docker-smoke` など）で main に繰り返し入るなら、その検査を Merge Queue に戻す。
- 有料のプランや大きな runner に移り、同時実行の上限が問題でなくなったら、E2E の部分集合を Merge Queue に入れることを考え直す。

## Consequences（影響）

- 良い影響
  - Merge Queue と PR の待ちと runner の消費が減り、Merge Queue に複数のグループが並んでも同時 20 ジョブを使い切りにくくなる。
  - main のすべてのコミットが、Merge Queue で検査済みであると同時に、最後のコミットはマージ後の検査（E2E など）を約 8〜9 分で受ける。
  - 何をどこで守るかが表とワークフローの名前で読める。1 文脈だけを `workflow_dispatch` で回せる。
  - `schema`・`ui-test`・`docker-smoke` が PR の必須チェックに入り、落ちたままマージされることがなくなる。
- 悪い影響・トレードオフ
  - `docker-smoke` と `ui-test` が Merge Queue から外れ、Go や Web の変更でイメージやスクリーンショットが壊れた場合は main に入ってから見つかる。
  - 実 PostgreSQL のテストは、DB に関わるパスを変えない限り nightly まで回らない。
  - ワークフローのファイルが増える。再利用ワークフローのジョブ名は「呼び出し側 / 呼ばれた側」になり、ログを探す場所が 1 段深くなる。
  - E2E のシャードは準備の時間の分だけ runner を多く使う。
  - `scripts/changed-contexts.sh` の判定に、Merge Queue と main の両方がいっそう頼る。判定の漏れは「知らないパスはすべての文脈」で防ぐ（今のまま）。
- 置き換え
  - ADR-0040 の「ほかのジョブ（`check`、`schema`、`ui-test`、`docker-smoke`）は今までどおり PR と main の両方で動く」「main の実行は取り消さず、コミットごとに最後まで回す」と、
    `web-e2e` を 1 つのジョブで回す部分を、この ADR が置き換える。E2E を PR で回さないこと、main で部分集合・nightly で全体を回すこと、main が一時的に E2E で落ちることを許すことは ADR-0040 のまま。
  - ADR-0041 の「コンテナの PostgreSQL での契約テストは、main への push と nightly で回す」を、「nightly と、DB に関わるパスを変えた main への push で回す」に置き換える。pgmem の採用と毎 PR の `db-check` は ADR-0041 のまま（`db-check` はスキーマの文脈を変えたときに PR と Merge Queue で回す）。
  - ADR-0046 は置き換えない。CI に脆弱性検査のジョブを持たないことはそのまま。Dependabot の PR は lockfile を変えるので、共有の設定としてすべての文脈を回す（今のまま）。
  - ADR-0015 の Build matrix（linux/arm64 のビルドを毎 PR）のうち、arm64 の**イメージ**のビルドを Docker の文脈を変えた PR と nightly に移す。Go の `CGO_ENABLED=0` のビルドは毎 PR のまま。
- 後続の作業（#560 と #561 は #559 のあと、#562 は #561 のあと、#563 は #559 と #561 のあと、#564 は最後）
  - #559: `ci.yml` を入口と文脈ごとの再利用ワークフローに分け、`schema`・`ui-test`・`docker-smoke` を `check` に入れる
  - #560: Merge Queue の `scope: queue` で PR だけの検査を飛ばし、`web` を静的検査とテストの 2 ジョブに分ける
  - #561: `main.yml` を作り、main への push で回すものをマージ後の検査とキャッシュの保存だけにする
  - #562: `web-e2e.yml` をビルド・シャード・レポートに分ける
  - #563: arm64 のイメージのビルドを Docker の文脈を変えた PR と nightly に移し、`docker-smoke` を PR では Docker の文脈と `cmd/`・`internal/platform` で絞る
  - #564: CONTRIBUTING.md・AGENTS.md・`docs/agents/workflow.md` の CI の説明を新しい振り分けに合わせる

## Pros and Cons of the Options（詳細比較・任意）

| | 1. `ci.yml` 1 本 | 2. 文脈ごとのワークフロー + `paths` | 3. 再利用ワークフロー + 入口の `check` |
| --- | --- | --- | --- |
| 必須チェック | `check` 1 つ | 文脈の数だけ。パスでスキップされると Pending のまま止まる | `check` 1 つ。スキップはジョブの `if` なので Success 扱い |
| Merge Queue での絞り込み | `changes` で絞れる | `merge_group` にパスのフィルタがなく、毎回すべて動く | `changes` で絞れる |
| 読みやすさ・手で回す | 1 ファイルが長い。1 文脈だけは回せない | 文脈ごとに読める・回せる | 文脈ごとに読める・`workflow_dispatch` で回せる |
| 実装の手間 | なし | 中（必須チェックの回避策が要る） | 中 |

選択肢 A（今のまま）の最も強い利点は、Merge Queue と main でも PR と同じ検査を回すので、重ねて壊れる種類を考え漏らしても止まることである。
ただし main への push の検査は Merge Queue と同じ木を 2 度検査しているだけで、しかも 63% が取り消されて検査されていない。

E1（1 ジョブのまま）の利点は runner の消費が最も少ないことだが、main の E2E が約 20 分かかり、main への push が続くあいだは最後のコミット以外の結果が得られない。

## Open Questions / 要検証

- E2E のシャードの worker 数を 3 にしたときの速さと、フレークの増減
