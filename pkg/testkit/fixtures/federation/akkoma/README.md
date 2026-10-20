# Akkoma fixtures

See [pkg/testkit/README.md](../../../README.md#federation-fixtures) for the layout and how to add a fixture.

`3.20/` comes from Akkoma v3.20.1. The examples of
[docs/docs/development/ap_extensions.md](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/docs/docs/development/ap_extensions.md)
are © 2021-2022 Pleroma Authors and © 2022 Akkoma Authors under CC BY 4.0; they are reformatted and otherwise unchanged
(they already use example hosts). `actor.json` is the user mock of Akkoma's tests, © Pleroma Authors and Akkoma Authors,
AGPL-3.0, with `{{nickname}}` → `akko`, `example.com` → `akkoma.example`, the bio replaced, and `publicKeyPem` replaced.

| File | Source | License |
| --- | --- | --- |
| `3.20/actor.json` | [test/fixtures/users_mock/user.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/users_mock/user.json) | AGPL-3.0 |
| `3.20/emoji-react.json` | the Unicode emoji example of "Emoji reactions" in ap_extensions.md | CC BY 4.0 |
| `3.20/emoji-react-custom.json` | the custom emoji example of "Emoji reactions" in ap_extensions.md | CC BY 4.0 |
| `3.20/emoji-react-remote.json` | the remote custom emoji example of "Emoji reactions" in ap_extensions.md | CC BY 4.0 |
| `3.20/undo-emoji-react.json` | the `Undo` example of "Emoji reactions" in ap_extensions.md | CC BY 4.0 |
| `3.20/note-quote.json` | the example of "Quote Posts" in ap_extensions.md | CC BY 4.0 |
| `3.20/actor-emoji.json` | `3.20/actor.json` with `:blobcat:` in `name` and `summary` and the `tag` that `take_emoji_tags` of [transmogrifier.ex](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/lib/pleroma/web/activity_pub/transmogrifier.ex) renders with `emoji_object!` of [builder.ex](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/lib/pleroma/web/activity_pub/builder.ex) (the `tag` of [user_view.ex](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/lib/pleroma/web/activity_pub/views/user_view.ex)) | AGPL-3.0 |
| `circa-2021/note-emoji-summary.json` | [test/fixtures/tesla_mock/emoji-in-summary.json](https://akkoma.dev/AkkomaGang/akkoma/src/commit/98dd958886a8b0ef65d5bd827aba3789f1dc5116/test/fixtures/tesla_mock/emoji-in-summary.json), a Pleroma note with custom emoji in `content` and `summary`; `patch.cx` → `akkoma.example`, the user → `alice` | AGPL-3.0 |
