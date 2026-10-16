# ADR-0001: サーバーの実装言語は Go とし、Cloudflare Workers は対象外とする

- Status: Accepted
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #2

## Context

候補は Go と Bun（TypeScript）。ホスト先として Cloudflare / Google Cloud / AWS / Self-hosted の 4 系統を視野に入れていた（G2）。
この決定は DB ライブラリ・キュー実装・連合ライブラリ・Web / モバイルとのコード共有・ツールチェーン・CI のすべてに波及するため、最初に確定させる必要がある。

## Decision Drivers

1. **低コスト（G1）**: 各ターゲットで最安構成が組めるか
2. **Self-hosted のフットプリントと配布の容易さ（NFR-COST-3, NFR-DIST-2）**
3. **連合の正しさ（G4）**: HTTP Signatures、JSON-LD の揺れ、Mastodon 拡張への追従
4. **可搬性（G2, NFR-PORT）**
5. **クライアントとの共有（G5, G6）**
6. **コントリビュータとエージェントの生産性**

## Considered Options

### A. Go（単一バイナリ）

- Self-hosted / Cloud Run / Lambda（`provided.al2023`）/ k8s で最も軽量。メモリ数十 MB、配布は単一の静的バイナリ。
- 連合ライブラリは `go-ap` 程度で、採用実績のある汎用ライブラリは少ない。相互運用の細部は自前で詰める（GoToSocial は参考実装として有用）。
- Cloudflare Workers（V8）ではネイティブに動かない（Wasm か Cloudflare Containers が必要）。
- Web / Tauri の UI は TypeScript になるため 2 言語体制。

### B. TypeScript（Web 標準 API 準拠コア + Bun）+ Fedify

- workerd・Bun・Node で同一コードが動き、Cloudflare Workers をファーストクラスで支えられる。Fedify で連合互換性を外部化できる。
- Self-hosted のメモリは Go より大きい。Workers の CPU 時間・バンドルサイズ制約を意識した実装が必要。

### C. ハイブリッド（Go コア + Cloudflare 用 TS シム）

- 2 言語・2 ランタイムで保守コストが最大。

## Decision

**A. Go を採用する。Cloudflare Workers は実行環境の選択肢から除外する。**

- B の主な利点は「Cloudflare Workers をネイティブに支えられること」だった。Workers を対象外とすると、
  残るターゲット（Google Cloud / AWS / Self-hosted）ではいずれもコンテナ / 単一バイナリが素直に動き、Go の軽さ・配布の容易さ・実行効率が上回る。
- 連合の互換性は Fedify に頼れない分、**フィクスチャ駆動の回帰テストとライブ相互運用テスト**（ADR-0006, ADR-0015）で担保する。
- Cloudflare のサービスのうち、R2（S3 互換 API）や CDN は他ターゲットと組み合わせて引き続き利用できる。
  Cloudflare Containers 上での実行は v0 の対象外とし、需要があれば別 ADR で検討する。

### 言語・ツールチェーン

- サーバー: Go（バージョンは `.tool-versions` で固定）。lint は `golangci-lint`。
- Web クライアント・Tauri の UI: TypeScript。ツールチェーンは Bun（ADR-0008）。
- 共有物（i18n カタログ・API スキーマ）は言語非依存のファイル形式（ICU MessageFormat の JSON、OpenAPI 等）で持ち、両言語から読む / 生成する。

## Consequences

- 連合の署名ライブラリと自前実装の範囲は ADR-0006、SQL ツールとドライバ（いずれも cgo 不要）は ADR-0003 に定める。

- 対応ターゲットは **Google Cloud / AWS / Self-hosted（Docker, microk8s）の 3 系統** になる。
  Cloudflare Workers・D1・Cloudflare Queues 向けのアダプタとエントリポイントは v0 では作らない。
- D1 が対象外になったため、DB で対話的トランザクションを使えないという制約は緩和できる（ADR-0003 で再検討）。
- サーバーは単一バイナリの中で `serve` / `worker` / `cli` を切り替える構成にでき、Self-hosted の最小構成は「バイナリ + SQLite ファイル」になる。
- 2 言語体制のため、i18n カタログと API 型の共有は生成ツールで行う。
- `CGO_ENABLED=0` の静的ビルドを維持したいので、cgo が必要な依存（SQLite ドライバ、画像処理）の選定は spike で確認する。

## Open Questions / 要検証

- [ ] Self-hosted のアイドル時メモリの実測（M1）
