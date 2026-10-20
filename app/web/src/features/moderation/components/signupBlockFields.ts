// SPDX-License-Identifier: AGPL-3.0-only
// The choices that the form of a new sign-up block and the import of a list of them share: the help of each
// kind and severity, and the durations a block can last.
import type { ClientMessageKey } from "../../../lib/i18n";
import type { SignupBlockKind, SignupBlockSeverity } from "../api/signupBlocks";

export const signupBlockKindDescriptions: Readonly<Record<SignupBlockKind, ClientMessageKey>> = {
  email_domain: "client:admin.signup_blocks.kind.email_domain.description",
  email_hash: "client:admin.signup_blocks.kind.email_hash.description",
  ip_range: "client:admin.signup_blocks.kind.ip_range.description",
};

export const signupBlockSeverityDescriptions: Readonly<Record<SignupBlockSeverity, ClientMessageKey>> = {
  reject: "client:admin.signup_blocks.severity.reject.description",
  require_approval: "client:admin.signup_blocks.severity.require_approval.description",
};

/** A day in seconds, the unit of the durations. */
export const day = 86_400;

/** The durations a block can last, in seconds, from the shortest. */
export const signupBlockDurations = [day, 7 * day, 30 * day, 90 * day, 180 * day, 365 * day] as const;
