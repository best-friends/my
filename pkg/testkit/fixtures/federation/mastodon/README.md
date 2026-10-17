# Mastodon fixtures

See [pkg/testkit/README.md](../../../README.md#federation-fixtures) for the layout and how to add a fixture.

- `circa-2018/` are Mastodon 2.2-era documents (February to May 2018, with the OStatus context and Linked Data signatures)
  from the test fixtures of Akkoma v3.20.1, © Pleroma Authors and Akkoma Authors, AGPL-3.0.
  Replaced in all of them: `localtesting.pleroma.lol` → `remote.example`, the users `lain` → `bob` and `gargron` → `alice`,
  the media host → `files.mastodon.example.org`, `publicKeyPem`, and `signatureValue`.
- `circa-2023/` is a January 2023 document of mastodon.social from the same Akkoma fixtures;
  `mastodon.social` → `mastodon.example` and the user → `alice`.
- `4.8/` was captured from a running Mastodon 4.8 server; it is the document of
  [peers/mastodon/actor.json](https://github.com/best-friends/my/blob/main/internal/interface/activitypub/testdata/peers/mastodon/actor.json)
  (hosts, names, and texts already replaced there) with `publicKeyPem` replaced.

| File | Source | License |
| --- | --- | --- |
| `4.8/actor.json` | capture of a Mastodon 4.8 actor | capture |
| `circa-2018/follow.json` | [test/fixtures/mastodon-follow-activity.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-follow-activity.json) | AGPL-3.0 |
| `circa-2018/accept-follow.json` | [test/fixtures/mastodon-accept-activity.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-accept-activity.json) | AGPL-3.0 |
| `circa-2018/undo-follow.json` | [test/fixtures/mastodon-unfollow-activity.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-unfollow-activity.json) | AGPL-3.0 |
| `circa-2018/create-note.json` | [test/fixtures/mastodon-post-activity.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-post-activity.json) | AGPL-3.0 |
| `circa-2018/update-person.json` | [test/fixtures/mastodon-update.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-update.json) | AGPL-3.0 |
| `circa-2018/delete-note.json` | [test/fixtures/mastodon-delete.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-delete.json) | AGPL-3.0 |
| `circa-2018/delete-person.json` | [test/fixtures/mastodon-delete-user.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-delete-user.json) | AGPL-3.0 |
| `circa-2018/announce.json` | [test/fixtures/mastodon-announce.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-announce.json) | AGPL-3.0 |
| `circa-2018/like.json` | [test/fixtures/mastodon-like.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-like.json) | AGPL-3.0 |
| `circa-2018/undo-like.json` | [test/fixtures/mastodon-undo-like.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-undo-like.json) | AGPL-3.0 |
| `circa-2018/block.json` | [test/fixtures/mastodon-block-activity.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-block-activity.json) | AGPL-3.0 |
| `circa-2018/undo-block.json` | [test/fixtures/mastodon-unblock-activity.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon-unblock-activity.json) | AGPL-3.0 |
| `circa-2023/note-language.json` | [test/fixtures/mastodon/note_with_language.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/mastodon/note_with_language.json) | AGPL-3.0 |

No public source under a compatible license has a Mastodon `Flag` as JSON; one is added when it is captured.
