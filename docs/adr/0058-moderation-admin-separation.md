# ADR-0058: モデレーションとサーバー管理の分離

- Status: Accepted
- Date: 2026-10-20
- Deciders: @rosylilly
- Issue: #648

## Context（背景・課題）

決定者の指示（#648、原文）: 「モデレーションとサーバー管理の分離・整理」。

いまの運用者の機能は、Web も API も 1 つの `/admin/*` にまとまり、ロールの権限だけで分かれている。

- Web: `app/web/src/routes/admin.*.tsx` の 16 画面（連合先・ドメインへの処分・dead job・監査ログ・登録の審査・登録の拒否リスト・アカウント・通報・異議申し立て・規約とポリシー・サーバーの設定）。
  権限の検査は `routes/-admin/adminRoute.tsx`、一覧は `features/admin/components/AdminMenu.tsx` の 1 つのリストで、ナビゲーションの「管理」（`/admin`）と設定の画面の「管理」の区画が同じリストを出す（[web-client.md §9・§14](../architecture/web-client.md)）。
  「管理」のバッジは、未解決の通報・承認待ちの登録・審査待ちの異議申し立て（FR-712）というモデレーションの件数を出す。
- API: `/admin/*` の平らな一覧（`reports`・`accounts`・`account_actions`・`appeals`・`registrations`・`signup_blocks`・`domain_actions`・`instances`・`dead_jobs`・`rules`・`instance_texts`・`audit_log`・`roles`・`counts`）。
  タグは 1 つの `admin`、スコープは `admin:read` / `admin:write` で、各操作がロールの権限を検査する（[client-api.md](../architecture/client-api.md)）。
  利用者が自分への処分を見る `/moderation/actions` は、利用者本人の名前空間（タグ `moderation`）である。
- 権限（`internal/domain/identity/role.go`）: `manage_reports`・`manage_accounts`・`manage_domains`・`manage_rules`・`manage_settings`・`manage_roles`・`manage_invites`・`view_audit`・`manage_jobs`。
  組み込みのロールは `admin`（すべて）と `moderator`（`manage_reports`・`manage_accounts`・`view_audit`）。
- Go: モデレーションは `internal/app/moderation`（通報・処分・異議・ドメインへの処分・登録の拒否・監査ログ）、設定・規約・ルールは `internal/app/instance`、ロール・招待・登録は `internal/app/identity`、連合先の状態は `internal/app/federation`、dead job は `internal/app/queue` にある。
  `internal/interface/admin` は `doc.go` だけの空のパッケージで、どこからも使われない。

M4 では、運用者の機能がいくつも足される: インスタンス設定・ルール・招待（#624）、サーバー単位のテーマ（ADR-0059、#649）、お知らせ（#665）、webhook（ADR-0060、#650）、リレー（ADR-0061、#651）、カスタム絵文字の管理（ADR-0051、#618）。
このまま 1 つの `/admin` に足すと、モデレーターの毎日の仕事（通報を開いて処分する）と、管理者がたまに触る設定が 1 つのリストに混ざり、モデレーターには使えない項目が増える。置き場所を先に決めないと、#624 などの実装がそれぞれ別の場所を選ぶ。

決めること:

- モデレーションとサーバー管理の境界と、どちらにも関わるもの（監査ログ、ドメインへの処分、連合先）の置き場所
- Web の情報設計（経路・ナビゲーション・一覧）と、API の経路の分け方
- 権限の名前とロールの既定（FR-720）
- コードの置き場所（Go の module、Web の feature）

関連: FR-602（管理画面）、FR-701〜FR-721、[ADR-0036](0036-moderation-federation-propagation.md)、[ADR-0037](0037-audit-log.md)、[ADR-0039](0039-mobile-navigation-and-compose-entry.md)、[ADR-0002](0002-modular-monolith-hexagonal.md)、[ADR-0007](0007-client-api.md)。

### ほかの実装の分け方（2026-10-20 に一次資料で確認）

| | Web の分け方 | API の経路 | 権限とロールの既定 |
| --- | --- | --- | --- |
| Mastodon | 1 つの `/admin/*` の中で、メニューを「モデレーション」と「管理」の 2 つの見出しに分ける（[config/navigation.rb](https://github.com/mastodon/mastodon/blob/main/config/navigation.rb)）。モデレーション: 通報・異議・アカウント・ハッシュタグ・招待・連合先・メールと IP の拒否・警告の定型文・監査ログ。管理: ダッシュボード・設定・利用規約・ルール・ロール・お知らせ・カスタム絵文字・webhook・リレー | 平らな `/api/v1/admin/*`・`/api/v2/admin/*`（[docs](https://docs.joinmastodon.org/methods/admin/)） | 権限を moderation / administration / devops などの分類にまとめる（[user_role.rb](https://github.com/mastodon/mastodon/blob/main/app/models/user_role.rb) の `CATEGORIES`）。既定の Moderator は `view_dashboard`・`view_audit_log`・`view_feeds`・`manage_users`・`manage_reports`・`manage_taxonomies` だけで、連合（`manage_federation`）と拒否（`manage_blocks`）は Admin から（[config/roles.yml](https://github.com/mastodon/mastodon/blob/main/config/roles.yml)） |
| GoToSocial | 設定の画面の中で `/settings/moderation/*`（通報・アカウント・ドメインの制限と許可・拒否）と `/settings/admin/*`（インスタンスの設定とルール・カスタム絵文字・リレー・連合先・一回限りの操作・HTTP ヘッダーの許可）の 2 つの経路とメニューに分ける。モデレーションのメニューは `moderator`、管理のメニューは `admin` のロールで出す（[moderation/router.tsx](https://codeberg.org/superseriousbusiness/gotosocial/src/branch/main/web/source/settings/views/moderation/router.tsx)、[moderation/menu.tsx](https://codeberg.org/superseriousbusiness/gotosocial/src/branch/main/web/source/settings/views/moderation/menu.tsx)、[admin/menu.tsx](https://codeberg.org/superseriousbusiness/gotosocial/src/branch/main/web/source/settings/views/admin/menu.tsx)、[docs](https://docs.gotosocial.org/en/latest/admin/settings/)） | 平らな `/api/v1/admin/*` | ロールは user / moderator / admin の 3 段 |
| Misskey | 1 つの `/admin` の中で「管理」「設定」などの見出しに分ける（[pages/admin/index.vue](https://github.com/misskey-dev/misskey/blob/develop/packages/frontend/src/pages/admin/index.vue)） | 平らな `admin/*` のエンドポイントで、各エンドポイントが `requireModerator`（例: [admin/resolve-abuse-user-report](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/server/api/endpoints/admin/resolve-abuse-user-report.ts)）か `requireAdmin`（例: [admin/update-meta](https://github.com/misskey-dev/misskey/blob/develop/packages/backend/src/server/api/endpoints/admin/update-meta.ts)）を宣言する | ロールに「モデレーター」「管理者」の印と、絵文字の管理などのポリシーを持たせる |

3 つとも API は 1 つの `admin` の名前空間に置き、操作ごとの権限で分ける。Web は、GoToSocial が経路から、Mastodon と Misskey がメニューの見出しで分ける。

## Decision Drivers（判断基準）

1. **モデレーターが使えるものだけを見る**: モデレーターのナビゲーションと一覧に、使えない設定の項目を出さない。毎日の仕事（未対応の件数から通報・登録・異議を開く）が最短で届く
2. **権限が認可の唯一の根拠である**: 画面の出し分けも API の拒否も、ロールの権限から決まる。経路や名前空間を認可の代わりにしない
3. **これから足す機能の置き場所が一意に決まる**: #624・テーマ・お知らせ・webhook・リレー・カスタム絵文字が、迷わずどちらかに入る規則にする
4. **モジュールの境界（ADR-0002）を崩さない**: 「管理」のためだけにデータを持たないモジュールを作り、ほかのモジュールの内部に手を伸ばさない
5. **変更の量**: v0 は破壊的変更を許す（ADR-0012）が、意味のない改名で API と生成コードとテストを大きく動かさない

## Considered Options（選択肢）

### Web の情報設計

- **W1: 経路を `/moderation/*` と `/admin/*` に分け、ナビゲーションも 2 つの項目にする**（GoToSocial と同じ形）
- **W2: 経路は `/admin/*` のまま、一覧を「モデレーション」と「サーバー管理」の 2 つの見出しに分ける**（Mastodon と同じ形）
- **W3: いまのまま 1 つの一覧**

### API の経路

- **A1: `/admin/*` のまま、OpenAPI のタグを `admin-moderation` と `admin-server` に分ける**（経路とスコープは変えない）
- **A2: モデレーションを `/moderation/*`、サーバー管理を `/admin/*` に分け、スコープも `moderation:*` と `admin:*` に分ける**
- **A3: `/admin/moderation/*` と `/admin/server/*` の 2 段にする**

### Go のコードの置き場所

- **G1: サーバー管理の機能は、データを持つモジュールに置く**（instance・identity・federation・queue など）。管理だけのモジュールは作らない
- **G2: `internal/app/admin` のモジュールを作り、設定・お知らせ・webhook・リレーなどを集める**

## Decision（決定）

**W1 + A1 + G1** を採る。

### 1. 境界

- **モデレーション**は、人・投稿・リモートのサーバーについての判断と処分である。誰が参加でき、何が見えるかを個別の事案ごとに決める。
- **サーバー管理**は、サーバー自身の設定・サーバーが出す中身・運用・外部との連携である。サーバー全体に一度に効く。

| 区分 | 画面（Web の経路） | 権限 |
| --- | --- | --- |
| モデレーション | 通報（`/moderation/reports`） | `manage_reports` |
| モデレーション | 異議申し立て（`/moderation/appeals`） | `manage_accounts` |
| モデレーション | アカウントの検索と管理画面・処分（`/moderation/accounts`） | `manage_accounts` |
| モデレーション | 登録の審査（`/moderation/registrations`） | `manage_accounts` |
| モデレーション | 登録の拒否リスト（`/moderation/signup-blocks`、#626 の BAN からの作成を含む） | `manage_accounts` |
| モデレーション | ドメインへの処分（`/moderation/domain-actions`、#641 のインポートとエクスポートを含む） | `manage_domains` |
| モデレーション | 連合先（`/moderation/instances`。サーバーごとの配送の状態と再開、そのドメインへの処分への入口） | `manage_domains` |
| モデレーション | 招待（`/moderation/invites`、#624） | `manage_invites` |
| モデレーション | 監査ログ（`/moderation/audit-log`） | `view_audit` |
| サーバー管理 | サーバーの設定（`/admin/settings`。名前・説明・登録モード・既定の言語・タイムライン・連合の設定・処分したドメインの公開、#624） | `manage_settings` |
| サーバー管理 | 規約とポリシー（`/admin/documents`） | `manage_settings` |
| サーバー管理 | ルール（`/admin/rules`、#624） | `manage_rules` |
| サーバー管理 | ロール（`/admin/roles`。ロールと権限の一覧） | `manage_roles` |
| サーバー管理 | テーマ（`/admin/theme`、ADR-0059） | `manage_settings` |
| サーバー管理 | お知らせ（`/admin/announcements`、#665） | `manage_announcements`（新設） |
| サーバー管理 | カスタム絵文字（`/admin/custom-emojis`、ADR-0051） | `manage_custom_emojis`（新設） |
| サーバー管理 | webhook（`/admin/webhooks`、[ADR-0060](0060-instance-webhooks.md)） | `manage_webhooks`（新設） |
| サーバー管理 | リレー（`/admin/relays`、ADR-0061） | `manage_settings` |
| サーバー管理 | dead job（`/admin/dead-jobs`） | `manage_jobs` |
| サーバー管理 | 費用（`/admin/finance`。費用の項目の一覧とフォーム、週・月・年の集計、請求の取り込みの状態、[ADR-0067](0067-finance-cost-records.md)） | `manage_finance`（新設） |

どちらにも関わるものの置き場所:

- **監査ログ**はモデレーションに置く。処分の記録が大半で、`view_audit` は既定でモデレーターも持つ。設定とロールの変更も同じログに載り（FR-704）、サーバー管理の画面は自分の変更の行へ絞り込んだ監査ログにリンクする。経路は 1 つにする。
- **ドメインへの処分**はモデレーションに置く（FR-702 の処分であり、通報からも出す）。サーバーの全員に効くので、既定のモデレーターは `manage_domains` を持たない（4.）。
- **連合先**（federation.md §7.3 の配送の状態）はモデレーションに置き、いまと同じ `manage_domains` で出す。ドメインへの処分を決める人がそのサーバーの様子を見る画面で、詳細の「モデレーション」の区画からそのドメインへの処分（`/moderation/domain-actions?domain=…`）にリンクする。Mastodon も連合先をモデレーションに置き、配送の再開をそこに持つ。
- **招待**はモデレーションに置く。誰がサーバーに参加できるかを個別に決める点で、登録の審査と登録の拒否リストと同じ仕事である。登録モード（承認制・招待制）の切り替えはサーバーの設定に残る。
- **アカウントのロールの付与と剥奪**（FR-720）は、いまと同じくアカウントの管理画面（`/moderation/accounts/{id}`）の「ロール」の区画に置き、`manage_roles` を持つ閲覧者にだけ出す。ロールそのものの一覧（`/admin/roles`）はサーバー管理に置く。

### 2. Web の情報設計（W1）

- 経路を `/moderation/*` と `/admin/*` の 2 つの木に分ける。`/moderation` と `/admin` は、それぞれの区分で閲覧者が使える画面の一覧を出す（いまの `AdminPages` と同じ型）。使える画面がない区分は、その理由を `EmptyState` で出す。
- ナビゲーションは、モデレーションの権限（`manage_reports`・`manage_accounts`・`manage_domains`・`view_audit`）のどれかを持つ閲覧者に「モデレーション」、サーバー管理の権限のどれかを持つ閲覧者に「管理」を、設定の後に「モデレーション」「管理」の順で足す。
  レールとサイドバーとメニューのシートに出し、タブバーには出さない（ADR-0039 はメニューの中身をこの決定に任せている）。
- 未対応の件数（FR-712）のバッジは「モデレーション」に付ける。「管理」には件数を付けない。
- 設定の画面の「管理」の区画は、「モデレーション」と「管理」の 2 つの区画にする。
- 1 つの画面は 1 つの経路だけを持ち、両方の区分には置かない（`aria-current` が 1 つに決まる）。

### 3. API（A1）

- 経路は `/admin/*` のまま変えない。`/admin` はスタッフの操作の名前空間で、利用者本人の `/moderation/actions` と区別する。スコープも `admin:read` / `admin:write` のままにする。
- OpenAPI のタグを `admin-moderation`（上の表のモデレーションの操作と `/admin/counts`）と `admin-server`（サーバー管理の操作）に分け、[client-api.md](../architecture/client-api.md) のリファレンスと生成コードのまとまりを区分に合わせる。
- 新しいサーバー管理の操作は `/admin/<資源>`（`/admin/announcements`・`/admin/webhooks`・`/admin/relays`・`/admin/custom_emojis`・`/admin/settings` など）、新しいモデレーションの操作（`/admin/invites` など）も `/admin/<資源>` に置き、タグで区分を示す。
- 認可はいまのとおり、スコープの後に操作ごとの権限で決める（判断基準 2）。

### 4. 権限とロールの既定

- 権限に区分（`moderation` / `server`）を持たせ、`internal/domain/identity` に区分を返す関数を置く。ナビゲーションの出し分けと、`my admin role` などの CLI の表示がこれを使う。
  - モデレーション: `manage_reports`・`manage_accounts`・`manage_domains`・`manage_invites`・`view_audit`
  - サーバー管理: `manage_settings`・`manage_rules`・`manage_roles`・`manage_jobs`・`manage_announcements`・`manage_custom_emojis`・`manage_webhooks`
- 新しい権限は、その機能の Issue が足す（`manage_announcements` は #665、`manage_custom_emojis` は ADR-0051、`manage_webhooks` は ADR-0060）。テーマとリレーは設定の一部として `manage_settings` を使い、権限を分けない（ADR-0061 の決定に合わせる）。
- 組み込みのロールの既定: `admin` はすべての権限、`moderator` は `manage_reports`・`manage_accounts`・`view_audit`（いまと同じ）。モデレーターには「管理」が出ない。
  モデレーターはドメインへの処分（`manage_domains`）と招待（`manage_invites`）を既定では持たない。ドメインへの処分はサーバーの全員に効くので、管理者が決めるか、管理者がそれを持つロールを作って与える（Mastodon の既定の Moderator も連合と拒否を持たない）。
- 異議申し立ての審査は、処分と同じ人が審査するので `manage_accounts` のままにし、独立した権限（Mastodon の `manage_appeals`）は作らない。
- アカウントの検索と管理画面の権限（`manage_accounts`）は、メールアドレスと IP の開示（ADR-0038、FR-714）を含むので分けない。

### 5. コードの置き場所（G1）

- **Go**: サーバー管理の機能は、そのデータを持つモジュールに置く。設定・規約・ルール・テーマ・お知らせは `instance`、ロールと招待は `identity`、リレーと連合先の配送の状態は `federation`、dead job は `queue`、カスタム絵文字と webhook はそれぞれの ADR（ADR-0051・ADR-0060）が決めるモジュールに置く。`internal/app/moderation` はモデレーションだけを持つ。
  `internal/app/admin` は作らない。空の `internal/interface/admin` は消し、[overview §6](../architecture/overview.md#6-リポジトリ構成) からも消す。管理の HTTP の入口は `internal/interface/clientapi` だけにする。
  `clientapi` のハンドラのファイルは、いまの `admin_*.go` のまま区分ごとにまとめる。
- **Web**: `features/admin` を、モデレーションの `features/moderation` とサーバー管理の `features/admin` に分ける。両方が使う部品（`RequirePermission`・`RecordList`・`DetailSection`・`useFocusAfterChange`・権限の読み出し）は片方の `index.ts` から出す（web-client.md の規則 A3）。
  ルートは `routes/moderation.*.tsx` と `routes/admin.*.tsx`、権限の検査（`loadWithPermission`・`AdminError`）は両方の木が使う。

この決定を見直すきっかけ: 第三者のモデレーション用ツールに、サーバー管理を触れないトークンを渡す必要が出たとき（スコープを分ける A2 が要る）。モデレーターと管理者を分けて運用するサーバーがほとんどないと分かったとき（W2 で足りる）。

## Consequences（影響）

- 良い: モデレーターのナビゲーションには使える画面だけが並び、件数のバッジが仕事の入口と一致する。
- 良い: これから足す機能（#624・#665・ADR-0051・ADR-0059・ADR-0060・ADR-0061）の置き場所と権限が表から決まる。
- 良い: 新しい権限は、権限の分け方が運用で意味を持つもの（お知らせ・カスタム絵文字・webhook）にとどめ、設定の一部は `manage_settings` にまとめる。
- 良い: API の経路と生成コードの型名が変わらないので、変更は Web の経路・ナビゲーション・タグと権限の追加に収まる。
- 悪い: Web の経路が変わり、いまの `/admin/reports` などのリンク（通知のリンク、ブックマーク）は 404 になる。v0 なので転送は置かない（ADR-0012）。Web クライアントの中のリンク（モデレーターへの通知・監査ログの対象・連合先の詳細）は同時に直す。
- 悪い: 既定のモデレーターは連合先・ドメインへの処分・招待の画面を使えない。モデレーターに任せるサーバーは、`manage_domains` や `manage_invites` を持つロールを作って与える。
- 悪い: ナビゲーションの項目が、モデレーターと管理者を兼ねる人には 1 つ増える。
- 悪い: スコープを分けないので、`admin:*` のトークンはロールの権限の範囲で両方の区分を操作できる。第三者のツールを区分で縛れない。
- 悪い: Web の経路（`/moderation/*`）と API の経路（`/admin/*`）が一致しない。
- 後続: #659 がこの決定どおりに Web の経路・ナビゲーション・一覧・設定の画面の区画、OpenAPI のタグ、権限の区分、空の `internal/interface/admin` の削除、[web-client.md](../architecture/web-client.md) §9・§14 と [client-api.md](../architecture/client-api.md) と [data-model §4.1](../architecture/data-model.md) の更新を行う。
- 後続: #624・#626・#641・#665 と ADR-0051・ADR-0059・ADR-0060・ADR-0061 の実装は、上の表の経路と権限に置く。#659 より先に入るものは、いまの `/admin` に置いて #659 で移すか、#659 の後に入れるかを順番で決める。

## Pros and Cons of the Options（詳細比較・任意）

### W1: 経路とナビゲーションを分ける

- 良い: URL とナビゲーションの `aria-current` で、今どちらの区分にいるかが分かる。モデレーターの入口が短い。GoToSocial と同じ。
- 悪い: 経路が変わる。兼任の人にはナビゲーションの項目が 1 つ増える。

### W2: `/admin` の一覧を見出しで分ける

- 良い: 経路が変わらず、変更が一覧だけで済む。Mastodon と同じ。
- 悪い: モデレーターの入口は「管理」のままで、件数のバッジと名前が合わない。サーバー管理の項目が増えるほど一覧が長くなり、区分の境界が URL に出ない。

### W3: いまのまま

- 良い: 変更がない。
- 悪い: 決定者の指示に応えない。判断基準 1・3 を満たさない。

### A1: `/admin/*` のままタグで分ける

- 良い: 3 つの実装とも同じ形で、経路・スコープ・生成コードの型名が変わらない。認可は権限で決まるので、経路を分けても安全性は増えない。
- 悪い: 経路だけ見ても区分が分からない。トークンを区分で縛れない。

### A2: `/moderation/*` と `/admin/*`、スコープも分ける

- 良い: Web と API の経路がそろう。スコープでモデレーションだけのトークンを作れる。
- 悪い: 利用者本人の `/moderation/actions` と同じ名前空間にスタッフの操作が混ざる（本人の経路を改名するか、衝突を避ける規則が要る）。スコープが増え、OAuth の同意画面とクライアントの登録の扱いが変わる。ほかの実装に例がない。

### A3: `/admin/moderation/*` と `/admin/server/*`

- 良い: 経路で区分が分かり、本人の `/moderation/*` とも衝突しない。
- 悪い: すべての経路が長くなり、操作の名前と生成コードが動く。どちらにも関わる操作（`/admin/counts`、監査ログ）の置き場所をもう一度決める必要がある。

### G1: データを持つモジュールに置く

- 良い: ADR-0002 のモジュールの境界どおりで、データの所有者が一意。管理の機能は、そのモジュールのユースケースの 1 つになる。
- 悪い: 「サーバー管理」のコードが複数のモジュールに散る。区分は Web・タグ・権限の区分でだけ見える。

### G2: `internal/app/admin` に集める

- 良い: 管理のコードが 1 か所に見える。
- 悪い: 設定・ロール・連合などのデータの所有者が別のモジュールなので、`admin` はほかのモジュールの `internal/app` を呼ぶだけの層になるか、データを二重に持つ。ADR-0020 の「投影は 1 つのモジュールが持つ」と衝突しやすい。

## Open Questions / 要検証

- リモートのカスタム絵文字を無効にする操作の置き場所は、ADR-0051 の決定に合わせる。この ADR はカスタム絵文字の管理をサーバー管理（`manage_custom_emojis`）に置く。
