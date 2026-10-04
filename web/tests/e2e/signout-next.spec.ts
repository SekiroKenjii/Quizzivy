import { expect, test, type Page } from "@playwright/test";
import { anonymous, sessionAs, stubApi, studentUser } from "./support/api";
import { assignment, classes } from "./support/student";

const listing = {
  "GET /app/classes": { body: { items: classes } },
  "GET /app/assignments": {
    body: { dueNow: [assignment(1, true)], upcoming: [], completed: [] },
  },
};

function addresses(page: Page) {
  const seen: string[] = [];
  page.on("framenavigated", (frame) => {
    if (frame === page.mainFrame()) seen.push(frame.url());
  });
  return seen;
}

async function signOutFromClasses(page: Page) {
  await stubApi(page, {
    ...sessionAs(studentUser),
    ...listing,
    "POST /auth/logout": (route) => route.fulfill({ status: 204 }),
  });
  await page.goto("/app");
  await page.getByRole("link", { name: "Lớp", exact: true }).click();
  await page.waitForURL((url) => url.pathname === "/app/classes");
  const seen = addresses(page);
  await page.getByRole("button", { name: /^Tài khoản của/ }).click();
  await page.getByRole("menuitem", { name: "Đăng xuất" }).click();
  await expect(page.getByLabel("Email")).toBeVisible();
  return seen;
}

test("signing out leaves no next on the sign-in address", async ({ page }) => {
  const seen = await signOutFromClasses(page);
  expect(seen.filter((url) => url.includes("next="))).toEqual([]);
  expect(new URL(page.url()).pathname).toBe("/login");
  expect(new URL(page.url()).search).toBe("");
});

test("Back after a sign-out leaves no next either", async ({ page }) => {
  const seen = await signOutFromClasses(page);
  const before = seen.length;
  await page.goBack();
  await expect
    .poll(() => seen.slice(before).some((url) => new URL(url).pathname === "/login"))
    .toBe(true);
  await expect(page.getByLabel("Email")).toBeVisible();
  expect(seen.filter((url) => url.includes("next="))).toEqual([]);
  expect(new URL(page.url()).search).toBe("");
});

test("a session that expired still returns the user to their page", async ({
  page,
}) => {
  await stubApi(page, {
    ...sessionAs(studentUser),
    ...listing,
    "GET /app/classes": {
      status: 401,
      body: {
        error: {
          code: "UNAUTHORIZED",
          message: "Phiên đăng nhập không hợp lệ.",
          requestId: "e2e",
        },
      },
    },
    "POST /auth/refresh": anonymous["POST /auth/refresh"]!,
  });
  await page.goto("/app/classes");
  await expect(page.getByText("Vui lòng đăng nhập lại")).toBeVisible();
  await page.getByRole("button", { name: "Đăng nhập", exact: true }).click();
  await expect(page).toHaveURL("/login?next=%2Fapp%2Fclasses");
});
