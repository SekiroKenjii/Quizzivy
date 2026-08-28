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

  // Their own tree, not "/" -- which would bounce off the index route back to
  // the form they just completed.
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
  // §9 gives the student no profile block: their name and email come from the
  // teacher or from Google, and there is nothing here to edit.
  await expect(page.getByRole("heading", { name: "Hồ sơ" })).toHaveCount(0);
});

test("unlinking is disabled, with a reason, when Google is the only way in", async ({
  page,
}) => {
  // §15. The account would still exist, still hold its work, and nobody --
  // including its owner -- could sign into it.
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
