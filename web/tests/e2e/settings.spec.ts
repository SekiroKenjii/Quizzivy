import { expect, test } from "@playwright/test";
import { adminUser, anonymous, sessionAs, stubApi, studentUser } from "./support/api";

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
  await page.getByLabel("Mật khẩu", { exact: true }).fill("quizzivy-dev");
  await page.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(/\/app$/);
  // Their own app: greeted by name, and -- in no class yet -- offered the way in.
  await expect(page.getByRole("heading", { name: /^Chào / })).toBeVisible();
  await page.getByRole("button", { name: "Tham gia lớp" }).click();
  await expect(page.getByRole("dialog", { name: "Tham gia lớp" })).toBeVisible();
  await expect(page.getByLabel("Mã lớp")).toBeFocused();
});

test("E2E 2a: student settings groups profile, sign-in and appearance", async ({
  page,
}) => {
  await stubApi(page, sessionAs(studentUser));
  await page.goto("/app/settings");
  await expect(page.getByRole("region", { name: "Hồ sơ" })).toBeVisible();
  await expect(page.getByLabel("Họ và tên")).toBeEditable();
  await expect(page.getByLabel("Email")).toBeDisabled();
  await expect(page.getByRole("combobox", { name: "Ngôn ngữ" })).toBeVisible();
  const sections = page.getByRole("group", { name: "Mục cài đặt" });
  await sections.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(/\/app\/settings\/sign-in$/);
  const signIn = page.getByRole("region", { name: "Đăng nhập" });
  await expect(signIn.getByText("Mật khẩu", { exact: true })).toBeVisible();
  await expect(signIn.getByText("Google", { exact: true })).toBeVisible();
  await sections.getByRole("button", { name: "Giao diện" }).click();
  await expect(page).toHaveURL(/\/app\/settings\/appearance$/);
  await expect(page.getByRole("group", { name: "Chủ đề" })).toBeVisible();
  await expect(
    page.getByRole("switch", { name: "Chữ lớn hơn khi làm bài" }),
  ).toBeVisible();
});

test("the old section addresses lead to where their controls went", async ({
  page,
}) => {
  await stubApi(page, sessionAs(studentUser));
  await page.goto("/app/settings/security");
  await expect(page).toHaveURL(/\/app\/settings\/sign-in$/);
  await expect(page.getByRole("region", { name: "Đăng nhập" })).toBeVisible();
  await page.goto("/app/settings/preferences");
  await expect(page).toHaveURL(/\/app\/settings$/);
  await expect(page.getByRole("combobox", { name: "Ngôn ngữ" })).toBeVisible();
});

test("unlinking is disabled, with a reason, when Google is the only way in", async ({
  page,
}) => {
  await stubApi(
    page,
    sessionAs({ ...studentUser, hasPassword: false, linkedProviders: ["google"] }),
  );
  await page.goto("/app/settings/sign-in");

  const unlink = page.getByRole("button", { name: "Bỏ liên kết", exact: true });
  await expect(unlink).toHaveAttribute("aria-disabled", "true");
  await expect(unlink).not.toHaveAttribute("disabled");
  await expect(page.getByText(/cách duy nhất để đăng nhập/)).toBeVisible();
  const signIn = page.getByRole("region", { name: "Đăng nhập" });
  await expect(signIn.getByText("Mật khẩu", { exact: true })).toHaveCount(0);
  await expect(signIn.getByRole("button", { name: "Đổi", exact: true })).toHaveCount(0);
});

test("a teacher's settings screen adds the profile block", async ({ page }) => {
  await stubApi(page, sessionAs(adminUser));
  await page.goto("/teacher/settings");

  await expect(page.getByRole("heading", { name: "Hồ sơ" })).toBeVisible();
  await expect(page.getByLabel("Họ và tên")).toHaveValue("Thuong");
});

test("signing out lives behind the avatar, on the settings screen too", async ({
  page,
}) => {
  await stubApi(page, sessionAs(studentUser));

  await page.goto("/app");
  await expect(page.getByRole("button", { name: "Đăng xuất" })).toHaveCount(0);

  await page.getByRole("button", { name: /^Tài khoản của/ }).click();
  await page.getByRole("menuitem", { name: "Cài đặt" }).click();
  await expect(page).toHaveURL(/\/app\/settings$/);
  await expect(page.getByRole("heading", { name: "Cài đặt" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Đăng xuất" })).toHaveCount(0);
  await page.getByRole("button", { name: /^Tài khoản của/ }).click();
  await expect(page.getByRole("menuitem", { name: "Đăng xuất" })).toBeVisible();
});
