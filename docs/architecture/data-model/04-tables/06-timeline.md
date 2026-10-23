### 4.6 timeline

`timeline` モジュールは **自前の投影だけ** を読みます（ADR-0002 の例外は不要）。どれも `content` / `graph`（制限は `moderation`）のイベントを購読する同期 projector が更新します（§2.1）。

| テーブル | カラム | 制約 / 用途 | 元になるイベント |
| --- | --- | --- | --- |
| `feed_entries` | `id`（= 投稿の `status_id` またはブーストの `boost_id`）, `kind`（`enum(post, boost)`）, `status_id`, `author_account_id`（投稿者またはブーストした人）, `author_domain`, `original_author_account_id`（元投稿の作者。投稿なら `author_account_id` と同じ）, `original_author_domain`, `visibility`, `is_local`（投稿者またはブーストした人がローカルか）, `language`（`lang`。現在版の言語）, `revision_seq`（`int`。現在版の `seq`）, `is_reply`（`bool`。投稿が返信か。ブーストは常に偽） | PK `(id)`。インデックス `(author_account_id, id)`、`(status_id)`（版・削除での更新）、部分インデックス `(id) WHERE is_local AND kind = 'post' AND visibility IN ('public','local')`（ローカル）、`(id) WHERE kind = 'post' AND visibility = 'public'`（連合） | 投稿・版（言語）・削除・ブースト・ブースト取り消し |
| `feed_direct_recipients` | `entry_id`, `recipient_account_id` | PK `(recipient_account_id, entry_id)`、インデックス `(entry_id)`。`direct` 投稿の宛先（現在版のメンション） | 投稿・版・削除 |
| `feed_tag_entries` | `tag_id`, `entry_id`, `author_account_id`, `visibility`, `is_local` | PK `(tag_id, entry_id)`、インデックス `(entry_id)`。現在版のタグ。公開範囲は不変なので非正規化してよい（FR-203） | 投稿・版・削除 |
| `feed_follow_edges` | `follower_account_id`, `followee_account_id`, `show_boosts` | PK `(follower_account_id, followee_account_id)`。`active` のフォローだけ | フォロー（自動承認）・承認・終了、フォローごとの設定 |
| `feed_exclusions` | `viewer_account_id`, `target_account_id`, `reason`（`enum(block, blocked_by, mute)`）, `expires_at`（NULL 可。ミュートの期限） | PK `(viewer_account_id, target_account_id, reason)` | ブロック（両方向の 2 行）・ブロックの解除・ミュート・ミュートの解除 |
| `feed_domain_mutes` | `viewer_account_id`, `domain` | PK `(viewer_account_id, domain)`。サブドメインにも効く（FR-304） | ドメインのミュート・解除 |
| `feed_author_restrictions` | `account_id`, `restriction`（`enum(profile_view, visibility_ceiling_private, public_discovery)`）, `effective_until`（NULL 可） | PK `(account_id, restriction)`（複数の処分の和集合をまとめ、最も遅い期限を持つ） | 制限・ライフサイクル（凍結・削除済みは `profile_view` として表す。削除は最終なので、削除済み（`deleted` / `remote_deleted`）の `profile_view` は `effective_until` が NULL） |
| `feed_domain_restrictions` | `domain`, `restriction`（同上）, `effective_until`（NULL 可） | PK `(domain, restriction)`。サブドメインにも効く | ドメイン単位の制限 |

- ドメイン（`author_domain` / `original_author_domain`）はローカルなら `''`、リモートなら投稿・ブーストの URI のホストです。ドメイン単位の制限の照合に使います。
- ブーストの行は、元投稿の行から作者・ドメイン・言語・版を写します。元投稿の版が進むと、元投稿とそのブーストの行の `language` と `revision_seq` をまとめて更新します（`revision_seq` 以下の版は無視します）。
- 削除された投稿の行は、投稿・そのブースト・宛先・タグをまとめて消します。削除された投稿への遅れた版やブーストは反映しません。
- `feed_entries` の挿入は、同じ `id` があれば失敗させます（投稿とブーストの ID は一度しか挿入されないので、projector の二重登録を検出できます）。
- `feed_entries` は元の公開範囲を持ちます。サイレンスによる上限（§5.4）は、読み取り時に `feed_author_restrictions` と合成して適用します。
  データを書き換えないので、解除すれば元に戻ります。
- `feed_author_restrictions` は、アカウントの削除（`account_lifecycle_events` の `deleted` / `remote_deleted`）で `profile_view` の行を持ちます。
- `feed_author_restrictions` / `feed_domain_restrictions` の処分による行は、`moderation` のイベント `moderation.account_restrictions_changed` / `moderation.domain_restrictions_changed` から作ります。
  イベントは処分と解除のたびに、その後に有効な制限（制限ごとに最も遅い期限）を載せるので、projector はそのアカウント・ドメインの 3 つの制限を、載っているものは書き、載っていないものは消します。
- 将来 fan-out-on-write に切り替える場合も、`home_feed`（閲覧者 × エントリ）をこのモジュールの投影として追加するだけで済みます（ADR-0013）。
