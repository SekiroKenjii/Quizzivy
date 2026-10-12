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

const thisDevice = {
  familyId: "019535d9-3df7-79fb-b466-fa907fa18001",
  device: "Mac · Chrome",
  deviceKind: "computer",
  location: "Ho Chi Minh City, VN",
  lastUsedAt: "2026-10-10T09:59:00Z",
  current: true,
};

test("a teacher's settings open on the Profile, among the deck's sections", async ({
  page,
}) => {
  await stubApi(page, {
    ...sessionAs(adminUser),
    "GET /auth/sessions": { body: { items: [thisDevice] } },
  });
  await page.goto("/teacher/settings");

  await expect(page.getByRole("heading", { level: 1, name: "Cài đặt" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Hồ sơ" })).toBeVisible();
  await expect(page.getByLabel("Họ và tên")).toHaveValue("Thuong");
  await expect(page.getByLabel("Email")).toHaveAttribute("readonly", "");
  const nav = page.getByRole("navigation", { name: "Mục cài đặt" });
  await expect(nav.getByRole("link")).toHaveText([
    "Hồ sơ",
    "Đăng nhập & bảo mật",
    "Thông báo",
    "Mặc định khi giao bài",
    "Giao diện",
    "Tài liệu API",
  ]);
  await page.goto("/teacher/settings/preferences");
  await expect(page).toHaveURL(/\/teacher\/settings$/);
  await page.goto("/admin/settings/security");
  await expect(page).toHaveURL(/\/teacher\/settings\/security$/);
  await expect(page.getByText("Thiết bị này")).toBeVisible();
});

test("a saved language opens the next load in that language, with no Vietnamese first", async ({
  page,
}) => {
  let saved = { ...adminUser, locale: "vi" };
  await stubApi(page, {
    "GET /auth/me": (route) =>
      route.fulfill({ contentType: "application/json", body: JSON.stringify(saved) }),
    "PATCH /auth/me": async (route) => {
      saved = { ...saved, ...(route.request().postDataJSON() as object) };
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify(saved),
      });
    },
    "GET /auth/sessions": { body: { items: [thisDevice] } },
  });
  await page.goto("/teacher/settings");
  await page.getByRole("combobox", { name: "Ngôn ngữ" }).click();
  await page.getByRole("option", { name: "English" }).click();
  await page.getByRole("button", { name: "Lưu thay đổi" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();

  await page.addInitScript(() => {
    const seen: string[] = [];
    (window as unknown as { seenHeadings: string[] }).seenHeadings = seen;
    new MutationObserver(() => {
      for (const heading of document.querySelectorAll("h1"))
        seen.push(heading.textContent ?? "");
    }).observe(document, { childList: true, subtree: true, characterData: true });
    document.addEventListener("DOMContentLoaded", () => {
      seen.push(`lang:${document.documentElement.lang}`);
    });
  });
  await page.reload();
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();
  const seen = await page.evaluate(
    () => (window as unknown as { seenHeadings: string[] }).seenHeadings,
  );
  expect(seen).toContain("lang:en");
  expect(seen).not.toContain("Cài đặt");
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
