# ADR-0011: ライセンスは AGPL-3.0 とする

- Status: Accepted（`pkg/mykit`・`pkg/mybot`・`api/spec/` のライセンスを ADR-0093（API クライアントライブラリ pkg/mykit）が置き換える）
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #12

## Context

OSS として配布する（NFR-DIST-1）。ネットワークサービスであり、利用者が自由にカスタマイズ・セルフホストすることを想定する。
Fediverse の主要サーバー実装（Mastodon、Misskey、GoToSocial、Pleroma/Akkoma）は AGPL 系を採用している。

## Considered Options

1. **AGPL-3.0**
2. AGPL-3.0（サーバー）+ MIT / Apache-2.0（クライアント SDK・ユーティリティ）
3. Apache-2.0（全体）
4. MIT（全体）

## Decision

**リポジトリ全体を AGPL-3.0（SPDX: `AGPL-3.0-only`）とする。**

- 改変版をネットワーク越しに提供する事業者にもソース公開を求め、コミュニティへの還元を担保する。Fediverse の主要実装と同じ立場をとる。
- ルートに `LICENSE`（GNU 公式テキスト）を置く。ソースファイルには SPDX ヘッダ（`// SPDX-License-Identifier: AGPL-3.0-only`）を付ける。
- AGPL 第 13 条に従い、サーバーは利用者にソースの入手先を示す（NodeInfo の `software.repository`、Web クライアントのフッター / about ページ）。
  ソース URL は設定で変更でき、改変版の運営者が自分のリポジトリを指せるようにする。

## Consequences

- 依存ライブラリは AGPL-3.0 と両立するもの（MIT / BSD / Apache-2.0 / MPL-2.0 等）に限る。依存ライセンスの検査を CI に入れる（別 Issue）。
- 企業による採用の障壁になり得る点は受け入れる。
- 将来、一部（クライアント SDK 等）を MIT / Apache-2.0 で出したくなった場合、外部コントリビュータの同意が必要になる。
  外部コントリビュートを受け入れる前に、DCO / CLA の扱いと合わせて決める（別 Issue）。
