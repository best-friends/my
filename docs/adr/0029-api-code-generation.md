# ADR-0029: クライアント API のコード生成

- Status: Accepted（Consequences の「API を変えるときは、`api/openapi.yaml` を先に編集する」を ADR-0066（クライアント API の仕様をファイルに分けてバンドルする） が置き換える）
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #89

## Context

クライアント API は REST + OpenAPI 3.1 のスキーマファーストで、Go のサーバーと TS のクライアントを生成する（ADR-0016）。生成ツールは決まっていない。

## Considered Options

- Go: ogen（v1.24）/ oapi-codegen（v2.8、strict server）
- TS: openapi-typescript + openapi-fetch（+ openapi-react-query）/ `@hey-api/openapi-ts` / orval

## Decision

- **Go: oapi-codegen の strict server**（`net/http`）。
  - OpenAPI 3.1 への対応範囲が ogen より広い。
  - ADR-0019 のミドルウェア構成（認証・レート制限・CSRF）に組み込みやすい。
- **TS**
  - `openapi-typescript`: スキーマから型だけを生成する。
  - `openapi-fetch`: 薄い fetch ラッパー。ミドルウェアで Bearer トークンを付け、401 のときにトークンを更新する。
  - `openapi-react-query`: TanStack Query との接続に使う。
- **SSE（`/api/v1/streaming`）**: どちらの生成ツールもサーバー側を生成しないので、手書きのハンドラにする。
  - `EventSource` はヘッダを送れないので、クライアントは fetch のストリームで読む。
- **CI**
  - `make generate` の後に生成物の差分がないことを検査する。
  - スキーマを lint する。
  - Go のハンドラの応答をテストでスキーマと照合する（kin-openapi の `openapi3filter`）。
- 生成ツールが扱える 3.1 の部分集合だけを使い、lint で強制する（`$dynamicRef`、`if` / `then`、`prefixItems` などは使わない）。

## Consequences

- API を変えるときは、`api/openapi.yaml` を先に編集する。その後、Go と TS の生成物を同じ PR で更新する。
- JSON の処理が性能の問題になったら、ogen への切り替えを再検討する。
