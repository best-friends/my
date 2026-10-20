// SPDX-License-Identifier: AGPL-3.0-only
// The server settings screen through the whole app (web-client.md T4): an administrator with manage_settings
// names the server in each language, the app then calls the server by that name instead of its domain, and
// a name that the server would refuse is caught in the form; the descriptions are saved by language, and the
// policies (registration, language, timelines, federation, domain actions) together; with MSW playing the API.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test } from "vitest";
import { instanceFixture, type Schemas } from "../../test/fixtures";
import { apiBase, operatorHandler, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { instanceSettingsFixture } from "./test/handlers";

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
  const changes: { locale: string; body: string }[] = [];
  server.use(
    http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, name: names.get("en") ?? instanceFixture.domain })),
    http.get(`${apiBase}/admin/instance_texts`, () =>
      HttpResponse.json({ items: [...names].map(([locale, body]): Text => ({ key: "name", locale, body, revised_at: "2026-10-16T00:00:00Z" })) }),
    ),
    http.put(`${apiBase}/admin/instance_texts/name/:locale`, async ({ request, params }) => {
      const locale = String(params["locale"]);
      const { body } = (await request.json()) as Schemas["SetInstanceTextRequest"];
      changes.push({ locale, body });
      if (body === "") names.delete(locale);
      else names.set(locale, body);
      return HttpResponse.json({ key: "name", locale, body, revised_at: "2026-10-17T00:00:00Z" } satisfies Text);
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
      { locale: "en", body: "Example Social" },
      { locale: "ja", body: "例のソーシャル" },
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
    const refusal: Schemas["Problem"] = { type: "about:blank", title: "Unprocessable", status: 422, detail: "body is invalid" };
    server.use(
      http.put(`${apiBase}/admin/instance_texts/name/:locale`, () =>
        HttpResponse.json(refusal, { status: 422, headers: { "Content-Type": "application/problem+json" } }),
      ),
    );
    const { user } = await renderApp({ path: "/admin/settings" });
    const english = await screen.findByRole("textbox", { name: "Name in English" });
    await user.type(english, "Example Social");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Could not save the name: body is invalid")).toHaveAttribute("role", "alert");
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
    http.get(`${apiBase}/admin/instance_texts`, () => {
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

describe("the server's descriptions", () => {
  beforeEach(() => {
    server.use(signedInHandler, operatorHandler(["manage_settings"]));
  });

  test("saves only the descriptions that changed, each in its language", async () => {
    const changes: { key: string; locale: string; body: string }[] = [];
    server.use(
      http.get(`${apiBase}/admin/instance_texts`, () =>
        HttpResponse.json({ items: [{ key: "short_description", locale: "en", body: "A small server.", revised_at: "2026-10-16T00:00:00Z" }] satisfies Text[] }),
      ),
      http.put(`${apiBase}/admin/instance_texts/:key/:locale`, async ({ request, params }) => {
        const { body } = (await request.json()) as Schemas["SetInstanceTextRequest"];
        const key = String(params["key"]) as Schemas["InstanceTextKey"];
        changes.push({ key, locale: String(params["locale"]), body });
        return HttpResponse.json({ key, locale: String(params["locale"]), body, revised_at: "2026-10-17T00:00:00Z" } satisfies Text);
      }),
    );
    const { user } = await renderApp({ path: "/admin/settings" });
    const section = await screen.findByRole("region", { name: "Descriptions" });
    expect(within(section).getByRole("textbox", { name: "Short description in English" })).toHaveValue("A small server.");

    await user.type(within(section).getByRole("textbox", { name: "Description in Japanese" }), "小さなサーバーです。");
    await user.click(within(section).getByRole("button", { name: "Save the descriptions" }));

    expect(await within(section).findByText("Saved the descriptions. The audit log records the change.")).toBeInTheDocument();
    expect(changes).toEqual([{ key: "description", locale: "ja", body: "小さなサーバーです。" }]);
  });
});

describe("the server's policies", () => {
  beforeEach(() => {
    server.use(signedInHandler, operatorHandler(["manage_settings"]));
  });

  /** Plays PATCH /admin/settings and counts the reads of GET /instance; records the changes. */
  function settingsServer() {
    const patches: Schemas["UpdateInstanceSettingsRequest"][] = [];
    let instanceReads = 0;
    server.use(
      http.get(`${apiBase}/instance`, () => {
        instanceReads++;
        return HttpResponse.json(instanceFixture);
      }),
      http.patch(`${apiBase}/admin/settings`, async ({ request }) => {
        const body = (await request.json()) as Schemas["UpdateInstanceSettingsRequest"];
        patches.push(body);
        return HttpResponse.json(instanceSettingsFixture(body.values));
      }),
    );
    return { patches, instanceReads: () => instanceReads };
  }

  test("shows the saved settings and sends the changed ones together", async () => {
    const settings = settingsServer();
    const { user } = await renderApp({ path: "/admin/settings" });
    const registration = await screen.findByRole("region", { name: "Registration" });
    expect(within(registration).getByRole("radio", { name: "With approval" })).toBeChecked();
    const timelines = screen.getByRole("region", { name: "Timelines" });
    expect(within(timelines).getByRole("checkbox", { name: "Offer the local timeline" })).toBeChecked();

    await user.click(within(registration).getByRole("radio", { name: "With an invite" }));
    await user.click(within(timelines).getByRole("checkbox", { name: "Offer the local timeline" }));
    await user.click(within(screen.getByRole("region", { name: "Federation" })).getByRole("checkbox", { name: "Require signed requests (authorized fetch)" }));
    const domainActions = screen.getByRole("radiogroup", { name: "Who may read the list" });
    await user.click(within(domainActions).getByRole("radio", { name: "Signed-in users" }));
    const reads = settings.instanceReads();
    await user.click(screen.getByRole("button", { name: "Save the settings" }));

    expect(await screen.findByText("Saved the settings. They take effect at once, and the audit log records each change.")).toBeInTheDocument();
    expect(settings.patches).toEqual([
      { values: { registration_mode: "invite", local_timeline_enabled: "false", authorized_fetch: "true", domain_actions_visibility: "users" } },
    ]);
    // The navigation's timelines and the registration mode come from GET /instance, read again before the status.
    expect(settings.instanceReads()).toBeGreaterThan(reads);
    expect(within(registration).getByRole("radio", { name: "With an invite" })).toBeChecked();

    // Saving again without a change sends nothing.
    await user.click(screen.getByRole("button", { name: "Save the settings" }));
    expect(await screen.findByText("The settings are already saved.")).toBeInTheDocument();
    expect(settings.patches).toHaveLength(1);
  });

  test("sends the default language picked from the app's languages", async () => {
    const settings = settingsServer();
    const { user } = await renderApp({ path: "/admin/settings" });
    const language = await screen.findByRole("region", { name: "Language" });
    await user.click(within(language).getByRole("button", { name: /Default language/ }));
    await user.click(await screen.findByRole("option", { name: "Japanese" }));
    await user.click(screen.getByRole("button", { name: "Save the settings" }));

    expect(await screen.findByText(/^Saved the settings\./)).toBeInTheDocument();
    expect(settings.patches).toEqual([{ values: { default_locale: "ja" } }]);
  });

  test("shows why a save failed, keeps the choices and changes nothing", async () => {
    const refusal: Schemas["Problem"] = { type: "about:blank", title: "Unprocessable", status: 422, detail: "values.registration_mode is invalid" };
    server.use(
      http.patch(`${apiBase}/admin/settings`, () => HttpResponse.json(refusal, { status: 422, headers: { "Content-Type": "application/problem+json" } })),
    );
    const { user } = await renderApp({ path: "/admin/settings" });
    const registration = await screen.findByRole("region", { name: "Registration" });
    await user.click(within(registration).getByRole("radio", { name: "Nobody" }));
    await user.click(screen.getByRole("button", { name: "Save the settings" }));

    expect(await screen.findByText("Could not save the settings; nothing was changed: values.registration_mode is invalid")).toHaveAttribute("role", "alert");
    expect(within(registration).getByRole("radio", { name: "Nobody" })).toBeChecked();
  });
});
