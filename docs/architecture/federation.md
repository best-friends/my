# 連合の設計（ActivityPub S2S）

> 前提にしている ADR: [ADR-0002](../adr/0002-modular-monolith-hexagonal.md)、[ADR-0003](../adr/0003-database-abstraction.md)、[ADR-0004](../adr/0004-job-queue-abstraction.md)、[ADR-0005](../adr/0005-blob-storage-and-media.md)、[ADR-0006](../adr/0006-federation-protocol-scope.md)、[ADR-0017](../adr/0017-visibility-local.md)、[ADR-0018](../adr/0018-federated-uris.md)、[ADR-0019](../adr/0019-security-baseline.md)、[ADR-0020](../adr/0020-data-modeling-principles.md)。
> Proposed の [ADR-0032](../adr/0032-http-signature-profile.md)（署名）、[ADR-0033](../adr/0033-inbox-processing-model.md)（受信処理）、[ADR-0034](../adr/0034-remote-content-scope-and-retention.md)（取り込み範囲と保持）、[ADR-0035](../adr/0035-local-visibility-mention-tags.md)（`local` 投稿のメンション）の推奨案に従っています。決定で推奨案が変われば、この文書を直します。
> 要件: FR-305、FR-401〜410、FR-701〜702、NFR-SEC-1〜2、NFR-SEC-4〜5、NFR-PERF-4、NFR-REL-1〜2、NFR-COST-4〜5。テーブルは [論理データモデル](data-model.md) §4.8 ほかにあります。

## 1. 構成

### 1.1 モジュールとパッケージ

| パッケージ | 持つもの |
| --- | --- |
| `internal/domain/federation` | AS2 の正規化した型と、受信した JSON からの正規化（§4）。宛先から公開範囲への写像、配送先の算出規則、配送の結果の分類、サーキットブレーカーの規則。標準ライブラリだけを使う純粋な関数 |
| `internal/app/federation` | ユースケース（受信 `Receive`、アクティビティの処理、配送の展開 `FanOut`、配送 `Deliver`、Actor / オブジェクトの取得 `Fetch`、解決 `Resolve`）、ジョブハンドラ、ポート（`Signer`・`Verifier`・リポジトリ） |
| `internal/app/kernel` | `HTTPClient` ポート（連合のほか、メディアプロキシ・リンクプレビュー・Web Push も使うので全モジュール共通に置く） |
| `internal/adapter/httpclient` | SSRF ガード付きの `HTTPClient`（ADR-0019 §1）。コントラクトテストは `testkit.HTTPClientContract` |
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

## 3. 受信（Inbox）

ADR-0033 の推奨案に従い、**署名の検証までを同期**、**処理を非同期** にします（NFR-PERF-4）。

### 3.1 同期の部分

`POST /ap/inbox`（shared inbox）と `POST /ap/actors/{uuid}/inbox`（有効なローカルの Actor でなければ 404）は、次の順に処理します。どこかで失敗したら、DB とキューに触れずに応答します。

| 順 | 処理 | 失敗時 |
| --- | --- | --- |
| 1 | `Content-Type` が `application/activity+json`、`application/ld+json`（AS2 のプロファイル付き）、`application/json` のどれか | `415` |
| 2 | 本文を 1 MiB まで読む（`http.MaxBytesReader`、ADR-0019 §5） | `413` |
| 3 | IP 単位のレート制限（ADR-0019 §5） | `429` + `Retry-After` |
| 4 | 署名ヘッダを解析し、`keyId` のホストを得る | `401`（署名なし）/ `400`（解析不能） |
| 5 | `keyId` のホストのドメイン単位の処分（`federation` の制限、§11） | `403` |
| 6 | ドメイン単位のレート制限（`keyId` のホスト） | `429` |
| 7 | ダイジェストの照合 | `401` |
| 8 | 本文の JSON から `id`・`type`・`actor`・`object`（の `id`）だけを読む | `400` |
| 9 | 未知の Actor の自分自身の `Delete` / `Update`（`actor == object`、`accounts (uri)` に無い）なら、鍵を取りに行かずに捨てる（ADR-0019 §7） | `202` |
| 10 | 鍵の解決（未知・期限切れなら Actor を取得。5 秒で打ち切る） | `401`（取得できない）/ `503`（取得先がサーキットブレーカーで止まっている） |
| 11 | 署名の検証（ADR-0032 の必須の対象・時間窓） | `401` + `Accept-Signature` |
| 12 | 署名者と `actor` の照合。一致しなければジョブに `trust = refetch` を付ける | — |
| 13 | 1 つの短いトランザクションで、`received_activities` に `ON CONFLICT DO NOTHING` で挿入し、行が増えたときだけ `received_activity_documents` と `federation.inbox` ジョブ（`queue.Outbox`）を書く | — |
| 14 | 署名者のドメインの `instance_health` を `available` に戻す（§7.3） | — |
| 15 | `202 Accepted` | — |

- `id` の無いアクティビティは、`actor` と本文の SHA-256 から `urn:my:digest:<hex>` を合成して重複排除のキーにします（[data-model §2.2](data-model.md#22-冪等キーと一意制約)）。
- `id` のホストが `actor` のホストと異なるアクティビティは、`trust = refetch` にします。
- 鍵の取得はトランザクションの外で行います（ADR-0003）。取得した Actor の書き込みは、受信の記録とは別の短いトランザクションです。

### 3.2 非同期の部分

ジョブ `federation.inbox`（ペイロードは `activity_uri` と `trust`、冪等キーは `inbox:<activity_uri のハッシュ>`、`orderingKey` は `inbox:<actor の URI>`）が、原本を読んで処理します。

1. 原本を正規化する（§4）。正規化できなければ `Fail`（dead job に残る）。
2. `trust = refetch` なら、オブジェクトを `id` からオリジンに取得し直し（§6）、取得した文書を使う。`Announce` 以外で取得できなければ捨てる。
3. 主体（`actor`）のアカウントを解決する（未知なら取得、§6）。主体やそのドメインに `federation` の制限があれば捨てる（`Delete` / `Undo` / `Reject` は通す。§11）。
4. 種類ごとのハンドラ（§3.4）を呼ぶ。ハンドラは 1 つの短いトランザクションで、各モジュールの公開 API を呼んで書く。
5. 結果: 書けたら `Done`、取得の一時的な失敗は `Retry`（ADR-0004 のバックオフ）、恒久的に処理できないものは `Fail`。

- 受信の記録は同期の部分で書くので、送信側が同じアクティビティを再送してもジョブは増えません。処理に失敗したアクティビティは dead job として残り、CLI（FR-801）から再実行します。
- `orderingKey` は同じ Actor のアクティビティを直列化しますが（#77）、正しさはそれに頼りません（§3.3）。

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
- アクティビティの `object` が URI だけのとき、受信した文書の中に埋め込まれていない対象は、ローカルの DB から引くか、取り込み範囲に入るときだけ取得します。

### 3.4 アクティビティごとのハンドラ

| アクティビティ | 対象 | 処理 |
| --- | --- | --- |
| `Follow` | ローカルの Actor | `graph.ReceiveFollow`。鍵アカウントでなければ承認し、`Accept` を配送する。鍵アカウントなら承認待ち + 通知。ブロック関係があれば `Reject` を配送する |
| `Accept` | 自分が送った `Follow`（URI か埋め込み） | `graph.AcceptRemote`（`follow_acceptances`、`accepted_by = remote`）。`object` の `id` が未知なら、`object.actor`（ローカル）と `actor`（相手）の承認待ちのフォローで照合する |
| `Reject` | 自分が送った `Follow`、または承認済みのフォロー | 承認待ちなら `follow_rejections`、承認済みなら `follow_terminations`（`removed_by_followee`） |
| `Undo` | `Follow` / `Like` / `Announce` / `Block` / `Accept` | 対象の取り消し。`Undo Accept` はフォローの終了として扱う。対象が URI だけなら、主体が作った同じ URI の `follows` / `favourites` / `boosts` / `blocks` を順に引く |
| `Create` | `Note` / `Question`。`Article` / `Page` / `Event` / `Image` / `Video` / `Audio` は変換して取り込む | 取り込み範囲（ADR-0034）に入れば `content.IngestRemoteStatus`。オブジェクトの `id` のホストは `actor` のホストと一致しなければならない |
| `Update` | 投稿 | `content.ReviseRemoteStatus`（新しい版）。投稿の作者とアクティビティの主体が一致し、オリジンが同じであること（[ActivityPub §7.3](https://www.w3.org/TR/activitypub/#update-activity-inbox)） |
| `Update` | Actor（自分自身） | 埋め込みの文書で `account.UpsertRemoteActor`。`id` が主体と一致しなければ取得し直す |
| `Delete` | 投稿・`Tombstone` | `content.DeleteRemoteStatus`（`origin = remote`）。未知なら tombstone |
| `Delete` | Actor（自分自身） | `account.MarkRemoteDeleted`（`remote_deleted`）。投稿・フォロー・ブーストは消去ジョブが消す |
| `Announce` | 投稿（URI か埋め込み） | `content.RecordRemoteBoost`。埋め込みの投稿は、オリジンが主体と同じでなければ `id` から取得し直す |
| `Like` / `EmojiReact` | ローカルの投稿 | `content.RecordRemoteFavourite`。Misskey の絵文字リアクション（`_misskey_reaction` 付きの `Like`）と Pleroma / Akkoma の `EmojiReact` もお気に入りとして扱う |
| `Block` | ローカルの Actor | `graph.ReceiveBlock`。両方向のフォローを終了し、相手の投稿を閲覧者から隠す（`feed_exclusions`） |
| `Flag` | ローカルの Actor / 投稿 | `moderation.FileRemoteReport`（`reports.uri`）。送信者のドメインに `report_intake` の制限があれば捨てる |
| `Add` / `Remove` | 主体の `featured` | ピン留めの一覧を取得し直す（C） |
| `Move` ほか未知の型 | — | ログとメトリクスに残して完了にする（`Move` は v0 の非目標） |

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
- 404 / 410 と取得の失敗は、KV に 1 時間記録して同じ URI の取得を繰り返しません。Actor の 410 は `remote_deleted` にします。
- ドメイン単位の `federation` の制限があるドメインからは取得しません。

### 6.2 Actor とハンドル

- Actor の文書から `account.UpsertRemoteActor` が `remote_account_snapshots`（内容が変わったときだけ）、プロフィールの版、公開鍵を書きます。
- ハンドル（`account_handles`）は WebFinger で確かめたものだけです。`acct:{preferredUsername}@{Actor のホスト}` を引き、`self` リンクが Actor の `id` を指すこと、`subject` のドメインが異なるならそのドメインでも同じことを確かめます（ADR-0018 のドメイン分離に相当する構成）。確かめられなければハンドルを持たないアカウントとして扱います。
- `@user@host` / URL での解決（FR-305）は、`federation.Resolve` が WebFinger → Actor の取得 → 書き込みを行います。URL がオブジェクト（投稿）なら投稿として取り込みます。
- 取得から 24 時間を過ぎた Actor は、次に使われたとき（署名の検証・表示・配送）に非同期で取り直します。ローカルのアカウントがフォローしている Actor は、定期ジョブ `federation.refresh_actors` が `remote_account_heads (last_fetched_at)` の古い順に取り直します（ADR-0034）。

### 6.3 投稿

- 取り込み範囲は ADR-0034 に従います（ローカルのアカウントとの関係がある投稿だけ）。
- 返信先は受信時に 1 段だけ非同期（`federation.fetch_object`、ペイロードに残りの深さ）で取得し、スレッドの祖先と返信の一覧は利用者がスレッドを開いたときに深さ 20・件数 100 まで取得します。
- 投稿の ID は `published` から作った UUIDv7 です（[data-model §1.3](data-model.md#13-id-とキーセットページング)、#71）。

## 7. 送信（配送）

### 7.1 記録

ローカルの操作のトランザクションで、`federation` が他のモジュールのドメインイベントを購読し（`uow.Projectors`）、`outbound_activities` と `federation.fanout` ジョブ（`queue.Outbox`）を同じトランザクションで書きます。

| ドメインイベント | アクティビティ | 宛先 |
| --- | --- | --- |
| `content.status_created` | `Create` | §7.2 の公開範囲ごとの宛先 |
| `content.status_revised` | `Update` | 元の `Create` の `deliveries` の inbox（`public` / `unlisted` は加えてその時点のフォロワー。[data-model §7](data-model.md#7-公開範囲と配送)） |
| `content.status_deleted` | `Delete`（`Tombstone`） | 元の `Create` の `deliveries` の inbox |
| `content.boosted` / `content.boost_retracted`（`Federates` のときだけ） | `Announce` / `Undo Announce` | ブーストした人のリモートフォロワー + 元投稿の作者（リモートなら） |
| `content.favourited` / `content.unfavourited`（リモートの投稿だけ） | `Like` / `Undo Like` | 投稿の作者の inbox |
| `graph.follow_requested` / `graph.followed`（相手がリモート） | `Follow` | 相手の inbox |
| `graph.follow_terminated`（自分がフォローした相手がリモート） | `Undo Follow` | 相手の inbox |
| `graph.follow_accepted` / `graph.follow_rejected`（フォローした人がリモート） | `Accept` / `Reject` | フォローした人の inbox |
| `graph.follow_terminated`（`removed_by_followee`、フォローした人がリモート） | `Reject` | フォローした人の inbox |
| `graph.blocked` / `graph.unblocked`（相手がリモート） | `Block` / `Undo Block` | 相手の inbox |
| プロフィールの版（`account`） | `Update`（`Person`） | リモートフォロワーの inbox |
| アカウントの削除（`deleted`） | `Delete`（`Person`） | リモートフォロワーの inbox + 既知のドメインの shared inbox（[data-model §8](data-model.md#8-削除と消去)） |

- `local` 投稿のブースト（`Boost.Federates` が偽）は記録しません（ADR-0017）。
- アクティビティの文書は `federation.fanout` が 1 回だけ組み立て、`outbound_activity_documents` に保存します。すべての配送とリトライが同じ本文を送ります（再試行で内容が変わらず、消去後の `Delete` も送れる）。
- 投稿の文書は、イベントが指す版から組み立てます（後の編集で内容が変わらない）。

### 7.2 展開（fan-out）

`federation.fanout`（ペイロードは `outbound_activity_id` とカーソル）が、宛先を一定件数（500 アカウント）ずつ算出します。

1. 宛先のアカウントを集める: 作者の承認済みのリモートフォロワー（`graph` の公開 API をカーソルで）と、版のメンション先・宛先。
2. 公開範囲の規則で絞る: `content.CanView`（`ContextDelivery`、[visibility.md](visibility.md)）を、宛先のドメインごとに集約した関係で判定する。
   - `local` は承認済みのフォロワーだけで、**フォロワーでないリモートのメンション先には送りません**（ADR-0017）。
   - `local` 投稿の文書の `tag` には、フォロワーでないリモートのアカウントへの `Mention` を入れません（ADR-0035）。本文のリンクは残します。
3. inbox を決める: アカウントの `shared_inbox_url`（無ければ `inbox_url`）。同じ inbox は 1 つにまとめます（shared inbox への集約、NFR-COST-5、[ActivityPub §7.1.3](https://www.w3.org/TR/activitypub/#shared-inbox-delivery)）。
4. ドメインの状態で絞る: `federation` の制限（ドメイン単位の `suspend`、§11）があるドメインと、`instance_health` が `unavailable`（§7.3）のドメインには送りません（`Follow` と `Accept` / `Reject` は `unavailable` でも送る）。
5. `deliveries` に `ON CONFLICT DO NOTHING` で挿入し、新しい行ごとに `federation.deliver` ジョブ（冪等キー `deliver:<delivery_id>`）を投入します。
6. 残りがあれば、カーソルを進めた `federation.fanout` を投入します。

### 7.3 配送とリトライ

`federation.deliver`（ペイロードは `delivery_id`）:

1. 宛先のドメインが `unavailable` になっていたら、試行せず `skipped` を記録して完了にする。
2. 本文に署名して POST する（§2 の double-knocking）。
3. 結果を分類して `delivery_attempts` に記録する。同期 projector が `instance_health` を更新する。

| 応答 | 分類 | ジョブ |
| --- | --- | --- |
| 2xx | `succeeded` | `Done` |
| 408 / 429 / 5xx（501 を除く）/ 接続・TLS・タイムアウトのエラー | `retryable_failure` | `Retry`（`Retry-After` があればそれ以降） |
| 400 / 401（double-knocking の後）/ 403 / 404 / 410 / 413 / 422 などの 4xx、501 | `permanent_failure` | `Done`（相手が受け取らないと答えたので、リトライしない） |

- リトライの間隔は ADR-0004 の配送の例（1 分、5 分、30 分、2 時間、12 時間、24 時間 × 3。最大 9 回、約 4 日）で、ジッタを掛けます（`RegisterWithPolicy`）。使い切ったジョブは dead job になり、管理画面と CLI から再実行できます（NFR-REL-2）。
- **短期のサーキットブレーカー**: プロセス内で inbox のホストごとに、連続 10 回失敗したら 60 秒間はリクエストせずに `Retry` を返します（Mastodon の `Stoplight` と同じ値）。
- **`InstanceHealth`（長期）**: `retryable_failure` を記録した日（UTC の日付）がドメインごとに異なる 7 日に達したら、`instance_availability_events`（`marked_unavailable`）を記録し、`instance_health.state` を `unavailable` にします（Mastodon の `DeliveryFailureTracker` と同じ基準）。
  配送の成功、またはそのドメインから署名付きのリクエストを受けたら `marked_available` を記録して `available` に戻します。`unavailable` のドメインには、定期ジョブ `federation.probe_instances` が 1 日 1 回 NodeInfo を取得し、成功したら戻します。
- 1 件の配送は 1 つのジョブです。Cloud Tasks / SQS では 1 回の試行が 1 オペレーションとして課金されるので、shared inbox への集約と `unavailable` のドメインの除外で回数を抑えます（NFR-COST-5）。

### 7.4 文書の形

- `Create` の `id` は `{object}/activity`、それ以外は `/ap/activities/{uuid}`（ADR-0018）。
- 宛先: `public` は `to: [Public]`・`cc: [followers, メンション先]`、`unlisted` は `to: [followers]`・`cc: [Public, メンション先]`、`private` と `local` は `to: [followers]`（`private` は加えて `cc: [メンション先]`）、`direct` は `to: [メンション先]`。
- `contentMap` は版の言語ごとの本文、`content` は版の既定の言語の本文です（NFR-I18N-5）。`sensitive`・`summary`（CW）・`tag`（`Mention` / `Hashtag` / `Emoji`）・`attachment`（メディアの公開 URL・`mediaType`・`name`・`blurhash`・`focalPoint`・`width` / `height`）を付けます。
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
- クライアント API は、リモートのメディアの URL を **メディアプロキシの URL** `https://{host}/media/proxy/{media_id}/{variant}?sig=…` で返します。`sig` は `media_id` と `variant` の HMAC で、プロキシを任意の URL の中継に使わせません。
- プロキシ（`internal/interface/media`）は `HTTPClient` で元の URL を取得し、形式を中身で判定して（ADR-0019 §8。SVG は拒否）、`Cache-Control: public, max-age=…` と `Content-Security-Policy: default-src 'none'; sandbox` を付けて中継します。CDN がキャッシュします（NFR-COST-4）。
  サイズの上限は画像 16 MiB、動画・音声 100 MiB です（ADR-0019 §1）。
- インスタンス設定 `remote_media_policy`（`none` / `avatars` / `all`）と `remote_media_cache_ttl` で、TTL 付きのキャッシュを選べます（FR-409）。キャッシュは `remote_media_cache` と Blob（`remote/` のキー）に置き、`media.cleanup_remote` が期限と容量の上限で消します（FR-803）。
- ドメイン単位の `media_fetch` の制限（`reject_media`）があるドメインのメディアは、プロキシが取得せず 404 を返します。`media_sensitive` の制限（`force_sensitive`）は表示のときに添付をセンシティブにします。
- サムネイル・blurhash がない動画は、クライアントが「プレビューなし」として表示します。

## 10. コレクションとオブジェクトの GET

- `GET /ap/objects/{uuid}`（`/activity` を含む）は、署名から閲覧者（署名者のドメイン）を決め、`content.CanView`（`ContextObjectFetch`）を通します。権限が無いときと存在しないときは同じ 404 です（ADR-0018）。削除済みは `410 Gone` と `Tombstone` です（[ActivityPub §6.4](https://www.w3.org/TR/activitypub/#delete-activity-outbox)）。
- outbox は `public` / `unlisted` の投稿の `Create` をページで返し、followers / following は件数とページを返します（利用者が非公開にした場合は件数だけ）。featured はピン留めした `public` / `unlisted` の投稿です。
- Authorized Fetch（FR-408、S）は設定 `authorized_fetch` で有効にし、署名のない GET を Actor（鍵の取得のための最小の文書を除く）とオブジェクトで 401 にします。

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
| 順序のテスト | 同じアクティビティ列の順序を入れ替え・重複させて処理し、最終的な行が同じになることを確かめる性質テスト | `make check` |
| SSRF ガード | `testkit.HTTPClientContract`（ADR-0019 §1） | `make check` |
| ライブの相互運用 | docker compose で Mastodon・GoToSocial（余力があれば Misskey）とこのサーバーを、私設 CA の TLS とコンテナのネットワークの DNS で立て、フォロー・投稿・返信・ブースト・お気に入り・削除・編集・公開範囲ごとの配送先を往復させる（#44） | nightly / 手動（CI ジョブ `federation-live`） |

- フィクスチャは採取したときのホスト・名前・本文を例の値に置き換えて保存し、実装と版をディレクトリ名に残します。新しい不具合を直すときは、その文書をフィクスチャに足します（ADR-0006）。
- テストはネットワークを使わず、`HTTPClient` をホスト名で振り分けるインメモリの実装に差し替えます（ADR-0015）。
