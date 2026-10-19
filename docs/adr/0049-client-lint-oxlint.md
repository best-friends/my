# ADR-0049: クライアントの lint を Oxlint にし、型情報が要る規則だけ ESLint に残す

- Status: Proposed
- Date: 2026-10-19
- Deciders: @rosylilly
- Issue: #572

## Context（背景・課題）

クライアントのワークスペース（`app/`）の lint は、[ADR-0025](0025-web-client-stack.md) と [ADR-0027](0027-ui-quality-gates.md) が定める ESLint（flat config）と
そのプラグイン（typescript-eslint、`eslint-plugin-react-hooks`、`@eslint-react`、TanStack Query / Router、Testing Library、boundaries、better-tailwindcss、formatjs、eslint-comments）
で行い、[Web クライアントの設計規約](../architecture/web-client.md) の「強制」の列の多くを守っている。独自の禁止規則は `app/eslint/restrictions.js` にある。

この ESLint は 1 回に約 50 秒かかり（手元 52.8 秒、CI 50.4 秒）、`make web-lint` と CI の web ジョブで最も長い手順になっている。
時間の大半は typescript-eslint の型情報（TypeScript 6 のコンパイラ）で、ESLint 10 の `--concurrency` は 4 CPU の CI では 1 割ほどしか縮めない。
決定者は ESLint が遅いことを理由に、より速い linter への乗り換えを求めている（#572）。

調査（[#572 の報告](https://github.com/best-friends/my/issues/572#issuecomment-6265430885)）で、Biome、Oxlint、Oxlint と最小の ESLint の併用を、
今の 278 規則と独自の禁止規則のすべてについて比べ、同じ機械で時間を測った。

## Decision Drivers（判断基準）

1. web-client.md の「強制」の列の行を、lint から外さない
2. lint の時間を大きく縮める（CI の 4 vCPU で効く）
3. 公式が示す方法で設定し、独自の仕組みを増やさない
4. 公開から 2 週間以上たった版だけを使い、ライセンスが AGPL-3.0 と両立する
5. TypeScript 7（[ADR-0048](0048-typescript-7-with-typescript-6-api.md)）の妨げにならない

## Considered Options（選択肢）

1. ESLint のまま、`--concurrency auto` で並列にする
2. Oxlint 単独（ネイティブの規則、tsgolint による型情報の規則、ESLint のプラグインを JS プラグインとして動かす）
3. Oxlint と、Oxlint の JS プラグインでは動かない型情報が要る規則だけの ESLint を、並列に動かす
4. Biome（ネイティブの規則と GritQL のプラグイン）
5. Biome と、Biome にない規則の ESLint

調査で分かったこと（2026-10-19、各公式の文書と、spike ブランチでの計測）:

| 選択肢 | 時間（手元 24 コア / 4 CPU、warm の中央値） | 「強制」の列から外れる行 |
| --- | --- | --- |
| 1 | 31.0 s / 45.0 s | なし |
| 2 | 8.8 s / 8.6 s | R14 |
| 3 | 8.8 s / 9.6 s | なし |
| 4 | 1.4 s / 2.8 s | A1〜A3、A9、D8、D9、S1 と S7 の TanStack の規則、T2、Y2、R9、R14。D2・D5〜D7 は一部だけ |
| 5 | 1.4 s ＋ 49.7 s | なし |

- Oxlint の JS プラグインは alpha で、ESLint v9 以降のプラグイン API を対象にし、型情報を使う規則は動かない（[JS Plugins](https://oxc.rs/docs/guide/usage/linter/js-plugins.html)）。
  今の規則のうち動かないのは `@eslint-react/no-leaked-conditional-rendering`（R14）と `@eslint-react/no-unused-props` の 2 つだけだった。
- Oxlint の型情報の規則は tsgolint が typescript-go で動かし、typescript-eslint の型情報の規則 61 のうち 59 に対応し、TypeScript 7.0 以上を前提にする
  （[Type-Aware Linting](https://oxc.rs/docs/guide/usage/linter/type-aware.html)）。今の規則のうち対応しないのは `@typescript-eslint/no-unsafe-enum-assignment` だけだった。
- Biome のプラグインは GritQL で、一致したところを報告するだけで、ESLint のプラグインは動かない（[Linter Plugins](https://biomejs.dev/linter/plugins/)）。
  型を使う規則は TypeScript を使わない独自の推論で、今のコードに 74 件の偽陽性を出した。
- 選択肢 5 は、ESLint 側に typescript-eslint の型情報の規則が残るので速くならない。

## Decision（決定）

> Proposed の間は「推奨案」として書く。Accepted にするのは人間の Decider のみ。

**選択肢 3 を推奨する。**

- lint は Oxlint（`oxlint`、`oxlint-tsgolint`）で行う。設定は `app/.oxlintrc.json` にし、`@oxlint/migrate --type-aware` で今の `app/eslint.config.js` から作る。
  - typescript-eslint の型情報の規則は tsgolint で動かす（`options.typeAware`）。
  - `eslint-plugin-react-hooks` の規則は Oxlint のネイティブの `react/*` で動かす。
  - `@eslint-react`、TanStack Query / Router、Testing Library、boundaries、better-tailwindcss、formatjs、eslint-comments は JS プラグイン（`jsPlugins`）で動かす。
  - `no-restricted-syntax` は Oxlint にないので、ESLint の同じ規則を読み込む 1 ファイルの JS プラグインで動かし、`restrictions.js` のセレクタをそのまま渡す。
- ESLint は、JS プラグインでは型情報が得られない規則（`@eslint-react/no-leaked-conditional-rendering` と `@eslint-react/no-unused-props`）だけを動かす。
- `make web-lint` は Oxlint と ESLint を並列に動かし、どちらかが失敗したら失敗する。`restrictions.test.js`・knip・CSS の `100vh` の検査は今のまま。
- tsgolint はファイルごとに一番近い `tsconfig.json` を使うので、`app/ui` と `app/web` の `tsconfig.json` を `references` だけを持つ solution 形式にし、
  今の内容を `tsconfig.app.json` に移す（`tsc -p` に渡す名前も変える）。
- 無効化コメントは `eslint-disable` の形のまま使う。R9 の `no-restricted-disable` には、`react-hooks/*` に加えて Oxlint の `react/` の名前（`react/exhaustive-deps` など）を挙げる。

理由:

- 選択肢 3 は、「強制」の列の行をすべて保ったまま（判断基準 1）、時間を約 6 分の 1 にする（判断基準 2）。消えるのは列にない
  `@typescript-eslint/no-unsafe-enum-assignment` と、設定を持たない `react-hooks/config`・`react-hooks/gating` だけ。
- 選択肢 2 は同じ速さだが R14 を失う。選択肢 1 は CI では速くならない。選択肢 4 は最速だが、境界・i18n・Tailwind・Testing Library・TanStack の検査を失い、
  GritQL で書き直せるのは `restrictions.js` の構文の禁止だけ。
- 設定は Oxlint の公式の移行ツールと、ESLint の規則をそのまま使う（判断基準 3）。追加する依存は MIT で、公開から 2 週間以上たった版（判断基準 4）。

次のときはこの推奨を見直す:

- Oxlint の JS プラグインが型情報を受け取れるようになったら、ESLint をやめて選択肢 2 にする。
- 決定者が R14 を lint で検査しなくてよいとするなら、選択肢 2 にする。
- Biome がプラグインで境界・Tailwind・Testing Library に当たる検査を書けるようになったら、選択肢 4 を比べ直す。

## Consequences（影響）

- `make web-lint` の lint の手順が約 50 秒から約 9 秒になる。
- [ADR-0025](0025-web-client-stack.md) の技術スタックの表の「lint」の行と、[ADR-0027](0027-ui-quality-gates.md) の検査の層の表の「lint」の行は、受け入れたらこの ADR が置き換える
  （ESLint を Oxlint ＋ 型情報の規則だけの ESLint にする。プラグインと規則は同じ）。両 ADR の Status に、置き換える部分を書く。
- web-client.md の「強制」の列の規則名は、Oxlint で動く名前（`react-hooks/purity` は `react/purity` など）に合わせて書き直す。
- 「強制」の列の大半（境界・i18n・Tailwind・Testing Library・TanStack）が、alpha の Oxlint の JS プラグインで動く。Oxlint の版を上げるたびに、
  違反のフィクスチャで各行が検出されることを確かめる必要がある。
- React Compiler の規則は、`eslint-plugin-react-hooks` ではなく Oxlint の移植で動く。移植は `"use no memo"` の関数の中でも `react/incompatible-library` を報告するなど、
  細部が公式のプラグインと違う。
- 依存に ESLint と typescript-eslint が残るので、TypeScript 6 の API の別名（ADR-0048）も、その 2 規則のために残る。
- 後続の作業（Issue にする）: 移行（設定、`make web-lint`、tsconfig の solution 形式、無効化コメントの規則名、web-client.md の規則名、ADR-0025 / ADR-0027 の Status）。

## Open Questions / 要検証

- 選択肢 3（R14 を ESLint で保つ）と選択肢 2（ESLint をなくし、R14 を lint で検査しない）のどちらにするか
- 「強制」の列の大半を alpha の Oxlint の JS プラグインに載せてよいか
- knip が `.oxlintrc.json` の JS プラグインを依存として認識するか（要検証）
