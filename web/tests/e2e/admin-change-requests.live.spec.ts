import { expect, test, type Page } from "@playwright/test";
import {
  chooseOption,
  goAway,
  signInAsAdmin,
  signInAsStudent,
  startAttempt,
} from "./support/live";

test.use({ actionTimeout: 10_000 });

async function addQuestion(page: Page, prompt: string) {
  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).click();
  const input = page.getByLabel("Nội dung câu hỏi", { exact: true });
  await input.click();
  await expect(input).toHaveValue("");
  await input.fill(prompt);
}

async function publish(page: Page, version: number) {
  await page.getByRole("button", { name: "Phát hành", exact: true }).click();
  await expect(page).toHaveURL(/\/admin\/tests\/[0-9a-f-]+$/);
  await expect(page.getByText(`Bản đang phát hành · v${version}`)).toBeVisible();
}

async function confirm(page: Page, label: string) {
  await page
    .getByRole("dialog")
    .getByRole("button", { name: label, exact: true })
    .click();
  await expect(page.getByRole("dialog")).toBeHidden();
}

test("admin edits persist through selection, empty-group drops and immutable version changes", async ({
  page,
}) => {
  test.setTimeout(180_000);
  await signInAsAdmin(page);
  await page.goto("/admin/tests");
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).first().click();
  await expect(page).toHaveURL(/\/admin\/tests\/[0-9a-f-]+\/edit$/);
  const id = page.url().split("/").at(-2);
  const title = `Admin CR ${Date.now()}`;
  await page.getByLabel("Tên đề thi").fill(title);
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await addQuestion(page, "First saved prompt");
  await page.getByLabel("Thẻ", { exact: true }).fill("Ngữ pháp CR");
  await page.getByLabel("Thẻ", { exact: true }).press("Enter");
  await addQuestion(page, "Second prompt");
  await page.getByLabel("Thẻ", { exact: true }).fill("ngu phap cr");
  await expect(
    page.getByRole("button", { name: "Ngữ pháp CR", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Ngữ pháp CR", exact: true }).click();
  await page.getByRole("button", { name: "First saved prompt", exact: true }).click();
  await expect(page.getByLabel("Nội dung câu hỏi", { exact: true })).toHaveValue(
    "First saved prompt",
  );
  await page
    .getByLabel("Nội dung câu hỏi", { exact: true })
    .fill("First updated promptly");
  await page.getByRole("button", { name: "Second prompt", exact: true }).click();
  await expect(page.getByLabel("Nội dung câu hỏi", { exact: true })).toHaveValue(
    "Second prompt",
  );
  await page
    .getByRole("button", { name: "First updated promptly", exact: true })
    .click();
  await expect(page.getByLabel("Nội dung câu hỏi", { exact: true })).toHaveValue(
    "First updated promptly",
  );

  await page.getByText("Phần 1", { exact: true }).dblclick();
  await page.getByLabel("Tên phần").fill("Grammar");
  await page.getByLabel("Tên phần").press("Enter");
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  const target = page.getByText("Kéo câu hỏi vào đây hoặc thêm câu hỏi mới.", {
    exact: true,
  });
  const handle = page.getByRole("button", {
    name: "Kéo để đổi vị trí câu 2",
    exact: true,
  });
  const from = await handle.boundingBox();
  const to = await target.boundingBox();
  expect(from).not.toBeNull();
  expect(to).not.toBeNull();
  if (!from || !to) throw new Error("Missing drag target");
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width / 2, to.y + to.height / 2, { steps: 20 });
  await page.mouse.up();
  await expect(target).toBeHidden();
  await expect(page.getByText(/Đã lưu \d\d:\d\d/)).toBeVisible();
  await page.reload();
  const secondSection = page
    .locator("[data-outline-section]")
    .filter({ hasText: "Phần 2" });
  await expect(
    secondSection.getByRole("button", { name: "Second prompt", exact: true }),
  ).toBeVisible();
  await publish(page, 1);
  await page.getByRole("link", { name: "Mở trình soạn đề", exact: true }).click();
  await page
    .getByRole("button", { name: "First updated promptly", exact: true })
    .click();
  await page
    .getByLabel("Nội dung câu hỏi", { exact: true })
    .fill("Second version content");
  await publish(page, 2);

  const history = page.getByRole("complementary", { name: "Lịch sử phiên bản" });
  const firstVersion = history
    .getByRole("listitem")
    .filter({ has: page.getByText("v1", { exact: true }) });
  await firstVersion.getByRole("button", { name: /^v1/ }).click();
  await expect(page.getByText("First updated promptly", { exact: true })).toBeVisible();
  await expect(page.getByText("Second version content", { exact: true })).toBeHidden();
  await firstVersion
    .getByRole("button", { name: "Đặt làm mặc định", exact: true })
    .click();
  await confirm(page, "Đặt làm mặc định");
  await expect(firstVersion.getByText("Mặc định", { exact: true })).toBeVisible();
  await page.goto(`/admin/assignments/new?testId=${id}`);
  await expect(page.getByText(/Bài giao gắn với bản v1/)).toBeVisible();
  await expect(page.getByLabel("Số lượt làm", { exact: true })).toHaveValue("1");
  await page.getByLabel("Thời lượng làm bài", { exact: true }).click();
  await expect(page.getByRole("option")).toHaveCount(4);
  await page.getByRole("option", { name: "Nhập thời lượng khác", exact: true }).click();
  await page.getByLabel("Thời lượng tự nhập (phút)").fill("65");
  await page.getByLabel("Số lượt làm", { exact: true }).fill("2");
  await page.getByLabel("Số lần rời trang cho phép", { exact: true }).click();
  await expect(page.getByRole("option")).toHaveCount(3);
  await page.getByRole("option", { name: "Không được phép", exact: true }).click();
  await page.getByLabel("Khi vượt quá", { exact: true }).click();
  await page
    .getByRole("option", { name: "Tự nộp ngay, ghi nhận vi phạm", exact: true })
    .click();
  await expect(page.getByLabel("Khi vượt quá", { exact: true })).toBeFocused();
  await page.getByPlaceholder("thêm lớp").fill("Tiếng Anh");
  await page
    .getByRole("listbox")
    .getByRole("option", { name: /Tiếng Anh giao tiếp/ })
    .first()
    .click();
  const created = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/admin/assignments"),
  );
  await page.getByRole("button", { name: "Giao bài", exact: true }).click();
  const assignment = (await (await created).json()) as { id: string };
  await expect(page).toHaveURL(/\/admin\/assignments$/);
  await page.goto(`/admin/assignments/${assignment.id}/edit`);
  await expect(page.getByLabel("Thời lượng tự nhập (phút)")).toHaveValue("65");
  await expect(page.getByLabel("Số lượt làm", { exact: true })).toHaveValue("2");
  await expect(
    page.getByLabel("Số lần rời trang cho phép", { exact: true }),
  ).toHaveText("Không được phép");
  await expect(page.getByLabel("Khi vượt quá", { exact: true })).toHaveText(
    "Tự nộp ngay, ghi nhận vi phạm",
  );

  await page.goto(`/admin/tests/${id}`);
  const secondVersion = history
    .getByRole("listitem")
    .filter({ has: page.getByText("v2", { exact: true }) });
  await secondVersion
    .getByRole("button", { name: "Xoá phiên bản", exact: true })
    .click();
  await confirm(page, "Xoá phiên bản");
  await expect(history.getByText("v2", { exact: true })).toBeHidden();
  await firstVersion
    .getByRole("button", { name: "Sửa từ phiên bản này", exact: true })
    .click();
  await confirm(page, "Sửa từ phiên bản này");
  await expect(page).toHaveURL(/\/edit$/);
  await page
    .getByRole("button", { name: "First updated promptly", exact: true })
    .click();
  await expect(page.getByLabel("Nội dung câu hỏi", { exact: true })).toHaveValue(
    "First updated promptly",
  );
  await publish(page, 3);

  await page.goto("/admin/tests");
  await page.getByPlaceholder("Tìm theo tên đề").fill(title);
  const original = page
    .getByRole("row")
    .filter({ has: page.locator(`a[href="/admin/tests/${id}"]`) });
  await expect(original).toBeVisible();
  await original
    .getByRole("button", { name: `Nhân bản ${title}`, exact: true })
    .click();
  await expect(page).toHaveURL(/\/admin\/tests\?q=/);
  await expect(page.getByText("Vừa nhân bản").first()).toBeVisible();
  const copies = page
    .getByRole("row")
    .filter({ hasText: title })
    .filter({ hasText: "Bản nháp" });
  await expect(copies).toHaveCount(1);
  await original
    .getByRole("button", { name: `Nhân bản ${title}`, exact: true })
    .click();
  await expect(copies).toHaveCount(2);
  await original.getByRole("link", { name: title, exact: true }).click();
  await expect(page).toHaveURL((url) => url.pathname === `/admin/tests/${id}`);
  await page.getByRole("button", { name: "Quay lại", exact: true }).click();
  await expect(page.getByPlaceholder("Tìm theo tên đề")).toHaveValue(title);
  for (const checkbox of await copies.getByRole("checkbox").all())
    await checkbox.check();
  await page
    .getByRole("button", { name: "Lưu trữ các mục đã chọn", exact: true })
    .click();
  await confirm(page, "Xác nhận 2 mục");
  await page.getByRole("tab", { name: /^Lưu trữ/ }).click();
  await expect(page.getByRole("row").filter({ hasText: title })).toHaveCount(2);
  await page
    .getByRole("checkbox", { name: "Chọn tất cả mục trên trang này", exact: true })
    .check();
  await page.getByRole("button", { name: "Xoá vĩnh viễn", exact: true }).click();
  await confirm(page, "Xác nhận 2 mục");
  await expect(page.getByRole("row").filter({ hasText: title })).toHaveCount(0);
  await page.goto("/admin/settings");
  await page.getByRole("button", { name: /^Tài khoản của/ }).click();
  await page.getByRole("menuitem", { name: "Đăng xuất", exact: true }).click();
  await expect(page).toHaveURL((url) => url.pathname === "/login");
  await signInAsStudent(page);
  await startAttempt(page, assignment.id);
  await expect(page.getByText("First updated promptly", { exact: true })).toBeVisible();
  await chooseOption(page, "Lựa chọn 1");
  await goAway(page, 3500);
  await expect(
    page.getByRole("heading", {
      name: "Bài đã được nộp vì rời trang lần nữa.",
      exact: true,
    }),
  ).toBeVisible({ timeout: 15_000 });
  await expect(
    page.getByText(
      "Bài đã được tự nộp do vượt giới hạn rời trang. Câu trả lời được giữ để chấm và vi phạm đã được ghi nhận.",
      { exact: true },
    ),
  ).toBeVisible();
});
