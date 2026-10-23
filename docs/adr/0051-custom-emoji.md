# ADR-0051: カスタム絵文字

- Status: Accepted（§1 の `custom_emojis` の行と、§5 の「管理 API・CLI・管理画面の絵文字の操作はすべてこの権限で行う」を ADR-0070（権限の分割: 絵文字の作成と会計の閲覧） が置き換える）
- Date: 2026-10-20
- Deciders: @rosylilly
- Issue: #618

## Context（背景・課題）

M4（[vision](../requirements/00-vision.md)）は、カスタム絵文字とリアクションが Mastodon / Misskey / Pleroma(Akkoma) と往復することを完了条件にしている。
FR-111（カスタム絵文字）は、いまは受信の一部だけが実装されている。

- 受信: Note の `tag` の `Emoji` は `internal/domain/federation/normalize.go` が `name` の前後の `:` を外して読み（`icon` の URL と `id` を取り、`id` は無くてよい）、
  `internal/app/content/remote_status.go` の `remoteEmojis` が `custom_emojis`【リソース】・`custom_emoji_images`【イベント（版）】・`status_revision_emojis`【イベント（版の子）】に記録する。
  `domain` は作者のホスト、画像は作者を所有者とする `remote_media` で、キャッシュ方針 `avatars` の対象（[federation.md §9](../architecture/federation.md#9-リモートメディア)）。
- Actor の `tag` の `Emoji`（表示名・自己紹介・補足情報の絵文字）は `Actor.Emojis` に読むが、保存していない。
- クライアント API の Status / Account に `emojis` がなく、Web は `:shortcode:` を描画しない。サニタイザは `<img>` を落とす（[ADR-0019](0019-security-baseline.md) §2: カスタム絵文字はクライアントが DOM 要素として差し込む）。
- ローカルの絵文字の登録・管理、送る Note / Actor の `Emoji` タグ、絵文字の一覧の API、絵文字ピッカーがない。
  [data-model.md](../architecture/data-model/README.md) は `custom_emoji_settings`【現在値】を挙げているが、スキーマにはまだない。

絵文字リアクションの ADR（ADR-0052、#619）は、この ADR が決める `Emoji` タグと絵文字の参照の形を使う。そのため、形をここで固める。

関連: FR-111、FR-006（プロフィール）、FR-104（CW）、FR-112（投票）、FR-702（ドメイン単位の `reject_media`）、FR-704（監査ログ）、FR-720（ロール）、FR-409 / NFR-COST-4（リモートメディア）、
[ADR-0005](0005-blob-storage-and-media.md)、[ADR-0006](0006-federation-protocol-scope.md)（`toot:Emoji` は互換拡張）、[ADR-0018](0018-federated-uris.md)、[ADR-0019](0019-security-baseline.md)、
[ADR-0020](0020-data-modeling-principles.md)、[ADR-0037](0037-audit-log.md)、[ADR-0054](0054-media-variants-video-audio.md)（画像の派生。カスタム絵文字の箱は 64 / 128 / 256 px）、[ADR-0060](0060-instance-webhooks.md)（絵文字の管理操作の webhook）。

### 主要実装の比較

2026-10-20 に各実装のソースと公式文書で確認した（出典は末尾。確かめられなかったものは要検証）。

| 観点 | Mastodon | Misskey | GoToSocial | Pleroma / Akkoma |
| --- | --- | --- | --- | --- |
| 絵文字の `id` | `https://{host}/emojis/{数値 ID}`（`GET` で `Emoji` の JSON を返す） | `https://{host}/emojis/{name}` | `https://{host}/emoji/{ULID}`（要検証） | 画像の URL や `null` のことがある（フィクスチャ `akkoma/3.20/emoji-react-custom.json` は `"id": null`） |
| `name` | `:shortcode:` | `:name:` | `:shortcode:`（要検証） | `shortcode`（`:` なしのことがある） |
| `updated` | 送る。受信で `updated` が保存値以後、または画像の URL が違えば取り直す | 送る（無ければ現在時刻）。受信で `updated` が新しい・URL が違う・`uri` が無いときに更新 | 要検証 | 要検証 |
| `icon` | `Image`（`mediaType`、`url`） | `Image`（`mediaType`。不明なら `image/png`）、`_misskey_license` | `Image`（要検証） | `Image`（`url` だけのことがある） |
| 受信の識別 | `(shortcode, 送り手の Actor のドメイン)`。`reject_media` のドメインのものは保存しない | `(name, host)` | 要検証 | 要検証 |
| ローカルの形式・サイズ | PNG / GIF / WebP、256 KiB 未満、静止画の PNG を別に作る | 制限は設定次第（要検証） | 既定 50 KiB（受信は 100 KiB） | PNG（APNG 可）。Mastodon 互換のため 50 KiB 未満を推奨 |
| shortcode | `[a-zA-Z0-9_]{2,}`、ドメインごとに一意 | 英数字と `_`（要検証） | 要検証 | ファイル名か `emoji.txt` |
| 管理 | 管理画面: カテゴリ、ピッカーに出すか、無効化、削除、リモートの絵文字の複製（`copy!`）。権限 `manage_custom_emojis` | 管理画面: カテゴリ、別名、ライセンス、センシティブ、ローカル限定、ロールでの制限 | 管理 API・管理画面: カテゴリ、無効化、リモートの複製 | ファイルの絵文字パック、MRF `StealEmojiPolicy` での取り込み |

分かったこと:

- 相互運用に効くのは、`name`（`:shortcode:`）・`icon.url`・`updated` の 3 つ。どの実装も `id` の形式を前提にしない。Mastodon と Misskey は `(shortcode, ドメイン)` で絵文字を識別し、`updated` か画像の URL の変化で取り直す。
- `icon.mediaType` は Mastodon と Misskey が送る。受け手が `mediaType` を読まない実装（Misskey）もあるので、形式は中身で判定する。
- 送る画像の形式とサイズは、受け手の制限に合わせる必要がある。GoToSocial は既定で 100 KiB を超えるリモートの絵文字を取得しない。Mastodon は PNG / GIF / WebP しか絵文字として扱わない。

## Decision Drivers（判断基準）

1. **相互運用性**: Mastodon / Misskey / GoToSocial / Akkoma の絵文字がこちらで表示され、こちらの絵文字が相手で表示される（ADR-0006、M4 の完了条件）
2. **リアクションと共有できる参照の形**: 投稿・プロフィール・リアクション（ADR-0052）が同じ絵文字の行と同じ `Emoji` タグを使う
3. **既存の決定への適合**: ADR-0020 の分類、ADR-0018 の URI、ADR-0019 のサニタイズとアップロード、ADR-0037 の監査ログ、リモートメディアのプロキシ
4. **運用の軽さとコスト**: 1 人で運用するインスタンスで、管理画面だけで登録でき、ストレージを増やさない（NFR-COST）
5. **安全**: 他のサーバーの絵文字を偽れない、拒否したドメインの画像を出さない、動きを減らす設定を尊重する

## Considered Options（選択肢）

全体の方式:

1. **Mastodon 相当（採用）**: `content` の既存のテーブルにローカルの行と設定を足す。登録・画像の差し替え・カテゴリ・別名・ピッカーに出すか・無効化・削除・リモートの複製を、管理 API・CLI・管理画面で行う
2. **Misskey 相当**: 1 に加えて、ライセンス・センシティブ・ローカル限定・ロールによる利用の制限を持つ
3. **ファイルの絵文字パック（Akkoma 方式）**: 運用者がディレクトリに画像を置き、起動時に読み込む。管理画面と API を持たない

ローカルの絵文字の `id`:

- A. **`https://{host}/ap/emojis/{uuid}`（採用）**: ADR-0018 の規則のまま、`/ap/` の下に UUIDv7 で置く
- B. `https://{host}/emojis/{shortcode}`（Misskey 方式）: shortcode をパスに含める
- C. `id` を送らない、または画像の URL を `id` にする

## Decision（決定）

**全体の方式 1 と `id` の A を採用する。** 理由: 1 は Mastodon・GoToSocial の管理機能と同じ範囲で、判断基準 1・4 を満たす最小の形。2 の属性は相互運用に効かず（`_misskey_license` 以外は連合しない）、ロールの制限はリアクションの ADR の範囲を広げる。
3 はコンテナや Cloud Run の環境で運用者がファイルを置きにくく、管理画面で完結しない（判断基準 4）。
A は ADR-0018 の「`id` は UUIDv7 だけで、名前を含めない」をそのまま当てられ、shortcode を変えても削除して作り直しても `id` が重ならない。B は削除して同じ shortcode で作り直すと別の絵文字が同じ `id` になり、C は Mastodon の `GET` での解決と Misskey の `uri` の記録が使えない。

覆す条件: 主要実装のどれかが `id` の形式やローカル限定の属性を前提にし始めたとき、または Misskey とのリアクションの往復でライセンスやロールの制限が必要だと分かったとき。

### 1. データモデル（`content` が持つ。ADR-0020）

| テーブル | 分類 | カラム | 制約・規則 |
| --- | --- | --- | --- |
| `custom_emojis` | リソース（既存） | `id`, `shortcode`, `domain`（ローカルは `''`）, `uri`（NULL 可）, `created_at` | 一意 `(shortcode, domain)`, `(uri)`。ローカルの `uri` は NULL で、`id` から `https://{host}/ap/emojis/{id}` を導く（ADR-0018 の `Create` の `id` と同じく保存しない） |
| `custom_emoji_images` | イベント（版、既存） | `id`, `emoji_id`, `media_id`, `revised_at` | 最新の行が現在の画像。ローカルはアップロードした `media_files`、リモートは `remote_media`。画像の差し替えは行を足す |
| `custom_emoji_settings` | 現在値 | `emoji_id`（PK）, `enabled`（`bool`）, `visible_in_picker`（`bool`）, `category`（NULL 可、64 文字まで） | 行が無ければ `enabled = true`・`visible_in_picker = true`・カテゴリなし。リモートの絵文字は `enabled` だけを使う |
| `custom_emoji_aliases` | 現在値 | `emoji_id`, `alias`（shortcode と同じ文字種） | PK `(emoji_id, alias)`。ローカルだけ。ピッカーの検索に使う |
| `account_profile_revision_emojis` | イベント（版の子） | `revision_id`, `emoji_id` | PK `(revision_id, emoji_id)`。`account` が持ち、`emoji_id` は `content` の公開 API で得る |

- 状態（無効・ピッカーに出さない）は `custom_emoji_settings` の現在値で持ち、nullable な時刻で表さない。変更の履歴は監査ログ（§5）が持つ。
- **shortcode**: ローカルは `[A-Za-z0-9_]{2,30}`（Mastodon の文字種と GoToSocial の長さの上限に合わせる。要検証: GoToSocial の上限）。大文字と小文字を区別し、変えられない（投稿の本文が shortcode で参照するため）。
  リモートは受け取った値を `:` を外して保存し、長さは 128 文字までに切る（超えたものは捨てる）。
- **削除**: ローカルの絵文字の削除は、`custom_emojis`・`custom_emoji_images`・設定・別名の行と画像の Blob を同じトランザクションで物理削除する（Blob は outbox のジョブで消す）。
  過去の版の `status_revision_emojis` / `account_profile_revision_emojis` の行は残し、読み取りでは絵文字の無い参照として飛ばす（本文は `:shortcode:` のまま表示される）。
  同じ shortcode で作り直した絵文字は新しい `id` なので、過去の投稿には付かない。
- **リモートの絵文字の識別**: `domain` は投稿・プロフィール・リアクションの作者（送り手の Actor）の正規化したホストで、`(shortcode, domain)` で引く（Mastodon・Misskey と同じ）。
  `uri` は `Emoji` の `id` が `domain` と同じオリジンのときだけ記録し、違うとき・無いときは NULL にする。他のサーバーの絵文字の `id` を名乗って行を奪わせないため。
  画像の URL が記録と違えば `custom_emoji_images` に行を足す（既存の規則のまま）。
- **リモートの絵文字の消去**: どの版（投稿・プロフィール）とリアクションからも参照されなくなったリモートの絵文字は、1 日ごとのジョブ `content.purge_remote_emojis` が消す（FR-803、ADR-0034 の保持期間で投稿が消えた後に残らないように）。
  参照の判定はリアクションを持つモジュールの公開 API を含む（ADR-0052）。

### 2. 画像の制約（ADR-0005、ADR-0019 §8）

- 形式: PNG（APNG を含む）・GIF・WebP。中身で判定し、JPEG・AVIF・SVG は絵文字として受け付けない（Mastodon が絵文字として扱う形式に合わせる）。
- ローカルのアップロード: 運用者の設定 `custom_emoji_max_bytes`（既定 256 KiB、Mastodon の上限）以下、長辺 1,024 px 以下。アニメーションのフレーム数の上限は ADR-0019 §8 の値を使う。
  GoToSocial は既定で 100 KiB を超えるリモートの絵文字を取得しないので、管理画面は 100 KiB を超える画像に注意を表示する。
- 派生は ADR-0054 の用途 `emoji` で作る: S / M / L は 64×64 / 128×128 / 256×256 の箱に縦横比を保って収め、アニメーションは静止版（最初のフレーム）と動く版の両方を作る。原本は保存しない。
  形式も ADR-0054 §2・§3 に従う（L は PNG、動く版の L は GIF）。APNG は ADR-0054 §3 のとおり最初のフレームの静止画として扱い、アニメーション WebP は ADR-0054 の spike が通ったときだけ受け付ける。
- リモートの絵文字の画像はメディアプロキシ（federation.md §9）を通し、静止版はプロキシの `small`（ADR-0054 §7。絵文字の S の 64×64）で返す。プロキシは、`custom_emoji_images` が指すメディアには設定 `remote_custom_emoji_max_bytes`（既定 1 MiB）の上限を当て、超えるものを 404 にする。
  形式の許可リストは上の 3 つに絞る。

### 3. 連合（送信・受信）

**`Emoji` タグの形**（投稿・Actor・リアクションで共通。ADR-0052 はこの形をそのまま使う）:

```json
{
  "id": "https://social.example.org/ap/emojis/0192f3d0-1a2b-7c3d-8e4f-5a6b7c8d9e0f",
  "type": "Emoji",
  "name": ":blobcat:",
  "updated": "2026-10-20T03:04:05Z",
  "icon": {
    "type": "Image",
    "mediaType": "image/png",
    "url": "https://media.social.example.org/media/0192f3d1-.../0192f3d2-.../large.png"
  }
}
```

- `id`: ローカルの絵文字は `https://{host}/ap/emojis/{uuid}`（`{uuid}` は `custom_emojis.id`）。リモートの絵文字を参照するときは記録した `uri` で、NULL なら `id` を省く。
- `name`: `:` で囲んだ shortcode。
- `updated`: 現在の画像の `custom_emoji_images.revised_at`（RFC 3339、UTC、秒まで）。画像を差し替えると変わり、相手が取り直す。
- `icon`: `type` は `Image`、`mediaType` は `url` の形式（ローカルは派生の `content_type`）、`url` はローカルなら L の公開 URL（アニメーションなら動く版の L の GIF。ADR-0054 §5 の Actor の `icon` と同じ考え方）、リモートなら記録した元の URL（メディアプロキシの URL は送らない）。
- `@context` には `{"toot": "http://joinmastodon.org/ns#", "Emoji": "toot:Emoji"}` を入れる。

**ADR-0018 への追加**: URI 一覧に「カスタム絵文字 `https://{host}/ap/emojis/{uuid}`」を加え、受信した自サーバーの URI のパース（`^/ap/(actors|objects|activities)/{uuid}`）に `emojis` を加える。ADR-0018 の他の規則は変えない。

- `GET /ap/emojis/{uuid}` は上の `Emoji` を（`@context` を付けて）返す。存在しない・削除済み・無効の絵文字は同じ 404 にする。HTML を求めるリクエストも 404。
- **どこに付けるか**: Note（`Question` を含む）の `tag` には、その版の本文・CW（`summary`）・投票の選択肢（`name`）で使った絵文字を、Actor の `tag` には表示名・自己紹介・補足情報の名前と値で使った絵文字を入れる。
- **ローカルの本文からの抽出**: 投稿・プロフィールの保存のとき、サーバーがプレーンテキストの原文から `:shortcode:` を探し、有効なローカルの絵文字だけを版に記録する（無効・未登録の shortcode は文字のまま）。
  前後の境界は Mastodon の `CustomEmoji::SCAN_RE`（前後が英数字と `:` でないこと）に合わせる。ローカルの投稿で使えるのはローカルの絵文字だけ。
- **受信**: 投稿は既存の経路のまま。Actor の `tag` の `Emoji` は、プロフィールの版を作るときに同じ規則（§1 の識別）で記録し、`account_profile_revision_emojis` に結ぶ。
  1 つの文書から記録する絵文字の数は、既存の `MaxTags` の上限に従う。

### 4. クライアント API と表示

- 絵文字の参照（API の `CustomEmoji`）: `shortcode`（`:` なし）, `url`, `static_url`。ローカルは `url` が M（アニメーションなら動く版の M）、`static_url` が静止版の M の公開 URL。
  リモートは `url` がメディアプロキシの `full`、`static_url` が `small`。リモートの絵文字には寸法と派生の記録がないので、ADR-0054 の `MediaVariants` ではなくこの 2 つの URL にする。
- `Status.emojis` はその版の本文・CW・投票の選択肢の絵文字、`Account.emojis` は現在のプロフィールの版の表示名・自己紹介・補足情報の絵文字。通知は埋め込む Status / Account がそのまま持つ。
- 返さないもの: 無効（`enabled = false`）の絵文字、削除された絵文字、画像の所有者のドメインか URL のホストに `media_fetch` の制限（`reject_media`）があるリモートの絵文字。返さない絵文字は文字の `:shortcode:` として表示される。
- ピッカーの一覧 `GET /custom_emojis`: 有効でピッカーに出すローカルの絵文字を、`category`・`aliases` 付きで返す。キャッシュは ADR-0045 の方針に従う。
- 管理 API `/admin/custom_emojis`: 一覧（ローカル・リモート、shortcode とドメインで絞り込み）、作成（アップロード済みのメディアと shortcode）、画像の差し替え、設定と別名の変更、削除、リモートの絵文字のローカルへの複製（新しい shortcode を指定できる）。
  エンドポイントの細部は [client-api.md](../architecture/client-api.md) の手順で決める。
- **Web の描画**: サニタイズ済み HTML は DOM のテキストノードを、プレーンテキストのフィールド（表示名・CW・投票の選択肢・補足情報）は文字列を、同じ字句解析で `:shortcode:` に分け、`emojis` にある shortcode だけを
  `<img alt=":shortcode:" title=":shortcode:" draggable="false">` の DOM 要素として差し込む（`innerHTML` は使わない。ADR-0019 §2）。
  高さは周りの文字に合わせる（デザイントークン）。アニメーションは ADR-0054 §3 と同じく `<picture>` の `<source media="(prefers-reduced-motion: no-preference)">` に `url` を、`<img>` に `static_url` を置き、利用者の設定「アニメーションを自動再生しない」が有効なら `<source>` を出さない。
- **ピッカー**: 投稿の作成と編集、プロフィールの編集に置く。カテゴリごとの一覧と、shortcode・別名の検索を持ち、選ぶとカーソル位置に `:shortcode:` を入れる。
  最近使った絵文字はクライアントの端末ごとに持つ。リアクションでの使い方（Unicode の絵文字を含む）は ADR-0052 が決める。

### 5. 権限と監査ログ

- 新しい権限 `manage_custom_emojis` を作る。組み込みの `admin` ロールは持ち、`moderator` ロールは持たない（Mastodon と同じ）。管理 API・CLI・管理画面の絵文字の操作はすべてこの権限で行う。
  リモートの絵文字の無効化もこの権限で行う（モデレーションと管理の権限の分け方は ADR-0058、#648 が見直す）。
- 監査ログ（ADR-0037）に、作成・画像の差し替え・設定と別名の変更・無効化と有効化・削除・リモートの複製を記録する。
  同じ操作で ADR-0060 の webhook のイベントを出す: 作成とリモートの複製は `custom_emoji.created`、画像の差し替え・設定と別名の変更・無効化と有効化は `custom_emoji.updated`、削除は `custom_emoji.deleted`。`data` は管理 API `/admin/custom_emojis` の表現。受信でリモートの絵文字を記録したときは出さない。
  **ADR-0037 への追加**: `target_kind` に `custom_emoji` を加え、`target_label` は `:shortcode:`（リモートは `:shortcode:@domain`）にする。ADR-0037 の他の規則は変えない。
- CLI は `my admin emoji` の下に、作成（ファイルと shortcode）・一覧・画像の差し替え・カテゴリと別名とピッカーへの表示の変更・無効化と有効化（リモートの絵文字を含む）・削除・リモートの絵文字のローカルへの複製を持ち、管理 API と同じアプリ層の操作で行う。

## Consequences（影響）

- 良い影響
  - Mastodon・Misskey・GoToSocial・Akkoma の絵文字を投稿とプロフィールで表示でき、こちらの絵文字も相手で表示される。
  - リアクション（ADR-0052）は、絵文字の行・`Emoji` タグ・API の `CustomEmoji` をそのまま使える。
  - 既存のテーブル・メディアの処理・メディアプロキシ・監査ログを使うので、新しい仕組みは設定と別名の 2 つの現在値と 1 つの版の子だけ。
- 悪い影響・トレードオフ
  - 削除した絵文字は、過去の投稿でも文字に戻る。連合先が複製した画像は消せない（Mastodon と同じ）。
  - ライセンス・センシティブ・ロールの制限を持たないので、Misskey から届く `_misskey_license` は保存しない。
  - ADR-0054 は APNG を静止画として扱うので、ローカルの APNG の絵文字は動かない。GIF（とアニメーション WebP）で登録する。
  - 原本を保存しないので、Misskey などから届く大きな絵文字も、ローカルへ複製すると 256×256 の箱に縮む。
  - shortcode を大文字と小文字で区別するので、`Blob` と `blob` を別の絵文字として登録できる（Mastodon と同じ）。
- 後続で必要になる作業（Issue 化する）
  - 受信: Actor の絵文字の保存、`uri` のオリジンの検査、API の `emojis`、Web の描画（`@my/ui` の絵文字の部品）
  - ローカル: スキーマ（`custom_emoji_settings`・`custom_emoji_aliases`・`account_profile_revision_emojis`）、管理 API・CLI・管理画面、権限と監査ログ、送る `Emoji` タグ、`GET /ap/emojis/{uuid}`
  - ピッカー: `GET /custom_emojis` と Web のピッカー（WAI-ARIA APG の Grid / Combobox に従う）
  - 消去ジョブ `content.purge_remote_emojis` と、メディアプロキシの絵文字の上限
  - ADR-0060 の `custom_emoji.*` のイベント
  - 実装の Issue で data-model.md・federation.md・client-api.md を更新する。ADR-0018 の URI 一覧と ADR-0037 の `target_kind` への追加はこの ADR が持ち、両 ADR の本文は変えない
  - Mastodon の絵文字パック（`tootctl emoji import` の tar.gz）の取り込みは別の Issue で検討する

## Pros and Cons of the Options（詳細比較・任意）

| 観点 | 1. Mastodon 相当（採用） | 2. Misskey 相当 | 3. ファイルの絵文字パック |
| --- | --- | --- | --- |
| 相互運用 | ◯ どの実装とも `name`・`icon`・`updated` で往復 | ◯ 同じ。ライセンスも Misskey へ送れる | ◯ 同じ |
| 管理の手間 | ◯ 管理画面で完結 | △ 属性が多い | ✕ ファイルの配置と再起動が要る。Cloud Run では難しい |
| 実装量 | ◯ 既存のテーブルの拡張 | ✕ ロールの制限がリアクションと権限に波及 | △ 管理 API は不要だが、パックの読み込みと再読み込みが要る |
| 強み | 範囲が小さく、Mastodon の管理者に馴染みがある | Misskey の利用者に馴染みのある細かい制御 | 絵文字を Git で管理できる |

`id` の形:

| 観点 | A. `/ap/emojis/{uuid}`（採用） | B. `/emojis/{shortcode}` | C. 送らない・画像の URL |
| --- | --- | --- | --- |
| ADR-0018 との整合 | ◯ そのまま | ✕ 名前を含む | ✕ |
| 削除と作り直し | ◯ 別の `id` | ✕ 同じ `id` を別の絵文字が使う | △ |
| 読みやすさ | △ | ◯ | — |

## Open Questions / 要検証

- 要検証: GoToSocial の絵文字の `id` の形・shortcode の上限・受信の識別、Akkoma の受信の識別と `updated` の扱い、Misskey のローカルの絵文字の形式とサイズの制限。

### 出典（2026-10-20 確認）

- Mastodon: [ActivityPub の仕様（`toot:Emoji`）](https://docs.joinmastodon.org/spec/activitypub/)、
  [`app/models/custom_emoji.rb`](https://github.com/mastodon/mastodon/blob/main/app/models/custom_emoji.rb)（`LIMIT = 256.kilobytes`、`IMAGE_MIME_TYPES`、`SHORTCODE_RE_FRAGMENT`、`copy!`）、
  [`app/serializers/activitypub/emoji_serializer.rb`](https://github.com/mastodon/mastodon/blob/main/app/serializers/activitypub/emoji_serializer.rb)、
  [`app/lib/activitypub/activity/create.rb`](https://github.com/mastodon/mastodon/blob/main/app/lib/activitypub/activity/create.rb)（`process_emoji`）
- Misskey: [`ApRendererService.ts`](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/core/activitypub/ApRendererService.ts)（`renderEmoji`、`renderLike`）、
  [`ApNoteService.ts`](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/core/activitypub/models/ApNoteService.ts)（`extractEmojis`）
- GoToSocial: [Media の設定](https://docs.gotosocial.org/en/latest/configuration/media/)（`media-emoji-local-max-size` 50 KiB、`media-emoji-remote-max-size` 100 KiB）
- Akkoma: [Custom Emoji](https://docs.akkoma.dev/stable/configuration/custom_emoji/)、このリポジトリのフィクスチャ `pkg/testkit/fixtures/federation/akkoma/3.20/`
