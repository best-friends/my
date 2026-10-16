# ADR-0002: モジュラーモノリス + ヘキサゴナルアーキテクチャ

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #6

## Context

RDB・キュー・ストレージ・実行環境を利用者が差し替えられること（G3, NFR-PORT-1〜4）が最重要の構造要件。
同時に、小規模運営では「1 プロセス・1 DB」で完結する単純さ（NFR-COST-2）が必要。

## Decision Drivers

- 外部資源の差し替え可能性とテスト容易性
- 単一デプロイ単位での運用の簡単さ
- エージェントが局所的な変更で済む明確な境界

## Considered Options

1. マイクロサービス
2. レイヤードなモノリス（境界なし）
3. **モジュラーモノリス + Ports & Adapters**

## Decision（推奨案）

3 を採用する。

- 層: `domain`（純粋）→ `application`（ユースケース・ポート定義）→ `interface`（Mastodon API / ActivityPub / Admin）/ `adapters`（ポート実装）→ `entrypoints`（ターゲット別の組み立て）。
- モジュール（`identity` `account` `content` `graph` `timeline` `notification` `federation` `moderation` `instance`）は
  アプリケーション層の公開 API 経由でのみ相互参照する。DB テーブルの所有者はモジュール単位で決める。
- 依存性注入はコンストラクタ / ファクトリ関数で明示的に行い、DI コンテナは使わない。組み立ては entrypoint の 1 箇所（composition root）。
- 依存方向は lint（例: dependency-cruiser / eslint-plugin-boundaries、Go なら depguard）で CI 検査する。
- モジュール間の非同期連携はドメインイベント → Transactional Outbox → JobQueue。

## Consequences

- 新しいクラウドへの対応 = アダプタ + entrypoint の追加で済み、コアは変更不要。
- 抽象化の分だけボイラープレートは増える。ポートは「2 つ以上の実装が現に存在する or 確実に必要」なものに限る。
- アーキテクチャ境界の lint 設定は M1 の Walking Skeleton で導入する。
