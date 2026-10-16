# Spike #15: Go の ActivityPub / HTTP Signatures ライブラリ評価と最小 Actor の連合

- Issue: #15
- 関連 ADR: [ADR-0001](../adr/0001-server-language-and-runtime.md)（Accepted）、[ADR-0006](../adr/0006-federation-protocol-scope.md)（Proposed）、[ADR-0011](../adr/0011-license.md)（Accepted）
- 調査日: 2026-10-16
- プロトタイプ: [`spikes/15-go-activitypub/`](../../spikes/15-go-activitypub/)（使い捨て。プロダクトコードから import しない）

## 結論（要約）

- **HTTP Signatures はライブラリに任せる。暗号処理と署名ベースの組み立ては任せ、受け入れ条件（ポリシー）は自前で書く。**
  - RFC 9421: **`github.com/dadrus/httpsig`**（MIT）を推奨。`Content-Digest` の検証、`created` / `expires` の扱い、必須コンポーネントの指定ができ、go-ap も内部で採用している。
  - draft-cavage-12: **`code.superseriousbusiness.org/httpsig`**（BSD-3-Clause、GoToSocial が保守している go-fed/httpsig のフォーク）を推奨。本家 `go-fed/httpsig` は 2020 年以降コミットがない。
- **ActivityStreams の語彙（vocab）は自前の型にする。** `go-ap/activitypub` はパースの道具としては使えるが、拡張プロパティ（Mastodon の `toot:` 名前空間など）を保持できず、タグ付きリリースもない（擬似バージョンのみ）。コアの型としては採用しない。
- `go-ap/client` / `go-ap/auth` は採用しない。依存が重い（OAuth2 / osin / 自作キャッシュなど）うえ、ストレージやロガーまで go-ap 系の抽象と結合している。署名まわりの実装は参考になる。
- 最小 Actor（WebFinger + Actor + 署名検証付き Inbox + 署名付き Accept）は `net/http` と上記 2 ライブラリで **本体約 900 行・テスト約 450 行**で書けた。`CGO_ENABLED=0` でビルドでき、静的バイナリは約 13 MB。
- **実 Mastodon との往復は今回は未確認。** 作業環境に公開 URL がなく、外部トンネルサービスを使う設定は今回のスコープ外とした。代わりに、Mastodon と同じ形式の署名を（ライブラリを使わずに）手で組んで送るテストと、Mastodon から採取した形式の Follow / Actor JSON フィクスチャで検証した。

## 候補ライブラリ比較

GitHub API と Go module proxy で 2026-10-16 時点の状態を調べた。cgo の要否は `import "C"` の有無と `CGO_ENABLED=0` でのビルド結果で確認した。

### HTTP Signatures

| ライブラリ | 対応仕様 | ライセンス | 最終コミット / リリース | Open issues / PR | cgo | 評価 |
| --- | --- | --- | --- | --- | --- | --- |
| **dadrus/httpsig** | RFC 9421（+ RFC 9530 `Content-Digest`、`Accept-Signature`） | MIT | 2026-10-15 / v0.9.0（2026-05） | 3 / 3 | 不要 | **推奨（RFC 9421）**。`KeyResolver` インターフェースで keyid から鍵を遅延解決できる（＝ Actor を取得するまで鍵が分からない連合の用途に合う）。`content-digest` を覆っていれば本文のダイジェストも検証してくれる。RFC のテストベクタあり。v0 系なので破壊的変更はあり得る（v0.9.0 で NonceChecker の仕様が変わった）。依存は `dunglas/httpsfv` のみ。保守はほぼ 1 人。 |
| yaronf/httpsign | RFC 9421 | Apache-2.0 | 2026-10-08 / v0.6.2 | 1 / 0 | 不要 | 仕様の網羅度は高い（RFC 著者の 1 人による実装）。ただし Verifier を作る時点で鍵が必要なため、keyid → 鍵の解決を自前で 2 段階に組む必要がある。`lestrrat-go/jwx` などの依存が重い。次点。 |
| common-fate/httpsig | RFC 9421 | MIT | 2026-04 / v0.2.1（2024-11） | 1 / 0 | 不要 | 機能は揃っているが、リリースが止まっている。 |
| remitly-oss/httpsig-go | RFC 9421 | MIT | 2026-04 / v1.2.2 | 5 / 1 | 不要 | v1 系で API は安定している。ActivityPub での採用例が見当たらない。jwt 系に依存。 |
| **code.superseriousbusiness.org/httpsig** | draft-cavage-12 | BSD-3-Clause | v1.5.0（2025-10） | （Codeberg） | 不要 | **推奨（draft-cavage）**。go-fed/httpsig の API 互換フォークで、GoToSocial の本番で使われている。`:path` からクエリを除いて検証するオプションがある。 |
| go-fed/httpsig | draft-cavage-12 | BSD-3-Clause | 2020-12（v1.1.0） | 4 / 3 | 不要 | 実質的に保守停止。go-ap/client と go-ap/auth は今もこれに依存している。 |
| go-ap/httpsig | draft-cavage | Apache-2.0 | 2022-12 | — | — | アーカイブ済み。 |

### ActivityPub / JSON-LD

| ライブラリ | 内容 | ライセンス | 最終コミット / リリース | Open issues | cgo | 評価 |
| --- | --- | --- | --- | --- | --- | --- |
| go-ap/activitypub | AS2 語彙の型と JSON（デ）シリアライズ | MIT | 2026-10-15 / **タグなし**（擬似バージョンのみ） | 2 | 不要 | 活発に開発されている（ほぼ 1 人で 960 コミット超）。一方で API の変動が大きい（README にある `PersonNew` などのコンストラクタは現行版に存在しない）。**未知のプロパティを捨てる**ため、`discoverable` / `featured` などの Mastodon 拡張は往復で失われる。`publicKey` は単一の値しか扱えない（配列で複数鍵を持つ実装に未対応）。`type` の配列（複数型）には対応しており、パースの補助としては有用。実際にコンパイルされる依存は軽い（fastjson、x/text）。 |
| go-ap/jsonld | struct タグによる JSON-LD 風の marshal と `@context` 付与 | MIT | 2026-06 / タグなし | 2 | 不要 | JSON-LD の展開・圧縮（expansion / compaction）は**しない**。`@context` を付けるだけで、`json` パッケージの拡張に近い。 |
| go-ap/client | S2S/C2S クライアント（両方式で署名） | MIT | 2026-10-15 / タグなし | 0 | 不要 | `dadrus/httpsig` と `go-fed/httpsig` の両方を使って署名できる。ただし OAuth2、自作キャッシュ、`lw` ロガーなどが付いてくる。参考実装として読む価値はある。 |
| go-ap/auth | 受信署名の検証（RFC 9421 → draft の順で試す）+ OAuth2 | MIT | 2026-10-15 / タグなし | 0 | 不要 | 検証ロジックは本スパイクとほぼ同じ方針（`Signature-Input` があれば RFC 9421、なければ draft）。osin（OAuth2 サーバー）やストレージ抽象に結合しているため、ライブラリとしては取り込めない。 |
| go-fed/activity | AS2 の型生成 + ActivityPub のプロトコル実装 | BSD-3-Clause | 2022-01（v1.0.0 は 2020） | 21 | 不要 | 保守停止。 |
| code.superseriousbusiness.org/activity | go-fed/activity の GoToSocial フォーク | BSD-3-Clause | v1.19.0（2026-01） | （Codeberg） | 不要 | GoToSocial 専用に調整されている。生成コードが巨大で、型も重い。採用はしない。 |

ここに挙げたライブラリのライセンス（MIT / BSD-3-Clause / Apache-2.0）は、いずれも AGPL-3.0-only と両立する（ADR-0011）。

### 主要実装の状況（相互運用の前提）

- **Mastodon**（`CHANGELOG.md` で確認）
  - 4.4: RFC 9421 の受信検証を実験的に追加（`EXPERIMENTAL_FEATURES=http_message_signatures` で有効化）。
  - 4.5.0（2025-11）: **RFC 9421 の受信検証を常時有効にした。**
  - 4.7.0（2026-08）: **送信でも RFC 9421 を使い、受け入れられなければ draft にフォールバックする（送信側の double-knocking）。** HTTP Message Signatures で Ed25519 にも対応。FEP-8b32（Object Integrity Proofs）の検証も追加。
  - そのため、受信は RFC 9421 と draft-cavage の両方を受け付ける必要がある。送信は RFC 9421 を先に試し、draft にフォールバックする方式が、4.5 未満の Mastodon とも 4.7 系とも通じる。
- **GoToSocial**（参考のみ。コードは AGPL だが流用はしない）: `superseriousbusiness/httpsig` を使い、draft-cavage のみに対応している。署名対象は GET が `(request-target) host date`、POST がそれに `digest` を加えたもの。`algorithm="hs2019"` を名乗り、実際は RSA-SHA256 で署名する。検証時は RSA_SHA256 → RSA_SHA512 → ED25519 の順に試す。**クエリ文字列を含めた署名と含めない署名の両方を試す**、もう 1 つの double-knock も持っている（受信は再検証、送信は 401 のとき再送）。RFC 9421 には未対応。

## プロトタイプ

`spikes/15-go-activitypub/` は独立した `go.mod`（`github.com/best-friends/my/spikes/15-go-activitypub`）を持ち、Go 1.27.2（`.tool-versions`）でビルドする。

| ファイル | 内容 |
| --- | --- |
| `server.go` | WebFinger（`acct:` と Actor URI の両方を受け付ける）、Actor 文書（`publicKey` と Mastodon 拡張を含む。自前の map で組み立て）、Inbox（本文 1 MiB 上限 → 署名検証 → go-ap でパース → **署名鍵の所有者と `actor` の一致を確認** → Follow なら Accept を非同期で配送） |
| `verify.go` | 受信検証。`Signature-Input` があれば RFC 9421（dadrus）、なければ draft-cavage（sss/httpsig）。draft 側は、署名対象ヘッダ（`(request-target)`・`host`・`date` または `(created)`、POST では `digest`）の要件、`Date` の時間窓（Mastodon と同じく過去 12 時間 / 未来 1 時間）、`Digest` と本文の一致を**自前で**確認する。`@target-uri` は設定された公開オリジンから組み立て直し、`X-Forwarded-*` は信用しない |
| `client.go` | 署名付きの送信クライアント。**RFC 9421 で送り、400/401/403 が返ったら draft-cavage で 1 回だけ再送し、成功した方式をホストごとに覚える。** keyId から Actor を解決する処理（署名付き GET を使うので authorized fetch でも動く。素の `Key` オブジェクトが返ってきたら `owner` をたどる。`publicKey.id == keyId` と `owner == actor.id` を確認する） |
| `spike_test.go` + `testdata/` | 下記のテスト |

### テスト（`cd spikes/15-go-activitypub && go test ./...`）

ネットワークは使わない。ホスト名で振り分けるインメモリの `RoundTripper` で、`my.example`（本サーバー）と `mastodon.example`（偽 Mastodon）を相互に通信させる。

- `TestInboxMastodonCavageFollowGetsSignedAccept`: Mastodon と同じ形式の draft-cavage 署名（`headers="(request-target) host date digest content-type"`、`algorithm="rsa-sha256"`）を**ライブラリを使わずに組み立て**、採取した形式の Follow を送る。202 が返ること、偽 Mastodon の Inbox に署名付きの Accept が届き、偽 Mastodon 側で署名を検証できることを確認する。
- `TestInboxRFC9421FollowGetsSignedAccept`: RFC 9421 の署名ベース（§2.5）を手で組み立てて送り、dadrus での検証を独立した実装と突き合わせる。
- `TestDoubleKnockFallsBackToCavageAndRemembers`: 4.5 未満の Mastodon を模して、相手が RFC 9421 を 401 で拒否する場合。1 回目は RFC 9421 → draft の 2 回送信になり、2 回目からは draft の 1 回だけになることを確認する。
- `TestInboxRejections`: 署名なし、本文の改ざん（draft / RFC 9421 の両方）、`Date` の期限切れ、別の鍵での署名、`@target-uri` の不一致、鍵の所有者と `actor` の不一致（403）。
- `TestSignVerifyRoundTrip`: 両方式 × GET/POST（クエリ付き URL）の署名と検証の往復。
- `TestWebFinger`、`TestActorDocument`。

### 実 Mastodon との往復

**今回は実施していない。** 作業環境には公開 HTTPS オリジンがなく、cloudflared などの外部トンネルを立てることは今回の作業範囲外とした。手順は次のとおりで、人が手元で確認できる。

1. 公開 HTTPS で到達できるホストで `go run . -base https://<host> -user bob -addr :8080` を実行する（TLS はリバースプロキシで終端する。`-base` は公開オリジンと完全に一致させる。`@target-uri` の組み立てに使うため）。
2. Mastodon で `@bob@<host>` を検索してフォローし、ログで `inbox type=Follow scheme=...` と Accept の配送成功を確認する。Mastodon 側でフォローが「承認待ち」から「フォロー中」に変われば往復は成立している。

なお、この往復は ADR-0006 にある docker compose によるライブ相互運用テスト（nightly）で自動化するのが本筋である。

## 自前で実装する必要がある範囲

ライブラリが担うのは「署名ベースの組み立てと暗号処理」「`Content-Digest` の検証（RFC 9421）」だけ。それ以外は自前で書く。

1. **受信署名のポリシー**
   - 方式の判定と振り分け（`Signature-Input` の有無で判断する。どちらの方式も `Signature` ヘッダを使うため、1 つのリクエストに両方は載せられない）。
   - draft-cavage: `Signature` ヘッダのパース（ライブラリは署名対象ヘッダを公開しない）、必須ヘッダの確認、`Date` / `(created)` / `(expires)` の時間窓、`Digest`（RFC 3230）の検証、`hs2019` のときの鍵種別からのアルゴリズム決定。
   - RFC 9421: 必須コンポーネントの選定と、`expires` を必須にしない設定（ライブラリの既定は `created` と `expires` の両方を必須にし、有効期間を 30 秒とするため、Fediverse の実装には厳しすぎる）。複数の署名やラベルの扱い。
   - 公開オリジンから `@target-uri` / `(request-target)` を組み立て直すこと（プロキシ配下での不一致を防ぐ）。GoToSocial 式のクエリ有無の再試行を入れるかどうかの判断。
2. **鍵の解決**: keyId → Actor の取得（署名付き GET、`Key` オブジェクトの `owner` たどり）、`publicKey.id` / `owner` の照合、`publicKey` が配列の場合の扱い、キャッシュと鍵ローテーション（検証に失敗したら 1 回だけ取り直す）、取得失敗時の 401 と 503 の使い分け（Mastodon 4.5.4 は一時的な取得失敗に 503 を返す）。
3. **認可**: 署名者と `actor` の一致の確認。転送された activity（`actor` が別ホスト）は、原本を取り直すか、LD Signatures / FEP-8b32 で扱う。
4. **送信側の double-knocking**: RFC 9421 → draft へのフォールバック、ホストごとの方式の記憶（TTL を付けて定期的に RFC 9421 を再試行し、相手のアップグレードに追従する）、再送や配送ジョブ（ADR-0004）との組み合わせ。
5. **語彙と JSON**: Actor / Note / Activity の自前の型。Mastodon 拡張（`toot:`、`contentMap`、`Emoji`、`Hashtag`、`featured` など）。`@context` の生成。受信 JSON の揺れの吸収（文字列・オブジェクト・配列の混在、複数の `type`）。go-ap/activitypub をパースの補助として使うかどうかは実装開始時に判断する。
6. **WebFinger / host-meta / NodeInfo**（いずれも小さい）。
7. **SSRF ガード付き `HttpClient`**（NFR-SEC-1）を、鍵の取得・配送・メディア取得の全経路に通す。

## リスク

| リスク | 影響 | 緩和策 |
| --- | --- | --- |
| 署名ライブラリの保守者が少ない（dadrus/httpsig はほぼ 1 人、v0 系） | 破壊的変更や放置 | アダプタ境界（`federation/signature` ポート）の内側に閉じ込める。RFC テストベクタと実サーバーから採取したヘッダのフィクスチャで回帰テストし、入れ替えられる状態を保つ |
| draft-cavage は実装ごとの揺れが大きい（`hs2019`、`(created)`、クエリの有無、`Host` の扱い） | 特定の実装とだけ通じない | 実装ごとの採取フィクスチャを `testkit` に蓄積し、ライブ相互運用テストで確認する |
| go-fed 系の signer は `Host` を `req.Header` からしか読まない（クライアント側のリクエストでは `req.Host` にある） | 署名生成が失敗する（本スパイクで実際に踏んだ） | アダプタの中で `Host` ヘッダを補う。テストで固定する |
| dadrus の Verifier の既定値（`expires` 必須、有効期間 30 秒） | Mastodon などの署名を誤って拒否する | 既定値を使わず、明示的に設定する（本スパイクの `verify.go` を参照） |
| リバースプロキシや Cloud Run 配下での `@target-uri` / `@authority` の不一致 | すべての RFC 9421 検証が失敗する | 公開オリジンを設定値として持ち、そこから組み立て直す。`X-Forwarded-*` は信用しない |
| 送信方式の記憶が固定されてしまう | 相手がアップグレードしても draft のまま送り続ける | 記憶に TTL を付け、`InstanceHealth` に記録する |
| go-ap/activitypub の API が不安定で、タグ付きリリースもない | 追従のコスト | 採用するとしても受信パースの補助に限り、ドメインの型には持ち込まない |
| 実 Mastodon との往復を今回は検証していない | 想定外の拒否（例: RFC 9421 で Mastodon が要求するコンポーネントが想定と違う） | M1 の早い段階で docker compose のライブ相互運用テストを用意する（ADR-0006 の Open Question） |

## 参考

- Mastodon CHANGELOG（4.4.0 / 4.5.0 / 4.7.0 の HTTP Message Signatures の項）: <https://github.com/mastodon/mastodon/blob/main/CHANGELOG.md>
- GoToSocial: HTTP Signatures: <https://docs.gotosocial.org/en/latest/federation/http_signatures/>
- RFC 9421 HTTP Message Signatures / RFC 9530 Digest Fields
- SWICG ActivityPub HTTP Signature report（double-knocking）: <https://swicg.github.io/activitypub-http-signature/>
- dadrus/httpsig: <https://github.com/dadrus/httpsig>、yaronf/httpsign: <https://github.com/yaronf/httpsign>
- go-ap: <https://github.com/go-ap>
