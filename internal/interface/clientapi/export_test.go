// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"maps"
	"net/http"
	"slices"
)

// RequestErrorHandler and ResponseErrorHandler expose the error handlers that Register gives
// the generated code, so that tests reach them without an operation that fails.
func RequestErrorHandler(s *Server) func(http.ResponseWriter, *http.Request, error) {
	return s.requestError
}

func ResponseErrorHandler(s *Server) func(http.ResponseWriter, *http.Request, error) {
	return s.responseError
}

// GuardedPatterns returns the sorted patterns of security.gen.go.
func GuardedPatterns() []string { return slices.Sorted(maps.Keys(operationSecurity)) }

// Guard exposes Server.guard.
func Guard(s *Server, pattern string, h http.Handler) http.Handler { return s.guard(pattern, h) }
