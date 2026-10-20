// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the rules screen (FR-705, manage_rules, web-client.md S1): the rules in effect in
// every locale (GET /admin/rules), adding one, changing its texts, moving it among the others and retiring it.
// GET /instance lists the rules in the reader's locale (the about page, the report form), and every change goes
// to the audit log.
import type { QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import type { components } from "../../../lib/api/schema.gen";
import { ApiError } from "../../../lib/errors";
import { refreshAuditLog } from "../../moderation";

type Schemas = components["schemas"];

export type Rule = Schemas["AdminRule"];
export type RuleTextInput = Schemas["InstanceRuleTextInput"];

/** The most characters of a rule's text and of its hint (InstanceRuleTextInput). */
export const ruleTextLimit = 300;
export const ruleHintLimit = 1000;

/** The rules in effect, in order (GET /admin/rules). */
export function rulesQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/admin/rules");
}

/** Adds a rule after the others (POST /admin/rules), as the options of useMutation. */
export function createRuleMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/rules"] as const,
    mutationFn: async (texts: RuleTextInput[]): Promise<Rule> => {
      const { data, error } = await api.fetch.POST("/admin/rules", { body: { texts } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Replaces the texts of a rule (PATCH /admin/rules/{id}), as the options of useMutation. */
export function updateRuleMutation(api: Api) {
  return {
    mutationKey: ["patch", "/admin/rules/{id}"] as const,
    mutationFn: async ({ id, texts }: { id: string; texts: RuleTextInput[] }): Promise<Rule> => {
      const { data, error } = await api.fetch.PATCH("/admin/rules/{id}", { params: { path: { id } }, body: { texts } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Moves a rule to a place among the rules in effect, from 1 (POST /admin/rules/{id}/move). */
export function moveRuleMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/rules/{id}/move"] as const,
    mutationFn: async ({ id, position }: { id: string; position: number }): Promise<Rule> => {
      const { data, error } = await api.fetch.POST("/admin/rules/{id}/move", { params: { path: { id } }, body: { position } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Retires a rule (POST /admin/rules/{id}/retire); reports keep citing it. */
export function retireRuleMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/rules/{id}/retire"] as const,
    mutationFn: async (id: string): Promise<Rule> => {
      const { data, error } = await api.fetch.POST("/admin/rules/{id}/retire", { params: { path: { id } } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/**
 * Marks what a change of the rules changes (ADR-0045): the rules, whose order may change for every rule, the
 * rules of GET /instance and the audit log; the queries shown now are fetched again and awaited.
 */
export async function refreshRules(queryClient: QueryClient): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ["get", "/admin/rules"] }),
    queryClient.invalidateQueries({ queryKey: ["get", "/instance"] }),
    refreshAuditLog(queryClient),
  ]);
}
