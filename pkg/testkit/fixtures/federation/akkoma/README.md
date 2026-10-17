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
