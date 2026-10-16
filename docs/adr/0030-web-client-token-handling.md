# ADR-0030: Web クライアントのトークンの扱いとログイン画面

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #90

## Context

ADR-0019 §4 は、Web クライアントのアクセストークンをメモリに、リフレッシュトークンを IndexedDB に置くと定めている。
2026 年に発行された RFC 10017（OAuth 2.0 for Browser-Based Applications、BCP 212）は、この「ブラウザだけで完結する」方式を最も弱いものと位置づけている。
そのうえで、ブラウザのストレージ（localStorage / sessionStorage / IndexedDB）はトークンの持ち出しを防げないとしている。
XSS が 1 度あれば、30 日有効のリフレッシュトークンを盗まれる。

## Considered Options

1. 現状のまま（リフレッシュトークンを IndexedDB）+ DPoP
2. **トークンの仲介（token-mediating backend）**: リフレッシュトークンを HttpOnly の Cookie に置き、1 つのエンドポイントだけでそれを受け付ける
3. 完全な BFF（ブラウザにはトークンを渡さない）。ADR-0019 が Tauri との共通化を崩すとして採らなかった方式

## Decision（推奨案）

**2 を採用し、ADR-0019 §4 の「Web クライアントでのトークン保管」の項目を置き換える。**

- **リフレッシュトークン**: Web クライアントでは、`__Host-my_rt` Cookie（`HttpOnly`、`Secure`、`SameSite=Strict`、`Path=/`）に入れる。
- **Cookie を受け付けるのは `POST /oauth/token/refresh` だけにする。** このエンドポイントは、短命のアクセストークンだけを返す。
  - `net/http.CrossOriginProtection` と、必須の独自ヘッダで保護する。
- **アクセストークン**: メモリにだけ置く。`/api/v1` は引き続き Bearer トークンだけで認証する（CSRF の対象外という前提を保つ）。
- **Tauri**: リフレッシュトークンを OS のセキュアストレージに置く。API クライアントのコードは Web と共通で、「リフレッシュトークンの取得」部分だけを差し替える。
- **ログイン・登録・認可同意の画面は、Go が `html/template` で返すフォームにする**（OAuth の認可サーバーの画面）。
  - SPA は Authorization Code + PKCE でこの画面に遷移し、コールバックでコードをトークンに交換する。
  - 画面のスタイルには、Web クライアントのビルドで生成した CSS を共用する。文言は i18n のカタログ（`server` 名前空間）から引く。
  - JavaScript がなくても動くようにする。

## Consequences

- XSS でリフレッシュトークンを直接盗まれることはなくなる。アクセストークンは盗まれ得るが、有効期限は短い。
- SPA と API が同じサイトにあることが前提になる（Go が HTML の外枠を返す ADR-0028 の構成では満たされる）。
- ADR-0019 本体は書き換えない。この ADR を Accepted にするときに、ADR-0019 §4 の該当項目からこの ADR へ参照を追記する。
