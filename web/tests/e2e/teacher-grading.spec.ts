import { expect, test, type Page, type Route } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import { contractJson } from "../support/contractResponse";
import {
  ASSIGNMENT_ID,
  ATTEMPT_ID,
  STUDENT_ID,
  ESSAY_ID,
  review,
} from "../units/attempts/fixtures";
import type { AttemptListRow, GradingQueueItem } from "../../src/features/attempts/api";

async function prepare(page: Page, theme: "light" | "dark") {
  let grades = 0;
  let finishes = 0;
  let mark = false;
  const item: GradingQueueItem = {
    attemptId: ATTEMPT_ID,
    questionId: ESSAY_ID,
    assignmentId: ASSIGNMENT_ID,
    assignmentTitle: "Bài kiểm tra viết với tiêu đề dài dành cho lớp nền tảng buổi tối",
    studentId: STUDENT_ID,
    studentName: "Nguyễn Đức Minh",
    questionNumber: 2,
    type: "short_answer",
    prompt: "Write about your morning.",
    answer: { type: "text", value: "I wake up at six every day." },
    points: 1,
    score: null,
    comment: null,
    sampleAnswer: "I get up at half past six every morning.",
  };
  await page.addInitScript(() => localStorage.setItem("quizzivy.locale", "vi"));
  await page.emulateMedia({ colorScheme: theme });
  const user = { ...adminUser, preferences: { theme } };
  await stubApi(page, {
    ...sessionAs(user),
    "GET /teacher/summary": {
      body: { liveAssignments: 1, answersToGrade: 1, unreadNotifications: 0 },
    },
    "GET /teacher/grading/queue": async (route) => {
      const mode = new URL(route.request().url()).searchParams.get("mode") ?? "student";
      await route.fulfill({
        json: await contractJson("/teacher/grading/queue", "get", 200, {
          items: mark ? [] : [item],
          groups: mark
            ? []
            : [
                {
                  key: mode === "student" ? STUDENT_ID : `${ASSIGNMENT_ID}:${ESSAY_ID}`,
                  kind: mode,
                  label: mode === "student" ? item.studentName : "2",
                  sub: item.assignmentTitle,
                  remaining: 1,
                },
              ],
          answersRemaining: mark ? 0 : 1,
          studentsWaiting: mark ? 0 : 1,
        }).json(),
      });
    },
    "GET /teacher/attempts": { body: { items: [], total: 0, page: 1, pageSize: 100 } },
    [`GET /teacher/attempts/${ATTEMPT_ID}`]: async (route) => {
      await route.fulfill({
        json: await contractJson(
          "/teacher/attempts/{id}",
          "get",
          200,
          review({ essayScore: mark ? 0 : null }),
        ).json(),
      });
    },
    [`POST /teacher/attempts/${ATTEMPT_ID}/grade`]: async (route) => {
      grades += 1;
      mark = true;
      await route.fulfill({
        json: await contractJson("/teacher/attempts/{id}/grade", "post", 200, {
          earned: 0,
          total: 1,
          pendingManual: 0,
        }).json(),
      });
    },
    [`POST /teacher/attempts/${ATTEMPT_ID}/finish-grading`]: async (route) => {
      finishes += 1;
      if (finishes === 1)
        await route.fulfill({
          status: 500,
          json: { error: { code: "UNKNOWN", message: "Finish failed" } },
        });
      else
        await route.fulfill({
          json: await contractJson(
            "/teacher/attempts/{id}/finish-grading",
            "post",
            200,
            { ...review().attempt, status: "graded" },
          ).json(),
        });
    },
  });
  return { grades: () => grades, finishes: () => finishes };
}

for (const theme of ["light", "dark"] as const) {
  for (const width of [360, 768, 1024, 1280, 1440]) {
    test(`explicit grade and Finish recovery at ${width}px in ${theme}`, async ({
      page,
    }) => {
      const counts = await prepare(page, theme);
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/teacher/grading");
      await expect(
        page.getByRole("heading", { name: "Chấm bài", exact: true }),
      ).toBeVisible();
      await expect(page.locator(".qz-boot")).toHaveCount(0);
      await expect
        .poll(() =>
          page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        )
        .toBe(true);
      await expect(page.getByRole("button", { name: "Chấm 0 điểm" })).toHaveCSS(
        "height",
        "44px",
      );
      await page.getByRole("button", { name: "Chấm 0 điểm" }).click();
      await expect.poll(counts.grades).toBe(1);
      expect(counts.finishes()).toBe(0);
      await page.getByRole("button", { name: "Lưu & câu tiếp theo" }).click();
      await expect(page.getByRole("button", { name: "Hoàn tất chấm" })).toBeEnabled();
      expect(counts.finishes()).toBe(0);
      await page.getByRole("button", { name: "Hoàn tất chấm" }).click();
      await expect(
        page.getByText(
          "Đã lưu điểm cho Nguyễn Đức Minh, nhưng chưa hoàn tất chấm bài.",
        ),
      ).toBeVisible();
      await page.getByRole("button", { name: "Thử hoàn tất lại" }).click();
      await expect.poll(counts.finishes).toBe(2);
      expect(counts.grades()).toBe(1);
      await expect(
        page.getByRole("heading", { name: "Đã chấm xong tất cả" }),
      ).toBeVisible();
    });
  }
}

test("progress uses 200ms native ease and is static under reduced motion", async ({
  page,
}) => {
  await prepare(page, "light");
  await page.goto("/teacher/grading");
  const fill = page.getByRole("progressbar").locator("div");
  await expect(fill).toHaveCSS("transition-duration", "0.2s");
  await expect(fill).toHaveCSS("transition-timing-function", "ease");
  await fill.evaluate((element) => {
    element.addEventListener(
      "transitionrun",
      ((event: TransitionEvent) => {
        if (event.propertyName !== "width") return;
        const transition = element
          .getAnimations()
          .find(
            (animation): animation is CSSTransition =>
              animation instanceof CSSTransition &&
              animation.transitionProperty === "width",
          );
        transition?.pause();
        (
          window as unknown as { heldTransition?: CSSTransition | undefined }
        ).heldTransition = transition;
      }) as EventListener,
      { once: true },
    );
  });
  await page.getByRole("button", { name: "Chấm 0 điểm" }).click();
  await page.waitForFunction(
    () => (window as unknown as { heldTransition?: CSSTransition }).heldTransition,
  );
  const held = await fill.evaluate((element) => {
    const transition = (window as unknown as { heldTransition: CSSTransition })
      .heldTransition;
    const effect = transition.effect as KeyframeEffect;
    const widthAt = (ms: number) => {
      transition.currentTime = ms;
      return element.getBoundingClientRect().width;
    };
    const measured = {
      duration: effect.getTiming().duration,
      start: widthAt(0),
      middle: widthAt(100),
      end: widthAt(200),
    };
    transition.finish();
    return measured;
  });
  expect(held.duration).toBe(200);
  expect(held.start).toBe(0);
  expect(held.end).toBeGreaterThan(0);
  expect(held.middle / held.end).toBeCloseTo(0.8, 1);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect(fill).toHaveCSS("transition-property", "none");
});

async function prepareLong(page: Page, theme: "light" | "dark") {
  const items: GradingQueueItem[] = Array.from({ length: 24 }, (_, index) => ({
    attemptId: `018f0000-0000-7000-8000-${String(index).padStart(12, "0")}`,
    questionId: ESSAY_ID,
    assignmentId: ASSIGNMENT_ID,
    assignmentTitle: "Bài viết cuối kỳ",
    studentId: `018f0000-0000-7000-9000-${String(index).padStart(12, "0")}`,
    studentName: `Học viên số ${index + 1}`,
    questionNumber: 2,
    type: "short_answer",
    prompt: "Write about your morning.",
    answer: {
      type: "text",
      value: Array.from(
        { length: 40 },
        (_, line) => `Line ${line + 1}: I wake up at six and walk to the bus stop.`,
      ).join("\n"),
    },
    points: 5,
    score: null,
    comment: null,
  }));
  await page.addInitScript(() => localStorage.setItem("quizzivy.locale", "vi"));
  await page.emulateMedia({ colorScheme: theme });
  const user = { ...adminUser, preferences: { theme } };
  await stubApi(page, {
    ...sessionAs(user),
    "GET /teacher/summary": {
      body: { liveAssignments: 1, answersToGrade: 24, unreadNotifications: 0 },
    },
    "GET /teacher/grading/queue": async (route) => {
      await route.fulfill({
        json: await contractJson("/teacher/grading/queue", "get", 200, {
          items,
          groups: items.map((item) => ({
            key: item.studentId,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: 1,
          })),
          answersRemaining: 24,
          studentsWaiting: 24,
        }).json(),
      });
    },
    "GET /teacher/attempts": { body: { items: [], total: 0, page: 1, pageSize: 100 } },
  });
}

for (const theme of ["light", "dark"] as const) {
  for (const width of [768, 1024, 1280]) {
    test(`the queue and the card scroll inside a viewport-high page at ${width}px in ${theme}`, async ({
      page,
    }) => {
      await prepareLong(page, theme);
      await page.setViewportSize({ width, height: 800 });
      await page.goto("/teacher/grading");
      const queue = page.getByRole("complementary");
      const card = page.getByRole("region", { name: "Câu trả lời đang chấm" });
      await expect(card).toBeVisible();
      const box = await queue.locator("../..").boundingBox();
      expect(box?.height).toBeCloseTo(800 - 120, 0);
      const scrolls = (locator: typeof card) =>
        locator.evaluate((element) => element.scrollHeight > element.clientHeight);
      expect(await scrolls(queue.locator("> div").last())).toBe(true);
      expect(await scrolls(card.locator("> div").first())).toBe(true);
      await expect(
        page.getByRole("button", { name: "Lưu & câu tiếp theo" }),
      ).toBeInViewport();
      await page.keyboard.press("j");
      await page.keyboard.press("j");
      await expect(
        queue.getByRole("button", { name: /Học viên số 3/ }),
      ).toHaveAttribute("aria-current", "true");
      for (let step = 0; step < 18; step += 1) await page.keyboard.press("j");
      await expect(
        queue.getByRole("button", { name: /Học viên số 21/ }),
      ).toBeInViewport();
      await expect
        .poll(() =>
          page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        )
        .toBe(true);
    });
  }
}

test("below 768 the page stays static and the queue keeps its 180px cap", async ({
  page,
}) => {
  await prepareLong(page, "light");
  await page.setViewportSize({ width: 360, height: 800 });
  await page.goto("/teacher/grading");
  const queue = page.getByRole("complementary");
  await expect(queue).toBeVisible();
  const list = await queue.locator("> div").last().boundingBox();
  expect(list?.height).toBeLessThanOrEqual(180);
  const column = await queue.locator("../..").boundingBox();
  expect(column?.height).toBeGreaterThan(800 - 120);
});

async function prepareAi(page: Page, theme: "light" | "dark") {
  let grades = 0;
  let suggestions = 0;
  const item: GradingQueueItem = {
    attemptId: ATTEMPT_ID,
    questionId: ESSAY_ID,
    assignmentId: ASSIGNMENT_ID,
    assignmentTitle: "Mid-term Reading Mock",
    studentId: STUDENT_ID,
    studentName: "Nguyễn Đức Minh",
    questionNumber: 27,
    type: "short_answer",
    prompt: "What are the car-free streets called? Write NO MORE THAN TWO WORDS.",
    answer: { type: "text", value: "play street" },
    points: 1,
    score: null,
    comment: null,
    sampleAnswer: "play streets",
  };
  await page.addInitScript(() => localStorage.setItem("quizzivy.locale", "vi"));
  await page.emulateMedia({ colorScheme: theme });
  const user = { ...adminUser, preferences: { theme } };
  await stubApi(page, {
    ...sessionAs(user),
    "GET /public/status": { body: { maintenance: null, ai: true } },
    "GET /teacher/ai/usage": {
      body: {
        month: "2026-10-01",
        used: 1200,
        allowance: 20000,
        left: 18800,
        paused: false,
        requests: true,
        pending: null,
        categories: [],
      },
    },
    "GET /teacher/summary": {
      body: { liveAssignments: 1, answersToGrade: 1, unreadNotifications: 0 },
    },
    "GET /teacher/grading/queue": async (route) => {
      await route.fulfill({
        json: await contractJson("/teacher/grading/queue", "get", 200, {
          items: [item],
          groups: [
            {
              key: STUDENT_ID,
              kind: "student",
              label: item.studentName,
              sub: item.assignmentTitle,
              remaining: 1,
            },
          ],
          answersRemaining: 1,
          studentsWaiting: 1,
        }).json(),
      });
    },
    "GET /teacher/attempts": { body: { items: [], total: 0, page: 1, pageSize: 100 } },
    [`POST /teacher/ai/attempts/${ATTEMPT_ID}/answers/${ESSAY_ID}/score-suggestion`]:
      async (route) => {
        suggestions += 1;
        await route.fulfill({
          json: await contractJson(
            "/teacher/ai/attempts/{attemptId}/answers/{questionId}/score-suggestion",
            "post",
            200,
            {
              score: 0.5,
              confidence: "medium",
              reasons: [
                "Right idea, but the passage uses the plural “play streets”.",
                "The word form counts in IELTS-style marking.",
              ],
              comment: "Very close! Check the plural: the passage says “play streets”.",
              usage: { credits: 400, left: 18400 },
            },
          ).json(),
        });
      },
    [`POST /teacher/attempts/${ATTEMPT_ID}/grade`]: async (route) => {
      grades += 1;
      await route.fulfill({
        json: await contractJson("/teacher/attempts/{id}/grade", "post", 200, {
          earned: 0.5,
          total: 1,
          pendingManual: 0,
        }).json(),
      });
    },
  });
  return { grades: () => grades, suggestions: () => suggestions };
}

for (const theme of ["light", "dark"] as const) {
  for (const width of [360, 768, 1024, 1280, 1440]) {
    test(`Suggest a score fills the card and Save & next saves it at ${width}px in ${theme}`, async ({
      page,
    }) => {
      const counts = await prepareAi(page, theme);
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/teacher/grading");
      const box = page.getByRole("region", { name: "Gợi ý của AI" });
      const run = box.getByRole("button", { name: "Gợi ý điểm" });
      await expect(run).toHaveCSS("height", "30px");
      const order = await page
        .getByRole("region", { name: "Câu trả lời đang chấm" })
        .evaluate((card) => {
          const answer = card.querySelector("[aria-label='Gợi ý của AI']");
          const score = [...card.querySelectorAll("legend")].find(
            (legend) => legend.textContent === "Điểm",
          );
          return (
            answer !== null &&
            score !== undefined &&
            answer.getBoundingClientRect().bottom <= score.getBoundingClientRect().top
          );
        });
      expect(order).toBe(true);
      await run.click();
      await expect(box.getByText("Độ tin cậy trung bình")).toBeVisible();
      const score = box.getByText("/ 1", { exact: true }).locator("..");
      await expect(score).toHaveText("0,5 / 1");
      await expect(score).toHaveCSS("font-size", "17px");
      await box.getByRole("button", { name: "Dùng điểm và nhận xét" }).click();
      await expect(page.getByRole("button", { name: "Chấm 0,5 điểm" })).toHaveAttribute(
        "aria-pressed",
        "true",
      );
      await expect(page.getByLabel(/^Nhận xét/)).toHaveValue(
        "Very close! Check the plural: the passage says “play streets”.",
      );
      expect(counts.suggestions()).toBe(1);
      expect(counts.grades()).toBe(0);
      await expect
        .poll(() =>
          page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        )
        .toBe(true);
      await page.getByRole("button", { name: "Lưu & câu tiếp theo" }).click();
      await expect.poll(counts.grades).toBe(1);
    });
  }
}

async function prepareFinish(page: Page, theme: "light" | "dark", waiting: boolean) {
  const finished = new Set<string>();
  const papers: AttemptListRow[] = Array.from({ length: 7 }, (_, index) => {
    const n = String(index + 1).padStart(12, "0");
    return {
      id: `018f0000-0000-7000-8000-${n}`,
      studentId: `018f0000-0000-7000-9000-${n}`,
      studentName: `Học viên có tên khá dài số ${index + 1}`,
      assignmentId:
        index === 6 ? "018f0000-0000-7000-8000-0000000000d2" : ASSIGNMENT_ID,
      testTitle: index === 6 ? "Vocab Quiz: Travel" : "Reading Mock 3",
      status: index === 1 ? "timed_out" : "submitted",
      submittedAt: `2026-09-22T0${index + 1}:00:00Z`,
      pendingManual: 0,
      flagged: index === 1,
    };
  });
  const item: GradingQueueItem = {
    attemptId: ATTEMPT_ID,
    questionId: ESSAY_ID,
    assignmentId: ASSIGNMENT_ID,
    assignmentTitle: "Writing Mock 2",
    studentId: STUDENT_ID,
    studentName: "Phan Mỹ Duyên",
    questionNumber: 2,
    type: "short_answer",
    prompt: "Write about your morning.",
    answer: { type: "text", value: "I wake up at six every day." },
    points: 1,
    score: null,
    comment: null,
  };
  await page.addInitScript(() => localStorage.setItem("quizzivy.locale", "vi"));
  await page.emulateMedia({ colorScheme: theme });
  const user = { ...adminUser, preferences: { theme } };
  const perPaper = Object.fromEntries(
    papers.flatMap((paper) => {
      const read = review({ essayScore: 3 });
      read.attempt = {
        ...read.attempt,
        id: paper.id,
        assignmentId: paper.assignmentId,
        studentId: paper.studentId,
        status: paper.status,
        integrity: { focusLossCount: paper.flagged ? 3 : 0, flagged: paper.flagged },
      };
      return [
        [
          `GET /teacher/attempts/${paper.id}`,
          async (route: Route) =>
            route.fulfill({
              json: await contractJson(
                "/teacher/attempts/{id}",
                "get",
                200,
                read,
              ).json(),
            }),
        ],
        [
          `POST /teacher/attempts/${paper.id}/finish-grading`,
          async (route: Route) => {
            finished.add(paper.id);
            await route.fulfill({
              json: await contractJson(
                "/teacher/attempts/{id}/finish-grading",
                "post",
                200,
                {
                  ...read.attempt,
                  status: "graded",
                },
              ).json(),
            });
          },
        ],
      ];
    }),
  );
  await stubApi(page, {
    ...sessionAs(user),
    "GET /teacher/summary": {
      body: {
        liveAssignments: 1,
        answersToGrade: waiting ? 1 : 0,
        unreadNotifications: 0,
      },
    },
    "GET /teacher/grading/queue": {
      body: {
        items: waiting ? [item] : [],
        groups: waiting
          ? [
              {
                key: STUDENT_ID,
                kind: "student",
                label: item.studentName,
                sub: item.assignmentTitle,
                remaining: 1,
              },
            ]
          : [],
        answersRemaining: waiting ? 1 : 0,
        studentsWaiting: waiting ? 1 : 0,
      },
    },
    "GET /teacher/attempts": async (route) => {
      const status = new URL(route.request().url()).searchParams.get("status");
      const items = papers.filter(
        (paper) => paper.status === status && !finished.has(paper.id),
      );
      await route.fulfill({
        json: await contractJson("/teacher/attempts", "get", 200, {
          items,
          total: items.length,
          page: 1,
          pageSize: 100,
        }).json(),
      });
    },
    ...perPaper,
  });
  return { finished: () => finished.size };
}

const heights = { 360: [44, 44], 768: [36, 40], 1280: [32, 36] } as const;

for (const theme of ["light", "dark"] as const) {
  for (const width of [360, 768, 1280] as const) {
    test(`an empty queue lists the papers ready to finish at ${width}px in ${theme}`, async ({
      page,
    }) => {
      const counts = await prepareFinish(page, theme, false);
      const [small, large] = heights[width];
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/teacher/grading");
      await expect(
        page.getByRole("heading", { name: "Không còn câu trả lời cần chấm" }),
      ).toBeVisible();
      await expect(page.getByText("Không còn câu trả lời nào cần chấm.")).toBeVisible();
      const card = page.getByRole("region", { name: "Sẵn sàng hoàn tất" });
      await expect(card.getByRole("heading", { level: 3 })).toHaveText([
        "Reading Mock 3· 6 bài",
        "Vocab Quiz: Travel· 1 bài",
      ]);
      await expect(card.getByRole("button", { name: /^Xem bài/ })).toHaveCount(6);
      await expect(card.getByRole("button", { name: "Hiện thêm 1 bài" })).toHaveCSS(
        "height",
        `${large}px`,
      );
      const refresh = card.getByRole("button", { name: "Làm mới" });
      await expect(refresh).toHaveCSS("height", `${small}px`);
      if (width === 360) await expect(refresh).toHaveCSS("width", "44px");
      const review = page.getByRole("button", { name: "Xem 7 bài" });
      await expect(review).toHaveCSS("height", `${large}px`);
      await review.click();
      const finish = card.getByRole("button", { name: "Hoàn tất chấm" });
      await expect(finish).toBeFocused();
      await expect(finish).toHaveCSS("height", `${large}px`);
      await expect(card.getByText("Hết giờ")).toBeVisible();
      await expect
        .poll(() =>
          page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        )
        .toBe(true);
      await finish.click();
      await expect(
        page.getByText("Đã hoàn tất chấm bài của Học viên có tên khá dài số 1.", {
          exact: true,
        }),
      ).toBeVisible();
      await expect(page.locator("p[aria-live=polite].sr-only")).toHaveText(
        "Đã hoàn tất chấm bài của Học viên có tên khá dài số 1. Tiếp theo: Học viên có tên khá dài số 2.",
      );
      await expect.poll(counts.finished).toBe(1);
      await expect(card.getByRole("button", { name: "Hoàn tất chấm" })).toBeFocused();
      await expect(
        card
          .getByRole("button", { name: "Đóng", expanded: true })
          .locator("xpath=ancestor::li"),
      ).toContainText("Học viên có tên khá dài số 2");
      await expect(card.getByRole("button", { name: /^Xem bài/ })).toHaveCount(5);
    });

    test(`the card folds below the panes while answers wait at ${width}px in ${theme}`, async ({
      page,
    }) => {
      await prepareFinish(page, theme, true);
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/teacher/grading");
      const card = page.getByRole("region", { name: "Sẵn sàng hoàn tất" });
      const show = card.getByRole("button", { name: "Hiện" });
      await expect(show).toHaveAttribute("aria-expanded", "false");
      const panes = await page.getByRole("complementary").locator("..").boundingBox();
      const folded = await card.boundingBox();
      expect(folded!.y).toBeGreaterThanOrEqual(panes!.y + panes!.height);
      await page.getByRole("button", { name: "7 bài sẵn sàng hoàn tất" }).click();
      await expect(
        card.getByRole("heading", { name: "Sẵn sàng hoàn tất" }),
      ).toBeFocused();
      await expect(card.getByRole("button", { name: /^Xem bài/ })).toHaveCount(6);
      await expect(card.getByRole("button", { name: "Ẩn" })).toHaveAttribute(
        "aria-expanded",
        "true",
      );
      await expect
        .poll(() =>
          page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        )
        .toBe(true);
    });
  }
}
