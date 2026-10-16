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

## スキーマの変更

DB スキーマは方言ごとの目標スキーマファイルを正とし、[Atlas](https://atlasgo.io/) Community Edition で適用します
（[ADR-0023](docs/adr/0023-declarative-schema-management.md)）。マイグレーション SQL は書きません。

- `schema/sqlite.sql` と `schema/postgres.sql` を、同じ PR で一緒に編集します。
  テーブルは [論理データモデル](docs/architecture/data-model.md) に従い、論理型は同文書の §1.2 で方言の型に対応づけます。
- 拡張機能・シーケンス・ビュー・トリガーなど Atlas Community Edition が扱えないものは、目標スキーマに書きません。
  PostgreSQL の拡張機能は `schema/postgres.bootstrap.sql`（冪等な準備 SQL）に書きます。
- Atlas は `scripts/install-atlas.sh` がバージョンとチェックサムを固定して `bin/atlas` に入れます（`make atlas`。
  `db-*` ターゲットが自動で実行します）。`atlas login` が必要な機能は使いません。

```sh
make db-apply                      # SQLite: 目標スキーマを my.db に適用（開発用 DB はインメモリ）
make db-diff                       # 適用される SQL を表示するだけ
make db-reset                      # DB の中身をすべて消してから適用（データが消える）
make db-check                      # db-reset のあと差分が空であることを検査（CI と同じ）

make db-up DB_DIALECT=postgres     # PostgreSQL コンテナ（DB `my` と開発用 DB `my_dev`）を起動
make db-apply DB_DIALECT=postgres  # db-diff / db-reset / db-check も同様
make db-down                       # コンテナとデータを削除
```

- 接続先は `DB_URL`（対象 DB）と `DB_DEV_URL`（Atlas が差分の正規化に使う空の DB）で変えられます。
  PostgreSQL の URL は `search_path=public` を付けたスキーマ単位の URL にします。
- コンテナ以外の PostgreSQL に適用するときは、準備 SQL の実行方法も `PG_BOOTSTRAP` で指定します
  （例: `PG_BOOTSTRAP='psql -v ON_ERROR_STOP=1 -q "postgres://..."'`）。開発用 DB にも準備 SQL を先に流しておきます。
- 差分の適用に失敗する変更（v0 では許容される破壊的変更、[ADR-0012](docs/adr/0012-v0-breaking-change-policy.md)）は `make db-reset` で作り直します。
- CI は両方言について、空の DB への適用と、適用後に差分が残らないことを検査します（`make db-check`）。

## SQL とデータベースのテスト

- クエリは方言ごとに `queries/<dialect>/*.sql` に書き、[sqlc](https://sqlc.dev/) で Go コードを生成します（[ADR-0003](docs/adr/0003-database-abstraction.md)）。
  設定は `sqlc.yaml`、sqlc のバージョンは `tools/go.mod` で固定しています。生成物はコミットし、`make check` が古くないかを検査します。

```sh
make sqlc        # sqlc.yaml から生成し直す
make sqlc-check  # 生成物が最新でなければ失敗（make check に含まれる）
```

- DB アダプタは `pkg/testkit` のコントラクトテスト（例: `testkit.UnitOfWorkContract`）を SQLite と PostgreSQL の両方で通します。
  テスト用の DB には目標スキーマ `schema/<dialect>.sql` を直接流します（空の DB に対する Atlas の適用と同じ SQL になります）。
- PostgreSQL のテストは Docker があれば testcontainers-go でコンテナを起動し、なければスキップします。
  既存のサーバーを使うときは `TEST_POSTGRES_URL=postgres://...`（DB を作成できるユーザー）を指定します。
  CI は `TEST_POSTGRES_REQUIRED=1` を設定し、PostgreSQL のテストをスキップせず必ず実行します。

## 設計判断

アーキテクチャに関わる変更は ADR を書きます: `make adr-new TITLE="..." SLUG=...`。
ADR を `Accepted` にするのはメンテナです。

## 言語

- 設計ドキュメント（ADR・要件）: v0 の間は日本語
- コード・コメント・コミットメッセージ・PR タイトル: 英語
- Issue・PR 本文: 日本語 / 英語どちらでも可
