// SPDX-License-Identifier: AGPL-3.0-only
// The icons of a post's actions. They are decorative (the buttons are named by their labels); a toggle's
// icon is filled while it is on, so its state shows by shape and not by color alone (WCAG 1.4.1).
import type { ReactNode } from "react";

type IconProps = { filled?: boolean };

function Icon({ children, filled = false }: IconProps & { children: ReactNode }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill={filled ? "currentColor" : "none"}
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {children}
    </svg>
  );
}

export function ReplyIcon() {
  return (
    <Icon>
      <path d="M9 14 4 9l5-5" />
      <path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11" />
    </Icon>
  );
}

/** Two arrows in a loop; it has no fill, so a boost on draws its arrows bolder instead. */
export function BoostIcon({ filled = false }: IconProps) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={filled ? "3" : "2"}
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M17 2l4 4-4 4M3 11V9a3 3 0 0 1 3-3h15M7 22l-4-4 4-4M21 13v2a3 3 0 0 1-3 3H3" />
    </svg>
  );
}

export function StarIcon({ filled }: IconProps) {
  return (
    <Icon filled={filled ?? false}>
      <path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1 6.2L12 17.3 6.5 20.2l1-6.2L3 9.6l6.2-.9z" />
    </Icon>
  );
}

export function BookmarkIcon({ filled }: IconProps) {
  return (
    <Icon filled={filled ?? false}>
      <path d="M6 3h12v18l-6-4-6 4z" />
    </Icon>
  );
}

export function MoreIcon() {
  return (
    <Icon filled>
      <circle cx="5" cy="12" r="1" />
      <circle cx="12" cy="12" r="1" />
      <circle cx="19" cy="12" r="1" />
    </Icon>
  );
}
