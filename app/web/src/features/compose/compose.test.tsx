// SPDX-License-Identifier: AGPL-3.0-only
// The new-post form through the whole app (web-client.md T4): the real router and QueryClient, MSW for the
// server. jsdom has no matchMedia, so the layout is the phone's (a button that opens a sheet) unless a
// test stubs it; it has no createImageBitmap / OffscreenCanvas either, so the image tests stub those
// browser APIs and check what the client did with them.
import { Button } from "@my/ui";
import { createMemoryHistory, createRootRouteWithContext, createRouter, Outlet } from "@tanstack/react-router";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { RouterContext } from "../../lib/query";
import { apiBase, instanceFixture, oauthPaths, preferencesFixture, problem, viewerStatusFixture, tokenResponse, uploadedMediaId } from "../../test/msw/handlers";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { ComposeEntry, ComposeProvider, useReply } from ".";

/** Resumes a session with the token the default handlers accept. */
function signedIn() {
  server.use(http.post(oauthPaths.refresh, () => tokenResponse("test-access-token")));
}

type Sent = { body: Record<string, unknown>; idempotencyKey: string | null };

/** Records every POST /statuses; `answers` are used in turn, then the default handler answers. */
function recordPosts(...answers: (() => Response)[]) {
  const sent: Sent[] = [];
  server.use(
    http.post(`${apiBase}/statuses`, async ({ request }) => {
      sent.push({ body: (await request.clone().json()) as Record<string, unknown>, idempotencyKey: request.headers.get("Idempotency-Key") });
      const answer = answers.shift();
      return answer === undefined ? undefined : answer();
    }),
  );
  return sent;
}

/** Finds a visible toast by its text (the announcer's live regions say it too). */
async function findToast(text: string) {
  return within(await screen.findByRole("region", { name: "Messages" })).findByText(text);
}

/** Opens the sheet from the floating button and returns it. */
async function openSheet(user: Awaited<ReturnType<typeof renderApp>>["user"]) {
  await user.click(await screen.findByRole("button", { name: "Write a post" }));
  return screen.findByRole("dialog", { name: "New post" });
}

beforeEach(() => {
  sessionStorage.clear();
});

describe("the new-post form", () => {
  test("is not offered while signed out", async () => {
    await renderApp();

    expect(await screen.findByRole("heading", { level: 1, name: "Home" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Write a post" })).not.toBeInTheDocument();
  });

  test("posts the text with its content warning, visibility and language, with an Idempotency-Key", async () => {
    signedIn();
    const sent = recordPosts();
    const { user } = await renderApp();
    const sheet = await openSheet(user);

    const text = await within(sheet).findByRole("textbox", { name: "Your post" });
    expect(text).toHaveFocus();
    await user.type(text, "Hello, world");
    expect(within(sheet).getByText("488 characters left")).toBeInTheDocument();
    await user.click(within(sheet).getByRole("checkbox", { name: "Add a content warning" }));
    await user.type(within(sheet).getByRole("textbox", { name: "Content warning" }), "Spoilers");
    // The warning counts toward the instance's limit.
    expect(within(sheet).getByText("480 characters left")).toBeInTheDocument();

    await user.click(within(sheet).getByRole("button", { name: "Visibility: Public" }));
    const choices = await screen.findByRole("radiogroup", { name: "Who can see this post?" });
    expect(within(choices).getAllByRole("radio").map((radio) => radio.getAttribute("value"))).toEqual(["public", "local", "unlisted", "private", "direct"]);
    expect(within(choices).getByRole("radio", { name: "This server" })).toHaveAccessibleDescription(
      "People signed in here, and your followers on other servers. Shown on this server's local timeline.",
    );
    await user.click(within(choices).getByRole("radio", { name: "Followers" }));
    await user.click(screen.getByRole("button", { name: "Done" }));
    await waitFor(() => {
      expect(within(sheet).getByRole("button", { name: "Visibility: Followers" })).toHaveFocus();
    });

    await user.click(within(sheet).getByRole("button", { name: /Language/ }));
    await user.click(await screen.findByRole("option", { name: "Japanese" }));

    await user.click(within(sheet).getByRole("button", { name: "Post" }));

    expect(await findToast("Posted.")).toBeInTheDocument();
    expect(sent).toEqual([
      {
        body: { text: "Hello, world", spoiler_text: "Spoilers", visibility: "private", language: "ja" },
        idempotencyKey: expect.stringMatching(/^[\da-f-]{36}$/) as unknown,
      },
    ]);
    // The sheet closes; the next draft keeps the visibility and language.
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    const next = await openSheet(user);
    expect(within(next).getByRole("textbox", { name: "Your post" })).toHaveValue("");
    expect(within(next).getByRole("button", { name: "Visibility: Followers" })).toBeInTheDocument();
  });

  test("keeps the draft when posting fails, and sends it again with the same Idempotency-Key", async () => {
    signedIn();
    const sent = recordPosts(() => problem(503, "about:blank", "The server is busy"));
    const { user } = await renderApp();
    const sheet = await openSheet(user);

    await user.type(await within(sheet).findByRole("textbox", { name: "Your post" }), "Do not lose me");
    await user.click(within(sheet).getByRole("button", { name: "Post" }));

    expect(await within(sheet).findByText("Could not post: The server is busy")).toBeInTheDocument();
    expect(within(sheet).getByRole("textbox", { name: "Your post" })).toHaveValue("Do not lose me");
    expect(screen.getByRole("alert")).toHaveTextContent("Could not post: The server is busy");

    await user.click(within(sheet).getByRole("button", { name: "Post" }));

    expect(await findToast("Posted.")).toBeInTheDocument();
    expect(sent).toHaveLength(2);
    expect(sent[1]?.idempotencyKey).toBe(sent[0]?.idempotencyKey);
  });

  test("a failed post stays in the outbox when another draft was started meanwhile", async () => {
    signedIn();
    let fail: () => void = () => undefined;
    const failure = new Promise<void>((resolve) => {
      fail = resolve;
    });
    const sent = recordPosts();
    server.use(
      http.post(`${apiBase}/statuses`, async () => {
        await failure;
        return problem(500, "about:blank", "Broken");
      }, { once: true }),
    );
    const { user } = await renderApp();
    const sheet = await openSheet(user);

    await user.type(await within(sheet).findByRole("textbox", { name: "Your post" }), "First");
    await user.click(within(sheet).getByRole("button", { name: "Post" }));
    const outbox = await within(sheet).findByRole("list", { name: "Posts being sent" });
    expect(within(outbox).getByText("First")).toBeInTheDocument();
    expect(within(outbox).getByRole("progressbar", { name: "Sending…" })).toBeInTheDocument();
    await user.type(within(sheet).getByRole("textbox", { name: "Your post" }), "Second");
    fail();

    expect(await within(outbox).findByText("Not sent: Broken")).toBeInTheDocument();
    expect(within(sheet).getByRole("textbox", { name: "Your post" })).toHaveValue("Second");
    await user.click(within(outbox).getByRole("button", { name: "Send again" }));

    expect(await findToast("Posted.")).toBeInTheDocument();
    expect(sent.map((s) => s.body["text"])).toEqual(["First"]);
  });

  test("keeps the draft across a reload of the page", async () => {
    signedIn();
    const view = await renderApp();
    const sheet = await openSheet(view.user);
    await view.user.type(await within(sheet).findByRole("textbox", { name: "Your post" }), "Half-written");
    view.unmount();

    const { user } = await renderApp();
    const again = await openSheet(user);

    expect(await within(again).findByRole("textbox", { name: "Your post" })).toHaveValue("Half-written");
  });

  test("counts against the instance's limit and refuses text that is too long", async () => {
    signedIn();
    server.use(http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, limits: { max_post_characters: 10 } })));
    const { user } = await renderApp();
    const sheet = await openSheet(user);

    await user.type(await within(sheet).findByRole("textbox", { name: "Your post" }), "Twelve chars");

    expect(within(sheet).getByText("2 characters too many")).toBeInTheDocument();
    expect(within(sheet).getByRole("textbox", { name: "Your post" })).toHaveAttribute("aria-invalid", "true");
    expect(within(sheet).getByRole("button", { name: "Post" })).toBeDisabled();
  });
});

/** Stubs the browser's image decoding and encoding, and records what the client asked of them. */
async function stubImageApis() {
  const drawn: { width: number; height: number }[] = [];
  // Node.js's fetch (the Request that the API client builds) cannot read a body made of jsdom's FormData
  // and Blob, so the upload uses Node.js's own: Blob and File from node:buffer (the web tsconfig has no
  // Node.js types, hence the untyped import), and the FormData class of Node.js's fetch, from a Response.
  const nodeBuffer = "node:buffer";
  const { Blob: NodeBlob, File: NodeFile } = (await import(/* @vite-ignore */ nodeBuffer)) as { Blob: typeof Blob; File: typeof File };
  const NodeFormData = (await new Response("", { headers: { "Content-Type": "application/x-www-form-urlencoded" } }).formData())
    .constructor as typeof FormData;
  vi.stubGlobal("FormData", NodeFormData);
  vi.stubGlobal("Blob", NodeBlob);
  vi.stubGlobal("File", NodeFile);
  vi.stubGlobal(
    "createImageBitmap",
    vi.fn(() => Promise.resolve({ width: 6000, height: 4000, close: vi.fn() })),
  );
  vi.stubGlobal(
    "OffscreenCanvas",
    class {
      getContext() {
        return {
          drawImage: (_image: unknown, _x: number, _y: number, width: number, height: number) => {
            drawn.push({ width, height });
          },
        };
      }
      convertToBlob({ type }: { type: string }) {
        return Promise.resolve(new Blob(["re-encoded"], { type }));
      }
    },
  );
  // jsdom has no object URLs.
  Object.defineProperty(URL, "createObjectURL", { configurable: true, value: () => "blob:preview" });
  Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: () => undefined });
  return drawn;
}

/** Records the files of POST /media. */
function recordUploads() {
  const files: File[] = [];
  server.use(
    http.post(`${apiBase}/media`, async ({ request }) => {
      const file = (await request.formData()).get("file");
      if (file instanceof File) files.push(file);
      return undefined;
    }),
  );
  return files;
}

/** The hidden file input of the "Add images" button (React Aria's FileTrigger). */
function fileInput(): HTMLInputElement {
  // eslint-disable-next-line testing-library/no-node-access -- the file input is hidden behind its button, as the browser's picker is.
  const input = document.querySelector<HTMLInputElement>('input[type="file"]');
  if (input === null) throw new Error("no file input");
  return input;
}

describe("images", () => {
  test("are scaled down and re-encoded before upload, always have a description field, and post with it", async () => {
    signedIn();
    const drawn = await stubImageApis();
    const uploads = recordUploads();
    const sent = recordPosts();
    const { user } = await renderApp();
    const sheet = await openSheet(user);
    await user.type(await within(sheet).findByRole("textbox", { name: "Your post" }), "A photo");

    await user.upload(fileInput(), new File(["exif and pixels"], "IMG_0001.jpeg", { type: "image/jpeg" }));

    const description = await within(sheet).findByRole("textbox", { name: "Description of image 1" });
    expect(description).toHaveAccessibleDescription(/Describe the image for people who cannot see it\./);
    expect(within(sheet).getByRole("button", { name: "Post" })).toBeDisabled();
    // 6000 × 4000 is more than 3840 × 2160 pixels of area: scaled to fit, with the aspect ratio kept.
    await waitFor(() => {
      expect(uploads).toHaveLength(1);
    });
    expect(drawn).toEqual([{ width: 3527, height: 2351 }]);
    expect(uploads[0]?.name).toBe("IMG_0001.jpg");
    expect(uploads[0]?.type).toBe("image/jpeg");
    expect(await uploads[0]?.text()).toBe("re-encoded");

    await user.type(description, "A cat on a sofa");
    await user.click(within(sheet).getByRole("checkbox", { name: "Hide the images until readers show them" }));
    // The upload was processing; the poll finds it ready.
    const post = within(sheet).getByRole("button", { name: "Post" });
    await waitFor(() => {
      expect(post).toBeEnabled();
    });
    await user.click(post);

    expect(await findToast("Posted.")).toBeInTheDocument();
    expect(sent[0]?.body).toEqual({
      text: "A photo",
      visibility: "public",
      language: "en",
      media: [{ id: uploadedMediaId, description: "A cat on a sofa" }],
      sensitive: true,
    });
  });

  test("ask before posting without descriptions when the preferences require them", async () => {
    signedIn();
    await stubImageApis();
    server.use(http.get(`${apiBase}/preferences`, () => HttpResponse.json({ ...preferencesFixture, require_media_description: true })));
    const sent = recordPosts();
    const { user } = await renderApp();
    const sheet = await openSheet(user);
    await user.type(await within(sheet).findByRole("textbox", { name: "Your post" }), "Look");
    await user.upload(fileInput(), new File(["png"], "chart.png", { type: "image/png" }));
    const description = await within(sheet).findByRole("textbox", { name: "Description of image 1" });
    expect(description).toHaveAccessibleDescription(/Your settings ask for a description of every image\./);
    const post = within(sheet).getByRole("button", { name: "Post" });
    await waitFor(() => {
      expect(post).toBeEnabled();
    });

    await user.click(post);
    const question = await screen.findByRole("alertdialog", { name: "Post without image descriptions?" });
    expect(within(question).getByText(/1 image has no description\./)).toBeInTheDocument();
    await user.click(within(question).getByRole("button", { name: "Add descriptions" }));
    await waitFor(() => {
      expect(description).toHaveFocus();
    });
    expect(sent).toHaveLength(0);

    await user.click(post);
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Post anyway" }));

    expect(await findToast("Posted.")).toBeInTheDocument();
    expect(sent[0]?.body["media"]).toEqual([{ id: uploadedMediaId }]);
  });
});

/** A post by someone else, in the `local` visibility, that mentions the viewer and a third account. */
const localPost = viewerStatusFixture({
  id: "0192f0c4-7a8e-7000-8000-0000000000c1",
  account: { ...viewerStatusFixture().account, id: "0192f0c4-7a8e-7000-8000-000000000002", username: "bob", acct: "bob", display_name: "Bob" },
  content: "<p>Anyone around?</p>",
  visibility: "local",
  mentions: [
    { id: "0192f0c4-7a8e-7000-8000-000000000001", username: "alice", acct: "alice", url: "https://my.example/@alice" },
    { id: "0192f0c4-7a8e-7000-8000-000000000003", username: "carol", acct: "carol@remote.example", url: "https://remote.example/@carol" },
  ],
});

/** A stand-in for a timeline: the compose provider, a reply button as other features use it, and the form. */
function ReplyPage() {
  const reply = useReply();
  return (
    <>
      <h1>Thread</h1>
      {reply !== null && (
        <Button
          onPress={() => {
            reply(localPost);
          }}
        >
          Reply to Bob
        </Button>
      )}
    </>
  );
}

function createReplyRouter({ history, ...context }: RouterContext & { history: ReturnType<typeof createMemoryHistory> }) {
  const rootRoute = createRootRouteWithContext<RouterContext>()({
    component: () => (
      <ComposeProvider layout="sheet">
        <ReplyPage />
        <ComposeEntry />
        <Outlet />
      </ComposeProvider>
    ),
  });
  return createRouter({ routeTree: rootRoute, context, history });
}

describe("replies", () => {
  test("name the post, start with its mentions, default to its narrower visibility and post in reply to it", async () => {
    signedIn();
    const sent = recordPosts();
    const { user } = await renderApp({ createRouter: createReplyRouter });

    await user.click(await screen.findByRole("button", { name: "Reply to Bob" }));

    const sheet = await screen.findByRole("dialog", { name: "New post" });
    expect(within(sheet).getByText("Replying to Bob")).toBeInTheDocument();
    expect(within(sheet).getByText("Anyone around?")).toBeInTheDocument();
    const text = within(sheet).getByRole("textbox", { name: "Your post" });
    expect(text).toHaveValue("@bob @carol@remote.example ");
    // A reply to a `local` post is `local` by default (ADR-0017), narrower than the preference (public).
    expect(within(sheet).getByRole("button", { name: "Visibility: This server" })).toBeInTheDocument();

    await user.type(text, "Here!");
    await user.click(within(sheet).getByRole("button", { name: "Reply" }));

    expect(await findToast("Posted.")).toBeInTheDocument();
    expect(sent[0]?.body).toEqual({
      text: "@bob @carol@remote.example Here!",
      visibility: "local",
      language: "en",
      in_reply_to_id: localPost.id,
    });
  });

  test("can be turned into a normal post", async () => {
    signedIn();
    const sent = recordPosts();
    const { user } = await renderApp({ createRouter: createReplyRouter });
    await user.click(await screen.findByRole("button", { name: "Reply to Bob" }));
    const sheet = await screen.findByRole("dialog", { name: "New post" });

    await user.click(within(sheet).getByRole("button", { name: "Cancel reply" }));

    expect(within(sheet).queryByText("Replying to Bob")).not.toBeInTheDocument();
    expect(within(sheet).getByRole("button", { name: "Visibility: Public" })).toBeInTheDocument();
    await user.click(within(sheet).getByRole("button", { name: "Post" }));
    expect(await findToast("Posted.")).toBeInTheDocument();
    expect(sent[0]?.body).not.toHaveProperty("in_reply_to_id");
  });
});

describe("on wide screens", () => {
  test("the form is a side panel next to the page", async () => {
    signedIn();
    vi.stubGlobal("matchMedia", (query: string) => ({ matches: query === "(min-width: 80rem)", media: query, addEventListener: vi.fn(), removeEventListener: vi.fn() }));
    const sent = recordPosts();
    const { user } = await renderApp();

    const panel = await screen.findByRole("region", { name: "New post" });
    expect(screen.queryByRole("button", { name: "Write a post" })).not.toBeInTheDocument();
    await user.type(await within(panel).findByRole("textbox", { name: "Your post" }), "From the panel");
    await user.click(within(panel).getByRole("button", { name: "Post" }));

    expect(await findToast("Posted.")).toBeInTheDocument();
    expect(sent[0]?.body["text"]).toBe("From the panel");
    expect(within(panel).getByRole("textbox", { name: "Your post" })).toHaveValue("");
  });
});
