# ADR-0007: Mastodon Client API とは互換させず、独自のクライアント API を提供する

- Status: Accepted
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #9

## Context

Mastodon Client API に互換させれば、既存クライアント（Ivory, Tusky, Elk, Phanpy など）を使える。
しかし互換のためには Mastodon 固有のデータ表現・ページング・エンティティ構造への変換（Glue コード）が大量に必要になり、
本プロジェクト独自の機能（例: 公開範囲 `local`、[ADR-0017](0017-visibility-local.md)）とも衝突する。

## Considered Options

1. Mastodon Client API 互換（サブセット）+ 名前空間を分けた拡張 API
2. **独自のクライアント API のみ**

## Decision

**2 を採用する。Mastodon Client API（Streaming API、アプリ登録 API を含む）とは一切互換させない。**

- 自前の Web クライアント（PWA）とモバイルアプリが唯一の公式クライアントで、API は自前クライアントの要件に合わせて設計する。
- API の様式（REST + OpenAPI / RPC / GraphQL など）は [ADR-0016](0016-client-api-style.md) で決める。
- 認可は OAuth 2.0（Authorization Code + PKCE）を使う。将来サードパーティクライアントを受け入れられるようにするため。
- **連合（ActivityPub / S2S）は従来どおり Mastodon などと相互運用する**（[ADR-0006](0006-federation-protocol-scope.md)）。この決定はクライアント API にだけ適用される。

## Consequences

- 変換層が無くなり、ドメインモデルをそのまま API に出せる。独自機能を自由に追加できる。
- 既存の Mastodon アプリは使えないため、v0 では **自前クライアントが必須** になる。モバイル体験は PWA が担う（ADR-0008）。
- サードパーティクライアントのエコシステムはゼロから。将来に備え、API スキーマと認可フローを公開できる品質で保つ。
