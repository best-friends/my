# ADR-0097: 絵文字ピッカーのグリッドを TanStack Virtual で仮想化する

- Status: Accepted
- Date: 2026-10-24
- Deciders: @rosylilly
- Issue: #1202

## Context（背景・課題）

[ADR-0091](0091-composer-emoji-picker.md) §3 の「性能」は、`@my/ui` の `EmojiPicker` を、まず仮想化せずにすべてを描いて計測すると決めた。
そのうえで、`@tanstack/react-virtual` を採らない理由を、描かない行があると React Aria の `GridList` が矢印キーでフォーカスを移す先の要素が無くなるためとした。
遅いときの手は、React Aria Components の `Layout` を継承した「節のあるグリッド」のレイアウトを `Virtualizer` に渡すことだった。

実装（#1147、#1148、#1149）の後、ピッカーは Unicode 絵文字の全集合（肌の色の異体を除いて 1,923 個）とカスタム絵文字を一度に描く。
1 つの絵文字が `GridListItem` の行とセルの要素になり、開いたピッカーは数千の要素を持つ。
CI の Web の E2E（`web-e2e.yml`）では、`e2e/emoji-picker.spec.ts` と `e2e/reactions.spec.ts` で、ピッカーを開いた画面の axe と横スクロールの検査が 30 秒の上限を超えて失敗した（run 40094615925、40096646446）。
手元でも負荷の高いときに同じ検査が上限を超える（#1149）。

決定者の回答（#1202、原文）:

> TanStack Virtual など使わないと絵文字ピッカーは負荷が高すぎると思います
> （仮想化の方式）TanStack Virtual

`@tanstack/react-virtual` は `@my/ui` の依存にすでにあり（`Feed`）、[ADR-0031](0031-ui-kit-behavior-dependencies.md) の許可リストに載っている。
TanStack Virtual で行を描かないと、ADR-0091 の指摘どおり `GridList` のキー操作は使えない。
そこで、仮想化した行でのキー操作とフォーカス（APG の grid）、支援技術に伝える行と列の位置、カテゴリのボタン・`aria-current`・検索との関係、React Aria に残す部品を、ここで決める。
関係する要件は FR-101（投稿の作成）と FR-116（絵文字リアクション）、NFR のアクセシビリティ（[ADR-0027](0027-ui-quality-gates.md)）である。

## Decision Drivers（判断基準）

1. **開く速さと DOM の大きさ**: 開いたピッカーの要素の数が、全集合の大きさではなく、表示している範囲で決まる。CI の E2E が既定のワーカー数で通る。
2. **APG の grid とアクセシビリティ**: ADR-0091 §6 のキー操作（Tab の順、書字方向に沿う矢印キー、Home / End、Enter・Space・押下で選ぶ）を保ち、描いていない行があっても支援技術が位置と総数を分かる。
3. **ADR-0091 の振る舞いを保つ**: カテゴリのボタン（押すとその節へスクロールして最初の絵文字にフォーカス、`aria-current`）、順位付きの検索と件数の告知、肌の色、Shift で続けて選ぶ、RTL、呼び出し側の API。
4. **依存と保守**: ADR-0025 / ADR-0031 の外の依存を足さない。自前で持つキー操作のコードを小さく保ち、テストで固める。

## Considered Options（選択肢）

### 仮想化の方式

1. **TanStack Virtual で行を仮想化し、グリッドのキー操作を `@my/ui` で持つ**: 節の見出しの行と、幅から決めた N 列の絵文字の行を 1 本の行の並びにし、`useVirtualizer` で見える行と前後の数行だけを描く。
   `GridList` をやめ、APG の grid のキー操作を `EmojiPicker` に書く。`Feed` が同じライブラリで、フォーカスのある項目を範囲の外でも描く仕組み（`rangeExtractor`）をすでに持つ。
2. **React Aria の `Virtualizer` に節のあるグリッドの `Layout` を自作する**（ADR-0091 §3 の次の手）: `GridList` のキー操作・選択・フォーカスの扱いをそのまま使える。
   ただし、React Aria Components 1.21.1 の `GridLayout` は節を配置しないので、`Layout` の抽象（`getVisibleLayoutInfos`・`getLayoutInfo`・`getContentSize` と、キー操作が使う `getKeyAbove` などの位置の問い合わせ）を自分で書き、React Aria の内部の版の変化に合わせて保守する。
3. **仮想化せず、節をタブで分けて描く**（ADR-0091 の覆る条件）: 一度に描くのは 1 つの節だけになる。ただし最大の節（人と体、顔と感情）だけで数百の絵文字があり、1 本のグリッドを続けてスクロールする ADR-0091 §3 の形を崩す。
4. **仮想化せず、画面の外の節に `content-visibility: auto` を付ける**: 描画を省けるが、要素の数は減らず、axe の検査の時間も変わらない。web-client §12 のとおり、Chromium は開いたときにすべてのフォントの分割を読む。

### 仮想化した行でのフォーカス

1. **roving tabindex**: フォーカスのある絵文字のセルだけを `tabindex="0"`、ほかを `-1` にし、矢印キーで DOM のフォーカスをセルに移す。フォーカスのあるセルの行は、表示範囲の外でも描いておく。
2. **`aria-activedescendant`**: フォーカスはグリッド（または検索欄）の要素に置いたまま、`aria-activedescendant` で今のセルを指す。
   検索欄から矢印キーでグリッドを動かす（combobox に近い）形にもできるが、指す先のセルは DOM にある必要があるので、範囲の外でも描くことは roving tabindex と変わらない。

## Decision（決定）

**仮想化の方式は選択肢 1（TanStack Virtual）、フォーカスは roving tabindex を採用する**（決定者の回答「TanStack Virtual」）。
ADR-0091 §3 の「性能」の項（仮想化せずにすべてを描いて計測し、遅ければ `Virtualizer` の `Layout` を書く。`@tanstack/react-virtual` は使わない）をこの ADR が置き換える。
ADR-0091 §3 のそのほかの項（並び・カテゴリのボタン・よく使う・検索・肌の色）と §4〜§7 の決定は変えない。ただし §4 の「列の数はコンテナクエリで決める」は、下の「行の並び」が置き換える。

### 1. 行の並び

- グリッドは 1 本の行の並びにする。節ごとに、見出しの行を 1 つと、その節の絵文字を N 個ずつに切った絵文字の行を続ける。検索中は「検索結果」の見出しの行と結果の絵文字の行だけになる（ADR-0091 §3）。
- 列の数 N は、グリッドの内側の幅から計算する。`ResizeObserver` でグリッドの幅と、画面に出さない見本のセル 2 つ（`aria-hidden`、同じクラス）の大きさと間隔を測り、`N = floor((幅 + 間隔) / (セルの大きさ + 間隔))`（1 以上）とする。
  セルの大きさはフォントの大きさの設定と `pointer-coarse:` で変わる（ADR-0091 §4・web-client D4 の 40 px / 44 px）ので、固定の値を使わずに測る。幅を測れない環境（jsdom）では N = 8（22 rem のポップオーバーの列の数）とする。
  ADR-0027 の「部品はコンテナクエリ」は、部品が画面ではなく自分の幅に応じることを求めるもので、この計算も部品の幅に応じる。
- 行の高さは固定にする。絵文字の行はセルの高さと間隔、見出しの行は見本の見出しの高さで、どちらも同じ測り方で得る。行ごとの `measureElement` は使わない。
  高さが決まっているので、スクロールの位置から描く範囲と、見出しの位置が計算だけで決まる。
- グリッドの高さは今の上限（`max-h-64`）のまま、中身が短ければ縮む。

### 2. 仮想化

- `@tanstack/react-virtual` の `useVirtualizer` で、グリッドの要素をスクロールの要素にする。行は絶対配置（`translateY`）で、高さの合計を持つ入れ物の中に置く。
- `overscan` は 4 行（`Feed` と同じ）。見える行（上限の高さで 6 行ほど）と前後の 4 行ずつを描く。
- **フォーカスのあるセルの行は表示範囲の外でも描く**。`rangeExtractor` で、既定の範囲（`defaultRangeExtractor`）に今のセルの行を足す（`Feed` と同じ。web-client Y6 と同じ考え）。
  ホイールやタッチでスクロールしてフォーカスのあるセルが見えなくなっても、その要素は DOM に残り、フォーカスは失われない。スクロールして戻ると同じ要素がそのまま見える。
- 仮想化を読むフックは、`Feed` と同じく `"use no memo"` で React Compiler から外す（web-client R4）。仮想化の結果はそのフックの戻り値として読み、ほかのコードはコンパイラの対象のままにする。
- スクロールの滑らかさは CSS の `scroll-behavior` ではなく、TanStack Virtual の `scrollToIndex` の `behavior` で決める。CSS で滑らかにすると、仮想化の側が目標の位置に着いたかを確かめる処理と食い違うためである。
  カテゴリのボタンによる移動だけを、`prefers-reduced-motion: no-preference` のときに `smooth` にする（web-client D6）。キー操作による移動は即座にスクロールする。

### 3. ARIA の構造

- グリッドは `role="grid"`（`aria-label` は今の `labels.grid`）に `aria-rowcount`（見出しの行を含む行の総数）と `aria-colcount`（N）を付ける。
  APG の grid は、DOM に無い行や列があるときに総数を `aria-rowcount` / `aria-colcount`、位置を `aria-rowindex` / `aria-colindex` で示すことを求める（2026-10-24 に [APG の Grid Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/grid/) で確認）。
- 描いた行は節ごとに `role="rowgroup"`（`aria-label` は節の見出し）にまとめる。見出しの行が範囲の外で DOM に無くても、節の名前が伝わるように `aria-labelledby` ではなく `aria-label` にする。
  行の入れ物（高さの合計を持つ要素）は `role="presentation"` にする。
- 見出しの行は `role="row"`（`aria-rowindex`）の中の `role="rowheader"`（`aria-colspan` は N）にする（今の `GridListHeader` と同じ形）。見出しはフォーカスを受けない（APG の「機能の無い見出しのセル」）。
- 絵文字の行は `role="row"`（`aria-rowindex`）、各絵文字は `role="gridcell"`（`aria-colindex`、`aria-label` は ADR-0091 §6 の名前）にする。カスタム絵文字の画像は今と同じく中に置く。
- 一致する絵文字が無いときは、今と同じく 1 つの行とセルに文言（`labels.empty`）を出す。
- 件数の告知（`role="status"`）は今のまま。

### 4. キー操作とフォーカス（roving tabindex）

- グリッドの中で Tab で止まるのは 1 つのセルだけにする。初めは最初の絵文字、その後は最後にフォーカスのあったセルが `tabindex="0"` になる。Tab の順（検索欄 → 肌の色 → カテゴリのボタンの列 → グリッド）は ADR-0091 §6 のまま。
- 今のセルは、節と絵文字の鍵（今の `GridListItem` の ID と同じ）で持つ。列の数が変わって行の切り方が変わっても、同じ絵文字に留まる。検索の文字列が変わって今のセルが結果に無くなったら、最初の絵文字を今のセルとみなす（state を Effect で直さず、描画の中で導く。web-client R6）。
- 矢印キーで DOM のフォーカスをセルに移す。移した先の行が描かれていなければ、今のセルを先に変えて（`rangeExtractor` がその行を描く）、描かれた後にフォーカスを移す。どちらも `focus({ preventScroll: true })` で移し、`scrollToIndex`（`align: "auto"`）でその行を見える位置に入れる。
- キーと動き（左右は書字方向に沿う。RTL では入れ替わる。React Aria の `useLocale` の向きを使う）:

  | キー | 動き |
  | --- | --- |
  | → / ← | 並びの次 / 前の絵文字。行の端では次 / 前の行へ、節の境では次 / 前の節へ続く（APG の layout grid の任意の折り返し。今の `GridList` と同じ）。最初と最後で止まる |
  | ↓ / ↑ | 次 / 前の絵文字の行の同じ列（見出しの行は飛ばす）。その行が短ければ、その行の最後の絵文字。最後 / 最初の行で止まる。節の最初の行へ上がるときは、その節の見出しの行も見える位置に入れる |
  | Home / End | 今の行の最初 / 最後の絵文字（APG） |
  | Ctrl + Home / Ctrl + End | グリッドの最初 / 最後の絵文字（APG） |
  | Page Down / Page Up | 見えている高さに入る行の数（1 以上）だけ下 / 上の行の同じ列。行き先が見出しなら、その向きの次の絵文字の行。端を越えるときは最後 / 最初の絵文字の行 |
  | Enter / Space / 押下 | 選ぶ。Shift を押しながら（Shift+Enter・Shift+クリック）は閉じずに続ける。タッチでは続けない（ADR-0091 §5） |

  Home / End は、今の `GridList` のグリッドの最初 / 最後から、APG のとおり行の最初 / 最後に変わる。グリッドの最初 / 最後は Ctrl + Home / Ctrl + End が受け持つ。
  `GridList` の文字の入力による移動（typeahead）は持たない。名前で探すのは検索欄が受け持つ。
- 押下は React Aria の `usePress` で受ける。Shift の有無と、押したのがタッチかどうかを今と同じく `PressEvent` から読み、Enter と Space の扱いも React Aria にそろう。見た目の状態（フォーカスのリング・ホバー・押下）も `useFocusRing`・`useHover` の `data-*` で付ける（今のクラスのまま）。
- **roving tabindex を選ぶ理由**:
  - ピッカーの検索欄とグリッドは別の Tab の止まり場所で、文字の入力は検索欄、選ぶのはグリッドと分けている（ADR-0091 §6）。検索欄から矢印キーで動かす形にすると、IME の変換の候補を矢印キーで選ぶ操作と競合し、変換中かどうか（`isComposing`）でキーの意味を変えることになる。
  - DOM のフォーカスがセルにあれば、支援技術はフォーカスの移動として読み上げる。`aria-activedescendant` は、iOS の VoiceOver などの対応がフォーカスの移動より弱く（要検証。手動の検査は ADR-0027 の手動の層で行う）、ADR-0027 の手動の検査の対象（VoiceOver・TalkBack・NVDA）の全部で確かめる手間が大きい。
  - Shift で続けて選ぶとき、フォーカスを選んだ絵文字に残す（ADR-0091 §5）のは、DOM のフォーカスがそのまま表す。
  - 範囲の外でも今のセルの行を描くことは、どちらの方式でも要る。roving tabindex で増えるものはない。

### 5. カテゴリのボタンと `aria-current`

- カテゴリのボタン（React Aria の `Toolbar`、ADR-0091 §3）を押すと、その節の最初の絵文字を今のセルにしてフォーカスを移し、`scrollToIndex` でその節の見出しの行をグリッドの上端に合わせる（`align: "start"`）。
  最初の絵文字の行は、スクロールが着く前でも `rangeExtractor` で描かれているので、すぐにフォーカスを移せる。
- `aria-current="true"` は、スクロールの位置から導く。グリッドの上端の帯（高さの 10%、今と同じ）より上にある最後の見出しの行の節を、今の節とする。行の位置は計算で決まるので、`IntersectionObserver` は使わない。
  スクロールの位置が変わると仮想化が描き直すので、`aria-current` もスクロールに合わせて移る。検索中は今と同じくボタンを無効にし、`aria-current` を付けない。

### 6. React Aria に残すもの・やめるもの

- 残す: 検索欄（`@my/ui` の `TextField` の `type="search"`）、肌の色の `MenuTrigger` / `Menu`、カテゴリのボタンの `Toolbar`、`Button`・`Tooltip`、呼び出し側の `Popover`、セルの `usePress`・`useFocusRing`・`useHover`、書字方向の `useLocale`。
- やめる: `GridList`・`GridListSection`・`GridListHeader`・`GridListItem`。グリッドのキー操作・フォーカス・ARIA の属性は §3・§4 のとおり `EmojiPicker` が持つ。
- `EmojiPicker` の props と、`searchEmojis`・`skinToneVariant` などの公開する関数は変えない。呼び出し側（投稿フォーム・編集・プロフィール・リアクション）は変わらない。

### 覆る条件

- TanStack Virtual の版の更新で、`rangeExtractor` や `scrollToIndex` の振る舞いが変わり、フォーカスを保てなくなった場合は、選択肢 2（React Aria の `Virtualizer` と自作の `Layout`）を見直す。
- React Aria Components の `GridLayout` が節を配置するようになった場合は、キー操作を React Aria に戻せるかを見直す。
- 仮想化してもミドルレンジの Android でピッカーを開くのが 100 ms を超える場合は、ADR-0091 の覆る条件のとおり、節をタブで分けて描く形を見直す。

## Consequences（影響）

- 良い影響
  - 開いたピッカーの要素の数が、全集合の大きさに依らず、見える行と前後の数行の分（百数十のセル）になる。開く時間と axe の検査の時間が短くなる。
  - 初めて開いたときに読む絵文字のフォントの分割（ADR-0095）が、描いた行の絵文字のグループの分だけになる。ADR-0095 §4 の「すべての分割を読み込む」は、スクロールして出したグループの分だけになる。
  - 描いていない行があっても、`aria-rowcount` / `aria-rowindex` / `aria-colindex` で総数と位置が伝わる。
- 悪い影響・トレードオフ
  - グリッドのキー操作・フォーカスの移動・ARIA の属性を自前で持つ。React Aria の `GridList` が持っていた細かな扱い（typeahead など）は持たない。テスト（Testing Library のキー操作と Storybook の実ブラウザ）で固める。
  - React Compiler から外すフックが `Feed` のほかにもう 1 つ増える（web-client R4 の記述を更新する）。
  - Home / End が、グリッドの最初 / 最後から行の最初 / 最後に変わる（APG のとおり）。
  - ブラウザーのページ内検索（Ctrl+F）は、描いていない絵文字を見つけない。名前で探すのはピッカーの検索欄が受け持つ。
- 後続で必要になる作業（Issue 化する）
  1. `@my/ui` の `EmojiPicker` の実装、テスト（キー操作・節をまたぐ移動・カテゴリのボタン・検索・`aria-*` の数）、全集合のストーリー、E2E の更新と計測（#1202）。
  2. 文書の更新: [web-client §12](../architecture/web-client/12-compose.md) のピッカーの描き方、web-client R4 の「コンパイラから外すフック」（この ADR の PR で行う）。

## Pros and Cons of the Options（詳細比較・任意）

| | 良い点 | 悪い点 |
| --- | --- | --- |
| 1 TanStack Virtual | 依存はすでにある。`Feed` と同じ仕組み。行の高さが固定なので計算だけで範囲と位置が決まる | キー操作・フォーカス・ARIA を自前で持つ |
| 2 React Aria の `Layout` | `GridList` のキー操作をそのまま使える | 節のあるグリッドの `Layout` を自作し、React Aria の内部に合わせて保守する。決定者の判断と異なる |
| 3 タブで分ける | 仮想化が要らない | 1 つの節でも数百。1 本のグリッドの形を崩す |
| 4 `content-visibility` | 変更が小さい | 要素の数と検査の時間が変わらない。フォントの分割も読む |
| roving tabindex | 支援技術の対応が広い。IME と競合しない。Shift で続けるときのフォーカスが自然 | フォーカスのある行を範囲の外でも描く（どちらの方式でも要る） |
| `aria-activedescendant` | フォーカスを動かさずに済む | 支援技術の対応にばらつき（要検証）。検索欄から動かす形は IME と競合する |

## Open Questions / 要検証

- `aria-activedescendant` と roving tabindex の、iOS の VoiceOver と Android の TalkBack での読み上げの差（要検証。ADR-0027 の手動の層で、この ADR の roving tabindex の読み上げを確かめる）。
