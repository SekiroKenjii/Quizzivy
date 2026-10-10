import { expect, test, type Page, type Route } from "@playwright/test";
import { adminUser, anonymous, sessionAs, stubApi } from "./support/api";
import {
  dashboard23,
  dashboard23Assignments,
  dashboard23Summary,
} from "../support/dashboard23";
import { contractJson } from "../support/contractResponse";

async function prefer(page: Page, theme: string) {
  await page.addInitScript((value) => {
    localStorage.setItem("quizzivy.theme", value);
  }, theme);
}

const background = (page: Page) =>
  page.evaluate(() => getComputedStyle(document.body).backgroundColor);

const noClasses = async (route: Route) => {
  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(
      await contractJson("/teacher/classes", "get", 200, {
        page: 1,
        pageSize: 20,
        total: 0,
        items: [],
        facets: { all: 0, joinable: 0, archived: 0, students: 0 },
      }).json(),
    ),
  });
};

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
  const user = { ...adminUser, preferences: { theme: "dark" as const } };
  await stubApi(page, { ...sessionAs(user), "GET /teacher/classes": noClasses });
  await prefer(page, "dark");
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/teacher/classes");
  await expect(page.getByRole("heading", { name: "Lớp học", level: 1 })).toBeVisible();
  await expect(page.locator("[data-columns] main")).toBeVisible();
  await expect(page.locator('[data-scale="deck"] main')).toHaveCount(0);
  await expect(
    page.getByRole("navigation", { name: "Điều hướng chính" }),
  ).toBeVisible();
  await expect(page.locator("html")).not.toHaveClass(/\bdark\b/);
  expect(await background(page)).toBe("rgb(255, 255, 255)");
});

test("restores the dark preference after visiting a console not yet rebuilt through SPA links", async ({
  page,
}) => {
  const user = { ...adminUser, preferences: { theme: "dark" as const } };
  await stubApi(page, {
    ...sessionAs(user),
    "GET /teacher/dashboard": async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          await contractJson("/teacher/dashboard", "get", 200, dashboard23()).json(),
        ),
      });
    },
    "GET /teacher/summary": { body: dashboard23Summary() },
    "GET /me/summary": { body: { unreadNotifications: 0 } },
    "GET /teacher/assignments": { body: dashboard23Assignments() },
    "GET /auth/sessions": { body: { items: [] } },
    "GET /teacher/classes": noClasses,
  });
  await prefer(page, "dark");
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/teacher");
  await expect(page.locator('[data-slot="teacher-dashboard"]')).toBeVisible();
  await expect(page.getByText(/14 trong 14 ngày qua/)).toBeVisible();
  await expect(page.locator('[data-scale="deck"] main')).toBeVisible();
  await expect(page.locator("html")).toHaveClass(/\bdark\b/);
  expect(await background(page)).toBe("rgb(14, 18, 19)");
  expect(await page.evaluate(() => localStorage.getItem("quizzivy.theme"))).toBe(
    "dark",
  );

  await page.getByRole("button", { name: "Tài khoản của Thuong" }).click();
  await page.getByRole("menuitem", { name: "Cài đặt", exact: true }).click();
  await expect(page).toHaveURL(/\/teacher\/settings$/);
  await expect(page.getByRole("heading", { name: "Cài đặt", level: 1 })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Họ và tên" })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Họ và tên" })).toHaveValue("Thuong");
  await expect(page.locator('[data-scale="deck"] main')).toBeVisible();
  await expect(page.locator("[data-columns] main")).toHaveCount(0);
  await expect(page.locator("html")).toHaveClass(/\bdark\b/);
  expect(await background(page)).toBe("rgb(14, 18, 19)");

  await page
    .getByRole("navigation", { name: "Điều hướng chính" })
    .getByRole("link", { name: "Lớp học", exact: true })
    .click();
  await expect(page).toHaveURL(/\/teacher\/classes$/);
  await expect(page.getByRole("heading", { name: "Lớp học", level: 1 })).toBeVisible();
  await expect(page.locator("[data-columns] main")).toBeVisible();
  await expect(page.locator('[data-scale="deck"] main')).toHaveCount(0);
  await expect(page.locator("html")).not.toHaveClass(/\bdark\b/);
  expect(await background(page)).toBe("rgb(255, 255, 255)");
  expect(await page.evaluate(() => localStorage.getItem("quizzivy.theme"))).toBe(
    "dark",
  );

  await page
    .getByRole("navigation", { name: "Điều hướng chính" })
    .getByRole("link", { name: "Tổng quan", exact: true })
    .click();
  await expect(page).toHaveURL(/\/teacher$/);
  await expect(page.locator('[data-slot="teacher-dashboard"]')).toBeVisible();
  await expect(page.getByText(/14 trong 14 ngày qua/)).toBeVisible();
  await expect(page.locator('[data-scale="deck"] main')).toBeVisible();
  await expect(page.locator("[data-columns] main")).toHaveCount(0);
  await expect(page.locator("html")).toHaveClass(/\bdark\b/);
  expect(await background(page)).toBe("rgb(14, 18, 19)");
  expect(await page.evaluate(() => localStorage.getItem("quizzivy.theme"))).toBe(
    "dark",
  );
});
