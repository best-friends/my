# ADR-0095: 絵文字を Noto Color Emoji で描く

- Status: Accepted
- Date: 2026-10-24
- Deciders: @rosylilly
- Issue: #1145

## Context（背景・課題）

Web クライアントの Unicode 絵文字は、OS のフォントで描いている。`@my/ui` の `--font-sans`（`app/ui/src/theme.css`）は OS のフォントだけで、最後に `"Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji"` を並べる。
[ADR-0009](0009-internationalization.md) は「フォントは OS フォントを優先し」とし、[ADR-0091](0091-composer-emoji-picker.md) §2（B1）は絵文字も OS のフォントで描いて、OS が描けない版の絵文字を canvas で判定してピッカーから隠す。

決定者は、Web クライアントのすべての場所（投稿の本文・名前・ピッカー・リアクション・補完）で、Unicode 絵文字を Noto Color Emoji で描くと決めた（#1145 の聞き取り）。理由は次の 3 つである。

- 端末によって絵文字の形が違うと、同じ投稿やリアクションの見え方がそろわない。
- 古い OS では新しい絵文字が豆腐（□）になる。
- ADR-0027 のスクリーンショットの比較で、OS の絵文字に左右されない。

この ADR は、配る形式、作り方、ライセンス、読み込み方、フォントの並び、ADR-0091 と ADR-0092 への影響を決める。
ADR-0091 のピッカーの実装（#1147・#1148）より前に入れる必要があるので、今決める。関係する要件は FR-101（投稿）、FR-116（リアクション）、NFR の性能（初期の読み込み）である。

## Decision Drivers（判断基準）

1. **すべての場所・すべての対象のブラウザーで同じ形**: Chrome・Firefox・Safari、Tauri の WebView（iOS の WKWebView、Android の WebView）で Noto の形になる。
2. **CSP とプライバシー**: `font-src 'self'`（[ADR-0019](0019-security-baseline.md) §9、[ADR-0059](0059-design-tokens-and-instance-theme.md)）を変えず、第三者のオリジンに閲覧を知らせない。
3. **転送量**: 絵文字を含まない画面ではフォントを読まない。読むのは画面にある絵文字の分だけにする。
4. **版がそろうこと**: フォントがサーバーの `EMOJI_VERSION`（18.0）のすべての絵文字を描ける（ADR-0091 判断基準 1）。
5. **ライセンスと道具**: フォントが AGPL-3.0 の配布と両立する（ADR-0011）。生成の道具は生成のときだけ使い、サーバーに cgo や実行時の依存を足さない。

## Considered Options（選択肢）

1. **O1 COLRv1 だけを自前で配り、対応しないブラウザーは OS のフォント**: `googlefonts/noto-emoji` の `Noto-COLRv1.ttf` を `unicode-range` で分割した WOFF2 として同じオリジンから配る。
   COLRv1 を描けない Safari と WKWebView は `@font-face` を使えず、`--font-sans` の後ろの OS の絵文字（Apple Color Emoji）に落ちる。
2. **O2 ブラウザーごとに形式を選ぶ（COLRv1 と OpenType-SVG）**: O1 と同じ分割を、COLRv1 と OpenType-SVG の 2 つの形式で作る。
   1 つの `@font-face` の `src` に `tech(color-COLRv1)` と `tech(color-SVG)` の 2 つを並べ、ブラウザーが描ける方を選ぶ。
3. **O3 OS のフォントのまま**: ADR-0091 の B1 を続ける。
4. **O4 Google Fonts から読む**: `fonts.googleapis.com` の CSS と `fonts.gstatic.com` のフォントを使う。ブラウザーごとの形式の選び分けと分割は Google がする。

CBDT（ビットマップ。`NotoColorEmoji.ttf`）は、10.7 MB と最も大きい。
README によれば、macOS で CBDT を描けるのは Chrome だけで、Safari では描けない（2026-10-23 に noto-emoji の [README](https://github.com/googlefonts/noto-emoji/blob/e20cbc2bbec1926686be9f9bee7d1d2cfa1fea0e/README.md) で確認）。O2 の形式の候補から外す。

## Decision（決定）

**O2（COLRv1 と OpenType-SVG を自前で分割して配り、ブラウザーが `tech()` で選ぶ）を採用する。**
O1 は Safari と iOS（Tauri を含む）で判断基準 1 を満たさない。O3 は決定者の判断に反する。O4 は判断基準 2 に反する。
O2 は、O1 の分割に OpenType-SVG を足すだけで、すべての対象のブラウザーで同じ形になる。

### 1. フォントの出所と版

- **COLRv1**: `googlefonts/noto-emoji` のリリース `v2026-09-24-unicode18_0`（2D v2.057、Unicode 18.0）の `2D/fonts/Noto-COLRv1.ttf`（5.0 MB）を使う。
- **OpenType-SVG**: noto-emoji は SVG のフォントをビルドしない。`google/fonts` の `ofl/notocoloremoji/NotoColorEmoji-Regular.ttf`（v2.055、25 MB）を使う。このファイルは `COLR`・`CPAL`・`SVG` の表を併せ持つので、`COLR` と `CPAL` を外す。
- **確認したこと（2026-10-24）**: どちらのファイルも、Emoji 18.0 の `emoji-test.txt` の fully-qualified の 3,963 列をすべて 1 つのグリフに組む。HarfBuzz（`uharfbuzz`）で整形して確かめた。
  Google Fonts は、Chrome と Firefox の User-Agent に COLRv1 の表の WOFF2 を、Safari の User-Agent に SVG の表の WOFF2 を返す。どちらも 11 個に分けている。
  確認に使ったリリースは [v2026-09-24-unicode18_0](https://github.com/googlefonts/noto-emoji/releases/tag/v2026-09-24-unicode18_0) と [google/fonts の METADATA.pb](https://github.com/google/fonts/blob/e7db6ca603fd4862010383ff17ffd6cc4ffac1c0/ofl/notocoloremoji/METADATA.pb) である。
- 2 つのファイルはコミット（noto-emoji は `e20cbc2bbec1926686be9f9bee7d1d2cfa1fea0e`）と SHA-256 で固定し、ADR-0091 の `EMOJI_VERSION`・`CLDR_VERSION` と同じタスクで取得する。
  生成器は、`EMOJI_VERSION` の `emoji-test.txt` のすべての fully-qualified の列が両方のフォントで 1 つのグリフになることを確かめ、そうでなければ失敗する。
  `EMOJI_VERSION` を上げるのは、それを描ける Noto のリリースが出てからにする（判断基準 4）。

### 2. 分割と生成

- `emoji-test.txt` のグループごとに分ける。`Flags` は、国旗（地域指示記号の対、`U+1F1E6..1F1FF`）とそれ以外に分ける。
  各列はそのグループの分割に入れ、分割の `unicode-range` は、その分割の列に現れるすべての符号位置にする。ZWJ（`U+200D`）・`U+FE0F`・肌の色の修飾子は、複数の分割に重なって入る。
  重なり方は Google Fonts と同じである。Google Fonts の分割も `U+200D` を複数の `unicode-range` に入れている。分割をまたぐ ZWJ の列が別々のグリフに割れないように、列を丸ごと 1 つの分割に入れる。
- `fontTools.subset`（`pyftsubset` と同じもの）で、分割ごとに GSUB の閉包を保ったまま部分集合を作り、WOFF2 にする。
  COLRv1 の版は `SVG` の表を、SVG の版は `COLR` と `CPAL` の表を落とす。
- **大きさ（2026-10-24 に試しに生成して計測）**:

  | 形式 | 合計 | 最大の分割 | 顔と感情の分割 | Google Fonts の配信（11 分割の合計） |
  | --- | --- | --- | --- | --- |
  | COLRv1 | 1.96 MB | 国旗 693 KB | 75 KB | 2.08 MB |
  | SVG | 3.72 MB | 国旗 1,152 KB | 153 KB | 3.95 MB |

  COLRv1 は、分けないときの WOFF2 が 1.90 MB で、分割で増えるのは 3% である。
- **道具**: fontTools（MIT）、brotli（MIT）、SVG の部分集合に要る lxml（BSD-3-Clause）を、生成のときだけ使う。どれも AGPL-3.0 と両立する。
  リポジトリの道具に Python は無い（`.tool-versions`）。そこで、`uv` のロックファイルで版を固定した使い捨ての環境か、版を固定したコンテナで動かす。
  生成した WOFF2 と `@font-face` の CSS はコミットする。`scripts/task.sh check` とビルドに Python は要らない。サーバーには何も足さない。
- 置き場所は `@my/ui`（`app/ui/src/fonts/emoji/` と、`theme.css` から読む `emoji.css`）にする。タイポグラフィは `@my/ui` が持ち、Storybook のスクリーンショットも同じフォントで撮れる。
  Vite が URL にハッシュを付けて `dist/assets/` に出す。

### 3. `@font-face` とフォントの並び

- **ファミリー**: 分割ごとに、`font-family: "my Noto Color Emoji"` の `@font-face` を 1 つ置く。OS に入っている同じ名前の CBDT の Noto（Linux・Android）と区別するため、ファミリー名を変える。
- **`src`**: `src: url(<colrv1>.woff2) format("woff2") tech(color-COLRv1), url(<svg>.woff2) format("woff2") tech(color-SVG)` とする。
  ブラウザーは、描ける最初の項目を選ぶ（Firefox は COLRv1 に対応するので最初の項目を選ぶ）。
  対応する版は次のとおりである（2026-10-24 に MDN の [browser-compat-data](https://github.com/mdn/browser-compat-data/blob/8371d582c202efe1f21622ab8cfac6ab00a7b209/css/at-rules/font-face.json) と caniuse の [colr-v1](https://github.com/Fyrd/caniuse/blob/664a78d75bcf581e1c6279144ad66b797a1635ab/features-json/colr-v1.json) で確認）。

  | 機能 | Chrome | Firefox | Safari | 備考 |
  | --- | --- | --- | --- | --- |
  | `src` の `tech()` | 108 | 107 | 17 | |
  | COLRv1 | 98 | 107 | TP まで未対応 | iOS の Safari も 27.2 まで未対応。Android の Chrome・WebView は対応 |

- **`unicode-range`**: 次の符号位置は、本文の文字として使われるので外す。`#`・`*`・`0`〜`9`（キーキャップの基）と、`©`・`®`・`™`・`‼`・`⁉`・`↔` など `emoji-test.txt` で既定がテキストの表示の記号のうち、本文の記号として使われるものである。
  外した字の絵文字の表示（`#️⃣`、`©️`）は、OS の絵文字のフォントで描く。外す字の一覧は、生成器の定数として 1 か所に置く。
- **並び**: `--font-sans` と `--font-mono` の先頭に `"my Noto Color Emoji"` を置く。`unicode-range` が絵文字の符号位置だけなので、ほかの字には効かない。
  先頭に置かないと、`☀`・`♠`・`❤` のように CJK のフォントや Segoe UI が字形を持つ字が、そちらのフォントで描かれる。末尾の OS の絵文字のフォントは、フォントを読めないときのために残す。
  ADR-0009 の「フォントは OS フォントを優先し」は、絵文字の符号位置についてだけこの ADR が置き換える。本文の文字は OS のフォントのままにする。
- **`font-display: swap`**: 読み込み中は OS の絵文字で描き、届いたら入れ替える。`block` にすると、遅い回線で絵文字が最大 3 秒見えない。
  入れ替えで行の高さが変わらないように、`@font-face` に `ascent-override`・`descent-override` を付けて OS の絵文字に近づける（値は実装で決める）。
- **キャッシュ**: Service Worker（ADR-0028）は今 `**/*.woff2` を precache するので、絵文字の WOFF2 を `globIgnores` で外す。2 つの形式の約 5.7 MB を、インストールのたびに取らないためである。
  代わりに、実行時に cache-first で持つ。URL にハッシュが付くので、内容が変わらない限り再取得しない。ほかの WOFF2 の precache はそのままにする。
- **使えないとき**: フォントを読めない場合は、OS の絵文字に落ちる。この場合、新しい絵文字が豆腐になり得る。
  Safari 16 以前（`tech()` を知らない）も OS の絵文字に落ちることを想定するが、振る舞いは要検証である（Open Questions）。

### 4. ADR-0091 への影響（§2 の B1 を置き換える）

- ADR-0091 §2「表示（B1）」は、この ADR が置き換える。ピッカー・補完・リアクション・投稿の本文は、同じ `--font-sans` で Noto の形になる。
- ADR-0091 §2 の「OS が描けない版の絵文字を canvas で判定して隠す」はやめる。フォントが `EMOJI_VERSION` のすべてを描くことは、生成のときに確かめる（§1）。
  フォントを読めない環境で豆腐になる絵文字があるのは、§3 の「使えないとき」に書いた例外として受け入れる。
- ピッカーを初めて開くと、すべてのグループの絵文字が描かれる。このため、すべての分割（COLRv1 で約 2 MB、SVG で約 3.7 MB）を読み込む。
  ピッカーの節に `content-visibility: auto` を付けて、画面の外の節の分割を読まないようにできるかを実装で確かめる（要検証）。

### 5. ADR-0092 への影響

ADR-0092 §4 の強調表示の層は、`textarea` と同じフォント・字間・行の高さで文字を並べる。`--font-sans` の先頭の `@font-face` は両方に同じく効くので、決定は変わらない。
ただし、`font-display: swap` の入れ替えの前後で字の幅が変わる。このため、層の位置合わせは、フォントの読み込み（`document.fonts` の `loadingdone`）の後にも測り直す。

### 6. ライセンスと表示

- Noto Color Emoji は SIL Open Font License 1.1 で、Reserved Font Name を定めていない。部分集合を作っても名前を変える義務は無い（2026-10-24 に noto-emoji の [2D/fonts/LICENSE](https://github.com/googlefonts/noto-emoji/blob/e20cbc2bbec1926686be9f9bee7d1d2cfa1fea0e/2D/fonts/LICENSE) と google/fonts の [OFL.txt](https://github.com/google/fonts/blob/e7db6ca603fd4862010383ff17ffd6cc4ffac1c0/ofl/notocoloremoji/OFL.txt) で確認）。
- OFL 1.1 は、各複製に著作権の表示と許諾の文を含めれば、ソフトウェアと一緒に配ることを許す。フォントはプログラムと結合せず、別のファイルとして配るので、AGPL-3.0 のリポジトリと両立する。
- WOFF2 の隣に `OFL.txt`（著作権の表示を含む）を置き、ビルドの成果物にも含める。ADR-0091 の Unicode のライセンスの表示と同じ場所に載せる。

### 見直す条件

- Safari が COLRv1 に対応したら、SVG の版をやめて O1 に寄せる（2026-10-24 時点で caniuse は Safari TP まで未対応）。
- `google/fonts` が SVG の表を持つファイルを配らなくなったら、noto-emoji の SVG の素材から nanoemoji（Apache-2.0）で作るかを見直す。
- 初めてピッカーを開いたときの転送量（SVG で約 3.7 MB）が、モバイルの回線で受け入れられないと分かったら、分割を細かくするか、ピッカーの描き方（§4）を見直す。

## Consequences（影響）

- 良い影響
  - 端末・OS の版に関わらず、絵文字が同じ形になる。古い OS でも `EMOJI_VERSION` の絵文字が豆腐にならない（`tech()` に対応するブラウザーでは）。
  - Storybook と E2E のスクリーンショット（ADR-0027）が、CI のコンテナの OS の絵文字に左右されない。
  - ADR-0091 の canvas の判定と、その実機での確認が要らなくなる。
  - 第三者のオリジンに閲覧を知らせない（O4 との違い）。
- 悪い影響・トレードオフ
  - 絵文字を含む画面で、その分割を読む転送量が増える。顔と感情の分割は COLRv1 で 75 KB、SVG で 153 KB である。ピッカーを初めて開くとすべての分割を読む。
  - `font-display: swap` の入れ替えで、絵文字の形が一度変わる。
  - OS の絵文字に慣れた利用者には、見慣れない形になる。利用者の設定で OS の絵文字に戻す選択肢は持たない。
  - フォントの出所が 2 つ（noto-emoji と google/fonts）あり、版（2.057 と 2.055）が違い得る。細部の形が形式によって違うことがある。
  - リポジトリに約 5.7 MB のバイナリが入り、Unicode の版を上げるたびに増える。
  - 生成に Python の道具（fontTools・brotli・lxml・uharfbuzz）が要る。
  - 通知（Web Push）・`document.title`・OS のネイティブの UI（`select` の一覧など）は OS のフォントで描かれ、Noto にならない。
- 後続で必要になる作業（Issue 化する。ADR-0091 のピッカーのタスク #1147・#1148 より前に入れる）
  1. フォントの生成と組み込み（1 つの PR）:
     - 生成器とタスク: 2 つのフォントの取得と固定、`emoji-test.txt` との突き合わせ、分割と WOFF2、`@font-face` の CSS。ADR-0091 の `emoji:generate` と同じタスクにまとめる。
     - `@my/ui` の `--font-sans` と `--font-mono` の先頭への追加と、`app/ui/README.md` の Typography の節の更新。
     - Service Worker の precache からの除外と、実行時の cache-first。
     - `OFL.txt` の同梱。
     - Storybook と E2E のスクリーンショットの基準画像の更新。
     - `docs/reference/commands.md` と `tools/agentkit/context-map.yaml` の更新。
  2. web-client の §12（ピッカーの表示）と、該当すれば §8（サーバーが描くページも同じ `--font-sans` を使うか）を更新する。
  3. ADR-0091 のタスクの見直し: #1148 から canvas の判定を外す。#1150 の Tauri の WebView での確認を、canvas の判定から Noto の描画（WKWebView で SVG、Android の WebView で COLRv1）と転送量の計測に変える。

## Pros and Cons of the Options（詳細比較・任意）

| | 良い点 | 悪い点 |
| --- | --- | --- |
| O1 COLRv1 だけ | 配る量が最小（1.96 MB）。出所が noto-emoji の 1 つ | Safari・iOS（Tauri を含む）は OS の絵文字のままで、判断基準 1 を満たさない |
| O2 COLRv1 と SVG | すべての対象のブラウザーで Noto の形。Google Fonts と同じ選び分けを同じオリジンで行う | SVG の版が大きい（3.72 MB）。出所が 2 つ。生成が 2 倍 |
| O3 OS のフォント | 転送量なし。作業なし | 端末で形が違い、古い OS で豆腐。決定者の判断に反する |
| O4 Google Fonts | 生成と保守が要らない。分割と形式の選び分けを Google がする | `font-src 'self'` を緩める必要がある（ADR-0019・ADR-0059）。閲覧のたびに Google に要求が届く。版を固定できない |

## Open Questions / 要検証

- `@font-face` の `src` の `tech()` を知らない Safari 16 以前で、`src` 全体が無効になって OS の絵文字に落ちるか、COLRv1 の項目を読んで何も描かないか（要検証。古い Safari の実機か BrowserStack が要る）。
- ピッカーの節に `content-visibility: auto` を付けたとき、画面の外の節の分割の読み込みを遅らせられるか（要検証。実装の PR で計測）。
- `unicode-range` から外すテキストの表示の記号の一覧（`©`・`®`・`™` 以外に何を外すか）。実装の PR で、投稿の本文の見た目のスクリーンショットで決める。
- WKWebView（Tauri の iOS）で OpenType-SVG の絵文字を描く速さと、Android の WebView で COLRv1 を描く速さ（要検証。実機が要る。#1150 で計測）。
