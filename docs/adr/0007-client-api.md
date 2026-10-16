# ADR-0007: クライアント API は Mastodon 互換を主とする

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: <!-- 起票後に追記 -->

## Context

自前の Web / モバイルクライアントを作るが、v0 の時点でモバイルアプリを完成させるのは負荷が高い。
Mastodon Client API に互換させれば、既存のクライアント（Ivory, Ice Cubes, Tusky, Moshidon, Elk, Phanpy など）をすぐに使える。

## Considered Options

1. **Mastodon Client API 互換（サブセット）+ 名前空間を分けた拡張 API**
2. 独自 API（OpenAPI）のみ
3. GraphQL

## Decision（推奨案）

1 を採用する。

- v0 の対象エンドポイントは `docs/api/mastodon-compat.md`（M2 で作成）に一覧化し、実装状況を表で管理する。
  優先順: OAuth / apps → accounts（verify_credentials, lookup, statuses, follow 系）→ statuses → timelines → notifications → media → instance → search。
- 認可は OAuth 2.0 Authorization Code + PKCE。スコープは Mastodon 互換（`read`, `write`, `follow`, `push` と細分化スコープ）。
- 自前クライアントも同じ API を使う。Mastodon に存在しない機能は `/api/x/v1/...`（名称は実装時に確定）で提供し、
  Mastodon 互換レスポンスには拡張フィールドを足さない（既存クライアントの互換性を守る）。
- 型とテスト: コミュニティ管理の Mastodon OpenAPI 定義を参照し、レスポンススキーマの検証テストを自動生成する（ライセンス確認の上）。
- エラーメッセージは `Accept-Language` でローカライズ（FR-503）。

## Consequences

- 既存エコシステムのおかげで、v0 ではモバイル体験を既存アプリで代替できる。
- Mastodon API の仕様（ID の文字列扱い、ページングの Link ヘッダ等）に引きずられる。
- Streaming API は v0 では C。

## Open Questions / 要検証

- [ ] Mastodon OpenAPI 定義のライセンスと網羅度
- [ ] 主要クライアントが要求する最小エンドポイント集合の調査（起動時に叩く API）
