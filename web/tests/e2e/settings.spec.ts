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
  });

  await page.goto("/login");
  await page.getByLabel("Email").fill("hocvien@example.com");
  await page.getByLabel("Mật khẩu").fill("quizzivy-dev");
  await page.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(/\/app$/);
  await expect(page.getByRole("heading", { name: "Bài của tôi" })).toBeVisible();
});

test("E2E 2a: the student settings screen renders §9's three sections", async ({
  page,
}) => {
  await stubApi(page, sessionAs(studentUser));
  await page.goto("/app/settings");

  for (const heading of ["Mật khẩu", "Tài khoản Google", "Ngôn ngữ"]) {
    await expect(page.getByRole("heading", { name: heading })).toBeVisible();
  }
  await expect(page.getByRole("heading", { name: "Hồ sơ" })).toHaveCount(0);
});

test("unlinking is disabled, with a reason, when Google is the only way in", async ({
  page,
}) => {
  await stubApi(
    page,
    sessionAs({ ...studentUser, hasPassword: false, linkedProviders: ["google"] }),
  );
  await page.goto("/app/settings");

  await expect(page.getByRole("button", { name: "Bỏ liên kết Google" })).toBeDisabled();
  await expect(page.getByText(/cách duy nhất để đăng nhập/)).toBeVisible();
  // And no password form to offer instead -- there is no password to change.
  await expect(
    page.getByText(/đăng nhập bằng Google nên chưa có mật khẩu/),
  ).toBeVisible();
});

test("a teacher's settings screen adds the profile block", async ({ page }) => {
  await stubApi(page, sessionAs({ ...studentUser, role: "admin", fullName: "Thuong" }));
  await page.goto("/admin/settings");

  await expect(page.getByRole("heading", { name: "Hồ sơ" })).toBeVisible();
  await expect(page.getByText("Thuong")).toBeVisible();
});

test("signing out lives on the settings screen, not one mis-tap from the work", async ({
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
