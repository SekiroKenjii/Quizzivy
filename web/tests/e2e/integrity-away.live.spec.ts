import { expect, test } from "@playwright/test";
import {
  ASSIGNMENT,
  chooseOption,
  freshAttempt,
  goAway,
  signInAsStudent,
} from "./support/live";

/**
 * E2E 6 (§16, phase-3 exit criterion): leaving the page is noticed, said out
 * loud, and survives on the server.
 *
 * The fixture sets minAwayMs to 0 and maxFocusLoss to 1, so one episode counts
 * and the dialog appears on it.
 */
test("E2E 6: leaving the page warns the student and the count comes back from the server", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signInAsStudent(page);
  const attemptId = await freshAttempt(page, ASSIGNMENT.integrity);

  // The allowance before anything happens, so the assertion after the reload
  // is comparing two different states rather than matching a constant.
  await expect(page.getByText("Còn 1 lần rời trang")).toBeVisible();

  await goAway(page, 500);

  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Bạn vừa rời khỏi trang làm bài")).toBeVisible();
  // §10.2: the dialog states the consequence and the timer keeps running.
  await expect(dialog.getByText(/Đồng hồ vẫn đang chạy/)).toBeVisible();
  await dialog.getByRole("button", { name: "Tiếp tục làm bài" }).click();
  await expect(dialog).toBeHidden();

  // An answer, so the buffered events flush with the autosave batch.
  await chooseOption(page, "went");
  await expect(page.getByText(/^Đã lưu /)).toBeVisible({ timeout: 30_000 });

  // The reload is the point: this tab's own count is gone, so the spent
  // allowance it shows afterwards was read back from the server.
  await page.reload();
  await expect(page).toHaveURL(new RegExp(`/app/attempts/${attemptId}$`));
  await expect(page.getByText("Hết lần rời trang")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText("Còn 1 lần rời trang")).toBeHidden();
});
