# ADR-0015: テスト戦略

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #14

## Context

エージェントが自律的に作業するには、変更の正しさを機械的に判定できる必要がある（NFR-DEV-1, 2）。
加えて、アダプタの差し替え可能性（NFR-PORT-3）と連合互換性（G4）を継続的に保証したい。

## Decision（推奨案）

| レベル | 対象 | 実行環境 | 実行タイミング |
| --- | --- | --- | --- |
| Unit | domain / application（フェイクのポート使用） | ローカル・CI | 毎 PR |
| Contract | 各ポートの全アダプタ（同一スイート） | SQLite in-memory / PG（コンテナ）/ MinIO / LocalStack（SQS）/ Cloud Tasks エミュレータ | 毎 PR（クラウド実体が必要なものは nightly） |
| Build matrix | `CGO_ENABLED=0` で linux/amd64・linux/arm64 をビルド、ビルドタグでアダプタを除外してもビルドできる | CI | 毎 PR |
| API conformance | Mastodon API のレスポンススキーマ | CI | 毎 PR |
| Federation fixtures | 実サーバー由来の JSON を入力にした受信・生成の回帰テスト | CI | 毎 PR |
| Federation live | docker compose で Mastodon / GoToSocial 等と実際に連合 | CI | nightly / 手動 |
| E2E (web) | Playwright。`en`, `ja`, 疑似ロケール `en-XA`, `ar-XB` | CI | 毎 PR（スモーク）/ nightly（全体） |
| Deploy smoke | 各ターゲットへデプロイしてヘルスチェック + 投稿 | 実クラウド | nightly / リリース前 |

- `make check` = lint + 型検査 + Unit + Contract（ローカル完結分）+ i18n 検査 + docs 検査。外部ネットワーク不要。
- テストは決定的に: `Clock` / `IdGenerator` / `HttpClient` を注入し、ネットワークはフィクスチャで置き換える。
- カバレッジは指標としてのみ使い、閾値で CI を落とさない（domain 層は高く保つ）。

## Consequences

- コントラクトテストのおかげで、利用者が自作したアダプタも同じ基準で検証できる（`testkit` を公開パッケージにする）。
- nightly ジョブ用のクラウドアカウント・シークレット管理が必要（M4）。
