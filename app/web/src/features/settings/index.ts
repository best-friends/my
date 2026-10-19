// SPDX-License-Identifier: AGPL-3.0-only
// The settings feature (FR-006, FR-007, FR-208, FR-302, FR-303): the profile editor, the preferences, the
// lists of blocked and muted accounts and of follow requests, displaying the app in the user's locale
// and time zone, and the color theme of this browser. Routes and other features use it through this file only (web-client.md A3).
export { credentialsQueryOptions, preferencesQueryOptions } from "./api/me";
export { blocksQueryOptions, followRequestsQueryOptions, mutesQueryOptions } from "./api/relationships";
export { BlockedAccounts, FollowRequests, MutedAccounts } from "./components/AccountLists";
export { PreferencesForm } from "./components/PreferencesForm";
export { ProfileForm } from "./components/ProfileForm";
export { SettingsMenu } from "./components/SettingsMenu";
export { ThemeRadioGroup } from "./components/ThemeRadioGroup";
export { useUserDisplaySettings } from "./hooks/useUserDisplaySettings";
// Asking before leaving a form with unsaved changes (web-client.md §9), which the admin screens' forms share.
export { useUnsavedChangesGuard } from "./hooks/useUnsavedChangesGuard";
