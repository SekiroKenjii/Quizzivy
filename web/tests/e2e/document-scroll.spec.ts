import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, studentUser, stubApi } from "./support/api";
import { paper, start } from "./support/engine";
import { assignment, classes } from "./support/student";
import {
  previewGroup,
  previewQuestions,
  previewSection,
} from "../support/groupPreview";
import type { components } from "../../src/lib/api/schema";

test.use({
  launchOptions: async ({ launchOptions }, provide) => {
    await provide({ ...launchOptions, ignoreDefaultArgs: ["--hide-scrollbars"] });
  },
});

const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const STAMP = "2026-10-01T00:00:00Z";

const question: components["schemas"]["AdminQuestion"] = {
  level: null,
  skill: null,
  id: QUESTION_ID,
  type: "single_choice",
  prompt: "They ___ to the museum last weekend with their grandparents.",
  points: 1,
  tags: [],
  options: ["went", "have gone", "go", "were going"].map((text, ordinal) => ({
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

const published: components["schemas"]["Test"] = {
  ...draft,
  title: "Đề kiểm tra ngữ liệu",
  status: "published",
  currentVersion: 1,
  nextVersion: 2,
  totalPoints: 3,
  questionCount: 3,
  audioCount: 2,
  unpublishedChanges: 0,
  sections: [],
};

/** teacher stubs a signed-in teacher with a published test and a bank question. */
async function teacher(page: Page) {
  await stubApi(page, {
    ...sessionAs(adminUser),
    [`GET /teacher/tests/${TEST_ID}`]: { body: published },
    [`GET /teacher/tests/${TEST_ID}/versions`]: {
      body: {
        items: [
          {
            id: "018f0000-0000-7000-8000-0000000000e1",
            version: 1,
            totalPoints: 3,
            questionCount: 3,
            audioCount: 2,
            manualCount: 0,
            skills: [],
            assignmentCount: 0,
            changeNote: null,
            publishedAt: STAMP,
            publishedBy: "Giáo viên",
          },
        ],
      },
    },
    [`GET /teacher/tests/${TEST_ID}/preview`]: {
      body: {
        version: 1,
        questions: previewQuestions,
        sections: [previewSection],
        groups: [previewGroup],
      },
    },
    [`GET /teacher/questions/${QUESTION_ID}`]: { body: question },
  });
  await page.route("https://assets.example/**", (route) => route.abort());
}

/** settle opens a route and waits for its heading, the sign that the page has its data. */
async function settle(page: Page, path: string, heading: string | RegExp) {
  await page.goto(path);
  await expect(page.getByRole("heading", { level: 1, name: heading })).toBeVisible();
  await expect(page.locator("[data-slot='skeleton']")).toHaveCount(0);
}

/**
 * documentScroll is how far the document itself scrolls down, the width a
 * window scrollbar takes, and the most any scroller on the page scrolls past
 * the end of its own content, less a pixel of rounding. A visually hidden
 * element positioned against an ancestor outside its scroller lengthens that
 * ancestor, the document included, by where it sits.
 */
function documentScroll(page: Page) {
  return page.evaluate(() => {
    const past = (scroller: Element) => {
      const top = scroller.getBoundingClientRect().top + scroller.clientTop;
      const bottoms = [...scroller.children]
        .filter((child) => !/absolute|fixed/.test(getComputedStyle(child).position))
        .map(
          (child) =>
            child.getBoundingClientRect().bottom -
            top +
            scroller.scrollTop +
            parseFloat(getComputedStyle(child).marginBottom),
        );
      const end =
        Math.max(0, ...bottoms) + parseFloat(getComputedStyle(scroller).paddingBottom);
      return Math.max(
        0,
        Math.floor(scroller.scrollHeight - Math.max(end, scroller.clientHeight) - 1),
      );
    };
    const scrollers = [...document.body.querySelectorAll("*")].filter(
      (node) =>
        node.children.length > 0 &&
        /auto|scroll/.test(getComputedStyle(node).overflowY),
    );
    return {
      down: document.scrollingElement!.scrollHeight - innerHeight,
      scrollbar: innerWidth - document.documentElement.clientWidth,
      past: Math.max(0, ...scrollers.map(past)),
    };
  });
}

const viewports = [
  { width: 1280, height: 700 },
  { width: 360, height: 700 },
];

for (const viewport of viewports) {
  const at = `${viewport.width}x${viewport.height}`;

  test(`the test detail scrolls only its main at ${at}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await teacher(page);
    await settle(page, `/teacher/tests/${TEST_ID}`, published.title);
    await expect(page.getByRole("heading", { name: previewGroup.title })).toBeVisible();
    expect(await documentScroll(page)).toEqual({ down: 0, scrollbar: 0, past: 0 });
  });

  test(`the builder scrolls only its main at ${at}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await stubApi(page, {
      ...sessionAs(adminUser),
      [`GET /teacher/tests/${TEST_ID}`]: { body: draft },
      [`GET /teacher/questions/${QUESTION_ID}`]: { body: question },
    });
    await page.goto(`/teacher/tests/${TEST_ID}/edit`);
    await expect(
      page.getByRole("textbox", { name: "Lựa chọn D", exact: true }),
    ).toBeVisible();
    expect(await documentScroll(page)).toEqual({ down: 0, scrollbar: 0, past: 0 });
  });

  test(`the question editor scrolls only its main at ${at}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await teacher(page);
    await page.goto(`/teacher/question-bank/${QUESTION_ID}`);
    await expect(
      page.getByRole("textbox", { name: "Lựa chọn D", exact: true }),
    ).toBeVisible();
    expect(await documentScroll(page)).toEqual({ down: 0, scrollbar: 0, past: 0 });
  });

  test(`the student home scrolls only its main at ${at}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await stubApi(page, {
      ...sessionAs(studentUser),
      "GET /app/classes": { body: { items: classes } },
      "GET /app/assignments": {
        body: {
          dueNow: [1, 2, 3, 4, 5, 6, 7, 8].map((index) =>
            assignment(index, index === 1),
          ),
          upcoming: [],
          completed: [],
        },
      },
    });
    await page.goto("/app");
    await expect(page.getByRole("button", { name: "Tiếp tục làm bài" })).toBeVisible();
    expect(await documentScroll(page)).toEqual({ down: 0, scrollbar: 0, past: 0 });
  });

  test(`the take-test engine scrolls only its panes at ${at}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    const session = paper();
    const choice = session.questions[0]!;
    choice.options = "ABCDEFGHIJ".split("").map((letter) => ({
      id: letter,
      text: `Đáp án ${letter}`,
    }));
    await start(page, session);
    await expect(page.getByRole("radio", { name: "Đáp án J" })).toBeAttached();
    expect(await documentScroll(page)).toEqual({ down: 0, scrollbar: 0, past: 0 });
  });
}
