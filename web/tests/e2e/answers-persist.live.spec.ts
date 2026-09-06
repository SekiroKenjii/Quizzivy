import { expect, test } from "@playwright/test";
import {
  ASSIGNMENT,
  chooseOption,
  freshAttempt,
  signInAsStudent,
  submitAttempt,
} from "./support/live";

/**
 * E2E 2 (§14): password login, start, answer, reload mid-test, the answer is
 * still there, submit, see the result.
 *
 * §1.2's first goal is a student completing a test "without losing work on
 * refresh, tab close, or brief network loss", so the reload in the middle is
 * the headline promise; Phase 3 ran that half, Phase 4 adds the result page.
 */
test("E2E 2: an answer survives a reload, and the result shows what was earned", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signInAsStudent(page);
  const attemptId = await freshAttempt(page, ASSIGNMENT.persistence);

  await chooseOption(page, "went");
  // Waited for, because a reload before the autosave lands would be testing
  // the debounce rather than the persistence.
  await expect(page.getByText(/^Đã lưu /)).toBeVisible({ timeout: 30_000 });

  await page.reload();
  await expect(page).toHaveURL(new RegExp(`/app/attempts/${attemptId}$`));

  await expect(page.getByRole("radio", { name: "went" })).toBeChecked();
  await expect(page.getByRole("radio", { name: "gone" })).not.toBeChecked();

  // The second question is still open, so the reload restored an answer rather
  // than a finished paper.
  await page.getByRole("button", { name: "Câu sau" }).click();
  await expect(page.getByRole("textbox", { name: "Bài làm của bạn" })).toHaveValue("");

  // ---------------------------------------------------------------- submit
  await submitAttempt(page);

  // ---------------------------------------------------------------- result
  // By URL: the fixture allows fifty attempts, so after one the home still
  // offers the assignment as due rather than filing it under completed.
  await page.goto(`/app/attempts/${attemptId}/result`);
  await expect(page.getByText("Điểm của bạn")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(/Lượt \d+\/50/)).toBeVisible();
  // The choice given before the reload is marked as the student's own.
  await expect(page.getByText("bạn chọn").first()).toBeVisible();
  await expect(page.getByText("went")).toBeVisible();
});
