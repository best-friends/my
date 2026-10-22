// SPDX-License-Identifier: AGPL-3.0-only
// The sign-up blocks screen below its heading (FR-719): the form of a new block, the import of a list of them
// (SignupBlockImport), then the blocks, in effect or all, newest first. The form follows the browser's constraint validation (React Aria's native validation); a
// block takes effect at once and is lifted as easily, so neither asks first. What adding or lifting did stays on
// the page in a StatusMessage (role="status") next to what caused it. The tabs follow the WAI-ARIA APG tabs
// pattern with automatic activation (React Aria Tabs), kept in the URL by the route (web-client.md S3); only the
// selected panel is mounted.
import {
  Box,
  Button,
  DescriptionItem,
  DescriptionList,
  DurationSelect,
  EmptyState,
  Heading,
  InlineError,
  Radio,
  RadioGroup,
  ReasonField,
  SectionHeader,
  Stack,
  StatusMessage,
  Tab,
  TabList,
  TabPanel,
  Tabs,
  Text,
  TextField,
} from "@my/ui";
import { useRef, useState } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval, useFocusHeadingAfterLift } from "../../../lib/useFocusAfterRemoval";
import {
  type SignupBlock,
  signupBlockCommentLimit,
  type SignupBlockFilter,
  signupBlockFilters,
  type SignupBlockKind,
  signupBlockKinds,
  signupBlockSeverities,
  type SignupBlockState,
  signupBlockValueLimit,
} from "../api/signupBlocks";
import {
  type LiftSignupBlock,
  signupBlockKindLabels,
  signupBlockSeverityLabels,
  signupBlockValueInText,
  useLiftSignupBlock,
  useSignupBlockForm,
  useSignupBlocks,
} from "../hooks/useSignupBlocks";
import { DateTime, DetailSection, DomainName, RecordList, RequirePermission } from "./parts";
import { SignupBlockImportForm } from "./SignupBlockImport";
import { day, signupBlockDurations, signupBlockKindDescriptions, signupBlockSeverityDescriptions } from "./signupBlockFields";

/** The label, help and error of the value field for each kind. */
const valueFields: Readonly<Record<SignupBlockKind, { label: ClientMessageKey; description: ClientMessageKey; required: ClientMessageKey }>> = {
  email_domain: {
    label: "client:admin.signup_blocks.value.email_domain",
    description: "client:admin.signup_blocks.value.email_domain.description",
    required: "client:admin.signup_blocks.value.email_domain.required",
  },
  email_hash: {
    label: "client:admin.signup_blocks.value.email_hash",
    description: "client:admin.signup_blocks.value.email_hash.description",
    required: "client:admin.signup_blocks.value.email_hash.required",
  },
  ip_range: {
    label: "client:admin.signup_blocks.value.ip_range",
    description: "client:admin.signup_blocks.value.ip_range.description",
    required: "client:admin.signup_blocks.value.ip_range.required",
  },
};

const stateLabels: Readonly<Record<SignupBlockState, ClientMessageKey>> = {
  active: "client:admin.signup_blocks.state.active",
  expired: "client:admin.signup_blocks.state.expired",
  lifted: "client:admin.signup_blocks.state.lifted",
};

const filterLabels: Readonly<Record<SignupBlockFilter, ClientMessageKey>> = {
  active: "client:admin.signup_blocks.filter.active",
  all: "client:admin.signup_blocks.filter.all",
};

function isFilter(key: unknown): key is SignupBlockFilter {
  return signupBlockFilters.some((filter) => filter === key);
}

export type SignupBlocksProps = {
  /** The selected filter, from the URL. */
  filter: SignupBlockFilter;
  /** Selects another filter: the route puts it into the URL. */
  onFilterChange: (filter: SignupBlockFilter) => void;
};

export function SignupBlocks(props: SignupBlocksProps) {
  const t = useT();
  return (
    <RequirePermission permission="manage_accounts">
      <Stack gap="8">
        <DetailSection title={t("client:admin.signup_blocks.new.title")}>
          <NewSignupBlockForm />
        </DetailSection>
        <DetailSection title={t("client:admin.signup_blocks.import.title")}>
          <SignupBlockImportForm />
        </DetailSection>
        <DetailSection title={t("client:admin.signup_blocks.list.title")}>
          <SignupBlockTabs {...props} />
        </DetailSection>
      </Stack>
    </RequirePermission>
  );
}

function NewSignupBlockForm() {
  const t = useT();
  const form = useSignupBlockForm();
  const { draft } = form;
  const valueField = valueFields[draft.kind];
  return (
    <Stack as="form" onSubmit={form.submit} gap="6">
      <RadioGroup
        label={t("client:admin.signup_blocks.kind")}
        value={draft.kind}
        onChange={(kind) => {
          const chosen = signupBlockKinds.find((item) => item === kind);
          if (chosen !== undefined) form.change({ kind: chosen });
        }}
        isRequired
      >
        {signupBlockKinds.map((kind) => (
          <Radio key={kind} value={kind} description={t(signupBlockKindDescriptions[kind])}>
            {t(signupBlockKindLabels[kind])}
          </Radio>
        ))}
      </RadioGroup>
      <TextField
        label={t(valueField.label)}
        description={t(valueField.description)}
        value={draft.value}
        onChange={(value) => {
          form.change({ value });
        }}
        isRequired
        maxLength={signupBlockValueLimit}
        autoComplete="off"
        spellCheck="false"
        errorMessage={t(valueField.required)}
      />
      <RadioGroup
        label={t("client:admin.signup_blocks.severity")}
        value={draft.severity}
        onChange={(severity) => {
          const chosen = signupBlockSeverities.find((item) => item === severity);
          if (chosen !== undefined) form.change({ severity: chosen });
        }}
        isRequired
        errorMessage={t("client:admin.signup_blocks.severity.required")}
      >
        {signupBlockSeverities.map((severity) => (
          <Radio key={severity} value={severity} description={t(signupBlockSeverityDescriptions[severity])}>
            {t(signupBlockSeverityLabels[severity])}
          </Radio>
        ))}
      </RadioGroup>
      <DurationSelect
        label={t("client:admin.signup_blocks.duration")}
        description={t("client:admin.signup_blocks.duration.description")}
        durations={signupBlockDurations}
        formatDuration={(seconds) => t("client:admin.signup_blocks.duration.days", { count: seconds / day })}
        indefiniteLabel={t("client:admin.signup_blocks.duration.indefinite")}
        value={draft.expiresIn}
        onChange={(expiresIn) => {
          form.change({ expiresIn });
        }}
      />
      <ReasonField
        label={t("client:admin.signup_blocks.comment")}
        description={t("client:admin.signup_blocks.comment.description")}
        value={draft.comment}
        onChange={(comment) => {
          form.change({ comment });
        }}
        characterLimit={signupBlockCommentLimit}
        formatRemaining={(remaining) =>
          remaining < 0 ? t("client:admin.signup_blocks.characters_over", { count: -remaining }) : t("client:admin.signup_blocks.characters_left", { count: remaining })
        }
        errorMessage={t("client:admin.signup_blocks.too_long", { max: signupBlockCommentLimit })}
      />
      {form.error !== undefined && (
        <InlineError alert size="inherit">
          {t("client:admin.signup_blocks.add_failed", { reason: form.error })}
        </InlineError>
      )}
      <Box>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:admin.signup_blocks.submit")}
        </Button>
      </Box>
      <StatusMessage>{form.status}</StatusMessage>
    </Stack>
  );
}

function SignupBlockTabs({ filter, onFilterChange }: SignupBlocksProps) {
  const t = useT();
  const [status, setStatus] = useState<string | undefined>(undefined);
  return (
    <Stack gap="3">
      <StatusMessage>{status}</StatusMessage>
      <Tabs
        selectedKey={filter}
        onSelectionChange={(key) => {
          if (isFilter(key)) onFilterChange(key);
        }}
      >
        <TabList aria-label={t("client:admin.signup_blocks.filter.label")}>
          {signupBlockFilters.map((item) => (
            <Tab key={item} id={item}>
              {t(filterLabels[item])}
            </Tab>
          ))}
        </TabList>
        <TabPanel id={filter}>
          <SignupBlockList
            filter={filter}
            onLifted={(block) => {
              setStatus(t("client:admin.signup_blocks.lifted", { kind: t(signupBlockKindLabels[block.kind]), value: signupBlockValueInText(t, block) }));
            }}
          />
        </TabPanel>
      </Tabs>
    </Stack>
  );
}

function SignupBlockList({ filter, onLifted }: { filter: SignupBlockFilter; onLifted: (block: SignupBlock) => void }) {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const list = useSignupBlocks(filter);
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((block) => block.id),
  );
  const keepFocus = useFocusHeadingAfterLift(listRef);
  const lift = useLiftSignupBlock((block) => {
    // The lift button goes away: with the row from the list of blocks in effect, or from the lifted row.
    if (filter === "active") removed(block.id);
    else keepFocus(block.id);
    onLifted(block);
  });
  const empty =
    filter === "active" ? (
      <EmptyState headingLevel={3} heading={t("client:admin.signup_blocks.empty.active")} description={t("client:admin.signup_blocks.empty.active.description")} />
    ) : (
      <EmptyState headingLevel={3} heading={t("client:admin.signup_blocks.empty.all")} />
    );
  return (
    <RecordList list={list} getId={(block) => block.id} listRef={listRef} empty={empty}>
      {(block) => <SignupBlockRow block={block} lift={lift} />}
    </RecordList>
  );
}

/** What a block matches: a domain in both of its forms, else the value as the server keeps it. */
function SignupBlockValue({ block }: { block: SignupBlock }) {
  if (block.kind === "email_domain") return <DomainName domain={block.value} />;
  return <span dir="ltr">{block.value}</span>;
}

function SignupBlockRow({ block, lift }: { block: SignupBlock; lift: LiftSignupBlock }) {
  const t = useT();
  const intl = useIntl();
  const kind = t(signupBlockKindLabels[block.kind]);
  return (
    <Stack gap="3">
      <SectionHeader
        actions={
          block.state === "active" && (
          <Button
            size="sm"
            isPending={lift.pendingIds.includes(block.id)}
            aria-label={t("client:admin.signup_blocks.lift_action", {
              kind,
              value: signupBlockValueInText(t, block),
              date: intl.formatDate(block.created_at, { dateStyle: "medium", timeStyle: "short" }),
            })}
            onPress={() => {
              lift.lift(block);
            }}
          >
            {t("client:admin.signup_blocks.lift")}
          </Button>
          )
        }
      >
        <Heading level={3} wrap="anywhere" focusable>
          <SignupBlockValue block={block} />
        </Heading>
      </SectionHeader>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.signup_blocks.kind")}>{kind}</DescriptionItem>
        <DescriptionItem term={t("client:admin.signup_blocks.severity")}>{t(signupBlockSeverityLabels[block.severity])}</DescriptionItem>
        <DescriptionItem term={t("client:admin.signup_blocks.state")}>
          <Text {...(block.state === "active" ? { weight: "semibold" as const } : {})}>{t(stateLabels[block.state])}</Text>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.signup_blocks.created_at")}>
          <DateTime value={block.created_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.signup_blocks.expires_at")}>
          {block.expires_at === undefined ? t("client:admin.signup_blocks.duration.indefinite") : <DateTime value={block.expires_at} />}
        </DescriptionItem>
        {block.lifted_at !== undefined && (
          <DescriptionItem term={t("client:admin.signup_blocks.lifted_at")}>
            <DateTime value={block.lifted_at} />
          </DescriptionItem>
        )}
        <DescriptionItem term={t("client:admin.signup_blocks.source")}>
          {block.moderation_action_id === undefined ? t("client:admin.signup_blocks.source.manual") : t("client:admin.signup_blocks.source.ban")}
        </DescriptionItem>
      </DescriptionList>
      {block.comment !== undefined && (
        <DescriptionList>
          <DescriptionItem term={t("client:admin.signup_blocks.comment")}>
            {/* Written by a moderator, in any language. */}
            <Text dir="auto" whitespace="preWrap">
              {block.comment}
            </Text>
          </DescriptionItem>
        </DescriptionList>
      )}
    </Stack>
  );
}
