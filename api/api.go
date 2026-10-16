// SPDX-License-Identifier: AGPL-3.0-only

// Package api embeds the OpenAPI document of the client API (openapi.yaml), the source of truth
// from which the server interface and the client types are generated (ADR-0016, ADR-0029).
package api

import _ "embed"

// Spec is the OpenAPI 3.1 document of the client API, in YAML.
//
//go:embed openapi.yaml
var Spec []byte
