// SPDX-License-Identifier: AGPL-3.0-only
// Multipart uploads in tests. A request whose body is jsdom's FormData with a jsdom File fails in Node.js's
// fetch under MSW (Vitest converts the body, and the length no longer matches the content-length). A browser
// has one FormData, so a test that uploads uses Node.js's FormData and File instead, and the request then
// reaches the MSW handlers as it would reach the server.
/// <reference types="node" />
import { File as NodeFile } from "node:buffer";
import { vi } from "vitest";

/** Node.js's FormData constructor, which jsdom's global replaces in the tests; Node.js's fetch still makes them. */
const NodeFormData = (await new Response(new URLSearchParams()).formData()).constructor as typeof FormData;

/** Node.js's File constructor, typed as the DOM's, as the app sees its files. */
const NodeFileConstructor = NodeFile as unknown as typeof File;

/**
 * Makes the app build its multipart bodies with Node.js's FormData for the rest of the test (the stub is
 * undone after it, `unstubGlobals` in vitest.config.ts), and returns a Node.js File to upload with user-event.
 */
export function nodeUploadFile(content: string, name: string, type: string): File {
  vi.stubGlobal("FormData", NodeFormData);
  return new NodeFileConstructor([content], name, { type });
}
