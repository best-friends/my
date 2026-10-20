# ADR-0057: 検索エンジンと AI クローラーの拒否の表明

- Status: Proposed
- Date: 2026-10-20
- Deciders: @rosylilly
- Issue: #647

## Context（背景・課題）

決定者は「個人単位・サーバー単位での検索インデックス拒否・AI クロール拒否の明示の導入。正確な robots.txt の配置。必要な well-known の掲示」を求めている（#647）。
実装は #656 がこの決定を待つ。関連する要件と決定は FR-006（プロフィール）、FR-401 / FR-402（WebFinger・host-meta・NodeInfo）、FR-705（インスタンス設定）、
[ADR-0017](0017-visibility-local.md)（公開範囲）、[ADR-0018](0018-federated-uris.md)（URI と HTML / AP の振り分け）、
[ADR-0028](0028-html-shell-link-previews-and-pwa.md)（Go が返す HTML の外枠とリンクプレビュー）。

main の現状:

- `/robots.txt` の経路はない。`internal/interface/web` の `Handler.ServeHTTP` は予約されていないパスに SPA の汎用の外枠（HTML、200）を返すので、
  `/robots.txt` にも HTML が返る。RFC 9309 のパーサーは HTML から規則を読めず、全体を許可として扱う。
- `noindex` は `<meta name="robots">`（`internal/interface/web/shell.go`）と `X-Robots-Tag`（`internal/interface/web/preview.go`）で、
  `unlisted` の投稿と、見せない・存在しないページの 404 にだけ付く。
  リモートのアカウントのページ（`/@bob@remote.example`）とアプリの画面（`/home` `/settings` など）は、汎用の外枠を 200 で `noindex` なしに返す。
- アカウントの `discoverable` / `indexable` は `account_profile_revisions` に保存され、クライアント API で変えられ（登録時はどちらも `false`）、
  Actor に載る（`internal/app/federation/actor_document.go`）。ほかでは読まれず、ページにも検索にも効かない。
  リモートの Actor の `indexable` は正規化（`internal/domain/federation/normalize.go`）で読まれず、常に `false` で保存される。
- メディアは `/media/` を `internal/interface/media` が返すか、オブジェクトストレージと CDN が同じキーで直接返す（ADR-0005）。キーにアカウントは含まれない。
- `/.well-known/` にあるのは `webfinger`・`host-meta`・`host-meta.json`・`nodeinfo` だけ。

### 表明の手段の状態

2026-10-20 に一次情報で確かめた。

| 手段 | 状態 | 主な読み手 | 出典 |
| --- | --- | --- | --- |
| `robots.txt`（`User-agent` / `Allow` / `Disallow`） | 標準（RFC 9309、2022-09）。ホストの `/robots.txt`、`text/plain`、UTF-8。少なくとも 500 KiB まで読む | 主要な検索エンジンと AI の各社が「従う」と明記 | [RFC 9309](https://www.rfc-editor.org/rfc/rfc9309) |
| `<meta name="robots">` / `X-Robots-Tag` の `noindex` `nofollow` `noarchive` など | 事実上の標準（Google・Bing が文書化） | 検索エンジン | [Google: robots meta](https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag) |
| AI 用の User-agent（学習: `GPTBot` `ClaudeBot` `CCBot` `Google-Extended` `Applebot-Extended` など。AI 検索・利用者の依頼: `OAI-SearchBot` `ChatGPT-User` `Claude-SearchBot` `Claude-User` など） | 各社の文書。`Google-Extended` は取得をせず「Gemini の学習と grounding に使わない」ことだけを表す。`ChatGPT-User` は利用者の操作なので robots.txt が効かないことがある | 各社 | [Google](https://developers.google.com/crawling/docs/crawlers-fetchers/google-common-crawlers)、[OpenAI](https://developers.openai.com/api/docs/bots)、[Anthropic](https://support.claude.com/en/articles/8896518-does-anthropic-crawl-data-from-the-web-and-how-can-site-owners-block-the-crawler) |
| AI クローラーの一覧 | `ai-robots-txt/ai.robots.txt`（MIT）。`robots.json` に名前・運営者・用途を持ち、`robots.txt` を生成する。link preview の取得者（`facebookexternalhit`）や検索（`Applebot`）も含む | — | [GitHub](https://github.com/ai-robots-txt/ai.robots.txt) |
| IETF AI Preferences（`Content-Usage`） | 草案（Proposed Standard を目指す WG 文書）。語彙は `draft-ietf-aipref-vocab-08`（2026-09-13）で `train-ai` / `ai-use` / `search`、値は `y` / `n`（RFC 9651 の Dictionary）。付け方は `draft-ietf-aipref-attach-05`（2026-08-18）で HTTP の `Content-Usage` フィールドと robots.txt の `Content-Usage: [path] <prefs>` 行。衝突したら最も制限の強い表明が勝つ。WG の合意を反映していない旨の注記がある | 対応を表明した大手のクローラーは未確認（要検証） | [vocab](https://datatracker.ietf.org/doc/draft-ietf-aipref-vocab/)、[attach](https://datatracker.ietf.org/doc/draft-ietf-aipref-attach/) |
| Content Signals（`Content-Signal: search=yes, ai-train=no, ai-input=no`） | Cloudflare の方針（2025-09-24、CC0）。robots.txt の行。EU DSM 指令 4 条の留保と位置づける | Cloudflare の管理する robots.txt。各社の対応は未確認（要検証） | [Cloudflare blog](https://blog.cloudflare.com/content-signals-policy/) |
| TDM Reservation Protocol（TDMRep） | W3C Community Group の Final Report（2024-05-10）。W3C 標準ではない。`/.well-known/tdmrep.json`（`location` / `tdm-reservation` / `tdm-policy` の配列）、HTTP ヘッダーと meta の `tdm-reservation: 1` | 欧州の出版業界。AI 各社の対応は未確認（要検証） | [TDMRep](https://www.w3.org/community/reports/tdmrep/CG-FINAL-tdmrep-20240510/) |
| `noai` / `noimageai`（robots meta / `X-Robots-Tag`） | 非標準。DeviantArt が 2022 年に始めた値。対応を表明した大手のクローラーは見つからない | Misskey・GoToSocial が出す | [IPTC metawatch](https://metawatch.iptc.org/ai-policy/noai-meta/) |
| `ai.txt` | 非標準。Spawning の形式のほか、同名の別の提案（個人の Internet-Draft など）が並立する。大手のクローラーが読むという文書はない | — | [draft-car-ai-txt-wellknown-00](https://www.ietf.org/ietf-ftp/internet-drafts/draft-car-ai-txt-wellknown-00.txt) |
| `security.txt` | 標準（RFC 9116）。`/.well-known/security.txt`、`Contact` と `Expires` が必須 | 脆弱性の報告者 | [RFC 9116](https://www.rfc-editor.org/rfc/rfc9116) |

日本の著作権法 30 条の 4 では、こうした表明は法的な拒否として扱われない場合がある（要検証）。それでも多くの事業者が自主的に従うので、表明には意味がある。

### 主要実装の扱い

2026-10-20 に各実装のソースで確かめた。

| 観点 | Mastodon | Misskey | GoToSocial |
| --- | --- | --- | --- |
| robots.txt | 静的。`GPTBot` を全面拒否し、`*` は `/media_proxy/` `/interact/` などを拒否（[public/robots.txt](https://github.com/mastodon/mastodon/blob/main/public/robots.txt)） | 動的。`*` に設定・API・`/proxy` などを拒否。未ログインに投稿を見せない設定では `/@` と `/notes` も拒否（[ClientServerService.ts](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/server/web/ClientServerService.ts)） | 埋め込み。ai.robots.txt の一覧（`facebookexternalhit` を含む）を `Disallow: /`、`*` に API・認証・`/users/`・webfinger などを拒否（[robots.go](https://codeberg.org/superseriousbusiness/gotosocial/src/branch/main/internal/api/util/robots.go)） |
| 個人の検索エンジン拒否 | 利用者設定 `noindex`（既定はサーバー設定 `noindex`）で、プロフィールと投稿に `noindex, noarchive` | `noCrawle` で `noindex`。リモートの利用者の投稿と renote も常に `noindex` | プロフィールの設定で `noindex, nofollow` |
| 個人の AI 拒否 | なし | `preventAiLearning` で `noai` `noimageai`（meta と `X-Robots-Tag`） | 常に `noai, noimageai` |
| 連合 | Actor の `discoverable`（ディレクトリなど）と `indexable`（公開投稿の全文検索、opt-in）を送る。Mastodon の文書は外部のサービスにも尊重を求める（[spec](https://docs.joinmastodon.org/spec/activitypub/)） | `discoverable` などを送る | `discoverable` などを送る |

## Decision Drivers（判断基準）

1. **利用者の意思を尊重する**: 本人が拒んだものは、サーバーの設定でも、連合先での表示でも、許可に変わらない。表明が衝突したら最も制限の強いものを採る
2. **読まれる手段で表明する**: 主要なクローラーが文書で「従う」としている手段（robots.txt の User-agent、`noindex`）を軸にし、標準化の途中の手段は安く足せるものだけ足す
3. **既存の決定を壊さない**: リンクプレビュー（ADR-0028）、存在を漏らさない 404（ADR-0017 / ADR-0018）、連合の取得（WebFinger・NodeInfo・AP）を止めない
4. **運営の手間が小さい**: AI クローラーの一覧の保守を運営者に求めない。キャッシュと CDN の配信（NFR-COST）を壊さない
5. **語彙を増やしすぎない**: 設定の数を最小にし、`indexable` / `discoverable` の連合での意味（Mastodon の定義）とずらさない

## Considered Options（選択肢）

### A. 表明の手段

1. **A1: robots.txt を軸に、`noindex` と IETF AI Preferences と TDMRep を重ねる**（`noai` / `noimageai` も安いので付ける。`ai.txt` は出さない）
2. A2: robots.txt と `noindex` だけ（標準と事実上の標準だけ）
3. A3: GoToSocial と同じく、ai.robots.txt の一覧をそのまま `Disallow: /` にする

### B. 個人とサーバーの関係

1. **B1: サーバーと個人の両方に設定を持ち、実際の表明は両者のうち制限の強いほうにする**
2. B2: サーバーの設定だけ（個人の設定を持たない）
3. B3: 個人の設定がサーバーの設定より優先する（個人が許可を広げられる）

### C. 個人の検索の設定

1. **C1: 既存の `indexable` を「公開投稿とプロフィールを検索（このサーバーの検索、外部の検索エンジン）に載せてよい」として使う**
2. C2: Mastodon と同じく、外部の検索エンジン用に `noindex` を別に持つ

### D. 個人の AI の拒否のサーバー外への伝え方

1. **D1: このサーバーが返すページとヘッダーで表明する。AP の独自の拡張は作らない**
2. D2: Actor に独自の拡張（例: `noai`）を足して連合先にも伝える

## Decision（決定）

> 推奨案（Proposed）。Accepted にするのは人間の Decider のみ。

**A1・B1・C1・D1 を推奨する。**

### 設定

| 単位 | 設定 | 値 | 既定 |
| --- | --- | --- | --- |
| サーバー（インスタンス設定、`instance_setting_revisions` の新しいキー） | `search_indexing` | `allow` / `deny` | `allow` |
| サーバー | `ai_usage` | `allow` / `deny_training`（学習を拒否）/ `deny`（学習と AI での利用を拒否） | `deny` |
| サーバー | `security_contact` | `mailto:` か `https:` の URI。空なら `security.txt` を返さない | 空 |
| 個人（`account_profile_revisions`） | `indexable`（既存） | `true` / `false` | `false`（opt-in） |
| 個人 | `ai_opt_out`（新規） | `true` で学習と AI での利用を拒否 | `false`（サーバーに従う） |
| 個人 | `discoverable`（既存） | ディレクトリとおすすめへの掲載。クローラーの表明には使わない | `false` |

- あるページの実際の表明は、**サーバーの設定と、そのページの持ち主の設定のうち制限の強いほう**にする（IETF AI Preferences の「最も制限の強いものが勝つ」と同じ）。
  - 検索: `search_indexing = allow` かつ持ち主の `indexable = true` のときだけ索引を許す。
  - AI: サーバーの `ai_usage` と、持ち主の `ai_opt_out` から決める。個人はサーバーより広く許可できない。
- 管理画面とクライアントは、サーバーが拒否しているときに個人の設定が効かないことを示す（設定の画面に説明を出す）。

### robots.txt

`GET /robots.txt` を Go が動的に返す（`text/plain; charset=utf-8`、`Cache-Control: public, max-age=3600`、ETag）。
外枠と同じく `reservedPrefixes` に入れ、SPA の外枠は返さない。`account_domain` を分けている構成では、`host` 側で返す（ハンドル側は運営者の領分）。

```text
# AI の学習のクローラー（ai_usage が deny_training か deny のとき）
User-agent: GPTBot
User-agent: ClaudeBot
User-agent: CCBot
User-agent: Google-Extended
User-agent: Applebot-Extended
# …同梱の一覧の「学習」のもの
Disallow: /

# AI の検索・利用者の依頼のクローラー（ai_usage が deny のとき）
User-agent: OAI-SearchBot
User-agent: ChatGPT-User
User-agent: Claude-SearchBot
User-agent: Claude-User
User-agent: PerplexityBot
# …同梱の一覧の「AI での利用」のもの
Disallow: /

User-agent: *
Content-Usage: train-ai=n, ai-use=n
Content-Signal: search=yes, ai-train=no, ai-input=no
Disallow: /api/
Disallow: /oauth/
Disallow: /auth/
Disallow: /ap/
Disallow: /media/proxy/
Allow: /
```

- `User-agent: *` の群は、検索エンジンの拒否にも `Disallow: /` を使わない。取得を止めると `noindex` を読めず URL だけが索引に残ることがあり、
  リンクプレビューの取得者の一部（`Twitterbot` など）は robots.txt に従うので、ADR-0028 のプレビューが止まる。検索の拒否は `noindex` で表す。
- `Content-Usage` と `Content-Signal` の値はサーバーの設定から作る（`search_indexing = deny` なら `search=n` / `search=no`）。
  未知の行は RFC 9309 のパーサーが無視するので、草案の行を足しても既存のクローラーを壊さない。
- `/.well-known/` と `/nodeinfo` は拒否しない。統計の収集やサーバーの一覧（FR-402）を止めないため。
- AI クローラーの一覧は `ai-robots-txt/ai.robots.txt` の `robots.json`（MIT、AGPL-3.0 と両立）から作り、**用途（学習 / AI での利用）で分類した表をリポジトリに同梱する**。
  - 実行時には取得しない。リンクプレビューの取得者（`facebookexternalhit` `Twitterbot` `Discordbot` `Slackbot` など）と、汎用の検索エンジン（`Googlebot` `Bingbot` `Applebot`）は表に入れない。
  - 更新はスクリプトで差分を作って PR にする（月に 1 度程度、取り込む元の版を記録する）。
- 運営者が規則を書き足す手段は設けない（Open Questions）。

### ページの meta と HTTP ヘッダー

| 応答 | 検索を拒否するとき | AI を拒否するとき |
| --- | --- | --- |
| プロフィールと `public` の投稿の外枠（ADR-0028） | `<meta name="robots" content="noindex, noarchive">` と `X-Robots-Tag: noindex, noarchive` | `Content-Usage`（`train-ai=n` か `train-ai=n, ai-use=n`）、`tdm-reservation: 1`、meta と `X-Robots-Tag` に `noai, noimageai` |
| `unlisted` の投稿、見せない・存在しないページの 404 | 常に `noindex`（現状どおり） | サーバーの設定に従う |
| リモートのアカウントのページ・アプリの画面（`/home` `/settings` `/admin` など） | 常に `noindex` | サーバーの設定に従う |
| `/`（汎用の外枠） | サーバーの設定に従う | サーバーの設定に従う |
| `/media/`（このサーバーが返すとき） | `noindex` は付けない（画像検索はサーバーの `search_indexing` に従う） | サーバーの設定に従う。持ち主ごとの表明はしない |
| クライアント API・AP の JSON・`/.well-known/` | `X-Robots-Tag: noindex` | 付けない（robots.txt で足りる） |

- 外枠の ETag には、表明の値（サーバーの設定と持ち主の設定）を含める（ADR-0028 の「投稿の版」と同じ扱い）。
- `noindex` はリンクプレビューを止めない（ADR-0028 と [link-previews.md](../operations/link-previews.md) のとおり）。
- メディアは持ち主がキーに無く、CDN が直接返す構成もあるので、**持ち主ごとの AI の拒否はメディアに及ばない**。サーバー単位の表明は robots.txt で全体に効く。
  CDN でヘッダーを付ける方法をデプロイ文書に書く。

### `/.well-known/`

| パス | 返す条件 | 中身 |
| --- | --- | --- |
| `/.well-known/tdmrep.json` | `ai_usage` が `allow` 以外 | `[{"location": "/*", "tdm-reservation": 1}]`。`ai_usage = allow` のときは `tdm-reservation: 0` ではなく 404（何も表明しない） |
| `/.well-known/security.txt` | `security_contact` があるとき | `Contact`、`Expires`（応答の時刻から 30 日後）、`Preferred-Languages`（既定の言語と `en`）、`Canonical` |
| 既存の `webfinger` `host-meta` `host-meta.json` `nodeinfo` | 変えない | — |

- `ai.txt` と `/.well-known/` の AI 用の独自の文書は出さない。大手のクローラーが読むという文書が無く、同名の提案が並立しているため。

### 連合

- Actor の `indexable` と `discoverable` は、Mastodon の定義のまま送る（現状どおり）。
- リモートの Actor の `indexable` を正規化で読み、保存する（今は捨てている）。
- リモートのアカウントの投稿とプロフィールは、このサーバーで外部の検索エンジンに出さない（上表の `noindex`）。
  このサーバーの検索（将来の全文検索）は、`indexable = true` のリモートのアカウントの公開投稿だけを対象にする。
- 個人の `ai_opt_out` を AP の拡張で連合先に伝えることはしない（D1）。共通の語彙が無く、受け手が従う見込みも無い。
  連合先が返すページでの表明は連合先の設定に任せる。この限界を設定の画面の説明に書く。

### 推奨を覆す条件

- IETF AI Preferences が RFC になり、主要な AI の事業者が `Content-Usage` に従うと表明したら、User-agent の一覧の保守をやめて `Content-Usage` を軸にする。
- Fediverse で AI の拒否を表す共通の AP の語彙（FEP など）が広まったら、D2 に切り替える。

## Consequences（影響）

- 良い影響
  - `/robots.txt` が正しく返り、AI クローラーの拒否と検索の許否がサーバーと個人の両方で明示される。
  - 既存の `indexable` がページと検索に効き、Mastodon の定義と同じ意味で送受信される。
  - リンクプレビューと連合の取得は止まらない。
- 悪い影響・トレードオフ
  - robots.txt も meta も従うかどうかはクローラー次第で、技術的な遮断ではない。遮断はこの ADR の対象外（WAF・レート制限）。
  - 同梱の AI クローラーの一覧は古くなる。更新の PR を定期的に作る必要がある。
  - 既定の `ai_usage = deny` は、ChatGPT などで利用者がこのサーバーのページを読ませる操作も拒む表明になる。
  - 既定の `indexable = false` では、サーバーが検索を許していても、利用者が選ぶまで検索エンジンに載らない。
  - 持ち主ごとの AI の拒否はメディアと連合先のページに及ばない。
  - `noai` / `noimageai`・`Content-Usage`・`Content-Signal`・TDMRep は読み手が少ないか未確認で、効果は限られる。
- 後続で必要になる作業（Issue 化する）
  - #656 の実装: robots.txt、ページの meta とヘッダー、`/.well-known/tdmrep.json` と `security.txt`、インスタンス設定 3 つと個人の `ai_opt_out`（スキーマ・API・管理画面・設定の画面・i18n）
  - リモートの Actor の `indexable` の取り込み（boy scout で #656 に含めてよい）
  - AI クローラーの一覧の生成スクリプトと、定期更新の手順
  - [link-previews.md](../operations/link-previews.md) とデプロイ文書（CDN でメディアにヘッダーを付ける方法）の更新

## Pros and Cons of the Options（詳細比較・任意）

### A. 表明の手段

- **A1**: 読まれる手段（robots.txt の User-agent、`noindex`）で実効を取り、標準化の途中の手段（`Content-Usage`、TDMRep）は数行で足せるので先に出しておける。
  表明の手段が多く、テストの対象が増える。
- A2: 実装が最小。IETF AI Preferences が RFC になったときに追いかける必要があり、EU の TDM の留保のような機械可読の表明を求める場面で弱い。
- A3: 保守が最も楽で、一覧が最も広い。`facebookexternalhit` などのプレビューの取得者や検索の `Applebot` まで止めるので、ADR-0028 のリンクプレビューと矛盾する。

### B. 個人とサーバーの関係

- **B1**: 個人の拒否がサーバーの設定で消えない。サーバーが拒否しているときに個人の許可が効かないことを利用者に説明する必要がある。
- B2: 単純だが、決定者の求める個人単位の表明ができない。
- B3: 個人が広く許可できるが、サーバー全体を拒否にしたい運営者の意図（robots.txt はサーバー全体にしか効かない）と矛盾する。

### C. 個人の検索の設定

- **C1**: 設定が 1 つで済み、連合での意味（公開投稿の索引）と外部の検索エンジンの扱いがそろう。Mastodon の利用者が想定する「検索エンジンには出るが全文検索には出ない」状態は作れない。
- C2: Mastodon と同じ細かさだが、設定が増え、`indexable = true` かつ `noindex` のような分かりにくい組み合わせが生まれる。

### D. 個人の AI の拒否の伝え方

- **D1**: 作るものが少ない。連合先に表明が届かない。
- D2: 連合先に届く可能性があるが、受け手の実装が無い独自の語彙で、ADR-0006 の互換拡張の範囲を広げる。

## Open Questions / 要検証

- 既定値: `ai_usage` を `deny`（AI の検索・利用者の依頼も拒否）にするか `deny_training` にするか。`indexable` を opt-in（`false`）のままにするか、サーバー設定で既定を選べるようにするか。
- 運営者が robots.txt に独自の規則を足す手段（設定ファイルの追記）を設けるか。
- IETF AI Preferences と Content Signals に対応すると表明した大手のクローラーがあるか（要検証）。
- 日本の著作権法 30 条の 4 と EU DSM 指令 4 条に対し、この表明をどう説明するか（利用規約・サーバーの情報ページの文言）。
- Tauri のアプリ（`/.well-known/assetlinks.json` と `apple-app-site-association`）と `/.well-known/change-password` は、それぞれの機能の Issue で扱う。
