The icon set of @my/ui (`app/ui/src/Icons.tsx`): Phosphor icons (react-icons/pi, MIT), each exported under a meaningful name. These SVGs are copies for reference and handoff; in the app, import the named component from `@my/ui`.

- **Ink:** the files are drawn in `charcoal` #2E2E2E so they show on a light tile. In code every icon is `currentColor` and takes its control's colour (`fg-muted` at rest, `accent` / `success` / `warning` when on).
- **Weight:** Phosphor Regular. A toggle that is on switches shape — Fill where the shape closes (`StarIcon-filled`, `BookmarkIcon-filled`, `WarningIcon-filled`, `PollIcon-filled`), Bold where it does not (`BoostIcon-filled`) — so state never rests on colour alone.
- **Size:** set by the control, never by the icon: 16px in small buttons and links, 20px in buttons and toggles, 24px in navigation.
- **Direction:** BackIcon, ExternalLinkIcon, SubmenuIcon, ReplyIcon, PreviousIcon, NextIcon, PollIcon and SignInIcon mirror in right-to-left text (`rtl:-scale-x-100`). Up / down arrows never mirror.
- **Decorative:** every icon is `aria-hidden`; the visible label or the control's `aria-label` carries the meaning.

| Name | Phosphor | Use |
| --- | --- | --- |
| CloseIcon | PiX | close and dismiss |
| PlusIcon | PiPlus | create (a post) |
| MoreIcon | PiDotsThreeBold | overflow menu |
| BackIcon | PiArrowLeft | back |
| ExternalLinkIcon | PiArrowSquareOut | after a link that leaves the app |
| SubmenuIcon | PiCaretRight | menu item that opens a submenu |
| MoveUpIcon / MoveDownIcon | PiArrowUp / PiArrowDown | reorder |
| ReplyIcon | PiArrowBendUpLeft | reply |
| NewItemsIcon | PiArrowUp | “N new posts” |
| HideMediaIcon | PiEyeSlash | conceal sensitive media again |
| PreviousIcon / NextIcon | PiCaretLeft / PiCaretRight | media viewer |
| BoostIcon | PiRepeat → PiRepeatBold | boost |
| StarIcon | PiStar → PiStarFill | favourite |
| BookmarkIcon | PiBookmarkSimple → Fill | bookmark |
| ImageIcon | PiImage | attach images |
| WarningIcon | PiWarning → PiWarningFill | content warning, errors |
| EyeIcon | PiEye | post visibility |
| LanguageIcon | PiTranslate | post language |
| EmojiIcon | PiSmiley | emoji picker |
| PollIcon | PiChartBarHorizontal → Fill | add a poll |
| HomeIcon, LocalIcon, FederatedIcon | PiHouse, PiTextAlignLeft, PiGlobe | timelines |
| SearchIcon, NotificationsIcon, ProfileIcon, SettingsIcon | PiMagnifyingGlass, PiBell, PiUser, PiSlidersHorizontal | navigation |
| ModerationIcon, AdminIcon | PiShieldCheck, PiWrench | operators |
| MenuIcon, AboutIcon | PiList, PiInfo | menu, server info |
| SignInIcon, SignUpIcon | PiSignIn, PiUserPlus | account |
