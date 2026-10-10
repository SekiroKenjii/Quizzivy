import { fileURLToPath } from "node:url";
import { expect, test, type Page, type Response } from "@playwright/test";
import type { Monitor } from "../../src/features/attempts/api";
import { assignToClass } from "./support/live";

/**
 * E2E 1 (§14): the teacher logs in, authors a test with one question of each
 * of §7's five types including audio, publishes it, and assigns it. Phase 2
 * ran the half up to publishing as 1a; Phase 4 closes it with the assignment.
 */

const ADMIN = { email: "thuong@quizzivy.com", password: "quizzivy-dev" };
const AUDIO = fileURLToPath(new URL("./fixtures/unit5-listening.mp3", import.meta.url));

async function signIn(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(ADMIN.email);
  await page.getByLabel("Mật khẩu", { exact: true }).fill(ADMIN.password);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
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
  await expect(page.getByLabel("Nội dung câu hỏi", { exact: true })).toHaveValue(
    "Câu hỏi mới — nhập nội dung ở đây",
  );
  await page.getByRole("tab", { name: type }).click();
  await expect(page.getByRole("tab", { name: type })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  await page.getByLabel("Nội dung câu hỏi", { exact: true }).click();
  await page.getByLabel("Nội dung câu hỏi", { exact: true }).fill(prompt);
}

test("E2E 1: an admin authors a test with all five question types, publishes and assigns it", async ({
  page,
}) => {
  test.setTimeout(120_000);
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
  await addQuestion(page, "Điền từ", "She {{1}} in Hanoi since 2019.");
  await page.getByRole("button", { name: "Thêm chỗ trống" }).click();
  await page.getByLabel("Đáp án được chấp nhận").fill("has lived");

  // --------------------------------------------------------- short_answer
  await addQuestion(page, "Tự luận", "Viết 2–3 câu tả thói quen buổi sáng của bạn.");
  await page.getByLabel("Đáp án mẫu").fill("I usually wake up at six.");

  // ------------------------------------------ audio, with a real upload
  await addQuestion(page, "Một đáp án", "Người phụ nữ đề nghị làm gì?");
  await page.getByLabel("Chọn tệp từ máy").setInputFiles(AUDIO);

  await expect(async () => {
    const rejected = page
      .getByRole("alert")
      .filter({ hasText: "Không dùng được tệp này" });
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
  await page.getByRole("button", { name: "Phát hành" }).click();

  // Publishing lands on the detail page, previewing the version just written.
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+$/, { timeout: 30_000 });
  await expect(page.getByText("Phiên bản 1 · mặc định cho bài giao mới")).toBeVisible();
  await expect(page.getByRole("heading", { name: title })).toBeVisible();

  // Six questions, one of each type plus the audio one, in the student payload.
  await expect(page.getByText("They ___ to the museum last weekend.")).toBeVisible();
  await expect(page.getByText("Người phụ nữ đề nghị làm gì?")).toBeVisible();

  // And the version history records it: six questions, six points, by name.
  // At 1280 the history is the sheet the header opens.
  await page.getByRole("button", { name: "Lịch sử phiên bản" }).click();
  const history = page.getByRole("dialog", { name: "Lịch sử phiên bản" });
  const versionOne = history.locator('[data-version="1"]');
  await expect(versionOne.getByText("Mặc định", { exact: true })).toBeVisible();
  await expect(versionOne.getByText("6 câu · 6 điểm")).toBeVisible();
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
