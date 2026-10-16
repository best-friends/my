# ADR-0014: タスク管理とコーディングエージェントの作業フロー

- Status: Accepted
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: —

## Context

タスク管理は GitHub の Issue / Project / Milestone で行う。複数のコーディングエージェント（Claude Code ほか）が自律的に Issue を拾い、
人間はレビューと意思決定に集中したい。

## Decision

### Issue の種類（ラベル `type:*`）

| 種類 | 用途 | テンプレート |
| --- | --- | --- |
| `type:adr` | 設計判断の議論・決定依頼。結論は ADR ファイルに反映 | ADR |
| `type:spike` | 調査・検証（タイムボックス付き）。成果物はレポート（Issue コメント or `docs/spikes/`）と使い捨てコード | Spike |
| `type:task` | 実装・ドキュメント・設定の作業 | Task |
| `type:epic` | 複数 Task をまとめる親 Issue（sub-issues を使う） | — |
| `type:bug` | 不具合 | Bug |

### 状態ラベル

- `agent:ready` — 受け入れ基準が明確で、依存する ADR が Accepted。**エージェントはこのラベルの Issue だけを自律的に着手してよい。**
- `status:needs-decision` — 人間の判断待ち（Proposed ADR に依存など）。エージェントは着手しない。
- `agent:blocked` — エージェントが作業中に判断が必要になった。理由を Issue にコメントして人間を待つ。
- `area:*` — 影響範囲（`federation`, `api`, `db`, `queue`, `storage`, `web`, `mobile`, `i18n`, `infra`, `harness`, `docs`）
- `priority:p0`〜`p2`

### 作業フロー（エージェント）

1. `agent:ready` かつ担当者なしの Issue を選ぶ（Milestone の若い順 → priority の高い順）。自分を assign し、着手コメントを残す。
2. ブランチ `<type>/<issue番号>-<slug>`（例: `task/42-sqlite-adapter`）。
3. 関連 ADR・要件 ID を読み、Accepted の決定に従う。決定にない設計判断が必要になったら、
   小さいものは PR 説明に明記、大きいものは Proposed ADR を起票して `agent:blocked`。
4. `make check` が通る状態でコミット（Conventional Commits、英語）。
5. PR を作成（テンプレートに従い `Closes #N`、受け入れ基準のチェック結果を記載）。**main への直接 push はしない。**
6. マージは人間が行う。

### Definition of Ready / Done

- Ready: 背景・受け入れ基準・関連 ADR / 要件 ID・影響範囲が書かれている。依存 Issue が解決済み。
- Done: 受け入れ基準を満たす、`make check` が通る、ドキュメント（ADR / docs / i18n カタログ）が更新済み、PR がレビュー承認されマージ済み。

### Milestone と Project

- Milestone = 期間ではなく到達状態（M0〜M5、[ビジョン](../requirements/00-vision.md) 参照）。
- GitHub Project（v2）でボード管理: Status（Backlog / Ready / In progress / In review / Done）、Priority、Area、Milestone。

## Consequences

- エージェントは「どれを拾ってよいか」をラベルだけで判断できる。
- 人間のボトルネックは `status:needs-decision` と PR レビューに集約される。
