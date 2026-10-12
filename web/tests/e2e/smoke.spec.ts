import { expect, test } from "@playwright/test";
import { anonymous, stubApi } from "./support/api";

/**
 * Proves the E2E harness works against a real production build. The scenarios
 * that matter are spec §14's nine; they land with the features they cover.
 */

test("the app boots and lands on /login", async ({ page }) => {
  await stubApi(page, anonymous);
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Đăng nhập");
});

test("serves Vietnamese and declares it on the document", async ({ page }) => {
  await stubApi(page, anonymous);
  await page.goto("/login");
  await expect(page.locator("html")).toHaveAttribute("lang", "vi");
});

test("an unknown path renders the 404 page, not a blank screen", async ({ page }) => {
  await stubApi(page, anonymous);
  await page.goto("/duong-dan-khong-ton-tai");
  await expect(
    page.getByRole("heading", { name: "Trang này không tồn tại" }),
  ).toBeVisible();
});

test("no console errors on a cold load", async ({ page }) => {
  await stubApi(page, anonymous);
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error" && !m.text().startsWith("Failed to load resource:")) {
      errors.push(m.text());
    }
  });
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  expect(errors).toEqual([]);
});
