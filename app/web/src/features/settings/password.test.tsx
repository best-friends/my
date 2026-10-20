// SPDX-License-Identifier: AGPL-3.0-only
// The password page through the whole app (web-client.md T4): changing the password with the current one, what a
// wrong current password and too many of them say, with MSW playing the server.
import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test } from "vitest";
import type { Schemas } from "../../test/fixtures";
import { apiBase, problem, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

beforeEach(() => {
  server.use(signedInHandler);
  document.documentElement.lang = "en";
  document.documentElement.dir = "ltr";
});

/** The server's password of the viewer, "correct horse", with the changes it received. */
function passwordServer() {
  const changes: Schemas["ChangePasswordRequest"][] = [];
  server.use(
    http.post(`${apiBase}/accounts/password`, async ({ request }) => {
      const body = (await request.json()) as Schemas["ChangePasswordRequest"];
      changes.push(body);
      if (body.current_password === "limited") {
        return problem(429, "about:blank", "Too many requests.", { headers: { "Retry-After": "60" } });
      }
      if (body.current_password !== "correct horse") return problem(422, "/api/v1/problems/wrong-password", "The password is wrong.");
      return new HttpResponse(null, { status: 204 });
    }),
  );
  return changes;
}

describe("password", () => {
  test("is in the settings menu", async () => {
    await renderApp({ path: "/settings" });
    const link = await screen.findByRole("link", { name: "Password" });
    expect(link).toHaveAttribute("href", "/settings/password");
  });

  test("changes the password after the current one: a wrong one is the field's error", async () => {
    const changes = passwordServer();
    const { user } = await renderApp({ path: "/settings/password" });

    expect(await screen.findByRole("heading", { level: 1, name: "Password" })).toBeInTheDocument();
    const current = screen.getByLabelText(/^Current password/);
    const next = screen.getByLabelText(/^New password/);
    expect(current).toHaveAttribute("autocomplete", "current-password");
    expect(next).toHaveAttribute("autocomplete", "new-password");
    expect(next).toHaveAttribute("minlength", "8");
    expect(next).toHaveAccessibleDescription("8 to 128 characters.");

    await user.type(current, "wrong");
    await user.type(next, "a new password");
    await user.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("The password is wrong.")).toBeInTheDocument();
    expect(current).toHaveAttribute("aria-invalid", "true");

    await user.clear(current);
    await user.type(current, "correct horse");
    await user.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("Your password is changed. Other browsers and apps are signed out.")).toBeInTheDocument();
    expect(changes).toEqual([
      { current_password: "wrong", new_password: "a new password" },
      { current_password: "correct horse", new_password: "a new password" },
    ]);
    expect(current).toHaveValue("");
    expect(next).toHaveValue("");
  });

  test("says why when too many current passwords were wrong", async () => {
    passwordServer();
    const { user } = await renderApp({ path: "/settings/password" });
    await user.type(await screen.findByLabelText(/^Current password/), "limited");
    await user.type(screen.getByLabelText(/^New password/), "a new password");
    await user.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText(/^Too many incorrect passwords:/)).toHaveAttribute("role", "alert");
  });
});
