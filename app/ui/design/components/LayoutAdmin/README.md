System pages for moderators and administrators (通報, アカウント, ドメイン, ルール, サーバー設定): denser, wider, and plainly separate from the social app.

**Structure (PC):** a 240px admin sidebar on `surface-raised` (the App-Icon + 「管理」 + the server name as a Tag; sections with RailItem rows, counts as `danger` Badges; 「アプリに戻る」 at the bottom) → the work area, full width: page title 26px/800 with a one-line summary and secondary actions (書き出す) → a toolbar of `toolbar` Selects for filters and a pill search → a list-and-detail split: a table in a Card (12px/800 `fg-muted` column heads, `border` row rules, the selected row in `soft`, states as Tags) beside a sticky 320px detail Card (DescriptionList, then the actions) → Pagination.

- **Tone:** quieter than the app — no serif headings, no sparks, no hero; brand colour only for state and the one destructive path.
- **Every irreversible action** (停止, ドメインブロック, 削除) opens a ConfirmDialog with a ReasonField and, where it applies, a DurationSelect; its result stays on the page as a StatusMessage.
- **Phone:** a small top bar with the admin menu button (opens a Sheet) and the page title; the table becomes a stack of Cards, and the detail opens as its own page.
- Admin pages sit under their own route and never reuse the app's left rail, so it is always clear which mode you are in.
