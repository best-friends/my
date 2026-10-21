# ADR-0075: my.surface に payments を加える

- Status: Accepted
- Date: 2026-10-22
- Deciders: @rosylilly
- Issue: #919

## Context（背景・課題）

[ADR-0055](0055-service-observability.md) §3 は、HTTP サーバーの指標（`http.server.request.duration`・`http.server.active_requests`）のラベル `my.surface` の値を `client_api` / `activitypub` / `oauth` / `web` / `media` / `admin` に決めている。
[ADR-0074](0074-payment-provider-port.md) §3 は、決済の事業者が呼ぶ webhook の受け口 `POST /payments/webhooks/{provider}` を新しい `internal/interface/payments` に置く。
この経路はクライアント API の外にあり、利用者は呼ばない。運営者は、ここへの要求の数とエラー（署名の誤りの `400`、DB の失敗の `503`）を、ほかの面と分けて見る必要がある。webhook が届いていないことや、署名の設定が誤っていることに気付くためである。

決定者の回答（PR #936、原文）:

> 一覧に payments を加える。ADR-0055 の書き換えは小さな ADR で行い、それは Accepted とする。ボーイスカウトルールで PR に含めて良い。

## Decision Drivers（判断基準）

1. 決済の webhook の件数・遅延・エラー率を、ほかの面と混ぜずに見られること
2. ラベルの値の数（時系列の数）を小さく保つこと（ADR-0055 §3）
3. ADR-0055 の本文を書き換えずに済むこと

## Considered Options（選択肢）

1. `web` に含める
2. `payments` を加える

## Decision（決定）

**選択肢 2 を採用する。** ADR-0055 §3 の HTTP サーバーの指標の `my.surface` の値に `payments` を加え、`POST /payments/webhooks/{provider}` の要求に付ける。

この ADR は、ADR-0055 §3 の `my.surface` の値の一覧だけを置き換える。ADR-0055 は Accepted のままで、その他の決定と本文は変えない。ADR-0055 の Status の行に、この ADR が置き換えることを注記する（ADR-0066 が ADR-0016・ADR-0029 にしたのと同じ）。

## Consequences（影響）

- 良い影響
  - webhook の件数とエラー率を `my.surface = payments` で絞って見られる。webhook が届かないことや署名の誤りに気付きやすい。
- 悪い影響・トレードオフ
  - ラベルの値が 1 つ増え、HTTP の指標の時系列が少し増える。課金が無効なら、この面には要求が来ない。
- 後続で必要になる作業
  - なし（実装と運用の文書の一覧は PR #936 で更新する）

## Pros and Cons of the Options（詳細比較・任意）

### 選択肢 1: `web` に含める

- 良い点: 値が増えず、ADR-0055 の一覧のままでよい。
- 悪い点: `web` は Web クライアントのページの面で、ページの要求の数に紛れて webhook の失敗が見えにくい。呼ぶのが事業者のサーバーであり、性質も違う。

### 選択肢 2: `payments` を加える

- 良い点: 事業者が呼ぶ経路の件数とエラーを分けて見られる。`activitypub` の inbox がほかのサーバーから呼ばれる面として分かれているのと揃う。
- 悪い点: 値が 1 つ増える。
