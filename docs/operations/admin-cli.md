# 管理者の作成

運営者向けの手順です。要件は FR-801（[機能要件](../requirements/01-functional.md)）、パスワードの扱いは [ADR-0019 §3](../adr/0019-security-baseline.md) です。

## 前提

- 設定（`host`・`db`・`database_url`・`kek` など）が [設定リファレンス](../reference/configuration.md) どおりに与えられている。`my config check` で確かめられる。
- DB にスキーマが適用済み（Atlas で `schema/<dialect>.sql` を適用する。[ADR-0023](../adr/0023-declarative-schema-management.md)）。

## 作る

```sh
my admin create -username alice -email alice@example.com
```

- 端末から実行すると、パスワードをエコーなしで 2 回尋ねる。
- 標準入力が端末でなければ、標準入力の 1 行目をパスワードとして読む（例: `my admin create ... < password.txt`）。
  コマンドライン引数や環境変数では渡せない（プロセス一覧や履歴に残るため）。
- パスワードは 8〜128 文字。表示・ログ・エラーメッセージには出ない。
- 1 回のトランザクションで、ユーザー・パスワード・主メールアドレス・ローカルアカウント（鍵を含む）・`admin` ロールの付与（付与者なし = CLI）を作る。
  組み込みロールが無ければ先に作る。
- 成功すると `created admin @alice@<account_domain> (user <id>, account <id>)` を表示して終了コード 0 で終わる。

## 再実行したとき

- ユーザー名（大文字小文字を区別しない）かメールアドレス（大文字小文字を区別しない）が既に使われていれば、
  何も変更せずに終了コード 1 で終わり、どちらが使われているかを表示する。既存のユーザーのパスワードやロールは変えない。
- 失敗した実行は何も書き込まないので、原因を直してそのまま再実行してよい。
- 終了コード 2 は引数の誤り。
