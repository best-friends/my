# ADR-0005: Blob ストレージとメディア処理

- Status: Proposed
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #7

## Context

Fediverse サーバーの運営コストで最も膨らみやすいのが **リモートメディアのキャッシュ**（ストレージと転送量）。
また画像のリサイズ・サムネイル生成は CPU とメモリを使い、libvips 等のネイティブライブラリは cgo を要求する（ADR-0001 の静的ビルド方針と衝突）。

## Decision（推奨案）

### BlobStore ポート

- 操作: `put` / `get`（ストリーム）/ `delete` / `head` / `publicUrl` / `signedUrl`（任意）
- アダプタ: `blob/fs`（ローカル FS）、`blob/s3`（S3 互換 API: AWS S3 / Cloudflare R2 / GCS の XML 互換 API / MinIO / その他）。必要なら `blob/gcs`（ネイティブ API）
- 配信は可能な限り CDN / オブジェクトストレージの公開 URL から直接行い、アプリを経由させない。

### MediaProcessor ポート

- 責務: 画像の EXIF 除去・リサイズ・サムネイル・blurhash 生成・形式検証
- アダプタ: `media/native`（pure Go: `golang.org/x/image` 等。既定）、`media/vips`（libvips、cgo。ビルドタグで任意）、
  `media/none`（無加工。ただし EXIF 除去だけは軽量実装で必須）
- 自前クライアント（Web / モバイル）は **アップロード前にクライアント側でリサイズ・EXIF 除去** する。サーバー側処理は保険。

### リモートメディア方針（FR-409, NFR-COST-4）

- 既定: **キャッシュしない**。クライアントには自インスタンスの **メディアプロキシ URL** を返し、プロキシは CDN キャッシュヘッダを付けて中継する
  （利用者の IP をリモートに漏らさない、かつストレージを消費しない）。
- オプション: TTL 付きでキャッシュ（アバター・ヘッダーのみ / 全メディア）、上限容量を設定可能。
- ドメインブロックの「メディア拒否」はプロキシ層で適用。

## Consequences

- 最小構成ではストレージ費用はローカルメディアの分だけになる。
- メディアプロキシは転送量を消費する（CDN キャッシュヒット率に依存）。Cloudflare R2 + CDN を Blob 先に選ぶと、転送量課金を抑えられる（他ターゲットと併用可）。
- 動画のトランスコードは v0 では行わない（サイズ・形式制限のみ）。

## Open Questions / 要検証

- [ ] pure Go の画像処理（JPEG / PNG / WebP / AVIF のデコードとエンコード）の対応状況・速度・メモリの実測
- [ ] GCS の S3 互換 API での署名付き URL 互換性
