// SPDX-License-Identifier: AGPL-3.0-only

/**
 * The app's path of a document's page from the address that GET /instance gives (`terms_url` and the like, absolute
 * URLs of the web client): the app opens it through its router, in the same tab, whatever scheme it is served at.
 */
export function documentPath(url: string): string {
  try {
    const { pathname, search, hash } = new URL(url);
    return pathname + search + hash;
  } catch {
    return url;
  }
}
