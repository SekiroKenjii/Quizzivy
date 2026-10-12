import { expect, test } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";

const thisDevice = {
  familyId: "019535d9-3df7-79fb-b466-fa907fa18001",
  device: "Mac · Chrome",
  deviceKind: "computer",
  location: "Ho Chi Minh City, VN",
  lastUsedAt: "2026-10-10T09:59:00Z",
  current: true,
};

test("teacher settings keep an unsaved profile between sections and fit at every width", async ({
  page,
}) => {
  await stubApi(page, {
    ...sessionAs(adminUser),
    "GET /auth/sessions": { body: { items: [thisDevice] } },
    "GET /me/notification-preferences": {
      body: [
        "attempt.submitted",
        "attempt.flagged",
        "assignment.closing",
        "assignment.due_soon",
        "result.ready",
      ].map((event) => ({ event, inApp: true, email: false })),
    },
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/teacher/settings");
  await expect
    .poll(() =>
      page
        .getByRole("navigation", { name: "Điều hướng chính" })
        .evaluate((node) => node.scrollWidth <= node.clientWidth),
    )
    .toBe(true);

  await page.getByRole("textbox", { name: "Họ và tên" }).fill("Tên chưa lưu");
  await expect(page.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
  const nav = page.getByRole("navigation", { name: "Mục cài đặt" });
  await nav.getByRole("link", { name: "Đăng nhập & bảo mật" }).click();
  await expect(page).toHaveURL(/settings\/security$/);
  await expect(page.getByLabel("Mật khẩu mới", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Thiết bị đã đăng nhập" }),
  ).toBeVisible();
  await nav.getByRole("link", { name: "Thông báo" }).click();
  await expect(page).toHaveURL(/settings\/notifications$/);
  await expect(
    page.getByRole("switch", { name: "Học viên nộp bài trong ứng dụng" }),
  ).toBeVisible();
  await nav.getByRole("link", { name: "Mặc định khi giao bài" }).click();
  await expect(page).toHaveURL(/settings\/defaults$/);
  await page.getByRole("button", { name: "60 phút" }).click();
  await nav.getByRole("link", { name: "Giao diện" }).click();
  await expect(page).toHaveURL(/settings\/appearance$/);
  await expect(page.getByRole("switch", { name: "Bảng thu gọn" })).toBeVisible();
  await nav.getByRole("link", { name: "Hồ sơ" }).click();
  await expect(page.getByRole("textbox", { name: "Họ và tên" })).toHaveValue(
    "Tên chưa lưu",
  );

  for (const width of [1024, 768, 360]) {
    await page.setViewportSize({ width, height: 900 });
    await expect
      .poll(() =>
        page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      )
      .toBe(true);
  }
  await expect(page.getByRole("textbox", { name: "Họ và tên" })).toHaveValue(
    "Tên chưa lưu",
  );
  await nav.getByRole("link", { name: "Mặc định khi giao bài" }).click();
  await expect(page.getByRole("button", { name: "60 phút" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
});
