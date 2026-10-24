// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"net/http"

	"github.com/mycroblog/my/api"
)

// openAPIPattern is the http.ServeMux pattern of the bundled OpenAPI document.
const openAPIPattern = "GET " + BasePath + "/openapi.yaml"

// registerOpenAPI serves the bundled OpenAPI document of the client API at
// GET /api/v1/openapi.yaml, so that third-party apps read the contract from the server they
// talk to (ADR-0090 section 8). It is public and the same for everyone, so a cache may keep it.
func (s *Server) registerOpenAPI(r Router) {
	r.Handle(openAPIPattern, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "application/yaml; charset=utf-8")
		h.Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(api.Spec)
	}))
}
