// SPDX-License-Identifier: AGPL-3.0-only

// Package web serves the web client (ADR-0028): the HTML shell that starts the SPA, the
// Vite build output, and the Web App Manifest generated from the instance settings.
//
// The build of app/web is embedded from dist/, which `make web-build` fills with a copy of
// app/web/dist (the Dockerfile mounts the build there instead). Only dist/.gitkeep is
// committed, so the package compiles and its tests run without Bun; a binary built that way
// serves a shell without scripts and logs a warning at start.
//
// The shell is the same for every SPA route and every visitor: requests for HTML are always
// treated as anonymous, so the shell is rendered once and cached by shared caches. The
// instance's runtime settings reach the client as a JSON data block in the shell
// (<script type="application/json" id="config">), not through the build, so the
// Content-Security-Policy needs no nonce and allows only scripts from the server itself.
//
// Routing between the shell and ActivityPub documents (content negotiation) is the caller's:
// this package serves only HTML.
package web
