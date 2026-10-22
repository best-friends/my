The personal settings page: a section menu beside one pane of settings that apply as soon as they change.

**Structure (PC):** the app's left rail (設定 current) → a heading block (eyebrow 「Personal settings」, a serif `page-title` 「自分らしい場所に、整える。」) → a two-column body: a sticky 220px menu Card (プロフィール, アカウント, プライバシー, 通知, 表示とテーマ, ミュートとブロック — RailItem style, current in `soft`) and the pane Card (title 21px, one muted sentence, then Switch rows divided by `border`, Selects and fields).

- **Saving:** switches and selects save at once and confirm with a StatusMessage 「保存しました」 in the pane; only forms with typed text (profile, password) get a 「変更を保存」 primary button at the bottom, with the field errors inline.
- **Phone:** the menu becomes its own page (a list of the sections); each section opens full-width with a 「← 設定」 back link at the top, above the tab bar (メニュー current).
- One pane at a time; no tabs inside a pane. Destructive actions (アカウントの削除) sit at the end of アカウント behind a ConfirmDialog.
