import { expect, test } from "@playwright/test";
import {
  ASSIGNMENT,
  chooseOption,
  freshAttempt,
  signInAsStudent,
} from "./support/live";

/**
 * E2E 2's middle (T-3.16): answers survive a reload mid-test.
 *
 * §1.2's first goal is a student completing a test "without losing work on
 * refresh, tab close, or brief network loss", so this is the phase's headline
 * promise. T-3.16 asked for a manual pass because the full E2E 2 needs Phase
 * 4's result page; the half that does not need it is worth automating.
 */
test("E2E 2 (middle): an answer given before a reload is still there after it", async ({
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
  await expect(page.getByRole("textbox", { name: "Bài làm của bạn" })).toHaveValue(
    "",
  );
});
