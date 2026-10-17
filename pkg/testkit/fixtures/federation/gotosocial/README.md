# GoToSocial fixtures

See [pkg/testkit/README.md](../../../README.md#federation-fixtures) for the layout and how to add a fixture.

`0.22/` comes from GoToSocial v0.22.2: the JSON that its own tests expect it to serialize, and the examples of its
federation documentation, © GoToSocial Authors, AGPL-3.0. Replaced in all of them: `localhost:8080` → `gotosocial.example`,
`fossbros-anonymous.io` → `remote.example`, `unknown-instance.com` and `somewhere.com` → `other.example`, and `publicKeyPem`.
The id of `update-person.json`, which the test generates, is an example value.

| File | Source | License |
| --- | --- | --- |
| `0.22/actor.json` | `TestAccountToAS` in [internal/typeutils/internaltoas_test.go](https://codeberg.org/superseriousbusiness/gotosocial/src/commit/fb2bf1330c355f19c6f3614d7434c56f8d55aeaf/internal/typeutils/internaltoas_test.go) | AGPL-3.0 |
| `0.22/create-note.json` | `TestWrapNoteInCreate` in [internal/typeutils/wrap_test.go](https://codeberg.org/superseriousbusiness/gotosocial/src/commit/fb2bf1330c355f19c6f3614d7434c56f8d55aeaf/internal/typeutils/wrap_test.go) | AGPL-3.0 |
| `0.22/update-person.json` | `TestWrapAccountableInUpdate` in [internal/typeutils/wrap_test.go](https://codeberg.org/superseriousbusiness/gotosocial/src/commit/fb2bf1330c355f19c6f3614d7434c56f8d55aeaf/internal/typeutils/wrap_test.go) | AGPL-3.0 |
| `0.22/announce.json` | `TestSelfBoostFollowersOnlyToAS` in [internal/typeutils/internaltoas_test.go](https://codeberg.org/superseriousbusiness/gotosocial/src/commit/fb2bf1330c355f19c6f3614d7434c56f8d55aeaf/internal/typeutils/internaltoas_test.go) | AGPL-3.0 |
| `0.22/flag.json` | `TestReportToAS` in [internal/typeutils/internaltoas_test.go](https://codeberg.org/superseriousbusiness/gotosocial/src/commit/fb2bf1330c355f19c6f3614d7434c56f8d55aeaf/internal/typeutils/internaltoas_test.go) | AGPL-3.0 |
| `0.22/accept-reply-request.json` | `TestAcceptRemoteReplyRequest` in [internal/federation/federatingdb/accept_test.go](https://codeberg.org/superseriousbusiness/gotosocial/src/commit/fb2bf1330c355f19c6f3614d7434c56f8d55aeaf/internal/federation/federatingdb/accept_test.go) | AGPL-3.0 |
| `0.22/delete-note.json` | the first `Delete` example of [docs/federation/posts.md](https://codeberg.org/superseriousbusiness/gotosocial/src/commit/fb2bf1330c355f19c6f3614d7434c56f8d55aeaf/docs/federation/posts.md) | AGPL-3.0 |
