# testkit

Port contract test suites, deterministic fakes (`Clock`, `IDGenerator`), and fixtures for adapter authors and for this
repository's own tests ([ADR-0015](../../docs/adr/0015-testing-strategy.md)).

## Federation fixtures

`fixtures/federation/` holds AS2 documents (activities, objects, actors) that other implementations send, for the
regression tests of [federation.md §12](../../docs/architecture/federation.md#12-テスト) ([ADR-0006](../../docs/adr/0006-federation-protocol-scope.md)).
`testkit.FederationFixtures()` returns them and `testkit.FederationFixturesFS()` exposes the tree.

### Layout

```text
fixtures/federation/<implementation>/<version>/<name>.json
fixtures/federation/<implementation>/README.md      provenance of every file of that implementation
testdata/golden/normalize/<implementation>/<version>/<name>.json   federation.Normalize result
```

- `<implementation>` is the lower-case software name: `mastodon`, `misskey`, `gotosocial`, `akkoma`.
- `<version>` is the release that produced the document (`4.8`, `0.22`, `2025.4`). When the source does not record it,
  it is `circa-<year>` with the year of the document's timestamps.
- `<name>` is the activity type and, where it helps, the object type or variant, in kebab case:
  `follow`, `accept-follow`, `create-note`, `update-person`, `delete-note`, `undo-like`, `like-reaction`, `actor`.

### Sources

A fixture comes from a public source whose license allows copying it into this AGPL-3.0-only repository:

- the implementation's own test data or test expectations (Mastodon, Misskey, GoToSocial, and Akkoma are AGPL-3.0);
- the implementation's documentation, when its license is compatible (Akkoma's `docs/` is CC BY 4.0, which requires
  attribution; Mastodon's documentation is GFDL and must not be copied);
- a document captured from a running server, recorded with the server's version and the capture date.
- the document that the implementation's own rendering code produces, rendered from its source at a pinned release or
  commit, when no test data has it; the README names the functions and links the source.

Never write a fixture by hand from memory: a fixture shows what an implementation really sends.

### Adding a fixture

1. Take the document from the source unchanged.
2. Replace the example values:
   - every host of a real server with an example host (RFC 2606): the sender becomes `<implementation>.example`
     (or keeps a host that is already an example host, such as `mastodon.example.org`), other servers become `remote.example`
     and `other.example`. Replace the host everywhere it appears: URLs, `acct:` handles, `@user@host` mentions, `tag:` URIs.
   - the user names, display names, bios, and post texts of real people with neutral ones (`alice`, `bob`, `Alice`),
     keeping the markup around them. Names that the source's test data made up (`the_mighty_zork`, `akko`) may stay.
   - every `publicKeyPem` with `testkit.ExamplePublicKeyPEM` and every Linked Data `signatureValue` with
     `RVhBTVBMRS1TSUdOQVRVUkU=`.
   - nothing else: keep property names, ids, value types, nulls, and dialect extensions as they are, since they are what
     the test is about.
3. Indent the JSON with two spaces and save it at `fixtures/federation/<implementation>/<version>/<name>.json`.
4. Add a row to `fixtures/federation/<implementation>/README.md`: the file, the source (a commit permalink, or the
   server version and capture date), the license, and what was replaced.
5. Run `go test ./pkg/testkit -run TestFederationFixtures -update`, read the new golden file under
   `testdata/golden/normalize/`, and commit both.

`TestFederationFixturesAnonymized` fails on a URL whose host is neither an example host nor a JSON-LD namespace host,
and on a key other than the example key; `TestFederationFixturesProvenance` fails on a fixture without a README row.

### Golden files

`TestFederationFixturesNormalize` normalizes every fixture with `federation.Normalize` and compares the result with its
golden file; a document that cannot be normalized records the error. When a change to the normalization changes a
result on purpose, run the test with `-update` and review the golden diff in the PR. `testkit.CompareGolden` is the
same comparison for other golden tests.
