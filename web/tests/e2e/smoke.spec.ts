import { expect, test } from "@playwright/test";

/**
 * Proves the E2E harness works against a real production build. The scenarios
 * that matter are spec §14's nine; they land with the features they cover.
 */

test("the app boots and lands on /login", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Quizzivy");
});

test("serves Vietnamese and declares it on the document", async ({ page }) => {
  await page.goto("/login");
  await expect(page.locator("html")).toHaveAttribute("lang", "vi");
});

test("an unknown path renders the 404 page, not a blank screen", async ({ page }) => {
  await page.goto("/duong-dan-khong-ton-tai");
  await expect(
    page.getByRole("heading", { name: "Không tìm thấy trang" }),
  ).toBeVisible();
});

// §8: "Collapsible sidebar ≤1280px." Both halves of that are asserted, because
// the first version of this test used Playwright's default 1280px viewport and
// failed -- at exactly 1280 the sidebar is collapsed, which is correct.
test("admin sidebar is open by default above 1280px", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/admin");
  await expect(
    page.getByRole("navigation", { name: "Điều hướng chính" }),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: "Đề thi" })).toBeVisible();
});

test("admin sidebar collapses at 1280px and the toggle reopens it", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/admin");

  const nav = page.getByRole("navigation", { name: "Điều hướng chính" });
  await expect(nav).toBeHidden();

  const toggle = page.getByRole("button", { name: "Mở menu" });
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await toggle.click();

  await expect(nav).toBeVisible();
  await expect(page.getByRole("button", { name: "Đóng menu" })).toHaveAttribute(
    "aria-expanded",
    "true",
  );
});

// §14 requires keyboard operability; the sidebar toggle is the one control in
// the admin shell that could plausibly be mouse-only.
test("admin sidebar toggle is keyboard operable", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/admin");

  const nav = page.getByRole("navigation", { name: "Điều hướng chính" });
  await expect(nav).toBeHidden();

  await page.getByRole("button", { name: "Mở menu" }).focus();
  await page.keyboard.press("Enter");
  await expect(nav).toBeVisible();
});

test("no console errors on a cold load", async ({ page }) => {
  const errors: string[] = [];
  page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  expect(errors).toEqual([]);
});
