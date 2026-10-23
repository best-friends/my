# ADR-0073: 収入の記録と公開

- Status: Accepted
- Date: 2026-10-22
- Deciders: @rosylilly
- Issue: #914

## Context（背景・課題）

M4.2（Supporters、Epic #925）は、課金のプランの支払いを収入として記録し（FR-917）、`view_finance` を持つ人に収入の集計を見せる（FR-916）。
決定者の指示（#925、原文）: 「また、この収入状況を Finance 同様に公開できるようにする。公開の粒度・範囲は Finance 同様にサーバー管理者が選べる」。

決定者の回答（#925、原文の抜粋）:

> 11\. 推奨: 公開するのは手数料を引いた額で、返金はその月で差し引きます。費用と並べて収支を出します。 → これで OK
> 公開範囲の設定は、finance_visibility とは別のキーにします。費用は公開しても、収入は公開しない、という選び方ができるようにするためです。 → こちらも OK
> 5\. 収入のうち、誰がいくら払ったかは個人情報です。 → こちらは見せない
> 収入については、各プランに何人いるかまでは見せられるように設定できるようにします。デフォルトは全体。

spike #909 の確認事項への決定者の回答（#914、原文）:

> 1\. Paddle を選択肢に残すか : 規約で認められないなら無理ですね。落とす
> 3\. Paddle を除外したので考慮外
> 4\. Stripe の Billing の手数料 : 収入から差し引く

この ADR の確認事項への決定者の回答（PR #931、原文）:

> 1\. 按分した年払いを返金したときの扱い: 推奨案で OK
> 2\. 収入の日付の基準: 推奨案で OK
> 3\. プランごとの人数を、いまの人数だけにするか: 推奨案で OK
> 4\. そのほかの推奨案（O1・L1・V1・E1）: そのまま採ってよいか。 : 推奨案ままで OK

事業者の事実は spike #909 の[調査報告](https://github.com/mycroblog/my/issues/909#issuecomment-6326974258)（2026-10-22 に公式の文書で確認）による。

- Stripe: 決済・返金・dispute はそれぞれ balance transaction になり、`amount`（総額）・`fee`・`net` = `amount` − `fee` を入金の通貨（日本のアカウントなら JPY）で持つ。返金で処理の手数料は返らず、dispute は ¥1,500 の手数料が勝っても返らない。
  Billing の 0.7%・Stripe Tax・Radar の手数料は決済ごとの `fee` に入らず、別の `stripe_fee` の取引として載る。

既存の決定と、この ADR を待つもの:

- [ADR-0067](0067-finance-cost-records.md) は `finance` モジュールで費用を記録し、期間（ISO 週・暦月・暦年）を日付で区切り、通貨ごとに合計し、現金主義と按分（既定）を出し、`finance_visibility` で公開する。
  [ADR-0068](0068-billing-import.md) は事業者の値を取り込みごとのイベントとして記録し、事業者の請求の日付の基準（タイムゾーン）をそのまま持つ。
- [ADR-0070](0070-permission-split-emoji-finance.md) §5 は `finance_viewer_income`（`totals` / `plans`、既定 `totals`）を決め、収入の合計の値と `plans` の人数を数える時点をこの ADR に任せている。
- 決済の事業者のポート `PaymentProvider` と webhook・照合のジョブは ADR-B（#913）、加入の状態とプランは ADR-A（#912）が決める。ADR-B は ADR-0074 として並行して書かれている。事業者は Stripe だけで、Paddle は利用規約（AUP）の懸念で外れた（上の回答 1）。
  この ADR は、ポートの後ろの事業者によらず同じ形で `finance` に渡す契約を決める。

決めること: 収入の記録の持ち主と経路、記録の単位と net、返金・チャージバック・手数料の扱い、日付と期間、通貨、計上のしかた（年払いの按分）、個人ごとの支払いの扱い、公開範囲と公開先、`view_finance` に見せる収入、データモデル、ADR-0067 との関係。
実装の #923（収入の記録と公開）と #915（`view_finance`）の収入の部分がこの ADR を待っている。

## Decision Drivers（判断基準）

1. 誰が何を払ったかを外に出さないこと: どの公開範囲・どの閲覧の権限でも、個人ごとの支払いは `manage_finance` の管理画面の外に出ない
2. 収支が正しく、説明できること: 収入と費用が同じ期間・同じ計上のしかた・同じ通貨の規則で並び、公開した過去の期間の額が後から動かない（返金はその月で差し引く）
3. 事業者の明細と照らし合わせられること: 月の収入が事業者のダッシュボードやレポートの月の値と合う
4. 事業者に依存しないこと: 事業者は Stripe だけだが、`finance` は事業者の型を知らず、ポートの後ろに別の事業者を足しても `finance` を変えない
5. 既存の規約（ADR-0002 の依存の向き、ADR-0020 のデータモデル、ADR-0003 の冪等な書き込み、ADR-0067 の期間と通貨）にそのまま乗ること
6. 実装が小さいこと: 加入者は数十〜数百人の規模

## Considered Options（選択肢）

論点ごとに比べる。各論点の 1 つめを採用する。

- **持ち主と経路**: O1 `finance` が収入の行を持ち、加入のモジュールが記録を呼ぶ / O2 加入のモジュールが収入の行を持ち、`finance` がポートで読む
- **記録の単位**: L1 事業者のお金の動き 1 件ごとのイベント（総額・手数料・net）/ L2 期間ごとの net の合計の現在値
- **日付**: D1 事業者のレポートのタイムゾーンでの日付 / D2 UTC の日付 / D3 新しいインスタンスのタイムゾーン設定
- **計上のしかた**: B1 費用と同じく現金主義と按分を出し、按分が既定。返金は元の支払いの期間に沿って打ち消す / B2 現金主義だけ / B3 按分で、返金は全額を返金の日に載せる
- **公開範囲**: V1 `income_visibility` = `private` / `totals` / `plans` / V2 `private` / `totals` の 2 段階
- **公開先**: E1 新しい `GET /instance/income_totals` に収入と収支を載せる / E2 `GET /instance/cost_totals` に収入を足す
- **プランごとの人数の時点**: N1 いまの人数 / N2 期間ごとの終わりの人数

## Decision（決定）

**O1・L1・D1・B1・V1・E1・N1 を採用する。** Stripe の Billing の手数料は収入から引く（§2。決定者の回答 4）。

### 1. 持ち主と経路（O1）

- 収入の行は `finance` が持つ。費用と同じモジュールで期間・按分・通貨の関数（`internal/domain/finance` の `Spread`・`Summarize`）を共有し、収支を 1 か所で計算する。
- 事業者からの取り込みは、ADR-B の照合と webhook のジョブ（加入のモジュール。名前は ADR-A が決める）が `PaymentProvider` からお金の動きを取り、`internal/app/finance` の `RecordIncome(ctx, []IncomeMovement)` を呼んで渡す。
  `internal/app` どうしの呼び出しは ADR-0002 の唯一のモジュール間の経路で、向きは「加入のモジュール → `finance`」だけにする。
- `finance` から加入のモジュールへの読み取り（プランごとの人数、§7。管理画面の加入者の表示、§6）は、`internal/app/finance` のポート `SupporterDirectory` にし、`internal/platform` が加入のモジュールで実装を組み立てる（ADR-0070 §4 の `CustomEmojiQuota` と同じ形）。import の循環を作らない。
- 課金が無効なら、`SupporterDirectory` は空を返し、収入の行は作られない。

**事業者によらない契約**（ADR-B の `MoneyMovement` をこの形に写す）:

```go
type IncomeMovement struct {
    Provider       string    // "stripe" (the providers ADR-B enables)
    ProviderID     string    // Stripe: balance transaction id; unique per provider
    Kind           IncomeKind // payment, refund, chargeback, chargeback_reversal, fee
    ParentID       string    // refund / chargeback / reversal: ProviderID of the payment; "" otherwise
    SubscriptionID string    // our subscription id (ADR-A); "" for fees not tied to one subscription
    OccurredAt     time.Time // when the provider booked it (UTC)
    OccurredOn     Date      // OccurredAt as a date in the provider's reporting time zone (§3)
    Covers         DateRange // payment: the billing period it pays for; others: zero
    Gross, Fee, Net string   // decimal in major units of Currency; Net = Gross - Fee; negative for refunds
    Currency       string    // ISO 4217 (§4)
}
```

- 渡すのは事業者の側で額が確定したものだけにする。Stripe は balance transaction ができたとき（`fee` と `net` はそのときに決まる）。見込みの値は持たない（ADR-0068 §3 の見込みと確定の版は要らない）。
- 契約テスト（ADR-B のポートの契約に足す）: 各事業者の adapter が、決済・一部と全額の返金・チャージバック・その取り消し・独立した手数料の筋書きで、`Net = Gross − Fee`、符号、`ParentID`、`Covers`、`OccurredOn` の日付の基準を満たす `IncomeMovement` を返すことを確かめる。

### 2. 記録の単位と net（L1）

- お金の動き 1 件を 1 行のイベントにする（§8 の `income_movements`）。`(provider, provider_id)` で一意にし、同じ動きを webhook・照合・再試行で何度渡しても 1 行になる（ADR-0003。挿入の衝突は何もしない）。
  Stripe の balance transaction は作られた後に変わらないので、版を持たない。事業者の側の訂正は、新しい動き（`adjustment`）として来る。
- 総額・手数料・net の 3 つを持ち、公開と `view_finance` には net だけを出す（決定者の回答 11）。総額と手数料は `manage_finance` の管理画面で、手数料率の確かめに使う。
- 種類ごとの値:

| 種類 | 総額 | 手数料 | net | Stripe |
| --- | --- | --- | --- | --- |
| `payment` | 支払いの額 | 決済ごとの手数料 | 総額 − 手数料 | charge の balance transaction |
| `refund` | −返金の額 | 返金にかかる手数料（Stripe は 0。元の決済の手数料は返らない） | 総額 − 手数料 | `refund` の balance transaction |
| `chargeback` | −取り戻された額 | dispute の手数料（¥1,500） | 総額 − 手数料 | `adjustment`（dispute）の balance transaction |
| `chargeback_reversal` | 戻った額 | 0 | 総額 | 正の `adjustment` |
| `fee` | 0 | 加入に結び付かない手数料 | −手数料 | `stripe_fee` |

- **Stripe の Billing の手数料**（決定者の回答 4「収入から差し引く」）: Billing の 0.7%・Stripe Tax・Radar の手数料は決済ごとの `fee` に入らず、別の `stripe_fee` の balance transaction として載る。
  ADR-B の照合のジョブがこれも取り込み、`fee` の種類の行（`subscription_id` を持たない）としてその日の収入から引く。加入ごとに割り当てられるかは **要検証** だが、割り当てなくても期間の net は正しい。
- 個人ごとの支払いの扱い（決定者の回答 5）: `income_movements` は `subscription_id` だけを持ち、利用者の ID・名前・メールアドレスを持たない。誰の加入かは `SupporterDirectory` を通して管理画面だけが引く。アカウントの消去で加入のモジュールが利用者との結び付きを消せば、収入の行は額だけを残し、集計は変わらない。

### 3. 日付と期間（D1）

- 期間は ADR-0067 §5 と同じく暦の日付で区切り（ISO 週・暦月・暦年）、週・月・年を独立に日付で区切る。
- お金の動きは時刻を持つので、`OccurredAt` を**事業者のレポートのタイムゾーン**での日付にして `occurred_on` に持つ。ADR-0068 が AWS を UTC、Google Cloud を太平洋時間のまま持つのと同じく、月の収入を事業者のダッシュボードとレポートの月の値に合わせるため（判断基準 3）。
  - Stripe: アカウントのタイムゾーン（ダッシュボードの設定）。adapter が API（`Account.settings.dashboard.timezone`）で読む。API で読めなければ UTC にする（D2 と同じ）。API で読めるかは **要検証**（#923 の実装と #924 のテストモードでの確認で確かめる）。
  - ADR-B の `PaymentProvider` に、ADR-0068 の `BillingSource.Location()` と同じ `Location() *time.Location` を持たせ、adapter が日付にする。`occurred_at` も残し、管理画面の 1 件の表示に使う。
- 集計の近くに示す区切りの規則（ADR-0067 §5）に、収入の日付の基準（事業者のレポートのタイムゾーン）を足す。
- 公開するのは今日を含む期間までで、ADR-0067 §8 と同じく後の期間は出さない。年払いの按分（§5）で来年に載る分も、その期間が来るまで出さない。

### 4. 通貨

- ADR-0067 §6 と同じく、通貨の最小単位の整数と ISO 4217 の通貨コードで持ち、合計は通貨ごとに出し、換算しない。事業者の 10 進の金額は、1 件ごとに最近接偶数への丸めで最小単位にする（`MinorUnitsOf`）。
- Stripe は balance transaction の `currency`（入金の通貨）で記録する。プランの価格の通貨と違えば Stripe が換算した後の額になる。

### 5. 計上のしかたと年払い（B1）

- ADR-0067 §4 と同じ 2 つの見方を出し、既定は按分にする。収支は収入と費用を**同じ見方**で並べる。
- **現金主義**（`cash`）: 各行の net を `occurred_on` に載せる。
- **按分**（`amortized`）:
  - `payment` で、`Covers` が 1 か月より長い（年払い）ものは、net を `Covers` の日数に日割り（ADR-0067 §4 の累積の丸め、`Spread`）で載せる。月払いと、`Covers` を持たない支払いは `occurred_on` に載せる。
    ADR-0067 §4 が年払いの費用を按分するのと同じ規則で、年払いの加入者がいる月だけ収入が跳ねることを防ぎ、按分の費用と並べたときに「月にいくら足りるか」を示す。
  - `refund`・`chargeback`・`chargeback_reversal` で、元の支払い（`parent_id`）が按分されているものは、net を元の支払いと同じ `Covers` に日割りし、**`occurred_on` より前の日の分をまとめて `occurred_on` に載せ**、残りの日は元の支払いと同じ日割りで打ち消す。
    返金した月にそれまでに載った分が差し引かれ（決定者の回答 11「返金はその月で差し引きます」）、公開した過去の期間の額は動かず、全額の返金なら後の月の按分は 0 になる。返金の総額は net の合計に一致する。
  - それ以外（元の支払いが按分されていない返金、`fee`、返らない手数料）は `occurred_on` に載せる。
- 例: 2026-04-10 に 12,000 円の年払い（決済の手数料 3.6% の 432 円を引いて net 11,568 円、`Covers` 2026-04-10〜2027-04-09 の 365 日）を、2026-06-01 に全額返金（net −12,000 円。決済の手数料は返らない）。
  - 按分では、4 月と 5 月（52 日）に元の支払いの `floor(11568·52/365) = 1,648` 円が載る。
  - 6 月には、返金の 4/10〜5/31 の分（約 −1,710 円）がまとめて載り、6 月以降の日ごとには元の支払い（約 +31.7 円）と返金（約 −32.9 円）の日割りが並ぶ。
  - 返らない手数料 432 円は、返金の日以降の日割りの差として残る。どの期間の分け方でも、全期間の合計は 11,568 − 12,000 = −432 円になる。
  - 現金主義では、4 月に 11,568 円、6 月に −12,000 円が載る。

### 6. 権限と管理画面

- 収入の記録は事業者からの取り込みだけで、手入力・変更・取り消しの操作を持たない。監査ログ（ADR-0037）には何も足さない（スタッフの操作がない）。公開範囲の変更は既存の設定の変更として残る。
- `manage_finance` を持つ人は、`/admin/finance` の収入の区画で、期間ごとの通貨ごとの総額・手数料・net の合計、収支、個々のお金の動き（日時・種類・総額・手数料・net・事業者の ID・加入者のアカウント）を見る。
  個々の動きと加入者のアカウントは、この画面と管理 API だけに出す（判断基準 1）。
- `view_finance` を持つ人（ADR-0070 §5）は `/finance` で、期間ごとの通貨ごとの net の合計と収支を見る。`finance_viewer_income = plans` か、`income_visibility = plans`（公開に出ている範囲は見せる）なら、§7 の人数を足す。総額・手数料・個々の動き・加入者・プランごとの金額は見せない。

### 7. 公開範囲（V1）と公開先（E1）

- インスタンス設定のキー `income_visibility`（`enum(private, totals, plans)`、未設定は `private`）を `instance` に足す。`finance_visibility` と同じく `manage_settings` で守り、`/admin/settings` の費用の区画の隣に置く。
  - `private`: 公開の API は `404` を返し、`/about` に収入を出さない。
  - `totals`: 期間ごとの通貨ごとの net の合計を出す。
  - `plans`: 合計に加え、プランごとのいまの加入者の数（N1）を出す。プランごとの金額は出さない。
- どの段階でも、個々の支払い、加入者、プランごとの金額、総額と手数料は出さない。
- **プランごとの人数**（N1）: 問い合わせの時点で、プランのロールが付いている加入（ADR-A の状態で「有効」「解約の予約」「猶予の内の支払いの遅れ」）の数をプランごとに数える。`SupporterDirectory.PlanCounts(ctx)` が返す（プランの ID・表示名・人数。廃止したプランは人数が 0 なら省く）。
  期間ごとの人数を出さないので、加入と解約の時期が期間の表から読めない。加入者の少ないプランでは人数から誰かが推測できることを、設定の画面で `plans` を選ぶところに書く（ADR-0070 の悪い影響と同じ）。
- **収支**: 収入と費用の両方が公開されているとき（`income_visibility` と `finance_visibility` がどちらも `private` でない）だけ、期間ごとの通貨ごとに「収入の net − 費用」を出す。片方だけが公開なら収支を出さない（収支から非公開の側が逆算できるため）。
  通貨が揃わなければ通貨ごとの収支が並ぶ（ADR-0067 §6）。
- **公開先**: `/about` の「運営の費用」の区画に収入と収支を足し、区画は `finance_visibility` か `income_visibility` のどちらかが `private` でなければ出す。`/about/costs` のページに、費用の表と並べて収入と収支の表を出す（経路は変えない）。

### 8. データモデル（ADR-0020）

`finance` が所有する。ADR-0067 §2 と ADR-0068 §5 の表に次を足す。

| テーブル | 分類 | 内容 |
| --- | --- | --- |
| `income_movements` | イベント | `id`, `provider`（`enum(stripe)`。事業者を足すときに値を足す）, `provider_id`（`string(255)`）, `kind`（`enum(payment, refund, chargeback, chargeback_reversal, fee)`）, `parent_movement_id`（NULL 可。`refund`・`chargeback`・`chargeback_reversal` は元の `payment` の行）, `subscription_id`（NULL 可 = `fee`。ADR-A の加入の ID への論理参照で、外部キーを付けない）, `occurred_at`, `occurred_on`（`date`）, `covers_starts_on`・`covers_ends_on`（`date`、NULL 可。`payment` だけが持つ。`CHECK`）, `gross_minor`・`fee_minor`・`net_minor`（`bigint`、負も可。`net_minor = gross_minor − fee_minor` の `CHECK`）, `currency`（`string(3)`）, `recorded_at`。一意 `(provider, provider_id)`。インデックス `(occurred_on)`, `(subscription_id, occurred_at)` |

- 投影を持たず、集計は読み取りのときに計算する（ADR-0067 の A1）。数百人 × 年 12 回の規模で、期間の範囲の行を読んで按分しても軽い。
- 元の支払いより先に返金が来たとき（照合の順序）は、ADR-B のジョブが元の支払いを事業者から取り直して先に渡す。`RecordIncome` は元の行がない返金を受けず、ジョブの再試行に任せる。
- 状態（返金済み・按分中）を列に持たない。返金済みかは子の行の有無と合計で、按分は `covers_*` と見方で決まる（規約 4）。

### 9. API の形（[client-api.md](../architecture/client-api.md)）

ADR-0067 §9 と同じ問い合わせ（`granularity`・`from`・`to`・`basis`）と金額の表現（主単位の 10 進の文字列と通貨）、期間の数の上限を使う。

- 公開（タグ `instance`、`security: []`、`Cache-Control: public, max-age=60`）: `GET /instance/income_totals` は期間ごとの通貨ごとの net の合計、`plans` ならプランごとの人数、収支（§7 の条件のとき）、`rules`（日付の基準・按分の規則）を返す。`private` では `404`。
  `GET /instance` に `income_visibility` を足す。
- `view_finance`（タグ `finance`、スコープ `read:finance`、ADR-0070 §5）: `GET /finance/income_totals` は同じ形で、収支を常に、人数を §6 の条件で返す。`Cache-Control: private, no-store`。
- 管理（タグ `admin-server`、`admin:read` と `manage_finance`）: `GET /admin/income_totals`（総額・手数料・net の合計と収支）と `GET /admin/income_movements`（キーセットページング。期間・種類で絞り込む。加入者のアカウントを含む）。
- 金額と期間のラベルの書式化は ADR-0067 §10 のとおり Web クライアントが `Intl.*` で行う。収入・収支・手数料・種類・人数（`plural`）の文言は `client*` 名前空間に足す。

### ADR-0067 との関係

この ADR は ADR-0067 の次の文だけを置き換える。ADR-0067 は Accepted のままで、その他の決定と本文は変えない。ADR-0067 の Status の行に、この ADR が置き換える文を注記する（ADR-0066 が ADR-0016・ADR-0029 にしたのと同じ）。

- ADR-0067 §1「ほかのモジュールは `finance` を呼ばない」: 加入のモジュールが `finance` の `RecordIncome` を呼ぶ（§1）。`finance` からほかのモジュールへの読み取りはポートにする。
- ADR-0067 §8「公開する場所: 匿名で読める API（§9）と、`/about` の「運営の費用」の区画と、そこからリンクする `/about/costs` のページ」のうち区画を出す条件: 区画は `finance_visibility` か `income_visibility` のどちらかが `private` でなければ出し、収入と収支を含む（§7）。

追加（置き換えではない）: ADR-0067 §2 の表に `income_movements`、§8 の設定に `income_visibility`、§9 の API に収入の集計、§5 の区切りの規則に事業者ごとの日付の基準。

### この決定を覆す条件

- 運営者が事業者の外の収入（銀行振込の寄付など）も載せたいと求めたら、手入力の収入の項目（費用の項目と同じ形）を別の ADR で足す。
- 返金の按分の打ち消し（§5）が運営者に分かりにくいと分かったら、B3（返金は全額を返金の日に載せる）にし、按分の表で後の月に返金済みの収入が残ることを受け入れる。
- アカウントのタイムゾーンを API で読めても、UTC の日付との違いが運営者に分かりにくいと分かったら、すべて D2（UTC）にする。
- プランの人数の推移を見たいという要望が多ければ、N2（期間ごとの人数）を、加入者の少ないプランを伏せる規則とともに検討する。

## Consequences（影響）

- 良い影響
  - 収入と費用が同じ期間・見方・通貨の規則で並び、収支を 1 つの関数で計算できる。年払いの収入と年払いの費用が同じ按分で並ぶ。
  - 返金が公開した過去の期間を動かさず、返金の月に差し引かれる。どの期間の分け方でも、全期間の合計が net の合計に一致する。
  - 収入の行は事業者の ID で一意なので、webhook・照合・再試行がどの順で何度来ても同じ結果になる。
  - `finance` は利用者の ID を持たず、個人ごとの支払いは管理画面と管理 API の外に出る経路がない。
  - 契約が事業者によらないので、ADR-B（ADR-0074）の adapter と独立に `finance` の側を作れ、事業者を足しても `finance` は変わらない。
- 悪い影響・トレードオフ
  - 按分の表では、返金した年払いの元の支払いと返金の日割りが後の月に並ぶ（打ち消し合って 0 になる）。表の行を見る `manage_finance` には説明が要る。
  - 日付の基準が事業者のタイムゾーンなので、日本の運営者の Stripe のタイムゾーンが UTC のままだと、月の境目の支払いが前の月に載る。設定の確かめを運営者向けの docs に書く。
  - Stripe から銀行への入金にかかる費用（あれば）は収入から引かれず、運営者が ADR-0067 の費用の項目として記録する。
  - Stripe の `stripe_fee` はアカウント全体の手数料なので、同じ Stripe のアカウントをほかの販売にも使うと、その手数料も収入から引かれる。運営者向けの docs で、このサーバー専用のアカウントを勧める（ADR-0068 の絞り込みに当たる仕組みは持たない）。
  - 収支は両方が公開のときだけ出るので、収入だけを公開したサーバーは「足りているか」を示せない。
  - 加入の通貨と費用の通貨が違うと、収支が通貨ごとに分かれて 1 つの数にならない（ADR-0067 §6 と同じ）。
  - `plans` の人数は加入者の少ないプランで個人を推測させうる（ADR-0070 と同じ）。
- 後続で必要になる作業
  - #923（収入の記録と公開）:
    - `internal/domain/finance`: `IncomeMovement` の検査、種類、按分の規則（年払いの日割り、返金の前の日の分の繰り上げと残りの打ち消し）、収支の関数と表駆動のテスト（一部・全額の返金、チャージバックと取り消し、手数料、期間の境界、うるう年）
    - スキーマ: `income_movements`（`schema/<dialect>.sql`・sqlc）、data-model.md の `finance` の所有の表
    - `internal/app/finance`: `RecordIncome`（一意の衝突は何もしない、元の行がなければ失敗）、ポート `SupporterDirectory` と課金が無効なときの空の実装、`internal/platform` での組み立て
    - 設定 `income_visibility`（`instance`、`/admin/settings`）、`GET /instance` の項目
    - API: `GET /instance/income_totals`・`GET /admin/income_totals`・`GET /admin/income_movements`（`api/spec/` と生成物）
    - Web: `/admin/finance` の収入の区画、`/about` の区画と `/about/costs` の収入と収支の表、区切りの規則の日付の基準、`client*` のメッセージ（`locales/en`・`locales/ja`）
    - ADR-B の adapter への受け入れの条件: `PaymentProvider.Location()` と `MoneyMovement` から `IncomeMovement` への写し、§1 の契約テスト（#919・#920 と調整）
  - #915（`view_finance`）: `GET /finance/income_totals`（収支を常に、人数を `finance_viewer_income = plans` か `income_visibility = plans` のとき）と `/finance` の収入と収支の区画。#923 の後に足すか、#915 が `finance_viewer_income` を足すときに収入の口だけ空で置くかは、実装の順で決める
  - #912（ADR-A）: `SupporterDirectory` が返すプランごとの人数（ロールが付いている状態の定義）と、加入の ID から利用者を引く口、アカウントの消去での結び付きの扱い
  - #913（ADR-B）: `MoneyMovement` に `Covers`・`ParentID`・`Location()` を持たせ、確定した動きだけを渡す
  - 運営者向けの docs: Stripe のアカウントのタイムゾーンと専用のアカウント

## Pros and Cons of the Options（詳細比較・任意）

### 持ち主: O1 `finance`（採用）/ O2 加入のモジュール

- O2 の最も強い論点: お金の動きは加入と同じジョブで取り込み、個人との結び付きが加入のモジュールの中に閉じる。`finance` を誰も呼ばない ADR-0067 §1 も保てる。
  悪い点: 期間・按分・通貨の関数を加入のモジュールが `finance` から借りる（`internal/app/finance` を import する）か複製することになり、収支の計算が 2 つのモジュールにまたがる。O1 でも利用者の ID は `finance` に入らない。

### 記録: L1 動きごとのイベント（採用）/ L2 期間ごとの合計の現在値

- L2 の良い点: 行が少なく、公開の集計が読むだけで済む。悪い点: 按分と現金主義の両方、週・月・年の区切りごとに合計を持つことになり、返金の繰り上げ（§5）を合計の上では表せない。冪等な書き込みも「足す」操作になり、二重の取り込みで額がずれる。

### 日付: D1 事業者のタイムゾーン（採用）/ D2 UTC / D3 インスタンスのタイムゾーン

- D2 の良い点: adapter の設定も要検証の項目も要らない。悪い点: 日本の運営者の 5/1 午前の支払いが 4 月に載り、Stripe のダッシュボードの月の値と合わない。
- D3 の最も強い論点: 収入と手入力の費用を運営者の暦で揃えられる。悪い点: ADR-0067 が T2 を採らなかった理由（設定が増え、事業者の明細と合わない）がそのまま当たる。

### 計上: B1 按分が既定・返金は打ち消す（採用）/ B2 現金主義だけ / B3 返金は全額をその日に

- B2 の最も強い論点: 事業者の入金と 1 件ずつ一致し、返金も「その月で差し引く」がそのまま成り立つ。悪い点: 按分の費用と並べると、年払いの加入者が更新する月だけ黒字に、ほかの月は赤字に見える。費用を現金主義で見る場合と揃えるため、B1 でも現金主義の見方は出す。
- B3 の良い点: 返金の扱いが現金主義と同じで説明が短い。悪い点: 全額を返金した年払いの元の支払いの日割りが後の月に残り、返金済みの収入が按分の表に出続ける。全期間の合計も、返金の月に全額を引くので、後の月の分だけ二重に引かれたように見える。

### 公開範囲: V1 3 段階（採用）/ V2 2 段階

- V2 の良い点: 人数から個人を推測させる設定がない。悪い点: 決定者の指示（「公開の粒度・範囲は Finance 同様にサーバー管理者が選べる」）と、`view_finance` に人数まで見せる設定（ADR-0070）に対して、公開だけが狭くなる。

### 公開先: E1 新しい `GET /instance/income_totals`（採用）/ E2 `GET /instance/cost_totals` に足す

- E2 の良い点: 1 回の要求で費用・収入・収支が揃う。悪い点: `finance_visibility = private` で `404` を返す ADR-0067 §9 の契約と、収入だけを公開する組み合わせ（決定者の回答 11）がぶつかる。E1 では、`/about/costs` のページは公開されている側の API だけを呼ぶ。

### 人数の時点: N1 いま（採用）/ N2 期間ごと

- N2 の最も強い論点: 支援者の推移が分かり、収入の増減の理由を説明できる。悪い点: 加入の状態の履歴から過去の時点を復元する問い合わせが ADR-A に要り、参加者の一覧（FR-914）と合わせると、誰がいつ解約したかが期間の表から読める。

## Open Questions / 要検証

- **要検証**: Stripe のアカウントのタイムゾーンを API で読めるか（§3。#923 と #924 で確かめる。読めなければ UTC）。`stripe_fee` を加入ごとに割り当てられるか（spike §13。割り当てられなくてもこの ADR の形は変わらない）。
