# ADR-0000: 設計判断を ADR で記録する

- Status: Accepted（ADR の一覧を README.md に生成して CI で検査する部分を ADR-0081（Go が読む生成物だけをコミットする） が置き換える）
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: —

## Context

本プロジェクトは複数のコーディングエージェントが並行して自律的に作業する前提です。
口頭やチャットで決めたことはエージェントから見えないため、設計判断をリポジトリ内に、機械的に辿れる形で残す必要があります。

## Decision

- 設計判断は `docs/adr/NNNN-kebab-case-title.md` に MADR 風フォーマット（[template.md](template.md)）で記録する。
- 新規作成は `make adr-new TITLE="..."`。一覧（[README.md](README.md)）は `make adr-index` で再生成し、CI で差分を検査する。
- ステータスは `Proposed` → `Accepted` / `Rejected`。後から覆す場合は新しい ADR を書き、旧 ADR を `Superseded by ADR-XXXX` にする（本文は書き換えない。誤字修正や追記は可）。
- **エージェントは ADR を `Proposed` で起票・改訂してよいが、`Accepted` への変更は人間の Decider が行う。**
- 議論は ADR ごとの GitHub Issue（ラベル `type:adr`）で行い、結論を ADR 本文に反映して PR でマージする。
- ドキュメントの言語: 設計ドキュメント（ADR・要件）は v0 の間は日本語。コード・識別子・コミットメッセージ・コードコメントは英語。
  OSS 公開前に README と主要ドキュメントの英語版を用意する（別 Issue）。

## Consequences

- エージェントは `docs/adr/README.md` を見れば、何が決まっていて何が未決かを判断できる。
- 未決（Proposed）の ADR に依存する実装 Issue は `status:needs-decision` で止めておける。
