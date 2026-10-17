# 連合の設計（ActivityPub S2S）

> 前提にしている ADR: [ADR-0002](../adr/0002-modular-monolith-hexagonal.md)、[ADR-0003](../adr/0003-database-abstraction.md)、[ADR-0004](../adr/0004-job-queue-abstraction.md)、[ADR-0005](../adr/0005-blob-storage-and-media.md)、[ADR-0006](../adr/0006-federation-protocol-scope.md)、[ADR-0017](../adr/0017-visibility-local.md)、[ADR-0018](../adr/0018-federated-uris.md)、[ADR-0019](../adr/0019-security-baseline.md)、[ADR-0020](../adr/0020-data-modeling-principles.md)。
> 署名は [ADR-0032](../adr/0032-http-signature-profile.md)、受信処理は [ADR-0033](../adr/0033-inbox-processing-model.md)、取り込み範囲と保持は [ADR-0034](../adr/0034-remote-content-scope-and-retention.md)、`local` 投稿のメンションは [ADR-0035](../adr/0035-local-visibility-mention-tags.md) に従います。
> 要件: FR-305、FR-401〜410、FR-701〜702、NFR-SEC-1〜2、NFR-SEC-4〜5、NFR-PERF-4、NFR-REL-1〜2、NFR-COST-4〜5。テーブルは [論理データモデル](data-model.md) §4.8 ほかにあります。

## 1. 構成

### 1.1 モジュールとパッケージ

| パッケージ | 持つもの |
| --- | --- |
| `internal/domain/federation` | AS2 の正規化した型と、受信した JSON からの正規化（§4）。宛先から公開範囲への写像、配送先の算出規則、配送の結果の分類、サーキットブレーカーの規則。標準ライブラリだけを使う純粋な関数 |
| `internal/app/federation` | ユースケース（受信 `Receive`、アクティビティの処理、配送の展開 `FanOut`、配送 `Deliver`、Actor / オブジェクトの取得 `Fetch`、解決 `Resolve`）、ジョブハンドラ、ポート（`Signer`・`Verifier`・リポジトリ） |
| `internal/app/kernel` | `HTTPClient` ポート（連合のほか、メディアプロキシ・リンクプレビュー・Web Push も使うので全モジュール共通に置く） |
| `internal/adapter/httpclient` | SSRF ガード付きの `HTTPClient`（ADR-0019 §1）。コントラクトテストは `testkit.HTTPClientContract` |
| `internal/adapter/htmlsanitizer` | `kernel.HTMLSanitizer` ポートの実装（§5、ADR-0019 §2）。`bluemonday` はこの中に閉じ込める。ポリシーの版は定数 `PolicyVersion`。コントラクトテストは `testkit.HTMLSanitizerContract` |
| `internal/adapter/httpsig` | `Signer` / `Verifier` の実装。RFC 9421 は `github.com/dadrus/httpsig`、draft-cavage-12 は `code.superseriousbusiness.org/httpsig`（ADR-0006）。ライブラリはこの中に閉じ込める |
| `internal/interface/activitypub` | HTTP の入口（Inbox の `POST`、Actor・オブジェクト・コレクションの `GET`、WebFinger、NodeInfo）。ユースケースを呼ぶだけ |
| `internal/interface/media` | メディアプロキシ `/media/proxy/…`（§9） |

- 受け入れのポリシー（必須の対象・時間窓・鍵と `actor` の一致）は `internal/app/federation` に書き、暗号処理と署名ベースの組み立てだけをアダプタに任せます（ADR-0006）。
- テーブルの所有者はモジュール単位です（ADR-0002）。`federation` が書くのは §4.8 のテーブルだけで、リモートのアカウント・投稿・フォローなどは、それぞれのモジュールのアプリケーション層の公開 API を **同じトランザクションの中で** 呼んで書きます（利用者登録と同じ形。[data-model §2.1](data-model.md#21-トランザクションの規律)）。

| 受信した内容 | 呼ぶ公開 API（書くモジュール） |
| --- | --- |
| Actor の文書 | `account.UpsertRemoteActor`（`accounts`・`remote_accounts`・`remote_account_snapshots`・プロフィールの版・`account_keys`） |
| `Create` / `Update` の投稿 | `content.IngestRemoteStatus` / `content.ReviseRemoteStatus` |
| `Delete` の投稿 / Actor | `content.DeleteRemoteStatus` / `account.MarkRemoteDeleted` |
| `Announce` / `Undo Announce` | `content.RecordRemoteBoost` / `content.RetractRemoteBoost` |
| `Like` / `Undo Like` | `content.RecordRemoteFavourite` / `content.RemoveRemoteFavourite` |
| `Follow` / `Accept` / `Reject` / `Undo Follow` | `graph.ReceiveFollow` / `graph.AcceptRemote` / `graph.RejectRemote` / `graph.TerminateRemote` |
| `Block` / `Undo Block` | `graph.ReceiveBlock` / `graph.RetractRemoteBlock` |
| `Flag` | `moderation.FileRemoteReport` |

### 1.2 自サーバーの URI

- 受信した文書の中の自サーバーの URI（ホストが設定 `host` のもの）は、ADR-0018 の正規表現でパースして DB を引き、HTTP で取得しません。パースできない自サーバーの URI は存在しないものとして扱います。
- リモートの URI は、`https` で、ホストを正規化した IRI として正規化してから比較・保存します。フラグメントは `#main-key` などの鍵を除いて保存しません。

#### ホスト名の正規化（IDN）

- 外から入るホストはすべて、比較・保存の前に IDNA 2008 の A-label（小文字の punycode）にそろえます。U-label（`例え.jp`）と A-label（`xn--r8jz45g.jp`）、全角や大文字で書かれた同じホストは同じ値になり、オリジンの照合（`id` と `attributedTo`、`keyId` と `actor`、取得結果の `id`）、ドメイン単位の処分、`InstanceHealth`、shared inbox の集約で同じホストとして扱われます。
- 写像は `golang.org/x/net/idna` で行うので、`internal/app/federation` に置きます（ドメイン層は標準ライブラリだけを使うため）。
  - 受信・取得した文書は `federation.Normalize`（アプリケーション層）で読み、文書の中のすべての IRI のホストを写像します。
  - 文書の外から来る IRI（取得する URL・`keyId`・利用者が入力した URL）は `federation.CanonicalIRI`、WebFinger の `acct:` のドメインと、運営者が入力するドメイン単位の処分のドメインは `federation.CanonicalHost` を通します。
  - ドメイン単位の処分の照合は、正規化したホストとその親ドメイン（`federation.DomainAndParents`）で引きます。
- 写像は WHATWG URL のホストの解析と同じ UTS #46 の非推移的処理です（`ß` は `ss` にしない）。Bidi 規則と ContextJ を確かめ、ラベルと名前の長さを検証します。ラベルの 3・4 文字目の `--`（`r3---sn-x.example` など）と `_` は、実在するホストがあるので許します。
- 写像できないホスト（解読できない `xn--` ラベル、許されない文字、空のラベル、長すぎるラベル）は不正です。文書の中の不正な IRI は無いものとして読み、文書の外から来たものはエラーにします。
- IP アドレスはそのままの表記（小文字）で扱います。末尾の `.` は取り除きます。
- **表示**: ホストを人に見せるときは `federation.DisplayHost` で U-label に戻します。ただし、なりすましを防ぐため、どれかのラベルが UTS #39 §5.2 の「highly restrictive」に当てはまらない（文字が 1 つの用字だけでも、ラテン文字＋漢字＋ひらがな・カタカナ、ラテン文字＋漢字＋注音字母、ラテン文字＋漢字＋ハングルのどれかの組み合わせでもない）ときは、ホスト全体を A-label のまま表示します。キリル文字の `а` とラテン文字の混じった `xn--pple-43d.com` は A-label で表示されます。保存と比較には常に A-label を使います。

## 2. 署名

ADR-0032 のプロファイルに従います。要点:

- **受信**: `Signature-Input` があれば RFC 9421、なければ draft-cavage-12 で検証します。POST はダイジェスト（`Digest` か `Content-Digest`）が必須で、受け取った本文から計算した値と照合します。
- **送信**: draft-cavage-12（`rsa-sha256`）で送り、400 / 401 なら RFC 9421 で 1 回だけ再送し、成功したホストを KV に 7 日記憶します。
- **鍵**: リモートの公開鍵は `account_keys` に持ち、取得から 24 時間で古いとみなします。検証に失敗したら、`keyId` ごとに 5 分に 1 回まで取り直して 1 回だけ再検証します（ADR-0019 §5, §7）。
- **署名者と主体**: 鍵の持ち主（`owner`）がアクティビティの `actor` と一致し、`keyId` のホストが `actor` のホストと一致することを確かめます。一致しなければ、オブジェクトを `id` からオリジンに取り直します（§3.3）。
- **インスタンス Actor**: オブジェクトの取得（GET）は、インスタンス Actor（`Application`）の鍵で署名します。相手が Authorized Fetch でも取得でき、特定の利用者の鍵を使わないので、相手のブロックに巻き込まれません。

### 2.1 実装

| 型（`internal/app/federation`） | 役割 |
| --- | --- |
| `Verifier` / `Signer`（ポート） | 署名の解析・署名ベースの組み立て・暗号の検証と署名だけを行う。実装は `internal/adapter/httpsig`、コントラクトテストは `testkit.HTTPSignatureContract`（RFC 9421 付録 B のテストベクタ、Mastodon・GoToSocial 形式の draft-cavage の署名、拒否の各条件、double-knocking） |
| `SignatureVerifier` | 受信のポリシー。`Parse`（署名なし `401`、解析不能 `400`）→ `Check`（必須の対象・時間窓・宣言されたアルゴリズム・ダイジェスト。I/O なし）→ `Verify`（鍵の解決と検証、鍵の取り直し）。拒否は `SignatureError`（`Status` と、`401` なら `Accept-Signature` の値） |
| `PublicKeyResolver`（ポート） | `keyId` から公開鍵と持ち主（`owner`）を返す。`refresh` で取り直す。取得できなければ `ErrPublicKeyNotFound`（`401`）、取得先が止まっていれば `ErrPublicKeyUnavailable`（`503`）。実装は `RemoteActors`（§6） |
| `SignedHTTPClient` | 送信の double-knocking。`kernel.HTTPClient` の上で署名し、結果の方式（`delivery_attempts.signature_scheme`）を返す |

- `@target-uri` と `(request-target)` は `NewSignedRequest` が設定の公開オリジンとリクエストのパス・クエリから組み立て、`Host` も公開オリジンのホストにします（別のホスト宛ての署名は検証に通らない）。
- RFC 9421 で `alg` が無い RSA の署名は、`rsa-v1_5-sha256`、`rsa-pss-sha512` の順に検証します（RFC 9421 付録 B の署名は `alg` を持たない）。
- 送信の RFC 9421 の `created` はシステムの時計から取ります（`dadrus/httpsig` が時計を差し替えられないため）。draft-cavage の `Date` は `kernel.Clock` から取ります。
- `code.superseriousbusiness.org/httpsig` は `algorithm="hs2019"` を固定で書くので、RSA の署名はアダプタが `rsa-sha256` に書き換えます（`algorithm` は署名の対象ではない）。
  このライブラリは、署名が `(created)` / `(expires)` を対象にするときだけ、システムの時計で 10 秒を超える未来の `created` と 10 秒を超えて過ぎた `expires` も拒否します（主要実装は `Date` に署名するので影響しない）。
- `Accept-Signature` を付けた `401` には、RFC 9421 で再送します。要求された対象をプロファイルの対象に加え、ラベル・`tag`・`nonce` も要求に合わせます。鍵とアルゴリズムは自分のものを使います。
- KV の名前空間: `federation.signature_scheme`（ホストごとの記憶。値は `rfc9421`、7 日）、`federation.key_refresh`（`keyId` ごとの取り直しの間隔、5 分）。記憶のあるホストで draft-cavage への再送が成功したら記憶を消します。
- ライブラリ（`dadrus/httpsig`、`code.superseriousbusiness.org/httpsig`、`dunglas/httpsfv`）は `internal/adapter/httpsig` の外から使えません（`.golangci.yml` の depguard）。

## 3. 受信（Inbox）

ADR-0033 の推奨案に従い、**署名の検証までを同期**、**処理を非同期** にします（NFR-PERF-4）。

### 3.1 同期の部分

`POST /ap/inbox`（shared inbox）と `POST /ap/actors/{uuid}/inbox`（有効なローカルの Actor でなければ 404）は、次の順に処理します。どこかで失敗したら、DB とキューに触れずに応答します。

| 順 | 処理 | 失敗時 |
| --- | --- | --- |
| 1 | `Content-Type` が `application/activity+json`、`application/ld+json`（AS2 のプロファイル付き）、`application/json` のどれか | `415` |
| 2 | 本文を 1 MiB まで読む（`http.MaxBytesReader`、ADR-0019 §5） | `413` |
| 3 | IP 単位のレート制限（ADR-0019 §5。1,000 / 5 分、IPv6 は /64 単位） | `429` + `Retry-After` |
| 4 | 署名ヘッダを解析し、`keyId` のホストを得る | `401`（署名なし）/ `400`（解析不能） |
| 5 | `keyId` のホストのドメイン単位の処分（`federation` の制限、§11） | `403` |
| 6 | ドメイン単位のレート制限（`keyId` のホスト。2,000 / 5 分） | `429` + `Retry-After` |
| 7 | ダイジェストの照合と、鍵の要らない検査（必須の対象・時間窓・宣言されたアルゴリズム。`SignatureVerifier.Check`） | `401` + `Accept-Signature` |
| 8 | 本文の JSON から `id`・`type`・`actor`・`object`（の `id`）だけを読む（`federation.ReadEnvelope`）。`type` と `https` の `actor` が必須で、IRI は 2,048 バイト、`type` は 255 バイトまで | `400` |
| 9 | 未知の Actor の自分自身の `Delete` / `Update`（`actor == object`、`accounts (uri)` に無い）なら、鍵を取りに行かずに捨てる（ADR-0019 §7） | `202` |
| 10 | 鍵の解決（未知・期限切れなら Actor を取得。5 秒で打ち切る） | `401`（取得できない）/ `503`（取得先がサーキットブレーカーで止まっている） |
| 11 | 署名の検証（ADR-0032。保存した鍵で検証できなければ鍵を取り直して 1 回だけ再検証する） | `401` + `Accept-Signature` |
| 12 | 署名者と `actor` の照合。一致しなければジョブに `trust = refetch` を付ける | — |
| 13 | 1 つの短いトランザクションで、`received_activities` に `ON CONFLICT DO NOTHING` で挿入し、行が増えたときだけ `received_activity_documents` と `federation.inbox` ジョブ（`queue.Outbox`）を書く | — |
| 14 | 署名者のドメインの `instance_health` を `available` に戻す（§7.3） | — |
| 15 | `202 Accepted` | — |

- `id` の無いアクティビティは、`actor`・改行・受け取った本文をつないだ SHA-256 の 16 進表記から `urn:my:digest:<hex>` を合成して重複排除のキーにします（[data-model §2.2](data-model.md#22-冪等キーと一意制約)）。
- `id` のホストが `actor` のホストと異なるアクティビティと、鍵の持ち主（`owner`）が `keyId` のホストに無いアクティビティは、`trust = refetch` にします。
- 鍵の取得はトランザクションの外で行います（ADR-0003）。取得した Actor の書き込みは、受信の記録とは別の短いトランザクションです。
- 手順 1・2 と、個人の inbox の宛先の確認（404）は `internal/interface/activitypub` が、手順 3〜14 は `federation.Inbox.Receive` が行います。拒否は `InboxRejection`（`Status`、`429` の `Retry-After`、`401` の `Accept-Signature`）で返します。レート制限のストアが使えないときは、制限を掛けずに通します。
- 受信の記録は `InboxRepository`（`received_activities` と `received_activity_documents`）に書きます。コントラクトテストは `testkit.InboxContract` です。
- 個人の inbox と shared inbox は同じ記録をします。誰宛てかはアクティビティの宛先が決めます。

### 3.2 非同期の部分

ジョブ `federation.inbox`（ペイロードは `activity_uri` と `trust`、冪等キーは `inbox:<activity_uri のハッシュ>`、`orderingKey` は `inbox:<actor の URI>`）が、原本を読んで処理します。

1. 原本を正規化する（§4）。正規化できなければ `Fail`（dead job に残る）。
2. `trust = refetch` なら、オブジェクトを `id` からオリジンに取得し直し（§6）、取得した文書を使う。`Announce` 以外で取得できなければ捨てる。
3. 主体（`actor`）のアカウントを解決する（未知なら取得、§6）。主体やそのドメインに `federation` の制限があれば捨てる（`Delete` / `Undo` / `Reject` は通す。§11）。
4. 種類ごとのハンドラ（§3.4）を呼ぶ。ハンドラは 1 つの短いトランザクションで、各モジュールの公開 API を呼んで書く。
5. 結果: 書けたら `Done`、取得の一時的な失敗は `Retry`（ADR-0004 のバックオフ）、恒久的に処理できないものは `Fail`。

- 受信の記録は同期の部分で書くので、送信側が同じアクティビティを再送してもジョブは増えません。処理に失敗したアクティビティは dead job として残り、CLI（FR-801）から再実行します。
- `orderingKey` は同じ Actor のアクティビティを直列化しますが（#77）、正しさはそれに頼りません（§3.3）。
- 実装は `federation.InboxProcessor`（`internal/app/federation`）です。
  - 手順 2 の取り直しでは、埋め込みの文書をすべて信用しないものにします。`Create` / `Update` はオブジェクトを `id` から取得し、`attributedTo` が主体のものに差し替えます。`Delete` は対象がオリジンで消えている（404 / 410 / `Tombstone`）ときだけ通します。自分自身の `Update` はハンドラが Actor を取得し直します。それ以外はアクティビティを `id` から取得し、同じ `id`・型・主体のときにその文書を使います。
  - 主体はポート `InboxActors`（`RemoteActors.Resolve`）で解決し、ローカルの主体は捨てます。制限はポート `DomainPolicy`（主体のホスト）と `AccountPolicy`（主体のアカウント）で判定します。
  - ハンドラは `Handle(型, ハンドラ)` で登録します。主体が自分自身を対象にする `Update` / `Delete` は `HandleOwnActor` で別に登録します（`RemoteActors.RegisterInboxHandlers` が登録する Actor の更新・削除）。登録の無い型は `info` のログを残して完了にします（未知の型・`Move`・まだハンドラの無い型）。
  - `Undo` は取り消す活動の型ごとに `HandleUndo(型, ハンドラ)` で登録します（`Handle(Undo, …)` は使わない）。`object` に活動が埋め込まれていればその型のハンドラだけに、URI だけなら登録順にハンドラを呼び、`ErrUnknownObject` 以外を返したところで止めます。どれも対象を知らなければ tombstone です。埋め込みの型は振り分けにだけ使い、ハンドラは対象が主体のものであることを DB で確かめます。
  - ハンドラは、対象が無いことを `ErrUnknownObject`、捨てることを `ErrDropActivity`、処理できないことを `ErrUnprocessable`（`Fail`）で返します。取得の `ErrFetchNotFound` / `ErrFetchGone` / `ErrFetchRefused` は捨てる、`ErrFetchFailed` / `ErrFetchUnavailable` とその他のエラーは `Retry` です。
  - 記録や原本が無い・正規化できない・記録と主体が違うジョブは `Fail` です。
  - 結果は `InboxOutcome`（`handled` / `unhandled` / `tombstoned` / `dropped`）です。コントラクトテストは `testkit.InboxProcessingContract` です。

### 3.3 順序と冪等性

ActivityPub の配送は順序を保証しません。ハンドラは、どの順で届いても、何回処理しても同じ結果になるように書きます（NFR-REL-1）。

| 状況 | 振る舞い |
| --- | --- |
| 同じアクティビティの再処理 | 対象テーブルの一意制約（`statuses (uri)`、`boosts (uri)`、`follows (uri)`、`favourites (uri)`、`blocks (uri)`、`reports (uri)`、版の `(status_id, seq)`）で no-op |
| `Create` より先に `Delete` | `remote_object_tombstones` に URI を記録する。後から届いた `Create` / `Update` / 取得は、tombstone があれば捨てる |
| `Create` より先に `Update` | 取り込み範囲（ADR-0034）に入るなら `Create` として扱う。`published` が 1 日より古いものは取り込まない（Mastodon と同じ） |
| 古い `Update` が後から届く | `updated`（無ければ `published`）が現在版の `revised_at` 以下なら版を作らない |
| `Follow` より先に `Undo Follow` | `Undo` の対象の URI を tombstone に記録し、後から届いた `Follow` を捨てる |
| 対象が無い `Accept` / `Reject` / `Undo Like` / `Undo Announce` | 何もせず完了にする |
| `Like` / `Announce` の対象のローカルの投稿が無い・削除済み | 何もせず完了にする |
| 同じ投稿への同時の `Create` と `Update` | `statuses (uri)` と版の `(status_id, seq)` の一意制約で後の方が再実行され、既存の行を使う |

- tombstone は `origin = remote` の `status_deletions` と、未知の URI のための `remote_object_tombstones`（30 日で消す）の 2 つです。
- `remote_object_tombstones` は `InboxProcessor` が書きます。`Delete` / `Undo` のハンドラが無いか、ハンドラが `ErrUnknownObject` を返したとき、対象の URI が主体と同じオリジンで主体自身でなければ記録します（ほかのサーバーのオブジェクトを封じさせない。`federation.TombstoneFor`）。
- `Delete` / `Undo` 以外のアクティビティは、ハンドラを呼ぶ前に、アクティビティの URI に tombstone があれば捨てます。`Create` / `Update`（自分自身の Actor 以外）は、オブジェクトの URI の tombstone でも捨てます。
- アクティビティの `object` が URI だけのとき、受信した文書の中に埋め込まれていない対象は、ローカルの DB から引くか、取り込み範囲に入るときだけ取得します。
  `Create` / `Update` の投稿が URI だけなら、作者をローカルのアカウントがフォローしているときだけ取得します（投稿が無いと、メンションや返信先で範囲を判定できない）。

### 3.4 アクティビティごとのハンドラ

| アクティビティ | 対象 | 処理 |
| --- | --- | --- |
| `Follow` | ローカルの Actor | `graph.ReceiveFollow`。鍵アカウントでなければ承認し、`Accept` を配送する。鍵アカウントなら承認待ち + 通知。ブロック関係があれば `Reject` を配送する |
| `Accept` | 自分が送った `Follow`（URI か埋め込み） | `graph.AcceptRemote`（`follow_acceptances`、`accepted_by = remote`）。`object` の `id` が主体をフォローされる側とするフォローに無ければ、埋め込みの `object.actor`（ローカル）と `actor`（相手）の承認待ち・承認済みのフォローで照合する。埋め込みが `Follow` 以外（GoToSocial の返信の承認など）なら捨てる |
| `Reject` | 自分が送った `Follow`、または承認済みのフォロー | `graph.RejectRemote`。照合は `Accept` と同じ。承認待ちなら `follow_rejections`（`rejected_by = remote`）、承認済みなら `follow_terminations`（`removed_by_followee`） |
| `Undo` | `Follow` / `Like` / `Announce` / `Block` / `Accept` | 対象の取り消し。`Undo Follow` は `graph.TerminateRemote`（`unfollowed`）で、`object` の `id` が主体のフォローに無ければ、埋め込みの `object.object`（ローカル）とのフォローで照合する（Misskey の `Undo` は送った `Follow` と違う `id` を持つ）。`Undo Accept` は `graph.RejectRemote` でフォローの終了として扱う。対象が URI だけなら、主体が作った同じ URI の `follows` / `favourites` / `boosts` / `blocks` を順に引く |
| `Create` | `Note` / `Question`。`Article` / `Page` / `Event` / `Image` / `Video` / `Audio` は変換して取り込む | 取り込み範囲（ADR-0034）に入れば `content.IngestRemoteStatus`。オブジェクトの `id` は `actor` と同じオリジンで、`attributedTo` は（あれば）`actor` でなければならない。保存済みの投稿は何もせず、削除済みの投稿は捨てる |
| `Update` | 投稿 | `content.ReviseRemoteStatus`（新しい版）。投稿の作者とアクティビティの主体が一致し、オリジンが同じであること（[ActivityPub §7.3](https://www.w3.org/TR/activitypub/#update-activity-inbox)）。未知の投稿は §3.3 のとおり `Create` として扱う |
| `Update` | Actor（自分自身） | `RemoteActors.ReceiveActorUpdate`。オリジンが同じ埋め込みの文書で `account.UpsertRemoteActor`（内容が変わればプロフィールの版が増える）。埋め込みが無い・`id` が主体と一致しなければ取得し直す。削除済みの Actor の `Update` は捨てる |
| `Delete` | 投稿・`Tombstone` | `content.DeleteRemoteStatus`（`origin = remote`）。未知なら tombstone |
| `Delete` | Actor（自分自身） | `RemoteActors.ReceiveActorDelete`。`account.MarkRemoteDeleted`（`remote_deleted`）で、プロフィールと投稿（タイムラインでは `feed_author_restrictions` の `profile_view`）がすぐに表示されなくなる。未知の Actor なら何もしない。投稿・フォロー・ブーストは消去ジョブが消す |
| `Announce` | 投稿（URI か埋め込み） | `content.RecordRemoteBoost`。ローカルの投稿と保存済みの投稿は `id` で引く。保存していないリモートの投稿は、ブーストした人をローカルのアカウントがフォローしているときだけ取り込み（ADR-0034）、埋め込みの投稿は、オリジンが主体と同じでなければ `id` から取得し直す。ブーストの公開範囲は、`Announce` の宛先から決めた公開範囲と元投稿の公開範囲の狭いほう（`direct` になるものは捨てる）。`local` の投稿、ほかの人の `private` の投稿、主体が閲覧できないローカルの投稿は捨てる |
| `Like` / `EmojiReact` | ローカルの投稿 | `content.RecordRemoteFavourite`。Misskey の絵文字リアクション（`_misskey_reaction` 付きの `Like`）と Pleroma / Akkoma の `EmojiReact` もお気に入りとして扱う（絵文字は保存しない）。主体が閲覧できない投稿（`ContextObjectFetch`）は捨てる |
| `Block` | ローカルの Actor | `graph.ReceiveBlock`。両方向のフォローを終了し、相手の投稿を閲覧者から隠す（`feed_exclusions`） |
| `Flag` | ローカルの Actor / 投稿 | `moderation.FileRemoteReport`（`reports.uri`）。送信者のドメインに `report_intake` の制限があれば捨てる |
| `Add` / `Remove` | 主体の `featured` | ピン留めの一覧を取得し直す（C） |
| `Move` ほか未知の型 | — | ログとメトリクスに残して完了にする（`Move` は v0 の非目標） |

- ブーストとお気に入りのハンドラは `federation.RegisterInteractionHandlers` です。`Announce` の対象の取り込みは投稿のハンドラ（`StatusHandlers`）と同じ規則で行います。コントラクトテストは `testkit.RemoteInteractionContract` で、主要実装のフィクスチャの `Announce` / `Like` / `EmojiReact` / `Undo` を受け取り、`Like` / `Undo Like` を送ります。

## 4. AS2 の正規化

受信した文書は JSON-LD として処理せず、AS2 の正規の `@context` で圧縮された JSON として読みます。AS2 は消費者に JSON-LD の処理を求めていません（[AS2 Core §2](https://www.w3.org/TR/activitystreams-core/#jsonld)）。
`internal/domain/federation` の正規化関数が、主要実装の方言の差をここで吸収し、以降のコードは正規化した型だけを扱います。

| 項目 | 規則 |
| --- | --- |
| `@context` | 値は解釈しない。トップレベルの文書には `https://www.w3.org/ns/activitystreams` を含むことを求める（無ければ正規の文脈を仮定する。AS2 Core §2.1）。`signature`（LD 署名）は検証せず、読み捨てる |
| 単数と配列 | 配列になり得るプロパティは、単数・配列・`null` のどれでも配列として読む（`type` も配列を許し、知っている型の最初のものを使う） |
| リンクとオブジェクト | `object`・`inReplyTo`・`attributedTo`・`actor` などは、文字列（IRI）・`Link`（`href`）・埋め込みのオブジェクト（`id`）のどれでも IRI を取り出す。埋め込みの中身を使うのはオリジンが同じときだけ |
| 公開の宛先 | `https://www.w3.org/ns/activitystreams#Public`、`as:Public`、`Public` を同じ値として扱う（[ActivityPub §5.6](https://www.w3.org/TR/activitypub/#public-addressing)） |
| 公開範囲 | `to` に公開 → `public`、`cc` に公開 → `unlisted`、作者の `followers` が `to` / `cc` に → `private`、それ以外 → `direct`。リモートの投稿が `local` になることはない |
| 本文 | `contentMap` の言語ごとに 1 行（`status_revision_bodies`）。`content` だけなら `contentMap` の最初の言語、なければ `und` の 1 行。言語タグは BCP 47 として正規化し、不正なら `und`（NFR-I18N-5） |
| CW | `summary`（無ければ `summaryMap` の最初の値）をプレーンテキストにして `spoiler_text`。`sensitive` は `toot:sensitive` |
| 変換する型 | `Article` / `Page` / `Event` / `Image` / `Video` / `Audio` は、`name` を太字の段落、`summary` を段落にし、`url` へのリンクを付けた本文にする（Mastodon の `CONVERTED_TYPES` と同じ） |
| `tag` | `Mention`（`href` を Actor の IRI として解決）、`Hashtag`（`name` から `#` を除いて `tags`）、`Emoji`（`icon.url`、カスタム絵文字） |
| 宛先のメンション | `to` / `cc` に含まれるローカルの Actor は、`tag` に無くてもメンションとして記録する（`direct` / `private` の閲覧判定のため） |
| `attachment` | `Document` / `Image` / `Video` / `Audio`。`url`（文字列か `Link`）、`mediaType`、`name`（代替テキスト）、`blurhash`、`focalPoint`、`width` / `height`。`href` だけの `Link` は添付として扱わない。4 件を超える分は捨てる |
| `url` | 文字列・`Link`・配列のどれでもよい。`text/html` の最初のものを表示用 URL にする |
| 時刻 | `published` / `updated` は RFC 3339 として読み、未来の時刻は現在時刻に丸める。投稿の `id`（UUIDv7）は `published` から作る（#71） |
| 投票 | `Question` の `oneOf` / `anyOf`（各 `name` と `replies.totalItems`）、`endTime`、`votersCount`（FR-112、C） |
| 返信の一覧 | 投稿の `replies` はコレクションへの参照（IRI か埋め込み）として読む。`Collection` / `OrderedCollection` / `CollectionPage` / `OrderedCollectionPage` は、`items`（順序付きなら `orderedItems`）を 100 件まで、`first` と `next` をページへの参照として読む（Mastodon は最初のページを埋め込む） |
| 方言 | Misskey の `_misskey_content`（MFM の原文）・`_misskey_summary` は使わず、`content` / `summary` を使う。`_misskey_quote` / `quoteUrl` / `quoteUri` は引用（FR-114 は W）なので、本文に含まれるリンクとしてだけ扱う。`isCat` などの未知のプロパティは無視する |
| 未知のプロパティ | 無視する（[AS2 Core §5](https://www.w3.org/TR/activitystreams-core/#extensibility)）。原本は `received_activity_documents` に短期間残る |
| 上限 | 文書のネストの深さ 16、`tag` 100 件、`to` / `cc` 合わせて 1,000 件、本文 HTML 100 KiB。超えたら、その文書を正規化できないものとして扱う |

## 5. リモート HTML のサニタイズ

- リモートの投稿本文（`content` / `contentMap`）・プロフィールの `summary`・補足情報（`attachment` の `PropertyValue` の `value`）は、**取り込み時に** サーバー側のサニタイザ（`bluemonday`、ADR-0019 §2）を通し、サニタイズ済みの HTML を保存します。
  サニタイズ済みの HTML には、使ったポリシーの版（`html_policy_version`）を記録し、ポリシーを変えたら消去と同じく一定件数ずつ再サニタイズします。
- ポリシーは Mastodon の `MASTODON_STRICT` に合わせた許可リストです。
  - 要素: `p` `br` `span` `a` `del` `s` `pre` `blockquote` `code` `b` `strong` `u` `i` `em` `ul` `ol` `li` `ruby` `rt` `rp`
  - 属性: すべての要素の `lang`、`a[href rel class translate]`、`span[class translate]`、`p[class]`、`ol[start reversed]`、`li[value]`
  - `class` は microformats（`h-` `p-` `u-` `dt-` `e-` で始まるもの）と `mention` `hashtag` `ellipsis` `invisible` `quote-inline` だけを残す。`translate` は `no` だけを残す
  - `href` のスキームは `http` `https` `dat` `dweb` `ipfs` `ipns` `ssb` `gopher` `xmpp` `magnet` `gemini`。それ以外の `a` は中のテキストだけを残す
  - `a` には `rel="nofollow noopener noreferrer"` と `target="_blank"` を付ける
  - `h1`〜`h6` は `<p><strong>…</strong></p>` に、許可外のブロック要素は `p` に変える。`<math>` は TeX の注釈があればそのテキストにする
- ローカルの投稿の HTML は、サーバーがプレーンテキストから生成します（`internal/app/content` の `text.go`）。生成する要素と属性はこの許可リストの部分集合です。
- **Web クライアントの Trusted Types ポリシー `my-server-sanitized-html`（`@my/ui` の `PostContent`）に渡る HTML は、ローカルで生成したものか、このサニタイザを通したものだけです**（[web-client.md](web-client.md)）。
  クライアント API は、本文・プロフィール・補足情報の HTML を保存したサニタイズ済みの値からだけ返し、受信した文書の値をそのまま返す経路を持ちません。表示名・ユーザー名・投票の選択肢・代替テキストはプレーンテキストとして返します（タグを除く）。
- カスタム絵文字の `:shortcode:` は HTML に `img` として埋め込まず、クライアントが絵文字の一覧から DOM 要素を差し込みます（ADR-0019 §2）。

## 6. リモートの Actor / オブジェクトの取得

### 6.1 取得の規則

- すべて `HTTPClient` を通し、インスタンス Actor の鍵で署名した GET で取得します（§2）。`Accept: application/activity+json, application/ld+json; profile="https://www.w3.org/ns/activitystreams"`。
- 応答の `Content-Type` が AS2 でなければ採用しません。本文は 1 MiB まで（ADR-0019 §1）。
- **同じオリジン**: 最後のリダイレクト先のオリジンと、文書の `id` のオリジンが一致しなければ採用しません。要求した URL と `id` が異なるときは、`id` を 1 回だけ取得し直して一致を確かめます（[ActivityPub 付録 B.2](https://www.w3.org/TR/activitypub/#security-verification)、FEP-fe34）。
- 投稿は、`attributedTo` のオリジンが投稿の `id` と同じでなければ採用しません。
- 再帰的な取得（返信先・埋め込み・`Announce` の対象）は、1 回の処理につき深さと件数に上限を置きます（[ActivityPub 付録 B.5](https://www.w3.org/TR/activitypub/#security-recursive-objects)、ADR-0034）。
- 404 / 410 と取得の失敗は、KV に 1 時間記録して同じ URI の取得を繰り返しません（名前空間 `federation.fetch_failure`、キーは URI の SHA-256）。Actor の 410 は `remote_deleted` にし、ほかの失敗は `remote_account_heads.consecutive_fetch_failures` に数えます。
- ドメイン単位の `federation` の制限があるドメインと、自サーバーの URI からは取得しません。`instance_health` が `unavailable` のドメインからも取得しません（鍵の解決は `503`）。
- 実装は `internal/app/federation` の `Fetcher`（取得の規則）と `RemoteActors`（Actor の解決・保存・ジョブ、`PublicKeyResolver`）です。コントラクトテストは `testkit.RemoteActorContract` です。

### 6.2 Actor とハンドル

- Actor の文書から `account.UpsertRemoteActor` が `remote_account_snapshots`（内容が変わったときだけ）、プロフィールの版（サニタイズした内容が変わったときだけ）、公開鍵を書きます。アバターとヘッダーは `content.MediaService.RecordRemoteMedia` で記録します。
- 鍵は、`publicKey` の PEM（PKIX と PKCS #1）、`publicKey` が `id` だけで示す鍵の文書（取得する。2 件まで）、FEP-521a の `assertionMethod` の `Multikey`（Ed25519）から読みます。鍵の `id` は Actor と同じオリジンで、`owner` / `controller` は Actor でなければなりません。
- `keyId` を取得して `Key` の文書が返ったら、`owner`（または `controller`）の Actor を取得します（ADR-0032）。
- 初めて文書を保存したドメインは `remote_instances` に記録し、ジョブ `federation.observe_instance` が NodeInfo 2.x（`/.well-known/nodeinfo` が指す同じホストの文書）のソフトウェア名と版を `instance_nodeinfo_observations` に記録します（前回と同じなら記録しない）。
- ハンドル（`account_handles`）は WebFinger で確かめたものだけです。`acct:{preferredUsername}@{Actor のホスト}` を引き、`self` リンクが Actor の `id` を指すこと、`subject` のドメインが異なるならそのドメインでも同じことを確かめます（ADR-0018 のドメイン分離に相当する構成）。
  - 答え（404 を含む）が確かめなければ `mismatch` を記録し、ハンドルを持たないアカウントとして扱います。
  - WebFinger が答えなかった（接続の失敗・429・5xx）ときは何も記録せず、それまでのハンドルを残します。
- `@user@host` / URL での解決（FR-305）は、`RemoteActors.ResolveHandle` と `ResolveURL` が行います。
  - ハンドルは、保存済みの `account_handles` にあればそれを返します。なければ `https://{domain}` の WebFinger で `acct:{user}@{domain}` を引き、`self` リンクの Actor を `Resolve`（保存済みなら取得しない）で取得・保存します。WebFinger にも §6.1 の取得先の規則（自サーバー・`federation` の制限・`unavailable` のドメインには送らない）が掛かります。
  - URL は、Actor の `id` か Actor を返す URL（プロフィールのページなど）として `Resolve` します。
  - どちらも、ハンドルを WebFinger で確かめた Actor だけを返し、それ以外（WebFinger が Actor を示さない・取得できない・ハンドルが `mismatch`）は `ErrUnresolvable` です。
  - ハンドルの解決は、返す Actor のハンドルが求めたハンドル（ユーザー名は大文字小文字を区別しない、ホストは正規化）と一致することも確かめます。
  - ローカル投稿のメンション（data-model §4.4）は `ResolveHandle` を、トランザクションの外で、件数と時間に上限を置いて呼びます。
  - クライアント API は `GET /api/v1/accounts/lookup` の `resolve=true` で呼びます（トークン必須、アカウントごとに 30 回 / 5 分）。
- 検索の解決は `federation.Resolver.Resolve` です。ハンドル（`@user@host` / `user@host`）は `ResolveHandle` に渡します。
  URL は、正規化した IRI で保存済みのアカウント・投稿を引き、無ければ文書を 1 回だけ取得して、Actor なら保存してから `ResolveURL` と同じくハンドルを確かめ、投稿なら §6.3 の取得した投稿として取り込みます（ADR-0034 の「URL の検索で解決した」）。投稿の返信先は受信時と同じく 1 段だけ取得します。
  自サーバーの URL は取得せず、保存済みのものだけを返します。それ以外（コレクションなど）と取得できないものは `ErrUnresolvable` です。
- 取得から 24 時間を過ぎた Actor は、次に使われたとき（署名の検証・表示・配送）に非同期で取り直します（ジョブ `federation.refresh_actor`、冪等キー `refresh_actor:<account_id>`）。ローカルのアカウントがフォローしている Actor は、定期ジョブ `federation.refresh_actors`（1 時間ごと）が `remote_account_heads (last_fetched_at)` の古い順に 200 件ずつ読み、フォローされているものを取り直します（ADR-0034）。カーソル（`last_fetched_at` と `account_id`）を持った次のジョブが続きを読みます。

### 6.3 投稿

- 取り込み範囲は ADR-0034 に従います（ローカルのアカウントとの関係がある投稿だけ）。判定は `federation.StatusHandlers`（`internal/app/federation`）が、受信ごとに次の順で行います。
  1. 保存済みのアカウントのうち、投稿がメンションする・宛先に含めるローカルのアカウント（`active` なもの）がある
  2. 作者に承認済みのフォロワーがいる（リモートの Actor のフォロワーはローカルのアカウントだけ。`graph` の `ActiveFollowerIDs`）
  3. 返信先がローカルの投稿か、返信先の `conversation_uri` に削除されていないローカルの投稿がある
  - どれにも当てはまらない `Create` は `ErrDropActivity` で捨て、受信の記録だけが残ります（`info` のログ）。未知のリモートの Actor へのメンションは、範囲に入ると決まってから取得します。
- 書き込む行とメンション・ハッシュタグ・カスタム絵文字・添付・投票の扱いは [data-model §4.4 のリモート投稿のコマンド](data-model.md#リモート投稿のコマンド) にあります。コントラクトテストは `testkit.RemoteStatusContract` で、主要実装のフィクスチャの `Note` を取り込みます。
- 返信先は受信時に 1 段だけ非同期（`federation.fetch_object`、ペイロードに残りの深さ）で取得し、スレッドの祖先と返信の一覧は利用者がスレッドを開いたときに深さ 20・件数 100 まで取得します。
  - **取得した投稿**: 取得した文書が投稿で、`attributedTo` があり、そのオリジンが投稿の `id` と同じで、作者を解決でき（未知なら取得）、作者が有効なリモートのアカウントで、URI に削除（`status_deletions` の tombstone）も `remote_object_tombstones` も無いときだけ、`content.IngestRemoteStatus`（`activity_uri` は NULL）で取り込みます。取り込み範囲の判定はしません（受信した投稿の返信先・利用者が開いたスレッド・検索で解決した投稿・フォローされている人がブーストした投稿は ADR-0034 の範囲です）。`Announce` の対象・`fetch_object`・`Resolver` は、この取り込み（`StatusHandlers.fetched`）を共有します。`Announce` の対象と検索で解決した投稿は、受信した投稿と同じく返信先を 1 段だけ取得します。
  - **受信時**: 取り込んだ返信の親が未知（`conversation_uri` が NULL）でリモートなら、取り込みのトランザクションで `federation.fetch_object`（祖先 0）を `queue.Outbox` に投入します。取得した親の親は取得しません。
  - **スレッドを開いたとき**: `GET /api/v1/statuses/{id}/context` が、閲覧できた投稿について `StatusHandlers.FetchThread` を呼び、応答は待たずに保存済みのものだけを返します（取得したものは次に開いたときに出る）。返信先の無いローカルの投稿は何もしません（返信は配送で届く）。
    同じ投稿は 1 時間に 1 回だけです（KV の名前空間 `federation.thread_fetch`、キーは URI の SHA-256、`IncrBy` で最初の 1 回だけ投入する）。
  - `federation.fetch_object` のペイロードは `uri`、スレッドの起点の `thread`（受信時は空）、残りの祖先の深さ `ancestors`、残りの返信の深さ `replies`、祖先の取得の印 `up` で、冪等キーは `fetch_object:<ペイロードの SHA-256>` です。1 つのジョブは 1 つの投稿を扱います。
    1. 未保存なら取得して取り込む。保存済みのリモートの投稿で `replies` が残っていれば、返信の一覧を読むために文書を取得する。
    2. `ancestors` が残っていれば、保存済みの祖先をたどり、最初の未知のリモートの祖先の取得を、残りの深さを 1 減らして投入する。
    3. `replies` が残っていれば、`replies` のコレクション（投稿と同じオリジンのものだけ）とそのページを、埋め込みならそのまま、そうでなければ取得して読み、リモートの返信ごとに、深さを 1 減らした取得を投入する（最後の段で保存済みの返信は投入しない）。
  - **件数の上限**: スレッドの取得が取得する文書（投稿・コレクション・ページ）は、起点ごとの KV のカウンタ（名前空間 `federation.thread_budget`、1 時間）で数えて合わせて 100 件までです。祖先は深さ 20 までなので 20 件を超えず、返信の側（起点の文書・コレクション・ページ・返信）は 100 − 20 = 80 件で止めて祖先の分を残します。上限を超えた取得はカウンタを戻して何もしません。
  - 取得できない・取り込まない文書は `info` のログを残してジョブを完了にします。取得の失敗は `Fetcher` が 1 時間記憶するので（§6.1）、同じ URI を別のジョブが取り直しません。ストアの失敗は `Retry` です。
- 投稿の ID は `published` から作った UUIDv7 です（[data-model §1.3](data-model.md#13-id-とキーセットページング)、#71）。

## 7. 送信（配送）

### 7.1 記録

ローカルの操作のトランザクションで、`federation` が他のモジュールのドメインイベントを購読し（`uow.Projectors`）、`outbound_activities` と `federation.fanout` ジョブ（`queue.Outbox`）を同じトランザクションで書きます。

| ドメインイベント | アクティビティ | 宛先 |
| --- | --- | --- |
| `content.status_created` | `Create` | §7.2 の公開範囲ごとの宛先 |
| `content.status_revised` | `Update` | 元の `Create` の `deliveries` の inbox（`public` / `unlisted` は加えてその時点のフォロワー。[data-model §7](data-model.md#7-公開範囲と配送)） |
| `content.status_deleted` | `Delete`（`Tombstone`） | 元の `Create` の `deliveries` の inbox（`public` / `unlisted` は加えてその時点のフォロワー） |
| `content.boosted` / `content.boost_retracted`（`Federates` のときだけ） | `Announce` / `Undo Announce` | ブーストした人のリモートフォロワー + 元投稿の作者（リモートなら） |
| `content.favourited` / `content.unfavourited`（リモートの投稿だけ） | `Like` / `Undo Like`（本文は projector がイベントから組み立てて保存する。Mastodon と同じく宛先を持たない） | 投稿の作者の inbox |
| `graph.follow_requested` / `graph.followed`（相手がリモート） | `Follow` | 相手の inbox |
| `graph.follow_terminated`（`unfollowed` / `blocked` / `moderation`、自分がフォローした相手がリモート） | `Undo Follow` | 相手の inbox |
| `graph.followed` / `graph.follow_accepted` / `graph.follow_rejected`（フォローした人がリモート。相手の `Accept` / `Reject` によるものを除く） | `Accept` / `Reject` | フォローした人の inbox |
| `graph.follow_terminated`（`removed_by_followee`、フォローした人がリモート） | `Reject` | フォローした人の inbox |
| `graph.blocked` / `graph.unblocked`（相手がリモート） | `Block` / `Undo Block` | 相手の inbox |
| `account.profile_revised`（ローカルのプロフィールの 2 版目以降。最初の版はアカウントの作成で、まだだれもフォローしていない） | `Update`（Actor の文書） | リモートフォロワーの inbox |
| `account.lifecycle_event_recorded`（ローカルのアカウントの `deleted`） | `Delete`（Actor の `id`） | リモートフォロワーの inbox + 既知の shared inbox（[data-model §8](data-model.md#8-削除と消去)） |

- フォローの活動（`object_type = follow`）の主体は、`Follow` と `Undo` ではフォローした人、`Accept` と `Reject` ではフォローされた人です。`Follow` の `id` はフォローの `uri`（`/ap/activities/{follow_id}`）で、`Accept` / `Reject` / `Undo` はフォローの `Follow`（受け取ったフォローなら相手が送った `id`）を埋め込みます。宛先は相手だけで、`to` に相手を入れます。
  相手が送った `Accept` / `Reject` / `Undo` と、どちらかのアカウントの削除（`account_deleted`）による変化は、何も送り返しません。
- 記録するのはローカルの投稿・ブーストの活動だけです。リモートの投稿のイベントと、`local` 投稿のブースト（`Boost.Federates` が偽）は記録しません（ADR-0017）。
- projector は表の 1 行につき 1 つです（`federation.RegisterOutboundProjectors`）。送信する活動を増やすときは、その活動を送るユースケースと一緒に projector を足します。
- `inline` のキュー（テストと開発用）はトランザクションでの投入ができないので、`queue.AfterCommit` がコミットの後に投入します。
- アクティビティの文書は `federation.fanout` が 1 回だけ組み立て、`outbound_activity_documents` に保存します。すべての配送とリトライが同じ本文を送ります（再試行で内容が変わらない）。
- 投稿の `Delete` の文書は、projector が削除のトランザクションでイベントだけから組み立てて保存します。消去ジョブが投稿の行を消した後でも、`Delete` は保存した文書で元の `Create` の `deliveries` の inbox に届きます。
  宛先は投稿自身の公開範囲で作者の followers に向け（§7.4）、メンション先は入れません（`Delete` は対象が消えたことしか伝えない）。
- 投稿の文書は、イベントが指す版から組み立てます（後の編集で内容が変わらない）。

### 7.2 展開（fan-out）

`federation.fanout`（ペイロードは `outbound_activity_id`、`Create` / `Update` が示す版の `revision_id`、カーソル）が、宛先を一定件数（500 件）ずつ算出します。
カーソルは宛先の出どころ（フェーズ）とその中の位置で、1 つのジョブは 1 つのフェーズの 1 回分だけを処理します。

| アクティビティ | フェーズ（順に） |
| --- | --- |
| `Create` | 版のメンション先 → 作者の承認済みのフォロワー |
| `Update` / `Delete` | 元の `Create` の `deliveries` → （`public` / `unlisted` だけ）その時点のフォロワー |
| `Announce` / `Undo Announce` | ブーストした人の承認済みのフォロワー → 元投稿の作者 |
| `Like` / `Undo Like` | 投稿の作者 |
| `Update`（Actor） | Actor の承認済みのフォロワー |
| `Delete`（Actor） | Actor の承認済みのフォロワー → 既知の shared inbox（`account.KnownSharedInboxes`） |
| `Follow` / `Accept` / `Reject` / `Undo Follow` | フォローの相手（その Actor の inbox。無ければ shared inbox）。公開範囲では判定しない |

1. 宛先のアカウントを集める: 作者の承認済みのフォロワー（`graph` の公開 API をカーソルで）、版のメンション先、元投稿の作者。ローカルのアカウントは inbox を持たないので落ちます。
2. 公開範囲の規則で絞る: `content.CanView`（`ContextDelivery`、[visibility.md](visibility.md)）を宛先のアカウントごとに判定する。
   - フォロワーは `Follow = active` として判定します（ブロックは両方向のフォローを同じトランザクションで終わらせるので、承認済みのフォロワーとの間にブロックはありません）。メンション先は `graph` の公開 API から関係（フォロー・ブロック）を読みます。
   - ブーストは「ブーストした人の、ブーストの公開範囲の投稿」として判定し、元投稿の作者をメンション先として扱います。
   - `Delete` は対象が消えたことしか伝えないので判定しません。Actor の `Update` も、プロフィールは公開なので判定しません。
   - `local` は承認済みのフォロワーだけで、**フォロワーでないリモートのメンション先には送りません**（ADR-0017）。
   - `local` 投稿の文書の `tag` には、フォロワーでないリモートのアカウントへの `Mention` を入れません（ADR-0035）。本文のリンクは残します。
3. inbox を決める: アカウントの `shared_inbox_url`（無ければ `inbox_url`）。同じ inbox は 1 つにまとめます（shared inbox への集約、NFR-COST-5、[ActivityPub §7.1.3](https://www.w3.org/TR/activitypub/#shared-inbox-delivery)）。
4. ドメインの状態で絞る: `federation` の制限（ドメイン単位の `suspend`、§11）があるドメインと、`instance_health` が `unavailable`（§7.3）のドメインには送りません（`Follow` と `Accept` / `Reject` は `unavailable` でも送る）。
5. `deliveries` に `ON CONFLICT DO NOTHING` で挿入し、新しい行ごとに `federation.deliver` ジョブ（冪等キー `deliver:<delivery_id>`）を投入します。
6. 残りがあれば、カーソルを進めた `federation.fanout` を投入します。

### 7.3 配送とリトライ

`federation.deliver`（ペイロードは `delivery_id`）:

1. 宛先のドメインに送らない活動なら（`unavailable`、またはドメイン単位の `suspend`。§7.2 の手順 4 と同じ `federation.Deliverable`）、試行せず `skipped` を記録して完了にする。
2. inbox のホストのサーキットブレーカーが開いていたら、送らずに `retryable_failure`（`signature_scheme` は NULL）を記録して `Retry` を返す。
3. `outbound_activity_documents` の本文に、Actor の RSA 鍵（`{actor}#main-key`）で署名して POST する（§2 の double-knocking）。
4. 結果を分類して `delivery_attempts` に記録し、同じトランザクションでイベント `federation.delivery_attempted` を発行する。同期 projector が `instance_health` を更新する。

| 応答 | 分類 | ジョブ |
| --- | --- | --- |
| 2xx | `succeeded` | `Done` |
| 408 / 429 / 5xx（501 を除く）/ 接続・TLS・タイムアウトのエラー | `retryable_failure` | `Retry`（`Retry-After` があればそれ以降） |
| 400 / 401（double-knocking の後）/ 403 / 404 / 410 / 413 / 422 などの 4xx、501 / `HTTPClient` が拒否した宛先（SSRF ガード・リダイレクトの上限） | `permanent_failure` | `Done`（相手が受け取らないと答えた、または送れない宛先なので、リトライしない） |

- `Retry-After` は秒数と HTTP の日付のどちらも読み、24 時間で打ち切ります。ジョブの期限（リース）や停止で応答の前に打ち切られた試行は記録せず、そのまま `Retry` にします。
- 配送や Actor が見つからない、Actor に署名の鍵がないジョブは `Fail` にします（dead job に残る）。

- リトライの間隔は ADR-0004 の配送の例（1 分、5 分、30 分、2 時間、12 時間、24 時間 × 3。最大 9 回、約 4 日）で、ジッタを掛けます（`RegisterWithPolicy`）。使い切ったジョブは dead job になり、管理 API（`GET /api/v1/admin/dead_jobs` と `POST /api/v1/admin/dead_jobs/{id}/retry`、権限 `manage_jobs`）から再実行できます（NFR-REL-2）。
- **短期のサーキットブレーカー**: プロセス内で inbox のホストごとに、`retryable_failure` が連続 10 回続いたら 60 秒間はリクエストせずに `Retry` を返します（Mastodon の `Stoplight` と同じ値）。
  60 秒の後に送った 1 回がまた失敗すると、すぐにまた開きます。成功と `permanent_failure`（相手が答えた）で閉じます。開いている間の `Retry` も試行の 1 回に数え、待ち時間は上のリトライの間隔に従います（60 秒ごとに試行を使い切らない）。
- **`InstanceHealth`（長期）**: `retryable_failure` を記録した日（UTC の日付）がドメインごとに異なる 7 日に達したら、`instance_availability_events`（`marked_unavailable`）を記録し、`instance_health.state` を `unavailable` にします（Mastodon の `DeliveryFailureTracker` と同じ基準）。
  配送の成功、またはそのドメインから署名付きのリクエストを受けたら `marked_available` を記録して `available` に戻します。`unavailable` のドメインには、定期ジョブ `federation.probe_instances` が 1 日 1 回 NodeInfo を取得し、成功したら戻します。
  運営者は管理 API（`POST /api/v1/admin/instances/{domain}/mark_available`、権限 `manage_domains`）でも戻せます。理由は `operator` で、失敗の日数も 0 に戻ります。
  `instance_health` と `instance_availability_events` は管理 API（`GET /api/v1/admin/instances`、`.../{domain}/availability_events`）で読めます。
  - 署名付きの受信と probe の成功は、`federation.Health.Reached`（理由 `inbound_request` / `probe_succeeded`）がイベント `federation.instance_reached` を発行し、配送と同じ projector が `instance_health` を更新します。失敗の記録が無いドメインでは、コミット済みの値を読むだけで書きません。
  - probe は `/.well-known/nodeinfo` と、そこから同じホストの NodeInfo 2.x の文書を署名なしで取得し、どちらも 2xx の JSON なら成功とします。1 つのジョブは 50 ドメインを順に処理し、残りがあれば続きのジョブを投入します。
- 1 件の配送は 1 つのジョブです。Cloud Tasks / SQS では 1 回の試行が 1 オペレーションとして課金されるので、shared inbox への集約と `unavailable` のドメインの除外で回数を抑えます（NFR-COST-5）。

### 7.4 文書の形

- `Create` の `id` は `{object}/activity`、それ以外は `/ap/activities/{uuid}`（ADR-0018）。
- 宛先: `public` は `to: [Public]`・`cc: [followers, メンション先]`、`unlisted` は `to: [followers]`・`cc: [Public, メンション先]`、`private` と `local` は `to: [followers]`（`private` は加えて `cc: [メンション先]`）、`direct` は `to: [メンション先]`。
- Actor の `Update` は `to: [Public]` で、`object` に `GET` と同じ Actor の文書（`federation.LocalActorDocument`、`@context` なし）を埋め込みます。文書は展開の始めの時点の Actor から組み立てます（後の版はそれぞれの `Update` を送る）。有効でなくなったアカウントの `Update` は送りません。
- Actor の `Delete` は `to: [Public]` で、`object` は Actor の `id` です。
- `contentMap` は版の言語ごとの本文、`content` は版の既定の言語の本文です（NFR-I18N-5）。言語が `und` の本文は `contentMap` に入れません。`sensitive`・`summary`（CW）・`tag`（`Mention` / `Hashtag` / `Emoji`）・`attachment`（メディアの公開 URL・`mediaType`・`name`・`blurhash`・`focalPoint`・`width` / `height`）を付けます。
  - `sensitive` は CW があれば真です（Mastodon と同じ）。`summary` と `contentMap` は空なら省き、`sensitive`・`content`・`tag`・`attachment` は常に付けます。
  - `attachment` は処理の終わったメディアだけで、種類を問わず `type` は `Document`（Mastodon と同じ。種類は `mediaType` で伝える）、URL・`mediaType`・寸法は `full` の変種のものです。焦点が中央なら `focalPoint` を省きます。
  - `Mention` は `href` が Actor の IRI、`name` が `@user@domain`（ローカルのアカウントは設定 `account_domain`）、`Hashtag` は `href` が `https://{host}/tags/{name}`（ADR-0018）、`name` が `#name` です。
- Note は `federation.notes`（`internal/app/federation`）の 1 か所で組み立て、配送（`Create` / `Update` の `object`）とオブジェクトの GET（§10）が同じものを使います。版のメディア・メンション先・ハッシュタグは `content.Reader.Parts` で引きます。
- `@context` は AS2・security・Mastodon 拡張（`toot:`）の既存の Actor の文脈と同じものです。

## 8. 公開範囲 `local` と連合

ADR-0017 を連合の各経路に当てはめた結果です。

| 経路 | 振る舞い |
| --- | --- |
| 配送 | 承認済みのリモートフォロワーの inbox（shared inbox に集約）だけ。フォロワーでないメンション先には送らない |
| 文書 | `to` は followers だけ。`as:Public` を入れない。フォロワーでないリモートのアカウントへの `Mention` を `tag` に入れない（ADR-0035） |
| オブジェクトの GET | 署名者のドメインに作者の承認済みのフォロワーがいるときだけ返す。それ以外は 404（ADR-0018） |
| ブースト | ローカルのブーストは連合しない。リモートからはフォロワー限定に見えるのでブーストされない |
| `Update` / `Delete` | 元の `Create` の `deliveries` の inbox だけ |
| outbox コレクション | 列挙しない（`public` / `unlisted` だけ、ADR-0018） |

## 9. リモートメディア

ADR-0005 の方針（既定はキャッシュしない）に従います。

- リモートの添付・アバター・ヘッダー・カスタム絵文字は、`media_files`（`origin = remote`）と `remote_media` に、受信した文書が宣言した値（URL・`mediaType`・寸法・`blurhash`・焦点・代替テキスト）を記録します。取り込み時にファイルは取得しません。
- 取り込みは `content.MediaService.RecordRemoteMedia` を受信のトランザクションの中で呼び、同じ所有者・同じ URL のファイルは最初に記録したものを返します。
- クライアント API は、リモートのメディアの URL を **メディアプロキシの URL** `https://{host}/media/proxy/{media_id}/{variant}?sig=…`（`content.MediaProxy.URL`）で返します。
  `sig` は `media_id` と `variant` の HMAC-SHA256（base64url）で、プロキシを任意の URL の中継に使わせません。
  鍵は設定 `kek` から HKDF-SHA256 で導くので、プロキシに専用の秘密はありません。`kek_previous` から導いた鍵も検証に使うので、KEK のローテーション中も前の URL が使えます。
  `variant` は元のファイルそのものの `full` だけです。
- プロキシ（`content.MediaProxy` と `internal/interface/media`）は、`HTTPClient` で元の URL を取得し、形式を中身の先頭のバイトで判定して（ADR-0019 §8。許可リストは JPEG / PNG / GIF / WebP / AVIF / MP4 / WebM / MP3 / Ogg で、SVG は拒否）、
  判定した `Content-Type`、`Cache-Control: public, max-age={remote_media_cache_ttl}`、`X-Content-Type-Options: nosniff`、`Content-Security-Policy: default-src 'none'; sandbox`、`Cross-Origin-Resource-Policy: cross-origin` を付けて中継します。
  リモートのヘッダ（`Content-Type` など）と利用者のリクエストのヘッダは使いません。CDN がキャッシュします（NFR-COST-4）。
- サイズの上限は、中身で判定した種類ごとに画像 16 MiB、動画・音声 100 MiB です（ADR-0019 §1）。`Content-Length` が上限を超えるものは本文を読まずに断ります。
  長さのない画像は上限まで読んでから返し、長さのない動画・音声は中継の途中で上限を超えたらレスポンスを中断します（CDN が途中までのファイルをキャッシュしないように）。
- 署名がない・合わない URL、記録のないメディア、`media_fetch` の制限、SSRF ガードが拒否した宛先、リモートの 4xx、許可リストにない形式、上限を超えるものは、どれも 404 です。リモートに届かないときと 5xx は 502 です。エラーのレスポンスは `Cache-Control: no-store` です。
- インスタンス設定 `remote_media_policy`（`none` / `avatars` / `all`、既定 `none`）と `remote_media_cache_ttl`（秒、既定 86400）で、TTL 付きのキャッシュを選べます（FR-409）。
  キャッシュは `remote_media_cache` と Blob（`remote/` のキー）に置き、`media.cleanup_remote` が期限と容量の上限で消します（FR-803）。
  キャッシュはまだ実装していない（#276）ので、プロキシはどの方針でも `none` と同じく毎回中継し、`remote_media_cache_ttl` は `max-age` にだけ効きます。
- ドメイン単位の `media_fetch` の制限（`reject_media`）は、メディアの所有者のドメインと URL のホストの両方に当て、どちらかが制限されていればプロキシは取得せず 404 を返します。
  判定はポート `content.MediaFetchPolicy` を通します。`moderation` の投影ができるまでは、どのドメインも制限しない `content.NoMediaRestrictions` を組み込みます。
  `media_sensitive` の制限（`force_sensitive`）は表示のときに添付をセンシティブにします。
- サムネイル・blurhash がない動画は、クライアントが「プレビューなし」として表示します。

## 10. コレクションとオブジェクトの GET

- `GET /ap/objects/{uuid}`（`/activity` を含む）は、署名から閲覧者（署名者のドメイン）を決め、`content.CanView`（`ContextObjectFetch`）を通します。権限が無いときと存在しないときは同じ 404 です（ADR-0018）。削除済みは、削除の前に閲覧できた相手にだけ `410 Gone` と `Tombstone` を返し、それ以外には 404 を返します（[ActivityPub §6.4](https://www.w3.org/TR/activitypub/#delete-activity-outbox)）。
- 判定は `federation.Objects`（`internal/app/federation`）が行います。
  - 署名は §2 の受信の規則（`SignatureVerifier`）で検証します。署名が無い・検証できない GET は未ログインとして扱い、401 にはしません（Mastodon と同じ。401 にするのは Authorized Fetch だけ）。
  - 閲覧者のドメインは鍵の持ち主（`owner`）の Actor のホストです。`keyId` のホストと一致しないとき、ドメイン単位の `federation` の制限（§11）があるとき、自サーバーのときは未ログインとして扱います。
  - 関係は署名者のドメインで集約します（[visibility.md §1](visibility.md#1-入力)）。ブロックは署名者のアカウントと作者の間のものです。メンション先とフォロワーは、`CanView` が許さなかったときだけ読みます（フォロワーは 1,000 件ずつ、そのドメインのアカウントがいるかを `account.AnyOnDomain` で確かめる）。
  - 未ログインでも同じ結果になる応答（`public` / `unlisted`）だけ `Cache-Control: public, max-age=60` を付け、それ以外は `private, no-store` です。
  - HTML を求めるリクエスト（ブラウザ）は署名を持たないので、未ログインで閲覧できる投稿だけ表示用 URL へ `303` でリダイレクトし、それ以外は 404 です。
- コレクションはだれにも同じ内容を返します。outbox・followers・following は `totalItems` と最初のページ（`?page=true`）へのリンクを持つ `OrderedCollection` で、ページ（`OrderedCollectionPage`）は `partOf` と次のページ（`?max_id={カーソル}&page=true`）を持ちます（Mastodon と同じ形）。
  - outbox は、未ログインの閲覧者がプロフィールで見られる投稿（`public` / `unlisted`）の `Create` を、新しい順に 20 件ずつ返します。ブーストは載せません。`totalItems` は削除されていない投稿の数です（Mastodon と同じ）。
  - followers / following は、承認済みのフォローの相手の Actor の `id` を、新しいフォロー順に 40 件ずつ返します。有効でないアカウントは省き、`totalItems` はフォローの件数です。
  - featured は、ピン留めした投稿のうち未ログインで閲覧できるもの（`public` / `unlisted`）の `Note` を `orderedItems` に並べます。featured-tags は空の `Collection` です。
- Authorized Fetch（FR-408、S）はインスタンス設定 `authorized_fetch`（既定 `false`、[data-model §4.9](data-model.md#49-instance)）で有効にします。設定はリクエストごとに読むので、変更はすぐに効きます。
  有効なときは、検証できる署名の無い GET（署名なし・検証できない・未ログインとして扱う署名者）に次のように答えます。

  | 対象 | 署名なし | 検証できる署名 |
  | --- | --- | --- |
  | 利用者の Actor | 鍵の取得のための最小の文書（`@context`・`id`・`type`・`preferredUsername`・`inbox`・`publicKey`・`endpoints`。Mastodon と同じ項目） | Actor の文書全体 |
  | インスタンス Actor | 文書全体（もともと最小の文書） | 文書全体 |
  | オブジェクト（`/activity` を含む）・コレクション | `401` | 無効なときと同じ判定 |
  | WebFinger・NodeInfo・host-meta・HTML を求めるリクエスト | 無効なときと同じ | 無効なときと同じ |

  - `401` は、存在するかどうかを確かめる前に返します（存在が漏れない）。ADR-0032 の GET の署名を求める `Accept-Signature: sig1=("@method" "@target-uri");created` を付けるので、double-knocking のクライアントは RFC 9421 で再送します。
  - 応答は署名によって変わるので、すべて `private, no-store` です。だれにも同じ文書を返すインスタンス Actor だけは `public, max-age=60` のままです。
  - インスタンス Actor の GET は署名を検証せずに答えるので、Authorized Fetch を使うサーバーどうしでも、インスタンス Actor の鍵で署名した取得を、相手がその鍵を取得して（§6 のとおり自分のインスタンス Actor の鍵で署名して）検証できます。鍵の取得が鍵の検証を待つ循環になりません。利用者の Actor の最小の文書は、署名せずに鍵を取得する実装のためのものです。
  - 無効なとき、Actor とコレクションの GET は署名を検証しません（だれにも同じ応答なので）。

## 11. モデレーションとの連携

`moderation` の投影（`account_restrictions`・`domain_restrictions`、[data-model §5.2](data-model.md#52-処分の種類と制限能力の剥奪)）を、`moderation` の公開 API（`moderation.FederationPolicy`）から読みます。`federation` はテーブルを直接見ません。

| 制限 | 受信 | 送信 | 取得・表示 |
| --- | --- | --- | --- |
| ドメインの `federation`（`suspend`） | 署名の検証より前に `403`（§3.1） | 送らない | 取得しない。既存の投稿・アカウントは `profile_view` で隠す |
| ドメインの `public_discovery` / `unsolicited_reach`（`limit`） | 受け付ける | 送る | 連合タイムライン・検索に出さない。フォローしていない相手への通知を作らない |
| ドメインの `media_fetch`（`reject_media`） | 受け付ける | 送る | メディアプロキシが取得しない |
| ドメインの `report_intake`（`reject_reports`） | `Flag` を捨てる | — | — |
| ドメインの `media_sensitive`（`force_sensitive`） | 受け付ける | 送る | 添付をセンシティブとして表示 |
| アカウントの `federation`（`suspend` / `ban`） | リモートなら `Delete` / `Undo` / `Reject` 以外を捨てる | ローカルなら送らない | — |

- 判定はサブドメインにも効きます（[data-model §4.3](data-model.md#ドメイン単位の処分)）。
- `Block` の受信は `graph` の関係として記録します（相手からのブロックで、モデレーションの処分ではありません）。
- `Flag` の受信は `reports`（`uri` は送信者のホストのものだけ残す）になり、モデレーターの通報一覧に入ります（FR-701）。コメントは 5,000 文字で切ります。
- 通報の転送（`forwarded`）は、インスタンス Actor から対象のアカウントの inbox へ `Flag` を送ります（FR-701）。

## 12. テスト

| 種類 | 内容 | 実行 |
| --- | --- | --- |
| フィクスチャの回帰テスト | 実サーバー（Mastodon・Misskey・GoToSocial・Akkoma）から採取したアクティビティ・Actor・オブジェクトの JSON（`pkg/testkit/fixtures/federation/<実装>/<版>/…`）を正規化・ハンドラに通し、結果（正規化した値・書いた行）を golden ファイルと比べる。送信する文書は主要実装の形と比べる（既存の `internal/interface/activitypub/testdata/peers`） | `make check`（毎 PR） |
| 署名のテストベクタ | RFC 9421 付録 B のテストベクタ、Mastodon と同じ形式の draft-cavage の署名、クエリ文字列・ダイジェストの改ざん | `make check` |
| 順序のテスト | 同じアクティビティ列の順序を入れ替え・重複させて処理し、最終的な行が同じになることを確かめる性質テスト（`testkit.InboxProcessingContract` の `OrderAndDuplicates`。ケースは `inboxOrderCases` で、ハンドラを足すときにケースも足す） | `make check` |
| SSRF ガード | `testkit.HTTPClientContract`（ADR-0019 §1） | `make check` |
| ライブの相互運用 | docker compose で Mastodon（secure mode）・GoToSocial（余力があれば Misskey）とこのサーバーを、私設 CA の TLS とコンテナのネットワークの DNS で立て、WebFinger・Actor の署名付き取得・フォロー・投稿・返信・ブースト・お気に入り・削除・編集・公開範囲ごとの配送先を往復させる（#44）。構成とシナリオは `deploy/docker/federation-live/`。未実装の処理を待つシナリオは課題の番号を付けて skip する | nightly / 手動（CI ジョブ `federation-live`、`make federation-live`） |

- フィクスチャは採取したときのホスト・名前・本文・鍵を例の値に置き換えて保存し、実装と版をディレクトリ名に残します。新しい不具合を直すときは、その文書をフィクスチャに足します（ADR-0006）。
- 置き換えと追加の手順は [pkg/testkit/README.md](../../pkg/testkit/README.md#federation-fixtures)、各ファイルの出典とライセンスは `pkg/testkit/fixtures/federation/<実装>/README.md` にあります。正規化の golden は `pkg/testkit/testdata/golden/normalize/` にあり、`go test ./pkg/testkit -update` で書き直します。
- `make check` のテストはネットワークを使わず、`HTTPClient` をホスト名で振り分けるインメモリの実装に差し替えます（ADR-0015）。
- ライブの相互運用テストは、このサーバーが受信したアクティビティを保存したとおりに成果物へ書き出します。フィクスチャの採取元にする手順は [deploy/docker/federation-live/README.md](../../deploy/docker/federation-live/README.md#adding-a-captured-activity-as-a-fixture) にあります。
