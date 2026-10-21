# ADR-0016: クライアント API の様式

- Status: Accepted（Decision の「`api/openapi.yaml` を正とし」のうち正とするファイルと、Consequences の「API の変更はまず `openapi.yaml` を編集する」を ADR-0066（クライアント API の仕様をファイルに分けてバンドルする） が置き換える）
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #32

## Context

[ADR-0007](0007-client-api.md) で独自のクライアント API を提供することにした。サーバーは Go、クライアント（Web / Tauri）は TypeScript なので、
両言語で型を共有し、エージェントが API を変更してもクライアントとのずれを機械的に検出できる必要がある。

## Decision Drivers

1. Go と TypeScript の間でスキーマから型・クライアントを生成できること
2. 読み取り（タイムライン・プロフィール）を CDN / HTTP キャッシュに載せやすいこと（NFR-COST）
3. リアルタイム更新（通知・タイムライン）の手段があること
4. 将来のサードパーティ開発者にとっての分かりやすさ
5. 生成ツールの成熟度と、エージェントの習熟度

## Considered Options

1. **REST + OpenAPI 3.1（スキーマファースト）**: Go サーバーは ogen / oapi-codegen で生成、TS クライアントは openapi-typescript 系で生成。リアルタイムは SSE
2. Connect RPC（Protocol Buffers）: connect-go / connect-es。型の一貫性が強く、ストリーミングも標準。GET キャッシュは限定的
3. GraphQL（gqlgen）: クライアント主導の取得。キャッシュと N+1・認可の扱いが重い

## Decision

**1. REST + OpenAPI 3.1 をスキーマファーストで採用する。**

- `api/openapi.yaml` を正とし、Go のサーバーインターフェイスと TS クライアントを生成する。生成物と仕様のずれは CI で検査する。
- パス: `/api/v1/...`。v0 期間は破壊的変更を許容する（ADR-0012）。
- ページング: UUIDv7 によるキーセットページングで、不透明なカーソル（`cursor` / `next_cursor`）を返す。
- エラー: RFC 9457（`application/problem+json`）。`title` / `detail` は `Accept-Language` に応じてローカライズし、機械判定用に安定した `type` を持たせる。
- リアルタイム: Server-Sent Events（`/api/v1/streaming`）。WebSocket は必要になってから。
- 認可: OAuth 2.0 Authorization Code + PKCE。自前クライアントはファーストパーティとして登録済みのものを使う。スコープは独自に定義する。
- 匿名で取れる公開リソースの GET には `Cache-Control` を付け、CDN でキャッシュできるようにする。

**覆す条件**: ストリーミングや双方向通信の比重が大きい、あるいは OpenAPI の生成ツールの品質が Go 側で不足する場合は 2（Connect RPC）が有力。

## Consequences

- API の変更はまず `openapi.yaml` を編集する、というルールをエージェントに課せる。
- REST の設計判断（リソース粒度・命名）を自分たちで決める必要がある。API 設計ガイドを別途用意する。

## Open Questions / 要検証

- [ ] Go の OpenAPI 生成ツール（ogen / oapi-codegen）の比較（3.1 対応、strict server、SSE の扱い）
- [ ] TS クライアント生成（openapi-typescript + openapi-fetch 等）と Tauri での動作
