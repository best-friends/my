// SPDX-License-Identifier: AGPL-3.0-only
// Public surface of @my/ui, the shared UI kit (ADR-0026). The package ships TypeScript
// source with no build step: apps compile it with their own Vite and React Compiler setup,
// and their Tailwind entry scans it with @source.
export { Center, type CenterProps } from "./Center";

// The app-wide settings of the kit's components, which an app renders once at its root, so that it needs
// no direct dependency on react-aria-components:
// - I18nProvider: the locale (built-in strings, number and date formats, and the direction of arrow keys);
// - RouterProvider: the app router's navigate for every Link, Menu item or Tab with an href.
export { I18nProvider, type I18nProviderProps, RouterProvider } from "react-aria-components";

// Form controls.
export { Button, type ButtonProps } from "./Button";
export { Checkbox, type CheckboxProps } from "./Checkbox";
export { DurationSelect, type DurationSelectProps } from "./DurationSelect";
export type { FieldErrorMessage, FieldTextProps } from "./Field";
// The hidden file input behind a Button child (React Aria FileTrigger): it has no look of its own.
export { FileTrigger, type FileTriggerProps } from "react-aria-components";
export { Link, type LinkProps } from "./Link";
export { Radio, RadioGroup, type RadioGroupProps, type RadioProps } from "./RadioGroup";
export { ReasonField, type ReasonFieldProps } from "./ReasonField";
export { Select, SelectItem, type SelectItemProps, type SelectProps } from "./Select";
export { Switch, type SwitchProps } from "./Switch";
export { TextArea, type TextAreaProps } from "./TextArea";
export { TextField, type TextFieldProps } from "./TextField";
export { ToggleButton, type ToggleButtonProps } from "./ToggleButton";

// Overlays, tabs and announcements.
export { ConfirmDialog, type ConfirmDialogProps } from "./ConfirmDialog";
export { AnnouncerProvider, useAnnouncer, type Announce, type AnnouncerProviderProps, type Politeness } from "./Announcer";
export { Dialog, DialogTrigger, Sheet, type DialogProps, type DialogTriggerProps, type SheetProps } from "./Dialog";
export {
  Menu,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
  SubmenuTrigger,
  type MenuItemProps,
  type MenuProps,
  type MenuTriggerProps,
  type SubmenuTriggerProps,
} from "./Menu";
export { Popover, PopoverTrigger, type PopoverProps } from "./Popover";
export { Tab, TabList, TabPanel, Tabs, type TabListProps, type TabPanelProps, type TabProps, type TabsProps } from "./Tabs";
export { ToastProvider, useToast, type ToastApi, type ToastOptions, type ToastProviderProps, type ToastTone } from "./Toast";
export { Tooltip, TooltipTrigger, type TooltipProps, type TooltipTriggerProps } from "./Tooltip";

// Display.
export { Avatar, type AvatarProps } from "./Avatar";
export { Badge, type BadgeProps } from "./Badge";
export { DescriptionItem, DescriptionList, type DescriptionItemProps, type DescriptionListProps } from "./DescriptionList";
export { EmptyState, type EmptyStateProps, type HeadingLevel } from "./EmptyState";
export { OfflineBanner, type OfflineBannerProps } from "./OfflineBanner";
export { RegionError, type RegionErrorProps } from "./RegionError";
export { Skeleton, type SkeletonProps } from "./Skeleton";
export { AccountRowSkeleton, NotificationRowSkeleton, PostSkeleton, type SkeletonRowProps } from "./SkeletonRows";
export { Spinner, type SpinnerProps } from "./Spinner";
export { StatusMessage, type StatusMessageProps } from "./StatusMessage";
export { VisuallyHidden, type VisuallyHiddenProps } from "./VisuallyHidden";

// Icons (web-client.md §16.3): decorative, 24 × 24 with a 2px stroke, sized by the control around them.
export {
  BackIcon,
  BookmarkIcon,
  BoostIcon,
  CloseIcon,
  ExternalLinkIcon,
  FederatedIcon,
  HomeIcon,
  LocalIcon,
  MenuIcon,
  MoreIcon,
  NotificationsIcon,
  PlusIcon,
  ProfileIcon,
  ReplyIcon,
  SearchIcon,
  SettingsIcon,
  StarIcon,
  TimelinesIcon,
  type ToggleIconProps,
} from "./Icons";

// Timeline.
export { ContentWarning, type ContentWarningProps } from "./ContentWarning";
export { Feed, FeedArticle, type FeedArticleProps, type FeedProps } from "./Feed";
export { MediaGrid, type MediaGridProps, type MediaItem } from "./MediaGrid";
export { NewItemsButton, type NewItemsButtonProps } from "./NewItemsButton";
export { PostCard, PostCardAuthor, type PostCardAuthorProps, type PostCardProps } from "./PostCard";
export { PostContent, serverSanitizedHtml, type PostContentProps, type SanitizedHtml } from "./PostContent";
