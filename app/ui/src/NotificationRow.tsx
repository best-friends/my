// SPDX-License-Identifier: AGPL-3.0-only
// The parts of a notification's row that PostCard does not have: the avatars of the others of a group above the
// sentence, the sentence with its unread mark, and the links that lead on. PostCard lays the row out.
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Cluster } from "./Cluster";
import { Text } from "./Text";
import { VisuallyHidden } from "./VisuallyHidden";

const row = tv({
  slots: {
    others: "mb-1 flex flex-wrap gap-1",
    mark: "inline-block size-2 shrink-0 self-center rounded-full bg-accent",
  },
});

export type NotificationOthersProps = {
  /** The other people of a group: `PostCardAvatarLink`s around small decorative avatars. */
  children: ReactNode;
};

/** The avatars of the others of a group, in a row above the sentence. */
export function NotificationOthers({ children }: NotificationOthersProps) {
  return <div className={row().others()}>{children}</div>;
}

export type NotificationSentenceProps = {
  /** Whether it was unread when the page opened: a dot before the sentence and a heavier weight. */
  isUnread: boolean;
  /** What the dot means for screen readers ("Unread"); read before the sentence. */
  unreadLabel: string;
  /** The sentence, with a `PostCardNameLink` for the name in it. */
  children: ReactNode;
};

/** The sentence that says what happened, in the author slot of the row. */
export function NotificationSentence({ isUnread, unreadLabel, children }: NotificationSentenceProps) {
  return (
    <Cluster as="p" align="baseline" gap="2" wrap={false}>
      {isUnread && (
        <>
          <span aria-hidden="true" className={row().mark()} />
          {/* The space keeps the words apart in the article's name. */}
          <VisuallyHidden>{unreadLabel}</VisuallyHidden>{" "}
        </>
      )}
      <Text weight={isUnread ? "semibold" : "normal"}>{children}</Text>
    </Cluster>
  );
}

/** The links of the row's actions side by side: where the user follows the notification up. */
export function NotificationLinks({ children }: { children: ReactNode }) {
  return (
    <Cluster gap="4" rowGap="0">
      {children}
    </Cluster>
  );
}
