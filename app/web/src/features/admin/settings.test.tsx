// SPDX-License-Identifier: AGPL-3.0-only
// The server settings screen through the whole app (web-client.md T4): an administrator with manage_settings
// names the server in each language, the app then calls the server by that name instead of its domain, and
// a name that the server would refuse is caught in the form; with MSW playing the API.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test } from "vitest";
import { instanceFixture, type Schemas } from "../../test/fixtures";
import { apiBase, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { operatorHandler } from "./test/handlers";

beforeEach(() => {
  document.documentElement.lang = "en";
  document.documentElement.dir = "ltr";
});

type Text = Schemas["AdminInstanceText"];

/**
 * Plays the instance texts of the server: GET /instance names the instance by its English name, or its domain
 * without one, and each PUT changes a name; records the changes.
 */
function namesServer(initial: Readonly<Record<string, string>>) {
  const names = new Map(Object.entries(initial));
  const changes: { locale: string; text: string }[] = [];
  server.use(
    http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, name: names.get("en") ?? instanceFixture.domain })),
    http.get(`${apiBase}/admin/instance/texts`, () =>
      HttpResponse.json({ items: [...names].map(([locale, text]): Text => ({ key: "name", locale, text, revised_at: "2026-10-16T00:00:00Z" })) }),
    ),
    http.put(`${apiBase}/admin/instance/texts/name/:locale`, async ({ request, params }) => {
      const locale = String(params["locale"]);
      const { text } = (await request.json()) as Schemas["SetInstanceTextRequest"];
      changes.push({ locale, text });
      if (text === "") names.delete(locale);
      else names.set(locale, text);
      return HttpResponse.json({ key: "name", locale, text, revised_at: "2026-10-17T00:00:00Z" } satisfies Text);
    }),
  );
  return { changes };
}

describe("the server's name", () => {
  beforeEach(() => {
    server.use(signedInHandler, operatorHandler(["manage_settings"]));
  });

  test("shows the name in each language of the app, and the domain that stands in for a missing one", async () => {
    namesServer({ en: "my example" });
    await renderApp({ path: "/admin/settings" });

    expect(await screen.findByRole("heading", { level: 1, name: "Server settings" })).toBeInTheDocument();
    const section = screen.getByRole("region", { name: "Name" });
    expect(within(section).getByText(/in place of the domain my\.example/)).toBeInTheDocument();
    expect(within(section).getByRole("textbox", { name: "Name in English" })).toHaveValue("my example");
    expect(within(section).getByRole("textbox", { name: "Name in Japanese" })).toHaveValue("");
  });

  test("saves a new name, after which the app calls the server by it", async () => {
    const names = namesServer({ en: "my example" });
    const { user } = await renderApp({ path: "/admin/settings" });
    const english = await screen.findByRole("textbox", { name: "Name in English" });
    await waitFor(() => {
      expect(document.title).toBe("Server settings – my example");
    });

    await user.clear(english);
    await user.type(english, "Example Social");
    await user.type(screen.getByRole("textbox", { name: "Name in Japanese" }), "例のソーシャル");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Saved the name.")).toBeInTheDocument();
    expect(names.changes.toSorted((a, b) => a.locale.localeCompare(b.locale))).toEqual([
      { locale: "en", text: "Example Social" },
      { locale: "ja", text: "例のソーシャル" },
    ]);
    // The tab title and the sidebar take the new name without a reload.
    await waitFor(() => {
      expect(document.title).toBe("Server settings – Example Social");
    });
    expect(screen.getAllByText("Example Social").length).toBeGreaterThan(0);
    expect(screen.queryByText("my example")).not.toBeInTheDocument();

    // Saving again without a change sends nothing.
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("The names are already saved.")).toBeInTheDocument();
    expect(names.changes).toHaveLength(2);
  });

  test("clearing every name names the server by its domain", async () => {
    namesServer({ en: "Example Social" });
    const { user } = await renderApp({ path: "/admin/settings" });
    await user.clear(await screen.findByRole("textbox", { name: "Name in English" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Saved the name.")).toBeInTheDocument();
    await waitFor(() => {
      expect(document.title).toBe("Server settings – my.example");
    });
  });

  test("keeps a name the server would refuse in the form, with what to change", async () => {
    const names = namesServer({});
    const { user } = await renderApp({ path: "/admin/settings" });
    const english = await screen.findByRole("textbox", { name: "Name in English" });

    await user.type(english, "   ");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(english).toHaveAccessibleDescription(/Enter a name with letters, or leave the field empty\./);
    expect(english).toBeInvalid();

    await user.clear(english);
    await user.click(english);
    await user.paste("a".repeat(101));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(english).toHaveAccessibleDescription(/Use at most 100 characters\./);
    expect(names.changes).toEqual([]);
  });

  test("shows why a save failed and keeps the names", async () => {
    namesServer({});
    const refusal: Schemas["Problem"] = { type: "about:blank", title: "Unprocessable", status: 422, detail: "text is invalid" };
    server.use(
      http.put(`${apiBase}/admin/instance/texts/name/:locale`, () =>
        HttpResponse.json(refusal, { status: 422, headers: { "Content-Type": "application/problem+json" } }),
      ),
    );
    const { user } = await renderApp({ path: "/admin/settings" });
    const english = await screen.findByRole("textbox", { name: "Name in English" });
    await user.type(english, "Example Social");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Could not save the name: text is invalid")).toHaveAttribute("role", "alert");
    expect(english).toHaveValue("Example Social");
  });

  test("is in the administration menu", async () => {
    namesServer({});
    const { user } = await renderApp({ path: "/admin" });
    await user.click(await screen.findByRole("link", { name: "Server settings" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Server settings" })).toBeInTheDocument();
  });
});

test("a viewer without manage_settings sees why instead of the names, and nothing is read", async () => {
  server.use(signedInHandler, operatorHandler(["manage_reports"]));
  let reads = 0;
  server.use(
    http.get(`${apiBase}/admin/instance/texts`, () => {
      reads++;
      return HttpResponse.json({ items: [] });
    }),
  );
  await renderApp({ path: "/admin/settings" });

  expect(await screen.findByRole("heading", { level: 1, name: "Server settings" })).toBeInTheDocument();
  expect(await screen.findByText(/manage_settings/)).toBeInTheDocument();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(reads).toBe(0);
});
