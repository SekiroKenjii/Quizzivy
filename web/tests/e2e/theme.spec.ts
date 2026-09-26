import { expect, test, type Page } from "@playwright/test";
import { adminUser, anonymous, sessionAs, stubApi } from "./support/api";

async function prefer(page: Page, theme: string) {
  await page.addInitScript((value) => {
    localStorage.setItem("quizzivy.theme", value);
  }, theme);
}

const background = (page: Page) =>
  page.evaluate(() => getComputedStyle(document.body).backgroundColor);

test("paints a dark preference before the app runs", async ({ page }) => {
  await prefer(page, "dark");
  await page.route("**/assets/*.js", (route) => route.abort());
  await page.goto("/login");
  await expect(page.locator("html")).toHaveClass(/\bdark\b/);
  expect(await background(page)).toBe("rgb(14, 18, 19)");
  await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute(
    "content",
    "#0e1213",
  );
});

test("follows the device while the preference is system", async ({ page }) => {
  await stubApi(page, anonymous);
  await prefer(page, "system");
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.locator("html")).not.toHaveClass(/\bdark\b/);
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveClass(/\bdark\b/);
  expect(await background(page)).toBe("rgb(14, 18, 19)");
});

test("keeps a console not yet rebuilt light under a dark preference", async ({
  page,
}) => {
  await stubApi(page, sessionAs(adminUser));
  await prefer(page, "dark");
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/admin");
  await expect(
    page.getByRole("navigation", { name: "Điều hướng chính" }),
  ).toBeVisible();
  await expect(page.locator("html")).not.toHaveClass(/\bdark\b/);
  expect(await background(page)).toBe("rgb(255, 255, 255)");
});
