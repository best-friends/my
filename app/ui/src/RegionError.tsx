// SPDX-License-Identifier: AGPL-3.0-only
// The error state of one region of data (web-client/16-look-states-and-wording.md G16): what happened, the reason and "Try again", in place of
// the region's content only, so that a page keeps its h1 and a list keeps the items it has already loaded.
// It is not a live region unless `alert` is set: a failure the user only opened the page to see is found by reading,
// and only the result of the user's own action (a retry that failed again) interrupts as an alert
// (WAI-ARIA APG, Alert: https://www.w3.org/WAI/ARIA/apg/patterns/alert/).
import type { ComponentPropsWithRef, ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Button } from "./Button";
import { WarningIcon } from "./Icons";
import type { HeadingLevel } from "./EmptyState";

const regionError = tv({
  slots: {
    base: "flex items-start gap-3 rounded-lg bg-danger-bg px-4 py-3 text-fg",
    icon: "mt-px shrink-0 text-danger [&>svg]:size-5.5",
    body: "flex min-w-0 flex-col items-start gap-1.5",
    heading: "text-base font-extrabold text-fg",
    reason: "text-sm wrap-anywhere text-fg",
    action: "mt-1 *:bg-surface-raised",
  },
});

type RetryProps =
  | {
      /** The label of the retry button ("Try again"). */
      retryLabel: string;
      /** Requests the region's data again. */
      onRetry: () => void;
      /** The retry is in progress: the button waits and keeps its focus. */
      isRetrying?: boolean;
    }
  | { retryLabel?: never; onRetry?: never; isRetrying?: never };

export type RegionErrorProps = Omit<ComponentPropsWithRef<"div">, "children" | "title" | "role"> &
  RetryProps & {
    /** What happened, in one short sentence ("Couldn't load the posts"). */
    heading: ReactNode;
    /**
     * The heading's level in the page outline, one below the heading of the region it replaces. Without it the
     * message is a paragraph, as for the failure of the next page of a list.
     */
    headingLevel?: HeadingLevel;
    /** Why, from the error (`errorMessage(error)`), or what the user can do about it. */
    reason?: ReactNode;
    /** Makes the error an assertive live region (`role="alert"`); only for the result of the user's own action. */
    alert?: boolean;
  };

/** What a region shows instead of its content when loading it failed. */
export function RegionError({
  heading,
  headingLevel,
  reason,
  retryLabel,
  onRetry,
  isRetrying = false,
  alert = false,
  className,
  ...props
}: RegionErrorProps) {
  const slots = regionError();
  const Heading = headingLevel === undefined ? "p" : (`h${String(headingLevel)}` as `h${HeadingLevel}`);
  return (
    <div {...props} role={alert ? "alert" : undefined} className={slots.base({ className })}>
      <span className={slots.icon()}>
        <WarningIcon filled />
      </span>
      <div className={slots.body()}>
        <Heading className={slots.heading()}>{heading}</Heading>
        {reason !== undefined && <p className={slots.reason()}>{reason}</p>}
        {onRetry !== undefined && (
          <div className={slots.action()}>
            <Button size="sm" onPress={onRetry} isPending={isRetrying}>
              {retryLabel}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
