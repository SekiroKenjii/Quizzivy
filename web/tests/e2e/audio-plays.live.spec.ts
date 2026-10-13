import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import {
  assignToClass,
  publishInBuilder,
  signInAsAdmin,
  signInAsStudent,
} from "./support/live";

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

  await page.getByLabel("Chọn tệp từ máy").setInputFiles(AUDIO);
  // The upload is a real round trip through the API and object storage, and a
  // rejection renders as an alert rather than as a slow success.
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
  // §11.1's default, and exactly the allowance this test needs.
  await expect(
    page
      .getByRole("radiogroup", { name: "Số lần nghe" })
      .getByRole("radio", { name: "Hai lần" }),
  ).toBeChecked();

  for (const [index, text] of ["Gọi lại sau", "Đổi lịch hẹn"].entries()) {
    await page
      .getByPlaceholder(`Lựa chọn ${String.fromCharCode(65 + index)}`)
      .fill(text);
  }
  await page.getByLabel("Đánh dấu A là đáp án đúng", { exact: true }).check();

  await expect(page.locator('[role="status"][data-state="saved"]')).toBeVisible({
    timeout: 15_000,
  });
  await publishInBuilder(page);
  await expect(page).toHaveURL(/\/teacher\/tests\/[0-9a-f-]+$/, { timeout: 30_000 });
}

function playedTo(page: Page) {
  return page
    .locator("audio")
    .evaluate((element: HTMLAudioElement) => element.currentTime);
}

async function addChoiceQuestion(page: Page, prompt: string) {
  await page.getByLabel("Nội dung câu hỏi", { exact: true }).fill(prompt);
  for (const [index, text] of ["Rẽ trái", "Đi thẳng"].entries()) {
    await page
      .getByPlaceholder(`Lựa chọn ${String.fromCharCode(65 + index)}`)
      .fill(text);
  }
  await page.getByLabel("Đánh dấu A là đáp án đúng", { exact: true }).check();
}

async function publishGroupListeningTest(page: Page, title: string) {
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

  await page.getByRole("button", { name: "Thêm câu hỏi" }).click();
  await addChoiceQuestion(page, "Câu hỏi riêng, ngoài nhóm nghe.");
  await expect(page.locator('[role="status"][data-state="saved"]')).toBeVisible({
    timeout: 15_000,
  });

  await page
    .getByRole("button", { name: "Thêm nhóm câu dùng chung", exact: true })
    .click();
  await page.getByLabel("Tên nhóm câu hỏi", { exact: true }).fill("Hỏi đường");
  await page.getByRole("button", { name: "Thêm ngữ liệu", exact: true }).click();
  await page.getByLabel("Tên ngữ liệu", { exact: true }).fill("Hỏi đường");
  await page.getByRole("textbox", { name: "Nội dung ngữ liệu", exact: true }).click();
  await page
    .getByRole("group", { name: "Chèn nội dung vào ngữ liệu" })
    .getByRole("button", { name: "Bài nghe", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Chọn tệp từ máy").setInputFiles(AUDIO);
  await expect(dialog.getByText(/^Đã chọn: /)).toBeVisible({ timeout: 60_000 });
  await dialog.getByRole("button", { name: "Chèn vào ngữ liệu", exact: true }).click();
  await expect(dialog).toBeHidden();

  await page.getByRole("button", { name: "Thêm câu hỏi", exact: true }).last().click();
  await addChoiceQuestion(page, "Minh nên đi hướng nào?");
  await expect(page.locator('[role="status"][data-state="saved"]')).toBeVisible({
    timeout: 15_000,
  });
  await publishInBuilder(page);
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

    const card = page.getByRole("group", { name: "Đoạn ghi âm của câu hỏi" });
    const position = () => playedTo(page);
    await expect(card.getByText("Lượt nghe 1/2", { exact: true })).toBeVisible();

    await card.getByRole("button", { name: "Phát", exact: true }).click();
    await expect.poll(position).toBeGreaterThan(1);
    await card.getByRole("button", { name: "Tạm dừng", exact: true }).click();
    const stoppedAt = await position();
    await card.getByRole("button", { name: "Nghe tiếp", exact: true }).click();
    await expect.poll(position).toBeGreaterThan(stoppedAt);
    await expect(
      card.getByRole("button", { name: "Phát lại", exact: true }),
    ).toBeVisible({
      timeout: 20_000,
    });

    await page.reload();
    await expect(
      card.getByText("Đã nghe xong · còn 1 lượt", { exact: true }),
    ).toBeVisible({ timeout: 30_000 });

    await card.getByRole("button", { name: "Phát lại", exact: true }).click();
    await expect(card.getByText("Lượt nghe 2/2", { exact: true })).toBeVisible();
    await expect(page.getByText("Đã nghe 2/2 lượt")).toBeVisible({ timeout: 20_000 });

    // The whole point. The tab's own count goes with the reload, so a "2/2"
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

test("E2E 8b: a group's recording left mid-play resumes where it stopped, for no play", async ({
  browser,
}) => {
  test.setTimeout(240_000);
  const title = `E2E 8b — ${Date.now()}`;
  const teacher = await browser.newContext();
  const student = await browser.newContext();
  try {
    const admin = await teacher.newPage();
    await signInAsAdmin(admin);
    await publishGroupListeningTest(admin, title);
    await assignToClass(admin, title);

    const page = await student.newPage();
    const plays = new Set<string>();
    page.on("request", (request) => {
      if (request.method() === "POST" && request.url().endsWith("/group-audio-play")) {
        plays.add((request.postDataJSON() as { playId: string }).playId);
      }
    });
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

    const next = page.getByRole("button", { name: "Câu sau", exact: true });
    const back = page.getByRole("button", { name: "Câu trước", exact: true });
    const card = page.getByRole("article").getByRole("group");
    const position = () => playedTo(page);

    await next.click();
    await card.getByRole("button", { name: "Phát", exact: true }).click();
    await expect.poll(position).toBeGreaterThan(2);
    await back.click();
    await expect(page.locator("audio")).toHaveCount(0);
    await next.click();
    await expect(card.getByText("Đã tạm dừng", { exact: true })).toBeVisible();
    await expect.poll(position).toBeGreaterThan(2);
    const remembered = await position();
    await card.getByRole("button", { name: "Nghe tiếp", exact: true }).click();
    await expect.poll(position).toBeGreaterThan(remembered + 0.5);
    expect(plays.size).toBe(1);

    await page.reload();
    await expect(card.getByText("Lượt nghe 1/2", { exact: true })).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      card.getByText("Đã nghe xong · còn 1 lượt", { exact: true }),
    ).toBeVisible();
  } finally {
    await Promise.allSettled([teacher.close(), student.close()]);
  }
});
