# ADR-0040: Web の E2E を main への push でだけ回す

- Status: Accepted
- Date: 2026-10-19
- Deciders: @rosylilly
- Issue: <!-- 議論用 Issue なし（決定者の指示による） -->

## Context（背景・課題）

[ADR-0015](0015-testing-strategy.md) と [ADR-0027](0027-ui-quality-gates.md) は、Web の E2E（Playwright）を毎 PR で主要な流れだけ回し、nightly で全体を回すと決めている。
開発の段階が進み、エージェントが並行して多くの PR を出し、1 本ずつ main にマージするようになった。CI の各ジョブの所要時間は、`web-e2e` が約 15 分、`check` が約 8 分、ほかは 3 分以内で、PR の CI の待ち時間は `web-e2e` が決めている。
PR を main に取り込み直すたびに `web-e2e` が回り、その結果を待つ時間と実行の資源の多くが、すぐ次のマージで古くなる結果に使われている。

## Decision Drivers（判断基準）

- マージまでの待ち時間を短くする
- CI の実行の資源を、結果が使われない実行に使わない
- 主要な流れの退行は、main に入ったあと早く見つける

## Considered Options（選択肢）

1. 毎 PR で E2E を回す（今まで）
2. main への push でだけ E2E を回し、新しい push が来たら走っている E2E を取り消す
3. E2E を nightly だけにする

## Decision（決定）

**選択肢 2 を採用する。** 決定者の指示: 「main マージ時のみ E2E すべき + 実行中にマージが来たら先行で動いている E2E はキャンセル、とかでいいと思います」

- `ci.yml` の `web-e2e` ジョブは `push`（main）のときだけ動き、PR では動かない。範囲は今までの PR 用の部分集合（`matrix: pr`）のまま。
- `web-e2e` ジョブは main で 1 つの並行グループを持ち、新しい push が来たら走っている E2E を取り消す。
- ほかのジョブ（`check`、`schema`、`ui-test`、`docker-smoke`）は今までどおり PR と main の両方で動く。PR の実行は、同じ PR に新しい push が来たら取り消す。main の実行は取り消さず、コミットごとに最後まで回す。
- nightly は今までどおり E2E の全体（`matrix: full`）を回す。
- E2E に触れる PR では、作者が手元で `make web-e2e` を回す。

この決定は ADR-0015 と ADR-0027 の「毎 PR（スモーク / 主要な流れ）」の E2E の頻度を置き換える。ほかの部分は両 ADR のまま。

## Consequences（影響）

- PR の CI の待ち時間は `check` の約 8 分で決まる。
- E2E の退行は PR では見つからず、main に入ったあとの `web-e2e` か nightly で見つかる。見つけたら、壊した PR を特定して直す PR を出す（v0 のあいだは main が一時的に E2E で落ちることを許す）。
- main に続けてマージすると、間の E2E は取り消され、最後のコミットだけが検査される。どのコミットで壊れたかは、そのあとで切り分ける。
- `web-e2e` は必須チェック（ブランチ保護の `check`）に入っていないので、ブランチ保護は変えない。

## Open Questions / 要検証

- マージの頻度が落ちたあとで、PR での E2E（変更したファイルで絞るなど）に戻すかどうか。
