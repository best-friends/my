// SPDX-License-Identifier: AGPL-3.0-only
// Public surface of @my/ui, the shared UI kit (ADR-0026). The package ships TypeScript
// source with no build step: apps compile it with their own Vite and React Compiler setup,
// and their Tailwind entry scans it with @source.
export { Center, type CenterProps } from "./Center";
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
