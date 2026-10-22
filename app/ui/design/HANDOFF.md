# my デザインシステム → @my/ui 実装の引き渡し

このフォルダは my ブランド(Playful Human)のデザインシステム一式です。目的は、**@my/ui(`app/ui`)の見た目をこのデザインシステムに合わせること**。コンポーネントの構造・props・アクセシビリティ・挙動は今の @my/ui が正で、変えるのは見た目(トークンと `tv()` のクラス)だけです。

## このフォルダの中身

| パス | 内容 | 使い方 |
| --- | --- | --- |
| `README.md` | ブランドガイド(色・文字・形・状態・アイコン・ロゴのルール) | 最初に読む |
| `tokens.json` / `tokens.css` | デザインシステムのトークン(ブランド名 + @my/ui の意味トークン名) | 値の出典 |
| `proposed/theme.css` | **`app/ui/src/theme.css` の置き換え案** | 手順 1 |
| `proposed/tokens.ts` | **`app/ui/src/tokens.ts` の置き換え案**(新しい色の検査ペア) | 手順 1 |
| `components/bundle.css` | 全コンポーネントの見た目(CSS、クラス名は @my/ui のコンポーネント名・スロット名に対応) | 手順 3 の答え |
| `components/<Name>/preview.html` | 各コンポーネントの見た目の見本(状態ごと) | 見た目の正 |
| `components/<Name>/README.md` | props・状態・トークンの使い方 | 実装時に参照 |
| `index.html` | 全プレビューの一覧(ライト / ダーク切替) | ブラウザで開く |
| `layouts/*.html` / `layouts.md` | ページレイアウト案 5 種(原寸) | 手順 5 |
| `assets/Logos/`, `assets/Icons/` | ロゴ SVG、@my/ui の Phosphor アイコン(参考用) | |

`components/Layout*` と `components/Cover` はページ / 表紙の見本で、@my/ui のコンポーネントではありません。`Card`, `Poll`, `AudioPlayer`, `Pagination`, `RailItem`, `Segmented`, `Tag` はブランドキット由来で、@my/ui にはまだありません(作るかは別途判断)。

## 角丸の対応(proposed/theme.css)

`--radius-*` は sm 6px / md 10px / lg 16px / xl 19px の 4 段階だけです(`rounded-2xl` 以上は存在しない)。ピルは `rounded-full`、それ以外の値(11px の入力欄、12px、24px、アバターの 21%)は任意値クラスで書きます。

## 手順

各手順を 1 コミットにし、`make web-check`(トークンのコントラスト検査を含む)と Storybook のテストが通ることを確認しながら進めてください。

### 1. トークンを置き換える

1. `proposed/theme.css` を `app/ui/src/theme.css` と diff し、置き換える。変わるのはパレット、意味トークンの色、`--font-serif`、角丸、エレベーションだけ。
2. `proposed/tokens.ts` で `app/ui/src/tokens.ts` を置き換える。追加した色(`accent-fill*`, `soft`, `*-bg`, `toast*`)の検査ペアと、`alt-overlay` の装飾扱い、`--font-serif` の上書き許可が入っている。
3. `make web-tokens-gen` で `themeTokens.gen.ts` を再生成する。
4. 新しい意味トークン:`soft`, `accent-fill`, `accent-fill-hover`, `accent-fill-fg`, `danger-bg`, `success-bg`, `warning-bg`, `toast`, `toast-fg`, `toast-action`, `toast-success`, `toast-danger`, `toast-muted`, `alt-overlay`, エレベーション `--elevation-card`(`shadow-card`)。
5. 確認ポイント:全ペアが両テーマで WCAG AA(デザインシステム側で計算済み、48 ペアすべて合格)。ダークのエレベーションは黒ベースにしたい(`tokens.json` の `elevation-*` の dark 値)が、サーバーテーマの shadow 形式が `light-dark()` を受けられない場合は今のままでよい。

### 2. 全体に効く変更

- **フォーカスリング:** `focus-ring` ユーティリティを `outline: 3px solid var(--color-focus); outline-offset: 2px;` に(今は 2px)。
- **body:** UI の本文は 15px / 1.7(`text-base` は 16px のままでよい。コンポーネント側で `text-[15px]` 等を使うか、`--text-base` を 0.9375rem にするかはチームで決める)。
- **ボタンのラベル:** 800 ウェイト(`font-extrabold`)。

### 3. コンポーネントの `tv()` を直す

`components/bundle.css` の各ブロックと `preview.html` が正解の見た目です。主な差分:

| コンポーネント | 変更 |
| --- | --- |
| Button | 全 variant を `rounded-full`、`font-extrabold`。`primary` = `bg-accent-fill text-accent-fill-fg`、hover `bg-accent-fill-hover` + `motion-safe:-translate-y-px`。`secondary` = `bg-soft text-accent`(枠線なし)。`danger` はそのまま。高さ sm 32 / md 40、左右 14 / 20px。任意で `outline` variant(フォローボタン用、README 参照) |
| ToggleButton | `rounded-[10px]`。選択時はアイコンを filled に(今の実装どおり) |
| Link | 下線オフセット 3px。navigation は `text-fg`、現在地は `bg-soft text-accent` のピル(RailItem 参照) |
| Field 系(TextField / TextArea / Select) | `fieldControl` を `rounded-[11px] bg-surface-sunken border-border-strong`、invalid は 2px 相当(`ring-1 ring-danger`)。ラベル `font-bold text-[13px]`、エラーに `WarningIcon filled` を前置 |
| Select | ポップオーバー `rounded-lg`(16px)、項目 `rounded-[10px]` |
| Checkbox | 箱 `rounded-[6px]`。Radio / Switch はトラック 44×26、つまみ 18px(`bg-surface-raised` → ダークでは card 色) |
| Avatar | `rounded-full` → `rounded-[21%]`。初期値の頭文字はセリフ体・太字、5 色のパステル(`tokens.json` の `avatar-*`)をアカウント ID で決め打ち |
| Badge | `accent` トーン = `bg-accent-fill text-accent-fill-fg` |
| Skeleton | `circle` を `rounded-[21%]`(Avatar と同じ形)、`rect` を `rounded-lg` |
| PostCard | フラットな行 → カード:`rounded-xl border border-border bg-surface-raised shadow-card`(`rounded-xl` = 新しい 19px)、カード間 12px。スレッド線でつながるカードは接する角を四角に。`highlighted` は `border-2 border-accent`(左端バーは使わない) |
| PostContent | blockquote を `rounded-[12px] bg-surface-sunken px-3.5 py-2`(左罫線をやめる)、code / pre も `bg-surface-sunken` |
| ContentWarning | 注意書きを `bg-warning-bg rounded-lg` の箱に、先頭に `WarningIcon filled`(`text-warning`)。ボタンは `bg-surface-raised` |
| MediaGrid / LinkCard | 角丸 16px。トグル・「説明なし」バッジはピル型 |
| MediaViewer | **作り直し**:ヘッダーと余白をなくし画像を枠いっぱいに、× を画像右上の丸ボタンに(タイトルは `sr-only` で残す)、ページ送り「‹ 2 / 3 ›」を枠の外・下中央に。代替テキストがある画像だけ右下にノートアイコンのボタン(`aria-expanded`/`aria-controls`)、押すと `bg-alt-overlay text-toast-fg` で全面に表示。**`AltTextIcon`(Phosphor `PiNote`)を `Icons.tsx` に追加**、`MediaViewerLabels` に `altText` を追加 |
| Dialog / Sheet / ConfirmDialog | パネル `rounded-xl`(19px)、タイトル 20px / 800、× は丸ボタン。Sheet は上端 `rounded-t-[24px]` |
| Menu / Popover | Menu `rounded-lg`(16px)/ Popover `rounded-xl`(19px)、項目 `rounded-[10px]` |
| Tooltip | `bg-toast text-toast-fg rounded-[10px] font-semibold`(枠線なし)。ダークは `ring-1 ring-border` |
| Toast | 濃いピル `bg-toast text-toast-fg rounded-[18px] shadow-lg`。トーンは左端バーではなく先頭アイコン(neutral `AboutIcon` / success チェック `text-toast-success` / danger `WarningIcon filled` `text-toast-danger`)。アクション `text-toast-action font-extrabold`、× は `text-toast-muted` |
| StatusMessage | 左端バー → 色付きの箱:success `bg-success-bg` + チェック(`text-success`)、neutral `bg-surface-sunken` + `AboutIcon` |
| RegionError | 左端バー → `bg-danger-bg rounded-lg` + `WarningIcon filled`(`text-danger`)、再試行ボタンは `bg-surface-raised` |
| OfflineBanner | 塗りつぶしの `bg-warning` → `bg-warning-bg text-fg` + `WarningIcon filled`(`text-warning`) |
| EmptyState | 見出しをセリフ体 22px / 500、イラストを 88px のクリーム色タイル(`bg-surface-sunken`、`rounded-[24px]`、`text-accent`) |
| NewItemsButton | primary(= accent-fill のコーラル + 黒文字)のピル + `shadow-md` |
| Tabs | 選択中の下線 3px |

### 4. Storybook で確認

各ストーリーを `components/<Name>/preview.html`(`index.html` で一覧)と見比べ、ライト / ダーク両方で確認。play 関数のテストが通ること。

### 5. レイアウト(別タスク・要相談)

`layouts/` の 5 案(LP、情報ページ、タイムライン、設定、管理画面)はアプリ側(`app/`)のルートの話なので、@my/ui の作業とは分けてください。**タイムラインの新着自動挿入は、ADR-0027(新着は読者が押すまで保留)と方針が変わるため、実装前に ADR を更新して決めること。** 自動挿入する場合も、読んでいる位置を動かさない(スクロールアンカー)ことと、自動更新を止める設定(WCAG 2.2.2)が条件です。

## 判断が必要な点(実装前に人に確認)

1. タイムラインの新着自動挿入(上記)。
2. `outline` ボタン variant と、ブランドキット由来のコンポーネント(Card, Poll, AudioPlayer, Pagination, RailItem, Segmented, Tag)を @my/ui に加えるか。
3. 本文サイズを 15px にするか(`--text-base` を変えるとサーバーテーマの範囲検査にも影響)。
