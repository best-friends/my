# ADR-0011: ライセンス

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: <!-- 起票後に追記 -->

## Context

OSS として配布する（NFR-DIST-1）。ネットワークサービスであり、利用者が自由にカスタマイズ・セルフホストすることを想定する。
Fediverse の主要サーバー実装（Mastodon、Misskey、GoToSocial、Pleroma/Akkoma）は AGPL 系を採用している。

## Considered Options

1. **AGPL-3.0-only（サーバー）+ MIT / Apache-2.0（クライアント SDK・ユーティリティ）**
2. Apache-2.0（全体）
3. MIT（全体）

## Decision（推奨案）

判断はプロジェクトオーナーに委ねる（ビジネス上の判断を含むため）。参考として:

- **1 を選ぶ場合**: 改変版をホストする事業者にもソース公開を求められ、コミュニティへの還元が担保される。一方、企業による採用の障壁になり得る。
  依存ライブラリ（Fedify: MIT 等）との互換性は問題ない。
- **2 を選ぶ場合**: 採用障壁が最も低く、特許条項もある。改変版のクローズド運用を許容する。

どちらの場合も、外部コントリビュータには DCO（`Signed-off-by`）を求め、CLA は採用しない方針を推奨する。

## Consequences

- ライセンス決定は公開（リポジトリの public 化）前に必須。依存ライブラリのライセンス検査を CI に入れる。
