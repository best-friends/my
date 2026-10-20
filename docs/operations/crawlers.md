# 検索エンジンと AI クローラー

運営者向けの説明と確認の手順です。要件は FR-412（個人）と FR-725（サーバー）（[機能要件](../requirements/01-functional.md)）、
設計は [ADR-0057](../adr/0057-crawler-opt-out.md) です。

robots.txt も meta タグも、従うかどうかはクローラー次第の表明で、技術的な遮断ではありません。遮断が要るときは WAF やレート制限を使います。

## 設定

| 単位 | 設定 | 変える場所 | 未設定のとき |
| --- | --- | --- | --- |
| サーバー | `search_indexing`（`allow` / `deny`） | 管理画面「サーバーの設定」の「検索エンジンと AI」、`my instance set`、`PATCH /api/v1/admin/settings`（[インスタンス設定](instance-settings.md)） | `deny` |
| サーバー | `ai_usage`（`allow` / `deny_training` / `deny`） | 同上 | `deny` |
| サーバー | `indexable_default`（新規アカウントの `indexable`） | 同上 | `false` |
| サーバー | `security_contact`（脆弱性の連絡先） | 同上 | 空（`security.txt` を返さない） |
| 個人 | `indexable`（検索に載せる） | 設定の「プロフィール」、`PATCH /api/v1/accounts/update_credentials` | 登録時の `indexable_default` |
| 個人 | `ai_opt_out`（AI の学習と利用を拒否する） | 同上 | `false` |

- ページの表明は、サーバーの設定とページの持ち主の設定のうち制限の強いほうです。個人はサーバーより広く許可できません。
- 検索エンジンに載るのは、`search_indexing = allow` のサーバーで `indexable = true` のアカウントのプロフィールと `public` の投稿だけです。
  検索に載せたいサーバーは `search_indexing` と `indexable_default` の両方を変えます。`indexable_default` を変える前に登録した利用者は、自分で `indexable` を選ぶまで載りません。
- 設定を変えると、`/robots.txt`・ページ・メディアの応答に、再起動なしで反映されます。

## サーバーが返すもの

| 応答 | 検索を拒否するとき | AI を拒否するとき |
| --- | --- | --- |
| プロフィールと `public` の投稿のページ | `<meta name="robots">` と `X-Robots-Tag` に `noindex, noarchive` | `noai, noimageai`、`Content-Usage: train-ai=n`（`deny` では `train-ai=n, ai-use=n`）、`tdm-reservation: 1`（ヘッダーと meta） |
| `unlisted` の投稿、見せない・存在しないページの 404、リモートのアカウントのページ、アプリの画面（`/home` `/settings` など） | 設定によらず `noindex, noarchive` | サーバーの設定に従う |
| `/`、`/about` | サーバーの設定に従う | サーバーの設定に従う |
| `/media/`（このサーバーが返すとき） | サーバーの設定に従う。持ち主ごとの表明はしない | サーバーの設定に従う。持ち主ごとの表明はしない |
| クライアント API、ActivityPub の文書、`/.well-known/`、`/nodeinfo/`、サインインと OAuth の画面 | `X-Robots-Tag: noindex` | 付けない（robots.txt で足りる） |

`noindex` はリンクプレビューを止めません（[リンクプレビュー](link-previews.md)）。

### `/robots.txt`

`GET /robots.txt` は、サーバーの設定と同梱の AI クローラーの一覧から作ります（`text/plain; charset=utf-8`、`Cache-Control: public, max-age=3600`、ETag）。
運営者が規則を書き足す手段はありません。

- `ai_usage` が `deny_training` か `deny` なら、学習のクローラー（`GPTBot` `ClaudeBot` `CCBot` `Google-Extended` `Applebot-Extended` など）に `Disallow: /`。
- `ai_usage` が `deny` なら、AI 検索・アシスタント・エージェントのクローラー（`OAI-SearchBot` `ChatGPT-User` `Claude-User` `PerplexityBot` など）にも `Disallow: /`。
- すべてのクローラー（`User-agent: *`）には、`/api/` `/oauth/` `/auth/` `/ap/` `/media/proxy/` だけを拒否し、`Content-Usage`（IETF AI Preferences の草案）と
  `Content-Signal`（Content Signals）の行で設定を表します。検索を拒否しても `Disallow: /` にはしません。取得を止めると `noindex` を読めず、
  robots.txt に従うリンクプレビューの取得者も止まるからです。
- `/.well-known/` と `/nodeinfo` は拒否しません。サーバーの一覧や統計（FR-402）を止めないためです。
- `account_domain` をホストと分けている構成では、`host` 側で返します。ハンドルのドメイン側の robots.txt は運営者の領分です。

### `/.well-known/`

| パス | 返す条件 | 中身 |
| --- | --- | --- |
| `/.well-known/tdmrep.json` | `ai_usage` が `allow` 以外 | `[{"location": "/*", "tdm-reservation": 1}]`（TDM Reservation Protocol）。`allow` では 404 |
| `/.well-known/security.txt` | `security_contact` があるとき | `Contact`、`Expires`（応答の時刻の 30 日後）、`Preferred-Languages`（`default_locale` と `en`）、`Canonical`（RFC 9116） |

### 情報ページ

インスタンスの情報ページ（`/about`）は、`GET /api/v1/instance` の `search_indexing` と `ai_usage` から、検索と AI の許否と、その表明の意味を出します。
意味の説明には、日本の著作権法 30 条の 4（こうした表明は法的な拒否として扱われない場合がある）と、EU DSM 指令 4 条（機械可読な TDM の留保）に対する位置づけを含みます。
法的な効果を保証するものではありません。

## 限界

- 個人の `ai_opt_out` は、このサーバーが返すページでだけ表明します。メディアは持ち主がキーに無く、CDN が直接返す構成もあるので、メディアにはサーバー単位の表明だけが付きます。
  連合先が返すページでの表明は連合先の設定に任せます（AP の独自の拡張は送りません）。
- `ai_usage = deny` は、ChatGPT などで利用者がこのサーバーのページを読ませる操作も拒む表明になります。
- リモートのアカウントの `indexable`（Mastodon の `toot:indexable`）は受け取って保存します。リモートのアカウントのページは、このサーバーでは常に `noindex` です。

## オブジェクトストレージと CDN でメディアを返す構成

メディアをオブジェクトストレージや CDN が直接返す構成では、`/media/` の応答はこのサーバーを通らず、上表のヘッダーが付きません。
サーバー単位の表明は `/robots.txt` で全体に効きますが、ヘッダーも付けるときは、CDN の応答ヘッダーの規則で `/media/*` に次を足します（`ai_usage = deny` の例）。

```text
X-Robots-Tag: noindex, noarchive, noai, noimageai
Content-Usage: train-ai=n, ai-use=n
tdm-reservation: 1
```

設定を変えたときは、CDN の規則も合わせて変えます。`/media/proxy/` はこのサーバーが返すので不要です。

## AI クローラーの一覧の更新

一覧は [ai-robots-txt/ai.robots.txt](https://github.com/ai-robots-txt/ai.robots.txt)（MIT）の `robots.json` から作り、
`internal/interface/web/ai_crawlers.gen.go` に同梱します。実行時には取得しません。月に 1 度程度、次の手順で更新の PR を作ります。

1. `Makefile` の `AI_ROBOTS_COMMIT` を新しいコミットに、`AI_ROBOTS_SHA256` をそのコミットの `robots.json` の SHA-256 に変えます。
2. `make ai-crawlers` を実行します。
3. 差分を確かめます。分類（学習 / AI での利用）は `robots.json` の `function` の文から決めるので、新しく入ったクローラーの分類が誤っていれば、
   `scripts/ai-crawlers/main.go` の `overrides` で直します。リンクプレビューの取得者と汎用の検索エンジンは `excluded` で一覧から外します。

## 確認の手順

```sh
curl -sS https://<host>/robots.txt
curl -sS -o /dev/null -D - -H 'Accept: text/html' https://<host>/@alice | grep -iE 'x-robots-tag|content-usage|tdm-reservation'
curl -sS https://<host>/.well-known/tdmrep.json
curl -sS https://<host>/.well-known/security.txt
```

- 設定の無いサーバーでは、`/robots.txt` に `User-agent: GPTBot` と `Content-Usage: search=n, train-ai=n, ai-use=n` があり、
  プロフィールに `X-Robots-Tag: noindex, noarchive, noai, noimageai` が付くこと。
- `search_indexing = allow`・`ai_usage = allow` にし、`indexable` を選んだアカウントのプロフィールに `X-Robots-Tag` が付かず、`/.well-known/tdmrep.json` が 404 になること。
