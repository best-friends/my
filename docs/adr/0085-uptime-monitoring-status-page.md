# ADR-0085: 外形監視とステータスページとの連携

- Status: Accepted
- Date: 2026-10-23
- Deciders: @rosylilly
- Issue: #1024

## Context（背景・課題）

v0.x を公開して実運用を始めると、利用者が「サーバーが落ちているのか、自分の回線だけなのか」を確かめる手段と、運営者がサーバーを外から見張る手段が要る。
運営者は UptimeRobot・Better Stack・Gatus・Uptime Kuma・Upptime のような外形監視と、Atlassian Statuspage などのステータスページを使う。

いまあるのは FR-802 の 2 つのエンドポイントだけで、どちらもプラットフォームの検査（ロードバランサ・Kubernetes のプローブ）向けに作られている（`internal/platform/http.go`）。

- `/healthz`（liveness）: プロセスが HTTP に答えれば `200 {"status":"ok"}`。何にも触れない
- `/readyz`（readiness）: 呼ばれるたびに DB に届くこととスキーマの適用を確かめ、`200 {"status":"ok"}` か、`503` と RFC 9457 の problem 文書（ADR-0016）を返す。結果はキャッシュしない

これを外形監視にそのまま向けると、次の問題がある。

- **費用**: `/readyz` は毎回 DB に触れる。scale to zero の DB（Neon Free は 5 分無通信で停止し、月 100 CU 時間。[deployment-targets.md](../architecture/deployment-targets.md)）を 1〜5 分ごとの監視が起こし続けると、
  NFR-COST-1 の「アイドル時ほぼ $0」が崩れる。誰でも叩けるので、第三者が DB を起こし続けることもできる
- **メンテナンス中の意味の衝突**: メンテナンスモード（[ADR-0084](0084-maintenance-mode.md)）では、ロードバランサがインスタンスを外さないよう `/healthz`・`/readyz` は
  `200 {"status":"maintenance"}` を返す。状態コードだけを見る外形監視は、メンテナンス中を「正常」と判定してしまう
- **掲出先がない**: ステータスページの URL の設定がなく、Web クライアント・情報ページ・API に出す場所もない。機能要件にも行がない

v0 の公開（M5）の前に、外形監視が叩く先と、その応答の形・費用の上限・メンテナンス中の応答・ステータスページの URL の掲出を決める。
関連: FR-802、NFR-COST-1、NFR-OPS-3、[ADR-0007](0007-client-api.md)、[ADR-0016](0016-client-api-style.md)、[ADR-0019](0019-security-baseline.md)、[ADR-0056](0056-system-observability.md) §3、[ADR-0084](0084-maintenance-mode.md)。

## Decision Drivers（判断基準）

1. **アイドル時の費用を増やさない**（NFR-COST-1）: 監視の頻度や第三者の連打で DB を起こす回数に上限があること
2. **プラットフォームのプローブを壊さない**: `/healthz`・`/readyz` の意味（ADR-0056 §3、ADR-0084 のメンテナンス中の振る舞い）を外形監視の都合で変えない
3. **漏らさない**（ADR-0019）: 内部のホスト名・接続先・版・エラーの文言・どのアダプタを使っているかを未認証の応答に出さない
4. **主な外形監視の無料の範囲で使える**: 状態コード（2xx）か本文のキーワード・JSON の値で判定でき、ヘッダの追加や認証を要しない
5. **障害時に届く**: ステータスページへのリンクは、サーバーが答えないときにこそ要る
6. 実装と運用が小さいこと（v0）

## Considered Options（選択肢）

### エンドポイント

1. **A. `/readyz` をそのまま外形監視に使う**: 文書で案内するだけ。メンテナンス中の本文を足す
2. **B. 外形監視向けの公開の状態エンドポイントを足す**: `GET /api/v1/instance/health`。集約した状態だけを返し、結果をプロセスの中でキャッシュする。プローブ（`/healthz`・`/readyz`）とは分ける
3. **C. B に加えて部品ごとの詳しい状態を、運営者のトークン付きで返す**: DB・キュー・KV・Blob・配送の滞留・失敗したジョブの数

### ステータスページの URL の置き場所

- **a. インスタンス設定 `status_page_url`**（DB。管理画面・CLI・API から即時に変える。`security_contact` と同じ扱い）
- **b. 設定 `status_page_url`（環境変数 `MY_STATUS_PAGE_URL`・設定ファイル）**: DB に触れずに読めるので、DB の障害中やメンテナンスモード（ADR-0084 はアダプタを開かず設定だけを読む）でも出せる

### ステータスページの状態の取り込み

- **i. 取り込まない**（v0）: リンクを出すだけ
- **ii. 取り込む**: ステータスページの API（Statuspage の `summary.json` など）を `kernel.HTTPClient`（ADR-0019 §1）で定期に取得し、障害・メンテナンスの予定を Web に帯で出す

## Decision（決定）

エンドポイントは B、URL の置き場所は b、取り込みは i とする。

### 1. 状態エンドポイント `GET /api/v1/instance/health`

- 認証なし。クライアント API の一部として OpenAPI に載せる（ADR-0016）。Web クライアントも、通信の失敗が「サーバーの障害」か「自分の回線」かを見分けるのに使える
- 判定は readiness と同じ `App.Ready`（DB に届き、スキーマが適用済み）。`/readyz` と同じ 3 秒の上限を持つ
- **結果をプロセスの中で 60 秒キャッシュ** し（60 秒は固定で、設定にしない）、同時の要求は 1 回の検査にまとめる（`singleflight`）。1 プロセスが DB に触れるのは監視の頻度や要求の数によらず最大で 1 分に 1 回。
  KV を使うレート制限（`kernel.RateLimiter`）は、KV が DB のこともあり DB を起こしうるので使わない。キャッシュがレート制限の代わりになる
- 応答（`Content-Type: application/json`、`Cache-Control: no-store`）:

  | 状態 | HTTP | 本文 |
  | --- | --- | --- |
  | 正常 | `200` | `{"status":"ok","checked_at":"<RFC 3339>"}` |
  | 障害（DB に届かない・スキーマ未適用） | `503` | `{"status":"unavailable","checked_at":"<RFC 3339>"}` |
  | メンテナンス中（ADR-0084） | `503` と `Retry-After` | `{"status":"maintenance"}`。§4 |

- 本文はこの 2〜3 個のキーだけで、値は固定の語。理由・エラーの文言・ホスト名・版・アダプタの名前を出さない（ADR-0019）。詳細はいまのとおりログと失敗の数の指標（ADR-0056 §3）に残す。
  `status` は機械向けの語で、ローカライズしない
- 部品ごとの状態（キュー・KV・Blob・配送の滞留・失敗したジョブ）は v0 の公開の応答に入れない。滞留や失敗は「落ちている」ではなく運営者が見る量なので、
  管理画面（NFR-OPS-3）とサービスの指標（ADR-0055）で扱う

### 2. プローブとの分担（`/healthz`・`/readyz` の意味は FR-802 と ADR-0084 のまま）

| | `/healthz` | `/readyz` | `/api/v1/instance/health` |
| --- | --- | --- | --- |
| 使う者 | プラットフォームの liveness | ロードバランサ・Kubernetes の readiness | 外形監視・ステータスページ・Web クライアント |
| DB に触れるか | 触れない | 毎回（同時の要求は `singleflight` でまとめる） | 最大 1 分に 1 回 |
| メンテナンス中（ADR-0084） | `200 {"status":"maintenance"}` | `200 {"status":"maintenance"}`（インスタンスを外さない） | `503`・`Retry-After`・`{"status":"maintenance"}` |

- `/readyz` にも `singleflight` を足し、連打されても同時の検査は 1 本にする（意味は変えない）
- 運用文書では、外形監視に `/readyz` を向けないよう案内する。`/healthz` を監視するときは、メンテナンス中も 200 になるので、本文に `"ok"` があることを判定に使う

### 3. 費用の目安と監視の頻度

- scale to zero の DB で、監視の間隔が DB の停止までの時間（Neon は 5 分）より短いと、DB は止まらなくなる。連合サーバーは外からの配送で DB が起きている時間が長いので（[deployment-targets.md](../architecture/deployment-targets.md)）、増える費用は無通信の時間帯の分に限られる
- アイドル時の費用を優先する運営者には、`/healthz`（DB に触れない）を監視し、DB を含む検査は間隔を長くする構成を運用文書で示す。サーバーレスでは、監視の要求そのものの費用は小さい。5 分ごとで月約 8,640 回、1 分ごとで約 43,200 回で、
  Lambda の無料枠（月 100 万リクエスト・40 万 GB 秒、超えると 100 万リクエストあたり $0.20、時間は 1 ms 単位。2026-10-23 に [AWS Lambda pricing](https://aws.amazon.com/lambda/pricing/) で確認）に収まる。
  Cloud Run のリクエストごとの課金も、無料枠（月 200 万リクエスト・18 万 vCPU 秒・36 万 GiB 秒。時間は 100 ms 単位に切り上げ）に収まる
  （2026-10-23 に [Cloud Run pricing](https://cloud.google.com/run/pricing) で確認）

### 4. メンテナンスモードとの関係

- メンテナンスモード（[ADR-0084](0084-maintenance-mode.md)）の間、`/api/v1/instance/health` は DB に触れずに `503`・`Retry-After`（値は ADR-0084 に従う）・`{"status":"maintenance"}` を返す。
  ADR-0084 の応答の表で、client API のほかのパス（maintenance の Problem）とは別の行になり、形はこの ADR が持つ。本文の形は正常・障害・メンテナンス中で同じ `{"status": ...}`
- 多くの外形監視は 2xx 以外を「停止」とみなすので、`503` だけではメンテナンスと障害を区別できない。区別は本文のキーワード（`"maintenance"`）か JSON の値と、各サービスのメンテナンスの時間帯の設定で行うよう運用文書に書く。
  UptimeRobot の Free にはメンテナンスの時間帯がないので（§5）、メンテナンス中は監視を手で止めるか、停止の通知を受け入れる

### 5. 主な外形監視との対応表

| サービス | 推奨の監視の形 | 判定 | 確認 |
| --- | --- | --- | --- |
| UptimeRobot | キーワードの監視（Free で使える。間隔 5 分・50 監視） | 本文に `"status":"ok"` があれば正常 | 2026-10-23 に [料金ページ](https://uptimerobot.com/pricing.md) で確認。状態コードの扱いとヘッダを変える「Custom headers & statuses」とメンテナンスの時間帯は Free になく、Solo（有料、間隔 60 秒）から |
| Better Stack | HTTP の状態コードの監視（2xx を正常とみなす）か、キーワードの監視（本文にキーワードが無ければ通知。大文字と小文字を区別しない） | `200`、または本文に `"status":"ok"` | 2026-10-23 に [Uptime monitor](https://betterstack.com/docs/uptime/uptime-monitor/)・[Keyword monitor](https://betterstack.com/docs/uptime/keyword-monitor/)・[料金](https://betterstack.com/pricing)（Free は監視とハートビートで 10・ステータスページ 1）で確認 |
| Gatus | 条件 `[STATUS] == 200` と `[BODY].status == ok`。メンテナンスの時間帯を設定できる | 同左 | [README](https://github.com/TwiN/gatus)、2026-10-23 確認（既定の間隔 60 秒） |
| Uptime Kuma | HTTP(s) - Keyword（本文に `"status":"ok"`）か JSON Query（`status` が `ok`）。受け付ける状態コードの既定は `200-299` | 同左 | 2026-10-23 に [monitor.js](https://github.com/louislam/uptime-kuma/blob/0c287ac4b3a6723c56ab7d8d731017e58deff533/server/model/monitor.js#L478-L720)・[EditMonitor.vue](https://github.com/louislam/uptime-kuma/blob/0c287ac4b3a6723c56ab7d8d731017e58deff533/src/pages/EditMonitor.vue#L3349) で確認 |
| Upptime | GitHub Actions からの HTTP の監視。既定で 2xx と 3xx を正常とみなし、`__dangerous__body_down_if_text_missing` で本文の語を条件にできる | `200` と本文に `"status":"ok"` | 2026-10-23 に [Configuration](https://upptime.js.org/docs/configuration/) で確認（間隔は既定・最短とも 5 分） |
| Atlassian Statuspage | 監視ではなく掲出先。上の監視の通知や API から部品の状態を変える | — | 部品の状態は Operational・Under maintenance・Degraded performance・Partial outage・Major outage の 5 つ（2026-10-23 に [What is a component?](https://support.atlassian.com/statuspage/docs/what-is-a-component/) で確認）。API の値は `operational`・`under_maintenance`・`degraded_performance`・`partial_outage`・`major_outage`（2026-10-23 に [Statuspage API](https://developer.statuspage.io/) の部品の `status` の enum で確認）。監視の通知から部品の状態を自動で変える連携は要検証（サービスごとの連携の設定が要る） |

### 6. ステータスページの URL（設定 `status_page_url`）

- 設定 `status_page_url`（環境変数 `MY_STATUS_PAGE_URL`・設定ファイル。ADR-0010）: `https:` の絶対 URL のみ（空 = 未設定）。`my config check` で検証する。変更はプロセスの再起動で反映する
- 設定に置くのは、**DB に届かないときにこそ要る値** だから。メンテナンスモード（ADR-0084）はアダプタを開かず設定だけを読み、この同じキーをメンテナンスのページ・Problem の `status_page_url`・
  書き出し（`my maintenance export`）に使う。メンテナンス専用のキーは持たない
- [instance-settings.md](../operations/instance-settings.md) の「運営中に変える方針の値はインスタンス設定」という目安からは外れる。ステータスページの URL はめったに変わらず、
  デプロイの構成（どこで運用の状態を告げるか）に近いので、再起動での反映で足りると判断する。同じキーをインスタンス設定に置かない（両方に同じキーを持たない規則）
- 出す場所:
  - `GET /api/v1/instance` の `status_page_url`（未設定なら出さない）
  - サーバーの情報のページ（`/about`）と、サーバーが描画するサインインの画面
  - Web クライアント: 設定の画面のヘルプの区画と、エラー・オフラインの表示（G16・G17）。サーバーが答えないときは `GET /instance` を読めないので、
    **Web クライアントは最後に読んだ `status_page_url` をアプリが残し（Service Worker は API に答えないので、オフラインのホームのタイムラインと同じくアプリが残す。web-client §11、ADR-0045）、そこから URL を出す**
  - メンテナンスモードの応答と書き出したメンテナンスのページ（[ADR-0084](0084-maintenance-mode.md)）
  - DB の障害で返す HTML のエラーページ
- NodeInfo の `metadata` には v0 では足さない。Mastodon の NodeInfo の `metadata` にはステータスページのキーがなく（[node_info/serializer.rb](https://github.com/mastodon/mastodon/blob/131aabcaf1f1f13e0615f8ad2748ab05f966e2c9/app/serializers/node_info/serializer.rb#L40-L47)、2026-10-23 確認）、
  Misskey のリポジトリにも `statusPage`・`status_page` は見当たらない（GitHub のコード検索、2026-10-23）。Akkoma の `metadata` にもない（[nodeinfo.ex](https://akkoma.dev/AkkomaGang/akkoma/src/commit/a69f0b9e328dd6c12bf4a308892041035408d03a/lib/pleroma/web/nodeinfo/nodeinfo.ex#L52-L92)、2026-10-23 確認）。対応するキーが分かったら足す
- Mastodon は同じ値を管理画面の設定 `status_page_url` に持ち、インスタンスの API の `configuration.urls.status` と Web のフッターに出す
  （[instance_serializer.rb](https://github.com/mastodon/mastodon/blob/131aabcaf1f1f13e0615f8ad2748ab05f966e2c9/app/serializers/rest/instance_serializer.rb#L59-L67)、2026-10-23 確認）。出す場所はこれと揃う

### 7. ステータスページの状態の取り込み

v0 では取り込まない（i）。サービスごとに API の形が違い、取得には `kernel.HTTPClient`・定期のジョブ・キャッシュ・取得の失敗の扱いが要る。
告知はサーバー全体へのお知らせ（FR-726）で代えられる。

### 8. 機能要件

- **FR-807**（運用、M）: 外形監視向けの状態エンドポイント: 認証なしで、サーバーの状態（正常・障害・メンテナンス中）を HTTP の状態コードと JSON で返す。DB の検査の結果をキャッシュし、監視の頻度によらず DB に触れる回数に上限を設ける。内部の情報（ホスト名・版・エラーの文言）を出さない（ADR-0085）
- **FR-810**（運用、S）: ステータスページの URL: 設定で `https:` の URL を与えると、DB に届かないときも含めて、クライアント API・サーバーの情報ページ・サインインの画面・Web クライアントのエラーとオフラインの表示・メンテナンスのページに出す（ADR-0085）

### この決定を見直す条件

- 主な外形監視が `503` と本文から「メンテナンス」を自動で識別できる形（例: 特定の状態コードやヘッダ）を持つと分かれば、§4 の応答をそれに合わせる
- 運営者が部品ごとの状態を外から監視する需要が強ければ C（トークン付きの詳しい応答）を足す。その場合も公開の応答は §1 のまま
- ステータスページをサーバー自身が持つ（自前のステータスページ）方針になれば、§6・§7 を見直す

## Consequences（影響）

- 良い影響
  - 監視の頻度や第三者の連打があっても、1 プロセスが DB に触れるのは最大 1 分に 1 回で、費用の上限が読める
  - プローブ（ロードバランサ）と外形監視の意味を分けたので、メンテナンス中に LB がインスタンスを外さずに、監視には「メンテナンス中」と伝えられる
  - 主な外形監視の無料の範囲（状態コードかキーワード）で設定できる
  - サーバーが答えないときも、Web クライアントは残した値からステータスページへ案内できる。設定に置くので、DB の障害中とメンテナンス中もサーバー自身が URL を出せる
- 悪い影響・トレードオフ
  - 状態は最大 60 秒古い。障害の検知と復旧の検知が最大 1 分遅れる
  - プロセスごとのキャッシュなので、複数台では台ごとに結果が違いうる（ロードバランサが外した台には届かないので実害は小さい）
  - 集約した状態だけなので、キューの詰まり・配送の滞留・Blob の障害は外形監視で分からない（管理画面と指標で見る）
  - `503` を「停止」とみなす監視では、メンテナンス中も停止として通知される。運営者がメンテナンスの時間帯を監視の側でも設定する手間がある
  - 初めて開いた Web クライアント（残した値がない）は、サーバーが答えないときにステータスページを出せない
  - URL の変更に再デプロイが要り、管理画面からは変えられない
- 後続で必要になる作業（サーバー・client API・Web・文書の issue に分ける）
  - サーバー: `GET /api/v1/instance/health`（キャッシュ・`singleflight`・OpenAPI）、`/readyz` の `singleflight`、メンテナンス中の応答（ADR-0084 の実装と合わせる）
  - 設定 `status_page_url`（`docs/reference/configuration.md` の再生成）と `GET /api/v1/instance`・サインインの画面・`/about`・DB の障害時のエラーページ。ADR-0084 の実装はこのキーを読む
  - Web クライアント: 情報ページ、設定のヘルプ、エラー・オフラインの表示でのリンク（キャッシュからの表示を含む）
  - 文書: `docs/operations/monitoring.md`（§5 の表のサービスでの設定例、`/healthz` と `/api/v1/instance/health` の使い分け、scale to zero の DB での間隔の目安、メンテナンスの時間帯の設定）、
    機能要件への 2 行の追加

## Pros and Cons of the Options（詳細比較・任意）

### A. `/readyz` をそのまま使う

- 良い: 何も足さない。どの監視でも状態コードで判定できる
- 悪い: 毎回 DB に触れ、誰でも DB を起こし続けられる（判断基準 1）。メンテナンス中に「LB には受け付け可、監視にはメンテナンス中」を 1 つの応答で返せない（判断基準 2）。
  キャッシュを足すと、プローブとしての反応が遅れる

### B. 公開の状態エンドポイントを足す（採用）

- 良い: 費用の上限・プローブとの分離・漏えいの防止を同時に満たす。Web クライアントからも使える
- 悪い: エンドポイントが 1 つ増える。結果が最大 60 秒古い

### C. 部品ごとの詳しい状態（トークン付き）

- 良い: 運営者は外形監視だけで、どの部品が壊れたかまで分かる
- 悪い: トークンの発行・失効・保存が要り、外形監視のヘッダの追加は、UptimeRobot では有料の Solo からの機能（Free にない。2026-10-23 に料金ページで確認）。部品ごとの検査はそれぞれのミドルウェアを起こし、配送の滞留・失敗の数は DB の集計で費用が増える。
  同じ情報は管理画面と指標にある

### a. インスタンス設定と b. 設定（環境変数）

- a の良い点: 再起動なしに管理画面から変えられ、置き場所の規則と `security_contact` の前例に合う。Mastodon も管理画面の設定（DB）に置く
- b の良い点: DB に触れずに読めるので、DB の障害中やメンテナンスモードでも必ず出せる（判断基準 5）
- b を採るのは、判断基準 5（障害時に届く）を満たすのが b だけだから。a ではメンテナンスモードがこの値を読めず、メンテナンス専用の設定キーを別に要する（同じ URL を 2 か所に持つ）。
  a の利点（再起動なしの変更）は、めったに変わらない値では小さい

### i. 取り込まない と ii. 取り込む

- ii の良い点: 予定されたメンテナンスや障害を、利用者がステータスページを開かずに Web で知れる
- ii の悪い点: サービスごとの API の違い、外向きの取得（SSRF ガード）、定期のジョブ・キャッシュ・失敗の扱いが増える。FR-726 のお知らせで代えられる

## Open Questions / 要検証

- Better Stack の Free の確認の間隔とメンテナンスの時間帯（公式の料金ページに Free の範囲の記載がなく、第三者の記事だけが間隔 3 分とする）
- Atlassian Statuspage で、外形監視の通知から部品の状態を自動で変える連携の方法（サービスごとの実際のアカウントで確かめる）
