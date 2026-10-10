import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";
import { contractJson } from "../support/contractResponse";
import {
  ASSIGNMENT_ID,
  ATTEMPT_ID,
  STUDENT_ID,
  ESSAY_ID,
  review,
} from "../units/attempts/fixtures";
import type { GradingQueueItem } from "../../src/features/attempts/api";

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
        page.getByText("Điểm đã lưu, nhưng chưa hoàn tất chấm bài."),
      ).toBeVisible();
      await page.getByRole("button", { name: "Thử hoàn tất lại" }).click();
      await expect.poll(counts.finishes).toBe(2);
      expect(counts.grades()).toBe(1);
      await expect(page.getByText("Không có câu trả lời chờ chấm")).toBeVisible();
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
