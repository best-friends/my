# ADR-0042: サインインと登録のボット対策に CAPTCHA を設定で使えるようにする

- Status: Accepted
- Date: 2026-10-19
- Deciders: @rosylilly
- Issue: #490

## Context（背景・課題）

決定者のフィードバック（#490）: 「登録・ログイン時、API キーが設定されており有効な設定なら reCaptcha を利用できるようにしたい」。

いまのサインインと登録は、サーバーが描画する HTML のフォーム（`internal/interface/authpages`、`/auth/sign_in`・`/auth/sign_up`）だけが受け付ける。
クライアント API には登録・サインインのエンドポイントがない（[ADR-0030](0030-web-client-token-handling.md)）。
ボットへの対策としては、IP とアカウントごとのレート制限（[ADR-0019](0019-security-baseline.md) §5）、登録の承認制・招待制、登録の拒否リスト（signup blocks）がある。
レート制限は 1 つの IP からの総当たりは止めるが、多数の IP から少しずつ来るスパム登録やパスワードスプレーは止めない。

決めること:

- どの CAPTCHA サービスに対応するか（reCAPTCHA v2 / v3 / Enterprise、hCaptcha、Cloudflare Turnstile、自前ホストの proof-of-work）と、その形（ポートとアダプター、[ADR-0002](0002-modular-monolith-hexagonal.md)）
- 設定と鍵の扱い（秘密鍵は秘密、ADR-0019 §6、[設定リファレンス](../reference/configuration.md)）
- サーバー側の検証（外部 HTTP は SSRF ガードの `HTTPClient` を通す、タイムアウト、検証できないときの扱い）
- CSP の変更、プライバシー（第三者へのデータ送信、#489 のプライバシーポリシー）、アクセシビリティ、自前ホストの環境での扱い

各サービスの仕様（2026-10-19 に公式文書で確認）:

| | Cloudflare Turnstile | hCaptcha | reCAPTCHA v2（チェックボックス） |
| --- | --- | --- | --- |
| 検証 API | `POST https://challenges.cloudflare.com/turnstile/v0/siteverify` | `POST https://api.hcaptcha.com/siteverify` | `POST https://www.google.com/recaptcha/api/siteverify` |
| 送るもの | `secret`, `response`, 任意で `remoteip`, `idempotency_key` | `secret`, `response`, 任意で `remoteip`, `sitekey` | `secret`, `response`, 任意で `remoteip` |
| 返るもの | `success`, `challenge_ts`, `hostname`, `error-codes`, `action`, `cdata` | `success`, `challenge_ts`, `hostname`, `error-codes` | `success`, `challenge_ts`, `hostname`, `error-codes` |
| フォームの欄 | `cf-turnstile-response` | `h-captcha-response` | `g-recaptcha-response` |
| トークン | 1 回限り、発行から 300 秒 | 1 回限り、既定 120 秒 | 1 回限り、発行から 2 分（要検証） |
| CSP に足すオリジン | `https://challenges.cloudflare.com`（`script-src`, `frame-src`） | `https://hcaptcha.com`, `https://*.hcaptcha.com`（`script-src`, `frame-src`, `style-src`, `connect-src`） | `https://www.google.com/recaptcha/`, `https://www.gstatic.com/recaptcha/`（`script-src`）、`https://www.google.com/recaptcha/`, `https://recaptcha.google.com/recaptcha/`（`frame-src`）、`https://www.google.com/recaptcha/`（`connect-src`） |
| 費用 | 無料（上限なし） | 無料枠あり | Google Cloud のプロジェクトに属し、月 10,000 評価まで無料。超過すると siteverify は **成功（スコア 0.9）を返す**（fail open） |
| 利用者の操作 | 多くはなし（必要なときだけチェックボックス） | 画像の課題。テキストの課題・アクセシビリティ用 Cookie | チェックボックス、必要なときに画像・音声の課題 |

出典: [Turnstile server-side validation](https://developers.cloudflare.com/turnstile/get-started/server-side-validation/)、[hCaptcha developer guide](https://docs.hcaptcha.com/)、
[reCAPTCHA verify](https://developers.google.com/recaptcha/docs/verify)、[reCAPTCHA FAQ（CSP）](https://developers.google.com/recaptcha/docs/faq)、
[reCAPTCHA Classic から Enterprise への移行](https://cloud.google.com/recaptcha/docs/migration-overview)。

3 つとも「ブラウザのウィジェットがトークンをフォームに入れ、サーバーが秘密鍵とトークンを siteverify に POST して JSON を受け取る」同じ形をしている
（hCaptcha と Turnstile は reCAPTCHA の API に合わせて作られている）。
reCAPTCHA v3 は課題を出さずにスコア（0.0〜1.0）だけを返し、Enterprise は API キーとプロジェクト ID で `createAssessment` を呼ぶ別の API になる。

## Decision Drivers（判断基準）

1. **既定では何も外部に送らない**: CAPTCHA は運営者が明示的に選んだときだけ有効になり、選ばないインスタンス（自前ホスト・閉じたネットワークを含む）の挙動は変わらない（NFR-COST-2、ADR-0010）
2. **決定者の要望（reCAPTCHA）を満たしつつ、特定の事業者に縛られない**: プライバシーや費用の理由で別のサービスを選べる
3. **安全側の検証**: トークンの検証はサーバーで行い、外部 HTTP は SSRF ガードを通し（ADR-0019 §1）、秘密鍵はログに出さない（ADR-0019 §6）
4. **CSP を緩めすぎない**: `'unsafe-inline'` と `'unsafe-eval'` を足さない。緩めるのはウィジェットを出すページだけ（ADR-0019 §9）
5. **障害時に運営者自身を締め出さない**: CAPTCHA のサービスが落ちても管理者がサインインできる
6. **利用者への説明とアクセシビリティ**: 第三者にデータが渡ることを画面で知らせ、課題を解けない人にも登録の経路を残す（ADR-0009、NFR の a11y）

## Considered Options（選択肢）

1. **reCAPTCHA v2 だけを組み込む**: 決定者の要望どおり。サービスの差し替えは考えない
2. **`HumanVerifier` ポートと、siteverify 互換の 1 つのアダプター（Turnstile / hCaptcha / reCAPTCHA v2 を設定で選ぶ）**
3. **自前ホストの proof-of-work（[ALTCHA](https://altcha.org/)（MIT）、[mCaptcha](https://mcaptcha.org/)（AGPL-3.0）など）**: 第三者にデータを送らない
4. **CAPTCHA を使わない**: レート制限・承認制・招待制・登録の拒否リストで足りるとする

## Decision（決定）

**選択肢 2 を採用する。** reCAPTCHA を使えるという要望を満たし、同じ実装で Turnstile と hCaptcha も選べるようにする。
推奨するサービスは Turnstile（無料で上限がなく、多くの利用者に課題を出さない）だが、選ぶのは運営者で、既定は「使わない」。

### 1. ポートとアダプター

- `internal/app/kernel` に `HumanVerifier` ポートを置く。`Verify(ctx, token, action string) error` だけを持ち、
  トークンが無効・期限切れ・使用済み・別のホスト名や別の操作で発行されたものなら `ErrHumanCheckFailed` を、
  サービスに届かない・タイムアウト・5xx・秘密鍵が無効（`invalid-input-secret`）のときは `ErrHumanCheckUnavailable` を包んで返す。
  CAPTCHA を使わないときは `HumanVerifier` を渡さない（nil）。
- アダプターは `internal/adapter/captcha/siteverify` の 1 つで、サービスごとの違い（検証 API の URL、フォームの欄の名前、スクリプトの URL、CSP に足すオリジン、言語の指定方法）を
  プリセットの表として持つ。`pkg/testkit` に `HumanVerifierContract` を置き、`httptest` の siteverify スタブで成功・失敗・期限切れ・ホスト名の不一致・5xx・タイムアウト・大きすぎる応答を検査する。
- ウィジェットの描画に要るもの（サービス、サイトキー、スクリプトの URL、欄の名前、CSP のオリジン）は値の型 `kernel.HumanCheckWidget` として、`internal/platform` が `authpages` に渡す。
- 対応しないもの: reCAPTCHA v3（スコアだけで課題を出せず、閾値で落とされた人に代わりの経路がない）と reCAPTCHA Enterprise の `createAssessment`（API キーとプロジェクトが要る別の API）。
  必要になったら同じポートの別アダプターとして ADR を書く。自前ホストの proof-of-work（選択肢 3）も同じポートで後から足せる。

### 2. 設定

デプロイの設定（[設定リファレンス](../reference/configuration.md)）に足す。インスタンスの設定（DB）には置かない。秘密鍵をインスタンスの設定に置かないため（ADR-0019 §6）。

| キー | 既定 | 必須 | 秘密 | 内容 |
| --- | --- | --- | --- | --- |
| `captcha` | `none` | no | no | `none` / `turnstile` / `hcaptcha` / `recaptcha`（v2 チェックボックス） |
| `captcha_site_key` | なし | `captcha` が `none` 以外のとき | no | ウィジェットに渡す公開のサイトキー |
| `captcha_secret_key` | なし | `captcha` が `none` 以外のとき | yes | siteverify に送る秘密鍵。`MY_CAPTCHA_SECRET_KEY` か `MY_CAPTCHA_SECRET_KEY_FILE` だけで受け取る |

- 「鍵が設定されていれば自動で有効にする」ことはしない。アダプターは常に明示的なキーで選ぶ（設定リファレンスの規約）。
  `captcha` を選んだのに鍵が欠けていれば `my config check` が問題として挙げ、サーバーは起動しない。これが要望の「有効な設定なら」にあたる。
- 起動時に siteverify へ試しに問い合わせることはしない（起動を外部サービスに依存させない、Cloud Run / Lambda のコールドスタートを延ばさない）。
  秘密鍵が誤っていれば最初の検証で `invalid-input-secret` が返るので、ERROR のログを出し、`ErrHumanCheckUnavailable` として扱う（§4）。
- 秘密鍵は `slog.LogValuer` で `[REDACTED]` を出す型で持つ。

### 3. 検証する場所と順序

- 検証は `identity` のユースケース（`SignIn`・`SignUp`）の中で、トランザクションの外で行う（ADR-0003）。入力（`SignInInput`・`SignUpInput`）にトークンを足す。
  画面ではなくユースケースで検証するので、別の入口を足しても検査を忘れない。
- 順序: レート制限 → CAPTCHA の検証 → パスワードの照合（Argon2id）/ 登録の処理。外部の呼び出しより安いレート制限を先にし、高価なハッシュ計算をボットにさせない。
- CAPTCHA を有効にすると、サインインと登録の両方で必ず検証する。片方だけにかける設定は持たない。
- 有効な招待コードでの登録でも検証する。招待コードは漏れたり奪われたりしうるので、それだけでは人であることの証明にならない。
- 検証の失敗は、サインインでは IP のログイン失敗として数え（アカウントの失敗としては数えない。他人のアカウントをロックさせないため）、登録では登録の試行として数える。
- 応答の検査: `success` が真、`hostname` が設定の `host` と一致すること。Turnstile ではウィジェットに `data-action`（`sign_in` / `sign_up`）を付け、応答の `action` が一致することも検査する（あるフォームで得たトークンを別のフォームで使わせない）。
- `remoteip` は送らない。ブラウザはウィジェットを読むときにサービスへ直接つながっているので、サーバーから IP を重ねて渡さない（データの最小化）。hCaptcha には推奨どおり `sitekey` を送る。
- 外部 HTTP は `kernel.HTTPClient` を通す。用途 `HTTPPurposeHumanCheck` を足し、本文の上限 64 KiB、全体のタイムアウト 10 秒とする。
  トークンは 1 回限りなので再試行しない（Turnstile の `idempotency_key` も使わない）。

### 4. 検証できないときの扱い

| 場面 | トークンが不正（`ErrHumanCheckFailed`） | サービスに届かない・秘密鍵が無効（`ErrHumanCheckUnavailable`） |
| --- | --- | --- |
| 登録 | 拒否し、フォームを「確認をやり直してください」と出し直す | 拒否し（fail closed）、「しばらくしてからやり直してください」と出す |
| サインイン | 拒否し、フォームを出し直す | **通す**（fail open）。WARN のログを出し、レート制限とパスワードの照合はそのまま行う |

- サインインを fail open にするのは、サービスの障害や鍵の誤りで管理者を含む全員が締め出されないため（Decision Driver 5）。
  サービスを落とせない攻撃者には得がなく、総当たりはレート制限が止める。
- reCAPTCHA は無料枠を超えると siteverify が成功を返す。運営者向けの文書に書き、超過の検出はこのサーバーでは行わない。

### 5. 画面と CSP

- ウィジェットはサインインと登録のフォームにだけ出す。サーバーが描画するページはいまは JavaScript なしで動くが、CAPTCHA を有効にしたときだけ、この 2 つのページはサービスのスクリプトを読む。
  `<noscript>` で「この確認には JavaScript が要る」ことを知らせる。
- ウィジェットの言語はページのロケールに合わせ（Turnstile は `data-language`、hCaptcha と reCAPTCHA はスクリプトの `hl`）、テーマはページに合わせる（`data-theme`）。
- ウィジェットの近くに「このページは <サービス名> でボットかどうかを確認します。<サービス名> のプライバシーポリシー」の文と、サービスのプライバシーポリシーへのリンクを出す（en / ja のメッセージ、ADR-0009）。
  reCAPTCHA では、バッジの代わりに「This site is protected by reCAPTCHA」と Google のプライバシーポリシー・利用規約を示す表示（reCAPTCHA FAQ）を兼ねる。
- CSP は、CAPTCHA を有効にしたときのサインインと登録のページだけ、`contentSecurityPolicy` に §Context の表のオリジンを足す（`script-src`・`frame-src`、hCaptcha では `style-src`・`connect-src`、reCAPTCHA では `connect-src` も）。
  `'unsafe-inline'`・`'unsafe-eval'` は足さない。Web クライアント・API・同意画面など他のレスポンスの CSP は変えない。
- `Permissions-Policy`・`Cross-Origin-Opener-Policy: same-origin`・`frame-ancestors 'none'` はそのまま（ウィジェットは iframe の中で動き、こちらのページを埋め込まない）。

### 6. プライバシーと運営者の文書

- サービスは利用者の IP アドレス・ブラウザの情報・Cookie を受け取る。reCAPTCHA は Google のプライバシーポリシーと利用規約の対象になる。
- 運営者向けに `docs/operations/captcha.md` を置き、サービスごとの鍵の作り方、設定、CSP、無料枠、プライバシーポリシー（#489）に書くべきこと（どのサービスに何が渡るか、その目的）を説明する。
  プライバシーポリシーの本文は運営者が書くもので、サーバーは自動で書き足さない。

## Consequences（影響）

- 良い影響
  - 多数の IP から来るスパム登録とパスワードスプレーに、レート制限とは別の層ができる。
  - 1 つのアダプターで 3 つのサービスに対応し、運営者が費用・プライバシーで選べる。自前ホストの proof-of-work も同じポートで後から足せる。
  - 既定は `none` なので、自前ホストや閉じたネットワークのインスタンスは何も変わらない。データモデル（テーブル）の変更はない。
- 悪い影響・トレードオフ
  - 有効にすると、サインインと登録のページが第三者のスクリプトに依存し、そのページの CSP が広がる。サービスのスクリプトが侵害されればそのページは影響を受ける。
  - CAPTCHA を解けない人（視覚・認知の障害、JavaScript を無効にしている人、サービスに届かない国や組織）は登録できない。招待コードがあっても同じで、運営者に連絡して CAPTCHA を外してもらうほかない。
  - サインインの fail open は、サービスの障害中は CAPTCHA の保護がないことを意味する。
  - サービスの仕様（検証 API・CSP のオリジン）が変わったらプリセットを直す必要がある。
- 後続で必要になる作業
  1. **ポートとアダプターと設定**（#499）: `kernel.HumanVerifier`、`HTTPPurposeHumanCheck`、`internal/adapter/captcha/siteverify` と 3 つのプリセット、`pkg/testkit.HumanVerifierContract`、
     設定のキー 3 つと `make config-docs`、`internal/platform` での組み立て
  2. **サインインと登録での検証と画面**（#500）: `identity` の `SignIn` / `SignUp` での検証と失敗の数え方、`authpages` のウィジェット・`<noscript>`・説明文・CSP、en / ja のメッセージ、
     サーバーのテスト（スタブの `HumanVerifier`）。各サービスのテスト用の鍵で、厳しい CSP のもとでウィジェットが動くことをブラウザで確かめる（§Open Questions）
  3. **運営者の文書**: `docs/operations/captcha.md`（#500 に含める）

## Pros and Cons of the Options（詳細比較・任意）

### 1. reCAPTCHA v2 だけ

- 良い: 要望どおりで、実装が最小。利用者が見慣れている。
- 悪い: Google への依存とデータ送信を全運営者に強いる。無料枠（月 10,000 評価）を超えると黙って成功を返す。
  選択肢 2 との差はプリセットの表だけなので、縛る理由が小さい。

### 2. `HumanVerifier` ポートと siteverify 互換アダプター（採用）

- 良い: 3 つのサービスが同じ形の API なので、追加の実装はほぼ表だけ。ポートがあるので proof-of-work や Enterprise も後から足せる。
- 悪い: 3 つの CSP・ウィジェットの動作を確かめ続ける必要がある。

### 3. 自前ホストの proof-of-work

- 良い（最も強い論点）: 第三者にデータを送らず、プライバシーポリシーへの記載も CSP の外部オリジンも要らない。自前ホストの環境でもそのまま動く。
- 悪い: 計算量でコストを課すだけで、CPU を持つボットは通る。要望の reCAPTCHA を満たさない。JavaScript の依存（ウィジェット）を持ち込む。
- 結論: 選択肢 2 のポートがあれば後から足せるので、今回は作らない。

### 4. CAPTCHA を使わない

- 良い: 第三者への依存がなく、何も変えなくてよい。承認制・招待制はスパム登録をほぼ止める。
- 悪い: 承認制は管理者の手間が登録数に比例し、開放した登録とパスワードスプレーには効かない。要望を満たさない。

## Open Questions / 要検証

- 3 つのサービスのウィジェットが、`'unsafe-inline'` のない `style-src` と `require-trusted-types-for` のない現在の認証ページの CSP で動くか（各サービスのテスト用の鍵を使い、ブラウザで確かめる。後続の作業 2）
