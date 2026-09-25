import { expect, test } from "@playwright/test";
import { signInAsAdmin } from "./support/live";

test("group graph and uploaded material round-trip through the real API, then copy and archive independently", async ({
  page,
}, testInfo) => {
  test.setTimeout(120_000);
  await signInAsAdmin(page);
  await page.goto("/admin/question-bank/groups");
  await page.getByRole("button", { name: "Nhóm mới", exact: true }).click();
  await expect(page).toHaveURL(/\/question-bank\/groups\/[0-9a-f-]+$/);
  const originalPath = new URL(page.url()).pathname;
  const title = `Nhóm bài đọc ${Date.now()}`;
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill(title);
  await page.getByRole("button", { name: "Thêm ngữ liệu", exact: true }).click();
  await page.getByLabel("Tên ngữ liệu", { exact: true }).fill("Thông báo câu lạc bộ");
  const material = page.getByRole("textbox", {
    name: "Nội dung ngữ liệu",
    exact: true,
  });
  await material.fill(
    "Câu lạc bộ tổ chức hoạt động vào cuối tuần. Hãy đọc thông tin rồi trả lời câu hỏi.",
  );
  await page.getByRole("button", { name: "Hình ảnh", exact: true }).click();
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "Tải tệp mới lên", exact: true }).click();
  await (
    await chooser
  ).setFiles({
    name: `group-diagram-${Date.now()}.png`,
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jY9sAAAAASUVORK5CYII=",
      "base64",
    ),
  });
  await expect(page.getByText(/Đã chọn: group-diagram-/)).toBeVisible();
  await page.getByLabel("Mô tả hình ảnh", { exact: true }).fill("Sơ đồ hoạt động");
  await page.getByRole("button", { name: "Chèn vào ngữ liệu", exact: true }).click();
  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  const prompt = page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true });
  await prompt.fill("Hoạt động diễn ra khi nào?");
  await page.getByRole("button", { name: "Lưu", exact: true }).click();
  await expect(page.getByText("Đã lưu trên máy chủ", { exact: true })).toBeVisible();
  await prompt.fill("Theo thông báo, hoạt động diễn ra khi nào?");
  await page.getByRole("button", { name: "Lưu", exact: true }).click();
  await expect(page.getByText("Đã lưu trên máy chủ", { exact: true })).toBeVisible();
  await page.reload();
  await page
    .getByRole("button", {
      name: "1 Theo thông báo, hoạt động diễn ra khi nào?",
      exact: true,
    })
    .click();
  await expect(prompt).toHaveValue("Theo thông báo, hoạt động diễn ra khi nào?");
  await page.getByRole("button", { name: "Thông báo câu lạc bộ", exact: true }).click();
  await page.getByRole("button", { name: "Xem ngữ liệu", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "Sơ đồ hoạt động", exact: true }),
  ).toBeVisible();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.screenshot({
    path: testInfo.outputPath("group-editor-desktop.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "Xem như học viên", exact: true }).click();
  await page.getByRole("radio", { name: "Điện thoại", exact: true }).click();
  await expect(
    page
      .getByRole("dialog")
      .getByText("Theo thông báo, hoạt động diễn ra khi nào?", { exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("group-preview-phone.png"),
    fullPage: true,
  });
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Quay lại", exact: true }).click();
  const search = page.getByPlaceholder("Tìm nhóm theo tên hoặc nội dung câu hỏi…");
  await search.fill(title);
  const original = page
    .getByRole("row")
    .filter({ has: page.locator(`a[href="${originalPath}"]`) });
  await original
    .getByRole("button", { name: `Nhân bản ${title}`, exact: true })
    .click();
  await expect(page).toHaveURL(/\/question-bank\/groups\?/);
  const copyRow = page.getByRole("row").filter({ hasText: "Vừa nhân bản" });
  await expect(copyRow).toBeVisible();
  await original.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Lưu trữ", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Xác nhận 1 mục", exact: true })
    .click();
  await expect(original).toBeHidden();
  await expect(copyRow).toBeVisible();
  await page.getByRole("combobox", { name: "Trạng thái", exact: true }).click();
  await page.getByRole("option", { name: "Đã lưu trữ", exact: true }).click();
  const archived = page
    .getByRole("row")
    .filter({ has: page.getByRole("link", { name: title, exact: true }) });
  await archived.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Xoá vĩnh viễn", exact: true }).click();
  const deleted = page.waitForResponse(
    (response) =>
      response.request().method() === "DELETE" &&
      new URL(response.url()).pathname ===
        originalPath.replace("/question-bank/groups/", "/question-groups/"),
  );
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Xác nhận 1 mục", exact: true })
    .click();
  expect((await deleted).status()).toBe(204);
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(archived).toBeHidden();
  await page.getByRole("combobox", { name: "Trạng thái", exact: true }).click();
  await page.getByRole("option", { name: "Đang sử dụng", exact: true }).click();
  await copyRow.getByRole("link", { name: title, exact: true }).click();
  await page
    .getByRole("button", {
      name: "1 Theo thông báo, hoạt động diễn ra khi nào?",
      exact: true,
    })
    .click();
  await expect(prompt).toHaveValue("Theo thông báo, hoạt động diễn ra khi nào?");
  await page.getByRole("button", { name: "Thông báo câu lạc bộ", exact: true }).click();
  await page.getByRole("button", { name: "Xem ngữ liệu", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "Sơ đồ hoạt động", exact: true }),
  ).toBeVisible();
});
