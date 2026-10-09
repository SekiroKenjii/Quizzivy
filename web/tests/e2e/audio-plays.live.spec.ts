import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { assignToClass, signInAsAdmin, signInAsStudent } from "./support/live";

/**
 * E2E 8 (§16, phase-3 exit criterion): the listening allowance is the server's
 * count, not the tab's.
 *
 * §11.4 does not block playback — "over-limit plays are reported to the
 * teacher, not enforced retroactively" — so what this proves is the thing that
 * section is actually about: the count survives a reload, which the obvious
 * client-side counter does not. The plan's wording for E2E 8 says "button
 * disabled"; that predates §11.4 being implemented and is recorded as a
 * deviation in docs/plan/13-phase-3.md.
 *
 * It authors its own paper because the count needs a real audio object behind
 * a real signed URL, and the seed has none.
 */

const AUDIO = fileURLToPath(new URL("./fixtures/unit5-listening.mp3", import.meta.url));

async function publishListeningTest(page: Page, title: string) {
  await page.goto("/teacher/tests");
  await page.getByRole("button", { name: "Đề thi mới" }).first().click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+\/edit$/);

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

  await page.getByRole("button", { name: "Thêm câu hỏi" }).click();
  await page
    .getByLabel("Nội dung câu hỏi", { exact: true })
    .fill("Người phụ nữ đề nghị làm gì?");

  await page.getByRole("button", { name: "Cài đặt câu hỏi", exact: true }).click();
  const settings = page.getByRole("dialog", { name: "Cài đặt câu hỏi", exact: true });
  await expect(settings).toBeVisible();
  await page.getByLabel("Chọn tệp từ máy").setInputFiles(AUDIO);
  // The upload is a real round trip through the API and object storage, and a
  // rejection renders as an alert rather than as a slow success.
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
  // §11.1's default, and exactly the allowance this test needs.
  await expect(page.getByLabel("Số lần được nghe")).toHaveText("2 lần");

  await page.keyboard.press("Escape");
  await expect(settings).toBeHidden();

  for (const [index, text] of ["Gọi lại sau", "Đổi lịch hẹn"].entries()) {
    await page.getByPlaceholder(`Lựa chọn ${index + 1}`).fill(text);
  }
  await page.getByLabel("Lựa chọn 1", { exact: true }).check();

  await expect(page.locator('[role="status"][data-state="saved"]')).toBeVisible({
    timeout: 15_000,
  });
  await page.getByRole("button", { name: "Phát hành" }).click();
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+$/, { timeout: 30_000 });
}

test("E2E 8: the listening count is the server's and survives a reload", async ({
  browser,
}) => {
  test.setTimeout(240_000);
  const title = `E2E 8 — ${Date.now()}`;

  const teacher = await browser.newContext();
  const student = await browser.newContext();
  let bodyFailed = false;
  let bodyError: unknown;
  let closeOutcomes: PromiseSettledResult<void>[] = [];
  try {
    const admin = await teacher.newPage();
    await signInAsAdmin(admin);
    await publishListeningTest(admin, title);
    await assignToClass(admin, title);

    const page = await student.newPage();
    await signInAsStudent(page);
    const paper = page.getByRole("main").getByRole("link").filter({ hasText: title });
    await expect(paper).toBeVisible({ timeout: 30_000 });
    await paper.click();
    await page.getByRole("button", { name: "Bắt đầu làm bài" }).click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Bắt đầu", exact: true })
      .click();
    await expect(page).toHaveURL(/\/app\/attempts\/[0-9a-f-]+$/);

    const play = page.getByRole("button", { name: "Phát" });
    await expect(page.getByText("Còn 2 lượt nghe")).toBeVisible();

    await play.click();
    await expect(page.getByText("Còn 1 lượt nghe")).toBeVisible();
    await page.getByRole("button", { name: "Tạm dừng" }).click();

    await play.click();
    await expect(page.getByText("Đã nghe 2/2 lượt")).toBeVisible();

    // The whole point. The tab's own count goes with the reload, so a "Còn 0"
    // that comes back was read from attempt_audio_plays.
    await page.reload();
    await expect(page.getByText("Đã nghe 2/2 lượt")).toBeVisible({ timeout: 30_000 });
  } catch (error) {
    bodyFailed = true;
    bodyError = error;
  } finally {
    closeOutcomes = await Promise.allSettled(
      [teacher, student].map(async (context) => {
        await context.close();
      }),
    );
  }
  for (const [index, outcome] of closeOutcomes.entries()) {
    test.info().annotations.push({
      type: "context-cleanup",
      description: `${index === 0 ? "teacher" : "student"}: ${outcome.status}${
        outcome.status === "rejected" ? `: ${String(outcome.reason)}` : ""
      }`,
    });
  }
  if (bodyFailed) throw bodyError;
  const failures: unknown[] = closeOutcomes.flatMap((outcome) =>
    outcome.status === "rejected" ? [outcome.reason] : [],
  );
  if (failures.length) throw new AggregateError(failures, "Context cleanup failed");
});
