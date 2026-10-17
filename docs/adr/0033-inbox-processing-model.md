# ADR-0033: Inbox の受信処理モデル（同期・非同期の境界、重複排除、順序）

- Status: Accepted
- Date: 2026-10-17
- Deciders: @rosylilly
- Issue: #232

## Context（背景・課題）

[ADR-0006](0006-federation-protocol-scope.md) と NFR-PERF-4 は「Inbox は署名検証まで同期、処理本体は非同期」と決めている。
[overview §4.2](../architecture/overview.md#42-inbox-受信) は「ハンドラは Activity ID で重複排除」と書く。M3 の実装に入る前に、次の点を決める必要がある。

- 同期の部分にどこまで含めるか。特に、署名鍵が未知のときにリモートの Actor を **リクエストの中で** 取得するか（FR-404, FR-407）
- 重複排除の行（`received_activities`）を **受信時** に書くか、**処理後** に書くか（NFR-REL-1）
- 順序。ActivityPub は配送の順序を保証しない。`Create` より先に `Update` / `Delete` が届く、`Accept` より先に `Undo` が届く、といったことが起きる（[ActivityPub §7](https://www.w3.org/TR/activitypub/#server-to-server-interactions)）
- 未知のアクティビティ・オブジェクトの型の扱い（[AS2 Core §5](https://www.w3.org/TR/activitystreams-core/#extensibility) は、未知の拡張で処理を止めないことを求める）

キュー（ADR-0004）は `orderingKey` を持つが、pull runner はまだ直列化しない（#77）。SQS 標準キューと Cloud Tasks は、そもそも同じキーのジョブを直列化できない。

主要実装の方式（出典は [#232 の調査報告](https://github.com/best-friends/my/issues/232)）:

| 実装 | 同期で行うこと | 非同期 | 重複排除 |
| --- | --- | --- | --- |
| Mastodon | 本文の上限（1 MB）、未知の Actor の自分自身の `Delete` / `Update` を検証せず `202`、署名検証（未知の鍵はリクエスト中に取得）、ドメインブロック（`403`） | `ActivityPub::ProcessingWorker`（`retry: 8`） | アクティビティの `id` では行わない。オブジェクトごとの Redis ロックと既存行の照合。`Delete` が先に届いたら 6 時間の印と `Tombstone` |
| GoToSocial | 署名検証と要求者の Actor の解決、ブロック判定（`403`）、ハンドラの軽い部分 | 連合のワーカーキュー（副作用） | 直近 2,048 件のアクティビティ ID のメモリ上のキャッシュと、DB の一意制約 |
| Misskey | 署名ヘッダとダイジェストの形式の検査だけ | inbox キューで署名検証と処理（`attempts` 8） | 下流の一意制約だけ |

## Decision Drivers（判断基準）

1. 偽のリクエストでワーカーとストレージを消費させない（DoS、NFR-COST-3）
2. 送信側のリトライに処理を任せられること（失敗の扱いを単純にする）
3. at-least-once のジョブと、順序が保証されない配送のどちらでも結果が同じになること（NFR-REL-1）
4. どのキューアダプタでも同じ振る舞いになること（ADR-0004。順序をキューに頼らない）
5. 同期の部分を短くし、Lambda / Cloud Run の 1 リクエストあたりの時間とコストを抑えること（NFR-PERF-4）

## Considered Options（選択肢）

1. **署名検証まで同期（未知の鍵はその場で取得）+ 受信時に重複排除 + 順序に依存しないハンドラ**
2. **受信したらすぐキューに入れ、署名検証もワーカーで行う**（Misskey 方式）
3. **選択肢 1 と同じ境界で、重複排除は処理の完了時に記録し、順序は `orderingKey` で保証する**

## Decision（決定）

**選択肢 1 を採用する。**

- **同期の部分**: 本文の上限（1 MiB）→ IP のレート制限 → 署名ヘッダの解析 → 署名者のドメインのブロック判定 → ダイジェストの照合 → 鍵の解決（未知・期限切れなら、署名した Actor をインスタンス Actor の署名付き GET で取得。5 秒で打ち切る）→ 署名の検証（[ADR-0032](0032-http-signature-profile.md)）→ 本文の JSON から `id` / `type` / `actor` だけを読む → 署名者と `actor` の照合 → 重複排除の行と原本とジョブを 1 つのトランザクションで記録 → `202`。
  - 鍵を取得できなければ `401` を返し、送信側のリトライに任せる。
  - 未知の Actor の自分自身の `Delete` は、鍵を取りに行かず `202` で捨てる（ADR-0019 §7）。
- **重複排除は受信時**: `received_activities (activity_uri)` に `ON CONFLICT DO NOTHING` で挿入し、行が増えたときだけ原本（`received_activity_documents`）と `federation.inbox` ジョブを同じトランザクションで記録する。
  再送された同じアクティビティはジョブを作らない。処理に失敗したアクティビティは dead job として残り、CLI から再実行する（FR-801）。
- **署名者と `actor` が一致しない**（転送・リレー）アクティビティは、ジョブに「信用しない」の印を付け、ワーカーがオブジェクトを `id` からオリジンに取り直して検証する（[ActivityPub 付録 B.2](https://www.w3.org/TR/activitypub/#security-verification)、ADR-0019 §7）。
- **順序はキューに頼らない**: ジョブには `orderingKey = inbox:<actor の URI>` を付ける（#77 の後、同じ Actor のアクティビティは直列化される）が、ハンドラはどの順で届いても同じ結果になるように書く。
  - 未知のオブジェクトへの `Delete` は tombstone（`remote_object_tombstones`）を記録し、後から届いた `Create` / `Update` を捨てる。
  - 未知のオブジェクトへの `Update` は、取り込み範囲（[ADR-0034](0034-remote-content-scope-and-retention.md)）に入るなら `Create` と同じく扱う。
  - 版は `updated` が現在版より新しいときだけ作る（[data-model §2.2](../architecture/data-model.md#22-冪等キーと一意制約)）。
  - 対象が見つからない `Undo` / `Accept` / `Reject` / `Like` / `Announce`（ローカルの投稿宛て）は何もせず完了にする。
- **未知の型**: 対応表にないアクティビティ・オブジェクトの型は、ログとメトリクスに残して完了にする（`202` は同期の時点で返している）。

### この決定を見直す条件

- 同期の Actor 取得が、Lambda / Cloud Run の計測で p95 の応答時間やコストを許容できないほど押し上げる場合（選択肢 2 が有力になる）
- 受信時の重複排除で、処理に失敗したアクティビティの再送を受け付けないことが運用上の問題になる場合（選択肢 3 が有力になる）

## Consequences（影響）

- 良い影響
  - 偽の署名・未知の鍵・ブロック済みドメインのリクエストは、DB にもキューにも触れずに終わる。
  - 同じアクティビティの再送は、1 回の一意制約の照合で終わる（Mastodon は配送を指数バックオフで再送する）。
  - キューアダプタの順序保証の差に結果が左右されない。#77 は性能と競合の削減のためで、正しさの前提ではない。
- 悪い影響・トレードオフ
  - 未知の Actor からの最初のリクエストは、Actor の取得の分だけ遅くなる。リクエストの中で外部 I/O をする（トランザクションの外なので ADR-0003 には反しない）。
  - 受信時に記録するので、処理に失敗したアクティビティは送信側が再送しても処理し直されない。dead job の監視と再実行が必要になる。
  - tombstone の保持期間（既定 30 日）より後に届いた古い `Create` は取り込まれうる。
- 後続の作業（Issue 化する）
  - Inbox の同期部分（署名の検証は ADR-0032 のタスク）と、受信の記録・ジョブ投入
  - 種類ごとのハンドラと、順序を入れ替えたフィクスチャの回帰テスト
  - `remote_object_tombstones` と保持期間の消去ジョブ（FR-803）

## Pros and Cons of the Options（詳細比較・任意）

### 1. 署名検証まで同期 + 受信時に重複排除 + 順序に依存しない（推奨）

- 良い: 判断基準 1〜4 を満たす。Mastodon / GoToSocial と同じ境界で、相手の再送の挙動と合う。
- 悪い: 同期の Actor 取得が判断基準 5 と緊張関係にある。

### 2. すぐキューに入れる（Misskey 方式）

- 良い: リクエストは最短で終わる。Actor の取得もワーカーで行える。
- 悪い: 署名を検証する前にストレージとキューを消費するので、偽のリクエストで課金単位（ジョブ数）が増える。署名が不正でも `202` を返すので、送信側に失敗が伝わらない。NFR-PERF-4 の文言（署名検証は同期）とも合わない。

### 3. 処理後に重複排除 + `orderingKey` で順序を保証

- 良い: 失敗したアクティビティを相手の再送で処理し直せる。順序が保証されればハンドラが単純になる。
- 悪い: SQS 標準キュー・Cloud Tasks では順序を保証できない（判断基準 4）。同じアクティビティの同時の再送が両方ジョブになる。

## Open Questions / 要検証

- [ ] 同期の Actor 取得の所要時間（Cloud Run / Lambda のコールドスタートを含む）の計測
- [ ] tombstone の保持期間 30 日の妥当性（Mastodon が `Delete` を先に受けたときの保持期間との比較）
