import { expect, test } from "@playwright/test";
import {
  ASSIGNMENT,
  chooseOption,
  freshAttempt,
  signInAsStudent,
  startAttempt,
} from "./support/live";

/**
 * E2E 7 (§16, phase-3 exit criterion): the same attempt opened twice. The
 * newer sitting owns it and the older one becomes read-only rather than both
 * writing over each other.
 *
 * Two browser contexts, because the access token lives in memory and two pages
 * in one context would share the session the test is trying to split.
 */
test("E2E 7: a second device takes the attempt over and the first goes read-only", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const first = await browser.newContext();
  const second = await browser.newContext();

  try {
    const one = await first.newPage();
    await signInAsStudent(one);
    const attemptId = await freshAttempt(one, ASSIGNMENT.takeover);

    const two = await second.newPage();
    await signInAsStudent(two);
    const resumed = await startAttempt(two, ASSIGNMENT.takeover);
    expect(resumed).toBe(attemptId);

    // The first tab finds out on its next write, not by polling, so the test
    // has to make it write.
    await chooseOption(one, "went");
    await expect(
      one.getByText("Bài này đang mở ở thiết bị khác. Bạn không thể sửa ở đây nữa."),
    ).toBeVisible({ timeout: 30_000 });

    // The newer sitting is untouched.
    await chooseOption(two, "gone");
    await expect(two.getByText(/^Đã lưu /)).toBeVisible({ timeout: 30_000 });
  } finally {
    await first.close();
    await second.close();
  }
});
