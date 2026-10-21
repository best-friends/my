# サポーターの決済（Stripe）

運営者向けの手順です。決定は [ADR-0074](../adr/0074-payment-provider-port.md)（ポート・webhook・照合・Stripe の adapter）、[ADR-0072](../adr/0072-supporter-plans-and-roles.md)（プラン・加入・ロール）、[ADR-0073](../adr/0073-income-records-and-publication.md)（収入の記録）で、設定のキーは [設定リファレンス](../reference/configuration.md#payments_provider) です。

設定 `payments_provider = stripe` にすると、ローカルの利用者が Stripe のホスト型の決済画面（Checkout）で月払い・年払いのプランに加入し、支払いの状態に合わせてプランのロールが付け外しされます（FR-910・FR-911）。
課金を有効にすることとプランの作成は、管理者が管理画面で行います。この設定は、サーバーが Stripe と話せるようにするだけです。

1 つのサーバーで有効にできる事業者は 1 つだけです。後でほかの事業者に切り替えるときは、既存の加入者に新しい事業者で再加入してもらいます。

## サーバーが Stripe で行うこと

- **プランと価格**: プランごとに Product（ID は `my_plan_<プランの ID>`、`metadata.my_plan_id`）と、間隔・通貨ごとの Price（`lookup_key` は `my_<プランの ID>_<month|year>_<通貨>`）を API で作ります。
  Stripe の Price は金額を変えられないので、金額を変えると新しい Price を作って古い Price をアーカイブします。古い Price の加入はその金額のまま更新されます。年払いの割引は年の Price の金額で表し、クーポンは使いません。
  ダッシュボードでこれらの Product と Price を編集・削除しないでください。
- **加入**: Checkout を `mode=subscription` で作り、利用者のブラウザをその URL（`checkout.stripe.com`）へ移します。決済画面は 1 時間で期限が切れます。
  こちらの加入の ID を `client_reference_id`・`metadata.my_subscription_id`・サブスクリプションの `metadata.my_subscription_id` に入れます。税（Stripe Tax）の自動計算は有効にしません。
- **顧客ポータル**: 支払い方法の更新・請求書の履歴・期間の終わりでの解約だけを有効にした設定（`metadata.my=1`）を、最初に使うときに API で作ります。プランの変更はポータルではなく、このサーバーの画面で行います。
- **プランの変更**: 上げる変更はすぐ切り替えて差額を日割りで請求し（`proration_behavior=always_invoice`）、下げる変更と年払いから月払いへの変更は Subscription Schedule で期間の終わりに切り替えます。
- **状態の取り直し**: webhook は合図として受け、サーバーが Stripe の API からサブスクリプション・Checkout・お金の動きを取り直して状態を決めます。
  加えて `payments_reconcile_interval`（既定 6 時間）ごとの照合が、イベントの一覧・未完了の Checkout・balance transaction・全サブスクリプションの見回しを読み、webhook の取りこぼしを直します。
- **収入**: balance transaction の総額・手数料・net を記録します。Billing の 0.7% などの手数料は別の `stripe_fee` の取引として載り、収入から引かれます。
  `stripe_fee` はアカウント全体の手数料なので、**このサーバー専用の Stripe アカウントを使ってください**。ほかの販売に使っているアカウントでは、その手数料も収入から引かれます。
- **日付**: 収入の日付は Stripe のアカウントのタイムゾーン（ダッシュボードの「設定 → ビジネス → アカウントの詳細」のタイムゾーン、API の `settings.dashboard.timezone`）で数えます。読めないときは UTC で数え、1 時間ごとに読み直します。

サーバーが送る要求は、1 秒に 20 件（テストのモードでは 10 件）までに抑えます。Stripe が `429` を返したら、`Retry-After` が 30 秒以内ならそれだけ待って 2 回まで送り直し、それを越えたらジョブの再試行に任せます。

## カード情報と PCI DSS

カード番号などの支払い方法の入力は Stripe の Checkout と顧客ポータルだけで行い、このサーバーの Web クライアントは Stripe のスクリプトも iframe も読み込みません。
サーバーが保存するのは Stripe のオブジェクトの ID（`cus_`・`sub_`・`price_`・`prod_`・`cs_`・`txn_`）と金額・期間・状態だけで、カードの番号・下 4 桁・有効期限・請求先の住所は保存しません。webhook の本文と Stripe の応答はログにも DB にも残しません。
このため運営者の PCI DSS の範囲は、ホスト型の決済画面を使う販売者の SAQ A 相当になる見込みです。適用される範囲と自己問診の種類は、運営者が Stripe のダッシュボードの案内（「設定 → コンプライアンス」）で確かめてください。

## 1. Stripe のアカウントを用意する

1. Stripe のアカウントを作り、ダッシュボードでビジネスの情報を登録します。日本で販売するなら、特定商取引法の表記のページの URL を「設定 → ビジネス → 公開情報 → 特定商取引法に基づく表記」に登録すると、Checkout の下にリンクが出ます。
2. 「設定 → ビジネス → アカウントの詳細」でタイムゾーンを確かめます。収入の日付はこのタイムゾーンで数えます。
3. 「設定 → Billing → サブスクリプションとメール → 支払いの管理」で、支払いに失敗したときの再試行（Smart Retries）と、再試行が尽きたときの扱いを選びます。
   ロールを外す猶予（最初の失敗から 7 日）はこのサーバーが数えるので、Stripe の再試行の設定には左右されません。再試行が尽きたら「サブスクリプションをキャンセル」を勧めます。
4. 加入者に領収書を送るなら、「設定 → カスタマーへのメール」で支払いの成功の領収書を有効にします。

## 2. API の鍵を作る

「開発者 → API キー」で**制限付きのキー**（`rk_test_…` / `rk_live_…`）を作ることを勧めます。シークレットキー（`sk_test_…` / `sk_live_…`）も使えます。
鍵の接頭辞でモードが決まり、テストの鍵では実際の請求が起きません。公開可能キー（`pk_`）は使えません。

制限付きのキーに与える権限は次のとおりです（**要検証**: 正確な権限の名前と過不足は、#924 のテストモードでの確認で確かめます）。

| リソース | 権限 | 使う API |
| --- | --- | --- |
| Products | 書き込み | `GET`・`POST /v1/products`, `POST /v1/products/{id}` |
| Prices | 書き込み | `GET`・`POST /v1/prices`, `POST /v1/prices/{id}` |
| Checkout Sessions | 書き込み | `POST /v1/checkout/sessions`, `GET /v1/checkout/sessions/{id}` |
| Customer portal | 書き込み | `GET`・`POST /v1/billing_portal/configurations`, `POST /v1/billing_portal/sessions` |
| Subscriptions | 書き込み | `GET /v1/subscriptions`, `POST`・`DELETE /v1/subscriptions/{id}` |
| Subscription Schedules（Subscriptions に含まれる） | 書き込み | `POST /v1/subscription_schedules`, `POST /v1/subscription_schedules/{id}`, `POST /v1/subscription_schedules/{id}/release` |
| Invoices | 読み取り | `GET /v1/invoices/{id}`, `GET /v1/invoice_payments` |
| Charges・Refunds・Disputes | 読み取り | `GET /v1/charges/{id}`, `GET /v1/refunds` |
| Balance transactions（Balance） | 読み取り | `GET /v1/balance_transactions` |
| Events | 読み取り | `GET /v1/events` |
| Account（Connect の外のアカウントの情報） | 読み取り | `GET /v1/account`（タイムゾーン） |

鍵は環境変数 `MY_PAYMENTS_STRIPE_SECRET_KEY`（またはファイルの場所を `MY_PAYMENTS_STRIPE_SECRET_KEY_FILE`）で渡します。設定ファイルには書けません（[ADR-0010](../adr/0010-configuration-and-packaging.md)）。
鍵が漏れたら、ダッシュボードで鍵をローテーションし、新しい鍵で再デプロイします。

## 3. webhook のエンドポイントを登録する

「開発者 → Webhook → エンドポイントを追加」で、次を登録します。サーバーはエンドポイントを API で作りません。

- URL: `https://<サーバーのドメイン>/payments/webhooks/stripe`
- イベントの送信元: 「お客様のアカウント」（Connect のアカウントではない）
- ペイロードの形式: snapshot（スナップショット）。サーバーは本文の `id`・`type`・`livemode`・`created`・`data.object.id`・`data.object.object` だけを読むので、エンドポイントの API の版は問いません。
- 送るイベント（16 種類）:

  - `checkout.session.completed`・`checkout.session.expired`
  - `customer.subscription.created`・`customer.subscription.updated`・`customer.subscription.deleted`・`customer.subscription.paused`・`customer.subscription.resumed`
  - `invoice.paid`・`invoice.payment_failed`
  - `charge.refunded`・`refund.created`・`refund.updated`
  - `charge.dispute.created`・`charge.dispute.closed`・`charge.dispute.funds_withdrawn`・`charge.dispute.funds_reinstated`

登録したエンドポイントの「署名シークレット」（`whsec_…`）を、環境変数 `MY_PAYMENTS_STRIPE_WEBHOOK_SECRET`（または `_FILE`）で渡します。

受け口は署名（`Stripe-Signature` の HMAC-SHA256）と時刻（前後 5 分）を確かめ、テストのイベントをライブの鍵で受けたとき（その逆も）は `400` で拒みます。
受けたイベントは ID ごとに 1 回だけ記録してジョブに積み、`2xx` を返します。DB に書けなかったときは `503` を返し、Stripe が送り直します。本文は 1 MiB までです。
署名シークレットをローテーションしている間は Stripe が新旧の両方で署名するので、設定には新しいシークレットを置けば足ります。

事業者の状態（モード、最後に webhook を受けた時刻、最後の照合の結果と失敗の種類）は管理者向けに示されます（ADR-0074 §4）。webhook を一度も受けていない、または照合が加入を直したときは、webhook の設定の誤りを疑ってください。

## 4. サーバーの設定

```sh
MY_PAYMENTS_PROVIDER=stripe
MY_PAYMENTS_STRIPE_SECRET_KEY=rk_test_...
MY_PAYMENTS_STRIPE_WEBHOOK_SECRET=whsec_...
# 任意: 照合の周期（15m〜168h、既定 6h）と、1 回の照合が送る要求の上限（既定 1000）
MY_PAYMENTS_RECONCILE_INTERVAL=6h
MY_PAYMENTS_RECONCILE_MAX_REQUESTS=1000
```

`my config check` で設定を確かめられます。build tag `no_payments_stripe` で build したバイナリは `payments_provider = stripe` を拒みます（[ビルドタグ](../reference/build-tags.md)）。

## 5. テストのモードで試す

まずテストの鍵（`rk_test_` / `sk_test_`、またはサンドボックスの鍵）で動かします。テストのモードでは実際の請求が起きません。
テストの鍵とライブの鍵では、Product・Price・顧客・サブスクリプション・webhook のエンドポイントが別々です。ライブに移るときは、ライブのモードでエンドポイントを登録し直し、ライブの鍵と署名シークレットに差し替えて再デプロイします。プランの Price はライブのアカウントに改めて作られます。

テスト用のカード（[Stripe の文書](https://docs.stripe.com/testing)）:

| 番号 | 動き |
| --- | --- |
| `4242 4242 4242 4242` | 成功 |
| `4000 0000 0000 0341` | 登録はできるが、請求で失敗する（支払いの失敗と猶予の確認） |
| `4000 0000 0000 9995` | 残高不足で失敗 |
| `4000 0000 0000 0259` | 支払いの後に dispute が作られる |

有効期限は未来の日付、セキュリティコードは任意の 3 桁です。

更新・支払いの失敗・猶予を試すには、テストの時計（test clock）を使います。テストの時計に結び付けた顧客で加入し、ダッシュボードの「Billing → テストクロック」で時間を進めると、更新の請求と失敗が起きます。

## 6. ローカルで試す（Stripe CLI）

[Stripe CLI](https://docs.stripe.com/stripe-cli)（Apache-2.0）で、Stripe のイベントを手元のサーバーへ転送できます。公開の URL は要りません。

```sh
stripe login
stripe listen --forward-to http://localhost:8080/payments/webhooks/stripe \
  --events checkout.session.completed,checkout.session.expired,customer.subscription.created,customer.subscription.updated,customer.subscription.deleted,customer.subscription.paused,customer.subscription.resumed,invoice.paid,invoice.payment_failed,charge.refunded,refund.created,refund.updated,charge.dispute.created,charge.dispute.closed,charge.dispute.funds_withdrawn,charge.dispute.funds_reinstated
```

`stripe listen` が表示する `whsec_…` を `MY_PAYMENTS_STRIPE_WEBHOOK_SECRET` に置いて、サーバーを起動します（ポートは `listen` の設定に合わせます）。
ほかの端末で、単発のイベントを起こせます。

```sh
stripe trigger checkout.session.completed
stripe trigger invoice.payment_failed
stripe trigger charge.refunded
stripe trigger charge.dispute.created
stripe events resend evt_...   # 受け取り損ねたイベントを送り直す（30 日まで）
```

`stripe trigger` が作るオブジェクトはこのサーバーの加入の ID を持たないので、加入の状態は変わりません（サーバーは取り直して、こちらの加入でないものは無視します）。加入の流れ全体は、Web クライアントから加入してテスト用のカードで払って確かめます。

## 7. 運用

- 照合の失敗の種類が `permission` なら、鍵の誤りか権限の不足です。
- 照合が要求の上限に当たる回が多ければ、`payments_reconcile_max_requests` を上げるか `payments_reconcile_interval` を延ばします。
- 照合が 30 日より長く止まると、Stripe のイベントの一覧（30 日まで遡れる）からは追いつけません。そのときは全サブスクリプションの見回しが直します。
- 返金とチャージバックは Stripe のダッシュボードで行います。全額の返金とチャージバックはロールを外してサブスクリプションをすぐ解約し、一部の返金ではロールを外しません（ADR-0072）。dispute に勝ってもロールは自動では戻りません。

## 要検証の事項

次の事項は Stripe の文書から決めた動きで、テストのモードで確かめます（#924）。

- Subscription Schedule による期間の終わりの価格の変更と、その取り下げ（release）。
- アーカイブした Price の加入が、その Price のまま更新されること。
- サブスクリプションの一覧で `latest_invoice` と `schedule` を展開できること、支払いの失敗の時刻を請求書の確定（`status_transitions.finalized_at`）の時刻とみなしてよいこと。
- `GET /v1/account` の `settings.dashboard.timezone` を制限付きの鍵で読めること。
- 制限付きの鍵に要る権限の一覧（§2 の表）。
- 通貨ごとの最低の請求額の表と、Stripe が ISO 4217 と違う小数の桁で扱う通貨（ISK・HUF・TWD・UGX など）。
- 読み取りのレート制限の正確な値。
