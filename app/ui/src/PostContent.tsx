// SPDX-License-Identifier: AGPL-3.0-only
// The one audited component that renders HTML (web-client.md A8). ESLint allows dangerouslySetInnerHTML
// in this file only (app/eslint.config.js). The server sanitizes every post body to its allowlist
// before it reaches the client; this component trusts that and adds nothing of its own. The app's
// Content-Security-Policy requires Trusted Types (`require-trusted-types-for 'script'`, ADR-0019 §9), under
// which the browser refuses a plain string as innerHTML, so the HTML goes in as a TrustedHTML made by the
// one policy below, the only place that turns a string into TrustedHTML. The shell's CSP allows this
// policy's name once (`trusted-types my-server-sanitized-html …`), so no other script can make a policy
// of its own, and this module makes it once.
//
// The policy passes the HTML through unchanged. That is safe only because every HTML it receives comes
// out of the server's sanitizer (the allowlist in the Go server); HTML from anywhere else, such as the
// remote posts that federation brings (M3), must go through the same sanitizer on the server first.
import type { ComponentPropsWithRef } from "react";
import { tv } from "tailwind-variants";

declare const sanitizedHtmlBrand: unique symbol;

/**
 * HTML that the server has already sanitized, such as the rendered body of a post in an API response.
 * A plain `string` is not assignable to it, so HTML from anywhere else (user input, a URL, a template)
 * cannot reach `PostContent` by accident: it has to pass through `serverSanitizedHtml`.
 */
export type SanitizedHtml = string & { readonly [sanitizedHtmlBrand]: true };

/**
 * Marks a string as server-sanitized HTML. Call it only on a field the server documents as sanitized
 * HTML, at the place the API response is read; never on text the client built or received from elsewhere.
 */
export function serverSanitizedHtml(html: string): SanitizedHtml {
  return html as SanitizedHtml;
}

/** The part of the Trusted Types API (https://developer.mozilla.org/docs/Web/API/Trusted_Types_API) used here. */
type TrustedTypePolicyFactory = {
  createPolicy: (name: string, rules: { createHTML: (html: string) => string }) => TrustedTypePolicy;
};
type TrustedTypePolicy = { createHTML: (html: string) => unknown };

/** The policy of each Trusted Types factory, made on first use. */
const policies = new WeakMap<TrustedTypePolicyFactory, TrustedTypePolicy>();

/**
 * The HTML as a TrustedHTML where the browser has Trusted Types, and as it is elsewhere. The policy
 * passes the HTML through unchanged: only `SanitizedHtml`, which the server sanitized, reaches it.
 */
function trustedHtml(html: SanitizedHtml): string {
  const factory = (globalThis as { trustedTypes?: TrustedTypePolicyFactory }).trustedTypes;
  if (factory === undefined) return html;
  let policy = policies.get(factory);
  if (policy === undefined) {
    policy = factory.createPolicy("my-server-sanitized-html", { createHTML: (input) => input });
    policies.set(factory, policy);
  }
  // React assigns the value of __html to innerHTML as it is, and innerHTML accepts the TrustedHTML in
  // place of the string its DOM type names.
  return policy.createHTML(html) as string;
}

const postContent = tv({
  // Long URLs and words wrap anywhere, so no post scrolls sideways at 320 px (web-client.md D3).
  base: [
    "text-base wrap-anywhere text-fg",
    "[&_a]:text-accent [&_a]:underline [&_a:hover]:text-accent-hover",
    "[&_blockquote]:border-s-4 [&_blockquote]:border-border [&_blockquote]:ps-3 [&_blockquote]:text-fg-muted",
    "[&_code]:font-mono [&_pre]:overflow-x-auto [&_pre]:font-mono",
    "[&_ol]:list-decimal [&_ol]:ps-6 [&_ul]:list-disc [&_ul]:ps-6",
    "[&>*+*]:mt-2",
  ],
});

export type PostContentProps = Omit<ComponentPropsWithRef<"div">, "children" | "dangerouslySetInnerHTML" | "dir"> & {
  /** The post body as the server rendered and sanitized it. */
  html: SanitizedHtml;
  /** The language of the post, when the server knows it (a BCP 47 tag). */
  lang?: string | undefined;
};

/**
 * The body of a post: server-sanitized HTML in a block whose direction follows its own text (`dir="auto"`)
 * and whose `lang` is the post's language when known (web-client.md Y10). Give it an `id` to describe a
 * PostCard by it.
 */
export function PostContent({ html, lang, className, ...props }: PostContentProps) {
  return <div {...props} dir="auto" lang={lang} className={postContent({ className })} dangerouslySetInnerHTML={{ __html: trustedHtml(html) }} />;
}
