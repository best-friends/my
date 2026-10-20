# クライアント API の規約と追加手順

独自クライアント API（`/api/v1`）を設計・実装するときの規約と、エンドポイントを追加する手順です。
根拠は [ADR-0007](../adr/0007-client-api.md)（独自 API）、[ADR-0016](../adr/0016-client-api-style.md)（様式）、[ADR-0019](../adr/0019-security-baseline.md)（認証・レート制限・ヘッダ・CORS）、[ADR-0029](../adr/0029-api-code-generation.md)（コード生成）です。

## 1. ファイルの配置

| ファイル | 役割 |
| --- | --- |
| `api/openapi.yaml` | API の正。OpenAPI 3.1 の単一ファイル |
| `api/redocly.yaml` | スキーマの lint 設定（Redocly CLI）。推奨ルールと、使える 3.1 の部分集合・命名規約を強制する独自ルール |
| `api/api.go` | `openapi.yaml` を埋め込む Go パッケージ（`api.Spec`）。テストの照合ヘルパーが読む |
| `internal/interface/clientapi/oapi-codegen.yaml` | oapi-codegen の設定（strict server、`net/http`。`streamEvents` は生成から除く） |
| `internal/interface/clientapi/api.gen.go` | 生成物。手で編集しない |
| `internal/interface/clientapi/security.gen.go` | 生成物。spec の `security` から作る操作ごとの認証・スコープの表（`internal/securitygen`）。手で編集しない |
| `internal/interface/clientapi/auth.go`, `ratelimit.go` | 認証・認可・レート制限（§4.2） |
| `internal/interface/clientapi/*.go` | 生成された `StrictServerInterface` の実装。アプリケーション層のユースケースを呼ぶ |
| `internal/interface/clientapi/streaming.go` | 手書きの `GET /streaming`（SSE、§5） |
| `internal/interface/clientapi/apitest` | ハンドラの応答を spec と照合するテストヘルパー |
| `app/web/src/lib/api/schema.gen.ts` | 生成物。Web クライアントの型（openapi-typescript）。手で編集しない |
| `scripts/generate.sh` | 生成スクリプト（`make generate` / `make generate-check`）。TS の型の生成に `app/web` の依存を使うので、先に `make web-install` を実行する（Makefile の依存で自動） |

列挙型の定数は `<型名><値>`（`StatusVisibilityPublic`）の名前で生成します（`oapi-codegen.yaml` の `always-prefix-enum-values`）。別の列挙型を足しても名前が衝突・変化しません。

spec は分割せず `api/openapi.yaml` の 1 ファイルに書きます。バンドルの工程を持たないので、生成器・lint・照合ヘルパーが同じファイルをそのまま読みます。

## 2. 規約

### 2.1 パスと命名

- パスは `/api/v1/...` です。spec の `servers` が `/api/v1` なので、`paths` には `/api/v1` を除いた部分（`/instance`）を書きます。
- `operationId` は lowerCamelCase の動詞句（`getInstance`、`createStatus`）です。Go のメソッド名（`GetInstance`）と TS の操作名になります。
- プロパティ名とパス・クエリのパラメータ名は snake_case です（lint で強制）。
- タグはモジュール単位（`instance`、`statuses`、`accounts` など）で付けます。

### 2.2 ID とページング

- ID は UUIDv7 の文字列で、`#/components/schemas/Id` を参照します（`x-go-type: string` により Go では `string`）。
- 一覧はキーセットページングです。クエリで `#/components/parameters/Cursor` と `#/components/parameters/Limit` を受け、
  応答は `allOf` で `#/components/schemas/PageInfo` を含めて `next_cursor` を返し、要素の配列は `items` に入れます。最後のページでは `next_cursor` を省きます。
- カーソルは不透明な文字列です。クライアントは解釈せずにそのまま返します。
  ハンドラは `limit` より 1 件多く読み、次のページがあるときだけ `next_cursor` を返します。
  アカウントの一覧のカーソルは、ページの最後の項目の位置（フォロー ID・ブロック ID・ミュートの作成時刻と対象）を base64url にしたものです（`Server.parsePage`）。
- 生成コードは `minimum` / `maximum` などのスキーマの制約を検査しないので、`limit` やカーソル・本文の値の範囲はハンドラが検査し、外れたら 400 にします（`pageLimit`、`Server.invalidParameter`）。
- 見せられない項目（[visibility.md §6](visibility.md#6-アカウントの表示)）を省く一覧では、最後でないページでも `limit` より少ないことがあります。
- 投稿の一覧は `#/components/responses/StatusPage`（`items` が `#/components/schemas/Status` の配列）を返し、ハンドラは `Server.statuses` で投稿とブーストを詰め替えます（投稿は `Server.newStatus`）。
  ブーストは、ブーストした人の `Status` の `boost` に元の投稿を入れて表します。
  アカウントの投稿の一覧（`listAccountStatuses`）だけは、`StatusPage` に `remote_fetch_started` を加えた `#/components/responses/AccountStatusPage` を返します。
  その要求がリモートのアカウントの最近の投稿の取得を始めたときだけ `true` が入り、ほかのときは省きます（[federation.md §6.3](federation.md#63-投稿)、ADR-0044）。

### 2.3 エラー

- すべての操作は `default` 応答で `#/components/responses/Problem` を参照します（lint で強制）。
  本文は RFC 9457 の `application/problem+json` です。
- 状態コード以上の意味を持たない問題は `type: about:blank` です。クライアントが分岐に使う問題には、安定した `type` の URI（`/api/v1/problems/<名前>` の相対参照）を割り当て、
  spec の `#/components/schemas/Problem` の説明に列挙します。認証・認可の `type` は §4.2 の表のとおりです。
- `title` と `detail` は `internal/i18n` の `server` 名前空間からローカライズし、`Content-Language` を返します。
  ロケールは `i18n.Resolver` が ADR-0009 §2 の順（利用者設定 → `Accept-Language` → インスタンス設定 `default_locale` → `en`）で決めます。
  サーバーの HTTP 応答（クライアント API、ログイン画面、HTML の外枠、プラットフォームの問題応答）はすべて `platform.App.Locales` の同じ Resolver を使います。
  `default_locale` は解決のたびに読むので、変更は再起動なしで反映されます。
  文言は `locales/en` と `locales/ja` に追加して `make i18n-gen` を実行し、生成された型付きアクセサ（`l.ServerErrors...()`）で整形します。
- ハンドラが返す問題は `Server.NewProblem` で作り、その操作の `default` 応答型（`<Operation>defaultApplicationProblemPlusJSONResponse`）で返します。
  `Server.problem` は同じ問題をどの操作の `default` 応答型にも型変換できる `problem` で返します（`GetAccountdefaultApplicationProblemPlusJSONResponse(s.problem(...))`）。
  生成コードの外（認証・認可・レート制限、解釈できないリクエスト）で返す問題は `Server.writeProblem` が書き、`Cache-Control: no-store` を付けます。
  ローカライザは `Server.Localizer(ctx)` で取得します。
- 生成コードが解釈できないリクエスト（パラメータの形式違いなど）は 400 になり、`detail` がパラメータ名を示します。
  解釈できても本文の値をアプリケーション層が拒否した場合（長すぎる・多すぎる・不正な値）は 422（`about:blank`、`title` は `l.ServerErrorsUnprocessable()`）で、
  `detail` がプロパティ名を示します（`l.ServerErrorsTooLong`・`ServerErrorsTooMany`・`ServerErrorsInvalidValue`）。
  パス・クエリのパラメーターが不正な UTF-8 かヌル文字を含む要求は、`platform` が操作より前に 400 にします。
  JSON の本文の文字列がヌル文字を含む要求は、ハンドラより前に strict のミドルウェア（`rejectInvalidBodyText`）が 422（`l.ServerErrorsInvalidValue`。`media[1].description` のようにプロパティの位置を示す）にします（[data-model §1.2](data-model.md#12-論理型)）。
  ハンドラが `error` を返した場合は 500 で、内部の詳細は応答に含めずログに記録します。
- どの操作にも一致しない `/api/v1` 以下のリクエストは、`platform` のフォールバックが 404 / 405 の問題を返します。

### 2.4 キャッシュ・ヘッダ・CORS

- 匿名で取れる公開リソースの GET には `Cache-Control: public, max-age=60` を付けます（spec の `#/components/headers/CacheControl`）。
  利用者ごとに変わる応答には `private` か `no-store` を付けます。問題の応答は `no-store` です。
  匿名でも使える操作にトークン付きで来た要求は利用者のロケールで答えるので、`public` の代わりに `private, max-age=60` を付けます。
- `/api/v1` のすべての応答はローカライズされうるので、`Vary: Accept-Language` を付けます（`localize`）。
  `Vary` は spec の応答ヘッダに書きません（生成コードが `Set` で上書きし、CORS の `Vary: Origin` を消すため）。
- 匿名でも使える操作の応答はトークンの有無で変わるので、`guard` がすべての操作の応答に `Vary: Authorization` を付けます（匿名の応答をキャッシュがログイン中の要求に返さないため）。
- ADR-0019 §9 のセキュリティヘッダは `platform` が全応答に付けます。
- CORS は `platform` が `/api/v1` 以下に適用し、設定 `web_origin` のオリジンだけを `Access-Control-Allow-Origin` に返します。
  `Access-Control-Allow-Credentials` は付けません（API は `Authorization: Bearer` だけで認証する）。
  `Content-Language`・`X-Request-Id`・`WWW-Authenticate`・`Retry-After`・`RateLimit`・`RateLimit-Policy` を `Access-Control-Expose-Headers` で公開します。

### 2.5 使える OpenAPI 3.1 の部分集合

oapi-codegen と openapi-typescript が扱える範囲に限ります。`api/redocly.yaml` の独自ルールが次を禁止します。

- JSON Schema のキーワード: `$dynamicRef`、`$dynamicAnchor`、`$anchor`、`$defs`、`if` / `then` / `else`、`prefixItems`、`contains`、
  `minContains`、`maxContains`、`dependentSchemas`、`dependentRequired`、`unevaluatedProperties`、`unevaluatedItems`、
  `patternProperties`、`propertyNames`、`contentEncoding`、`contentMediaType`、`contentSchema`、`not`
- 配列の `type` と `null` 型。値のないプロパティは `null` にせず、オブジェクトから省きます（`required` に入れない）。
- `webhooks`、`jsonSchemaDialect`、`callbacks`

## 3. エンドポイントを追加する手順

1. `api/openapi.yaml` に操作を書きます。`operationId`、`summary`、`tags`、成功時の応答と `default: {$ref: "#/components/responses/Problem"}` を必ず書きます。
   `security` は §4.2 の 3 つの形のどれかで書き、アクセストークンを受け付ける操作は必要なスコープを必ず挙げます（挙げないと `make generate` が失敗します）。
2. `make api-lint` で spec を検査します。
3. `make generate` で `internal/interface/clientapi/api.gen.go`・`security.gen.go` と Web クライアントの `app/web/src/lib/api/schema.gen.ts` を再生成します。`StrictServerInterface` に新しいメソッドが増え、未実装のためビルドが失敗します。
   main を取り込んだあとも、生成物は手でマージせず `make generate` で作り直します。
4. `internal/interface/clientapi` にメソッドを実装します。アプリケーション層（`internal/app/<module>`）のユースケースを呼び、
   ドメインの型を生成された応答型に詰め替えます。必要な依存は `clientapi.Options` に追加し、`platform.NewApp` で渡します。
   閲覧者は `requireViewer(ctx)`（トークン必須の操作）か `ViewerFrom(ctx)`（匿名も受け付ける操作）で取ります。スコープの検査はハンドラの前に済んでいます。
   アカウントは `Server.newAccount` で `Account` に、閲覧者自身のアカウントは `Server.newCredentialAccount` で `CredentialAccount`（プロフィールの入力ソースと、`Options.Posting` から読む投稿の制限 `posting` を含む）に、
   投稿は `appcontent.Reader` が閲覧者に見せる形にした `StatusView` を `Server.newStatus` で `Status` に詰め替えます。
   `Account` のアバター・ヘッダーの URL（`avatar_url` / `header_url`）と、リモートのアカウントのプロフィールページ（`url`。Actor の `url`、無ければ `uri`）は、
   応答に含めるアカウントの分を `Server.accountParts`（投稿なら著者とメンション先を加える `Server.statusParts`）で
   1 回にまとめて読み、`newAccount` と `newStatus` に渡します。アカウントごとに読まない（N+1 にしない）ため、一覧は `Server.newAccounts` や `Server.presentStatuses` を使います。
   フォロー数を含めるのはアカウント自身の取得だけで、`Server.newProfile` を使います（`newCredentialAccount` もこれに重ねます）。
   ID で指定されたアカウントは `Server.visibleAccount` で引き、見せられないアカウント（[visibility.md §6](visibility.md#6-アカウントの表示)）は存在しないものと同じ 404 にします。
5. 新しい利用者向けの文言は `locales/en` と `locales/ja` の `server` 名前空間に追加し、`make i18n-gen` を実行します。
6. テストは `apitest.Load(t).Serve(t, handler, req)` を通してリクエストを送ります。応答の状態コード・`Content-Type`・本文が spec と一致しない場合、テストが失敗します。
   不正なリクエストを送るテストもそのまま使えます（`Serve` はリクエストを照合しません。照合するときは `ValidateRequest`）。
   正常系に加えて、問題の応答（`Accept-Language` によるローカライズを含む）を検査します。
   トークンが要る操作は `auth_test.go` の `newAuthEnv`（偽の `Authenticator`・`Accounts` と停止した時計のレート制限）で、トークンなし・スコープ不足も検査します。
   `TestEveryOperationIsGuarded` が spec のすべての操作に匿名とスコープなしのトークンで要求を送り、401 / 403 になることを確かめます。
7. `make check` を実行します。生成物の差分検査（`generate-check`）と spec の lint（`api-lint`）が含まれます。

## 4. 認可（OAuth 2.0）

クライアントは OAuth 2.0 Authorization Code + PKCE でアクセストークンを得て、`/api/v1` に `Authorization: Bearer` で送ります（[ADR-0019 §4](../adr/0019-security-baseline.md)、[ADR-0030](../adr/0030-web-client-token-handling.md)）。
認可サーバーのエンドポイントは `/api/v1` の外にあり、OpenAPI の spec には含めません。

| エンドポイント | 実装 | 役割 |
| --- | --- | --- |
| `GET` / `POST /oauth/authorize` | `internal/interface/authpages` | 認可エンドポイント。ログインしていなければログイン画面を経由して戻る。サードパーティのクライアントには同意画面を出し、ファーストパーティは省略する |
| `POST /oauth/token` | `internal/interface/oauth` | `authorization_code` と `refresh_token` のグラント。Cookie は読まない |
| `POST /oauth/token/refresh` | `internal/interface/oauth` | Web クライアント専用。`__Host-my_rt` Cookie だけを読み、新しいアクセストークンだけを返す |
| `POST /oauth/revoke` | `internal/interface/oauth` | トークンの失効（RFC 7009）。トークンの系列全体を失効させる |

- PKCE は `S256` だけを受け付けます。`redirect_uri` は認可リクエストで必須で、登録済みの値と完全一致で照合します。
- 認可応答は `303` のリダイレクトで、`code`（または `error`）・`state`・`iss`（RFC 9207）を付けます。
  `iss` はサーバーのオリジンです。`https://{host}` で、`web_origin` が `host` 上の `http` のオリジン（`http://localhost:8080` など、平文の HTTP のローカル構成）のときだけ `http://{host}` です。
- トークンには種別プレフィックス（アクセストークン `my_at_`、リフレッシュトークン `my_rt_`、認可コード `my_ac_`）が付きます。
  期限は認可コード 60 秒、アクセストークン 1 時間、リフレッシュトークン 30 日です。
- スコープは §4.1 のとおりです。認可リクエストで省くと、クライアントに許されたスコープ全体（ファーストパーティの Web クライアントは `admin`・`read`・`write`・`follow`・`push`）になります。
- ファーストパーティの Web クライアントは起動時に登録されます。`client_id` は `my-web`、リダイレクト URI は `<web_origin>/login/callback` です。
  - `POST /oauth/token` での認可コードの交換では、リフレッシュトークンを本文ではなく `__Host-my_rt` Cookie（`HttpOnly`、`Secure`、`SameSite=Strict`、`Path=/`）で返します。
    この交換は `net/http.CrossOriginProtection` を通る要求だけを受け付けます。
  - `refresh_token` グラントは使えません。`POST /oauth/token/refresh` に `X-My-Token-Refresh: 1` ヘッダを付けて呼びます。応答のたびに Cookie はローテーションされます。
    使用済みのリフレッシュトークンを送ると系列全体が失効するので、同時に複数の更新を送らないようにします（タブ間で更新を 1 つにまとめる）。
  - `POST /oauth/revoke` で失効させると、Cookie も削除されます。Web クライアントの失効はログアウトなので、その系列の認可コードを発行した
    認可サーバーの画面のセッション（`__Host-my_session`）も `logout` で失効させます。Cookie ではなく、認可コードに記録したセッションから辿ります。
- `web_origin` がサーバーのオリジンと異なる場合、これらのエンドポイントは `web_origin` にだけ資格情報付きの CORS を許可します。

### 4.1 スコープ

スコープは広いスコープ（`admin`・`read`・`write`・`follow`・`push`）と細かいスコープ（`read:accounts`、`write:statuses` など）の 2 段です。
一覧は spec の `#/components/securitySchemes/oauth2` と `internal/domain/identity` の `KnownScopes` で、両者が一致することをテストが確かめます。

- `read` はすべての `read:*` を、`write` はすべての `write:*` を、`follow` は `read:follows`・`read:blocks`・`read:mutes`・`write:follows`・`write:blocks`・`write:mutes` を、`admin` は `admin:read`・`admin:write` を許します（`identity.Scope.Grants`）。
  `read`・`write` は管理のスコープを許しません。
  `push` は Web Push だけです。
- 操作には細かいスコープを書きます（`verifyAccountCredentials` は `read:accounts`）。クライアントは広いスコープでも細かいスコープでも要求できます。
- スコープを増やすときは、`internal/domain/identity` の定数と `knownScopes`（ソート済み）、spec の `scopes`、同意画面の文言（`server:auth.authorize.scope.*` と `authpages` の `scopeDescriptions`）を同時に変えます。

### 4.2 `/api/v1` の認証・認可・レート制限

`Server.Register` は生成コードが登録する各操作のハンドラを `guard` で包みます。`guard` はリクエストの解釈より前に、次の順で動きます。

1. **認証**: `Authorization` ヘッダだけを読みます。Cookie とクエリ文字列は読みません（ADR-0019 §4。`TestCookiesNeverAuthenticate`）。
   ヘッダがなければ匿名です。あれば `Bearer <token>` でなければならず、`identity.Service.AuthenticateBearer` が照合します。
   照合はトークン（ハッシュ・失効・期限）に加えて、利用者が `Access.Allowed`（登録が承認済みで、必要ならメールが確認済みで、ログインを奪う処分を受けていない）であることと、ローカルアカウントが有効なことを要求ごとに確かめます。
   トークンを送った要求は、匿名を受け付ける操作でも必ず照合し、無効なら 401 にします（匿名として扱わない）。
2. **閲覧者**: 照合できたら `Viewer`（`UserID`・`AccountID`・`TokenID`・`ClientID`・`FamilyID`（トークン系列。Web Push の購読はこれに結び付く）・`Scope`）を context に入れます。
   `platform` の `i18n.Resolver.UserLocale` は `ViewerFrom` で閲覧者を見つけ、利用者設定のロケールを使います（ADR-0009 §2）。
3. **レート制限**: 閲覧者があればアカウント単位（`AccountLimit`、1,500 / 5 分）、なければクライアント IP 単位（`AnonymousLimit`、300 / 5 分、IPv6 は `/64`）です。
   無効なトークン・使えないアカウントの要求も IP 単位で数えます。カウンタは `KeyValueStore` 上の `kernel.RateLimiter` で、ストアが失敗したときは要求を通してログに残します。
   クライアント IP は `kernel.RequestClientIP` で、`platform` のミドルウェアが接続元と信頼するプロキシ（設定 `trusted_proxies`）の転送ヘッダから決めます
   （[deployment-targets.md](deployment-targets.md#信頼するプロキシとクライアント-ip)）。
4. **認可**: `security.gen.go` の操作の表を引き、トークンが要る操作に閲覧者がなければ 401、スコープが足りなければ 403 にします。
   表にない操作は `Register` が起動時に panic するので、検査されないまま公開されることはありません。
5. **本文の上限**: リクエスト本文は 1 MiB まで、`uploadMedia` だけはアップロードの上限（`media_max_image_bytes`）に 64 KiB を足した大きさまでです。
   `Content-Length` が上限を超える要求は本文を読まずに 413 にし、長さのない要求は `http.MaxBytesReader` で上限で打ち切ります（413）。
   `uploadMedia` の 413 は `/api/v1/problems/media-too-large`、ほかは `about:blank` です。

`security` の書き方は 3 つです（`internal/securitygen` がこれ以外を拒否します）。

| spec | 意味 |
| --- | --- |
| `security: []` | 匿名で使える。トークンが送られれば照合し、閲覧者を入れる |
| `security: [{oauth2: [read:accounts]}]` | トークンが必須で、列挙したスコープをすべて要する |
| `security: [{}, {oauth2: [read:statuses]}]` | 匿名でも使える。トークンが送られたら列挙したスコープを要する |

管理の操作（タグ `admin`、`/admin/...`）は、スコープ（`admin:read` / `admin:write`）に加えて、利用者のロールが操作の権限（`manage_domains`、`manage_jobs` など。[data-model §4.1](data-model.md)）を持つことを要します。
ハンドラが最初に `Server.requirePermission` で `Options.Permissions`（`identity.Service.HasPermission`）に問い合わせ、持たなければ 403 にします。
権限は spec の `security` に書けないので、操作の `description` に書きます。
クライアントが管理の画面を出し分けられるように、`GET /accounts/verify_credentials` の `CredentialAccount.permissions` が利用者のロールの権限を返します（`identity.Service.Permissions`。ロールがなければ空）。画面を隠すのは案内のためだけで、守るのは各操作の検査です。

応答:

| 状況 | 状態 | `type` | `WWW-Authenticate`（RFC 6750 §3） |
| --- | --- | --- | --- |
| トークンが要るのに `Authorization` がない | 401 | `/api/v1/problems/unauthenticated` | `Bearer` |
| `Bearer` でない、形式が違う、未知・失効・期限切れのトークン | 401 | `/api/v1/problems/invalid-token` | `Bearer error="invalid_token"` |
| トークンは有効だが利用者がアプリを使えない（承認待ち・却下・メール未確認・停止・削除） | 403 | `/api/v1/problems/account-unavailable` | なし |
| スコープ不足 | 403 | `/api/v1/problems/insufficient-scope` | `Bearer error="insufficient_scope", scope="<操作のスコープ>"` |
| 管理の操作で、利用者のロールに権限がない | 403 | `/api/v1/problems/missing-permission` | なし |
| レート制限の超過 | 429 | `about:blank` | なし（`Retry-After` を付ける） |

すべての応答に `RateLimit-Policy` と `RateLimit`（[draft-ietf-httpapi-ratelimit-headers](https://datatracker.ietf.org/doc/draft-ietf-httpapi-ratelimit-headers/) の Structured Fields 形式）を付けます。
ポリシー名は `"ip"` と `"account"` で、例えば `RateLimit-Policy: "account";q=1500;w=300` と `RateLimit: "account";r=1499;t=300` です
（`q` は窓あたりの上限、`w` は窓の秒数、`r` は残り、`t` は窓が終わるまで、超過時は次の要求が通るまでの秒数）。

操作ごとの制限（投稿 300 / 3 時間、アップロード 30 / 30 分など ADR-0019 §5 の表と、`lookupAccount` の `resolve` によるリモートのアカウントの取得 30 / 5 分、`createReport` の通報 30 / 1 時間）は、ハンドラが `Server.LimitProblem(ctx, limit, v.AccountID)` を呼び、
超過なら返された 429 の問題（`Retry-After` 付き）を `default` 応答で返します。制限は `kernel.Limit` の変数として `clientapi` に置きます。

## 5. ストリーミング

`GET /api/v1/streaming`（`streamEvents`）は Server-Sent Events のストリームで、閲覧者のホームの新着と新しい通知を送ります（FR-207）。
strict server は SSE を返せないので、`oapi-codegen.yaml` の `exclude-operation-ids` で生成から除き、`streaming.go` に手で書きます（ADR-0029）。
それでも spec には `text/event-stream` の操作として書き、`security.gen.go` の表と `guard`（認証・レート制限・スコープ `read:statuses` と `read:notifications`）を通します。

- クライアントは `fetch` のストリームで読み、`Authorization: Bearer` を送ります（`EventSource` はヘッダを送れません）。
- イベントは `update`（`data` は `Status`）と `notification`（`data` は `Notification`）です。`id` は付けず、`Last-Event-ID` での再開はしません。
  切れたら再接続し、ホームと通知を読み直します。
- 接続の流れは [overview §4.4](overview.md#44-リアルタイム更新sse) のとおりです。ハンドラは `stream.Subscriber` で購読し、知らせを受けるたびにユースケース
  （`HomeEntries.HomeEntry`、`Notifications.List`）で閲覧者に見せる形に読み直して、`Server.statuses` と `Server.newNotifications` で詰め替えます。
- 開くと `: connected` を、その後 30 秒ごと（`Options.StreamHeartbeat`）に `: heartbeat` のコメント行を送ります。ハートビートのたびにアクセストークンを照合し直し、無効なら閉じます。
- 接続数の上限は、アカウントごとに 8、プロセス全体で 1,000 です（`stream.HubOptions`）。上限はプロセスごとに数えるので、プロセス間のブローカー（設定 `stream`）を使う構成では、全体では「上限 × web のプロセス数」まで開けます。
  アカウントの上限を超えると 429（`/api/v1/problems/too-many-streams`）です。
- ブローカーが購読を受けられないとき（`stream.ErrUnavailable`: `stream.Relay` の listener が 10 秒以内に準備できない、または開始に失敗した）と
  シャットダウン中（`stream.ErrClosed`）は、503 と `Retry-After`（5 秒）を返します。購読は listener の準備を待つので、プロセスで最初のストリームは開くのが遅れることがあります。
- プロセスをまたぐブローカーは同じ知らせを 2 度届けることがあるので、各ストリームは最近送ったホームの新着の ID（512 件）を覚え、同じ ID を 2 度送りません。
  通知は「前回送ったものより新しい一覧」を読み直すので、重複しても同じ通知を 2 度送りません。
- 後始末: クライアントの切断・書き込みのタイムアウト（10 秒）・購読の終了で、ハンドラは戻って購読を閉じます。
  読みきれずにバッファ（64 件）が溢れた購読と、知らせを取りこぼしたかもしれない購読（`Relay` の listener が `Broken` になった）はブローカーが閉じます。サーバーの読み取りタイムアウトは接続ごとに外し、
  シャットダウンの開始時にブローカー（`Hub` か `Relay`）を閉じてすべての購読を閉じるので、グレースフルシャットダウンはストリームを待ちません。
- テストは、実際の HTTP サーバーでストリームを読み、状態と見出しを `apitest.Spec.ValidateResponse` で、各イベントの `data` を `ValidateSchema` で spec と照合します。
