import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";

/**
 * E2E 1a (§14, phase-2 exit criterion): the teacher logs in, authors a test
 * with one question of each of §7's five types including audio, and publishes
 * it. It stops before assigning, which is Phase 3.
 *
 * This is the ONE suite that talks to a real API. The audio question uploads an
 * actual mp3 from T-2.2's fixture corpus, so §11.1's magic-byte sniff, size
 * check and pure-Go duration probe all run — a mocked upload would only assert
 * that the frontend can display whatever the mock returned.
 *
 * Needs postgres, MinIO and the Go API up; `make up && make migrate && make
 * seed`, then the API on :8080. The API's CORS allowlist has to include
 * http://localhost:4173, which is `vite preview` -- see .env.example.
 *
 * Locally, kill any `vite preview` left over from a previous run before
 * re-running: `reuseExistingServer` is on outside CI, so Playwright will reuse
 * it and serve the build that server started with rather than rebuilding.
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
  // By label, not by role: single choice renders radios and multiple choice
  // renders checkboxes, and the test is about the option, not the widget.
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

test("E2E 1a: an admin authors a test with all five question types and publishes it", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signIn(page);

  // ---------------------------------------------------------------- create
  await page.goto("/admin/tests");
  await page.getByRole("button", { name: "Đề thi mới" }).click();
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

  // The server sniffed the bytes, measured the duration and stored the object.
  await expect(page.getByText("unit5-listening.mp3")).toBeVisible({ timeout: 30_000 });
  // §11.1's defaults arrive with the asset, visibly.
  await expect(page.getByLabel("Số lần được nghe")).toHaveValue("2");
  await expect(page.getByRole("switch", { name: "Cho tua" })).not.toBeChecked();

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
});
