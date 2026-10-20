// SPDX-License-Identifier: AGPL-3.0-only
// The icon set of the kit (web-client.md §16.3, G10–G13; ADR-0043). The shapes are Phosphor's, through
// react-icons/pi; this file is the only one that imports them. `iconTable` maps each meaningful name
// (`ReplyIcon`) to its Phosphor icon off and on and to whether it has a direction, and every export below
// renders one row of it. The Regular weight is the base; a toggle that is on uses the Fill weight when its
// shape closes (star, bookmark, warning) and the Bold weight when it does not (boost), so the state shows by
// shape and not by color alone (WCAG 1.4.1).
//
// Every icon is decorative (`aria-hidden`, `focusable="false"`): the visible label or the accessible name of
// the control carries the meaning. An icon sets no size of its own; the control around it does (`Button`,
// `ToggleButton` and `Link` size a child `svg`), and `currentColor` gives it the control's color. Icons with
// a direction mirror in right-to-left text with `rtl:-scale-x-100` (Phosphor's `mirrored` ignores the
// writing direction, so it is not used).
import {
  PiArrowBendUpLeft,
  PiArrowLeft,
  PiArrowSquareOut,
  PiArrowDown,
  PiArrowUp,
  PiBell,
  PiBookmarkSimple,
  PiBookmarkSimpleFill,
  PiCaretRight,
  PiDotsThreeBold,
  PiEye,
  PiEyeSlash,
  PiGlobe,
  PiHouse,
  PiImage,
  PiInfo,
  PiList,
  PiMagnifyingGlass,
  PiPlus,
  PiRepeat,
  PiRepeatBold,
  PiShieldCheck,
  PiSignIn,
  PiSmiley,
  PiSlidersHorizontal,
  PiStar,
  PiStarFill,
  PiTextAlignLeft,
  PiTranslate,
  PiUser,
  PiUserPlus,
  PiWarning,
  PiWarningFill,
  PiWrench,
  PiX,
} from "react-icons/pi";

/** The state of a toggle's icon: filled while on, so the state shows by shape and not by color alone (WCAG 1.4.1). */
export type ToggleIconProps = { filled?: boolean };

/**
 * One Phosphor icon and its name in the Phosphor catalog (https://phosphoricons.com/), which the Storybook
 * gallery shows (a minified build renames the components, so the name is written out).
 */
type Glyph = { readonly icon: typeof PiX; readonly phosphor: string };

type IconEntry = {
  readonly off: Glyph;
  /** The shape of a toggle that is on. */
  readonly on?: Glyph;
  /** Mirrors the icon in right-to-left text, for an icon whose direction follows the line (reply, back). */
  readonly directional?: boolean;
};

const glyph = (icon: typeof PiX, phosphor: string): Glyph => ({ icon, phosphor });

/** Every icon of the kit: its meaningful name, and the Phosphor icons that draw it. */
export const iconTable = {
  // Actions.
  CloseIcon: { off: glyph(PiX, "PiX") },
  PlusIcon: { off: glyph(PiPlus, "PiPlus") },
  // Regular's dots are too small to read at size-5, so the overflow menu uses Bold's.
  MoreIcon: { off: glyph(PiDotsThreeBold, "PiDotsThreeBold") },
  BackIcon: { off: glyph(PiArrowLeft, "PiArrowLeft"), directional: true },
  ExternalLinkIcon: { off: glyph(PiArrowSquareOut, "PiArrowSquareOut"), directional: true },
  SubmenuIcon: { off: glyph(PiCaretRight, "PiCaretRight"), directional: true },
  // Up and down are the order of a list in any writing direction, so they do not mirror.
  MoveUpIcon: { off: glyph(PiArrowUp, "PiArrowUp") },
  MoveDownIcon: { off: glyph(PiArrowDown, "PiArrowDown") },
  // Posts.
  ReplyIcon: { off: glyph(PiArrowBendUpLeft, "PiArrowBendUpLeft"), directional: true },
  // Up is where the new items wait (above the feed) in any writing direction, so it does not mirror.
  NewItemsIcon: { off: glyph(PiArrowUp, "PiArrowUp") },
  HideMediaIcon: { off: glyph(PiEyeSlash, "PiEyeSlash") },
  // A loop means a loop and not a direction, so it does not mirror; it has nothing to fill, so on is Bold.
  BoostIcon: { off: glyph(PiRepeat, "PiRepeat"), on: glyph(PiRepeatBold, "PiRepeatBold") },
  StarIcon: { off: glyph(PiStar, "PiStar"), on: glyph(PiStarFill, "PiStarFill") },
  BookmarkIcon: {
    off: glyph(PiBookmarkSimple, "PiBookmarkSimple"),
    on: glyph(PiBookmarkSimpleFill, "PiBookmarkSimpleFill"),
  },
  // The new-post form.
  ImageIcon: { off: glyph(PiImage, "PiImage") },
  // Filled, the exclamation mark stays cut out of the triangle.
  WarningIcon: { off: glyph(PiWarning, "PiWarning"), on: glyph(PiWarningFill, "PiWarningFill") },
  EyeIcon: { off: glyph(PiEye, "PiEye") },
  LanguageIcon: { off: glyph(PiTranslate, "PiTranslate") },
  EmojiIcon: { off: glyph(PiSmiley, "PiSmiley") },
  // Navigation.
  HomeIcon: { off: glyph(PiHouse, "PiHouse") },
  LocalIcon: { off: glyph(PiTextAlignLeft, "PiTextAlignLeft") },
  FederatedIcon: { off: glyph(PiGlobe, "PiGlobe") },
  SearchIcon: { off: glyph(PiMagnifyingGlass, "PiMagnifyingGlass") },
  NotificationsIcon: { off: glyph(PiBell, "PiBell") },
  ProfileIcon: { off: glyph(PiUser, "PiUser") },
  SettingsIcon: { off: glyph(PiSlidersHorizontal, "PiSlidersHorizontal") },
  ModerationIcon: { off: glyph(PiShieldCheck, "PiShieldCheck") },
  AdminIcon: { off: glyph(PiWrench, "PiWrench") },
  MenuIcon: { off: glyph(PiList, "PiList") },
  AboutIcon: { off: glyph(PiInfo, "PiInfo") },
  // The arrow points into the door along the line, so it mirrors in right-to-left text.
  SignInIcon: { off: glyph(PiSignIn, "PiSignIn"), directional: true },
  SignUpIcon: { off: glyph(PiUserPlus, "PiUserPlus") },
} as const satisfies Record<string, IconEntry>;

function renderIcon(entry: IconEntry, on = false) {
  const { icon: Glyph } = (on && entry.on) || entry.off;
  return <Glyph aria-hidden="true" focusable="false" className={entry.directional ? "rtl:-scale-x-100" : undefined} />;
}

// Actions.

/** The × of close and dismiss buttons. */
export function CloseIcon() {
  return renderIcon(iconTable.CloseIcon);
}

/** A plus, for a button that creates something (a post). */
export function PlusIcon() {
  return renderIcon(iconTable.PlusIcon);
}

/** Three dots, for the button of an overflow menu. */
export function MoreIcon() {
  return renderIcon(iconTable.MoreIcon);
}

/** An arrow to the start of the line, for a link back to the page before. It flips in right-to-left text. */
export function BackIcon() {
  return renderIcon(iconTable.BackIcon);
}

/** A box with an arrow leaving it, after the text of a link to another site. It flips in right-to-left text. */
export function ExternalLinkIcon() {
  return renderIcon(iconTable.ExternalLinkIcon);
}

/** A caret to the end of the line, after the label of a menu item that opens a submenu. It flips in right-to-left text. */
export function SubmenuIcon() {
  return renderIcon(iconTable.SubmenuIcon);
}

/** An arrow up, for a button that moves an item one place up in a list. */
export function MoveUpIcon() {
  return renderIcon(iconTable.MoveUpIcon);
}

/** An arrow down, for a button that moves an item one place down in a list. */
export function MoveDownIcon() {
  return renderIcon(iconTable.MoveDownIcon);
}

// Posts.

/** An arrow turning back to the start of the line, for replying. It flips in right-to-left text. */
export function ReplyIcon() {
  return renderIcon(iconTable.ReplyIcon);
}

/** An arrow up, for the button that shows the new posts waiting above a feed. */
export function NewItemsIcon() {
  return renderIcon(iconTable.NewItemsIcon);
}

/** An eye struck through, for the button that conceals sensitive media again. */
export function HideMediaIcon() {
  return renderIcon(iconTable.HideMediaIcon);
}

/** Two arrows in a loop, for boosting; bolder while on. */
export function BoostIcon({ filled = false }: ToggleIconProps) {
  return renderIcon(iconTable.BoostIcon, filled);
}

/** A star, for favouriting; filled while on. */
export function StarIcon({ filled = false }: ToggleIconProps) {
  return renderIcon(iconTable.StarIcon, filled);
}

/** A bookmark ribbon, for bookmarking; filled while on. */
export function BookmarkIcon({ filled = false }: ToggleIconProps) {
  return renderIcon(iconTable.BookmarkIcon, filled);
}

// The new-post form.

/** A framed picture, for attaching images. */
export function ImageIcon() {
  return renderIcon(iconTable.ImageIcon);
}

/** A triangle with an exclamation mark, for a content warning; filled while on. */
export function WarningIcon({ filled = false }: ToggleIconProps) {
  return renderIcon(iconTable.WarningIcon, filled);
}

/** An eye, for who can see a post (its visibility). */
export function EyeIcon() {
  return renderIcon(iconTable.EyeIcon);
}

/** Two scripts side by side, for the language of a post. */
export function LanguageIcon() {
  return renderIcon(iconTable.LanguageIcon);
}

/** A smiling face, for the button that opens the picker of custom emoji. */
export function EmojiIcon() {
  return renderIcon(iconTable.EmojiIcon);
}

// Navigation.

/** A house, for the home timeline. */
export function HomeIcon() {
  return renderIcon(iconTable.HomeIcon);
}

/** Lines of text, for the local timeline. */
export function LocalIcon() {
  return renderIcon(iconTable.LocalIcon);
}

/** A globe, for the federated timeline. */
export function FederatedIcon() {
  return renderIcon(iconTable.FederatedIcon);
}

/** A magnifying glass, for search. */
export function SearchIcon() {
  return renderIcon(iconTable.SearchIcon);
}

/** A bell, for notifications. */
export function NotificationsIcon() {
  return renderIcon(iconTable.NotificationsIcon);
}

/** A head and shoulders, for a profile. */
export function ProfileIcon() {
  return renderIcon(iconTable.ProfileIcon);
}

/** Sliders, for settings. */
export function SettingsIcon() {
  return renderIcon(iconTable.SettingsIcon);
}

/** A shield with a check, for the moderation. */
export function ModerationIcon() {
  return renderIcon(iconTable.ModerationIcon);
}

/** A wrench, for the server administration. */
export function AdminIcon() {
  return renderIcon(iconTable.AdminIcon);
}

/** Three lines, for the button that opens the menu with the rest of the navigation. */
export function MenuIcon() {
  return renderIcon(iconTable.MenuIcon);
}

/** An "i" in a circle, for the server's information page. */
export function AboutIcon() {
  return renderIcon(iconTable.AboutIcon);
}

/** An arrow into a door, for signing in. */
export function SignInIcon() {
  return renderIcon(iconTable.SignInIcon);
}

/** A person with a plus, for creating an account. */
export function SignUpIcon() {
  return renderIcon(iconTable.SignUpIcon);
}
