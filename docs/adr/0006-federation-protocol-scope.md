# ADR-0006: 連合プロトコルの対応範囲

- Status: Accepted
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #8

## Context

「ActivityPub 互換」の実態は仕様書よりも Mastodon をはじめとする主要実装との相互運用性で決まる（G4）。

## Decision

### 対応する仕様・拡張（v0）

| 区分 | 対象 |
| --- | --- |
| 必須 | ActivityPub（S2S のみ）、ActivityStreams 2.0、WebFinger（RFC 7033）、host-meta、NodeInfo 2.1 |
| 署名 | HTTP Signatures draft-cavage-12（`rsa-sha256` / `hs2019`）と RFC 9421、**double-knocking**、`Digest` / `Content-Digest`（RFC 9530） |
| 互換拡張 | Mastodon 拡張（`toot:` 名前空間: `featured`, `discoverable`, `indexable`, `Emoji`, `sensitive`, `Hashtag` 等）、`contentMap` |
| 任意 | Authorized Fetch（secure mode）、Object Integrity Proofs（FEP-8b32）は C |
| 非対応 | AP C2S、Linked Data Signatures による署名生成（受信時の検証は互換のため検討）、リレー、`Move` |

### 相互運用テスト対象

Mastodon 4.x、Misskey、GoToSocial、Akkoma（Pleroma 系）。余力があれば Pixelfed、Threads。

### 実装方針

- ライブラリ（Go）は次の構成とする（根拠は [#15 の調査報告](https://github.com/best-friends/my/issues/15#issuecomment-6209363986)）。
  - 署名: RFC 9421 は `github.com/dadrus/httpsig`（MIT）、draft-cavage-12 は `code.superseriousbusiness.org/httpsig`（BSD-3-Clause、go-fed/httpsig の GoToSocial フォーク）。どちらも cgo は不要。
  - 語彙（AS2 / Mastodon 拡張）は自前の型で持つ。`go-ap/activitypub` は未知のプロパティを捨て、タグ付きリリースもないため、コアの型にはしない（受信パースの補助に使うかは実装時に判断）。`go-ap/client` / `go-ap/auth` は依存が重く、go-ap 系の抽象に結合しているため採用しない。
  - ライブラリが担うのは署名ベースの組み立て・暗号処理・`Content-Digest` の検証まで。受け入れポリシー（必須ヘッダ / コンポーネント、時間窓、`Digest` の検証、鍵の所有者と `actor` の一致）、鍵の解決、送信側の double-knocking、配送、Mastodon 拡張の解釈は自前で書く。ライブラリはアダプタ境界の内側に閉じ込める。
- 署名の double-knocking: 受信は `Signature-Input` があれば RFC 9421、なければ draft-cavage で検証する。送信は RFC 9421 を先に試し、400/401/403 なら draft-cavage で 1 回だけ再送して、成功した方式をホストごとに TTL 付きで覚える（Mastodon 4.7 と同じ方針。Mastodon は 4.5 から RFC 9421 の受信を常時有効にしている）。
- `@target-uri` / `(request-target)` は設定した公開オリジンから組み立て直し、`X-Forwarded-*` は信用しない。
- Inbox は署名検証まで同期、処理は非同期（NFR-PERF-4）。
- **HttpClient ポートに SSRF ガード** を組み込み、すべての外部取得を通す（NFR-SEC-1）。
- 受信した HTML は許可リストでサニタイズ（NFR-SEC-2）。
- 公開範囲 `local` の配送先・オブジェクト取得の制限は [ADR-0017](0017-visibility-local.md) に従う。
- 配送はドメイン単位に shared inbox へ集約。連続失敗ドメインはサーキットブレーカーで一時停止し、`InstanceHealth` に記録。
- 互換性のためのテストは「実サーバーから採取した JSON フィクスチャ」を `testkit` に蓄積して回帰テストにする。
  実サーバーとのライブ相互運用テスト（docker compose で Mastodon / GoToSocial を起動）は nightly ジョブとする。

## Consequences

- 互換性を担うライブラリが無いため、相互運用の細部は自前で持つ。フィクスチャ回帰テストとライブ相互運用テストへの投資が必須になる。
- フィクスチャ駆動の回帰テストにより、エージェントが互換性を壊したことを CI で検知できる。

## Open Questions / 要検証

- [ ] 実サーバー（Mastodon / Misskey / GoToSocial）との往復確認（#44、M3）
- [ ] Mastodon が RFC 9421 の受信で必須とするコンポーネント（`@method` / `@target-uri` / `content-digest` で足りるか）と、送信時に付ける `alg` / `expires` の有無の確認
- [ ] GoToSocial 式のクエリ文字列の有無による再検証・再送を入れるか
- [ ] ライブ相互運用テスト環境の構築コスト
