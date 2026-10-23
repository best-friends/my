# リリースの手順

リリースを作る人（決定者）向けの手順です。リリースは `release.yml` だけが作ります（[ADR-0079](../adr/0079-ci-single-gate-merge-queue.md)）。
`release.yml` は、指定したコミットで遅い検査（`slow-suite.yml`）を回し、すべて通ったときだけ、そのコミットにタグを打って GitHub Release を下書きで作ります。
遅い検査の中身と、ほかにどこで回るかは [workflow.md の「What CI runs where」](../agents/workflow.md#what-ci-runs-where) にあります。

## 版

- 版は `vX.Y.Z` か、プレリリースの `vX.Y.Z-<識別子>`（例: `v0.1.0-rc.1`）です。形が違う版は最初のジョブで落ちます。
- `main` 以外のブランチからは、プレリリースの版だけを作れます。関門をブランチで試すためのもので、正式なリリースは `main` から作ります。
- すでにタグがある版は最初のジョブで落ちます。遅い検査の後にも、タグと同じ名前のリリースがないことを確かめ直します。

## 手順

1. 最新の Nightly が通っていることを確かめる。落ちているなら、先に直す（[workflow.md §9](../agents/workflow.md#failures-on-main-orchestrator)）。

   ```sh
   gh run list -R mycroblog/my --workflow nightly.yml --limit 3
   ```

2. `main` で `release.yml` を回す。回したときの `main` の先頭のコミットが、検査されタグを打たれるコミットになる。

   ```sh
   gh workflow run release.yml -R mycroblog/my --ref main -f version=v0.1.0
   gh run list -R mycroblog/my --workflow release.yml --limit 1
   ```

3. 終わるのを待つ。遅い検査（E2E の全体・連合・実 PostgreSQL・stream の契約・イメージ・CodeQL）は、各ジョブが並行して動いても時間がかかる。
   最後のジョブ `release` は、遅い検査のジョブが 1 つでも落ちるか取り消されると動かず、タグも Release もできない。
   落ちたときは原因を直して `main` に入れ、同じ版でもう一度回す。
4. 通ったら、下書きの Release（`gh release view v0.1.0 -R mycroblog/my`、または Releases の画面）を見て、自動で作られた変更の一覧を整え、公開する。
   プレリリースの版は、プレリリースとして下書きされる。

## 関門をブランチで試す

`release.yml` の変更は、プレリリースの版を使って、正式な版のタグを打たずにブランチで試せます。

```sh
gh workflow run release.yml -R mycroblog/my --ref <branch> -f version=v0.0.0-test.1
```

- 遅い検査が落ちるブランチで回すと、`release` のジョブが skip され、タグができないことを確かめられる（`gh api repos/mycroblog/my/git/ref/tags/v0.0.0-test.1` が 404）。
- 通るブランチで回すと、そのブランチの先頭にタグと下書きの Release ができる。確かめたら両方を消す。

  ```sh
  gh release delete v0.0.0-test.1 -R mycroblog/my --cleanup-tag --yes
  ```

## 成果物

今の `release.yml` が作るのは、タグと下書きの Release だけです。コンテナイメージの公開とリリースノートは、M5（[#30](https://github.com/mycroblog/my/issues/30)）が `release.yml` の `release` のジョブの後に足します。
