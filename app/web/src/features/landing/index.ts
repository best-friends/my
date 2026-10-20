// SPDX-License-Identifier: AGPL-3.0-only
// The landing feature: the page that people who are not signed in see at the root address (`/`), with what the
// server is, the ways to sign in and to sign up, its rules, a sample of its local timeline, and the links to its
// documents. Routes and other features use it through this file only (web-client.md A3).
export { instanceQueryOptions } from "./api/landing";
export { Landing } from "./components/Landing";
export { useSignUpHref } from "./hooks/useLanding";
