import { expect, test, type Page } from "@playwright/test";
import {
  chooseOption,
  goAway,
  fillStepper,
  pickInWizard,
  signInAsAdmin,
  signInAsStudent,
  startAttempt,
} from "./support/live";

test.use({ actionTimeout: 10_000 });

async function addQuestion(page: Page, prompt: string) {
  await page
    .getByRole("button", { name: "Thêm câu hỏi vào Phần 1", exact: true })
    .click();
  const input = page.getByLabel("Nội dung câu hỏi", { exact: true });
  await expect(input).toHaveValue("Câu hỏi mới — nhập nội dung ở đây");
  await input.click();
  await expect(input).toHaveValue("");
  await input.fill(prompt);
}

async function publish(page: Page, version: number) {
  await page.getByRole("button", { name: "Phát hành", exact: true }).click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+$/);
  await expect(
    page.getByText(`Phiên bản ${version} · mặc định cho bài giao mới`),
  ).toBeVisible();
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
  await page.goto("/teacher/tests");
  await page.getByRole("button", { name: "Đề thi mới", exact: true }).first().click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);
  const id = page.url().split("/").at(-2);
  const title = `Admin CR ${Date.now()}`;
  await page.getByRole("button", { name: "Tên đề thi", exact: true }).click();
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).fill(title);
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("Enter");
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  await addQuestion(page, "First saved prompt");
  await page.getByRole("button", { name: "Cài đặt câu hỏi", exact: true }).click();
  const settings = page.getByRole("dialog", { name: "Cài đặt câu hỏi", exact: true });
  await expect(settings).toBeVisible();
  await page.getByLabel("Thẻ", { exact: true }).fill("Ngữ pháp CR");
  await page.getByLabel("Thẻ", { exact: true }).press("Enter");
  await page.keyboard.press("Escape");
  await expect(settings).toBeHidden();
  await addQuestion(page, "Second prompt");
  await page.getByRole("button", { name: "Cài đặt câu hỏi", exact: true }).click();
  await expect(settings).toBeVisible();
  await page.getByLabel("Thẻ", { exact: true }).fill("ngu phap cr");
  await expect(
    page.getByRole("button", { name: "Ngữ pháp CR", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Ngữ pháp CR", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(settings).toBeHidden();
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

  await page.getByRole("button", { name: /^Phần 1 \d+ · [\d.,]+đ$/ }).dblclick();
  await page.getByLabel("Tên phần").fill("Grammar");
  await page.getByLabel("Tên phần").press("Enter");
  await page.getByRole("button", { name: "Thêm phần", exact: true }).click();
  const target = page.getByText("Kéo câu hỏi vào đây", {
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
  await expect(page.locator('[role="status"][data-state="saved"]')).toBeVisible();
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

  await page.setViewportSize({ width: 1440, height: 900 });
  const history = page.getByRole("complementary", { name: "Lịch sử phiên bản" });
  const firstVersion = history.locator('[data-version="1"]');
  await firstVersion.getByRole("button", { name: /^Phiên bản 1/ }).click();
  await expect(page.getByText("First updated promptly", { exact: true })).toBeVisible();
  await expect(page.getByText("Second version content", { exact: true })).toBeHidden();
  await firstVersion
    .getByRole("button", { name: "Đặt làm mặc định", exact: true })
    .click();
  await confirm(page, "Đặt làm mặc định");
  await expect(firstVersion.getByText("Mặc định", { exact: true })).toBeVisible();
  await pickInWizard(page, title, `?test=${id}`);
  await page.getByRole("button", { name: "Tiếp tục", exact: true }).click();
  const timeLimit = page.getByRole("group", { name: "Thời gian làm bài" });
  await expect(timeLimit.getByRole("button")).toHaveCount(4);
  await timeLimit.getByRole("button", { name: "Tuỳ chỉnh", exact: true }).click();
  await fillStepper(page, "Thời gian làm bài", "65");
  const attempts = page.getByRole("spinbutton", { name: "Số lượt mỗi học viên" });
  await expect(attempts).toHaveAttribute("aria-valuenow", "1");
  await fillStepper(page, "Số lượt mỗi học viên", "2");
  await page.getByRole("button", { name: "Tiếp tục", exact: true }).click();
  const leaving = page.getByRole("group", { name: "Rời khỏi bài" });
  await expect(leaving.getByRole("button")).toHaveCount(3);
  await leaving.getByRole("button", { name: "Không được phép", exact: true }).click();
  await page
    .getByRole("group", { name: "Khi vượt giới hạn" })
    .getByRole("button", { name: "Tự nộp bài", exact: true })
    .click();
  const created = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/teacher/assignments"),
  );
  await page.getByRole("button", { name: /^Giao (cho \d+ học viên|bài)$/ }).click();
  const assignment = (await (await created).json()) as {
    id: string;
    testVersion: number;
  };
  expect(assignment.testVersion, "the default version is the one assigned").toBe(1);
  await expect(page).toHaveURL(/\/teacher\/assignments\?status=(open|scheduled)$/);
  await page.goto(`/teacher/assignments/${assignment.id}/edit?step=3`);
  await expect(
    page.getByRole("spinbutton", { name: "Thời gian làm bài" }),
  ).toHaveAttribute("aria-valuenow", "65");
  await expect(
    page.getByRole("spinbutton", { name: "Số lượt mỗi học viên" }),
  ).toHaveAttribute("aria-valuenow", "2");
  await page.goto(`/teacher/assignments/${assignment.id}/edit?step=4`);
  await expect(
    page
      .getByRole("group", { name: "Rời khỏi bài" })
      .getByRole("button", { name: "Không được phép", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(
    page
      .getByRole("group", { name: "Khi vượt giới hạn" })
      .getByRole("button", { name: "Tự nộp bài", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");

  await page.goto(`/teacher/tests/${id}`);
  const secondVersion = history.locator('[data-version="2"]');
  await secondVersion.getByRole("button", { name: "Xoá", exact: true }).click();
  await confirm(page, "Xoá phiên bản");
  await expect(secondVersion).toHaveCount(0);
  await firstVersion
    .getByRole("button", { name: "Khôi phục thành bản nháp", exact: true })
    .click();
  await confirm(page, "Khôi phục thành bản nháp");
  await expect(page).toHaveURL(/\/edit$/);
  await page
    .getByRole("button", { name: "First updated promptly", exact: true })
    .click();
  await expect(page.getByLabel("Nội dung câu hỏi", { exact: true })).toHaveValue(
    "First updated promptly",
  );
  await publish(page, 3);

  await page.goto("/teacher/tests");
  await page.getByPlaceholder("Tìm theo tên đề").fill(title);
  const cards = page
    .getByRole("list", { name: "Đề thi", exact: true })
    .getByRole("listitem");
  const original = cards.filter({
    has: page.locator(`a[href="/teacher/tests/${id}"]`),
  });
  await expect(original).toBeVisible();
  await original
    .getByRole("button", { name: `Nhân bản ${title}`, exact: true })
    .click();
  await expect(page).toHaveURL(/\/teacher\/tests\?q=/);
  await expect(page.getByText("Vừa nhân bản").first()).toBeVisible();
  const copies = cards.filter({ hasText: title }).filter({ hasText: "Bản nháp" });
  await expect(copies).toHaveCount(1);
  await original
    .getByRole("button", { name: `Nhân bản ${title}`, exact: true })
    .click();
  await expect(copies).toHaveCount(2);
  await original.getByRole("link", { name: title, exact: true }).click();
  await expect(page).toHaveURL((url) => url.pathname === `/teacher/tests/${id}`);
  await page
    .locator('[data-slot="page-head"]')
    .getByRole("link", { name: "Đề thi", exact: true })
    .click();
  await expect(page.getByPlaceholder("Tìm theo tên đề")).toHaveValue(title);
  await expect(copies).toHaveCount(2);
  for (const copy of await copies.all()) await copy.getByRole("checkbox").check();
  await expect(
    page.getByRole("button", { name: "Lưu trữ các mục đã chọn", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Lưu trữ các mục đã chọn", exact: true })
    .click();
  await confirm(page, "Xác nhận 2 mục");
  const archived = cards.filter({ hasText: title }).filter({ hasText: "Đã lưu trữ" });
  await expect(archived).toHaveCount(2);
  for (const copy of await archived.all()) await copy.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Xoá vĩnh viễn", exact: true }).click();
  await confirm(page, "Xác nhận 2 mục");
  await expect(archived).toHaveCount(0);
  await expect(cards.filter({ hasText: title })).toHaveCount(1);
  await page.goto("/teacher/settings");
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
      "Bạn đã rời trang làm bài quá số lần được phép nên bài đã được nộp. Câu trả lời của bạn được giữ lại để chấm. Giáo viên đã được báo.",
      { exact: true },
    ),
  ).toBeVisible();
});
