// SPDX-License-Identifier: AGPL-3.0-only
import { Button, Checkbox, Radio, RadioGroup, Select, SelectItem, Switch } from "@my/ui";
import { useId, useRef } from "react";
import { languageOptions } from "../../compose";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useDisplaySettings } from "../../../lib/locale";
import type { Visibility } from "../api/me";
import { displayLanguageOf, displayLanguageOptions, timelineLanguageOptions } from "../hooks/languages";
import { useModeratorEmail } from "../hooks/useModeratorEmail";
import { usePushNotifications } from "../hooks/usePushNotifications";
import { deviceTimeZone, sameTimeZone, useTimeZoneOptions } from "../hooks/timeZones";
import { type PreferenceControl, usePreferences } from "../hooks/usePreferences";
import { AutoplaySwitch } from "./AutoplaySwitch";
import { NavigationSettings } from "./NavigationSettings";
import { ThemeRadioGroup } from "./ThemeRadioGroup";

/** The visibilities in the order of ADR-0017, widest first, with their label and description. */
const visibilities: readonly { id: Visibility; label: ClientMessageKey; description: ClientMessageKey }[] = [
  { id: "public", label: "client:settings.visibility.public", description: "client:settings.visibility.public.description" },
  { id: "local", label: "client:settings.visibility.local", description: "client:settings.visibility.local.description" },
  { id: "unlisted", label: "client:settings.visibility.unlisted", description: "client:settings.visibility.unlisted.description" },
  { id: "private", label: "client:settings.visibility.private", description: "client:settings.visibility.private.description" },
  { id: "direct", label: "client:settings.visibility.direct", description: "client:settings.visibility.direct.description" },
];

/**
 * The preferences: every control saves its value as soon as it changes; the theme, the autoplay of animations and the
 * arrangement of the navigation are kept on this browser.
 */
export function PreferencesForm() {
  const t = useT();
  const { locale, timeZone, defaultVisibility, defaultPostLanguage, requireMediaDescription, timelineLanguages, emailModeratorNotifications } =
    usePreferences();
  return (
    <div className="flex flex-col gap-8">
      <p className="text-fg-muted">{t("client:settings.preferences.autosave")}</p>
      <DisplayLanguageSelect control={locale} />
      <TimeZoneSelect control={timeZone} />
      <ThemeRadioGroup />
      <AutoplaySwitch />
      <NavigationSettings />
      <RadioGroup
        label={t("client:settings.default_visibility.label")}
        description={t("client:settings.default_visibility.description")}
        value={defaultVisibility.value}
        onChange={(value) => {
          const visibility = visibilities.find((v) => v.id === value);
          if (visibility !== undefined) defaultVisibility.onChange(visibility.id);
        }}
        isInvalid={defaultVisibility.error !== undefined}
        errorMessage={defaultVisibility.error}
      >
        {visibilities.map((visibility) => (
          <Radio key={visibility.id} value={visibility.id} description={t(visibility.description)}>
            {t(visibility.label)}
          </Radio>
        ))}
      </RadioGroup>
      <DefaultPostLanguageSelect control={defaultPostLanguage} />
      <Switch
        isSelected={requireMediaDescription.value}
        onChange={requireMediaDescription.onChange}
        description={t("client:settings.require_media_description.description")}
        isInvalid={requireMediaDescription.error !== undefined}
        errorMessage={requireMediaDescription.error}
      >
        {t("client:settings.require_media_description.label")}
      </Switch>
      <TimelineLanguages control={timelineLanguages} />
      <PushNotifications />
      <ModeratorNotifications control={emailModeratorNotifications} />
    </div>
  );
}

/**
 * Web Push on this device (FR-206), when the server sends it: a switch that turns it on for this signed-in app (the
 * APG switch pattern), and the kinds of notifications to push as a group of checkboxes; or why it cannot be on.
 */
function PushNotifications() {
  const t = useT();
  const headingId = useId();
  const kindsId = useId();
  const push = usePushNotifications();
  if (push.status === "hidden") return null;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:settings.push.title")}
      </h2>
      {push.status === "unsupported" && <p className="text-sm text-fg-muted">{t("client:settings.push.unsupported")}</p>}
      {push.status === "denied" && <p className="text-sm text-fg-muted">{t("client:settings.push.denied")}</p>}
      {push.status === "available" && (
        <>
          <Switch
            isSelected={push.enabled}
            onChange={push.setEnabled}
            isDisabled={push.isPending}
            description={t("client:settings.push.enable.description")}
          >
            {t("client:settings.push.enable")}
          </Switch>
          {push.enabled && (
            <fieldset aria-describedby={kindsId} className="@container flex min-w-0 flex-col gap-2">
              <legend className="text-sm font-medium text-fg">{t("client:settings.push.types.label")}</legend>
              <p id={kindsId} className="text-sm text-fg-muted">
                {t("client:settings.push.types.description")}
              </p>
              <ul className="grid grid-cols-1 gap-x-6 gap-y-1 @sm:grid-cols-2">
                {push.groups.map((group) => (
                  <li key={group.id}>
                    <Checkbox
                      isSelected={group.types.every((type) => push.alerts.includes(type))}
                      isDisabled={push.isPending}
                      onChange={(selected) => {
                        push.setGroup(group, selected);
                      }}
                    >
                      {t(group.label)}
                    </Checkbox>
                  </li>
                ))}
              </ul>
            </fieldset>
          )}
          {push.error !== undefined && (
            <p role="alert" className="text-sm text-danger">
              {push.error}
            </p>
          )}
        </>
      )}
    </section>
  );
}

/**
 * The moderator notifications, for a viewer whose roles get them: a switch that also sends them by email
 * (the APG switch pattern), or why they come only in the app when the server sends no email.
 */
function ModeratorNotifications({ control }: { control: PreferenceControl<boolean> }) {
  const t = useT();
  const headingId = useId();
  const email = useModeratorEmail(control);
  if (email.status === "hidden") return null;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:settings.moderator_notifications.title")}
      </h2>
      {email.status === "no-email" ? (
        <p className="text-sm text-fg-muted">{t("client:settings.moderator_notifications.no_email")}</p>
      ) : (
        <Switch
          isSelected={email.control.value}
          onChange={email.control.onChange}
          description={t("client:settings.moderator_notifications.email.description")}
          isInvalid={email.control.error !== undefined}
          errorMessage={email.control.error}
        >
          {t("client:settings.moderator_notifications.email")}
        </Switch>
      )}
    </section>
  );
}

function DisplayLanguageSelect({ control }: { control: PreferenceControl<string> }) {
  const t = useT();
  const { i18n } = useDisplaySettings();
  const value = displayLanguageOf(control.value);
  return (
    <Select
      label={t("client:settings.locale.label")}
      description={t("client:settings.locale.description")}
      value={value}
      onChange={(key) => {
        if (typeof key === "string" && key !== value) control.onChange(key);
      }}
      isInvalid={control.error !== undefined}
      errorMessage={control.error}
      items={displayLanguageOptions(i18n.locale)}
    >
      {(item) => (
        // The name is in its own language, so a screen reader reads it in that language.
        <SelectItem id={item.id} textValue={item.name}>
          <span lang={item.id}>{item.name}</span>
        </SelectItem>
      )}
    </Select>
  );
}

/**
 * The language new posts start in (FR-007): the choices of the post form's language select, "undetermined"
 * (`und`) first, so that what is chosen here is what the form starts with.
 */
function DefaultPostLanguageSelect({ control }: { control: PreferenceControl<string> }) {
  const t = useT();
  const { i18n } = useDisplaySettings();
  return (
    <Select
      label={t("client:settings.default_post_language.label")}
      description={t("client:settings.default_post_language.description")}
      value={control.value}
      onChange={(key) => {
        if (typeof key === "string" && key !== control.value) control.onChange(key);
      }}
      isInvalid={control.error !== undefined}
      errorMessage={control.error}
      items={languageOptions(i18n.locale, [control.value], t("client:compose.language.und"))}
    >
      {(item) => (
        <SelectItem id={item.id} textValue={item.name}>
          {item.name}
        </SelectItem>
      )}
    </Select>
  );
}

/**
 * The time zone select, and under it, while the chosen zone is not this device's, a note that names the
 * device's zone with a button that chooses it.
 */
function TimeZoneSelect({ control }: { control: PreferenceControl<string> }) {
  const t = useT();
  const options = useTimeZoneOptions(control.value);
  const groupRef = useRef<HTMLDivElement>(null);
  const device = deviceTimeZone();
  const differs = !sameTimeZone(control.value, device);
  const deviceLabel = options.find((option) => sameTimeZone(option.id, device))?.label ?? device;
  return (
    <div ref={groupRef} className="flex flex-col gap-3">
      <TimeZoneField control={control} options={options} />
      {differs && (
        <div className="flex flex-col items-start gap-2 rounded-md border border-s-4 border-border border-s-border-strong bg-surface-raised px-4 py-3">
          <p className="text-sm wrap-anywhere text-fg">{t("client:settings.time_zone.device_differs", { zone: deviceLabel })}</p>
          <Button
            size="sm"
            onPress={() => {
              // The button goes away with the difference: focus moves to the select, which shows the new zone.
              groupRef.current?.querySelector<HTMLElement>("button")?.focus();
              control.onChange(device);
            }}
          >
            {t("client:settings.time_zone.use_device")}
          </Button>
        </div>
      )}
    </div>
  );
}

/**
 * The time zone: each option is the zone's name in the display language with its difference from UTC, and is
 * described by its IANA name, which tells apart the zones of one name (America/New_York, America/Detroit).
 */
function TimeZoneField({ control, options }: { control: PreferenceControl<string>; options: ReturnType<typeof useTimeZoneOptions> }) {
  const t = useT();
  return (
    <Select
      label={t("client:settings.time_zone.label")}
      description={t("client:settings.time_zone.description")}
      value={control.value}
      onChange={(key) => {
        if (typeof key === "string") control.onChange(key);
      }}
      isInvalid={control.error !== undefined}
      errorMessage={control.error}
      items={options}
    >
      {(item) => (
        <SelectItem id={item.id} textValue={item.typeahead} description={item.id}>
          {item.label}
        </SelectItem>
      )}
    </Select>
  );
}

/** A group of checkboxes (a fieldset, as the APG checkbox pattern groups them); none checked shows every language. */
function TimelineLanguages({ control }: { control: PreferenceControl<string[]> }) {
  const t = useT();
  const { i18n } = useDisplaySettings();
  const descriptionId = useId();
  const errorId = useId();
  const chosen = new Set(control.value);
  const toggle = (tag: string, selected: boolean) => {
    const next = selected ? [...chosen, tag] : [...chosen].filter((c) => c !== tag);
    control.onChange(next.toSorted());
  };
  return (
    <fieldset
      aria-describedby={control.error === undefined ? descriptionId : `${descriptionId} ${errorId}`}
      className="@container flex min-w-0 flex-col gap-2"
    >
      <legend className="text-sm font-medium text-fg">{t("client:settings.timeline_languages.label")}</legend>
      <p id={descriptionId} className="text-sm text-fg-muted">
        {t("client:settings.timeline_languages.description")}
      </p>
      <ul className="grid grid-cols-1 gap-x-6 gap-y-1 @sm:grid-cols-2">
        {timelineLanguageOptions(control.value, i18n.locale).map((option) => (
          <li key={option.id}>
            <Checkbox
              isSelected={chosen.has(option.id)}
              onChange={(selected) => {
                toggle(option.id, selected);
              }}
            >
              {option.name}
            </Checkbox>
          </li>
        ))}
      </ul>
      {control.error !== undefined && (
        <p id={errorId} role="alert" className="text-sm text-danger">
          {control.error}
        </p>
      )}
    </fieldset>
  );
}
