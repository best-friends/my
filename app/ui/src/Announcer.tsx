// SPDX-License-Identifier: AGPL-3.0-only
// The app's single place for screen reader announcements (web-client.md Y7): one polite live region
// (role="status") and one assertive live region (role="alert"), rendered once near the root and present
// from the first render, because a live region only announces changes made after it is in the page.
// Announcing never moves focus.
//
// React Aria Components has no stable live announcer component: its Toast is UNSTABLE_ (react-aria-components
// 1.21) and its live announcer is internal to react-aria, so the kit renders its own regions.
import { createContext, type ReactNode, use, useEffect, useRef, useState } from "react";

/** `polite` waits until the screen reader is idle (counts, completions); `assertive` interrupts (errors). */
export type Politeness = "polite" | "assertive";

/** Reads `message` out once in the given region, without moving focus. */
export type Announce = (message: string, politeness?: Politeness) => void;

const AnnouncerContext = createContext<Announce | null>(null);

type Announcement = { id: number; message: string; politeness: Politeness };

export type AnnouncerProviderProps = {
  children: ReactNode;
  /**
   * How long an announcement stays in its region, in ms. It is removed afterwards so that a screen reader
   * user browsing the page does not meet stale messages, and the same message can be announced again.
   */
  clearAfter?: number;
};

/** Renders the live regions and gives `useAnnouncer()` to its children. Mount it once, at the app root. */
export function AnnouncerProvider({ children, clearAfter = 7000 }: AnnouncerProviderProps) {
  const [announcements, setAnnouncements] = useState<readonly Announcement[]>([]);
  const nextIdRef = useRef(0);
  const timersRef = useRef(new Set<ReturnType<typeof setTimeout>>());

  // Pending removals are timers outside React; clear them when the provider unmounts.
  useEffect(() => {
    const pending = timersRef.current;
    return () => {
      for (const timer of pending) clearTimeout(timer);
      pending.clear();
    };
  }, []);

  const announce: Announce = (message, politeness = "polite") => {
    const id = nextIdRef.current++;
    // A new node per message: screen readers announce added nodes, so a repeated message is read again.
    setAnnouncements((current) => [...current, { id, message, politeness }]);
    const timer = setTimeout(() => {
      timersRef.current.delete(timer);
      setAnnouncements((current) => current.filter((a) => a.id !== id));
    }, clearAfter);
    timersRef.current.add(timer);
  };

  return (
    <AnnouncerContext value={announce}>
      {children}
      {/* role="status" and role="alert" imply aria-atomic="true", which would read the earlier messages
          still in the region again; only the added message is read. data-live-announcer keeps the regions
          exposed while a modal hides the rest of the page from assistive technology (React Aria's ariaHideOutside). */}
      <div role="status" aria-live="polite" aria-atomic="false" data-live-announcer="true" className="sr-only">
        {announcements
          .filter((a) => a.politeness === "polite")
          .map((a) => (
            <p key={a.id}>{a.message}</p>
          ))}
      </div>
      <div role="alert" aria-live="assertive" aria-atomic="false" data-live-announcer="true" className="sr-only">
        {announcements
          .filter((a) => a.politeness === "assertive")
          .map((a) => (
            <p key={a.id}>{a.message}</p>
          ))}
      </div>
    </AnnouncerContext>
  );
}

/** Returns `announce`. Call it from event handlers and mutation callbacks, not during rendering. */
export function useAnnouncer(): Announce {
  const announce = use(AnnouncerContext);
  if (announce === null) throw new Error("useAnnouncer needs an <AnnouncerProvider> above it");
  return announce;
}
