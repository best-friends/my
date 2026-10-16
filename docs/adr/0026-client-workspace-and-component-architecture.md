# ADR-0026: クライアントのワークスペース構成と、ロジックと UI の責務分離

- Status: Accepted
- Date: 2026-10-16
- Deciders: @rosylilly
- Issue: #86

## Context

Web クライアントの UI 部品は、後で Tauri 2 のモバイルアプリと共有する（ADR-0008）。
部品がアプリのドメイン（API・ルーティング・サーバー状態）に依存すると共有できない。依存を後から剥がすのは高くつくので、最初から境界を構造で強制する。
ADR-0010 は `app/` にクライアントアプリを 1 アプリ 1 ディレクトリで置くと定めているが、アプリ間で共有するパッケージの置き場所は定めていない。

## Decision Drivers

1. UI 部品がアプリのドメインを知らないことを、レビューではなく構造と lint で保証する
2. 機能ごとにロジック・API・画面を閉じ込め、他の機能からは公開面だけを使わせる
3. 部品単位でアクセシビリティ・レスポンシブ・見た目のテストができる（ADR-0027）

## Considered Options

1. **UI キットを最初から別のワークスペースパッケージ（`app/ui`）にする**
2. まず `app/web/src/ui` に置き、必要になったら切り出す
3. 層を分けずに機能ごとに部品を作る

## Decision

**1 を採用する。** `app/` を Bun ワークスペースのルートとし、アプリと、アプリ間で共有するパッケージを置く。

```text
app/
  ui/              # @my/ui: 共有 UI キット（React Aria Components を包む部品、デザイントークン、Storybook）
  web/             # Web クライアント（PWA）
    src/
      routes/      # TanStack Router のファイルルート（薄い。パラメータ → 機能の部品）
      features/<x>/
        api/       # queryOptions・mutation（サーバー状態はここだけ）
        hooks/     # 機能のロジック（UI を持たないカスタムフック）
        components/# @my/ui を組み合わせた機能の部品
        index.ts   # 公開面。他の機能はここからだけ import する
      lib/         # API クライアント、QueryClient、SSE、i18n の初期化、ランタイム設定
  mobile/          # Tauri 2（将来）
```

### 依存の向き

- `routes → features → (@my/ui, lib)`、`features → lib`。逆向きは禁止。
- **`@my/ui` は React・React Aria Components・Tailwind のユーティリティにだけ依存する。** 次のものは import できない。
  - API・Query・Router・`features`
  - i18n のカタログ（文言は props / children で受け取る）
- 機能は、他の機能を `index.ts` 経由でだけ使う。
- これらは `eslint-plugin-boundaries` で検査する。`@my/ui` から `app/web` を import すると、パッケージの依存関係上も解決できない。

### ロジックと UI の分離

- **`@my/ui` の部品は表示に専念する。**
  - 状態はできるだけ外から渡す（`value` / `onChange`）。
  - 見た目の違いは型付きの props（`variant`・`size`・`tone`）で表し、`tailwind-variants` の 1 部品 1 レシピで定義する。
  - `className` は root（またはスロット）にだけ渡し、`tailwind-merge` で合成する。
- **振る舞い（キーボード・フォーカス・ARIA）は React Aria に任せる。** 独自の振る舞いが必要な部品だけ `react-aria` のフックで組み立てる。
- **機能のロジックは `features/<x>/hooks` のカスタムフックに置く。** フックは JSX を返さない。機能の部品はフックの戻り値を `@my/ui` に渡すだけにする。
- **サーバーから取得するデータは `features/<x>/api` の `queryOptions` / mutation だけで扱う。** `useState` や Context にコピーしない。

## Consequences

- UI キットは最初から Tauri と共有できる形になる。Storybook とアクセシビリティ・見た目のテストは `app/ui` に置く。
- ワークスペースの管理（パッケージ間の型参照、Tailwind の `@source` 設定）が増える。
- ADR-0010 の「`app/` は 1 アプリ 1 ディレクトリ」に、共有パッケージ（`app/ui`）を加える。
