import { expect, test, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";
import type { components } from "../../src/lib/api/schema";

type Session = components["schemas"]["AttemptSession"];
function paper(): Session {
  const now = new Date().toISOString();
  return {
    attempt: {
      id: "paper",
      assignmentId: "assignment",
      studentId: studentUser.id,
      testVersionId: "version",
      attemptNo: 1,
      status: "in_progress",
      startedAt: now,
      deadlineAt: new Date(Date.now() + 3600000).toISOString(),
    },
    testTitle: "Bài kiểm tra thao tác",
    sections: [{ id: "part", title: "Phần 1", instructions: null }],
    questions: [
      {
        id: "q1",
        sectionId: "part",
        type: "single_choice",
        prompt: "Chọn đáp án",
        points: 1,
        options: [
          { id: "a", text: "Đáp án A" },
          { id: "b", text: "Đáp án B" },
        ],
      },
      {
        id: "q2",
        sectionId: "part",
        type: "fill_blank",
        prompt: "She {{1}} yesterday.",
        points: 1,
        blanks: [{ id: "blank", ordinal: 1, caseSensitive: false }],
      },
      {
        id: "q3",
        sectionId: "part",
        type: "short_answer",
        prompt: "Viết một câu",
        points: 1,
      },
    ],
    sessionId: "session",
    beaconToken: "beacon",
    serverTime: now,
    audioPlays: {},
    answers: {},
    remainingAttempts: 1,
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: false,
      maxFocusLoss: 0,
      onLimitExceeded: "warn",
      minAwayMs: 3000,
    },
  };
}
async function start(page: Page, data: Session = paper()) {
  await stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/attempts/paper": { body: data },
    "PATCH /app/attempts/paper/answers": {
      body: { serverTime: data.serverTime, savedAt: data.serverTime },
    },
    "POST /app/attempts/paper/events": { body: {} },
  });
  await page.goto("/app/attempts/paper");
  await expect(page.getByRole("radio", { name: "Đáp án A" })).toBeVisible();
}

test("rich table blanks preserve frozen answer bindings and reload on a 320px phone", async ({
  page,
}, info) => {
  await page.setViewportSize({ width: 320, height: 850 });
  const data = paper();
  data.questions[1] = {
    id: "q2",
    sectionId: "part",
    type: "fill_blank",
    points: 2,
    prompt: "[2]\t[1]",
    promptContent: {
      format: "semantic_v1",
      blocks: [
        {
          type: "table",
          rows: [
            [
              {
                header: false,
                rowSpan: 1,
                colSpan: 1,
                content: [
                  {
                    type: "paragraph",
                    content: [{ type: "gap", id: "gap-b", label: "2" }],
                  },
                ],
              },
              {
                header: false,
                rowSpan: 1,
                colSpan: 1,
                content: [
                  {
                    type: "paragraph",
                    content: [{ type: "gap", id: "gap-a", label: "1" }],
                  },
                ],
              },
            ],
          ],
        },
      ],
    },
    blanks: [
      { id: "frozen-a", gapId: "gap-a", ordinal: 1, caseSensitive: false },
      { id: "frozen-b", gapId: "gap-b", ordinal: 2, caseSensitive: false },
    ],
  };
  await start(page, data);
  await page.route("**/app/attempts/paper/answers", async (route) => {
    const payload = route.request().postDataJSON() as { answers?: Session["answers"] };
    data.answers = { ...data.answers, ...payload.answers };
    await route.fulfill({
      json: { serverTime: data.serverTime, savedAt: data.serverTime },
    });
  });
  await page.getByRole("button", { name: "Câu sau", exact: true }).click();
  const first = page.getByRole("textbox", { name: "Chỗ trống 1", exact: true });
  const second = page.getByRole("textbox", { name: "Chỗ trống 2", exact: true });
  await first.fill("one");
  await second.fill("two");
  await expect
    .poll(() => data.answers["q2"])
    .toEqual({ type: "fill_blank", values: { "frozen-a": "one", "frozen-b": "two" } });
  await expect(page.locator("table input").first()).toHaveValue("two");
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
  ).toBe(true);
  await page.reload();
  await page.getByRole("button", { name: "Câu sau", exact: true }).click();
  await expect(first).toHaveValue("one");
  await expect(second).toHaveValue("two");
  await page.screenshot({
    path: info.outputPath("rich-blank-learner-320.png"),
    fullPage: true,
  });
});

for (const width of [320, 360, 1024, 1440]) {
  test(`student keyboard and fill-blank at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 850 });
    await start(page);
    await page.locator("label").filter({ hasText: "Đáp án A" }).click();
    await page.keyboard.press("b");
    await expect(page.getByRole("radio", { name: "Đáp án B" })).toBeChecked();
    await page.keyboard.press("f");
    await page.keyboard.press("ArrowRight");
    const blank = page.getByRole("textbox", { name: "Chỗ trống 1" });
    await blank.pressSequentially("went");
    await expect(blank).toBeFocused();
    await expect(blank).toHaveValue("went");
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    ).toBe(true);
    await page.screenshot({
      path: info.outputPath(`fill-blank-${width}.png`),
      fullPage: true,
    });
    await blank.press("ArrowLeft");
    await blank.press("X");
    await expect(blank).toHaveValue("wenXt");
    await page.getByRole("button", { name: "Câu sau", exact: true }).click();
    await page
      .getByRole("textbox", { name: "Bài làm của bạn" })
      .pressSequentially("A full sentence.");
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    ).toBe(true);
    const next = page
      .getByRole("button", { name: "Xem lại & nộp", exact: true })
      .last();
    const bounds = await next.boundingBox();
    expect(bounds).not.toBeNull();
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width);
    await page
      .getByRole("button", { name: "Xem lại & nộp", exact: true })
      .first()
      .click();
    await expect(
      page.getByRole("heading", { name: "Xem lại trước khi nộp" }),
    ).toBeVisible();
    const submit = page.getByRole("button", { name: "Nộp bài", exact: true }).first();
    await expect(submit).toBeInViewport();
    await submit.click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.keyboard.press("ArrowLeft");
    await expect(page.getByRole("dialog")).toBeVisible();
  });
}

test("a long paper keeps review actions reachable on a 320px phone", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 700 });
  const data = paper();
  data.questions = Array.from({ length: 80 }, (_, index) => ({
    ...data.questions[0]!,
    id: `question-${index}`,
  }));
  await start(page, data);
  await page.getByRole("button", { name: "Danh sách câu", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Xem lại & nộp", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Xem lại trước khi nộp" }),
  ).toBeVisible();
  const submit = page.getByRole("button", { name: "Nộp bài", exact: true });
  await expect(submit).toBeInViewport();
  await expect(
    page.getByRole("button", { name: "Quay lại làm tiếp", exact: true }),
  ).toBeInViewport();
  await submit.click();
  await expect(page.getByRole("dialog")).toContainText("Còn 80 câu bạn chưa trả lời");
});
