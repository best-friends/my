# クライアント API の規約と追加手順

独自クライアント API（`/api/v1`）を設計・実装するときの規約と、エンドポイントを追加する手順です。
根拠は [ADR-0007](../adr/0007-client-api.md)（独自 API）、[ADR-0016](../adr/0016-client-api-style.md)（様式）、[ADR-0019](../adr/0019-security-baseline.md)（ヘッダ・CORS）、[ADR-0029](../adr/0029-api-code-generation.md)（コード生成）です。

## 1. ファイルの配置

| ファイル | 役割 |
| --- | --- |
| `api/openapi.yaml` | API の正。OpenAPI 3.1 の単一ファイル |
| `api/redocly.yaml` | スキーマの lint 設定（Redocly CLI）。推奨ルールと、使える 3.1 の部分集合・命名規約を強制する独自ルール |
| `api/api.go` | `openapi.yaml` を埋め込む Go パッケージ（`api.Spec`）。テストの照合ヘルパーが読む |
| `internal/interface/clientapi/oapi-codegen.yaml` | oapi-codegen の設定（strict server、`net/http`） |
| `internal/interface/clientapi/api.gen.go` | 生成物。手で編集しない |
| `internal/interface/clientapi/*.go` | 生成された `StrictServerInterface` の実装。アプリケーション層のユースケースを呼ぶ |
| `internal/interface/clientapi/apitest` | ハンドラの応答を spec と照合するテストヘルパー |
| `scripts/generate.sh` | 生成スクリプト（`make generate` / `make generate-check`） |

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
  応答は `allOf` で `#/components/schemas/PageInfo` を含めて `next_cursor` を返します。最後のページでは `next_cursor` を省きます。
- カーソルは不透明な文字列です。クライアントは解釈せずにそのまま返します。

### 2.3 エラー

- すべての操作は `default` 応答で `#/components/responses/Problem` を参照します（lint で強制）。
  本文は RFC 9457 の `application/problem+json` です。
- 状態コード以上の意味を持たない問題は `type: about:blank` です。クライアントが分岐に使う問題には、安定した `type` の URI を割り当てます。
- `title` と `detail` は `Accept-Language` に応じて `internal/i18n` の `server` 名前空間からローカライズし、`Content-Language` を返します。
  文言は `locales/en` と `locales/ja` に追加して `make i18n-gen` を実行し、生成された型付きアクセサ（`l.ServerErrors...()`）で整形します。
- ハンドラが返す問題は `Server.NewProblem` で作り、その操作の `default` 応答型（`<Operation>defaultApplicationProblemPlusJSONResponse`）で返します。
  ローカライザは `Server.Localizer(ctx)` で取得します。
- 生成コードが解釈できないリクエスト（パラメータの形式違いなど）は 400 になり、`detail` がパラメータ名を示します。
  ハンドラが `error` を返した場合は 500 で、内部の詳細は応答に含めずログに記録します。
- どの操作にも一致しない `/api/v1` 以下のリクエストは、`platform` のフォールバックが 404 / 405 の問題を返します。

### 2.4 キャッシュ・ヘッダ・CORS

- 匿名で取れる公開リソースの GET には `Cache-Control: public, max-age=60` を付けます（spec の `#/components/headers/CacheControl`）。
  利用者ごとに変わる応答には `private` か `no-store` を付けます。問題の応答は `no-store` です。
- ADR-0019 §9 のセキュリティヘッダは `platform` が全応答に付けます。
- CORS は `platform` が `/api/v1` 以下に適用し、設定 `web_origin` のオリジンだけを `Access-Control-Allow-Origin` に返します。
  `Access-Control-Allow-Credentials` は付けません（API は `Authorization: Bearer` だけで認証する）。

### 2.5 使える OpenAPI 3.1 の部分集合

oapi-codegen と openapi-typescript が扱える範囲に限ります。`api/redocly.yaml` の独自ルールが次を禁止します。

- JSON Schema のキーワード: `$dynamicRef`、`$dynamicAnchor`、`$anchor`、`$defs`、`if` / `then` / `else`、`prefixItems`、`contains`、
  `minContains`、`maxContains`、`dependentSchemas`、`dependentRequired`、`unevaluatedProperties`、`unevaluatedItems`、
  `patternProperties`、`propertyNames`、`contentEncoding`、`contentMediaType`、`contentSchema`、`not`
- 配列の `type` と `null` 型。値のないプロパティは `null` にせず、オブジェクトから省きます（`required` に入れない）。
- `webhooks`、`jsonSchemaDialect`、`callbacks`

## 3. エンドポイントを追加する手順

1. `api/openapi.yaml` に操作を書きます。`operationId`、`summary`、`tags`、成功時の応答と `default: {$ref: "#/components/responses/Problem"}` を必ず書きます。
   認証が要らない操作は `security: []` を書きます（既定は Bearer）。
2. `make api-lint` で spec を検査します。
3. `make generate` で `internal/interface/clientapi/api.gen.go` を再生成します。`StrictServerInterface` に新しいメソッドが増え、未実装のためビルドが失敗します。
4. `internal/interface/clientapi` にメソッドを実装します。アプリケーション層（`internal/app/<module>`）のユースケースを呼び、
   ドメインの型を生成された応答型に詰め替えます。必要な依存は `clientapi.Options` に追加し、`platform.NewApp` で渡します。
5. 新しい利用者向けの文言は `locales/en` と `locales/ja` の `server` 名前空間に追加し、`make i18n-gen` を実行します。
6. テストは `apitest.Load(t).Serve(t, handler, req)` を通してリクエストを送ります。応答の状態コード・`Content-Type`・本文が spec と一致しない場合、テストが失敗します。
   不正なリクエストを送るテストもそのまま使えます（`Serve` はリクエストを照合しません。照合するときは `ValidateRequest`）。
   正常系に加えて、問題の応答（`Accept-Language` によるローカライズを含む）を検査します。
7. `make check` を実行します。生成物の差分検査（`generate-check`）と spec の lint（`api-lint`）が含まれます。
