// SPDX-License-Identifier: AGPL-3.0-only
// The server settings screen below its heading (manage_settings, FR-705): the instance's name and descriptions, how
// to reach its operators and its thumbnail (#663), then its policies (who may sign up, the default language, the
// public timelines, whether images need a description, the federation, who may read the list of the domain
// actions, how much of the costs is published and shown to the holders of view_finance) in one form saved together. The name, the descriptions and the thumbnail's alt text hold one value each,
// written in any language and shown as it is in every language; the labels are the app's. The name stands in for
// the domain where the instance is named to people; the domain stays in handles and links. The browser keeps a
// form from being sent while a field is invalid (React Aria's native validation: the error is under its field and
// linked to it, and focus moves to the first invalid field). What a save did stays on the page in a StatusMessage
// next to its button; a failure stays above the button (web-client.md E2).
import { Button, Checkbox, FileTrigger, Radio, RadioGroup, Select, SelectItem, StatusMessage, TextArea, TextField } from "@my/ui";
import { useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { DetailSection, RequirePermission } from "../../moderation";
import { acceptedImageTypes } from "../../settings";
import { thumbnailAltLimit } from "../api/settings";
import { useServerContact } from "../hooks/useServerContact";
import { type DescriptionField, descriptionLimits, useServerDescriptions } from "../hooks/useServerDescriptions";
import { type PolicyKey, useServerPolicies } from "../hooks/useServerPolicies";
import { useServerSettings } from "../hooks/useServerSettings";

export function ServerSettings() {
  return (
    <RequirePermission permission="manage_settings">
      <div className="flex flex-col gap-8">
        <NameForm />
        <DescriptionsForm />
        <ContactForm />
        <PoliciesForm />
      </div>
    </RequirePermission>
  );
}

function NameForm() {
  const t = useT();
  const form = useServerSettings();
  return (
    <DetailSection title={t("client:admin.settings.name.title")}>
      <p className="text-fg-muted">{t("client:admin.settings.name.description", { domain: form.domain })}</p>
      <form onSubmit={form.submit} className="flex flex-col gap-4">
        <TextField
          label={t("client:admin.settings.name.field")}
          value={form.field.value}
          onChange={form.field.onChange}
          // Only an invalid name sets isInvalid: React Aria ignores `validate` once isInvalid is given at all.
          {...(form.field.error === undefined ? {} : { isInvalid: true, errorMessage: form.field.error })}
        />
        {form.saveError !== undefined && (
          <p role="alert" className="text-danger">
            {t("client:admin.settings.name.save_failed", { reason: form.saveError })}
          </p>
        )}
        <div>
          <Button type="submit" variant="primary" isPending={form.isSaving}>
            {t("client:admin.settings.name.save")}
          </Button>
        </div>
        <StatusMessage>{form.status}</StatusMessage>
      </form>
    </DetailSection>
  );
}

function DescriptionInput({ field, label }: { field: DescriptionField; label: string }) {
  const t = useT();
  const limit = descriptionLimits[field.key];
  return (
    <TextArea
      label={label}
      value={field.value}
      onChange={field.onChange}
      rows={field.key === "description" ? 6 : 3}
      characterLimit={limit}
      formatRemaining={(remaining) =>
        remaining < 0 ? t("client:admin.settings.characters_over", { count: -remaining }) : t("client:admin.settings.characters_left", { count: remaining })
      }
      errorMessage={t("client:admin.settings.name.too_long", { max: String(limit) })}
    />
  );
}

function DescriptionsForm() {
  const t = useT();
  const form = useServerDescriptions();
  return (
    <DetailSection title={t("client:admin.settings.descriptions.title")}>
      <p className="text-fg-muted">{t("client:admin.settings.descriptions.description")}</p>
      <form onSubmit={form.submit} className="flex flex-col gap-6">
        <DescriptionInput field={form.short} label={t("client:admin.settings.descriptions.short")} />
        <DescriptionInput field={form.full} label={t("client:admin.settings.descriptions.full")} />
        {form.saveError !== undefined && (
          <p role="alert" className="text-danger">
            {t("client:admin.settings.descriptions.save_failed", { reason: form.saveError })}
          </p>
        )}
        <div>
          <Button type="submit" variant="primary" isPending={form.isSaving}>
            {t("client:admin.settings.descriptions.save")}
          </Button>
        </div>
        <StatusMessage>{form.status}</StatusMessage>
      </form>
    </DetailSection>
  );
}

function ContactForm() {
  const t = useT();
  const form = useServerContact();
  const thumbnailHeadingId = useId();
  const uploadErrorId = useId();
  return (
    <DetailSection title={t("client:admin.settings.contact.title")}>
      <p className="text-fg-muted">{t("client:admin.settings.contact.description")}</p>
      <form onSubmit={form.submit} className="flex flex-col gap-6">
        <TextField
          label={t("client:admin.settings.contact.account")}
          description={t("client:admin.settings.contact.account.description")}
          value={form.account.value}
          onChange={form.account.onChange}
          autoComplete="off"
        />
        <TextField
          type="email"
          label={t("client:admin.settings.contact.email")}
          description={t("client:admin.settings.contact.email.description")}
          value={form.email.value}
          onChange={form.email.onChange}
          autoComplete="off"
          errorMessage={t("client:admin.settings.contact.email.invalid")}
        />
        <section aria-labelledby={thumbnailHeadingId} aria-busy={form.thumbnail.isUploading} className="flex flex-col gap-3">
          <h3 id={thumbnailHeadingId} className="font-medium">
            {t("client:admin.settings.contact.thumbnail")}
          </h3>
          <p className="text-sm text-fg-muted">{t("client:admin.settings.contact.thumbnail.description")}</p>
          {form.thumbnail.url !== undefined && (
            // A preview of the image chosen in the form; the alt text below describes it.
            <img src={form.thumbnail.url} alt="" className="aspect-video w-full max-w-md rounded-md bg-surface-sunken object-cover" />
          )}
          <div className="flex flex-wrap gap-2">
            <FileTrigger
              acceptedFileTypes={acceptedImageTypes}
              onSelect={(files) => {
                const file = files?.[0];
                if (file !== undefined) form.thumbnail.choose(file);
              }}
            >
              <Button
                size="sm"
                isPending={form.thumbnail.isUploading}
                aria-describedby={form.thumbnail.error === undefined ? thumbnailHeadingId : `${thumbnailHeadingId} ${uploadErrorId}`}
              >
                {t("client:admin.settings.contact.thumbnail.choose")}
              </Button>
            </FileTrigger>
            {form.thumbnail.url !== undefined && (
              <Button size="sm" variant="ghost" onPress={form.thumbnail.remove} isDisabled={form.thumbnail.isUploading} aria-describedby={thumbnailHeadingId}>
                {t("client:admin.settings.contact.thumbnail.remove")}
              </Button>
            )}
          </div>
          {form.thumbnail.error !== undefined && (
            <p id={uploadErrorId} role="alert" className="text-sm text-danger">
              {form.thumbnail.error}
            </p>
          )}
          <TextArea
            label={t("client:admin.settings.contact.thumbnail.alt")}
            value={form.alt.value}
            onChange={form.alt.onChange}
            rows={2}
            characterLimit={thumbnailAltLimit}
            formatRemaining={(remaining) =>
              remaining < 0 ? t("client:admin.settings.characters_over", { count: -remaining }) : t("client:admin.settings.characters_left", { count: remaining })
            }
            errorMessage={t("client:admin.settings.name.too_long", { max: String(thumbnailAltLimit) })}
          />
        </section>
        {form.saveError !== undefined && (
          <p role="alert" className="text-danger">
            {t("client:admin.settings.contact.save_failed", { reason: form.saveError })}
          </p>
        )}
        <div>
          <Button type="submit" variant="primary" isPending={form.isSaving}>
            {t("client:admin.settings.contact.save")}
          </Button>
        </div>
        <StatusMessage>{form.status}</StatusMessage>
      </form>
    </DetailSection>
  );
}

/** The words and the description of each choice of the settings chosen with radio buttons. */
const choices = {
  registration_mode: [
    { value: "open", label: "client:admin.settings.registration.open", description: "client:admin.settings.registration.open.description" },
    { value: "approval", label: "client:admin.settings.registration.approval", description: "client:admin.settings.registration.approval.description" },
    { value: "invite", label: "client:admin.settings.registration.invite", description: "client:admin.settings.registration.invite.description" },
    { value: "closed", label: "client:admin.settings.registration.closed", description: "client:admin.settings.registration.closed.description" },
  ],
  remote_media_policy: [
    { value: "none", label: "client:admin.settings.remote_media.none", description: "client:admin.settings.remote_media.none.description" },
    { value: "avatars", label: "client:admin.settings.remote_media.avatars", description: "client:admin.settings.remote_media.avatars.description" },
    { value: "all", label: "client:admin.settings.remote_media.all", description: "client:admin.settings.remote_media.all.description" },
  ],
  domain_actions_visibility: [
    { value: "none", label: "client:admin.settings.audience.none", description: "client:admin.settings.domain_actions.list.none" },
    { value: "users", label: "client:admin.settings.audience.users", description: "client:admin.settings.domain_actions.list.users" },
    { value: "all", label: "client:admin.settings.audience.all", description: "client:admin.settings.domain_actions.list.all" },
  ],
  domain_actions_rationale_visibility: [
    { value: "none", label: "client:admin.settings.audience.none", description: "client:admin.settings.domain_actions.comments.none" },
    { value: "users", label: "client:admin.settings.audience.users", description: "client:admin.settings.domain_actions.comments.users" },
    { value: "all", label: "client:admin.settings.audience.all", description: "client:admin.settings.domain_actions.comments.all" },
  ],
  finance_visibility: [
    { value: "private", label: "client:admin.settings.finance.private", description: "client:admin.settings.finance.private.description" },
    { value: "totals", label: "client:admin.settings.finance.totals", description: "client:admin.settings.finance.totals.description" },
    { value: "breakdown", label: "client:admin.settings.finance.breakdown", description: "client:admin.settings.finance.breakdown.description" },
  ],
  finance_viewer_income: [
    { value: "totals", label: "client:admin.settings.finance.viewer.income.totals", description: "client:admin.settings.finance.viewer.income.totals.description" },
    { value: "plans", label: "client:admin.settings.finance.viewer.income.plans", description: "client:admin.settings.finance.viewer.income.plans.description" },
  ],
  supporter_list_visibility: [
    { value: "off", label: "client:admin.settings.supporter_list.off", description: "client:admin.settings.supporter_list.off.description" },
    { value: "public", label: "client:admin.settings.supporter_list.public", description: "client:admin.settings.supporter_list.public.description" },
    { value: "signed_in", label: "client:admin.settings.supporter_list.signed_in", description: "client:admin.settings.supporter_list.signed_in.description" },
  ],
  search_indexing: [
    { value: "allow", label: "client:admin.settings.crawlers.search.allow", description: "client:admin.settings.crawlers.search.allow.description" },
    { value: "deny", label: "client:admin.settings.crawlers.search.deny", description: "client:admin.settings.crawlers.search.deny.description" },
  ],
  ai_usage: [
    { value: "allow", label: "client:admin.settings.crawlers.ai.allow", description: "client:admin.settings.crawlers.ai.allow.description" },
    { value: "deny_training", label: "client:admin.settings.crawlers.ai.deny_training", description: "client:admin.settings.crawlers.ai.deny_training.description" },
    { value: "deny", label: "client:admin.settings.crawlers.ai.deny", description: "client:admin.settings.crawlers.ai.deny.description" },
  ],
} as const satisfies Partial<Record<PolicyKey, readonly { value: string; label: ClientMessageKey; description: ClientMessageKey }[]>>;

type RadioKey = keyof typeof choices;

function PolicyRadios({ form, setting, label, description }: { form: ReturnType<typeof useServerPolicies>; setting: RadioKey; label: string; description?: string }) {
  const t = useT();
  return (
    <RadioGroup
      label={label}
      {...(description === undefined ? {} : { description })}
      value={form.value(setting)}
      onChange={(value) => {
        form.change(setting, value);
      }}
    >
      {choices[setting].map((choice) => (
        <Radio key={choice.value} value={choice.value} description={t(choice.description)}>
          {t(choice.label)}
        </Radio>
      ))}
    </RadioGroup>
  );
}

function PolicyCheckbox({ form, setting, label, description }: { form: ReturnType<typeof useServerPolicies>; setting: PolicyKey; label: string; description: string }) {
  return (
    <Checkbox
      isSelected={form.value(setting) === "true"}
      onChange={(selected) => {
        form.change(setting, selected ? "true" : "false");
      }}
      description={description}
    >
      {label}
    </Checkbox>
  );
}

function PoliciesForm() {
  const t = useT();
  const form = useServerPolicies();
  return (
    <form onSubmit={form.submit} className="flex flex-col gap-8">
      <DetailSection title={t("client:admin.settings.registration.title")}>
        <PolicyRadios form={form} setting="registration_mode" label={t("client:admin.settings.registration.mode")} description={t("client:admin.settings.registration.description")} />
      </DetailSection>
      <DetailSection title={t("client:admin.settings.language.title")}>
        <Select
          label={t("client:admin.settings.language.default")}
          description={t("client:admin.settings.language.description")}
          value={form.value("default_locale")}
          onChange={(key) => {
            if (typeof key === "string") form.change("default_locale", key);
          }}
          items={form.localeChoices.map((choice) => ({ id: choice.value, name: choice.label }))}
        >
          {(choice) => (
            <SelectItem id={choice.id} textValue={choice.name}>
              {choice.name}
            </SelectItem>
          )}
        </Select>
      </DetailSection>
      <DetailSection title={t("client:admin.settings.timelines.title")}>
        <PolicyCheckbox
          form={form}
          setting="local_timeline_enabled"
          label={t("client:admin.settings.timelines.local")}
          description={t("client:admin.settings.timelines.local.description")}
        />
        <PolicyCheckbox
          form={form}
          setting="federated_timeline_enabled"
          label={t("client:admin.settings.timelines.federated")}
          description={t("client:admin.settings.timelines.federated.description")}
        />
      </DetailSection>
      <DetailSection title={t("client:admin.settings.posts.title")}>
        <PolicyCheckbox
          form={form}
          setting="require_media_description"
          label={t("client:admin.settings.posts.require_media_description")}
          description={t("client:admin.settings.posts.require_media_description.description")}
        />
      </DetailSection>
      <DetailSection title={t("client:admin.settings.federation.title")}>
        <PolicyCheckbox
          form={form}
          setting="authorized_fetch"
          label={t("client:admin.settings.federation.authorized_fetch")}
          description={t("client:admin.settings.federation.authorized_fetch.description")}
        />
        <PolicyRadios form={form} setting="remote_media_policy" label={t("client:admin.settings.remote_media.policy")} />
        <Select
          label={t("client:admin.settings.remote_media.ttl")}
          description={t("client:admin.settings.remote_media.ttl.description")}
          value={form.value("remote_media_cache_ttl")}
          onChange={(key) => {
            if (typeof key === "string") form.change("remote_media_cache_ttl", key);
          }}
          items={form.cacheTtlChoices.map((choice) => ({ id: choice.value, name: choice.label }))}
        >
          {(choice) => (
            <SelectItem id={choice.id} textValue={choice.name}>
              {choice.name}
            </SelectItem>
          )}
        </Select>
        <PolicyCheckbox
          form={form}
          setting="link_cards_enabled"
          label={t("client:admin.settings.federation.link_cards")}
          description={t("client:admin.settings.federation.link_cards.description")}
        />
      </DetailSection>
      <DetailSection title={t("client:admin.settings.domain_actions.title")}>
        <p className="text-fg-muted">{t("client:admin.settings.domain_actions.description")}</p>
        <PolicyRadios form={form} setting="domain_actions_visibility" label={t("client:admin.settings.domain_actions.list")} />
        <PolicyRadios form={form} setting="domain_actions_rationale_visibility" label={t("client:admin.settings.domain_actions.comments")} />
      </DetailSection>
      <DetailSection title={t("client:admin.settings.finance.title")}>
        <p className="text-fg-muted">{t("client:admin.settings.finance.description")}</p>
        <PolicyRadios form={form} setting="finance_visibility" label={t("client:admin.settings.finance.visibility")} />
        <p className="text-fg-muted">{t("client:admin.settings.finance.viewer.description")}</p>
        <PolicyCheckbox
          form={form}
          setting="finance_viewer_item_names"
          label={t("client:admin.settings.finance.viewer.item_names")}
          description={t("client:admin.settings.finance.viewer.item_names.description")}
        />
        <PolicyCheckbox
          form={form}
          setting="finance_viewer_item_memos"
          label={t("client:admin.settings.finance.viewer.item_memos")}
          description={t("client:admin.settings.finance.viewer.item_memos.description")}
        />
        <PolicyRadios form={form} setting="finance_viewer_income" label={t("client:admin.settings.finance.viewer.income")} />
      </DetailSection>
      <DetailSection title={t("client:admin.settings.supporter_list.title")}>
        <p className="text-fg-muted">{t("client:admin.settings.supporter_list.description")}</p>
        <PolicyRadios form={form} setting="supporter_list_visibility" label={t("client:admin.settings.supporter_list.visibility")} />
      </DetailSection>
      <DetailSection title={t("client:admin.settings.crawlers.title")}>
        <p className="text-fg-muted">{t("client:admin.settings.crawlers.description")}</p>
        <PolicyRadios form={form} setting="search_indexing" label={t("client:admin.settings.crawlers.search")} />
        <PolicyCheckbox
          form={form}
          setting="indexable_default"
          label={t("client:admin.settings.crawlers.indexable_default")}
          description={t("client:admin.settings.crawlers.indexable_default.description")}
        />
        <PolicyRadios form={form} setting="ai_usage" label={t("client:admin.settings.crawlers.ai")} />
        <TextField
          label={t("client:admin.settings.crawlers.security_contact")}
          description={t("client:admin.settings.crawlers.security_contact.description")}
          value={form.value("security_contact")}
          onChange={(value) => {
            form.change("security_contact", value);
          }}
        />
      </DetailSection>
      {form.saveError !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.settings.policies.save_failed", { reason: form.saveError })}
        </p>
      )}
      <div className="flex flex-col gap-3">
        <div>
          <Button type="submit" variant="primary" isPending={form.isSaving}>
            {t("client:admin.settings.policies.save")}
          </Button>
        </div>
        <StatusMessage>{form.status}</StatusMessage>
      </div>
    </form>
  );
}
