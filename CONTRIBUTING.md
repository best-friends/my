# Contributing

人間のコントリビュータもコーディングエージェントも、同じルールで作業します。詳細は [AGENTS.md](AGENTS.md) と
[ADR-0014](docs/adr/0014-task-management-and-agent-workflow.md) を参照してください。

## セットアップ

ツールのバージョンは `.tool-versions` で固定しています（[asdf](https://asdf-vm.com/) または [mise](https://mise.jdx.dev/)）。

```sh
asdf install   # or: mise install
make check
```

`make check` はネットワーク不要のチェック（Markdown lint、ADR 索引、Go の `go vet` / golangci-lint / テスト /
`CGO_ENABLED=0` ビルド / 依存ライセンス検査）をすべて実行します。初回のみ Go モジュールとツールのダウンロードが必要です。

- Go ツール（govulncheck, go-licenses）は `tools/go.mod` の `tool` ディレクティブでバージョン固定しています。
- 脆弱性検査はネットワークが必要なため `make check` には含めず、`make go-vuln` で実行します（CI では毎回実行）。
- 依存ライブラリのライセンスは AGPL-3.0 と両立するもの（`Makefile` の `GO_ALLOWED_LICENSES`）に限ります。
  増やす場合は PR で理由を説明してください。

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
