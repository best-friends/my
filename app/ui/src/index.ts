// SPDX-License-Identifier: AGPL-3.0-only
// Public surface of @my/ui, the shared UI kit (ADR-0026). The package ships TypeScript
// source with no build step: apps compile it with their own Vite and React Compiler setup,
// and their Tailwind entry scans it with @source.
export { Center, type CenterProps } from "./Center";

// Form controls.
export { Button, type ButtonProps } from "./Button";
export { Checkbox, type CheckboxProps } from "./Checkbox";
export type { FieldErrorMessage, FieldTextProps } from "./Field";
export { Link, type LinkProps } from "./Link";
export { Radio, RadioGroup, type RadioGroupProps, type RadioProps } from "./RadioGroup";
export { Select, SelectItem, type SelectItemProps, type SelectProps } from "./Select";
export { Switch, type SwitchProps } from "./Switch";
export { TextArea, type TextAreaProps } from "./TextArea";
export { TextField, type TextFieldProps } from "./TextField";

// Overlays, tabs and announcements.
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
