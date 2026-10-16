// SPDX-License-Identifier: AGPL-3.0-only
// The compose feature: the new-post form with its draft, visibility, content warning, language, counter,
// images with descriptions, and replies (FR-101 to FR-105, FR-110). Other features and routes use it
// through this file only (web-client.md A3): the layout places ComposeProvider and ComposeEntry, and a
// reply button calls the function from useReply.
export { ComposeEntry, ComposeProvider } from "./components/Compose";
export { useReply } from "./hooks/useCompose";
