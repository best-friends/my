# クラウドの利用料金の取り込み

運営者向けの手順です。決定は [ADR-0068](../adr/0068-billing-import.md)（費用の記録は [ADR-0067](../adr/0067-finance-cost-records.md)）、設定のキーは [設定リファレンス](../reference/configuration.md#billing_sources) です。

設定 `billing_sources` に挙げた事業者の利用料金を、サーバーが 24 時間ごとに取り込んで費用に載せます（FR-904）。既定は空で、何も取り込みません。
取り込むのは今月と先月（月の 15 日まで、または先月が確定するまで）の請求月の費用で、サービスと費用の種類（利用・クレジット・返金・税・サポート・料金・その他）ごとに 1 行になります。
事業者が月を締めるまでは見込み、締めた後は確定、確定の後に値が変わったら修正として記録し、過去の記録は書き換えません。
取り込みの状態（最後の成功、最後の失敗の種類、今月の要求数）は管理画面の費用のページと `GET /api/v1/admin/billing_imports` で見られます。

認証の鍵をこのサーバーの設定には置きません。各事業者の SDK の既定の認証の連鎖（AWS は実行ロール・環境変数・共有の認証ファイル、Google Cloud は Application Default Credentials）が読むものを、実行環境に渡します。

## AWS（`billing_sources = aws`）

AWS Cost Explorer の `GetCostAndUsage` を、請求月（UTC の暦月）ごと・サービス（`SERVICE`）と記録の種類（`RECORD_TYPE`）ごとに 1 回呼びます。エンドポイントは常に `us-east-1` です。
金額は USD で、Cost Explorer が月を見込み（`Estimated`）と示す間は前日までの額を見込みとして、そうでなくなったら確定として記録します。
build tag `no_billing_aws` で build したバイナリは `aws` を拒みます（[ビルドタグ](../reference/build-tags.md)）。

### 費用と上限

Cost Explorer の API は、ページ付けされた要求 1 回ごとに $0.01 かかります。小さなアカウントなら 1 日 1 回・1 ページで、月に約 $0.31 です。
想定外の課金を防ぐため、要求の数を UTC の日ごと（`billing_aws_max_requests_per_day`、既定 4）と UTC の暦月ごと（`billing_aws_max_requests_per_month`、既定 120 = $1.20）に数え、上限に達したら呼ばずに止めます。
定期の取り込み・ジョブの再試行・管理画面の「今すぐ取り込む」をすべて数えます。上限に達した取り込みは `request_cap` の失敗として残り、日の上限なら翌日、月の上限なら翌月の初日の取り込みが再び試します。

### Cost Explorer を有効にする

1. Billing and Cost Management のコンソールで Cost Explorer を開き、有効にします。一度有効にすると無効にはできません。データが揃うまで最大 24 時間かかります。
2. 有効でないアカウントは `DataUnavailableException` を返し、取り込みは `not_enabled` の失敗になります。

### IAM の権限

必要な権限は `ce:GetCostAndUsage` だけです（リソースは `*`）。

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow", "Action": "ce:GetCostAndUsage", "Resource": "*" }
  ]
}
```

実行環境ごとの渡し方:

| 実行環境 | 認証 |
| --- | --- |
| AWS Lambda / ECS | 実行ロール（タスクロール）にこのポリシーを付けます。設定は要りません |
| Google Cloud（Cloud Run） | 下の「Cloud Run から web identity で呼ぶ」の IAM ロールを作り、`billing_aws_role_arn` にその ARN を設定します。AWS の鍵は要りません |
| 自前のサーバー | このポリシーを付けた IAM ユーザーのアクセスキーを `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`（または共有の認証ファイルと `AWS_PROFILE`）で渡します。シークレットストアから渡してください |

権限がない、または認証情報が無効なときは `permission` の失敗になります。

### Cloud Run から web identity で呼ぶ

`billing_aws_role_arn` を設定すると、サーバーはメタデータサーバーから Cloud Run のサービスアカウントの Google の ID トークン（audience はそのロールの ARN）を取り、`AssumeRoleWithWebIdentity` でそのロールの一時的な認証情報を得ます。

1. Google Cloud で、Cloud Run のサービスのサービスアカウントの一意の ID（数字の `uniqueId`、`gcloud iam service-accounts describe <メール> --format='value(uniqueId)'`）を控えます。
2. AWS の IAM で、上の権限のポリシーを付けたロールを作り、信頼ポリシーで `accounts.google.com` を信頼します。

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Principal": { "Federated": "accounts.google.com" },
         "Action": "sts:AssumeRoleWithWebIdentity",
         "Condition": {
           "StringEquals": {
             "accounts.google.com:aud": "<サービスアカウントの uniqueId>",
             "accounts.google.com:oaud": "<このロールの ARN>"
           }
         }
       }
     ]
   }
   ```

   `accounts.google.com:aud` はトークンの `azp`（サービスアカウントの ID）、`accounts.google.com:oaud` はトークンの `aud`（サーバーが要求する audience = ロールの ARN）に対応します。
3. `MY_BILLING_AWS_ROLE_ARN=arn:aws:iam::<アカウント ID>:role/<ロール名>` を設定します。

この組み立ては実アカウントでの確認がまだです（#882）。

### Organizations の連結アカウントから呼ぶ

AWS Organizations の管理アカウントは、組織のすべてのメンバーアカウントの費用を見られます。メンバーアカウント（連結アカウント）の認証情報で呼ぶときは、
管理アカウントの Cost Explorer の設定（Preferences）で「Linked Account Access」を有効にしておく必要があります。無効だと IAM の権限に関係なく拒否され、取り込みは `not_enabled` か `permission` の失敗になります。

### どの費用を取り込むか（絞り込み）

`billing_aws_filter` で選びます。

| `billing_aws_filter` | 取り込む費用 | 追加の設定 |
| --- | --- | --- |
| `none`（既定） | 認証情報で見えるすべての費用 | なし |
| `linked_account` | 指定したメンバーアカウントの費用（`LINKED_ACCOUNT`） | `billing_aws_linked_accounts`（12 桁のアカウント ID、カンマ区切り） |
| `tag` | 指定したコスト配分タグの付いたリソースの費用 | `billing_aws_tag`（`key=value`、例 `app=my`） |

いちばん正確なのは、このサーバーだけのための AWS アカウント（Organizations のメンバーアカウントでもよい）を用意し、`none`（そのアカウントの認証情報で呼ぶ）か `linked_account`（管理アカウントから呼ぶ）で取ることです。
タグを付けられない税・サポート料・一部のクレジットは、タグの絞り込みから漏れることがあります（実アカウントでの確認は #882）。

### コスト配分タグを有効にする（`billing_aws_filter = tag`）

リソースに付けたタグは、コスト配分タグとして有効にするまで請求のデータに出ません。

1. このサーバーのリソース（Lambda 関数、RDS、S3 のバケット、CloudFront など）に同じタグ（例 `app=my`）を付けます。
2. Billing and Cost Management のコンソールの「コスト配分タグ」で、そのタグキーを選んで有効にします。タグを付けてからこの一覧に出るまで最大 24 時間、有効にしてから請求のデータに反映されるまでさらに最大 24 時間かかります。
3. 有効にする前の期間の費用は、同じ画面の「バックフィル」（API の `StartCostAllocationTagBackfill`）で遡って反映できます。
4. `MY_BILLING_AWS_FILTER=tag` と `MY_BILLING_AWS_TAG=app=my` を設定します。

### 指標

`billing_aws_metric` は、割引（Savings Plans・リザーブドインスタンス・個別の価格）の後の `NetUnblendedCost`（既定。請求書に近い）か、割引の前の `UnblendedCost` です。
Cost Explorer の値と請求書は更新の時期と丸めで違うことがあり、違うときは請求書の額が正です。

### 失敗の種類

| 種類 | AWS の応答 | 直し方 |
| --- | --- | --- |
| `permission` | `AccessDeniedException`、無効な認証情報 | IAM の権限と認証情報の渡し方を確かめる |
| `not_enabled` | `DataUnavailableException` | Cost Explorer を有効にする。連結アカウントなら Linked Account Access を有効にする |
| `not_found` | `BillExpirationException` | 取り込む期間の請求のデータが Cost Explorer にもうない |
| `throttled` | `LimitExceededException` | ジョブが時間を置いて再試行する（要求の上限に数える） |
| `unavailable` | 5xx、ネットワークの失敗 | ジョブが時間を置いて再試行する（要求の上限に数える） |
| `invalid_response` | 読めない応答、`ValidationException` など | 設定（タグ・アカウント ID）を確かめる |
| `request_cap` | （呼ばない） | 上限の日・月が明けるのを待つか、上限を上げる |

`throttled` と `unavailable` のほかは再試行しても直らないので、記録してそのまま終わり、翌日の定期の取り込みが再び試します。

## Google Cloud（`billing_sources = gcp`）

Google Cloud には実際の費用を返す API がないため、Cloud Billing の BigQuery への export（標準の使用料金）を、BigQuery の `jobs.query` で 1 日 1 回集計します（`billing_gcp_method = bigquery`、既定）。
請求月は export の `invoice.month` で、米国太平洋時間（夏時間あり）の暦月です。日付も太平洋時間のまま記録します。
金額は請求先アカウントの通貨で、請求月・サービス（`service.description`）・費用の種類（`cost_type`）ごとに `NUMERIC` で合計し、クレジット（`credits[]`）はサービスごとに 1 つのクレジットの行にします。export の訂正の打ち消しの行も合計に入ります。
export には確定の印がないので、翌月の 5 日（太平洋時間）からの取り込みでその月を確定とし、それまでは月の初日から最後の利用の日までの見込みとして記録します。確定の後も 15 日までは取り直し、違いは修正として記録します。
build tag `no_billing_gcp` で build したバイナリは `gcp` を拒みます（[ビルドタグ](../reference/build-tags.md)）。

### 費用と上限

1 回の取り込みは 1 回のクエリで、BigQuery はクエリごとにスキャンしたバイト数（最低 10 MB）を課金します。小さなアカウントなら月に 0.3 GB ほどで、毎月最初の 1 TiB の無料枠（請求先アカウントのほかの用途と共有）に収まり、$0 です。
想定外のスキャンを防ぐため、クエリに必ず `maximumBytesBilled`（`billing_gcp_max_bytes_billed`、既定 1 GiB）を付けます。超えるクエリは課金されずに失敗し、`bytes_cap` の失敗として残ります。課金されたバイト数は管理画面に出ます。

### BigQuery への export を有効にする

運営者が一度だけ、コンソールで設定します（API での設定の方法は見つかっていません）。

1. 請求先アカウントに紐づいたプロジェクトに、BigQuery のデータセット（例 `billing_export`）を作ります。場所はマルチリージョン（`US` か `EU`）にします。今月と先月の分が遡って入ります（リージョンだと有効にした日からになります）。場所は後で変えられません。
   テーブルの有効期限は付けないでください。消えた行は戻りません。
2. 「お支払い → 課金データのエクスポート → BigQuery Export」で「標準の使用料金」を有効にし、1. のデータセットを選びます。設定には請求先アカウントの「請求先アカウント費用管理者」か「請求先アカウント管理者」と、プロジェクトの BigQuery ユーザーの役割が要ります。
3. 数時間から 1 日ほどでテーブル `gcp_billing_export_v1_<BILLING_ACCOUNT_ID>`（ID のハイフンは `_`）が作られます。
4. `MY_BILLING_GCP_TABLE=<プロジェクト>.billing_export.gcp_billing_export_v1_<BILLING_ACCOUNT_ID>` を設定します。

データセットやテーブルが見つからないときは `not_found` の失敗になります。

### IAM の役割

取り込むサーバーのサービスアカウントに、次の 2 つを付けます。

| 役割 | 付ける場所 | 用途 |
| --- | --- | --- |
| `roles/bigquery.jobUser`（BigQuery ジョブユーザー） | クエリを実行するプロジェクト（`billing_gcp_query_project`、空ならテーブルのプロジェクト） | `bigquery.jobs.create`。スキャンのバイトはこのプロジェクトに課金されます |
| `roles/bigquery.dataViewer`（BigQuery データ閲覧者） | export のデータセット | `bigquery.tables.getData` |

```sh
gcloud projects add-iam-policy-binding <クエリのプロジェクト> \
  --member=serviceAccount:<サービスアカウント> --role=roles/bigquery.jobUser
bq add-iam-policy-binding --member=serviceAccount:<サービスアカウント> \
  --role=roles/bigquery.dataViewer <プロジェクト>:billing_export
```

クエリのプロジェクトで BigQuery API が有効でないときは `not_enabled`、役割が足りないときは `permission` の失敗になります。

認証は Application Default Credentials（ADC）です。

| 実行環境 | 認証 |
| --- | --- |
| Google Cloud（Cloud Run） | サービスのサービスアカウントで ADC。鍵も設定も要りません |
| AWS Lambda | Workload Identity Federation（AWS の種類）の credential configuration のファイルを置き、`GOOGLE_APPLICATION_CREDENTIALS` でそのパスを渡します（実アカウントでの確認は #882） |
| 自前のサーバー | Workload Identity Federation か、サービスアカウントの JSON 鍵を `GOOGLE_APPLICATION_CREDENTIALS` で渡します。鍵はシークレットストアから渡してください |

### どの費用を取り込むか（絞り込み）

既定では請求先アカウントのすべての費用を取り込みます。いちばん正確なのは、このサーバーだけのための請求先アカウントを使い、絞らずに取ることです。

| 設定 | 取り込む費用 |
| --- | --- |
| `billing_gcp_projects`（プロジェクト ID、カンマ区切り） | 指定したプロジェクトの費用（`project.id`） |
| `billing_gcp_label`（`key=value`、例 `app=my`） | 指定したラベルの付いたリソースの費用（`labels`） |

両方を設定すると、両方に当てはまる費用だけを取り込みます。
税など請求書の単位の費用はプロジェクトもラベルも持たないことがあり、そのときは絞り込みから漏れます（実アカウントでの確認は #882）。

### 失敗の種類

| 種類 | BigQuery の応答 | 直し方 |
| --- | --- | --- |
| `permission` | 401、403（`accessDenied`） | サービスアカウントの役割と認証情報の渡し方を確かめる |
| `not_enabled` | 403（`accessNotConfigured` など） | クエリのプロジェクトで BigQuery API を有効にする |
| `not_found` | 404 | `billing_gcp_table` と export の設定を確かめる |
| `bytes_cap` | 400（`bytesBilledLimitExceeded`） | スキャンが `billing_gcp_max_bytes_billed` を超えた。テーブルが大きいなら上限を上げる |
| `throttled` | 429、403（`rateLimitExceeded`・`quotaExceeded`） | ジョブが時間を置いて再試行する |
| `unavailable` | 5xx、ネットワークの失敗、終わらないクエリ | ジョブが時間を置いて再試行する |
| `invalid_response` | 読めない応答、月の中で通貨が混ざる行、ほかの 400 | export のテーブル（標準の使用料金か）を確かめる |

`throttled` と `unavailable` のほかは再試行しても直らないので、記録してそのまま終わり、翌日の定期の取り込みが再び試します。
