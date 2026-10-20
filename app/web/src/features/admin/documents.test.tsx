// SPDX-License-Identifier: AGPL-3.0-only
// The terms and policies screen through the whole app (web-client.md T4): an administrator with manage_settings
// writes the terms of service and the privacy policy in Markdown, one language at a time, saves and removes them,
// and is asked before leaving with unsaved texts, with MSW playing the admin API.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test } from "vitest";
import { instanceFixture, type Schemas } from "../../test/fixtures";
import { apiBase, operatorHandler, problem, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { instanceTextFixture } from "./test/handlers";

beforeEach(() => {
  server.use(signedInHandler, operatorHandler(["manage_settings"]));
  document.documentElement.lang = "en";
  document.documentElement.dir = "ltr";
});

type InstanceText = Schemas["AdminInstanceText"];

/** Plays the instance texts of the server: a list that saves change (an empty body removes the text); records the saves. */
function textsServer(initial: InstanceText[]) {
  let texts = initial;
  const saves: { key: string; locale: string; body: string }[] = [];
  let lists = 0;
  server.use(
    http.get(`${apiBase}/admin/instance_texts`, () => {
      lists++;
      return HttpResponse.json({ items: texts });
    }),
    http.put(`${apiBase}/admin/instance_texts/:key/:locale`, async ({ request, params }) => {
      const { body } = (await request.json()) as Schemas["SetInstanceTextRequest"];
      const key = String(params["key"]) as Schemas["InstanceTextKey"];
      const locale = String(params["locale"]);
      saves.push({ key, locale, body });
      const saved = instanceTextFixture(key, locale, body);
      texts = [...texts.filter((text) => text.key !== key || text.locale !== locale), ...(body === "" ? [] : [saved])];
      return HttpResponse.json(saved);
    }),
  );
  return { saves, lists: () => lists };
}

const terms = instanceTextFixture("terms", "en", "# Terms\n\nBe kind.");
const termsJa = { ...instanceTextFixture("terms", "ja", "# 利用規約"), revised_at: "2026-10-12T08:00:00Z" };

function section(name: string): HTMLElement {
  return screen.getByRole("region", { name });
}

describe("the terms and policies screen", () => {
  test("shows each document in the display language with the languages it is written in, and its page", async () => {
    textsServer([terms, termsJa]);
    server.use(http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, terms_url: "http://localhost:3000/auth/terms" })));
    const { user, leftTo } = await renderApp({ path: "/admin/documents" });

    expect(await screen.findByRole("heading", { level: 1, name: "Terms and policies" })).toBeInTheDocument();
    const termsSection = within(section("Terms of service"));
    expect(termsSection.getByRole("textbox", { name: "Text in English" })).toHaveValue("# Terms\n\nBe kind.");
    expect(termsSection.getByText("English and Japanese")).toBeInTheDocument();
    expect(termsSection.getByText(/^Oct 17, 2026/)).toBeInTheDocument();
    const privacy = within(section("Privacy policy"));
    expect(privacy.getByRole("textbox", { name: "Text in English" })).toHaveValue("");
    expect(privacy.getByText("No language yet, so the page and its links are hidden")).toBeInTheDocument();
    expect(privacy.getByText("Not written yet")).toBeInTheDocument();
    expect(privacy.queryByRole("link")).not.toBeInTheDocument();

    // The public page is the server's own: it loads in place of the app.
    await user.click(await termsSection.findByRole("link", { name: "Open the terms of service page" }));
    expect(leftTo.map(String)).toEqual(["http://localhost:3000/auth/terms"]);
  });

  test("saves a text and says that the audit log records it", async () => {
    const api = textsServer([]);
    const { user } = await renderApp({ path: "/admin/documents" });

    const privacy = within(await screen.findByRole("region", { name: "Privacy policy" }));
    const field = privacy.getByRole("textbox", { name: "Text in English" });
    await user.type(field, "We keep little.");
    await user.click(privacy.getByRole("button", { name: "Save" }));

    expect(await privacy.findByText("The text in English is saved. The audit log records the change.")).toBeInTheDocument();
    expect(api.saves).toEqual([{ key: "privacy_policy", locale: "en", body: "We keep little." }]);
    expect(privacy.getByText("English")).toBeInTheDocument();
    expect(field).toHaveValue("We keep little.");
  });

  test("edits another language from the URL and keeps the drafts of each language", async () => {
    const api = textsServer([terms, termsJa]);
    const { user, router } = await renderApp({ path: "/admin/documents" });

    const termsSection = within(await screen.findByRole("region", { name: "Terms of service" }));
    await user.type(termsSection.getByRole("textbox", { name: "Text in English" }), " Really.");

    await user.click(screen.getByRole("button", { name: /Language/ }));
    await user.click(await screen.findByRole("option", { name: "Japanese" }));
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ locale: "ja" });
    });
    expect(termsSection.getByRole("textbox", { name: "Text in Japanese" })).toHaveValue("# 利用規約");
    expect(termsSection.getByText(/^Oct 12, 2026/)).toBeInTheDocument();

    await router.navigate({ to: "/admin/documents", search: { locale: "en" } });
    expect(await termsSection.findByRole("textbox", { name: "Text in English" })).toHaveValue("# Terms\n\nBe kind. Really.");
    expect(api.saves).toEqual([]);
  });

  test("asks before removing a text, and removes it when the field is saved empty", async () => {
    const api = textsServer([terms]);
    const { user } = await renderApp({ path: "/admin/documents" });

    const termsSection = within(await screen.findByRole("region", { name: "Terms of service" }));
    await user.clear(termsSection.getByRole("textbox", { name: "Text in English" }));
    await user.click(termsSection.getByRole("button", { name: "Save" }));

    const dialog = await screen.findByRole("alertdialog", { name: "Remove the terms of service in English?" });
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(api.saves).toEqual([]);

    await user.click(termsSection.getByRole("button", { name: "Save" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Remove" }));

    expect(await termsSection.findByText("The text in English is removed. The audit log records the change.")).toBeInTheDocument();
    expect(api.saves).toEqual([{ key: "terms", locale: "en", body: "" }]);
    expect(termsSection.getByText("No language yet, so the page and its links are hidden")).toBeInTheDocument();
  });

  test("shows why a save failed under the form and keeps the text", async () => {
    textsServer([]);
    server.use(
      http.put(`${apiBase}/admin/instance_texts/:key/:locale`, () => problem(422, "about:blank", "The property body has an invalid value.")),
    );
    const { user } = await renderApp({ path: "/admin/documents" });

    const termsSection = within(await screen.findByRole("region", { name: "Terms of service" }));
    await user.type(termsSection.getByRole("textbox", { name: "Text in English" }), "Be kind.");
    await user.click(termsSection.getByRole("button", { name: "Save" }));

    expect(await termsSection.findByRole("alert")).toHaveTextContent("Could not save the text: The property body has an invalid value.");
    expect(termsSection.getByRole("textbox", { name: "Text in English" })).toHaveValue("Be kind.");
  });

  test("asks before leaving with unsaved texts", async () => {
    textsServer([]);
    const { user, router } = await renderApp({ path: "/admin/documents" });

    const termsSection = within(await screen.findByRole("region", { name: "Terms of service" }));
    await user.type(termsSection.getByRole("textbox", { name: "Text in English" }), "Draft");
    void router.navigate({ to: "/settings" });

    const dialog = await screen.findByRole("alertdialog", { name: "Leave without saving the texts?" });
    await user.click(within(dialog).getByRole("button", { name: "Keep editing" }));
    expect(router.state.location.pathname).toBe("/admin/documents");
    expect(termsSection.getByRole("textbox", { name: "Text in English" })).toHaveValue("Draft");
  });

  test("is not for a viewer without manage_settings", async () => {
    server.use(operatorHandler(["manage_reports"]));
    const api = textsServer([]);
    await renderApp({ path: "/admin/documents" });

    expect(await screen.findByText(/manage_settings/)).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(api.lists()).toBe(0);
  });

  test("the admin page links to it for manage_settings", async () => {
    await renderApp({ path: "/admin" });

    expect(await screen.findByRole("link", { name: "Terms and policies" })).toHaveAttribute("href", "/admin/documents");
  });
});
