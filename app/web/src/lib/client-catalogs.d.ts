// SPDX-License-Identifier: AGPL-3.0-only
// The modules of vite-plugins/client-catalogs.ts.

declare module "virtual:locales" {
  /** A loader of the client catalog per locale that has one. */
  const catalogs: Record<string, () => Promise<{ default: Record<string, { defaultMessage: string }> }>>;
  export default catalogs;
}

declare module "virtual:locales/*/client" {
  /** Every message of the locale's client catalog, in the FormatJS JSON catalog shape. */
  const catalog: Record<string, { defaultMessage: string }>;
  export default catalog;
}
