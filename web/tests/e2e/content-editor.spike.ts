import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/support/content-editor/index.html");
});

test("edits, previews, switches documents and isolates undo history", async ({
  page,
}) => {
  const editor = page.getByRole("textbox", { name: "Nội dung chỉnh sửa" });
  await editor.click();
  await page.keyboard.press("Control+End");
  await expect(
    page.getByRole("button", { name: "Tiêu đề", exact: true }),
  ).toHaveAttribute("aria-pressed", "false");
  await page.keyboard.type(" CHINH SUA");
  await expect(page.getByRole("region", { name: "Bản xem trước" })).toContainText(
    "CHINH SUA",
  );
  await page.getByRole("button", { name: "Bảng & danh sách", exact: true }).click();
  await expect(editor).toContainText("Thứ hai");
  await expect(
    page.getByRole("button", { name: "Hoàn tác", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Định dạng & ô trống", exact: true }).click();
  await expect(editor).toContainText("CHINH SUA");
  await expect(editor.locator("[data-gap-id='gap-synthetic-1']")).toHaveText("1");
});

test("keyboard formatting and undo preserve Vietnamese input", async ({ page }) => {
  const editor = page.getByRole("textbox");
  await editor.click();
  await page.keyboard.press("Control+End");
  await expect(
    page.getByRole("button", { name: "Tiêu đề", exact: true }),
  ).toHaveAttribute("aria-pressed", "false");
  await page.keyboard.press("Enter");
  await expect(editor.locator("p")).toHaveCount(4);
  await page.keyboard.press("Control+b");
  await expect(
    page.getByRole("button", { name: "In đậm", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.keyboard.insertText("Tiếng Việt mới");
  await expect(editor.locator("strong").last()).toHaveText("Tiếng Việt mới");
  await page.getByRole("button", { name: "Hoàn tác", exact: true }).click();
  await expect(editor).not.toContainText("Tiếng Việt mới");
  await page.getByRole("button", { name: "Làm lại", exact: true }).click();
  await expect(editor).toContainText("Tiếng Việt mới");
});

test("refuses formatted paste and remote assets without changing content", async ({
  page,
}) => {
  const remote: string[] = [];
  page.on("request", (request) => {
    if (!request.url().startsWith("http://localhost:")) remote.push(request.url());
  });
  const editor = page.getByRole("textbox");
  const before = await editor.innerText();
  await editor.evaluate((element) => {
    const data = new DataTransfer();
    data.setData(
      "text/html",
      '<p><img src="https://example.invalid/tracker"><script>alert(1)</script>lost marks</p>',
    );
    data.setData("text/plain", "lost marks");
    element.dispatchEvent(
      new ClipboardEvent("paste", {
        clipboardData: data,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await expect(page.getByRole("alert")).toContainText(
    "Nội dung hiện tại được giữ nguyên",
  );
  expect(await editor.innerText()).toBe(before);
  expect(remote).toEqual([]);
});

test("keeps merged cells editable and all controls reachable on a phone", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 850 });
  await page.getByRole("button", { name: "Bảng & danh sách", exact: true }).click();
  const editor = page.getByRole("textbox");
  await editor.getByText("Buổi học & nội dung", { exact: true }).click();
  await page.getByRole("button", { name: "Tách ô", exact: true }).click();
  await expect(editor.locator("tr").first().locator("th")).toHaveCount(2);
  await page.getByRole("button", { name: "Thêm hàng", exact: true }).click();
  await expect(editor.locator("tr")).toHaveCount(4);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.getByRole("button", { name: "In đậm", exact: true }).focus();
  await expect(page.getByRole("button", { name: "In đậm", exact: true })).toBeFocused();
  await page.keyboard.press("Space");
  await expect(editor).toBeFocused();
});

test("loads the standalone renderer without editor JavaScript", async ({ page }) => {
  const scripts: string[] = [];
  page.on("request", (request) => {
    if (request.resourceType() === "script") scripts.push(request.url());
  });
  await page.goto("/tests/support/content-editor/reader.html");
  await expect(page.getByText("Đọc kỹ phần được gạch chân")).toBeVisible();
  expect(scripts.some((url) => /\/editor-[^/]+\.js/.test(url))).toBe(false);
  await expect(page.locator("[contenteditable='true']")).toHaveCount(0);
});
