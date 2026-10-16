# ADR-0001: サーバーの実装言語とランタイム

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: <!-- 起票後に追記 -->

## Context

候補は Go と Bun（TypeScript）。ホスト先として Cloudflare / Google Cloud / AWS / Self-hosted の 4 系統を同一コードベースで支えたい（G2）。
この決定は DB ライブラリ・キュー実装・連合ライブラリ・Web / モバイルとのコード共有・ツールチェーン・CI のすべてに波及するため、最初に確定させる必要がある。

## Decision Drivers

1. **低コスト（G1）**: 4 ターゲットそれぞれで最安構成が組めるか。特に Cloudflare Workers（最も安くなりやすい）で素直に動くか
2. **連合の正しさ（G4）**: HTTP Signatures（draft-cavage / RFC 9421）、JSON-LD の揺れ、Mastodon 拡張への追従を自前で抱えずに済むか
3. **可搬性（G2, NFR-PORT）**: ランタイム固有 API に依存しないコアを書けるか
4. **Self-hosted のフットプリント（NFR-COST-3）**: 1GB VM で快適に動くか、配布の容易さ
5. **クライアントとの共有（G5, G6）**: i18n カタログ・API 型・バリデーションを Web / Tauri と共有できるか
6. **コントリビュータとエージェントの生産性**

## Considered Options

### A. Go（単一バイナリ）

- Self-hosted / Cloud Run / Lambda（`provided.al2023`）/ k8s では最も軽量で高速。メモリ数十 MB、配布は単一バイナリ。
- 連合ライブラリ: `go-ap`（RFC 9421 対応の報告あり、要検証）。採用実績のある汎用ライブラリは少なく、相互運用の細部は自前で詰めることになる
  （GoToSocial は参考実装として有用だがライブラリではない）。
- **Cloudflare が弱点**: Workers は V8 ランタイム。Go は (a) Go→Wasm（バンドル上限・CPU 時間・D1 / Queues バインディングの扱いが実験的）か、
  (b) Cloudflare Containers（Worker から起動するコンテナ、D1 / Queues へは Worker シム経由、課金体系が Workers と別、要検証）になる。
  Cloudflare では「Workers + D1 + Queues + R2 だけで完結」が難しい。
- Web / Tauri の UI は結局 TypeScript になるため、2 言語体制（i18n カタログ・API 型は生成で共有）。

### B. TypeScript（Web 標準 API で書き、Bun を主ランタイム / ツールチェーンにする）

- コアを **Web 標準 API（fetch / Request / Response / WebCrypto / Streams）** だけで書けば、同じコードが
  workerd（Cloudflare）・Bun（Self-hosted / Cloud Run）・Node.js（Lambda）で動く。
  Bun 固有 API（`Bun.sql` / `bun:sqlite` / `Bun.file` 等）はアダプタとエントリポイントに閉じ込める。
- 連合ライブラリ: **Fedify**（Node / Deno / Bun 対応、Cloudflare Workers 対応パッケージ `@fedify/cfworkers` あり）。
  HTTP Signatures（double-knocking 含む）、WebFinger、NodeInfo、Object Integrity Proofs、KvStore / MessageQueue の抽象を提供しており、
  本プロジェクトのポート設計と思想が一致する。
- DB: Kysely / Drizzle が PostgreSQL・SQLite・D1・libSQL の方言を持つ。
- Web(PWA)・Tauri 2 の UI・i18n カタログ・API 型を 1 言語で共有できる。
- 弱点: Self-hosted のメモリは Go より大きい（数十〜100MB 程度、要検証）。`bun build --compile` で単一実行ファイル化は可能だがサイズは大きい。
  Workers の CPU 時間・バンドルサイズ制限を意識した実装が必要。

### C. ハイブリッド（Go コア + Cloudflare 用 TS シム）

- Cloudflare では TS の Worker がバインディング（D1 / Queues / R2）を扱い、Go（Containers）に HTTP で中継。
- 2 言語・2 ランタイムで保守コストが最大。Cloudflare の最安構成の利点も薄れる。

## Decision（推奨案）

**推奨: B. TypeScript（Web 標準 API 準拠コア + Bun を主ランタイム / ツールチェーン）+ Fedify。**

理由:

- 判断基準 1・2 で決定的な差がある。Cloudflare Workers をファーストクラスで支えられ、Fedify により連合の互換性問題を大きく外部化できる。
- 判断基準 5 で、i18n（G5）とモバイル（G6）を 1 言語で回せる。
- 判断基準 4 の Go の優位は、Self-hosted を主戦場にするなら効くが、本プロジェクトの「4 ターゲット可搬」「最安運用」という目標の下では B の利点が上回る。

**この推奨を覆す条件**: Cloudflare を「対応ターゲット」から外す、または Self-hosted のフットプリント最優先とするなら A（Go）が有力。

既存の `go.mod` / `.tool-versions` の `golang` `golangci-lint` は、本 ADR の決定後に整理する。

## Consequences（B を採用した場合）

- コアパッケージは `lib: ["ES2023", "WebWorker"]` 相当の型環境でビルドし、Node / Bun 固有 API の import を lint で禁止する。
- ランタイム互換性 CI: コアのテストを Bun と workerd（Miniflare / `@cloudflare/vitest-pool-workers` 等）の両方で実行する。
- Lambda は Node.js マネージドランタイム（Bun カスタムランタイムは任意）。
- ツールチェーン: Bun（パッケージ管理・テストランナー・バンドル）、TypeScript strict、Biome（lint/format）を候補とする（別 Issue）。

## Open Questions / 要検証

- [ ] Spike: Fedify + Hono の最小 Actor を Bun と Workers の両方で起動し、Mastodon からフォローできるか
- [ ] Spike: Workers でのバンドルサイズ・CPU 時間（RSA 署名検証、Argon2 / PBKDF2）の実測
- [ ] Self-hosted で Bun プロセスのアイドル時メモリの実測
- [ ] （A を選ぶ場合）go-ap の保守状況と RFC 9421 実装の確認、Cloudflare Containers の価格・GA 状況の確認
