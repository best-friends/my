// SPDX-License-Identifier: AGPL-3.0-only
import { useRouter } from "@tanstack/react-router";
import { type RefObject, useLayoutEffect, useRef } from "react";

/**
 * Scrolls the element of `ref` to the top of the viewport once, the first time `when` is true: the post a
 * page is about, when the posts it replies to appear above it and would push it off the first screen.
 * It runs before the browser paints, so the page never shows the post pushed down first. Focus stays
 * where it is (the page's `h1`, web-client.md Y5); only the scroll position changes. Mount the component
 * again (a `key`) to reveal again.
 *
 * The router places the page itself once it has rendered (scroll restoration, on its `onRendered` event,
 * which comes after this component's layout effects in the same commit): to the top for a new page, or to
 * the saved position when going back. When the element is revealed in the commit that renders the page,
 * it is revealed again right after a new page's scroll to the top; a restored position is left alone.
 */
export function useRevealOnce(ref: RefObject<HTMLElement | null>, when: boolean): void {
  const router = useRouter();
  const revealedRef = useRef(false);
  useLayoutEffect(() => {
    if (!when || revealedRef.current) return;
    const reveal = () => {
      ref.current?.scrollIntoView({ block: "start" });
    };
    reveal();
    // The router's event of this commit comes before the next frame; one after it is another render's.
    const unsubscribe = router.subscribe("onRendered", () => {
      if (window.scrollY === 0) reveal();
    });
    const frame = requestAnimationFrame(() => {
      revealedRef.current = true;
      unsubscribe();
    });
    return () => {
      cancelAnimationFrame(frame);
      unsubscribe();
    };
  }, [router, ref, when]);
}
