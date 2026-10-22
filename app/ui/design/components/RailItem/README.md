A row of the left navigation rail: icon, label and an optional count. **Brand kit pattern — not in @my/ui yet.** In code it is a `Link variant="navigation"` with the current item in a pill.

**Consumer provides:** an @my/ui navigation icon (HomeIcon, SearchIcon, NotificationsIcon, ProfileIcon, SettingsIcon…), the label, an optional Badge, and `aria-current="page"` on the current page.

- 24px icon, 15px/700 label, 14px radius, 11×15 padding; hover `surface-sunken`.
- Current: `soft` fill with an `accent` label and icon (4.54:1 light).
- Under 700px the rail becomes a fixed bottom bar of icon + 10px label.
