import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import type { components } from "../../src/lib/api/schema";

const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const STAMP = "2026-10-01T00:00:00Z";

const draft: components["schemas"]["Test"] = {
  skills: [],
  assignments: { live: 0, scheduled: 0, closed: 0 },
  unpublishedChanges: null,
  id: TEST_ID,
  title: "IELTS Reading Practice 3",
  description: null,
  status: "draft",
  currentVersion: 0,
  totalPoints: 1,
  questionCount: 1,
  audioCount: 0,
  sections: [
    {
      id: "018f0000-0000-7000-8000-0000000000c1",
      ordinal: 0,
      title: "Ngữ pháp",
      instructions: null,
      questionIds: [QUESTION_ID],
    },
  ],
  createdAt: STAMP,
  updatedAt: STAMP,
};

const question: components["schemas"]["AdminQuestion"] = {
  level: null,
  skill: null,
  id: QUESTION_ID,
  type: "single_choice",
  prompt: "They ___ to the museum.",
  points: 1,
  tags: [],
  options: [
    {
      id: "018f0000-0000-7000-8000-0000000000d1",
      ordinal: 0,
      text: "went",
      isCorrect: true,
    },
    {
      id: "018f0000-0000-7000-8000-0000000000d2",
      ordinal: 1,
      text: "have gone",
      isCorrect: false,
    },
  ],
  createdAt: STAMP,
  updatedAt: STAMP,
};

/** openBuilder stubs the draft and its question, answers every question save with 400, and opens the builder. */
async function openBuilder(page: Page, width: number) {
  const run = { patches: 0, errors: [] as string[] };
  page.on("pageerror", (error) => run.errors.push(error.message));
  await stubApi(page, {
    ...sessionAs(adminUser),
    [`GET /teacher/tests/${TEST_ID}`]: (route) => route.fulfill({ json: draft }),
    [`GET /teacher/questions/${QUESTION_ID}`]: (route) =>
      route.fulfill({ json: question }),
    [`PATCH /teacher/questions/${QUESTION_ID}`]: (route) => {
      run.patches++;
      return route.fulfill({
        status: 400,
        json: {
          error: {
            code: "VALIDATION_FAILED",
            message: "Dữ liệu câu hỏi không hợp lệ.",
            details: { "options[1].text": "Phương án không hợp lệ." },
            requestId: "018f0000-0000-7000-8000-0000000000f1",
          },
        },
      });
    },
  });
  await page.setViewportSize({ width, height: 900 });
  await page.goto(`/teacher/tests/${TEST_ID}/edit`);
  return run;
}

for (const width of [1280, 390]) {
  test(`a question save the server refuses says so and leaves the editor up at ${width}px`, async ({
    page,
  }) => {
    const run = await openBuilder(page, width);

    await page
      .getByRole("textbox", { name: "Lựa chọn B", exact: true })
      .fill("have been");
    await expect.poll(() => run.patches, { timeout: 5_000 }).toBe(1);

    await expect(page.getByText("Chưa lưu được", { exact: true })).toBeVisible();
    await expect(page.getByText("Dữ liệu câu hỏi không hợp lệ.")).toBeVisible();
    await expect(
      page.getByRole("textbox", { name: "Lựa chọn B", exact: true }),
    ).toHaveValue("have been");
    expect(run.patches).toBe(1);
    expect(run.errors).toEqual([]);
  });
}

test("a blank option is not sent, and the label says what to fix", async ({ page }) => {
  const run = await openBuilder(page, 1280);

  await page.getByRole("button", { name: "Thêm lựa chọn", exact: true }).click();
  await expect(page.getByText("Còn lựa chọn để trống.")).toBeVisible({
    timeout: 5_000,
  });
  await expect(page.getByText("Chưa lưu được", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Lựa chọn C", exact: true }),
  ).toBeVisible();
  expect(run.patches).toBe(0);
  expect(run.errors).toEqual([]);
});
