import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
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
  await page.getByLabel("Mật khẩu").fill(ADMIN.password);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(/\/admin$/);
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
    await page.getByPlaceholder(`Lựa chọn ${index + 1}`).fill(text);
  }
  await page.getByLabel("Lựa chọn 1", { exact: true }).check();
}

/** Adds one question of `type` to the open builder and fills in its answer. */
async function addQuestion(page: Page, type: string, prompt: string) {
  await page.getByRole("button", { name: "Thêm câu hỏi" }).click();
  await expect(page.getByRole("tab", { name: type })).toBeVisible();
  await page.getByRole("tab", { name: type }).click();

  await page.getByLabel("Nội dung câu hỏi").click();
  await page.getByLabel("Nội dung câu hỏi").fill(prompt);
}

test("E2E 1: an admin authors a test with all five question types, publishes and assigns it", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signIn(page);

  // ---------------------------------------------------------------- create
  await page.goto("/admin/tests");
  await page.getByRole("button", { name: "Đề thi mới" }).first().click();
  await expect(page).toHaveURL(/\/admin\/tests\/[0-9a-f-]+\/edit$/);

  const title = `E2E 1a — ${Date.now()}`;
  await page.getByLabel("Tên đề thi").fill(title);
  await page.getByRole("button", { name: "Thêm phần" }).click();
  await expect(page.getByText("Phần 1")).toBeVisible();

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
    await expect(page.getByRole("button", { name: "Gỡ", exact: true })).toBeVisible({
      timeout: 1_000,
    });
  }).toPass({ timeout: 60_000 });
  await expect(page.getByText("unit5-listening.mp3")).toBeVisible();

  // The server sniffed the bytes and measured the duration.
  await expect(page.getByText(/0:10 · /)).toBeVisible();
  await expect(page.getByText("0:00 / 0:10")).toBeVisible();

  // §11.1's defaults arrive with the asset, visibly.
  await expect(page.getByLabel("Số lần được nghe")).toHaveValue("2");
  await expect(page.getByRole("switch", { name: "Cho tua" })).not.toBeChecked();
  await expect(
    page.getByRole("switch", { name: "Hiện lời thoại sau khi nộp" }),
  ).toBeChecked();

  await setOptions(page, ["Gọi lại sau", "Đổi lịch hẹn"]);

  // --------------------------------------------------------------- publish
  await expect(page.getByText(/Đã lưu \d\d:\d\d/)).toBeVisible({ timeout: 15_000 });
  await page.getByRole("button", { name: "Phát hành" }).click();

  // Publishing lands on the detail page, previewing the version just written.
  await expect(page).toHaveURL(/\/admin\/tests\/[0-9a-f-]+$/, { timeout: 30_000 });
  await expect(page.getByText("Bản đang phát hành · v1")).toBeVisible();
  await expect(page.getByRole("heading", { name: title })).toBeVisible();

  // Six questions, one of each type plus the audio one, in the student payload.
  await expect(page.getByText("They ___ to the museum last weekend.")).toBeVisible();
  await expect(page.getByText("Người phụ nữ đề nghị làm gì?")).toBeVisible();

  // And the version history records it: six questions, six points, by name.
  const history = page.getByRole("region", { name: "Lịch sử phiên bản" });
  await expect(history.getByText("v1", { exact: true })).toBeVisible();
  await expect(history.getByText("6 · 6")).toBeVisible();
  await expect(history.getByText(/Thuong/)).toBeVisible();

  // ---------------------------------------------------------------- assign
  await assignToClass(page, title);
  const row = page.getByRole("row").filter({ hasText: title });
  await expect(row).toBeVisible();
  await expect(row.getByText("Đang mở")).toBeVisible();

  // The monitor lists the class, nobody started, and says it will keep looking.
  await row.getByRole("link", { name: title }).click();
  await expect(page).toHaveURL(/\/admin\/assignments\/[0-9a-f-]+$/);
  await expect(page.getByText("Tự cập nhật 15 giây/lần")).toBeVisible();
  await expect(page.getByText("Chưa bắt đầu").first()).toBeVisible();
});
