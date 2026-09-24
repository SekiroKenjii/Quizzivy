import { expect, test, type Page } from "@playwright/test";
import { signInAsAdmin } from "./support/live";

async function saved(page: Page) {
  await expect(
    page.getByRole("status").filter({ hasText: /^Đã lưu \d/ }),
  ).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
}

test("mixed builder saves new sections, moves complete groups, copies context and publishes through the real API", async ({
  page,
}, testInfo) => {
  test.setTimeout(150_000);
  await signInAsAdmin(page);
  await page.goto("/admin/tests");
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).click();
  await expect(page).toHaveURL(/\/admin\/tests\/[0-9a-f-]+\/edit$/);
  const builderPath = new URL(page.url()).pathname;
  await page
    .getByRole("textbox", { name: "Tên đề thi", exact: true })
    .fill(`Đề nhóm ${Date.now()}`);
  const sections = page.locator("[data-outline-section]");
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);
  const sectionCount = await sections.count();
  expect(sectionCount).toBeGreaterThan(0);
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("End");
  await page
    .getByRole("textbox", { name: "Tên đề thi", exact: true })
    .pressSequentially(" kiểm tra");
  await saved(page);
  await expect(sections).toHaveCount(sectionCount);
  await page
    .getByRole("button", { name: "Thêm nhóm câu dùng chung", exact: true })
    .click();
  const title = `Bài đọc lớp ${Date.now()}`;
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill(title);
  await page.getByRole("button", { name: "Thêm ngữ liệu", exact: true }).click();
  await page.getByLabel("Tên ngữ liệu", { exact: true }).fill("Lịch sinh hoạt");
  await page
    .getByRole("textbox", { name: "Nội dung ngữ liệu", exact: true })
    .fill("Câu lạc bộ mở cửa vào thứ Bảy.");
  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  const prompt = page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true });
  await prompt.fill("Câu lạc bộ mở cửa vào ngày nào?");
  await saved(page);
  const original = page.locator("[data-outline-group]").first();
  const originalId = await original.getAttribute("data-outline-group");
  expect(originalId).toBeTruthy();
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);
  const moving = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      /\/admin\/tests\/[^/]+$/.test(new URL(response.url()).pathname),
  );
  await original
    .getByRole("button", { name: `Đưa nhóm ${title} xuống`, exact: true })
    .click();
  await prompt.fill("Theo lịch, câu lạc bộ mở cửa vào ngày nào?");
  expect((await moving).status()).toBe(200);
  await saved(page);
  await expect(
    sections.last().locator(`[data-outline-group="${originalId}"]`),
  ).toBeVisible();
  await page.getByRole("button", { name: "Xem như học viên", exact: true }).click();
  const preview = page.getByRole("dialog");
  await expect(
    preview.getByText("Câu lạc bộ mở cửa vào thứ Bảy.", { exact: true }),
  ).toBeVisible();
  await expect(
    preview.getByText("Theo lịch, câu lạc bộ mở cửa vào ngày nào?", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await page
    .getByRole("button", { name: "Lưu bản sao vào ngân hàng", exact: true })
    .click();
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: "Đã lưu bản sao độc lập vào ngân hàng." }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Lấy từ ngân hàng", exact: true }).click();
  await page.getByRole("button", { name: "Chọn cả nhóm câu hỏi", exact: true }).click();
  const picker = page.getByRole("dialog");
  await picker.getByRole("textbox").fill(title);
  await picker.getByRole("button", { name: new RegExp(title) }).click();
  await expect(picker).toBeHidden();
  await expect(page.locator("[data-outline-group]")).toHaveCount(2);
  await expect(page).toHaveURL(new RegExp(`${builderPath}$`));
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.screenshot({
    path: testInfo.outputPath("builder-groups-desktop.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 768, height: 1000 });
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    )
    .toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("builder-groups-tablet.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 1440, height: 1000 });
  const deleted = page.waitForResponse(
    (response) =>
      response.request().method() === "DELETE" &&
      new URL(response.url()).pathname === `/admin/question-groups/${originalId}`,
  );
  await page
    .locator(`[data-outline-group="${originalId}"]`)
    .getByRole("button", { name: `Xoá nhóm ${title}`, exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Xoá", exact: true })
    .click();
  expect((await deleted).status()).toBe(204);
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(page.locator("[data-outline-group]")).toHaveCount(1);
  await page.reload();
  await expect(page.locator("[data-outline-group]")).toHaveCount(1);
  await page.getByRole("button", { name: "Xem như học viên", exact: true }).click();
  await expect(
    page
      .getByRole("dialog")
      .getByText("Câu lạc bộ mở cửa vào thứ Bảy.", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await sections
    .last()
    .getByRole("button", { name: "Thao tác với phần", exact: true })
    .click();
  await page.getByRole("menuitem", { name: "Xoá phần", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Xoá", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await saved(page);
  const published = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname.endsWith("/publish"),
  );
  await page.getByRole("button", { name: "Phát hành", exact: true }).click();
  expect((await published).status()).toBe(201);
  await expect(page).toHaveURL(builderPath.replace(/\/edit$/, ""));
});
