import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { signInAsAdmin } from "./support/live";

const AUDIO = fileURLToPath(new URL("./fixtures/unit5-listening.mp3", import.meta.url));

async function saved(page: Page) {
  await expect(page.locator('[role="status"][data-state="saved"]')).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
}

test("mixed builder saves new sections, moves complete groups, copies context and publishes through the real API", async ({
  page,
}, testInfo) => {
  test.setTimeout(150_000);
  await signInAsAdmin(page);
  await page.goto("/teacher/tests");
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);
  const builderPath = new URL(page.url()).pathname;
  await page.getByRole("button", { name: "Tên đề thi", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Tên đề thi", exact: true })
    .fill(`Đề nhóm ${Date.now()}`);
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("Enter");
  const sections = page.locator("[data-outline-section]");
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);
  const sectionCount = await sections.count();
  expect(sectionCount).toBeGreaterThan(0);
  await page.getByRole("button", { name: "Tên đề thi", exact: true }).click();
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("End");
  await page
    .getByRole("textbox", { name: "Tên đề thi", exact: true })
    .pressSequentially(" kiểm tra");
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("Enter");
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
      /\/teacher\/tests\/[^/]+$/.test(new URL(response.url()).pathname),
  );
  await original
    .getByRole("button", { name: `Thao tác với nhóm ${title}`, exact: true })
    .click();
  await page.getByRole("menuitem", { name: "Di chuyển xuống", exact: true }).click();
  await prompt.fill("Theo lịch, câu lạc bộ mở cửa vào ngày nào?");
  expect((await moving).status()).toBe(200);
  await saved(page);
  await expect(
    sections.last().locator(`[data-outline-group="${originalId}"]`),
  ).toBeVisible();
  await page.getByRole("button", { name: "Xem trước", exact: true }).click();
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
  await page.getByRole("button", { name: "Từ ngân hàng câu hỏi", exact: true }).click();
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
      new URL(response.url()).pathname === `/teacher/question-groups/${originalId}`,
  );
  await page
    .locator(`[data-outline-group="${originalId}"]`)
    .getByRole("button", { name: `Thao tác với nhóm ${title}`, exact: true })
    .click();
  await page.getByRole("menuitem", { name: "Xoá nhóm", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Xoá", exact: true })
    .click();
  expect((await deleted).status()).toBe(204);
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(page.locator("[data-outline-group]")).toHaveCount(1);
  await page.reload();
  await expect(page.locator("[data-outline-group]")).toHaveCount(1);
  await page.getByRole("button", { name: "Xem trước", exact: true }).click();
  await expect(
    page
      .getByRole("dialog")
      .getByText("Câu lạc bộ mở cửa vào thứ Bảy.", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  const beforeRemoval = await sections.count();
  await sections
    .last()
    .getByRole("button", { name: "Thao tác với phần", exact: true })
    .click();
  await page.getByRole("menuitem", { name: "Gỡ phần", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(sections).toHaveCount(beforeRemoval - 1);
  await saved(page);
  const published = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname.endsWith("/publish"),
  );
  await page.getByRole("button", { name: "Phát hành", exact: true }).click();
  await page
    .getByRole("dialog", { name: "Phát hành đề thi?", exact: true })
    .getByRole("button", { name: "Phát hành", exact: true })
    .click();
  expect((await published).status()).toBe(201);
  await expect(page.getByText("Đã phát hành. Giờ bạn có thể giao bài.")).toBeVisible();
  await page.goto(builderPath.replace(/\/edit$/, ""));
  await expect(page).toHaveURL(builderPath.replace(/\/edit$/, ""));
});

test("the Shared content dialog saves a recording, then a passage over it, in the shapes the real API accepts", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signInAsAdmin(page);
  await page.goto("/teacher/tests");
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await saved(page);
  await page
    .getByRole("button", { name: "Thêm nhóm câu dùng chung", exact: true })
    .click();
  const title = `Nghe và đọc ${Date.now()}`;
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill(title);
  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .fill("Người nói hẹn gặp ở đâu?");
  await saved(page);
  const group = page.locator("[data-outline-group]").first();
  const dialogName = `Nội dung dùng chung của “${title}”`;
  const groupSaved = () =>
    page.waitForResponse(
      (response) =>
        response.request().method() === "PUT" &&
        /\/teacher\/question-groups\/[^/]+$/.test(new URL(response.url()).pathname),
    );

  await group
    .getByRole("button", { name: `Thao tác với nhóm ${title}`, exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Nội dung dùng chung", exact: true })
    .click();
  let dialog = page.getByRole("dialog", { name: dialogName });
  await dialog.getByRole("button", { name: "Bài nghe", exact: true }).click();
  await dialog.getByLabel("Chọn tệp từ máy").setInputFiles(AUDIO);
  await expect(dialog.getByText("unit5-listening.mp3", { exact: true })).toBeVisible({
    timeout: 30_000,
  });
  let response = groupSaved();
  await dialog.getByRole("button", { name: "Lưu", exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(dialog).toBeHidden();
  const audioLine = group.getByRole("button", {
    name: "Bài nghe · unit5-listening.mp3 · 2 lượt nghe",
    exact: true,
  });
  await expect(audioLine).toBeVisible();

  await audioLine.click();
  dialog = page.getByRole("dialog", { name: dialogName });
  await dialog.getByRole("button", { name: "Bài đọc", exact: true }).click();
  await dialog
    .getByRole("textbox", { name: "Tiêu đề", exact: true })
    .fill("Lịch câu lạc bộ");
  await dialog
    .getByRole("textbox", { name: "Bài đọc", exact: true })
    .fill("Câu lạc bộ mở cửa vào thứ Bảy.");
  await dialog.getByRole("button", { name: "Lưu", exact: true }).click();
  const confirm = page.getByRole("dialog", { name: "Thay nội dung dùng chung?" });
  await expect(confirm).toContainText("unit5-listening.mp3");
  response = groupSaved();
  await confirm.getByRole("button", { name: "Thay và lưu", exact: true }).click();
  expect((await response).status()).toBe(200);
  await expect(dialog).toBeHidden();
  await expect(
    group.getByRole("button", { name: "Bài đọc · Lịch câu lạc bộ", exact: true }),
  ).toBeVisible();

  await page.reload();
  await expect(
    page
      .locator("[data-outline-group]")
      .first()
      .getByRole("button", { name: "Bài đọc · Lịch câu lạc bộ", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Người nói hẹn gặp ở đâu?").first()).toBeVisible();
});
