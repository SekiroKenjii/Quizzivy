import { fileURLToPath } from "node:url";
import { expect, test, type Page, type Response } from "@playwright/test";
import type { Monitor } from "../../src/features/attempts/api";
import { assignToClass, publishInBuilder } from "./support/live";

/**
 * E2E 1 (§14): the teacher logs in, authors a test with one question of each
 * of §7's types including audio, publishes it, and assigns it. Phase 2 ran the
 * half up to publishing as 1a; Phase 4 closes it with the assignment. R6
 * (T-R6.14) adds the types it brought, authored through the editor: True /
 * False / Not given, "Choose N", a word limit, matching and an essay with the
 * IELTS rubric.
 */

const ADMIN = { email: "thuong@quizzivy.com", password: "quizzivy-dev" };
const AUDIO = fileURLToPath(new URL("./fixtures/unit5-listening.mp3", import.meta.url));

async function signIn(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(ADMIN.email);
  await page.getByLabel("Mật khẩu", { exact: true }).fill(ADMIN.password);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(/\/(admin|teacher)$/);
  await page.goto("/teacher");
  await expect(page).toHaveURL(/\/teacher$/);
}

/**
 * Rewrites the starter options and marks the first one correct.
 *
 * By placeholder rather than by value: the starter text is what the teacher is
 * replacing, so keying on it would make the test agree with a default instead
 * of with the field.
 */
async function setOptions(page: Page, texts: string[]) {
  for (const [index, text] of texts.entries()) {
    await page
      .getByPlaceholder(`Lựa chọn ${String.fromCharCode(65 + index)}`)
      .fill(text);
  }
  await page.getByLabel("Đánh dấu A là đáp án đúng", { exact: true }).check();
}

/** Adds one question of `type` to the open builder and fills in its answer. */
async function addQuestion(page: Page, type: string, prompt: string) {
  await page.getByRole("button", { name: "Thêm câu hỏi" }).click();
  const field = page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true });
  await expect(field).toHaveText("");
  await page.getByRole("button", { name: /^Loại câu hỏi: / }).click();
  await page.getByRole("menuitem", { name: new RegExp(`^${type}`) }).click();
  await expect(
    page.getByRole("button", { name: `Loại câu hỏi: ${type}`, exact: true }),
  ).toBeVisible();

  await field.click();
  await field.fill(prompt);
}

test("E2E 1: an admin authors a test with every question type, publishes and assigns it", async ({
  page,
}) => {
  test.setTimeout(240_000);
  await signIn(page);

  // ---------------------------------------------------------------- create
  await page.goto("/teacher/tests");
  await page.getByRole("button", { name: "Đề thi mới" }).first().click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);

  const title = `E2E 1a — ${Date.now()}`;
  await page.getByRole("button", { name: "Tên đề thi", exact: true }).click();
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).fill(title);
  await page.getByRole("textbox", { name: "Tên đề thi", exact: true }).press("Enter");
  await page.getByRole("button", { name: "Thêm phần" }).click();
  const sectionName = page.getByRole("textbox", { name: "Tên phần", exact: true });
  await expect(sectionName).toBeFocused();
  await sectionName.press("Enter");
  await expect(
    page.getByRole("button", { name: /^Phần 1 \d+ · [\d.,]+đ$/ }),
  ).toBeVisible();

  // ------------------------------------------------------- single_choice
  await addQuestion(page, "Một đáp án", "They ___ to the museum last weekend.");
  await setOptions(page, ["went", "have gone"]);

  // ----------------------------------------------------- multiple_choice
  await addQuestion(page, "Nhiều đáp án", "Chọn tất cả các câu đúng.");
  await setOptions(page, [
    "She has lived here since 2019.",
    "She lives here since 2019.",
  ]);

  // ----------------------------------------------------------- true_false
  await addQuestion(page, "Đúng/Sai", "“Since” đi với thì hiện tại hoàn thành.");

  // ----------------------------------------------------------- fill_blank
  await addQuestion(page, "Điền từ", "Since 2019 she has");
  await page
    .getByRole("textbox", { name: "Nội dung câu hỏi", exact: true })
    .press("End");
  await page
    .getByRole("button", { name: "Thêm chỗ trống", exact: true })
    .last()
    .click();
  const answer = page.getByRole("textbox", {
    name: "Đáp án được chấp nhận cho chỗ trống 1",
    exact: true,
  });
  await answer.fill("lived");
  await answer.press("Enter");

  // --------------------------------------------------------- short_answer
  await addQuestion(
    page,
    "Trả lời ngắn",
    "Viết 2–3 câu tả thói quen buổi sáng của bạn.",
  );
  await page.getByLabel("Đáp án mẫu").fill("I usually wake up at six.");
  // A word limit advises; it never cuts (§7).
  await page
    .getByRole("spinbutton", { name: "Giới hạn số từ", exact: true })
    .fill("60");

  // ------------------------------------------------- tfng (R6, T-R6.14)
  // A true / false question with "Not given" turned on is the IELTS kind.
  await addQuestion(page, "Đúng/Sai", "The museum opens on Mondays.");
  await page.getByRole("switch", { name: /^Thêm “Không có thông tin”/ }).click();
  await page
    .locator("label")
    .filter({ hasText: /^Không có thông tin$/ })
    .click();

  // ------------------------------------------------------------ Choose N
  await addQuestion(page, "Nhiều đáp án", "Which TWO words are verbs?");
  await page.getByRole("button", { name: "Thêm lựa chọn", exact: true }).click();
  await setOptions(page, ["swim", "blue", "run"]);
  // The visible box of the third row sits over the hidden input, so the row
  // is clicked as a teacher clicks it.
  await page
    .getByLabel("Đánh dấu C là đáp án đúng", { exact: true })
    .locator("xpath=..")
    .click();
  await expect(
    page.getByLabel("Đánh dấu C là đáp án đúng", { exact: true }),
  ).toBeChecked();
  await page.getByRole("switch", { name: "Học viên chọn đúng số đáp án" }).click();
  await expect(
    page.getByRole("spinbutton", { name: "Số đáp án học viên chọn", exact: true }),
  ).toHaveValue("2");

  // ------------------------------------------------------------- matching
  await addQuestion(page, "Nối", "Match each paragraph with its heading.");
  await page.getByRole("textbox", { name: "Mục A", exact: true }).fill("Paragraph A");
  await page.getByRole("textbox", { name: "Mục B", exact: true }).fill("Paragraph B");
  await page
    .getByRole("textbox", { name: "Lựa chọn i", exact: true })
    .fill("A river with nine mouths");
  await page
    .getByRole("textbox", { name: "Lựa chọn ii", exact: true })
    .fill("Salt water moving inland");
  await page.getByRole("button", { name: "Thêm lựa chọn", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Lựa chọn iii", exact: true })
    .fill("Markets on the water");
  for (const [item, choice] of [
    ["A", "i"],
    ["B", "ii"],
  ] as const) {
    await page
      .getByRole("combobox", { name: `Lựa chọn đúng cho mục ${item}`, exact: true })
      .click();
    await page.getByRole("option", { name: new RegExp(`^${choice} `) }).click();
  }

  // ------------------------------------------------- essay, with a rubric
  await addQuestion(
    page,
    "Bài luận",
    "Some people think rivers matter more than roads. Discuss.",
  );
  await page
    .getByRole("button", { name: "Dùng tiêu chí IELTS Writing", exact: true })
    .click();
  await expect(
    page.getByRole("textbox", { name: "Tiêu chí 4", exact: true }),
  ).toHaveValue("Độ đa dạng và chính xác ngữ pháp");

  // ------------------------------------------ audio, with a real upload
  await addQuestion(page, "Một đáp án", "Người phụ nữ đề nghị làm gì?");
  await page.getByLabel("Chọn tệp từ máy").setInputFiles(AUDIO);

  await expect(async () => {
    const rejected = page
      .getByRole("alert")
      .filter({ hasText: /Không dùng được tệp này|Máy chủ gặp lỗi khi nhận tệp/ });
    if (await rejected.isVisible()) {
      throw new Error(`upload rejected: ${await rejected.innerText()}`);
    }
    await expect(
      page.getByRole("button", { name: "Gỡ media", exact: true }),
    ).toBeVisible({ timeout: 1_000 });
  }).toPass({ timeout: 60_000 });
  await expect(page.getByText("unit5-listening.mp3", { exact: true })).toBeVisible();

  // The server sniffed the bytes and measured the duration.
  await expect(page.getByText(/0:10 · /)).toBeVisible();
  await expect(page.getByText("0:00 / 0:10")).toBeVisible();

  // §11.1's defaults arrive with the asset, visibly.
  await expect(
    page
      .getByRole("radiogroup", { name: "Số lần nghe" })
      .getByRole("radio", { name: "Hai lần" }),
  ).toBeChecked();
  await expect(page.getByRole("switch", { name: "Cho tua tới" })).not.toBeChecked();
  await expect(
    page.getByRole("switch", { name: "Hiện lời thoại sau khi nộp" }),
  ).toBeChecked();

  await setOptions(page, ["Gọi lại sau", "Đổi lịch hẹn"]);

  // --------------------------------------------------------------- publish
  await expect(page.locator('[role="status"][data-state="saved"]')).toBeVisible({
    timeout: 15_000,
  });
  await publishInBuilder(page);

  // The test's page previews the version just written.
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+$/, { timeout: 30_000 });
  await expect(page.getByText("Phiên bản 1 · mặc định cho bài giao mới")).toBeVisible();
  await expect(page.getByRole("heading", { name: title })).toBeVisible();

  // Ten questions, one of each type plus Choose N and the audio one, in the
  // student payload. The preview is the engine, one question at a time: the
  // first opens it, and nine steps reach the tenth, the last, with its
  // recording.
  const paper = page.getByRole("region", { name: "Màn hình của học viên" });
  await expect(
    paper.getByText("They chỗ trống to the museum last weekend.", { exact: true }),
  ).toBeVisible();
  await expect(paper.getByText(/^Câu 1 trên 10 · /)).toBeVisible();
  // Each of R6's types arrives as the engine draws it.
  const drawn: Record<number, () => Promise<void>> = {
    6: () =>
      expect(paper.getByRole("radio", { name: "Không có thông tin" })).toBeVisible(),
    7: () => expect(paper.getByText("Chọn hai đáp án.")).toBeVisible(),
    8: () => expect(paper.getByText("Đã nối 0/2")).toBeVisible(),
    9: () =>
      expect(paper.getByRole("textbox", { name: "Bài luận của bạn" })).toBeVisible(),
  };
  for (let n = 2; n <= 10; n += 1) {
    await paper.getByRole("button", { name: "Câu sau", exact: true }).click();
    await drawn[n]?.();
  }
  await expect(paper.getByText(/^Câu 10 trên 10 · /)).toBeVisible();
  await expect(paper.getByText("Người phụ nữ đề nghị làm gì?")).toBeVisible();
  await expect(
    paper.getByRole("group", { name: "Đoạn ghi âm của câu hỏi" }),
  ).toBeVisible();
  await expect(
    paper.getByRole("button", { name: "Hoàn tất", exact: true }),
  ).toBeVisible();

  // And the version history records it: ten questions, eighteen points (the
  // essay is worth nine by default), by name.
  // At 1280 the history is the sheet the header opens.
  await page.getByRole("button", { name: "Lịch sử phiên bản" }).click();
  const history = page.getByRole("dialog", { name: "Lịch sử phiên bản" });
  const versionOne = history.locator('[data-version="1"]');
  await expect(versionOne.getByText("Mặc định", { exact: true })).toBeVisible();
  await expect(versionOne.getByText("10 câu · 18 điểm")).toBeVisible();
  await expect(versionOne.getByText(/Thuong/)).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(history).toBeHidden();

  // ---------------------------------------------------------------- assign
  await assignToClass(page, title);
  const row = page.getByRole("row").filter({ hasText: title });
  await expect(row).toBeVisible();
  await expect(row.getByText("Đang mở")).toBeVisible();

  const link = row.getByRole("link", { name: title });
  const href = await link.getAttribute("href");
  expect(href).toMatch(/^\/teacher\/assignments\/[0-9a-f-]+$/);
  const monitorPath = `${href}/attempts`;
  const isMonitor = (response: Response) =>
    response.request().method() === "GET" &&
    new URL(response.url()).pathname === monitorPath;
  const firstRead = page.waitForResponse(isMonitor);
  await link.click();
  await expect(page).toHaveURL(/\/teacher\/assignments\/[0-9a-f-]+$/);
  await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
  await expect(
    page.getByRole("tab", { name: "Học viên", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await expect(
    page.locator('[data-slot="page-head"]').getByText(/Tiếng Anh giao tiếp — Lớp A/),
  ).toBeVisible();
  const first = await firstRead;
  expect(first.status()).toBe(200);
  const initial: Monitor = await first.json();
  expect(initial.rows.length).toBeGreaterThan(0);
  expect(
    initial.rows.every(
      (student) => student.state === "not_started" && !student.attemptId,
    ),
  ).toBe(true);
  const studentRow = page
    .getByRole("row")
    .filter({ hasText: initial.rows[0]!.fullName });
  await expect(studentRow).toBeVisible();
  await expect(studentRow.getByText("Chưa bắt đầu", { exact: true })).toBeVisible();
  const next = await page.waitForResponse(isMonitor, { timeout: 20_000 });
  expect(next.status()).toBe(200);
  const refreshed: Monitor = await next.json();
  expect(refreshed.rows.map((student) => [student.studentId, student.state])).toEqual(
    initial.rows.map((student) => [student.studentId, student.state]),
  );
  await expect(studentRow.getByText("Chưa bắt đầu", { exact: true })).toBeVisible();
});
