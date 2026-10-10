import { expect, test } from "@playwright/test";
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

for (const width of [1280, 390]) {
  test(`a refused question save says so and leaves the editor up at ${width}px`, async ({
    page,
  }) => {
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    let patches = 0;
    await stubApi(page, {
      ...sessionAs(adminUser),
      [`GET /teacher/tests/${TEST_ID}`]: (route) => route.fulfill({ json: draft }),
      [`GET /teacher/questions/${QUESTION_ID}`]: (route) =>
        route.fulfill({ json: question }),
      [`PATCH /teacher/questions/${QUESTION_ID}`]: (route) => {
        patches++;
        return route.fulfill({
          status: 400,
          json: {
            error: {
              code: "VALIDATION_FAILED",
              message: "Dữ liệu câu hỏi không hợp lệ.",
              details: { "options[2].text": "Phương án không được để trống." },
              requestId: "018f0000-0000-7000-8000-0000000000f1",
            },
          },
        });
      },
    });
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/teacher/tests/${TEST_ID}/edit`);

    await page
      .getByRole("textbox", { name: "Lựa chọn B", exact: true })
      .fill("have been");
    await expect.poll(() => patches, { timeout: 5_000 }).toBe(1);

    await expect(page.getByText("Chưa lưu được", { exact: true })).toBeVisible();
    await expect(
      page.getByRole("textbox", { name: "Lựa chọn B", exact: true }),
    ).toHaveValue("have been");
    await page.waitForTimeout(2_000);
    expect(patches).toBe(1);
    expect(errors).toEqual([]);
  });
}
