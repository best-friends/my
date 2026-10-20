// SPDX-License-Identifier: AGPL-3.0-only
// The custom emoji picker of the post form and the profile (web-client.md T4, ADR-0051 §4): the real router and
// QueryClient, MSW for the server.
import { screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, test } from "vitest";
import { signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { insertShortcode } from "./hooks/insertShortcode";
import { customEmojisHandler, pickerEmojiFixture } from "./test/handlers";

beforeEach(() => {
  sessionStorage.clear();
  localStorage.clear();
  document.documentElement.lang = "en";
  document.documentElement.dir = "ltr";
});

describe("inserting a shortcode", () => {
  test("puts it at the caret with the spaces that keep it a word of its own", () => {
    expect(insertShortcode("", 0, 0, "blobcat")).toEqual({ value: ":blobcat: ", caret: 10 });
    expect(insertShortcode("Hello", 5, 5, "wave")).toEqual({ value: "Hello :wave: ", caret: 13 });
    expect(insertShortcode("Hello world", 6, 6, "wave")).toEqual({ value: "Hello :wave: world", caret: 13 });
    expect(insertShortcode("Hello world", 5, 5, "wave")).toEqual({ value: "Hello :wave: world", caret: 12 });
    // A selection is replaced.
    expect(insertShortcode("I am happy", 5, 10, "smile")).toEqual({ value: "I am :smile: ", caret: 13 });
  });
});

describe("the picker of the post form", () => {
  test("is not offered while the server has no custom emoji", async () => {
    server.use(signedInHandler);
    const { user } = await renderApp();
    await user.click(await screen.findByRole("button", { name: "Write a post" }));
    const sheet = await screen.findByRole("dialog", { name: "New post" });

    expect(await within(sheet).findByRole("textbox", { name: "Your post" })).toBeInTheDocument();
    expect(within(sheet).queryByRole("button", { name: "Add emoji" })).not.toBeInTheDocument();
  });

  test("puts a picked emoji into the text at the caret, and offers it first next time", async () => {
    server.use(
      signedInHandler,
      customEmojisHandler([pickerEmojiFixture("wave", { aliases: ["hello"] }), pickerEmojiFixture("blobcat", { category: "Blobs" })]),
    );
    const { user } = await renderApp();
    await user.click(await screen.findByRole("button", { name: "Write a post" }));
    const sheet = await screen.findByRole("dialog", { name: "New post" });
    const text = await within(sheet).findByRole("textbox", { name: "Your post" });
    await user.type(text, "Hi");

    await user.click(await within(sheet).findByRole("button", { name: "Add emoji" }));
    const picker = await screen.findByRole("dialog", { name: "Custom emoji" });
    await waitFor(() => {
      expect(within(picker).getByRole("searchbox", { name: "Search emoji" })).toHaveFocus();
    });
    await user.type(within(picker).getByRole("searchbox", { name: "Search emoji" }), "hello");
    await user.click(within(picker).getByRole("row", { name: ":wave:" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "Custom emoji" })).not.toBeInTheDocument();
    });
    expect(text).toHaveValue("Hi :wave: ");
    await waitFor(() => {
      expect(text).toHaveFocus();
    });

    await user.click(within(sheet).getByRole("button", { name: "Add emoji" }));
    const again = await screen.findByRole("dialog", { name: "Custom emoji" });
    const recent = within(again).getAllByRole("row");
    expect(recent[0]).toHaveTextContent("Recently used");
    expect(recent[1]).toHaveAccessibleName(":wave:");
  });
});

describe("the picker of the profile", () => {
  test("puts a picked emoji into the display name", async () => {
    server.use(signedInHandler, customEmojisHandler([pickerEmojiFixture("blobcat")]));
    const { user } = await renderApp({ path: "/settings/profile" });
    const name = await screen.findByRole("textbox", { name: "Display name" });
    await user.clear(name);
    await user.type(name, "Alice");

    await user.click(await screen.findByRole("button", { name: "Add emoji to the display name" }));
    const picker = await screen.findByRole("dialog", { name: "Custom emoji" });
    await user.click(within(picker).getByRole("row", { name: ":blobcat:" }));

    await waitFor(() => {
      expect(name).toHaveValue("Alice :blobcat: ");
    });
  });
});
