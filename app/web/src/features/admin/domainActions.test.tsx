// SPDX-License-Identifier: AGPL-3.0-only
// The domain actions screen through the whole app (web-client.md T4): an operator with manage_domains lists the
// actions, takes new ones (a suspension only after its confirmation), and lifts them, with MSW playing the
// admin API. The server keeps domains in IDNA A-labels; the screen shows them with their U-labels.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test } from "vitest";
import type { Schemas } from "../../test/fixtures";
import { apiBase, problem, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { domainActionFixture, newDomainAction, operatorHandler } from "./test/handlers";

beforeEach(() => {
  server.use(signedInHandler, operatorHandler(["manage_domains"]));
  document.documentElement.lang = "en";
  document.documentElement.dir = "ltr";
});

type DomainAction = Schemas["AdminDomainAction"];
type NewAction = Schemas["CreateDomainActionRequest"];

/** U+2066 LEFT-TO-RIGHT ISOLATE … U+2069 POP DIRECTIONAL ISOLATE: a domain inside a sentence. */
function isolated(domain: string): string {
  return `⁦${domain}⁩`;
}

const idn = domainActionFixture({
  id: "0192f0c4-7a8e-7000-8000-0000000000a2",
  domain: "xn--r8jz45g.jp",
  kind: "suspend",
  public_comment: "Spam from every account.",
  private_note: "Reported 12 times.",
  obfuscate: true,
  issued_at: "2026-10-12T08:00:00Z",
  expires_at: "2026-11-11T08:00:00Z",
});
const limit = domainActionFixture();
const lifted = domainActionFixture({ id: "0192f0c4-7a8e-7000-8000-0000000000a0", kind: "reject_media", state: "lifted", lifted_at: "2026-10-11T00:00:00Z" });

/** Plays the domain actions of the server: a list that new actions join and lifts change; records the requests. */
function domainActionsServer(initial: DomainAction[]) {
  let actions = initial;
  const lists: string[] = [];
  const created: NewAction[] = [];
  const liftedIds: string[] = [];
  server.use(
    http.get(`${apiBase}/admin/domain_actions`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      lists.push(query.toString());
      const domain = query.get("domain");
      const items = actions.filter((action) => (query.get("active") !== "true" || action.state === "active") && (domain === null || action.domain === domain));
      return HttpResponse.json({ items });
    }),
    http.post(`${apiBase}/admin/domain_actions`, async ({ request }) => {
      const body = (await request.json()) as NewAction;
      created.push(body);
      // The server records U-labels as A-labels.
      const action = newDomainAction({ ...body, domain: body.domain === "例え.jp" ? "xn--r8jz45g.jp" : body.domain });
      actions = [action, ...actions];
      return HttpResponse.json(action);
    }),
    http.post(`${apiBase}/admin/domain_actions/:id/lift`, ({ params }) => {
      const id = String(params["id"]);
      liftedIds.push(id);
      const action = actions.find((item) => item.id === id);
      if (action === undefined) return problem(404, "about:blank", "Not Found");
      const next: DomainAction = { ...action, state: "lifted", lifted_at: "2026-10-17T00:00:00Z" };
      actions = actions.map((item) => (item.id === id ? next : item));
      return HttpResponse.json(next);
    }),
  );
  return { lists, created, liftedIds };
}

describe("the list of domain actions", () => {
  test("shows the actions in effect with both forms of a domain, and all of them in the other tab", async () => {
    const api = domainActionsServer([idn, limit, lifted]);
    const { user, router } = await renderApp({ path: "/admin/domain-actions" });

    expect(await screen.findByRole("heading", { level: 1, name: "Domain actions" })).toBeInTheDocument();
    const list = screen.getByRole("tabpanel");
    expect(within(list).getAllByRole("heading", { level: 3 }).map((heading) => heading.textContent)).toEqual(["例え.jp (xn--r8jz45g.jp)", "spam.example"]);
    const [row] = within(list).getAllByRole("listitem");
    if (row === undefined) throw new Error("no action is listed");
    const details = within(row);
    expect(details.getByText("Suspend")).toBeInTheDocument();
    expect(details.getByText("In effect")).toBeInTheDocument();
    expect(details.getByText(/^Oct 12, 2026/)).toBeInTheDocument();
    expect(details.getByText(/^Nov 11, 2026/)).toBeInTheDocument();
    expect(details.getByText("Partly hidden")).toBeInTheDocument();
    expect(details.getByText("Spam from every account.")).toBeInTheDocument();
    expect(details.getByText("Reported 12 times.")).toBeInTheDocument();
    expect(details.getByRole("button", { name: `Lift Suspend for ${isolated("例え.jp (xn--r8jz45g.jp)")}, taken ${details.getByText(/^Oct 12, 2026/).textContent}` })).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "All" }));
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ filter: "all" });
    });
    expect(await within(screen.getByRole("tabpanel")).findByText("Lifted")).toBeInTheDocument();
    expect(within(screen.getByRole("tabpanel")).getAllByRole("listitem")).toHaveLength(3);
    // A lifted action has no lift button.
    expect(within(screen.getByRole("tabpanel")).getAllByRole("button", { name: /^Lift / })).toHaveLength(2);
    expect(api.lists).toEqual(["limit=40&active=true", "limit=40"]);
  });

  test("says when no action is in effect", async () => {
    domainActionsServer([]);
    await renderApp({ path: "/admin/domain-actions" });

    expect(await screen.findByRole("heading", { level: 3, name: "No domain actions in effect" })).toBeInTheDocument();
  });

  test("lifts an action: it leaves the actions in effect, focus moves on, and the result stays on the page", async () => {
    const api = domainActionsServer([idn, limit]);
    const { user } = await renderApp({ path: "/admin/domain-actions" });

    await user.click(await screen.findByRole("button", { name: /^Lift Suspend for/ }));

    await waitFor(() => {
      expect(screen.queryByRole("heading", { level: 3, name: "例え.jp (xn--r8jz45g.jp)" })).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^Lift Limit for/ })).toHaveFocus();
    });
    expect(screen.getAllByRole("status").map((status) => status.textContent)).toContain(`Suspend for ${isolated("例え.jp (xn--r8jz45g.jp)")} is lifted.`);
    expect(api.liftedIds).toEqual([idn.id]);
  });

  test("a lifted action stays in the list of all of them, and focus moves to its heading", async () => {
    domainActionsServer([limit]);
    const { user } = await renderApp({ path: "/admin/domain-actions?filter=all" });

    await user.click(await screen.findByRole("button", { name: /^Lift Limit for/ }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 3, name: "spam.example" })).toHaveFocus();
    });
    expect(within(screen.getByRole("tabpanel")).getByText("Lifted")).toBeInTheDocument();
  });

  test("an action that cannot be lifted stays, and the failure says why", async () => {
    domainActionsServer([limit]);
    server.use(http.post(`${apiBase}/admin/domain_actions/:id/lift`, () => problem(503, "about:blank", "Service Unavailable")));
    const { user } = await renderApp({ path: "/admin/domain-actions" });

    await user.click(await screen.findByRole("button", { name: /^Lift Limit for/ }));

    expect(await screen.findByText("The action could not be lifted: Service Unavailable")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Lift Limit for/ })).toBeInTheDocument();
  });
});

describe("a new domain action", () => {
  test("is taken on a domain in non-Latin letters, recorded in A-labels, and shown in both forms", async () => {
    const api = domainActionsServer([]);
    const { user } = await renderApp({ path: "/admin/domain-actions" });

    await user.type(await screen.findByRole("textbox", { name: "Domain" }), "例え.jp");
    await user.click(screen.getByRole("radio", { name: "Limit" }));
    await user.click(screen.getByRole("button", { name: /Duration/ }));
    await user.click(await screen.findByRole("option", { name: "7 days" }));
    await user.type(screen.getByRole("textbox", { name: "Public comment" }), "Unmoderated spam.");
    await user.type(screen.getByRole("textbox", { name: "Private note" }), "See report 12.");
    await user.click(screen.getByRole("checkbox", { name: "Hide part of the domain in the public list" }));
    await user.click(screen.getByRole("button", { name: "Take action" }));

    expect(await screen.findByText(`Limit is in effect for ${isolated("例え.jp (xn--r8jz45g.jp)")}.`)).toBeInTheDocument();
    expect(api.created).toEqual([{ domain: "例え.jp", kind: "limit", obfuscate: true, public_comment: "Unmoderated spam.", private_note: "See report 12.", expires_in: 7 * 86_400 }]);
    expect(await within(screen.getByRole("tabpanel")).findByRole("heading", { level: 3, name: "例え.jp (xn--r8jz45g.jp)" })).toBeInTheDocument();
    // The form is ready for the next action on the same domain.
    expect(screen.getByRole("textbox", { name: "Domain" })).toHaveValue("例え.jp");
    expect(screen.getByRole("radio", { name: "Limit" })).not.toBeChecked();
    expect(screen.getByRole("textbox", { name: "Public comment" })).toHaveValue("");
  });

  test("needs a domain and a kind before anything is sent", async () => {
    const api = domainActionsServer([]);
    const { user } = await renderApp({ path: "/admin/domain-actions" });

    await user.click(await screen.findByRole("button", { name: "Take action" }));

    expect(await screen.findByText("Enter a domain")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Domain" })).toBeInvalid();
    expect(screen.getByText("Choose an action")).toBeInTheDocument();
    expect(api.created).toEqual([]);
  });

  test("a suspension asks first; cancelling sends nothing and confirming takes it", async () => {
    const api = domainActionsServer([]);
    const { user } = await renderApp({ path: "/admin/domain-actions" });

    await user.type(await screen.findByRole("textbox", { name: "Domain" }), "spam.example");
    await user.click(screen.getByRole("radio", { name: "Suspend" }));
    const submit = screen.getByRole("button", { name: "Take action" });
    await user.click(submit);

    const dialog = await screen.findByRole("alertdialog", { name: `Suspend ${isolated("spam.example")}?` });
    expect(dialog).toHaveAccessibleDescription(/lifting the suspension does not bring them back/);
    await waitFor(() => {
      expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    });
    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(submit).toHaveFocus();
    });
    expect(api.created).toEqual([]);

    await user.click(submit);
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Suspend" }));

    expect(await screen.findByText(`Suspend is in effect for ${isolated("spam.example")}.`)).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(api.created).toEqual([{ domain: "spam.example", kind: "suspend", obfuscate: false }]);
  });

  test("a suspension that fails says why in its confirmation, which stays open", async () => {
    domainActionsServer([]);
    server.use(http.post(`${apiBase}/admin/domain_actions`, () => problem(503, "about:blank", "Service Unavailable")));
    const { user } = await renderApp({ path: "/admin/domain-actions" });

    await user.type(await screen.findByRole("textbox", { name: "Domain" }), "spam.example");
    await user.click(screen.getByRole("radio", { name: "Suspend" }));
    await user.click(screen.getByRole("button", { name: "Take action" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Suspend" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("The action could not be taken: Service Unavailable");
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
  });

  test("an action the server refuses says why under the form, and keeps what was entered", async () => {
    domainActionsServer([]);
    server.use(
      http.post(`${apiBase}/admin/domain_actions`, () =>
        HttpResponse.json(
          { type: "about:blank", title: "Unprocessable Content", status: 422, detail: "domain: not a valid domain" },
          { status: 422, headers: { "Content-Type": "application/problem+json" } },
        ),
      ),
    );
    const { user } = await renderApp({ path: "/admin/domain-actions" });

    await user.type(await screen.findByRole("textbox", { name: "Domain" }), "not a domain");
    await user.click(screen.getByRole("radio", { name: "Reject reports" }));
    await user.click(screen.getByRole("button", { name: "Take action" }));

    expect(await screen.findByText("The action could not be taken: domain: not a valid domain")).toHaveAttribute("role", "alert");
    expect(screen.getByRole("textbox", { name: "Domain" })).toHaveValue("not a domain");
    expect(screen.getByRole("radio", { name: "Reject reports" })).toBeChecked();
  });
});

describe("the way from a remote server", () => {
  test("a server's page links to its domain actions, which are listed and filled in the form", async () => {
    const api = domainActionsServer([limit, domainActionFixture({ id: "0192f0c4-7a8e-7000-8000-0000000000a3", domain: "other.example" })]);
    const { user, router } = await renderApp({ path: "/admin/instances/spam.example" });

    const link = await screen.findByRole("link", { name: "Domain actions" });
    expect(link).toHaveAccessibleDescription("See the moderation actions on this server's domain, or take a new one.");
    await user.click(link);

    expect(await screen.findByRole("heading", { level: 1, name: "Domain actions" })).toBeInTheDocument();
    expect(router.state.location.search).toEqual({ domain: "spam.example" });
    expect(screen.getByRole("textbox", { name: "Domain" })).toHaveValue("spam.example");
    expect(screen.getByText(`Showing the actions on ${isolated("spam.example")}.`)).toBeInTheDocument();
    expect(within(screen.getByRole("tabpanel")).getAllByRole("heading", { level: 3 }).map((heading) => heading.textContent)).toEqual(["spam.example"]);

    await user.click(screen.getByRole("link", { name: "Show every domain" }));
    expect(await within(screen.getByRole("tabpanel")).findByRole("heading", { level: 3, name: "other.example" })).toBeInTheDocument();
    expect(api.lists).toEqual(["limit=40&active=true&domain=spam.example", "limit=40&active=true"]);
  });
});

describe("permissions", () => {
  test("the screen is in the administration menu of a user with manage_domains", async () => {
    await renderApp({ path: "/settings" });

    const menu = await screen.findByRole("region", { name: "Administration" });
    expect(within(menu).getByRole("link", { name: "Domain actions" })).toHaveAttribute("href", "/admin/domain-actions");
  });

  test("a user without manage_domains is told why, and nothing is asked of the server", async () => {
    server.use(operatorHandler(["manage_jobs"]));
    const api = domainActionsServer([limit]);
    await renderApp({ path: "/admin/domain-actions" });

    expect(await screen.findByRole("heading", { level: 2, name: "You cannot use this screen" })).toBeInTheDocument();
    expect(api.lists).toEqual([]);
  });
});
