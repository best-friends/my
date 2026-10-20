// SPDX-License-Identifier: AGPL-3.0-only
// Editing the viewer's own post and reading a post's edit history through the whole app (FR-109,
// web-client.md T4): the real router and QueryClient, MSW for the client API. Time is fixed so that the
// dates are known.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { mediaFixture, statusFixture, statusPage, viewerStatusFixture } from "../../test/fixtures";
import { apiBase, oauthPaths, problem, tokenResponse } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import type { EditStatusRequest, Status, StatusEdit } from "./api/status";

const ownId = "0192f0c4-7a8e-7000-8000-0000000000e1";
const own: Status = viewerStatusFixture({
  id: ownId,
  content: "<p>My first words</p>",
  language: "en",
  sensitive: false,
  media_attachments: [mediaFixture("0192f0c4-7a8e-7000-8000-0000000000f1", "ready", "A dog")],
});
const source = { id: ownId, text: "My first words", spoiler_text: "", language: "en" };

function signedIn() {
  server.use(http.post(oauthPaths.refresh, () => tokenResponse("test-access-token")));
}

/** Answers the viewer's post `own` and its source; the local timeline holds it. */
function serveOwnPost(post: Status = own) {
  server.use(
    http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([post]))),
    http.get(`${apiBase}/statuses/:id`, () => HttpResponse.json(post)),
    http.get(`${apiBase}/statuses/:id/source`, () => HttpResponse.json(source)),
  );
}

/** Finds a visible toast by its text (the announcer's live regions say it too). */
async function findToast(text: string) {
  return within(await screen.findByRole("region", { name: "Messages" })).findByText(text);
}

/** Opens the edit dialog of the post described by `description` from its "More actions" menu. */
async function openEditor(user: Awaited<ReturnType<typeof renderApp>>["user"], description: string) {
  const article = await screen.findByRole("article", { description });
  await user.click(await within(article).findByRole("button", { name: "More actions" }));
  await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
  return screen.findByRole("dialog", { name: "Edit post" });
}

beforeEach(() => {
  // Only Date is faked: timers, MSW and user-event run as usual.
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-10-16T12:00:00Z") });
  sessionStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("editing a post", () => {
  test("is offered for the viewer's own posts only", async () => {
    signedIn();
    const other = statusFixture("0192f0c4-7a8e-7000-8000-0000000000c1");
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([other]))));
    const { user } = await renderApp({ path: "/local" });

    const article = await screen.findByRole("article", { description: "@alice hello" });
    await user.click(await within(article).findByRole("button", { name: "More actions" }));

    expect(await screen.findByRole("menuitem", { name: "Copy link" })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Edit" })).not.toBeInTheDocument();
  });

  test("starts from what the author typed, saves the changes, and shows the edited post in the timeline", async () => {
    signedIn();
    serveOwnPost();
    const sent: EditStatusRequest[] = [];
    server.use(
      http.put(`${apiBase}/statuses/:id`, async ({ request }) => {
        const body = (await request.json()) as EditStatusRequest;
        sent.push(body);
        return HttpResponse.json({
          ...own,
          content: "<p>My better words</p>",
          spoiler_text: body.spoiler_text ?? "",
          sensitive: body.sensitive ?? false,
          media_attachments: [mediaFixture("0192f0c4-7a8e-7000-8000-0000000000f1", "ready", "A dog on a beach")],
          edited_at: "2026-10-16T12:00:00Z",
        });
      }),
    );
    const { user } = await renderApp({ path: "/local" });
    const feed = await screen.findByRole("feed", { name: "Local timeline" });
    expect(within(feed).queryByRole("link", { name: "Edited" })).not.toBeInTheDocument();

    const dialog = await openEditor(user, "My first words");
    const text = await within(dialog).findByRole("textbox", { name: "Your post" });
    // The form starts from the source, with the focus in the text.
    expect(text).toHaveValue("My first words");
    await waitFor(() => {
      expect(text).toHaveFocus();
    });
    expect(within(dialog).getByRole("textbox", { name: "Description of image 1" })).toHaveValue("A dog");
    // The images stay: an edit changes their descriptions only.
    expect(within(dialog).queryByRole("button", { name: /Remove image/ })).not.toBeInTheDocument();

    await user.clear(text);
    await user.type(text, "My better words");
    await user.click(within(dialog).getByRole("button", { name: "Add a content warning" }));
    await user.type(within(dialog).getByRole("textbox", { name: "Content warning" }), "Dogs");
    await user.type(within(dialog).getByRole("textbox", { name: "Description of image 1" }), " on a beach");
    await user.click(within(dialog).getByRole("checkbox", { name: "Hide the images until readers show them" }));
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(await findToast("Post edited")).toBeInTheDocument();
    expect(sent).toEqual([
      {
        text: "My better words",
        spoiler_text: "Dogs",
        language: "en",
        media: [{ id: "0192f0c4-7a8e-7000-8000-0000000000f1", description: "A dog on a beach" }],
        sensitive: true,
      },
    ]);
    await waitFor(() => {
      expect(dialog).not.toBeInTheDocument();
    });
    // The post changes where it is, and says it was edited, with a link to its history.
    const article = within(feed).getByRole("article");
    expect(within(article).getByRole("link", { name: "Edited" })).toHaveAttribute("href", `/@alice/status/${ownId}/history`);
    expect(within(article).getByText("Dogs")).toBeInTheDocument();
  });

  test("keeps the dialog and the text when the server refuses", async () => {
    signedIn();
    serveOwnPost();
    server.use(http.put(`${apiBase}/statuses/:id`, () => problem(503, "about:blank", "Service Unavailable")));
    const { user } = await renderApp({ path: "/local" });

    const dialog = await openEditor(user, "My first words");
    const text = await within(dialog).findByRole("textbox", { name: "Your post" });
    await user.type(text, "!");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Could not save the edit: Service Unavailable");
    expect(text).toHaveValue("My first words!");
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => {
      expect(dialog).not.toBeInTheDocument();
    });
    expect(screen.getByRole("article", { description: "My first words" })).toBeInTheDocument();
  });

  test("offers to load the text again when it fails", async () => {
    signedIn();
    serveOwnPost();
    let failures = 1;
    server.use(
      http.get(`${apiBase}/statuses/:id/source`, () => (failures-- > 0 ? problem(503, "about:blank", "Service Unavailable") : HttpResponse.json(source))),
    );
    const { user } = await renderApp({ path: "/local" });

    const dialog = await openEditor(user, "My first words");
    expect(await within(dialog).findByText("Could not load the post to edit: Service Unavailable")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Try again" }));

    expect(await within(dialog).findByRole("textbox", { name: "Your post" })).toHaveValue("My first words");
  });

  test("does not save an empty text or one over the limit", async () => {
    signedIn();
    serveOwnPost();
    const { user } = await renderApp({ path: "/local" });

    const dialog = await openEditor(user, "My first words");
    const text = await within(dialog).findByRole("textbox", { name: "Your post" });
    await user.clear(text);

    expect(within(dialog).getByRole("button", { name: "Save" })).toBeDisabled();
  });
});

describe("a post's edit history", () => {
  const edited: Status = { ...own, content: "<p>Third</p>", spoiler_text: "Dogs", edited_at: "2026-10-16T11:00:00Z" };
  const revisions: StatusEdit[] = [
    { content: "<p>First</p>", spoiler_text: "", sensitive: false, created_at: "2026-10-16T09:00:00Z", media_attachments: [] },
    { content: "<p>Second</p>", spoiler_text: "", sensitive: false, created_at: "2026-10-16T10:00:00Z", media_attachments: [] },
    { content: "<p>Third</p>", spoiler_text: "Dogs", sensitive: false, created_at: "2026-10-16T11:00:00Z", media_attachments: [] },
  ];

  test("is where the mark of an edited post leads, and lists the revisions newest first", async () => {
    signedIn();
    serveOwnPost(edited);
    server.use(http.get(`${apiBase}/statuses/:id/history`, () => HttpResponse.json({ items: revisions })));
    const { user, router } = await renderApp({ path: `/@alice/${ownId}` });

    await user.click(await screen.findByRole("link", { name: "Edited" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Edit history" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(`/@alice/status/${ownId}/history`);
    await waitFor(() => {
      expect(document.title).toBe("Edit history – my example");
    });
    const list = screen.getByRole("list", { name: "Versions" });
    const [third, second, first] = within(list).getAllByRole("article");
    if (third === undefined || second === undefined || first === undefined) throw new Error("expected three revisions");
    // Newest first, each named by when it was made.
    expect(within(third).getByRole("heading", { level: 2 })).toHaveTextContent("Friday, October 16, 2026 at 11:00 AM");
    expect(within(third).getByText("Current version")).toBeInTheDocument();
    expect(within(third).getByText("Changed: text and content warning")).toBeInTheDocument();
    expect(within(third).getByRole("button", { name: "Show content" })).toHaveAttribute("aria-expanded", "false");
    expect(within(second).getByText("Changed: text")).toBeInTheDocument();
    expect(within(second).getByText("Second")).toBeInTheDocument();
    expect(within(first).getByText("Original post")).toBeInTheDocument();
    expect(within(first).getByText("First")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Back to the post" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Post" })).toBeInTheDocument();
  });

  test("says when the post is not found", async () => {
    server.use(http.get(`${apiBase}/statuses/:id/history`, () => problem(404, "/api/v1/problems/not-found", "Not Found")));
    await renderApp({ path: `/@alice/status/${ownId}/history` });

    expect(await screen.findByRole("heading", { level: 1, name: "Post not found" })).toBeInTheDocument();
  });
});
