# Claude Code への依頼文(例)

このフォルダを `app/ui/design/` に置いたうえで、Claude Code に次のように頼んでください。

```text
app/ui/design/HANDOFF.md と app/ui/design/README.md を読んで、手順 1 から順に @my/ui に反映してください。

- 見た目の正解は app/ui/design/components/<名前>/preview.html と components/bundle.css です。
  コンポーネントの構造・props・アクセシビリティ・挙動は変えず、theme.css のトークンと tv() のクラスだけを変えてください。
- 手順ごとにコミットを分け、各手順で make web-check(トークンのコントラスト検査を含む)と Storybook のテストが通ることを確認してください。
- HANDOFF.md の「判断が必要な点」に当たるものは実装せず、選択肢をまとめて私に確認してください。
```

手順 1(トークン)だけ先に頼み、見た目を確認してから手順 3(コンポーネント)を頼む、という分け方もおすすめです。
