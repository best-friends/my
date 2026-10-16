# Contributing

人間のコントリビュータもコーディングエージェントも、同じルールで作業します。詳細は [AGENTS.md](AGENTS.md) と
[ADR-0014](docs/adr/0014-task-management-and-agent-workflow.md) を参照してください。

## セットアップ

ツールのバージョンは `.tool-versions` で固定しています（[asdf](https://asdf-vm.com/) または [mise](https://mise.jdx.dev/)）。

```sh
asdf install   # or: mise install
make check
```

## 進め方

1. GitHub Issue を選ぶ（`agent:ready` は受け入れ基準が明確で着手可能なもの）。
2. ブランチ `<type>/<issue>-<slug>` を切る。`main` に直接 push しない。
3. `make check` を通してコミット（Conventional Commits、英語）。
4. PR テンプレートに従って PR を作成する。

## 設計判断

アーキテクチャに関わる変更は ADR を書きます: `make adr-new TITLE="..." SLUG=...`。
ADR を `Accepted` にするのはメンテナです。

## 言語

- 設計ドキュメント（ADR・要件）: v0 の間は日本語
- コード・コメント・コミットメッセージ・PR タイトル: 英語
- Issue・PR 本文: 日本語 / 英語どちらでも可
