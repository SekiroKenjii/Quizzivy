import { expect, test } from "@playwright/test";
import { ASSIGNMENT, freshAttempt, signInAsStudent } from "./support/live";

/**
 * E2E 5 (§16, phase-3 exit criterion): the deadline passes and the attempt
 * submits itself.
 *
 * The fixture's duration is one minute, the contract's floor, and the deadline
 * is min(startedAt + duration, closesAt) — so this is the shortest expiry the
 * product can express and the wait is real rather than simulated.
 */
test("E2E 5: the timer runs out and the attempt submits without the student", async ({
  page,
}) => {
  test.setTimeout(180_000);
  await signInAsStudent(page);
  const attemptId = await freshAttempt(page, ASSIGNMENT.timer);

  await expect(page.getByRole("button", { name: "Thoát" })).toBeVisible();

  // Nothing is clicked from here on. The engine arms one timeout from the
  // server's clock, and the only thing that ends the attempt is that timeout.
  await expect(
    page.getByRole("heading", { name: "Bài đã hết giờ và được nộp tự động." }),
  ).toBeVisible({ timeout: 120_000 });
  await expect(page.getByRole("button", { name: "Thoát" })).toBeHidden();

  // Reopening it proves the server took the submission rather than the client
  // merely navigating away.
  await page.goto(`/app/attempts/${attemptId}`);
  await expect(page.getByText("Bài làm này đã kết thúc.")).toBeVisible();
});
