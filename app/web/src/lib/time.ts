// SPDX-License-Identifier: AGPL-3.0-only
// When something happened, as the lists show it (web-client.md G8): relative to now ("5 min. ago"), with
// the full date and time for `<time dateTime>` and its tooltip. Timelines and notifications use the same rule.
import { type IntlShape, useIntl } from "react-intl";
import { useNow } from "./clock";

export type TimeView = {
  /** The ISO date and time, for the `dateTime` of `<time>`. */
  dateTime: string;
  /** The time from now back to it, in the largest unit that fits, or the date for a week or more. */
  relative: string;
  /** The full date and time. */
  absolute: string;
};

const minute = 60_000;
const hour = 60 * minute;
const day = 24 * hour;

/**
 * The time from `now` back to `then`, in the largest unit that fits ("now", "5 min. ago", "3 hr. ago",
 * "2 days ago"), or the date for a week or more.
 */
function relativeTime(intl: IntlShape, then: number, now: number): string {
  const elapsed = Math.max(0, now - then);
  if (elapsed < minute) return intl.formatRelativeTime(0, "second", { numeric: "auto" });
  if (elapsed < hour) return intl.formatRelativeTime(-Math.floor(elapsed / minute), "minute", { style: "short" });
  if (elapsed < day) return intl.formatRelativeTime(-Math.floor(elapsed / hour), "hour", { style: "short" });
  if (elapsed < 7 * day) return intl.formatRelativeTime(-Math.floor(elapsed / day), "day", { style: "short" });
  return intl.formatDate(then, { dateStyle: "medium" });
}

/** Returns how a list shows the moment `dateTime` (ISO 8601); it renders again as the clock ticks. */
export function useTimeView(dateTime: string): TimeView {
  const intl = useIntl();
  const now = useNow();
  const then = Date.parse(dateTime);
  return {
    dateTime,
    relative: relativeTime(intl, then, now),
    absolute: intl.formatDate(then, { dateStyle: "full", timeStyle: "short" }),
  };
}
