import { expect, test } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";

test("teacher settings retain draft fields between sections and respect reduced motion", async ({
  page,
}) => {
  await stubApi(page, sessionAs(adminUser));
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/admin/settings");
  await expect
    .poll(() =>
      page
        .getByRole("navigation", { name: "Điều hướng chính" })
        .evaluate((node) => node.scrollWidth <= node.clientWidth),
    )
    .toBe(true);
  await page.getByRole("textbox", { name: "Họ và tên" }).fill("Tên chưa lưu");
  const nav = page.getByRole("navigation", { name: "Mục cài đặt" });
  await nav.getByRole("link", { name: "Bảo mật" }).click();
  await expect(page).toHaveURL(/settings\/security$/);
  await expect(page.getByLabel("Mật khẩu mới", { exact: true })).toBeVisible();
  await nav.getByRole("link", { name: "Tuỳ chọn" }).click();
  await expect(page.getByRole("heading", { name: "Ngôn ngữ" })).toBeVisible();
  await expect(page.locator(".settings-panel:not([hidden])")).toHaveCSS(
    "animation-name",
    "none",
  );
  await nav.getByRole("link", { name: "Hồ sơ" }).click();
  await expect(page.getByRole("textbox", { name: "Họ và tên" })).toHaveValue(
    "Tên chưa lưu",
  );
  await page.setViewportSize({ width: 768, height: 900 });
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    .toBe(true);
});
