# ADR-0036: モデレーションの処分を連合先へ伝える方法

- Status: Proposed
- Date: 2026-10-17
- Deciders: @rosylilly
- Issue: #327

## Context（背景・課題）

処分（FR-703, FR-713）とドメイン単位の処分（FR-702）は、いまはこのサーバーの中でだけ効く（[federation.md §11](../architecture/federation.md#11-モデレーションとの連携)）。
ローカルのアカウントを凍結しても連合先には投稿とプロフィールが残り、ドメインを凍結してもフォロー関係は残る。通報の転送（FR-709）の形も決まっていない。
FR-716・FR-717 と NFR-MOD-3 を実装する前に、どの処分で何を送るかを決める必要がある。

主要実装の挙動（2026-10-17 にソースで確認。出典は [#327 の調査報告](https://github.com/best-friends/my/issues/327)）:

- Mastodon はローカルのアカウントを凍結すると、Actor に `suspended: true` を付けた `Update` を届く限りの inbox に送り、解除で再び `Update` を送る。
  データは 30 日後に消し、そのとき `Delete` を送る（[`SuspendAccountService`](https://github.com/mastodon/mastodon/blob/main/app/services/suspend_account_service.rb)、[`AccountDeletionRequest`](https://github.com/mastodon/mastodon/blob/main/app/models/account_deletion_request.rb)、[`suspended`](https://docs.joinmastodon.org/spec/activitypub/#suspended)）。
- Mastodon はリモートのアカウントを凍結すると、そのアカウントのローカルの利用者へのフォローに `Reject(Follow)` を送って消す。ドメインの凍結ではフォローを両方向とも消し、影響を受けたローカルの利用者に `severed_relationships` の通知を出す。解除しても戻さない
  （[`BlockDomainService`](https://github.com/mastodon/mastodon/blob/main/app/services/block_domain_service.rb)、[管理マニュアル](https://docs.joinmastodon.org/admin/moderation/#server-wide-moderation)）。
- Misskey はローカルの利用者を凍結すると、既知の shared inbox すべてに `Delete(Person)` を送り、解除で `Undo(Delete)` を送る。フォローは切れる（[`UserSuspendService`](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/core/UserSuspendService.ts)）。
- GoToSocial のドメインブロックは凍結と同じで、関係・投稿・メディアを消し、ほとんど元に戻せない（[domain blocks](https://docs.gotosocial.org/en/latest/admin/domain_blocks/)）。
- サイレンス（limit）は連合しない。GoToSocial・Misskey も通報の転送をインスタンス Actor から送る（[GoToSocial](https://docs.gotosocial.org/en/latest/federation/moderation/)）。通報の転送はインスタンス Actor から、`object` に対象の Actor と投稿の URI、`content` にコメントを入れた `Flag` を対象の inbox に送る（[Flag](https://docs.joinmastodon.org/spec/activitypub/#Flag)、[`ReportService`](https://github.com/mastodon/mastodon/blob/main/app/services/report_service.rb)）。

## Decision Drivers（判断基準）

1. 凍結・BAN した内容が、連合先でも読まれ続けないこと（被害の拡大を止める）
2. 解除できる処分（期限付きの凍結）を解除したとき、連合先でも元に戻せること
3. Mastodon・Misskey・GoToSocial が解釈できる既存の語彙だけを使うこと（ADR-0006）
4. 通報者の身元を連合先に出さないこと（NFR-MOD-5）
5. 送るアクティビティの数を、処分 1 件につきフォロワーのサーバー数に比例する範囲に抑えること（NFR-COST-5）

## Considered Options（選択肢）

1. **Mastodon と同じ形**: 凍結は `Update(Actor)` に `suspended: true`、BAN は猶予期間の後に `Delete(Actor)`、ドメインの凍結はフォローを切って記録する
2. **凍結でもすぐ `Delete(Actor)` を送る**: 連合先の複製をすぐに消せるが、解除しても連合先には戻らない
3. **何も送らない**: 実装は最小だが、凍結したアカウントの内容が連合先に残り続ける

## Decision（決定）

> 推奨案。

**選択肢 1 を推奨する。**

| 処分 | 送るもの | 宛先 |
| --- | --- | --- |
| ローカルの `suspend` / `ban` | `Update(Actor)`（`suspended: true`、名前・アイコン・自己紹介・プロフィール補足情報を空にした Actor） | 承認済みのリモートのフォロワーの inbox と既知の shared inbox（`Delete(Actor)` と同じ。[data-model §8](../architecture/data-model.md#8-削除と消去)） |
| ローカルの `suspend` の解除・期限切れ | `Update(Actor)`（元のプロフィール） | 同上 |
| ローカルの `ban` | 上の `Update` に加えて、`deletion_scheduled` の期日に `Delete(Actor)`（アカウント削除と同じ） | 同上 |
| ローカルの投稿への `delete_statuses` | 投稿ごとの `Delete`（本人の削除と同じ） | 投稿を配送した宛先 |
| ローカルの `limit` / `restrict_posting` / `force_sensitive` など | 送らない（以後の投稿が上限どおりの公開範囲で配送されるだけ） | — |
| リモートのアカウントの `suspend` | そのアカウントからローカルの利用者へのフォローに `Reject(Follow)`、ローカルの利用者からそのアカウントへのフォローに `Undo(Follow)` | そのアカウントの inbox |
| ドメインの `suspend` | 送らない（配送を止めるため）。両方向のフォローを `follow_terminations`（`reason = moderation`）で切る | — |
| 通報の転送（FR-709） | `Flag`（主体はインスタンス Actor、`object` は対象の Actor と投稿の URI、`content` はコメント） | 対象のアカウントの inbox |

- `suspended` は Mastodon 拡張（`toot:suspended`）の語彙で、`@context` は既存の Actor と同じものを使う。凍結中の Actor の GET は、`404` ではなく、空のプロフィールに `suspended: true` を付けた Actor を返す（鍵は返す。`Delete` の署名を検証できるようにするため）。
- 凍結と BAN の配送は、処分のトランザクションで outbox に積む（ADR-0003）。期限切れは読み取り時の判定なので、期限の時刻に `Update` を積む定期ジョブを置く。
- 切ったフォローは、処分を解除しても戻さない。ドメインの凍結で切れた関係は、影響を受けたローカルの利用者ごとに件数を記録して通知する（FR-717）。
- 通報の転送は、ローカルの通報者が転送を選んだ通報だけを、通報を受け付けたトランザクションで outbox に積む。通報者・IP・メールアドレスは送らない。返信先の第三者のサーバーへの転送はしない。
- 受信した `suspended: true` の `Update(Actor)` は、リモートのアカウントの表示に「相手のサーバーで凍結」として出すだけで、このサーバーの処分にはしない。

この推奨を覆す条件: Misskey または GoToSocial が `suspended` を解釈せず、連合先での表示が凍結前のまま残ることが相互運用テストで分かった場合は、凍結でも `Delete(Actor)` を送る選択肢 2 を見直す。

## Consequences（影響）

- 良い影響: 期限付きの凍結を解除すれば、Mastodon の連合先ではプロフィールが戻る。BAN と削除は既存の `Delete(Actor)` の経路を使い回せる。
- 悪い影響: `suspended` を解釈しない実装では、凍結中もキャッシュした投稿が残る。ドメインの凍結を解除しても関係は戻らず、利用者はフォローし直す必要がある。
- 凍結した Actor の GET が 404 から変わるので、[federation.md §11](../architecture/federation.md#11-モデレーションとの連携) と ActivityPub の Actor の章を更新する。
- 後続の作業: ローカルの処分の伝播、ドメインの凍結でのフォローの切断と通知、通報の転送（#327 のタスク分割を参照）。

## Pros and Cons of the Options（詳細比較・任意）

- 選択肢 2 の強み: 連合先の複製を確実に消せる（Misskey の方式）。弱み: Mastodon は `Delete(Actor)` を受けるとアカウントを消し、`Undo(Delete)` を解釈しないので、期限付きの凍結が Mastodon の連合先では実質の BAN になり、判断基準 2 を満たさない（`Undo(Delete)` の扱いは要検証）。
- 選択肢 3 の強み: 処分の誤りが連合先へ広がらない。弱み: 判断基準 1 を満たさない。

## Open Questions / 要検証

- Misskey と GoToSocial が `suspended: true` の `Update(Actor)` をどう扱うか（#319 の相互運用テストに足す）
- 凍結した Actor の `Update` を、フォロワー以外の既知の shared inbox にまで送るか（Mastodon は届く限り送る。費用と効果を測る）
