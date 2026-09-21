import { expect, test } from "@playwright/test";
import { anonymous, sessionAs, stubApi, studentUser } from "./support/api";

/**
 * §14's E2E 2a — the Phase 1 exit criterion §16 asks for, scoped to what Phase 1
 * actually builds. (§16's own "E2E 2 passes" names grading, which is Phase 4.)
 */

test("E2E 2a: a student signs in with a password and reaches their own app", async ({
  page,
}) => {
  await stubApi(page, {
    ...anonymous,
    "POST /auth/login": {
      body: { accessToken: "e2e-token", expiresIn: 900, user: studentUser },
    },
    // What the home asks for once it lands.
    "GET /app/assignments": { body: { dueNow: [], upcoming: [], completed: [] } },
    "GET /app/classes": { body: { items: [] } },
  });

  await page.goto("/login");
  await page.getByLabel("Email").fill("hocvien@example.com");
  await page.getByLabel("Mật khẩu").fill("quizzivy-dev");
  await page.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(/\/app$/);
  // Their own app: greeted by name, and -- in no class yet -- offered the way in.
  await expect(page.getByRole("heading", { name: /^Chào / })).toBeVisible();
  await expect(page.getByRole("link", { name: "Tham gia lớp" })).toHaveAttribute(
    "href",
    "/join",
  );
});

test("E2E 2a: the student settings screen renders the four cards both boards draw", async ({
  page,
}) => {
  await stubApi(page, sessionAs(studentUser));
  await page.goto("/app/settings");

  // S-17's grid, in its order: Hồ sơ, Mật khẩu, Tài khoản Google, Ngôn ngữ.
  for (const heading of ["Hồ sơ", "Mật khẩu", "Tài khoản Google", "Ngôn ngữ"]) {
    await expect(page.getByRole("heading", { name: heading })).toBeVisible();
  }
  // The name is the one thing the account may change about itself; the email is
  // the login and only an admin moves it.
  await expect(page.getByLabel("Họ và tên")).toBeEditable();
  await expect(page.getByLabel("Email")).toBeDisabled();
});

test("unlinking is disabled, with a reason, when Google is the only way in", async ({
  page,
}) => {
  await stubApi(
    page,
    sessionAs({ ...studentUser, hasPassword: false, linkedProviders: ["google"] }),
  );
  await page.goto("/app/settings");

  // aria-disabled, not disabled: S-10 keeps the control focusable so the reason
  // beside it is announced instead of skipped.
  const unlink = page.getByRole("button", { name: "Bỏ liên kết Google" });
  await expect(unlink).toHaveAttribute("aria-disabled", "true");
  await expect(page.getByText(/cách duy nhất để đăng nhập/)).toBeVisible();
  // And no password card at all: S-10's Google-only settings frame draws three
  // cards, none of them "Mật khẩu" -- there is nothing to change and no way to
  // set one, so a card would only be a hole in S-17's grid.
  await expect(page.getByRole("heading", { name: "Mật khẩu" })).toHaveCount(0);
});

test("a teacher's settings screen adds the profile block", async ({ page }) => {
  await stubApi(page, sessionAs({ ...studentUser, role: "admin", fullName: "Thuong" }));
  await page.goto("/admin/settings");

  await expect(page.getByRole("heading", { name: "Hồ sơ" })).toBeVisible();
  await expect(page.getByLabel("Họ và tên")).toHaveValue("Thuong");
});

test("signing out lives behind the student's name, and on the settings screen (S-13)", async ({
  page,
}) => {
  await stubApi(page, sessionAs(studentUser));

  await page.goto("/app");
  await expect(page.getByRole("button", { name: "Đăng xuất" })).toHaveCount(0);

  await page.getByRole("button", { name: /^Tài khoản của/ }).click();
  await page.getByRole("menuitem", { name: "Cài đặt" }).click();
  await expect(page).toHaveURL(/\/app\/settings$/);
  await expect(page.getByRole("heading", { name: "Cài đặt" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Đăng xuất" })).toBeVisible();
});

test.describe("on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("settings is the bar's icon, and signing out sits at its foot (S-03, S-10)", async ({
    page,
  }) => {
    await stubApi(page, sessionAs(studentUser));

    await page.goto("/app");
    await expect(page.getByRole("button", { name: "Đăng xuất" })).toHaveCount(0);

    await page.getByRole("link", { name: "Cài đặt" }).click();
    await expect(page).toHaveURL(/\/app\/settings$/);
    await expect(page.getByRole("heading", { name: "Cài đặt" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Đăng xuất" })).toBeVisible();
  });
});
