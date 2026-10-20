// SPDX-License-Identifier: AGPL-3.0-only
// The import and export page below its heading (FR-008): the CSV files of the follows, blocked and muted accounts, and
// bookmarks to download; a form that imports such a file after checking what it would do; and the latest imports with
// their progress. Dates are in the user's locale and time zone, numbers in the user's locale.
import {
  Button,
  DescriptionItem,
  DescriptionList,
  EmptyState,
  FileTrigger,
  Radio,
  RadioGroup,
  StatusMessage,
} from "@my/ui";
import { type ReactNode, useId } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import {
  csvFileNames,
  type DataImport,
  type DataImportKind,
  type DataImportMode,
  type DataImportPreview,
  dataImportKinds,
  type DataImportSkipped,
} from "../api/importExport";
import { type CsvExports, type ImportForm, useCsvExports, useDataImports, useImportForm } from "../hooks/useImportExport";

const kindLabels: Readonly<Record<DataImportKind, ClientMessageKey>> = {
  following: "client:settings.import_export.kind.following",
  blocks: "client:settings.import_export.kind.blocks",
  mutes: "client:settings.import_export.kind.mutes",
  bookmarks: "client:settings.import_export.kind.bookmarks",
};

const modeLabels: Readonly<Record<DataImportMode, ClientMessageKey>> = {
  merge: "client:settings.import_export.import.mode.merge",
  overwrite: "client:settings.import_export.import.mode.overwrite",
};

const skipReasons: Readonly<Record<DataImportSkipped["reason"], ClientMessageKey>> = {
  invalid: "client:settings.import_export.import.preview.skip.invalid",
  duplicate: "client:settings.import_export.import.preview.skip.duplicate",
};

const stateLabels: Readonly<Record<DataImport["state"], ClientMessageKey>> = {
  in_progress: "client:settings.import_export.history.state.in_progress",
  finished: "client:settings.import_export.history.state.finished",
};

/** The format of every date and time on the page. */
const dateTimeFormat = { dateStyle: "medium", timeStyle: "short" } as const;

/** The skipped lines a preview lists; the rest are only counted by the server's answer. */
const maxSkippedShown = 20;

/** The file types the file picker offers. */
const csvFileTypes = [".csv", "text/csv"];

export function ImportExport() {
  const t = useT();
  const exports = useCsvExports();
  const form = useImportForm();
  const imports = useDataImports();
  return (
    <div className="flex flex-col gap-8">
      <Section title={t("client:settings.import_export.export.title")}>
        <ExportsView exports={exports} />
      </Section>
      <Section title={t("client:settings.import_export.import.title")}>
        <ImportView form={form} />
      </Section>
      <Section title={t("client:settings.import_export.history.title")}>
        <HistoryView imports={imports} />
      </Section>
    </div>
  );
}

/** A titled region of the page below its `h1`: an `h2` that names a `<section>`. */
function Section({ title, children }: { title: ReactNode; children: ReactNode }) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}

function ExportsView({ exports }: { exports: CsvExports }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-4">
      <p>{t("client:settings.import_export.export.description")}</p>
      {exports.error !== undefined && (
        <p role="alert" className="text-danger">
          {exports.error}
        </p>
      )}
      <ul className="flex flex-col divide-y divide-border">
        {dataImportKinds.map((kind) => (
          <li key={kind} className="flex flex-wrap items-center justify-between gap-2 py-3">
            <span className="flex min-w-0 flex-col">
              <span>{t(kindLabels[kind])}</span>
              <span className="text-sm wrap-anywhere text-fg-muted">{csvFileNames[kind]}</span>
            </span>
            <Button
              size="sm"
              aria-label={t("client:settings.import_export.export.download_of", { kind: t(kindLabels[kind]) })}
              isPending={exports.downloadingKind === kind}
              onPress={() => {
                exports.download(kind);
              }}
            >
              {t("client:settings.import_export.export.download")}
            </Button>
          </li>
        ))}
      </ul>
    </div>
  );
}

function ImportView({ form }: { form: ImportForm }) {
  const t = useT();
  const fileHeadingId = useId();
  const fileStateId = useId();
  return (
    <div className="flex flex-col gap-4">
      <p>{t("client:settings.import_export.import.description")}</p>
      <RadioGroup
        label={t("client:settings.import_export.import.kind")}
        value={form.kind}
        onChange={(value) => {
          if (isKind(value)) form.changeKind(value);
        }}
      >
        {dataImportKinds.map((kind) => (
          <Radio key={kind} value={kind}>
            {t(kindLabels[kind])}
          </Radio>
        ))}
      </RadioGroup>
      <RadioGroup
        label={t("client:settings.import_export.import.mode")}
        value={form.mode}
        onChange={(value) => {
          if (value === "merge" || value === "overwrite") form.changeMode(value);
        }}
      >
        <Radio value="merge" description={t("client:settings.import_export.import.mode.merge.description")}>
          {t(modeLabels.merge)}
        </Radio>
        <Radio value="overwrite" description={t("client:settings.import_export.import.mode.overwrite.description")}>
          {t(modeLabels.overwrite)}
        </Radio>
      </RadioGroup>
      <div className="flex flex-col gap-2">
        <h3 id={fileHeadingId} className="text-sm font-medium">
          {t("client:settings.import_export.import.file")}
        </h3>
        <div className="flex flex-wrap items-center gap-2">
          <FileTrigger
            acceptedFileTypes={csvFileTypes}
            onSelect={(files) => {
              const file = files?.[0];
              if (file !== undefined) form.chooseFile(file);
            }}
          >
            <Button size="sm" aria-describedby={`${fileHeadingId} ${fileStateId}`}>
              {t("client:settings.import_export.import.choose")}
            </Button>
          </FileTrigger>
          <span id={fileStateId} className="min-w-0 text-sm wrap-anywhere text-fg-muted">
            {form.fileName === undefined
              ? t("client:settings.import_export.import.no_file")
              : t("client:settings.import_export.import.chosen", { name: form.fileName })}
          </span>
        </div>
        {form.fileError !== undefined && (
          <p role="alert" className="text-sm text-danger">
            {form.fileError}
          </p>
        )}
      </div>
      <div>
        <Button onPress={form.preview} isDisabled={!form.canPreview} isPending={form.isPreviewing}>
          {t("client:settings.import_export.import.preview")}
        </Button>
      </div>
      {form.result !== undefined && <PreviewView result={form.result} form={form} />}
      {form.error !== undefined && (
        <p role="alert" className="text-danger">
          {form.error}
        </p>
      )}
      <StatusMessage tone="success">{form.status}</StatusMessage>
    </div>
  );
}

function isKind(value: string): value is DataImportKind {
  return (dataImportKinds as readonly string[]).includes(value);
}

function PreviewView({ result, form }: { result: DataImportPreview; form: ImportForm }) {
  const t = useT();
  const intl = useIntl();
  const headingId = useId();
  const skippedId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3 rounded-md border border-border px-4 py-3">
      <h3 id={headingId} className="font-semibold">
        {t("client:settings.import_export.import.preview.title")}
      </h3>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:settings.import_export.import.preview.entries")}>
          <span className="tabular-nums">{intl.formatNumber(result.entries)}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:settings.import_export.import.preview.present")}>
          <span className="tabular-nums">{intl.formatNumber(result.present)}</span>
        </DescriptionItem>
        {form.mode === "overwrite" && (
          <DescriptionItem term={t("client:settings.import_export.import.preview.removals")}>
            <span className="tabular-nums">{intl.formatNumber(result.removals)}</span>
          </DescriptionItem>
        )}
      </DescriptionList>
      {result.skipped.length > 0 && (
        <div className="flex flex-col gap-1">
          <h4 id={skippedId} className="text-sm font-medium">
            {t("client:settings.import_export.import.preview.skipped")}
          </h4>
          <ul aria-labelledby={skippedId} className="flex flex-col gap-1 text-sm">
            {result.skipped.slice(0, maxSkippedShown).map((line) => (
              <li key={line.line} className="flex flex-col">
                <span dir="auto" className="wrap-anywhere">
                  {t("client:settings.import_export.import.preview.skipped_line", { line: line.line, value: line.value })}
                </span>
                <span className="text-fg-muted">{t(skipReasons[line.reason])}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
      <div>
        <Button variant={form.mode === "overwrite" ? "danger" : "primary"} onPress={form.start} isPending={form.isStarting}>
          {t("client:settings.import_export.import.start")}
        </Button>
      </div>
    </section>
  );
}

function HistoryView({ imports }: { imports: DataImport[] }) {
  const t = useT();
  if (imports.length === 0) {
    return <EmptyState headingLevel={3} heading={t("client:settings.import_export.history.empty")} />;
  }
  return (
    <ul className="flex flex-col divide-y divide-border">
      {imports.map((item) => (
        <li key={item.id} className="@container py-3">
          <ImportRow item={item} />
        </li>
      ))}
    </ul>
  );
}

function ImportRow({ item }: { item: DataImport }) {
  const t = useT();
  const intl = useIntl();
  const headingId = `import-${item.id}`;
  return (
    <article aria-labelledby={headingId} className="flex flex-col gap-3">
      <h3 id={headingId} className="font-semibold">
        {t("client:settings.import_export.history.item", {
          kind: t(kindLabels[item.kind]),
          date: intl.formatDate(item.created_at, dateTimeFormat),
        })}
      </h3>
      <DescriptionList columns={4}>
        <DescriptionItem term={t("client:settings.import_export.history.state")}>{t(stateLabels[item.state])}</DescriptionItem>
        <DescriptionItem term={t("client:settings.import_export.history.mode")}>{t(modeLabels[item.mode])}</DescriptionItem>
        <DescriptionItem term={t("client:settings.import_export.history.progress")}>
          <span className="tabular-nums">
            {t("client:settings.import_export.history.progress_value", { processed: item.processed, total: item.total })}
          </span>
        </DescriptionItem>
        <DescriptionItem term={t("client:settings.import_export.history.failed")}>
          <span className="tabular-nums">{intl.formatNumber(item.failed)}</span>
        </DescriptionItem>
      </DescriptionList>
    </article>
  );
}
