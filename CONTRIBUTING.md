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
`CGO_ENABLED=0` ビルド / 依存ライセンス検査 / sqlc 生成物の検査、i18n カタログ検査、クライアント API の spec の lint と生成物の検査、`Dockerfile` の Go イメージと `.tool-versions` の一致、Web クライアントの型検査・ビルド・依存ライセンス検査）をすべて実行します。初回のみ Go モジュールとツール、`tools/i18n` と `app/` の npm パッケージのダウンロードが必要です。

- Go ツール（govulncheck, go-licenses, sqlc, oapi-codegen）は `tools/go.mod` の `tool` ディレクティブでバージョン固定しています。
- 脆弱性検査はネットワークが必要なため `make check` には含めず、`make go-vuln` で実行します（CI では毎回実行）。
- Docker Compose 構成のスモークテスト（イメージのビルド、スキーマ適用、管理者作成、WebFinger・Actor・NodeInfo の確認）は Docker が必要なため `make check` には含めず、`make docker-smoke` で実行します（CI では毎回実行）。
- 依存ライブラリのライセンスは AGPL-3.0 と両立するもの（`Makefile` の `GO_ALLOWED_LICENSES`。`tools/i18n` と `app/` の npm パッケージも同じ一覧で検査）に限ります。
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

## クライアント API

- `api/openapi.yaml`（OpenAPI 3.1）が正です。Go の strict server は oapi-codegen（`tools/go.mod` で固定）で `internal/interface/clientapi/api.gen.go` に生成し、生成物はコミットします（[ADR-0029](docs/adr/0029-api-code-generation.md)）。
- 規約とエンドポイントの追加手順は [docs/architecture/client-api.md](docs/architecture/client-api.md) にあります。

```sh
make generate        # api/openapi.yaml から生成し直す
make api-lint        # spec を Redocly CLI で lint（使える 3.1 の部分集合を含む。設定は api/redocly.yaml）
make api-check       # api-lint と、生成物が最新でなければ失敗する generate-check（make check に含まれる）
```

## Web クライアント

Web クライアント（`app/web`）は Vite でビルドし、`my serve` がバイナリに埋め込んだビルドと HTML の外枠を配ります
（[ADR-0028](docs/adr/0028-html-shell-link-previews-and-pwa.md)。設計規約は [docs/architecture/web-client.md](docs/architecture/web-client.md)）。

- `make web-build` は `app/web/dist` にビルドし、`internal/interface/web/dist` にコピーします。Go はそこを `//go:embed all:dist` で埋め込みます。
  コミットするのはプレースホルダの `.gitkeep` だけなので、Go のビルドとテストに Bun は要りません（そのバイナリはスクリプトのない外枠を返します）。
- `make build` は `make web-build` を先に実行し、Web クライアント入りの単一バイナリを作ります。Docker イメージも同じビルドを埋め込みます。

開発中は、Vite の開発サーバーを立て、`my serve` にその origin を渡します。ブラウザは `my serve` を開きます。
外枠・ランタイム設定・API・ActivityPub は本番と同じく Go が返し、スクリプトだけを Vite から HMR 付きで読み込みます。

```sh
make db-apply                                  # SQLite の my.db にスキーマを適用
cd app/web && bun run dev                      # Vite の開発サーバー（http://localhost:5173 固定）
MY_HOST=localhost:8080 MY_DATABASE_URL=my.db MY_KEK="$(openssl rand -base64 32)" \
  go run ./cmd/my serve -dev-vite-origin http://localhost:5173   # 別の端末で。http://localhost:8080 を開く
```

- `-dev-vite-origin` を付けると、外枠はキャッシュされず（`no-store`）、CSP はその origin のスクリプト・WebSocket・Vite が差し込む `<style>` を許す開発用のものになります。
  本番の CSP（nonce なしの `script-src 'self'`）での動作は、`make build` したバイナリで確認します。
- ランタイム設定の `apiOrigin` は開発時だけ `http://{host}` になります。

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

## 翻訳・メッセージ

利用者向けの文言は `locales/<locale>/<namespace>.json`（FormatJS JSON 形式）に追加し、`en` と `ja` を同時に更新します（[ADR-0009](docs/adr/0009-internationalization.md)）。
編集したら `make i18n-gen` で疑似ロケール（`en-XA` / `ar-XB`）と型付きアクセサを再生成してコミットします。
