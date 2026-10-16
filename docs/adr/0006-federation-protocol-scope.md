# ADR-0006: 連合プロトコルの対応範囲

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #8

## Context

「ActivityPub 互換」の実態は仕様書よりも Mastodon をはじめとする主要実装との相互運用性で決まる（G4）。

## Decision（推奨案）

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

- ライブラリ（Go）: go-ap 系（vocab・署名）と、RFC 9421 / draft-cavage の署名ライブラリを spike（#15）で評価する。
  ライブラリに依存する範囲（JSON-LD の型・署名）と自前実装する範囲（配送・受信処理・Mastodon 拡張の解釈）を分け、ライブラリはアダプタ境界の内側に閉じ込める。
- Inbox は署名検証まで同期、処理は非同期（NFR-PERF-4）。
- **HttpClient ポートに SSRF ガード** を組み込み、すべての外部取得を通す（NFR-SEC-1）。
- 受信した HTML は許可リストでサニタイズ（NFR-SEC-2）。
- 配送はドメイン単位に shared inbox へ集約。連続失敗ドメインはサーキットブレーカーで一時停止し、`InstanceHealth` に記録。
- 互換性のためのテストは「実サーバーから採取した JSON フィクスチャ」を `testkit` に蓄積して回帰テストにする。
  実サーバーとのライブ相互運用テスト（docker compose で Mastodon / GoToSocial を起動）は nightly ジョブとする。

## Consequences

- 互換性を担うライブラリが無いため、相互運用の細部は自前で持つ。フィクスチャ回帰テストとライブ相互運用テストへの投資が必須になる。
- フィクスチャ駆動の回帰テストにより、エージェントが互換性を壊したことを CI で検知できる。

## Open Questions / 要検証

- [ ] #15 go-ap の保守状況・RFC 9421 / double-knocking 対応、自前実装する範囲の見積り
- [ ] ライブ相互運用テスト環境の構築コスト
