# ADR-0004: Claude Code のフックで、高くつく読み方を止め、長い出力を切り詰める

- Status: Accepted
- Date: 2026-10-22
- Deciders: @rosylilly
- Issue: なし

## Context（背景・課題）

道具を用意しても、エージェントが従来どおり大きな文書を丸ごと読み、CI を監視し続ければ効果は出ない。
エージェントに依らない規則は AGENTS.md に書くが、Claude Code ではフックでツール呼び出しの前後に介入できる。
決定者の指示（原文）:

> 4 : 固有フックもいれる

## Decision Drivers（判断基準）

- 止めるのは、同じ答えをずっと安く得る方法がある場合だけにする
- フックの失敗でツール呼び出しを止めない（fail-open）
- 許可の流れを変えない（`allow` を返して確認を省かない）

## Considered Options（選択肢）

1. 規則を AGENTS.md に書くだけ
2. 規則に加えて Claude Code のフックを入れる

## Decision（決定）

選択肢 2。`.claude/settings.json` に次を置く。実体は `scripts/agentkit-hook.sh` を通して `bin/agentkit hook <event>` が処理する。

- SessionStart: `bin/agentkit` をビルドし、索引を最新にする（出力しない）
- PreToolUse（Bash・Read）
  - 拒否: 40KB 以上の Markdown（コードは 80KB 以上）を、範囲を指定せずに Read または `cat` すること。理由に `outline` と `show` の使い方を返す
  - 拒否: `gh pr checks --watch` と `gh run watch`。理由に、バックグラウンドで動かす `agentkit pr wait` を返す
  - 助言（`additionalContext`、拒否はしない）: 出力を絞らない再帰的な grep / rg に、`agentkit find` と `refs` を勧める
- PostToolUse（Bash）: 標準出力と標準エラーの合計が 2 万文字を超える出力を、先頭と末尾だけに切り詰めて `updatedToolOutput` で返し、全体は `.tmp/agentkit/out/` に保存する。agentkit 自身の出力（予算内に収めている）は切り詰めない
- しきい値は環境変数 `AGENTKIT_BIG_FILE`・`AGENTKIT_MAX_OUTPUT` で変えられる
- `bin/agentkit` が無いとき、入力が読めないとき、想定外の入力のときは何もしない。`allow` は返さない

## Consequences（影響）

- 大きな文書を Edit する前の Read も、範囲の指定が必要になる
- 切り詰めた出力の続きは、保存したファイルを grep するか範囲を指定して読む必要がある
- Claude Code 以外のエージェントには効かない。それらには AGENTS.md の規則と `scripts/agentkit.sh` だけが届く
