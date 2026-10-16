// SPDX-License-Identifier: AGPL-3.0-only
import { beforeEach, expect, test, vi } from "vitest";
import { loadI18n } from "./i18n";
import { AppLocale } from "./locale";

beforeEach(() => {
  document.documentElement.lang = "en";
  document.documentElement.dir = "ltr";
});

test("a locale change loads the closest catalog, sets <html lang dir> and notifies the subscribers", async () => {
  const locale = new AppLocale(await loadI18n("en"), { doc: document });
  const listener = vi.fn();
  locale.subscribe(listener);

  await locale.setLocale("ar-XB");

  expect(locale.locale).toBe("ar-XB");
  expect(document.documentElement).toHaveAttribute("lang", "ar-XB");
  expect(document.documentElement).toHaveAttribute("dir", "rtl");
  expect(locale.t("client:settings.title")).not.toBe("Settings");
  expect(listener).toHaveBeenCalledTimes(1);

  await locale.setLocale("ja-JP");
  expect(locale.locale).toBe("ja");
  expect(locale.t("client:settings.title")).toBe("設定");
});

test("the latest change wins when catalogs load out of order", async () => {
  const locale = new AppLocale(await loadI18n("en"), { doc: document });

  const first = locale.setLocale("ja");
  const second = locale.setLocale("en-XA");
  await Promise.all([first, second]);

  expect(locale.locale).toBe("en-XA");
  expect(document.documentElement).toHaveAttribute("lang", "en-XA");
});

test("the same locale or time zone changes nothing", async () => {
  const locale = new AppLocale(await loadI18n("en"), { doc: document });
  const listener = vi.fn();
  locale.subscribe(listener);
  const before = locale.getSnapshot();

  await locale.setLocale("en-US");
  locale.setTimeZone(undefined);

  expect(locale.getSnapshot()).toBe(before);
  expect(listener).not.toHaveBeenCalled();
  locale.setTimeZone("Asia/Tokyo");
  expect(locale.getSnapshot().timeZone).toBe("Asia/Tokyo");
});
