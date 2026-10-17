# ADR-0032: HTTP 署名の受信・送信プロファイル

- Status: Accepted
- Date: 2026-10-17
- Deciders: @rosylilly
- Issue: #232

## Context（背景・課題）

FR-404 は draft-cavage-12 と RFC 9421 の両方の署名と double-knocking を求める。
[ADR-0006](0006-federation-protocol-scope.md) は使うライブラリと受信の振り分け（`Signature-Input` があれば RFC 9421）を決めたが、次の点が残っている。

- **送信の既定の方式が 2 つの Accepted ADR で食い違っている。**
  ADR-0006 は「RFC 9421 を先に試し、400/401/403 なら draft-cavage で再送し、成功した方式をホストごとに覚える（Mastodon 4.7 と同じ方針）」とし、
  [ADR-0019](0019-security-baseline.md) §7 は「送信は draft-cavage-12（`rsa-sha256`）を既定とし、受信側が RFC 9421 を要求したら double-knocking で切り替える」とする。
- ADR-0006 の Open Questions: Mastodon が RFC 9421 の受信で必須とするコンポーネント、送信時の `alg` / `expires`、GoToSocial 式のクエリ文字列の再検証
- 鍵の解決・キャッシュ・ローテーションの扱い（ADR-0019 §7 は TTL 24 時間と再取得 1 回を決めている）

2026-10-17 に Mastodon の main（4.8.0-dev）のソースで確認したこと（出典は [#232 の調査報告](https://github.com/best-friends/my/issues/232)）:

- **送信は draft-cavage が先**。400 / 401 が返ったら RFC 9421（`@method @target-uri content-digest`）で 1 回だけ再送する。ホストごとの記憶はない（`app/lib/request.rb`）。
  ADR-0006 の「Mastodon 4.7 は RFC 9421 を先に試す」という記述は、現在のソースと合わない。
- RFC 9421 の受信で必須なのは、パラメータ `keyid` と `created`、コンポーネント `@method` と `@target-uri`、POST では `content-digest`（`sha-256` だけ）。`alg` は見ず、鍵の種類で決める。`expires` が無ければ `created + 5 分` とみなす（`app/lib/signed_request.rb`）。
- 時間窓は `EXPIRATION_WINDOW_LIMIT = 12.hours` と `CLOCK_SKEW_MARGIN = 1.hour`（ADR-0019 §7 と同じ）。
- draft-cavage の受信では、`(request-target)` をクエリ文字列付きで検証し、合わなければクエリ文字列なしで検証し直す。
- GoToSocial は draft-cavage だけに対応する（RFC 9421 は未対応）。送信は `algorithm="hs2019"`（実際は RSA-SHA256）で、GET は `(request-target) host date`、POST は加えて `digest` に署名する。受信はクエリ文字列付き → なしの順に検証し、送信は 401 ならクエリ文字列なしで再送する。
- Misskey（develop）も draft-cavage（`rsa-sha256`）だけで、RFC 9421 の記述はソースに無い。

## Decision Drivers（判断基準）

1. 主要実装（Mastodon・GoToSocial・Misskey・Akkoma）の現行版と旧版のどちらにも、追加の往復なしで届くこと（G4、NFR-COST-5: リクエスト数は課金単位）
2. 受信で、正しく署名されていないリクエストを受け入れないこと（ADR-0019 §7）
3. 送信者を不必要に拒否しないこと（ADR-0019 の判断基準 3）
4. 将来 RFC 9421 だけを受け付ける実装が増えても、設定変更なしで追従できること
5. 2 つの Accepted ADR の食い違いを解消できること

## Considered Options（選択肢）

1. **送信は draft-cavage を先に試し、400 / 401 なら RFC 9421 で再送する。成功した方式をホストごとに TTL 付きで覚える**（Mastodon と同じ順序 + 記憶）
2. **送信は RFC 9421 を先に試し、400 / 401 / 403 なら draft-cavage で再送する。成功した方式をホストごとに覚える**（ADR-0006 の記述）
3. **送信は draft-cavage だけ**（ADR-0019 §7 の文言どおりで、double-knocking は受信側の要求があったときだけ）

## Decision（決定）

**選択肢 1 を採用する。** 受信と送信のプロファイルを次のとおりにする。

### 受信

| 項目 | draft-cavage-12 | RFC 9421 |
| --- | --- | --- |
| 選ぶ条件 | `Signature-Input` がない | `Signature-Input` がある（`Signature` と両方あれば RFC 9421 だけを検証する） |
| 必須の対象 | `(request-target)`、`host`、`date`（または `(created)`）。POST は加えて `digest` | `@method`、`@target-uri`（または `@authority` + `@path`、クエリ文字列があれば `@query`）、パラメータ `keyid` と `created`。POST は加えて `content-digest` |
| ダイジェスト | `Digest: SHA-256=…`（RFC 3230）。`SHA-512` も受ける | `Content-Digest`（[RFC 9530](https://www.rfc-editor.org/rfc/rfc9530)）の `sha-256` / `sha-512` |
| アルゴリズム | `rsa-sha256`、`hs2019`（鍵の種類から決める） | `alg` があれば鍵の種類と一致することを確かめ、なければ鍵の種類から決める。RSA（2048 ビット以上）の `rsa-v1_5-sha256` / `rsa-pss-sha512` と `ed25519` |
| 時間窓 | `Date` が過去 12 時間より古い・1 時間より未来なら拒否 | `created` が 1 時間より未来なら拒否。`expires`（無ければ `created + 12 時間`）を 1 時間過ぎていたら拒否 |
| クエリ文字列 | クエリ文字列付きで検証し、合わなければクエリ文字列なしで 1 回だけ検証し直す | `@target-uri` は署名されたとおりに組み立てる（再検証しない） |

- `@target-uri` と `(request-target)` は設定の公開オリジンとリクエストのパス・クエリから組み立て、`X-Forwarded-*` は信用しない（ADR-0006）。
- 鍵の解決: `keyid` / `keyId` の値そのもので `account_keys` を引く（Mastodon は `{actor}#main-key`、GoToSocial は `{actor}/main-key`）。無い・取得から 24 時間を過ぎている場合は、フラグメントを除いた URL を取得する。
  取得した文書が `Key` なら `owner`（または `controller`）の Actor をたどり、`publicKey`（オブジェクト・配列・ID のどれでもよい）から `id == keyId` のものを使う。
  FEP-521a の `assertionMethod`（`Multikey`）があれば、Ed25519 の鍵もそこから読む。
- 検証に失敗したら、`keyId` ごとに 5 分に 1 回まで Actor を取り直して 1 回だけ再検証する（鍵のローテーション。ADR-0019 §7）。同じ `keyId` で公開鍵が変わったら、古い鍵を失効させて新しい鍵を記録する。
- `Accept-Signature`（[RFC 9421 §5](https://www.rfc-editor.org/rfc/rfc9421#section-5)）は、401 の応答に RFC 9421 の必須の対象を載せて返す。

### 送信

- 既定は draft-cavage-12、`algorithm="rsa-sha256"`、`keyId` は `{actor}#main-key`。
  POST は `(request-target) host date digest content-type`（`Digest: SHA-256=…`）、GET は `(request-target) host date` に署名する。
- 400 / 401 が返ったら、RFC 9421 で 1 回だけ再送する。対象は `("@method" "@target-uri" "content-digest");created=…;expires=…;keyid="…#main-key";alg="rsa-v1_5-sha256"`（`expires` は `created + 5 分`）。
- 再送が成功したら、そのホストは RFC 9421 で送ることを KV に 7 日記憶し、以後は RFC 9421 を先に試す（失敗したら draft-cavage で 1 回だけ再送する）。
- 相手の 401 に `Accept-Signature` があれば、それに従って RFC 9421 で再送する。
- GET（Actor・オブジェクトの取得）も同じ規則で、インスタンス Actor の鍵で署名する。
- `ed25519` での送信は、主要実装の受信の対応を確かめるまで既定にしない（Mastodon 4.7 以降は受信できる）。

### ADR-0006 と ADR-0019 との関係

この ADR が Accepted になったら、ADR-0006 の「送信は RFC 9421 を先に試し…」と ADR-0019 §7 の最後の項目は、この ADR に置き換わる（Decider が両 ADR の該当箇所に参照を付ける）。

### この決定を見直す条件

- 主要実装のいずれかが draft-cavage を受け付けなくなった場合（選択肢 2 が有力になる）
- 送信のリクエスト数の計測で、double-knocking の再送が配送全体の数 % を超える場合（記憶の TTL か順序を見直す）

## Consequences（影響）

- 良い影響
  - GoToSocial（draft-cavage だけ）と Mastodon（どちらも受ける）には 1 回で届く。RFC 9421 だけを受ける相手には、最初の 1 回だけ 2 回になり、以後は 1 回になる。
  - 受信の必須の対象が Mastodon の送信（draft-cavage と RFC 9421 の両方）を受け入れ、それより弱い署名は拒否する。
  - 2 つの Accepted ADR の食い違いが解消する。
- 悪い影響・トレードオフ
  - draft-cavage は仕様として古い（RFC 9421 に置き換えられた）。主要実装が RFC 9421 に移ったら順序を見直す必要がある。
  - クエリ文字列の再検証の分だけ、draft-cavage の検証が最大 2 回になる。
- 後続の作業（Issue 化する）
  - 署名と検証のアダプタ（`dadrus/httpsig` / `code.superseriousbusiness.org/httpsig`）と、受け入れポリシーの実装
  - 鍵の解決・キャッシュ・ローテーション
  - 送信の double-knocking とホストごとの記憶
  - 実サーバーとの往復確認（#44）で、この表の各行を確かめる

## Pros and Cons of the Options（詳細比較・任意）

### 1. draft-cavage が先 + RFC 9421 へのフォールバック + 記憶（推奨）

- 良い: 判断基準 1・3・4・5 を満たす。Mastodon の現行の送信と同じ順序なので、受信側の実装が Mastodon を前提にしていても通る。
- 悪い: 古い方式を既定に残す。

### 2. RFC 9421 が先

- 良い: 新しい仕様を既定にする。
- 悪い: GoToSocial と旧版の Mastodon（4.4 以前）・Misskey など RFC 9421 を受けない相手には、記憶が切れるたびに 2 回送る（判断基準 1）。根拠にした「Mastodon 4.7 と同じ方針」は現在のソースと合わない。

### 3. draft-cavage だけ

- 良い: 最も単純。
- 悪い: RFC 9421 だけを受ける実装が現れたら届かない（判断基準 4）。FR-404 の double-knocking を満たさない。

## Open Questions / 要検証

- [ ] Akkoma の RFC 9421 の受信と送信の対応状況（#44 で確かめる）
- [ ] 記憶の TTL 7 日の妥当性
