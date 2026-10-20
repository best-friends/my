// SPDX-License-Identifier: AGPL-3.0-only
// The one audited component that renders HTML (web-client.md A8). Lint allows dangerouslySetInnerHTML
// in this file only (app/.oxlintrc.json). The server sanitizes every post body to its allowlist
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
import type { ComponentPropsWithRef, RefCallback } from "react";
import { mergeRefs } from "react-aria";
import { tv } from "tailwind-variants";
import { type CustomEmojiImage, insertCustomEmojis, noCustomEmojis } from "./CustomEmoji";

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
  /**
   * Called when a link in the body is followed in the same tab: a primary click, or Enter on the
   * focused link, without a modifier key, on a link that has no `target` or `download`. Calling
   * `event.preventDefault()` keeps the browser from following it, so the app can open the link itself.
   * Opening a link in a new tab or window stays the browser's.
   */
  onLinkClick?: ((event: MouseEvent, link: HTMLAnchorElement) => void) | undefined;
  /**
   * The custom emoji the body uses (the API's `emojis`): each `:shortcode:` of one of them in the body's
   * text is drawn as its image, made with the DOM API rather than HTML (ADR-0019 §2, ADR-0051 §4).
   */
  emojis?: readonly CustomEmojiImage[] | undefined;
  /** Whether animated emoji may move when the user allows motion; false shows their still images. */
  emojiMotion?: boolean | undefined;
};

/**
 * Listens for the clicks on the links of the body. The links are in HTML that React does not render,
 * so the listener goes on the element itself; React removes it with the element or for a new handler.
 */
function linkClickListener(onLinkClick: PostContentProps["onLinkClick"]): RefCallback<HTMLDivElement> | undefined {
  if (onLinkClick === undefined) return undefined;
  return (element) => {
    if (element === null) return;
    const listener = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const link = event.target instanceof Element ? event.target.closest("a[href]") : null;
      if (!(link instanceof HTMLAnchorElement) || !element.contains(link) || link.target !== "" || link.hasAttribute("download")) return;
      onLinkClick(event, link);
    };
    element.addEventListener("click", listener);
    return () => {
      element.removeEventListener("click", listener);
    };
  };
}

/**
 * Draws the custom emoji in the body. The body is HTML that React does not render, so the emoji go in
 * after React set it, and come out again before React replaces it: the callback is made anew whenever
 * the body or its emoji change, and React calls the cleanup of the previous one first.
 */
function emojiDrawer(html: SanitizedHtml, emojis: readonly CustomEmojiImage[], allowMotion: boolean): RefCallback<HTMLDivElement> | undefined {
  if (emojis.length === 0 || html === "") return undefined;
  return (element) => {
    if (element === null) return;
    return insertCustomEmojis(element, emojis, allowMotion);
  };
}

/**
 * The body of a post: server-sanitized HTML in a block whose direction follows its own text (`dir="auto"`)
 * and whose `lang` is the post's language when known (web-client.md Y10). Give it an `id` to describe a
 * PostCard by it, `onLinkClick` to open the links of the body itself (the app opens the links to its
 * own pages through its router), and `emojis` to draw its custom emoji.
 */
export function PostContent({ html, lang, onLinkClick, emojis = noCustomEmojis, emojiMotion = true, className, ref, ...props }: PostContentProps) {
  return (
    <div
      {...props}
      ref={mergeRefs(ref, linkClickListener(onLinkClick), emojiDrawer(html, emojis, emojiMotion))}
      dir="auto"
      lang={lang}
      className={postContent({ className })}
      dangerouslySetInnerHTML={{ __html: trustedHtml(html) }}
    />
  );
}
