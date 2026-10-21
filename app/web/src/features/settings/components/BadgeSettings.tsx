// SPDX-License-Identifier: AGPL-3.0-only
import { RoleBadge, Switch } from "@my/ui";
import { useId } from "react";
import { useT } from "../../../lib/i18n";
import { useBadgeChoices } from "../hooks/useBadgeChoices";

/**
 * The badges of the user's roles on their profile (ADR-0071; the APG switch pattern): one switch per badge, named by
 * the badge itself, saved as soon as it changes. Without a role that has a badge, the section is not shown.
 */
export function BadgeSettings() {
  const t = useT();
  const headingId = useId();
  const badges = useBadgeChoices();
  if (badges.length === 0) return null;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:settings.profile.badges")}
      </h2>
      <p className="text-sm text-fg-muted">{t("client:settings.profile.badges.description")}</p>
      <ul className="flex flex-col gap-3">
        {badges.map((badge) => (
          <li key={badge.roleId}>
            <Switch
              isSelected={badge.displayed}
              onChange={badge.onChange}
              {...(badge.error !== undefined && {
                isInvalid: true,
                errorMessage: t("client:settings.profile.badges.failed", { reason: badge.error }),
              })}
            >
              <RoleBadge label={badge.label} color={badge.color} />
            </Switch>
          </li>
        ))}
      </ul>
    </section>
  );
}
