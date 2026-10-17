// SPDX-License-Identifier: AGPL-3.0-only
// The icon set of the kit (web-client.md §16.3, G10–G13). Every icon is drawn on a 24 × 24 grid with a
// 2px round stroke, and is decorative (`aria-hidden`, `focusable="false"`): the visible label or the
// accessible name of the control carries the meaning. An icon sets no size of its own; the control
// around it does (`Button`, `ToggleButton` and `Link` size a child `svg`), and `currentColor` gives it
// the control's color. Icons with a direction mirror in right-to-left text.
import type { ReactNode } from "react";

/** The state of a toggle's icon: filled while on, so the state shows by shape and not by color alone (WCAG 1.4.1). */
export type ToggleIconProps = { filled?: boolean };

type IconProps = {
  children: ReactNode;
  filled?: boolean;
  /** A heavier stroke, for an on state that has no fill. */
  bold?: boolean;
  /** Mirrors the icon in right-to-left text, for an icon whose direction follows the line (reply, back). */
  directional?: boolean;
};

function Icon({ children, filled = false, bold = false, directional = false }: IconProps) {
  return (
    <svg
      aria-hidden="true"
      focusable="false"
      viewBox="0 0 24 24"
      fill={filled ? "currentColor" : "none"}
      stroke="currentColor"
      strokeWidth={bold ? 3 : 2}
      strokeLinecap="round"
      strokeLinejoin="round"
      {...(directional && { className: "rtl:-scale-x-100" })}
    >
      {children}
    </svg>
  );
}

// Actions.

/** The × of close and dismiss buttons. */
export function CloseIcon() {
  return (
    <Icon>
      <path d="M6 6l12 12M18 6 6 18" />
    </Icon>
  );
}

/** A plus, for a button that creates something (a post). */
export function PlusIcon() {
  return (
    <Icon>
      <path d="M12 5v14M5 12h14" />
    </Icon>
  );
}

/** Three dots, for the button of an overflow menu. */
export function MoreIcon() {
  return (
    <Icon filled>
      <circle cx="5" cy="12" r="1" />
      <circle cx="12" cy="12" r="1" />
      <circle cx="19" cy="12" r="1" />
    </Icon>
  );
}

/** An arrow to the start of the line, for a link back to the page before. It flips in right-to-left text. */
export function BackIcon() {
  return (
    <Icon directional>
      <path d="M19 12H5M11 18l-6-6 6-6" />
    </Icon>
  );
}

/**
 * A box with an arrow leaving it, after the text of a link to another site. The arrow points to the end
 * of the line, so it flips in right-to-left text.
 */
export function ExternalLinkIcon() {
  return (
    <Icon directional>
      <path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" />
    </Icon>
  );
}

// Posts.

/** An arrow turning back to the start of the line, for replying. It flips in right-to-left text. */
export function ReplyIcon() {
  return (
    <Icon directional>
      <path d="M9 14 4 9l5-5" />
      <path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11" />
    </Icon>
  );
}

/**
 * Two arrows in a loop, for boosting. A loop has nothing to fill, so a boost that is on draws its arrows
 * bolder instead. It means a loop and not a direction, so it does not flip.
 */
export function BoostIcon({ filled = false }: ToggleIconProps) {
  return (
    <Icon bold={filled}>
      <path d="M17 2l4 4-4 4M3 11V9a3 3 0 0 1 3-3h15M7 22l-4-4 4-4M21 13v2a3 3 0 0 1-3 3H3" />
    </Icon>
  );
}

/** A star, for favouriting. */
export function StarIcon({ filled = false }: ToggleIconProps) {
  return (
    <Icon filled={filled}>
      <path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1 6.2L12 17.3 6.5 20.2l1-6.2L3 9.6l6.2-.9z" />
    </Icon>
  );
}

/** A bookmark ribbon, for bookmarking. */
export function BookmarkIcon({ filled = false }: ToggleIconProps) {
  return (
    <Icon filled={filled}>
      <path d="M6 3h12v18l-6-4-6 4z" />
    </Icon>
  );
}

// Navigation.

/** A house, for the home timeline. */
export function HomeIcon() {
  return (
    <Icon>
      <path d="M3 10.5 12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z" />
    </Icon>
  );
}

/** Lines of text, for the local timeline. */
export function LocalIcon() {
  return (
    <Icon>
      <path d="M4 6h16M4 12h16M4 18h10" />
    </Icon>
  );
}

/** A card with the edge of another behind it, for the screen with the local and federated timelines. */
export function TimelinesIcon() {
  return (
    <Icon>
      <rect x="4" y="9" width="16" height="11" rx="2" />
      <path d="M7 5h10" />
    </Icon>
  );
}

/** A globe, for the federated timeline. */
export function FederatedIcon() {
  return (
    <Icon>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18" />
    </Icon>
  );
}

/** A magnifying glass, for search. */
export function SearchIcon() {
  return (
    <Icon>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-4-4" />
    </Icon>
  );
}

/** A bell, for notifications. */
export function NotificationsIcon() {
  return (
    <Icon>
      <path d="M6 16v-5a6 6 0 1 1 12 0v5l2 2H4zM10 21h4" />
    </Icon>
  );
}

/** A head and shoulders, for a profile. */
export function ProfileIcon() {
  return (
    <Icon>
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21a8 8 0 0 1 16 0" />
    </Icon>
  );
}

/** Two sliders, for settings. */
export function SettingsIcon() {
  return (
    <Icon>
      <path d="M4 7h9M17 7h3M4 17h3M11 17h9" />
      <circle cx="15" cy="7" r="2" />
      <circle cx="9" cy="17" r="2" />
    </Icon>
  );
}

/** Three lines, for the button that opens the menu with the rest of the navigation. */
export function MenuIcon() {
  return (
    <Icon>
      <path d="M4 6h16M4 12h16M4 18h16" />
    </Icon>
  );
}
