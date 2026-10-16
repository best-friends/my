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

// Display.
export { Avatar, type AvatarProps } from "./Avatar";
export { Badge, type BadgeProps } from "./Badge";
export { EmptyState, type EmptyStateProps, type HeadingLevel } from "./EmptyState";
export { Skeleton, type SkeletonProps } from "./Skeleton";
export { Spinner, type SpinnerProps } from "./Spinner";
export { VisuallyHidden, type VisuallyHiddenProps } from "./VisuallyHidden";
