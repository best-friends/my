# ADR-0013: タイムラインは fan-out-on-read を既定とする

- Status: Accepted
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #13

## Context

Mastodon はホームタイムラインを Redis 上に fan-out-on-write で構築するが、これは常駐ミドルウェアを要求する（NFR-COST-2 に反する）。
小〜中規模では、読み取り時にフォロー関係と投稿を JOIN する fan-out-on-read で十分な性能が出る見込み。

## Decision

- v0 は **fan-out-on-read**: `statuses`（`account_id, id DESC` の複合インデックス）と `follows` を JOIN し、UUIDv7 の `id` でキーセットページング。
- 可視性フィルタ（ブロック・ミュート・公開範囲）はドメインの `canView` を SQL 条件に写像した共通クエリビルダで適用。
- `TimelineReader` をアプリケーション内部のインターフェイスにし、将来 `home_feed` テーブル（fan-out-on-write）や KV キャッシュに差し替え可能にする。
- 連合タイムラインはインスタンス設定で無効化可能（データ量と課金の抑制）。

## Consequences

- 追加インフラ不要。書き込みが軽い。
- フォロー数が多い利用者で読み取りが重くなる。Neon 等の従量課金 DB では読み取り量が費用に効くため、M2 で計測して閾値を決める。

## Open Questions / 要検証

- [ ] フォロー 1,000 / 投稿 100 万件規模でのクエリ計画と rows read の測定（SQLite / PG）
