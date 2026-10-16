// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import "net/http"

// RequestErrorHandler and ResponseErrorHandler expose the error handlers that Register gives
// the generated code, so that tests reach them without an operation that fails.
func RequestErrorHandler(s *Server) func(http.ResponseWriter, *http.Request, error) {
	return s.requestError
}

func ResponseErrorHandler(s *Server) func(http.ResponseWriter, *http.Request, error) {
	return s.responseError
}
