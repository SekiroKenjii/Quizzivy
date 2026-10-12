import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import type { components } from "../../src/lib/api/schema";

const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const STAMP = "2026-10-01T00:00:00Z";

const question: components["schemas"]["AdminQuestion"] = {
  level: null,
  skill: null,
  id: QUESTION_ID,
  type: "single_choice",
  prompt: "They ___ to the museum.",
  points: 1,
  tags: [],
  options: ["went", "have gone"].map((text, ordinal) => ({
    id: `018f0000-0000-7000-8000-0000000000d${ordinal + 1}`,
    ordinal,
    text,
    isCorrect: ordinal === 0,
  })),
  createdAt: STAMP,
  updatedAt: STAMP,
};

const draft: components["schemas"]["Test"] = {
  skills: [],
  assignments: { live: 0, scheduled: 0, closed: 0 },
  unpublishedChanges: null,
  id: TEST_ID,
  title: "IELTS Reading Practice 3",
  description: null,
  status: "draft",
  currentVersion: 0,
  nextVersion: 1,
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

/**
 * placement opens the prompt's link popover and measures it as the deck
 * places it: below its button, at the button's left edge unless that would
 * take it closer than 6px to the end of its field.
 */
async function placement(page: Page) {
  await page
    .getByRole("group", { name: "Chế độ soạn: Nội dung câu hỏi", exact: true })
    .getByRole("button", { name: "Văn bản định dạng", exact: true })
    .click();
  await page.getByRole("button", { name: "Áp dụng chuyển đổi", exact: true }).click();
  const toolbar = page.getByRole("toolbar", {
    name: "Thanh định dạng: Nội dung câu hỏi",
    exact: true,
  });
  await page.getByRole("textbox", { name: "Nội dung câu hỏi", exact: true }).click();
  await toolbar.getByRole("button", { name: "Liên kết", exact: true }).click();
  const panel = page.getByRole("dialog", { name: "Liên kết" });
  await expect(panel).toBeVisible();
  await panel.evaluate((node) =>
    Promise.all(node.getAnimations({ subtree: true }).map((motion) => motion.finished)),
  );
  return page.evaluate(() => {
    const opener = document.querySelector<HTMLElement>(
      "[role=toolbar] button[aria-expanded=true]",
    )!;
    const field = opener.closest("[role=toolbar]")!.parentElement!;
    const popover = document.querySelector("[data-slot=popover-content]")!;
    const at = opener.getBoundingClientRect();
    const box = popover.getBoundingClientRect();
    const end =
      field.getBoundingClientRect().left + field.clientLeft + field.clientWidth;
    return {
      gap: Math.round(box.top - at.bottom),
      fromButton: Math.round(box.left - at.left),
      toFieldEnd: Math.round(end - box.right),
    };
  });
}

for (const width of [1440, 1280, 360]) {
  test(`the link popover opens below its button and inside its field at ${width}px`, async ({
    page,
  }) => {
    await stubApi(page, {
      ...sessionAs(adminUser),
      [`GET /teacher/tests/${TEST_ID}`]: { body: draft },
      [`GET /teacher/questions/${QUESTION_ID}`]: { body: question },
    });
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`/teacher/tests/${TEST_ID}/edit`);
    const measured = await placement(page);
    expect(measured.gap).toBe(6);
    expect(measured.toFieldEnd).toBeGreaterThanOrEqual(6);
    expect(measured.fromButton === 0 || measured.toFieldEnd === 6).toBe(true);
  });
}
