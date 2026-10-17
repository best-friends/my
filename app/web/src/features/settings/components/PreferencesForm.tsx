// SPDX-License-Identifier: AGPL-3.0-only
import { Checkbox, Radio, RadioGroup, Select, SelectItem } from "@my/ui";
import { useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useDisplaySettings } from "../../../lib/locale";
import type { Visibility } from "../api/me";
import { displayLanguageOf, displayLanguageOptions, timelineLanguageOptions } from "../hooks/languages";
import { useTimeZoneOptions } from "../hooks/timeZones";
import { type PreferenceControl, usePreferences } from "../hooks/usePreferences";

/** The visibilities in the order of ADR-0017, widest first, with their label and description. */
const visibilities: readonly { id: Visibility; label: ClientMessageKey; description: ClientMessageKey }[] = [
  { id: "public", label: "client:settings.visibility.public", description: "client:settings.visibility.public.description" },
  { id: "local", label: "client:settings.visibility.local", description: "client:settings.visibility.local.description" },
  { id: "unlisted", label: "client:settings.visibility.unlisted", description: "client:settings.visibility.unlisted.description" },
  { id: "private", label: "client:settings.visibility.private", description: "client:settings.visibility.private.description" },
  { id: "direct", label: "client:settings.visibility.direct", description: "client:settings.visibility.direct.description" },
];

/** The preferences: every control saves its value as soon as it changes. */
export function PreferencesForm() {
  const t = useT();
  const { locale, timeZone, defaultVisibility, timelineLanguages } = usePreferences();
  return (
    <div className="flex flex-col gap-8">
      <p className="text-fg-muted">{t("client:settings.preferences.autosave")}</p>
      <DisplayLanguageSelect control={locale} />
      <TimeZoneSelect control={timeZone} />
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
      <TimelineLanguages control={timelineLanguages} />
    </div>
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
 * The time zone: each option is the zone's name in the display language with its difference from UTC, and is
 * described by its IANA name, which tells apart the zones of one name (America/New_York, America/Detroit).
 */
function TimeZoneSelect({ control }: { control: PreferenceControl<string> }) {
  const t = useT();
  const options = useTimeZoneOptions(control.value);
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
