// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// federationFixtures holds the received AS2 documents of other implementations, laid out as
// fixtures/federation/<implementation>/<version>/<name>.json (README.md describes the layout,
// the provenance of every file, and how to add one).
//
//go:embed fixtures/federation
var federationFixtures embed.FS

const federationFixtureRoot = "fixtures/federation"

// ExamplePublicKeyPEM is the public key that replaces every publicKeyPem of the federation
// fixtures, so that none of them carries a key of a real server.
const ExamplePublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAuA0aNwC+1ApDv/FNHeNl
2l2VS1mWVFBjlq6lCvXhu+af/+KnZsrkDqOY1s1dqKhsXPzBKQX3rVZbsnr62HYn
FF+6IGDJCtTF4q3cbtKfZ6dhli7FSEfLe0XulA2PJkcjN9xORcwq8YVQdqdWEsj5
M9d0d8d4tBLKfxE0z3ebyCl7SN3a6iDaMjC0TSqsDKpXerBzK7SwAc9sGJiz48+c
KBFcq5L2PGCMV0s31g9VOWDmOVMlsZ6KltKGIhLgwQZmPRl0/NAE8SOhHkt/7rsy
9E1H/XkrU44T7FITu5Vin/bza/W2D8Yij6C5McWe6J7SUwP9DG7tOd0++l1tNb7Z
/wIDAQAB
-----END PUBLIC KEY-----
`

// FederationFixture is one received AS2 document of another implementation.
type FederationFixture struct {
	Implementation string // mastodon, misskey, gotosocial, akkoma, ...
	Version        string // the release that produced the document, or circa-<year> when unknown
	Name           string // file name without .json, such as create-note
	JSON           []byte
}

// Path returns <implementation>/<version>/<name>, the fixture's path in FederationFixturesFS
// without the extension. Golden files use it as their name.
func (f FederationFixture) Path() string {
	return f.Implementation + "/" + f.Version + "/" + f.Name
}

// FederationFixturesFS returns the embedded fixture tree, including the README.md of each
// implementation, rooted at fixtures/federation.
func FederationFixturesFS() fs.FS {
	sub, err := fs.Sub(federationFixtures, federationFixtureRoot)
	if err != nil {
		panic(err) // the directory is embedded, so Sub cannot fail
	}
	return sub
}

// FederationFixtures returns every federation fixture in path order. A JSON file outside the
// <implementation>/<version>/ layout is an error.
func FederationFixtures() ([]FederationFixture, error) {
	var out []FederationFixture
	err := fs.WalkDir(federationFixtures, federationFixtureRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".json" {
			return err
		}
		parts := strings.Split(strings.TrimPrefix(p, federationFixtureRoot+"/"), "/")
		if len(parts) != 3 {
			return fmt.Errorf("testkit: federation fixture %s is not at <implementation>/<version>/<name>.json", p)
		}
		b, err := fs.ReadFile(federationFixtures, p)
		if err != nil {
			return err
		}
		out = append(out, FederationFixture{
			Implementation: parts[0],
			Version:        parts[1],
			Name:           strings.TrimSuffix(parts[2], ".json"),
			JSON:           b,
		})
		return nil
	})
	return out, err
}
