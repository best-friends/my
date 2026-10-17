# ADR-0035: 公開範囲 local の投稿は、フォロワーでないリモートのメンション先を tag に入れない

- Status: Accepted
- Date: 2026-10-17
- Deciders: @rosylilly
- Issue: #232

## Context（背景・課題）

[ADR-0017](0017-visibility-local.md) は `local` を「外部へは作者のフォロワーにしか届かない」公開範囲と定め、ActivityPub での表現を次のとおりにした。

- `to` は作者の followers コレクションだけ。`as:Public` は入れない
- 「メンションの `tag` は残すが、**フォロワーでないリモートのメンション先には配送しない**」

配送は shared inbox に集約する（ADR-0006、NFR-COST-5）。このため、メンション先と同じサーバーに作者のフォロワーが 1 人でもいれば、その投稿はメンション先のサーバーに届く。
2026-10-17 に Mastodon の main（4.8.0-dev）のソースで、followers 宛ての `Create` の `tag` に、フォロワーでも宛先でもないローカルの利用者への `Mention` があるときの扱いを確かめた（出典は [#232 の調査報告](https://github.com/best-friends/my/issues/232)）。

- 作者にそのサーバーのフォロワーがいれば、投稿を受け入れる（`app/lib/activitypub/activity/create.rb` の `related_to_local_activity?`）
- メンション先の利用者に `Mention` の行を作る（同 `process_mention`）
- 通知する。ただし「宛先に含まれないメンション」として `silenced` が付き、既定の通知ポリシーでは通知の一覧ではなく「フィルタされた通知」に入る（`app/services/fan_out_on_write_service.rb`、`app/services/notify_service.rb`）
- **メンション先の利用者は、その投稿を閲覧できる**。`private` の投稿は `owned? || following_author? || mention_exists?` で閲覧を許すため（`app/policies/status_policy.rb`）

つまり、ADR-0017 の表現のまま配送すると、「フォロワーでないメンション先には届けない」が、shared inbox を経由して破られる。ADR-0017 は Accepted なので、表現を変えるには新しい ADR が要る。

## Decision Drivers（判断基準）

1. `local` の約束（外部へはフォロワーにしか届かない、ADR-0017・FR-102）を、受信側の実装に依らず守ること
2. shared inbox への集約（NFR-COST-5）を諦めないこと
3. ローカルの閲覧者には、メンションが今までどおりリンクとして見えること
4. 実装が単純で、テストで確かめられること

## Considered Options（選択肢）

1. **文書の `tag` から、フォロワーでないリモートのアカウントへの `Mention` を除く**（本文のリンクは残す）
2. **メンション先のサーバーには、shared inbox ではなくフォロワーの個人の inbox に送る**
3. **ADR-0017 のまま（`tag` を残す）。受信側の実装に任せる**

## Decision（決定）

**選択肢 1 を採用する。**

- `local` 投稿の `Create` / `Update` の文書は、`tag` に、作者の承認済みのフォロワーでないリモートのアカウントへの `Mention` を入れない。
  ローカルのアカウントへの `Mention` と、フォロワーであるリモートのアカウントへの `Mention` は入れる。
- 本文の HTML のメンションのリンク（`<a href="…" class="u-url mention">`）はそのまま残す。受信側ではただのリンクとして表示される。
- 配送先の規則（フォロワーでないリモートのメンション先には送らない）は ADR-0017 のまま。
- フォロワーの判定は、`Create` の文書を組み立てるときの承認済みのフォローで行う。`Update` も同じ規則で組み立て直す（その時点でフォロワーになっていれば入る）。
- この ADR が Accepted になったら、ADR-0017 の「メンションの `tag` は残すが」の箇所はこの ADR に置き換わる（Decider が ADR-0017 に参照を付ける）。

### この決定を見直す条件

- 主要実装が「宛先に含まれない `tag` のメンション」を閲覧の許可に使わなくなったことを、#44 の往復確認で確かめられた場合（選択肢 3 でよくなる）

## Consequences（影響）

- 良い影響
  - shared inbox を使ったまま、フォロワーでないメンション先が、受信側のどの実装でも投稿を閲覧・通知できなくなる。
  - 規則が「文書を組み立てるときの絞り込み」1 か所で済み、フィクスチャのテストで確かめられる。
- 悪い影響・トレードオフ
  - リモートのフォロワーから見ると、メンションがメンションとして扱われない（通知もスレッドの参加者の扱いもない、ただのリンク）。投稿作成時の警告（ADR-0017、[web-client.md](../architecture/web-client.md)）で利用者に伝える。
  - `Update` の時点でフォロー関係が変わっていると、版によって `tag` が変わる。
- 後続の作業（Issue 化する）
  - 送信する文書の組み立てに絞り込みを入れ、フィクスチャのテストで `tag` を確かめる
  - #44 の往復確認に「`local` 投稿のフォロワーでないメンション先が、Mastodon で閲覧も通知もできない」ことを加える

## Pros and Cons of the Options（詳細比較・任意）

### 1. `tag` から除く（推奨）

- 良い: 判断基準 1〜4 を満たす。
- 悪い: リモートのフォロワーには、メンションの意味が伝わらない。

### 2. 個人の inbox に送る

- 良い: 文書を変えずに済む。
- 悪い: Mastodon は個人の inbox で受けても、`tag` のメンションから `Mention` の行を作る（受信した inbox に関わらず `process_mention` が動く）ので、判断基準 1 を満たさない。リクエスト数が増える（判断基準 2）。

### 3. ADR-0017 のまま

- 良い: 変更がない。メンションがリモートでもメンションとして扱われる。
- 悪い: Context のとおり、Mastodon ではフォロワーでないメンション先が投稿を閲覧できる（判断基準 1）。

## Open Questions / 要検証

- [ ] GoToSocial・Misskey・Akkoma で、宛先に含まれない `tag` のメンションが閲覧・通知に使われるか（#44）
