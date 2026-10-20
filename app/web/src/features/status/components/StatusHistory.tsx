// SPDX-License-Identifier: AGPL-3.0-only
import { ContentWarning, EmojiText, Link, MediaGrid, PostContent } from "@my/ui";
import { useId } from "react";
import { type PlainClientMessageKey, useLocale, useT } from "../../../lib/i18n";
import { type RevisionChange, type RevisionView, type StatusHistoryView, useStatusHistory } from "../hooks/useStatusHistory";
import { useMediaViewerLabels } from "../hooks/useMediaViewerLabels";

export type StatusHistoryProps = {
  /** The ID of the post; the route loader has fetched it and its history. */
  id: string;
};

const kindLabels: Readonly<Record<RevisionView["kind"], PlainClientMessageKey | null>> = {
  current: "client:status.history.current",
  original: "client:status.history.original",
  edit: null,
};

const changeLabels: Readonly<Record<RevisionChange, PlainClientMessageKey>> = {
  text: "client:status.history.change.text",
  warning: "client:status.history.change.warning",
  sensitive: "client:status.history.change.sensitive",
  media: "client:status.history.change.media",
};

/**
 * The revisions of a post (FR-109), newest first, under a link back to the post. Each revision is an
 * article named by its heading, the date and time it was made, and says whether it is the current content
 * or the post as first written, and what it changed from the revision before it.
 */
export function StatusHistory({ id }: StatusHistoryProps) {
  const t = useT();
  const history = useStatusHistory(id);
  return (
    <div className="flex flex-col">
      <p className="px-4 pb-2">
        <Link href={history.postPath}>{t("client:status.history.back")}</Link>
      </p>
      <ol aria-label={t("client:status.history.list")} className="divide-y divide-border border-y border-border">
        {history.revisions.map((revision) => (
          <li key={revision.number}>
            <Revision revision={revision} history={history} />
          </li>
        ))}
      </ol>
    </div>
  );
}

type RevisionProps = { revision: RevisionView; history: StatusHistoryView };

function Revision({ revision, history }: RevisionProps) {
  const t = useT();
  const locale = useLocale();
  const headingId = useId();
  const viewerLabels = useMediaViewerLabels();
  const kind = kindLabels[revision.kind];
  const changes =
    revision.changes.length === 0
      ? null
      : t("client:status.history.changes", {
          changes: new Intl.ListFormat(locale, { type: "conjunction" }).format(revision.changes.map((change) => t(changeLabels[change]))),
        });
  const body = <PostContent html={revision.html} lang={history.lang} emojis={revision.emojis} onLinkClick={history.onLinkClick} />;
  const media =
    revision.media.length > 0 ? (
      <MediaGrid
        items={revision.media}
        sensitive={revision.sensitive}
        noDescriptionLabel={t("client:timeline.media.no_description")}
        revealLabel={t("client:timeline.media.reveal")}
        viewerLabels={viewerLabels}
      />
    ) : null;
  return (
    <article aria-labelledby={headingId} className="flex flex-col gap-2 bg-surface px-4 py-3">
      <h2 id={headingId} className="text-lg font-semibold">
        <time dateTime={revision.time.dateTime}>{revision.time.absolute}</time>
      </h2>
      {(kind !== null || changes !== null) && (
        <p className="flex flex-wrap gap-x-3 gap-y-1 text-sm text-fg-muted">
          {kind !== null && <span className="font-medium text-fg">{t(kind)}</span>}
          {changes !== null && <span>{changes}</span>}
        </p>
      )}
      {revision.contentWarning === null ? (
        <>
          {body}
          {media}
        </>
      ) : (
        <ContentWarning summary={<EmojiText text={revision.contentWarning} emojis={revision.emojis} />} summaryLang={history.lang} toggleLabel={t("client:timeline.content_warning.toggle")}>
          {body}
          {media}
        </ContentWarning>
      )}
    </article>
  );
}
