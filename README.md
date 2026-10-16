# my

ActivityPub 互換のマイクロブログサーバー。Go 製。Google Cloud / AWS / セルフホスト（Docker, microk8s）で、
できる限り低コストに運営できることを目指す OSS です。

> **Status: v0 の開発中（M1: Walking Skeleton）。** まだ利用できる機能はありません。
> v0 の間は DB の作り直しを含む破壊的変更が頻繁に入ります（[ADR-0012](docs/adr/0012-v0-breaking-change-policy.md)）。

## 目指すもの

- 月額ほぼ $0〜$5 から運営できる（scale-to-zero、常駐ミドルウェア不要）
- RDB・キュー・ストレージ・実行環境を設定で切り替えられる
- Mastodon / Misskey / GoToSocial などと連合できる
- 最初から多言語対応
- Web（PWA）とモバイルアプリ

## ドキュメント

- [ドキュメント目次](docs/README.md)
- [設計判断（ADR）一覧](docs/adr/README.md)
- [コントリビュートの手引き](CONTRIBUTING.md) / [コーディングエージェント向け](AGENTS.md)

## License

[GNU Affero General Public License v3.0](LICENSE)（[ADR-0011](docs/adr/0011-license.md)）。
