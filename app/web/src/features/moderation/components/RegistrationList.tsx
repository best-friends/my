// SPDX-License-Identifier: AGPL-3.0-only
// The registrations screen below its heading (FR-721): the sign-ups that await approval, longest waiting
// first, each with the reason its applicant gave and buttons that approve or reject it. A decision cannot be
// changed afterwards, so both ask first in an alert dialog (ConfirmDialog, NFR-MOD-7) that takes an optional
// note for the moderators. A decided registration leaves the list, focus moves to the next one's buttons
// (lib/useFocusAfterRemoval), and the result stays above the list in a StatusMessage (role="status").
import { Button, ConfirmDialog, DescriptionItem, DescriptionList, EmptyState, ReasonField, StatusMessage } from "@my/ui";
import { useRef } from "react";
import { type Translate, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { type Registration, type RegistrationDecision, registrationNoteLimit } from "../api/registrations";
import { applicantInText, type RegistrationReview, useRegistrationReview, useRegistrations } from "../hooks/useRegistrations";
import { DateTime, RecordList, RequirePermission } from "./parts";

/** The texts of a decision's button and confirmation, about `applicant`. */
type DecisionTexts = { action: string; actionOf: string; title: string; description: string; tone: "primary" | "danger" };

function decisionTexts(t: Translate, decision: RegistrationDecision, applicant: string): DecisionTexts {
  if (decision === "approve") {
    return {
      action: t("client:admin.registrations.approve"),
      actionOf: t("client:admin.registrations.approve_applicant", { applicant }),
      title: t("client:admin.registrations.confirm_approve.title", { applicant }),
      description: t("client:admin.registrations.confirm_approve.description", { applicant }),
      tone: "primary",
    };
  }
  return {
    action: t("client:admin.registrations.reject"),
    actionOf: t("client:admin.registrations.reject_applicant", { applicant }),
    title: t("client:admin.registrations.confirm_reject.title", { applicant }),
    description: t("client:admin.registrations.confirm_reject.description", { applicant }),
    tone: "danger",
  };
}

const decisions = ["approve", "reject"] as const satisfies readonly RegistrationDecision[];

export function RegistrationList() {
  return (
    <RequirePermission permission="manage_accounts">
      <Registrations />
    </RequirePermission>
  );
}

function Registrations() {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const list = useRegistrations();
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((registration) => registration.user_id),
  );
  const review = useRegistrationReview(removed);
  return (
    <div className="flex flex-col gap-4">
      <StatusMessage>{review.status}</StatusMessage>
      <RecordList
        list={list}
        getId={(registration) => registration.user_id}
        listRef={listRef}
        empty={<EmptyState heading={t("client:admin.registrations.empty")} description={t("client:admin.registrations.empty.description")} />}
      >
        {(registration) => <RegistrationRow registration={registration} review={review} />}
      </RecordList>
      <DecisionDialog review={review} />
    </div>
  );
}

function RegistrationRow({ registration, review }: { registration: Registration; review: RegistrationReview }) {
  const t = useT();
  const applicant = applicantInText(t, registration);
  const handle = registration.username === undefined ? undefined : `@${registration.username}`;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col items-start gap-2 @md:flex-row @md:items-center @md:justify-between">
        <h2 className="font-semibold wrap-anywhere">
          {/* A username reads left to right in every locale. */}
          {handle === undefined ? t("client:admin.registrations.account_gone") : <span dir="ltr">{handle}</span>}
        </h2>
        <div className="flex flex-wrap gap-2">
          {decisions.map((decision) => {
            const texts = decisionTexts(t, decision, applicant);
            return (
              <Button
                key={decision}
                size="sm"
                variant={decision === "approve" ? "primary" : "secondary"}
                aria-label={texts.actionOf}
                onPress={() => {
                  review.open(registration, decision);
                }}
              >
                {texts.action}
              </Button>
            );
          })}
        </div>
      </div>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.registrations.submitted_at")}>
          <DateTime value={registration.submitted_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.registrations.email")}>
          {registration.email === "" ? (
            t("client:admin.registrations.email.none")
          ) : (
            <span dir="ltr" className="wrap-anywhere">
              {registration.email}
            </span>
          )}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.registrations.email_verified")}>
          {registration.email_verified ? t("client:admin.registrations.email_verified.yes") : t("client:admin.registrations.email_verified.no")}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.registrations.invite")}>
          {registration.invite_id === undefined ? t("client:admin.registrations.invite.none") : t("client:admin.registrations.invite.used")}
        </DescriptionItem>
      </DescriptionList>
      <DescriptionList>
        <DescriptionItem term={t("client:admin.registrations.reason")}>
          {registration.reason === undefined ? (
            t("client:admin.registrations.reason.none")
          ) : (
            // Written by the applicant, in any language and direction (web-client.md Y10).
            <span dir="auto" className="whitespace-pre-wrap wrap-anywhere">
              {registration.reason}
            </span>
          )}
        </DescriptionItem>
      </DescriptionList>
    </div>
  );
}

function DecisionDialog({ review }: { review: RegistrationReview }) {
  const t = useT();
  const { confirmation } = review;
  // The last confirmation stays while the dialog fades out; before the first one the dialog is closed and empty.
  const texts = decisionTexts(t, confirmation?.decision ?? "approve", confirmation === null ? "" : applicantInText(t, confirmation.registration));
  return (
    <ConfirmDialog
      isOpen={review.isOpen}
      onOpenChange={(isOpen) => {
        if (!isOpen) review.cancel();
      }}
      title={texts.title}
      description={texts.description}
      confirmLabel={texts.action}
      cancelLabel={t("client:admin.registrations.cancel")}
      tone={texts.tone}
      onConfirm={() => {
        review.confirm();
      }}
      isPending={review.isPending}
      {...(review.error === undefined ? {} : { errorMessage: t("client:admin.registrations.decide_failed", { reason: review.error }) })}
    >
      <ReasonField
        label={t("client:admin.registrations.note")}
        description={t("client:admin.registrations.note.description")}
        value={confirmation?.note ?? ""}
        onChange={review.changeNote}
        characterLimit={registrationNoteLimit}
        formatRemaining={(remaining) =>
          remaining < 0 ? t("client:admin.registrations.characters_over", { count: -remaining }) : t("client:admin.registrations.characters_left", { count: remaining })
        }
        errorMessage={t("client:admin.registrations.note.too_long", { max: registrationNoteLimit })}
      />
    </ConfirmDialog>
  );
}
