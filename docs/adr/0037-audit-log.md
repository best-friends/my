# ADR-0037: 監査ログを処分のイベントとは別の追記専用の記録にする

- Status: Accepted
- Date: 2026-10-17
- Deciders: @rosylilly
- Issue: #327

## Context（背景・課題）

FR-704（監査ログ）と NFR-MOD-1 は、モデレーターと管理者の操作を、誰が・いつ・何に・なぜ行ったかとともに新しい順に引けることを求める。
いまの [data-model §10](../architecture/data-model.md#10-機能要件とテーブルの対応) は、監査ログを「各イベントテーブル」としている。
対象の操作は `moderation`（処分・通報）、`identity`（登録審査・ロール）、`instance`（設定・ルール）の 3 つのモジュールにまたがる。
各イベントテーブルを横断して読むと、`moderation` が他のモジュールのテーブルを読むことになり、ADR-0002 のモジュール境界に反する。

主要実装の例（2026-10-17 にソースで確認。出典は [#327 の調査報告](https://github.com/best-friends/my/issues/327)）:

- Mastodon は専用の `admin_action_logs`（操作者・操作・対象の型と ID・対象の表示名の写し・変更の内容）を持ち、約 60 種の操作を記録する。自動の消去はしない（[`Admin::ActionLog`](https://github.com/mastodon/mastodon/blob/main/app/models/admin/action_log.rb)、[`ActionLogFilter`](https://github.com/mastodon/mastodon/blob/main/app/models/admin/action_log_filter.rb)）。
- Misskey は `moderation_log`（操作者・種類・詳細）を、Pleroma / Akkoma は `ModerationLog` を持つ。

GDPR 32 条と個人情報保護法 23 条（安全管理措置）は、処理の記録を検証できる仕組みを求める（調査報告 §1, §3）。

## Decision Drivers（判断基準）

1. モジュール境界（ADR-0002）とデータモデルの規約（ADR-0020）を守ること
2. 監査ログの 1 行が、対象が消えた後も読めること（対象の表示名の写し）
3. 監査ログと元の操作が食い違わないこと（同じトランザクションで書く。ADR-0003）
4. CLI からの操作も記録すること（いまの CLI は操作者を NULL で残す）
5. 1 人で運用するインスタンスに過剰な仕組みを持ち込まないこと

## Considered Options（選択肢）

1. **専用の追記専用テーブル**: `moderation` が持つ `audit_log_entries`【イベント】に、各モジュールがトランザクション内のドメインイベントで書く
2. **各イベントテーブルを横断して読む**: 新しいテーブルを作らず、各モジュールが自分のイベントを監査ログの形で返す API を持ち、`moderation` が束ねる
3. **構造化ログに出すだけ**: DB には残さず、NFR-OPS-1 の JSON ログに出す

## Decision（決定）

**選択肢 1 を採用する。**

- `audit_log_entries` は `moderation` が持つ【イベント】で、カラムは `id`, `actor_kind`（`enum(user, cli, system, remote)`）, `actor_user_id`（NULL 可）, `action`（操作の enum）, `target_kind`（`enum(account, status, domain, report, appeal, user, role, setting, rule, signup_block)`）, `target_id`, `target_label`（記録時の対象の表示名の写し。`@user@host`・ドメイン・設定のキーなど）, `source_event_id`（元のイベントの行の ID）, `occurred_at`。
- 各モジュールのユースケースは、元のイベントと同じトランザクションで `AuditRecorded` のドメインイベントを発行し、`moderation` の購読者がそれを書く（`uow.Event`。他のモジュールの `internal/app` を呼ぶ既存の経路だけを使う）。
- 理由・メモの本文は写さず、`source_event_id` で元のイベントから引く。監査ログ自体には IP アドレスを残さない。
- 更新・削除の API と CLI を持たない。消去は ADR-0038 の保持期間を過ぎた行を消す定期ジョブだけが行う。
- IP アドレスとメールアドレスを表示した操作（アカウントの詳細で「表示」を押したとき）も `action = revealed_private_data` で記録する。
- CLI は操作者を `actor_kind = cli` で記録する。CLI の実行者の OS ユーザー名は残さない。
- 改ざん検知（ハッシュ連鎖など）は持たない。DB に直接書ける運営者は監査ログも書き換えられ、1 人で運用するインスタンスでは効果が薄いため。

この決定を覆す条件: 複数のモデレーターで運用するインスタンスで、運営者自身の操作を第三者が検証する要件が出た場合は、ハッシュ連鎖か外部への複製を足す ADR を書く。

## Consequences（影響）

- 良い影響: 監査ログの一覧は 1 つのテーブルのキーセットページングで引ける（NFR-MOD-8）。対象が消去されても `target_label` で読める。
- 悪い影響: 操作ごとに 1 行の書き込みが増える。新しい管理操作を足すとき、`action` の enum と発行を忘れないようにテストで検査する必要がある。
- `target_label` は個人データ（ハンドル）を含むので、保持期間（ADR-0038）の対象になる。
- 後続の作業: `audit_log_entries` と購読者、既存の管理操作（ドメイン単位の処分・通報・登録審査・設定）からの発行、管理 API・CLI・画面での一覧。

## Pros and Cons of the Options（詳細比較・任意）

- 選択肢 2 の強み: 二重の記録がなく、元のイベントが唯一の事実になる（ADR-0020 の規約 1 に最も素直）。弱み: 一覧のたびに複数のテーブルを合成して並べ替える必要があり、ページングが複雑になる。対象が消去されると読めなくなる。
- 選択肢 3 の強み: 実装が最小。弱み: ログの保管先が構成ごとに違い、管理画面から引けない（FR-704 を満たさない）。

## Open Questions / 要検証

- `action` の enum の最初の一覧（#327 のタスク分割で、既存の管理操作から洗い出す）
