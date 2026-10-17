# Misskey fixtures

See [pkg/testkit/README.md](../../../README.md#federation-fixtures) for the layout and how to add a fixture.

- `circa-2019/` and `circa-2022/` are documents of Misskey servers from the test fixtures of Akkoma v3.20.1,
  © Pleroma Authors and Akkoma Authors, AGPL-3.0. The sending server's host → `misskey.example`, other servers → `remote.example`
  (`example.com` included), and the mentioned user → `bob`.
- `2025.4/` was captured from a running Misskey 2025.4 server; it is the document of
  [peers/misskey/actor.json](https://github.com/best-friends/my/blob/main/internal/interface/activitypub/testdata/peers/misskey/actor.json)
  (hosts, names, and texts already replaced there) with `publicKeyPem` replaced.

| File | Source | License |
| --- | --- | --- |
| `2025.4/actor.json` | capture of a Misskey 2025.4 actor | capture |
| `circa-2019/question.json` | [test/fixtures/tesla_mock/misskey_poll_no_end_date.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/tesla_mock/misskey_poll_no_end_date.json) | AGPL-3.0 |
| `circa-2022/create-note.json` | [test/fixtures/tesla_mock/mk.absturztau.be-93e7nm8wqg-activity.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/tesla_mock/mk.absturztau.be-93e7nm8wqg-activity.json) | AGPL-3.0 |
| `circa-2022/like-reaction.json` | [test/fixtures/custom-emoji-reaction.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/custom-emoji-reaction.json) | AGPL-3.0 |
| `circa-2022/note-quote.json` | [test/fixtures/misskey/quote.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/misskey/quote.json) | AGPL-3.0 |
