# ドキュメント目次

## 要件

- [ビジョンとスコープ](requirements/00-vision.md)
- [機能要件（v0）](requirements/01-functional.md)
- [非機能要件](requirements/02-non-functional.md)
- [用語集](requirements/03-glossary.md)

## 設計

- [アーキテクチャ概要（基礎設計）](architecture/overview.md)
- [論理データモデル](architecture/data-model.md)
- [公開範囲の閲覧判定（canView）](architecture/visibility.md)
- [連合の設計（ActivityPub S2S）](architecture/federation.md)
- [デプロイターゲットと低コスト構成](architecture/deployment-targets.md)
- [Web クライアントの設計規約](architecture/web-client.md)
- [ADR 一覧](adr/README.md)

## リファレンス

- [設定リファレンス](reference/configuration.md)（設定スキーマから生成）

## 運用

- [管理者とロール](operations/admin-cli.md)
- [インスタンス設定](operations/instance-settings.md)
- [利用者登録（登録モード・招待・承認・メール確認）](operations/registration.md)
- [ボット対策の CAPTCHA（Turnstile / hCaptcha / reCAPTCHA）](operations/captcha.md)
- [ドメイン単位の処分と通報の対応](operations/moderation.md)
- [鍵暗号化鍵（KEK）の管理](operations/key-management.md)
- [DB の初期化と作り直し](operations/database.md)
- [サーバーの起動・停止とヘルスチェック](operations/running.md)
- [リンクプレビュー（OGP / X Card / fediverse:creator）](operations/link-previews.md)

## 開発プロセス

- [コントリビュートの手引き](../CONTRIBUTING.md)
- [エージェント向け指示](../AGENTS.md)
- [Issue 作業手順（playbook）](agents/workflow.md)
- [ADR の書き方（playbook）](agents/writing-adrs.md)
