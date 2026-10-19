# ボット対策の CAPTCHA（Turnstile / hCaptcha / reCAPTCHA）

運営者向けの手順です。決定は [ADR-0042](../adr/0042-captcha-human-verifier.md)、設定のキーは [設定リファレンス](../reference/configuration.md#captcha) です。

CAPTCHA は既定では使いません（`captcha = none`）。そのときサインインと登録の画面は JavaScript なしで動き、第三者には何も送りません。
設定で CAPTCHA のサービスを選ぶと、サインイン（`/auth/sign_in`）と登録（`/auth/sign_up`）の 2 つのフォームにサービスのウィジェットを出し、
送られてきたトークンをサーバーがサービスの siteverify API で検証します。片方のフォームだけにかける設定はありません。

## サービスを選ぶ

| `captcha` | サービス | 費用 | 利用者の操作 |
| --- | --- | --- | --- |
| `turnstile`（推奨） | Cloudflare Turnstile | 無料（上限なし） | 多くはなし。必要なときだけチェックボックス |
| `hcaptcha` | hCaptcha | 無料枠あり | チェックボックスと、必要なときに画像の課題 |
| `recaptcha` | Google reCAPTCHA v2（チェックボックス） | 月 10,000 評価まで無料（下記） | チェックボックスと、必要なときに画像・音声の課題 |

reCAPTCHA v3（スコアだけ）と reCAPTCHA Enterprise には対応しません。

## 鍵を作る

どのサービスでも、公開の **サイトキー** と、サーバーだけが持つ **秘密鍵** の組を作ります。登録するホスト名（ドメイン）には設定 `host` のホスト名（ポートを除く）を入れます。
サーバーは siteverify の応答のホスト名が `host` と一致しないトークンを拒否します。

- **Turnstile**: Cloudflare のダッシュボードの「Turnstile」でウィジェットを追加し、ホスト名を登録します。ウィジェットのモードは「Managed」を選びます。
- **hCaptcha**: hCaptcha のダッシュボードでサイトを追加し、ホスト名を登録します。秘密鍵はアカウントの設定にあります。
- **reCAPTCHA**: Google の reCAPTCHA 管理コンソールで、種類に「チャレンジ（v2）」の「私はロボットではありません」チェックボックスを選び、ドメインを登録します。
  鍵は Google Cloud のプロジェクトに属します。

各サービスのテスト用の鍵（常に成功する鍵）は、ウィジェットが表示されてトークンができることの確認には使えます。
ただしテスト用の鍵の siteverify は固定のホスト名（Turnstile は `example.com`、hCaptcha は `dummy-key-pass` など）を返すので、このサーバーでのサインインと登録は「確認をやり直してください」で失敗します。

## 設定する

```sh
MY_CAPTCHA=turnstile
MY_CAPTCHA_SITE_KEY=0x4AAAAAAA...
MY_CAPTCHA_SECRET_KEY_FILE=/run/secrets/my_captcha_secret_key
```

- 秘密鍵は環境変数 `MY_CAPTCHA_SECRET_KEY` か、ファイルを指す `MY_CAPTCHA_SECRET_KEY_FILE` だけで渡せます。ログには `[REDACTED]` と出ます。
- `captcha` を選んで鍵が欠けていると、`my config check` が問題として挙げ、サーバーは起動しません。
- サーバーは起動時に鍵を試しません。サービスが秘密鍵やサイトキーを拒否すると、最初の検証で ERROR のログ
  「the CAPTCHA service refused the secret key or the site key」を出します。直すまでは下の「サービスに届かないとき」と同じ扱いになります。
- 変更はサーバーの再起動で効きます。

## 検証の流れ

- 順序はレート制限 → CAPTCHA の検証 → パスワードの照合・登録の処理です。レート制限を超えた要求はサービスに送りません。
- 有効な招待コードでの登録でも検証します。
- 検証に失敗したサインインは、接続元の IP アドレスのログイン失敗として数えます（アカウントの失敗としては数えないので、他人のアカウントを締め出せません）。
  検証に失敗した登録は、登録の試行として数えます（[利用者登録](registration.md)のレート制限）。
- Turnstile ではウィジェットに操作の名前（`sign_in` / `sign_up`）を付け、応答の操作と一致することも確かめます。あるフォームで得たトークンを別のフォームでは使えません。
- サーバーは利用者の IP アドレス（`remoteip`）をサービスに送りません。トークンは 1 回限りなので再試行もしません。タイムアウトは 10 秒です。

### サービスに届かないとき

サービスに届かない、タイムアウトした、5xx を返した、鍵を拒否したときの扱い:

| 画面 | 扱い |
| --- | --- |
| 登録 | 拒否し（`503`）、「しばらくしてからやり直してください」と表示します |
| サインイン | CAPTCHA なしで通し、WARN のログ「the CAPTCHA service could not check a sign-in; letting it through」を出します。レート制限とパスワードの照合はそのまま行います |

サインインを通すのは、サービスの障害や鍵の誤りで管理者を含む全員が締め出されないためです。WARN が続くときはサービスの状態と鍵を確かめてください。

### reCAPTCHA の無料枠

reCAPTCHA は月 10,000 評価の無料枠を超えると、siteverify が **常に成功** を返します（その間は CAPTCHA の保護がありません）。
このサーバーは超過を検出しないので、Google Cloud のコンソールで使用量を確かめてください。

## 画面と Content-Security-Policy

- ウィジェットの言語はページのロケールに、テーマはブラウザーの配色（ライト / ダーク）に合わせます。
  Turnstile はウィジェット自身が配色に従います。hCaptcha と reCAPTCHA はブラウザーが送るクライアントヒント `Sec-CH-Prefers-Color-Scheme` で選び、送らないブラウザーではライトになります。
- ウィジェットの近くに、どのサービスで確認しているか、サービスに IP アドレスとブラウザーの情報が渡ることと、サービスのプライバシーポリシー（hCaptcha と reCAPTCHA では利用規約も）へのリンクを出します。
  reCAPTCHA では Google が求める「This site is protected by reCAPTCHA and the Google Privacy Policy and Terms of Service apply.」の文を兼ねます。
- JavaScript が動かないブラウザーには、確認に JavaScript が要ることを `<noscript>` で知らせます。CAPTCHA を解けない利用者は登録もサインインもできません。
  その人には運営者が別の手段で対応する（`my admin create` で作る、一時的に `captcha = none` にするなど）ほかありません。
- CSP（`Content-Security-Policy`）が広がるのは、CAPTCHA を有効にしたときのサインインと登録のページだけです。`'unsafe-inline'` と `'unsafe-eval'` は足しません。
  Web クライアント・クライアント API・同意画面・メール確認などのページの CSP は変わりません。

| `captcha` | 足すソース |
| --- | --- |
| `turnstile` | `script-src`・`frame-src`: `https://challenges.cloudflare.com` |
| `hcaptcha` | `script-src`・`frame-src`・`style-src`・`connect-src`: `https://hcaptcha.com`, `https://*.hcaptcha.com` |
| `recaptcha` | `script-src`: `https://www.google.com/recaptcha/`, `https://www.gstatic.com/recaptcha/`／`frame-src`: `https://www.google.com/recaptcha/`, `https://recaptcha.google.com/recaptcha/`／`connect-src`: `https://www.google.com/recaptcha/` |

サーバーの前段のリバースプロキシや CDN で CSP を上書きしている場合は、この 2 つのページに同じソースを足してください。

## プライバシーポリシーに書くこと

CAPTCHA を有効にすると、サインインと登録のページを開いた人のブラウザーがサービスに直接つながり、サービスは IP アドレス・ブラウザーの情報・Cookie を受け取ります。
インスタンスのプライバシーポリシー（`my instance set privacy_policy`、[インスタンス設定](instance-settings.md)）は運営者が書くもので、サーバーは自動では書き足しません。少なくとも次を書いてください。

- どのサービス（事業者の名前）を使っているか、その目的（自動化されたサインインと登録の防止）
- サービスに渡る情報（IP アドレス、ブラウザーと端末の情報、Cookie、ウィジェットの操作）と、それを受け取るのがサインインと登録のページだけであること
- サービスのプライバシーポリシーへのリンク（Turnstile: <https://www.cloudflare.com/privacypolicy/>、hCaptcha: <https://www.hcaptcha.com/privacy>、reCAPTCHA: <https://policies.google.com/privacy>）
- reCAPTCHA では、Google のプライバシーポリシーと利用規約（<https://policies.google.com/terms>）が適用されること
