# インスタンス設定

運営者向けの手順です。要件は FR-001・FR-101・FR-705（[機能要件](../requirements/01-functional.md)）、
テーブルは [データモデル §4.9](../architecture/data-model.md#49-instance) です。管理画面からの変更は M5 で追加します。

## 設定（環境変数）とインスタンス設定（DB）の分担

| | 設定 | インスタンス設定 |
| --- | --- | --- |
| 内容 | デプロイの構成: 役割・アダプタ・接続先・ホスト名・秘密情報 | 運営の方針: 登録モード・文字数上限・既定のロケール・タイムラインの有効 / 無効、インスタンスの名前・説明・規約 |
| 置き場所 | 環境変数 `MY_*` と設定ファイル（[設定リファレンス](../reference/configuration.md)、ADR-0010） | DB。変更はすべて版として残る |
| 変更 | プロセスの再起動で反映 | `my instance set` で即時に反映 |
| 未設定のとき | スキーマの既定値（必須なら起動しない） | コードで定義した既定値 |

- **同じキーが両方にあることはありません。** どちらかがもう一方を上書きする優先順位は存在しません。
  環境変数や設定ファイルでインスタンス設定を与えることはできず、DB に設定（環境変数）の値を置くこともできません。
  両方のキーが重ならないことはテスト（`TestConfigAndInstanceSettingsShareNoKey`）で保証します。
- 新しい値をどちらに置くかは、「再起動なしに運営者が変えるものか」で決めます。デプロイごとに決まり、変えるとプロセスの構成が変わるものは設定、
  運営中に変えるものはインスタンス設定です。

## 前提

- 設定が [設定リファレンス](../reference/configuration.md) どおりに与えられている（`my config check`）。
- DB にスキーマが適用済み（Atlas で `schema/<dialect>.sql` を適用する。[ADR-0023](../adr/0023-declarative-schema-management.md)）。

## キー

### 設定値（1 キー 1 値）

| キー | 型 | 既定値 | 説明 |
| --- | --- | --- | --- |
| `registration_mode` | `open` / `approval` / `invite` / `closed` | `approval` | 登録モード（FR-001） |
| `max_status_length` | 整数 1〜100000 | `500` | 投稿の文字数上限（FR-101） |
| `default_locale` | BCP 47 言語タグ | `en` | 既定のロケール。利用者の希望に合うカタログが無いときと、インスタンスの文章がその言語に無いときに使う（ADR-0009） |
| `local_timeline_enabled` | `true` / `false` | `true` | ローカルタイムラインを提供するか（FR-202） |
| `federated_timeline_enabled` | `true` / `false` | `true` | 連合タイムラインを提供するか（FR-202） |

- 値は保存前に正規形にします（`TRUE` → `true`、`0750` → `750`、`ja_jp` → `ja-JP`）。不正な値は何も書き込まずに拒否します。

### 文章（ロケール別）

| キー | 上限 | 説明 |
| --- | --- | --- |
| `name` | 100 文字・1 行 | インスタンスの名前 |
| `short_description` | 500 文字 | 短い説明 |
| `description` | 20000 文字 | 説明 |
| `terms` | 100000 文字 | 利用規約 |
| `privacy_policy` | 100000 文字 | プライバシーポリシー |

- 文章はロケールごとに持ちます。読むときは、読み手のロケールのフォールバックチェーン → `default_locale` のチェーン → `en` の順に探します
  （例: `ja-JP` → `ja` → 既定のロケール → `en`）。
- 空の文章を設定すると、そのロケールの文章を消したことになり、チェーンの次のロケールが使われます。

## 使い方

```sh
my instance list                                      # すべての設定値（既定値かどうか）と文章
my instance get registration_mode
my instance set registration_mode open
my instance set name "Example Social"                 # default_locale のロケールに設定
my instance set -locale ja name "例のソーシャル"
my instance set -locale ja description - < description.ja.txt   # 値 "-" は標準入力から読む
my instance get -locale ja-JP name                    # フォールバックチェーンで探す
my instance set -locale ja name ""                    # ja の名前を消す
my instance history registration_mode                 # 変更履歴（新しい順、-limit で件数）
my instance history -locale ja description
```

- フラグはキーと値より前に書きます。
- 変更は 1 回のトランザクションで版（`instance_*_revisions`）を記録し、現在値（`instance_settings` / `instance_texts`）を更新します。
  CLI からの変更は変更者なし（履歴の `BY` が `cli`）として残ります。
- 同じ値を設定し直しても版は増えます。
- 終了コード: 0 は成功、1 は値の拒否や DB のエラー（何も変更しない）、2 は引数の誤り。
