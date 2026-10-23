# ADR-0084: メンテナンスモードとメンテナンス HTML の書き出し

- Status: Accepted
- Date: 2026-10-23
- Deciders: @rosylilly
- Issue: #1023

## Context（背景・課題）

DB の作り直し（v0 はスキーマを作り直す。[ADR-0012](0012-v0-breaking-change-policy.md)、公開後の扱いは [ADR-0087](0087-schema-changes-after-launch.md)）、PostgreSQL の版の更新、
KV（Valkey）やキューの入れ替えのあいだ、サーバーは DB や KV に届かず、リクエストは 500 や `/readyz` の 503 になる。
このときロードバランサはインスタンスを外して自前のエラーを返し、利用者には理由が分からず、連合先は配送の失敗を数える。

次の 2 つの機能が要る。要件の表（FR-8xx）には行がない。

1. **メンテナンスモード**: `my serve` を、DB・KV・キュー・Blob などのミドルウェアに一切触れない状態で動かし、すべてのリクエストに「メンテナンス中」の応答を返す。
2. **メンテナンス HTML の書き出し**: `my serve` そのものを動かせないとき（イメージの入れ替え中、Cloud Run のリビジョンが起動しないときなど）に、
   ロードバランサ・CDN・リバースプロキシに置ける静的な HTML を書き出す。

関わる要件と決定: FR-801（CLI）、FR-802（`/healthz`・`/readyz`）、[ADR-0007](0007-client-api.md) と [ADR-0016](0016-client-api-style.md)（RFC 9457 の Problem）、
[ADR-0009](0009-internationalization.md)（i18n）、[ADR-0010](0010-configuration-and-packaging.md)（設定の出所）、[ADR-0019](0019-security-baseline.md) §9（CSP）、
[ADR-0059](0059-design-tokens-and-instance-theme.md)（サーバー単位のテーマ）。
外形監視とステータスページは [ADR-0085](0085-uptime-monitoring-status-page.md)、クローズ後の静的なページは [ADR-0088](0088-service-shutdown-modes.md) が扱い、どちらもこの ADR の応答と書き出しを使う。
v0 の公開（M5）の前に、運用の手順の前提として決める。

## Decision Drivers（判断基準）

1. **ミドルウェアに触れない**: 切り替えの手段も、応答に使う値も、DB・KV・キュー・Blob・Mailer・外への HTTP に依存しない。DB の設定には置けない。
2. **連合先に死んだサーバーと見なされない**: Inbox・Actor・WebFinger・NodeInfo が「一時的に使えない」と答え、連合先が配送を再試行する。
3. **すべてのターゲットで同じ手順**: Cloud Run・Lambda・Docker compose・microk8s で、同じ操作で切り替えられる（NFR-DIST-2）。インスタンスが複数でも状態が揃う。
4. **利用者と監視に理由が分かる**: Web・Web クライアント・Tauri アプリ・外形監視が「メンテナンス中」と区別できる。i18n（ADR-0009）に従う。
5. **作る量と保守の量が小さい**: 通常の起動経路（`platform.NewApp`）に分岐を散らさない。メンテナンスモードと書き出しが同じテンプレートを使う。

## Considered Options（選択肢）

### 切り替えの方法

1. **A1: 設定キー `maintenance`（環境変数・TOML・`my serve -maintenance`）で、再起動して切り替える**
2. **A2: 起動中に切り替える**（シグナル・ファイルの監視・管理 API）
3. **A3: `my serve` では持たず、前段（LB・CDN・nginx）の静的な応答だけで済ませる**

### 応答に使うサーバーの名前・告知文・テーマの出所

1. **B1: 設定キーに持つ**（メンテナンスモードは設定だけを読む。書き出しは DB に届けば名前とテーマを DB から読んで焼き込む）
2. **B2: 通常の運用中に DB の値をファイルへ書き出しておき、メンテナンスモードはそのファイルを読む**

## Decision（決定）

A1 と B1 を採り、A3 の静的な書き出しを A1 の補助として併せて用意する。
メンテナンスモードは通常のサーバーとは別の小さな HTTP ハンドラで、設定の一部だけを読んで起動する。書き出しコマンドは同じテンプレートで静的な HTML を作る。

### 1. 切り替え

- 設定キー `maintenance`（真偽値。環境変数 `MY_MAINTENANCE`、TOML、`my serve -maintenance`）が真なら、`my serve` はメンテナンスモードで起動する。切り替えは再起動で行う。
  - 起動中の切り替え（A2）は採らない。複数のインスタンス（Cloud Run・Lambda・k8s のレプリカ）に状態を揃えるには共有の置き場が要り、それは DB や KV そのものだから。
    Cloud Run・Lambda・k8s では環境変数の変更が新しいリビジョン（ロールアウト）になり、通常のデプロイと同じ操作で全インスタンスが揃う。
- `role`（web / worker / all）にかかわらず、メンテナンスモードのプロセスは HTTP だけを受け、ジョブの取得・定期実行・ストリームのブローカーへの接続をしない。
- 起動の経路を分ける: `cmd/my` は設定の読み込みの前に `maintenance` だけを見て、真なら `platform.NewMaintenanceApp` を組み立てる。
  これは `platform.NewApp` を呼ばず、アダプタを 1 つも作らない。応答とテンプレートは新しいパッケージ `internal/interface/maintenance`（標準ライブラリと `internal/i18n` だけに依存）に置く。

### 2. 設定の読み込みと検証

- メンテナンスモードで読む設定は次に限る。アダプタの選択と接続のキー（`db`・`database_url`・`queue`・`blob`・`kv`・`mail`・`stream_broker` など）は**読まず、検証もしない**。
  作業中に DB の URL が一時的に無効でも起動できるようにするため。
  - 共通: 待ち受けのアドレス、サーバーのホストとオリジン（Web・API）、ログとテレメトリのうち外への接続が要らないもの（標準エラーへのログ）。
  - メンテナンス用（新設）:

| キー | 環境変数 | 内容 |
| --- | --- | --- |
| `maintenance` | `MY_MAINTENANCE` | 真ならメンテナンスモードで起動する |
| `maintenance_server_name` | `MY_MAINTENANCE_SERVER_NAME` | ページに出すサーバーの名前。空ならホスト名 |
| `maintenance_message` | `MY_MAINTENANCE_MESSAGE` | 告知文（プレーンテキスト、上限 2,000 文字）。ADR-0009 §2 の管理者が入力する値と同じく 1 つの値をどのロケールでも出す |
| `maintenance_until` | `MY_MAINTENANCE_UNTIL` | 終了予定の時刻（RFC 3339）。任意 |
| `maintenance_retry_after` | `MY_MAINTENANCE_RETRY_AFTER` | `maintenance_until` が無いか過ぎたときの `Retry-After` の秒数。既定 600 |

- ステータスページの URL は [ADR-0085](0085-uptime-monitoring-status-page.md) の設定キー `status_page_url`（`MY_STATUS_PAGE_URL`、`https` のみ）を読む。設定キーなのでメンテナンスモードでも読め、書き出しも同じ値を使う。
- `my config check -maintenance` は上の範囲だけを検証する。通常の `my config check` は `maintenance_*` の値の形も検証する。
- 通常の運用で `maintenance_*` を常に設定しておいてよい（`maintenance` が偽なら使われない）。

### 3. 応答の表

共通: どの応答にも `Retry-After`（`maintenance_until` が未来ならそこまでの秒数、上限 86,400。それ以外は `maintenance_retry_after`）と
`Cache-Control: no-store` と、ADR-0019 のセキュリティヘッダを付ける。応答は振り分けのための最小のルーティングだけで決め、通常のルート表は使わない。

| 対象 | 判定 | 状態 | 本文 |
| --- | --- | --- | --- |
| `/healthz` | パス | 200 | `{"status":"maintenance"}` |
| `/readyz` | パス | 200 | `{"status":"maintenance"}`。ロードバランサ・k8s がインスタンスを外さず、メンテナンスの応答が利用者に届くように 200 にする。監視は本文で区別する（ADR-0085） |
| 外形監視の端点 `GET /api/v1/instance/health`（[ADR-0085](0085-uptime-monitoring-status-page.md)） | パス | 503 | `{"status":"maintenance"}`（形は ADR-0085）。監視がキーワードで「メンテナンス中」と判定できる |
| 上の端点を除く client API（`/api/*`）と OAuth の端点のうち JSON を返すもの（`POST /oauth/token`・`/oauth/token/refresh` など） | パス | 503 | `application/problem+json`、`type: /api/v1/problems/maintenance`、`title` は `Accept-Language` で選んだ「メンテナンス中」、拡張メンバー `message`・`until`（設定があれば）・`status_page_url`（ADR-0085 の設定があれば） |
| ActivityPub（Inbox の POST、Actor・オブジェクト・コレクションの GET） | `Accept` / `Content-Type` が ActivityPub、または `/ap/*`・`/inbox` | 503 | `application/json` の短い本文（`{"error":"maintenance"}`） |
| WebFinger（`/.well-known/webfinger`）・NodeInfo（`/.well-known/nodeinfo`、`/nodeinfo/*`）・host-meta | パス | 503 | 同上 |
| キューの push の受け口（Cloud Tasks・Lambda の SQS 受け口）・決済の webhook など外部からの通知 | パス | 503 | 同上。送り手の再試行に任せる |
| Web の HTML（ページ・サインイン・認可同意など、上に当たらない `GET`・`HEAD`） | 残り | 503 | メンテナンスのページ（§4） |
| Web クライアントの SPA と静的なアセット（JS・CSS・`sw.js`・`manifest.webmanifest`） | — | 503 | 出さない。メンテナンスのページが自己完結（インライン CSS・スクリプトなし）なので要らない |
| 上に当たらない `POST` などの非安全メソッド | 残り | 503 | メンテナンスのページ（`Accept` が JSON なら Problem） |

- **Web クライアントと Tauri アプリ**: Problem の `type` が `/api/v1/problems/maintenance` なら、画面全体をメンテナンスの表示（`message`・`until`・ステータスページへのリンク）に替え、
  `Retry-After` の後か利用者の操作で再読み込みする。**503 でトークンを捨てない・サインアウトしない**。キャッシュ済みの Service Worker が古いシェルを出しても、API の応答で同じ表示になる。
  ブラウザは `sw.js` の更新の取得が 2xx でなければ更新をやめ、登録済みの Service Worker を使い続ける（2026-10-23 に [Service Workers の Update 手順](https://w3c.github.io/ServiceWorker/#update-algorithm)で確認。登録を消すのは既存の worker が無いときだけ）。
- **連合先**: 503 と `Retry-After` は、こちらの配送の分類（[federation.md §7.3](../architecture/federation.md)）と同じく、送り手から見て「再試行する失敗」に当たる。
  - Mastodon は 5xx を再試行する（再試行しないのは 501 と、401・408・429 を除く 4xx）。再試行は 16 回で、`n` 回目の待ちは `n⁴ + 15` 秒にジッタ（最大 `0.5 × n⁴`）を足したもの（合計およそ 2〜3 日）。
    `Retry-After` は読まない。失敗した試行のたびにその日（UTC）を記録し、異なる 7 日に達したドメインを配送の対象から外す（`DeliveryFailureTracker`。こちらの `InstanceHealth` はこれに合わせている）。
    出典: [`delivery_worker.rb`](https://github.com/mastodon/mastodon/blob/131aabcaf1f1f13e0615f8ad2748ab05f966e2c9/app/workers/activitypub/delivery_worker.rb)・[`delivery_failure_tracker.rb`](https://github.com/mastodon/mastodon/blob/131aabcaf1f1f13e0615f8ad2748ab05f966e2c9/app/lib/delivery_failure_tracker.rb)・[`json_ld_helper.rb`](https://github.com/mastodon/mastodon/blob/131aabcaf1f1f13e0615f8ad2748ab05f966e2c9/app/helpers/json_ld_helper.rb)（2026-10-23 に確認）。
  - Misskey は 5xx を再試行し（再試行しないのは 429 を除く 4xx）、既定 12 回、待ちは `(2ⁿ − 1)` 分（上限 8 時間）に最大 20% のジッタで、合計およそ 1.5 日。`Retry-After` は読まない。
    失敗すると相手を「応答なし」にし、その状態が 7 日続くと自動で配送を停止する（`autoSuspendedForNotResponding`）。
    出典: [`DeliverProcessorService.ts`](https://github.com/misskey-dev/misskey/blob/2dd2e114b72dc256bdfa0ce88a8387f5cf6bd348/packages/backend/src/queue/processors/DeliverProcessorService.ts)・[`QueueService.ts`](https://github.com/misskey-dev/misskey/blob/2dd2e114b72dc256bdfa0ce88a8387f5cf6bd348/packages/backend/src/core/QueueService.ts)・[`QueueProcessorService.ts`](https://github.com/misskey-dev/misskey/blob/2dd2e114b72dc256bdfa0ce88a8387f5cf6bd348/packages/backend/src/queue/QueueProcessorService.ts)（2026-10-23 に確認）。
  - Akkoma は配送のジョブ（`federator_outgoing`）を 6 回まで試し、待ちは 16 秒・約 1 分・約 12 分・約 2.3 時間・約 21 時間で、合計およそ 1 日（[`config.exs`](https://akkoma.dev/AkkomaGang/akkoma/src/commit/a69f0b9e328dd6c12bf4a308892041035408d03a/config/config.exs)・[`publisher_worker.ex`](https://akkoma.dev/AkkomaGang/akkoma/src/commit/a69f0b9e328dd6c12bf4a308892041035408d03a/lib/pleroma/workers/publisher_worker.ex)・[`worker_helper.ex`](https://akkoma.dev/AkkomaGang/akkoma/src/commit/a69f0b9e328dd6c12bf4a308892041035408d03a/lib/pleroma/workers/worker_helper.ex)、2026-10-23 に確認）。
  - Pleroma は配送のジョブを 13 回まで試し、`n` 回目の後の待ちは `n⁵ + 15` 秒にジッタ（最大 `30n` 秒）を足したもので、合計およそ 7 日。最後の試行も失敗すると相手を到達不能とし、以後の配送のジョブを取り消す
    （2026-10-23 に git.pleroma.social の GitHub ミラーの [`publisher_worker.ex`](https://github.com/minibikini/pleroma/blob/149236e468c453495e0dcf83a9728057436b463b/lib/pleroma/workers/publisher_worker.ex) で確認）。
  - 最も早く尽きるのが Akkoma のおよそ 1 日なので、メンテナンスは**数時間以内**を目安にし、1 日を超えると連合先の再試行が尽きて受け取れない活動が出る。運用の文書にこの目安を書く。
- **検索エンジン**: Google は 503 を一時的な停止として扱い、`Retry-After` を再クロールの時期に使うことがある。長く続く 503 は恒久的な停止と見なされ索引から外されうる（2026-10-23 に [Google Search Central Blog](https://developers.google.com/search/blog/2011/01/how-to-deal-with-planned-site-downtime) で確認）。
  `/robots.txt` も 503 を返す。`robots.txt` が 5xx の間、Google は最初の 12 時間はサイトのクロールを止め、その後は最後に取得できた版を使う（2026-10-23 に [robots.txt の仕様](https://developers.google.com/crawling/docs/robots-txt/robots-txt-spec)で確認）。
  数時間のメンテナンスでクロールが止まるのは望ましく、`robots.txt` の中身は DB の設定（FR-725）から作るのでメンテナンス中には作れない。
  Google は 1〜2 日を超えてサイトを止めるときは `robots.txt` に 503 を返さないよう勧めているので（[サイトの一時停止](https://developers.google.com/search/docs/crawling-indexing/pause-online-business)）、それより長い停止はメンテナンスモードではなく ADR-0088 で扱う。

### 4. メンテナンスのページ（共通のテンプレート）

- `internal/interface/maintenance` の 1 つの `html/template` を、メンテナンスモードの応答と書き出しの両方が使う。
- 中身: サーバーの名前、「メンテナンス中」の見出しと説明、告知文（`dir="auto"`・`lang=""`。ADR-0009 §5）、終了予定の時刻（あれば）、ステータスページへのリンク（あれば）。
- 文言は新しいメッセージ（`server` の名前空間、`maintenance.*`）を `locales/en` と `locales/ja` に足し、生成した型付きアクセサで整形する。
  ロケールは `Accept-Language` → `en` で選ぶ（利用者設定とインスタンスの既定ロケールは DB にあるので使わない）。ルート要素の `lang` と `dir` をロケールから設定する。
- 終了予定の時刻: Go の側の名前空間は `date` / `time` の引数を使えない（AGENTS.md の i18n の規則）ので、`<time datetime="…">` に UTC の RFC 3339 の値を入れ、
  表示は ISO 8601 の UTC（例 `2026-10-24 03:00 UTC`）を文字列の引数として渡す。ロケールに合わせた整形はしない。
  Go の側に CLDR の日付の整形を持つと、メンテナンスのページのためだけに整形の実装とそのテストが要るため。
- 見た目: CSS は `<style>` にインラインで 1 つ。色・文字・余白は `theme.css` の既定のトークン（ライト / ダーク）から生成した値を使い、`prefers-color-scheme` でダークにする。CSS は論理プロパティだけを使う。
  画像は使わない（ロゴを出すなら `data:` のインライン SVG）。スクリプトは使わない。
- CSP: `default-src 'none'; style-src 'sha256-<インライン CSS のハッシュ>'; img-src data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`。
  ADR-0019 §9 を緩めないため、`'unsafe-inline'` ではなくハッシュで許す。書き出した HTML には同じ方針を `<meta http-equiv="Content-Security-Policy">` で入れる（`frame-ancestors` は meta では効かないので除く）。

### 5. 書き出しコマンド

- 形: `my maintenance export -out <dir> [-lang en,ja] [-config <file>] [-offline]`。
- 出力: 自己完結した HTML（インライン CSS、外部の資源なし）を、言語ごとに `<dir>/<lang>.html`、`-lang` の言語を順に並べた 1 枚を `<dir>/index.html` に書く。
  `index.html` は `Accept-Language` で振り分けられない前段（CloudFront・Cloud Storage のエラーページなど）向けで、どの言語の読み手にも 1 枚で伝わるように全言語を並べる。
  `<lang>.html` は nginx などで振り分ける前段向け。
- 値の出所: 設定の `maintenance_*` を使う。`-offline` でなく DB に届けば（短いタイムアウト）、インスタンスの名前（設定の `maintenance_server_name` が空のとき）・
  既定ロケール（`-lang` が無いときの先頭）・テーマのトークンの上書き（ADR-0059 §2。任意の CSS は含めない）を DB から読んで焼き込む。
  届かなければ警告を出して設定と既定のテーマで書き出し、終了コードは 0 にする。
- `my serve` を動かさずに実行でき、メンテナンスモードと同じテンプレートなので、どちらから出しても同じ見た目と文言になる。ADR-0088 のクローズ後のページもこのテンプレートと書き出しを使う。

### 6. ターゲットごとの置き方（運用文書の範囲）

`docs/operations/maintenance.md` を新設し、次を書く。IaC のテンプレートへの組み込みは M6（ADR-0010 の `deploy/`）。

- 手順: 告知（[ADR-0085](0085-uptime-monitoring-status-page.md) のステータスページ、FR-726 のお知らせ）→ `MY_MAINTENANCE=true` で再デプロイ → 作業 → 外して再デプロイ。
  push 型のキューは作業の前に止める。止めないと 503 の再試行で試行回数を使い切る。
  - Cloud Tasks: `gcloud tasks queues pause` で配信を止め（タスクの追加は続けられる）、`resume` で再開する。試行の上限はキューの `retryConfig.maxAttempts`（ドキュメントの既定の例は 100、`maxBackoff` 3600 秒）で決まる（2026-10-23 に [キューの設定](https://cloud.google.com/tasks/docs/configuring-queues)と [Queue の REST リファレンス](https://cloud.google.com/tasks/docs/reference/rest/v2/projects.locations.queues)で確認）。
  - SQS（Lambda のイベントソース）: イベントソースマッピングの `Enabled` を偽にするとポーリングと呼び出しが止まる（2026-10-23 に [UpdateEventSourceMapping](https://docs.aws.amazon.com/lambda/latest/api/API_UpdateEventSourceMapping.html) で確認）。
    止めないと失敗のたびに受信回数が増え、redrive policy の `maxReceiveCount` に達したメッセージはデッドレターキューへ移る（2026-10-23 に [SQS のデッドレターキュー](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html)で確認）。
- Cloud Run・Lambda: 環境変数を変えた新しいリビジョン。`my serve` も動かせないときは、前段のエラーページに `index.html` を置く。
  - CloudFront: カスタムエラーレスポンスで、オリジンの 5xx に S3 などに置いた `index.html` を返し、ビューアへの状態コード（`ResponseCode`）を 503 にできる。
    エラーページはオリジンと別の場所（S3）に置く（同じオリジンだと 5xx の間は取れない）。`Retry-After` はレスポンスヘッダーポリシーのカスタムヘッダーで付けられるが、ポリシーは付けたキャッシュビヘイビアのすべての応答に付くので、メンテナンスの間だけポリシーを付ける
    （2026-10-23 に [CustomErrorResponse](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_CustomErrorResponse.html)・[エラーページの設定](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/custom-error-pages-procedure.html)・[レスポンスヘッダーポリシー](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/understanding-response-headers-policies.html)で確認）。
  - Google Cloud: カスタムエラーレスポンスのポリシーは**グローバル外部アプリケーションロードバランサだけ**が持ち（リージョンとクラシックは非対応）、バックエンドの 5xx に、バックエンドバケット（Cloud Storage）の `index.html` を状態コードを上書き（`overrideResponseCode`）して返せる。
    リクエストのヘッダー（`Content-Type` など）で HTML と JSON を出し分けられる。エラーの取得元に定義したカスタムレスポンスヘッダーは返す応答に付かないので、この経路では `Retry-After` を返せない。
    ポリシーが効くのはバックエンドから来た応答だけで、Google Front End が作る応答には効かない（2026-10-23 に [カスタムエラーレスポンスの概要](https://cloud.google.com/load-balancing/docs/https/custom-error-response)で確認）。
    Cloud Run を直接（ロードバランサなしで）公開する最小構成には前段が無いので、メンテナンスモード（§1）だけで対応する。
- Docker compose: `MY_MAINTENANCE=true` で `my` のサービスを再作成。前段に nginx などがあれば `<lang>.html` を `error_page 502 503 504` に置く例。
- microk8s: Deployment の環境変数の変更（ロールアウト）。Ingress の既定のバックエンド・エラーページの例。
- ALB の固定応答は状態コード・`Content-Type`・最大 1,024 文字の本文だけを持ち、`Retry-After` などのヘッダを付けられない（2026-10-23 に [FixedResponseActionConfig](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_FixedResponseActionConfig.html) で確認）。
  書き出した HTML は 1,024 文字に収まらないので ALB の固定応答には置けず、短い文と 503 だけを返す例を書く。`Retry-After` を返せない前段では、連合先は §3 の自前の間隔で再試行する。

見直す条件: 運用者が無停止での切り替え（再デプロイなし）を強く求め、かつ全ターゲットで共有できる DB 以外の置き場（例: 設定の再読み込みをする k8s の ConfigMap、Cloud Run の再デプロイなしの環境の変更）が使えると分かった場合は A2 を足す。
連合先の再試行が 503 でも短時間で尽きる（数時間のメンテナンスで配送を止められる）と分かった場合は、応答の表を見直す。

### 要件（FR-805・FR-806）

`docs/requirements/01-functional.md` の「運用」の表に FR-805・FR-806 を足す。

| ID | 要件 | 優先度 |
| --- | --- | --- |
| FR-805 | メンテナンスモード: 設定 `maintenance`（環境変数・設定ファイル・`my serve -maintenance`）で、DB・KV・キュー・Blob・Mailer・外への HTTP に触れずに起動し、Web にはメンテナンスのページを、client API には `/api/v1/problems/maintenance` の Problem を、ActivityPub・WebFinger・NodeInfo には応答を、いずれも `503` と `Retry-After` で返す。`/healthz`・`/readyz` は 200 でメンテナンス中と分かる本文を返す。告知文・終了予定・ステータスページへのリンクを設定から出し、文言は `Accept-Language` で選ぶ（[ADR-0084](../adr/0084-maintenance-mode.md)） | M |
| FR-806 | メンテナンスのページの書き出し: `my maintenance export` が、メンテナンスモードと同じテンプレートで、言語ごとの自己完結した静的な HTML を書き出す。DB に届けばサーバーの名前とテーマを焼き込み、届かなくても設定から書き出せる（[ADR-0084](../adr/0084-maintenance-mode.md)） | S |

## Consequences（影響）

- 良い影響
  - DB・KV・キューの作業のあいだも、利用者・クライアント・連合先・監視が「メンテナンス中」と区別でき、連合先は配送を再試行する。
  - 切り替えは環境変数の変更と再デプロイだけで、どのターゲットでも同じ手順になる。通常の起動経路（`platform.NewApp`）に分岐を入れない。
  - 書き出しと応答が 1 つのテンプレートを使い、ADR-0088 のクローズ後のページにも流用できる。
- 悪い影響・トレードオフ
  - 切り替えに再起動（再デプロイ）が要り、その間の数秒〜数十秒は前段のエラーになりうる。
  - メンテナンスモードではサーバーの名前とテーマが設定の値と既定のテーマになり、通常の画面と見た目が揃わないことがある（書き出しは DB から焼き込める）。
  - 告知文は設定に置くので、管理画面からは変えられない（DB に触れない以上避けられない）。
  - `/readyz` がメンテナンス中に 200 を返すので、「readiness = DB に届く」という意味がメンテナンス中だけ変わる。監視は本文を見る必要がある（ADR-0085 と揃える）。
  - 終了予定の時刻はロケールに合わせて整形されない（ISO 8601 の UTC）。
  - 長いメンテナンスでは連合先の再試行が尽き、その間の活動を受け取れない。
- 後続で必要になる作業（それぞれ Issue にする）
  1. メンテナンスモード: 設定キーと部分的な読み込み・検証（`my config check -maintenance`、`docs/reference/configuration.md` の再生成）、`platform.NewMaintenanceApp`、
     `internal/interface/maintenance` のハンドラとテンプレート、メッセージ（en / ja）、応答の表のテスト、`tools/agentkit/context-map.yaml` への追加。
  2. client API: `/api/v1/problems/maintenance` を spec の Problem の説明に足し、Web クライアントと Tauri アプリにメンテナンスの表示（トークンを捨てない）を足す。
  3. 書き出しコマンド `my maintenance export`（DB からの焼き込みと `-offline`）。`docs/operations/admin-cli.md` に追記。
  4. 運用文書 `docs/operations/maintenance.md`（§6）。
  5. 要件の表に FR-805・FR-806 を足し、FR-802 の説明（readiness）にメンテナンス中の扱いを書く。

## Pros and Cons of the Options（詳細比較・任意）

### A1: 設定キーで再起動して切り替える（採用）

- 良い: 状態の共有が要らず、複数のインスタンスでも確実に揃う。ミドルウェアに触れない。ターゲットごとの差がない。
- 悪い: 再デプロイが要る。サーバーレスでは新しいリビジョンの起動を待つ。

### A2: 起動中に切り替える

- 良い（最も強い論点）: 再デプロイなしに即座に切り替えられ、作業の始めと終わりの空白がない。単一の VM（Docker compose で 1 プロセス）ならシグナルやファイルの監視で十分に動く。
- 悪い: 複数のインスタンスに揃えるには共有の置き場が要り、DB・KV は使えない。Cloud Run・Lambda はプロセスにシグナルを送る手段がなく、ファイルも共有されない。管理 API は DB の認証を通るので使えない。

### A3: 前段の静的な応答だけ

- 良い: サーバーのコードを変えない。`my serve` が起動しないときにも出せる。
- 悪い: Cloud Run・Lambda の Function URL には前段が無い構成がある。前段によっては `Retry-After` や状態コードを選べず、ActivityPub・API に JSON で答え分けることも難しい。
  そのため単独では判断基準 2・3・4 を満たさず、A1 の補助（書き出し）として採る。

### B1: 設定キーに持つ（採用）

- 良い: メンテナンスモードが何にも依存しない。値を作業の直前に決められる。
- 悪い: 管理画面の値（名前・テーマ）と二重になる。

### B2: 通常の運用中にファイルへ書き出しておき、メンテナンスモードが読む

- 良い（最も強い論点）: 名前とテーマが通常の画面と揃い、管理画面で告知文を書ける。
- 悪い: Cloud Run・Lambda のファイルシステムは揮発し、共有のボリュームか Blob が要る（Blob はメンテナンスの対象になりうる）。書き出しの時機と古さの管理が要る。
  書き出しコマンドで焼き込んだ HTML を前段に置けば同じ利点の多くが得られるので、v0 では採らない。

## Open Questions / 要検証

- `maintenance_retry_after` の既定 600 秒と、`maintenance_message` の上限 2,000 文字は、実装後の運用で見直す。
