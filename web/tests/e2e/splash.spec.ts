import { expect, test } from "@playwright/test";
import { API, sessionAs, studentUser, stubApi } from "./support/api";
import { previewQuestions, previewSection } from "../support/groupPreview";
import type { components } from "../../src/lib/api/schema";

const classId = "018f0000-0000-7000-8000-0000000000c1";
const assignmentId = "018f0000-0000-7000-8000-0000000000a1";
const attemptId = "018f0000-0000-7000-8000-0000000000e1";

function liveCard(): components["schemas"]["StudentAssignmentCard"] {
  return {
    id: assignmentId,
    testTitle: "Bài luyện tập buổi tối",
    classId,
    className: "Lớp luyện tập buổi tối",
    status: "open",
    opensAt: new Date(Date.now() - 3_600_000).toISOString(),
    closesAt: new Date(Date.now() + 86_400_000).toISOString(),
    durationMinutes: 45,
    questionCount: 3,
    totalPoints: 10,
    attemptsUsed: 1,
    maxAttempts: 2,
    hasLiveAttempt: true,
    liveDeadlineAt: new Date(Date.now() + 1_800_000).toISOString(),
  };
}

function session(): components["schemas"]["AttemptSession"] {
  const now = new Date().toISOString();
  return {
    attempt: {
      id: attemptId,
      assignmentId,
      studentId: studentUser.id,
      testVersionId: attemptId,
      attemptNo: 1,
      status: "in_progress",
      startedAt: now,
      deadlineAt: new Date(Date.now() + 1_800_000).toISOString(),
    },
    testTitle: "Bài luyện tập buổi tối",
    sections: [previewSection],
    questions: previewQuestions.slice(1),
    groups: [],
    audioPlays: {},
    answers: {},
    sessionId: attemptId,
    beaconToken: "synthetic",
    serverTime: now,
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: false,
      maxFocusLoss: 0,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
  };
}

const student = {
  ...sessionAs(studentUser),
  "GET /app/classes": { body: { items: [] } },
  "GET /app/assignments": {
    body: { dueNow: [liveCard()], upcoming: [], completed: [] },
  },
};

test("an offline cold load counts down, retries on its own and lands on the page", async ({
  page,
}) => {
  await page.clock.install();
  await page.route(`${API}/**`, (route) => route.abort("internetdisconnected"));
  await page.goto("/app");

  await expect(page.getByText("Mất kết nối")).toBeVisible();
  await expect(page.getByText("Tự thử lại sau 10s")).toBeVisible();

  await page.unroute(`${API}/**`);
  await stubApi(page, student);
  await page.clock.runFor(10_000);

  await expect(page.getByRole("button", { name: "Tiếp tục làm bài" })).toBeVisible();
  await expect(page.locator(".qz-boot")).toHaveCount(0);
  await expect(page).toHaveURL(/\/app$/);
});

test("maintenance at boot shows the maintenance page until the status says it is over", async ({
  page,
}) => {
  let over = false;
  const window = {
    startsAt: new Date(Date.now() - 600_000).toISOString(),
    endsAt: new Date(Date.now() + 3_600_000).toISOString(),
  };
  await stubApi(page, {
    ...student,
    "GET /auth/me": (route) =>
      over
        ? route.fulfill({ json: studentUser })
        : route.fulfill({
            status: 503,
            json: {
              error: {
                code: "MAINTENANCE",
                message: "Quizzivy đang được cập nhật.",
                requestId: "e2e",
                details: window,
              },
            },
          }),
    "GET /public/status": (route) =>
      route.fulfill({
        json: { maintenance: over ? null : { ...window, active: true } },
      }),
  });
  await page.goto("/app");

  const overlay = page.getByRole("dialog", { name: "Quizzivy đang được cập nhật" });
  await expect(overlay).toBeVisible();
  await page.getByRole("button", { name: "Kiểm tra lại" }).click();
  await expect(overlay).toBeVisible();

  over = true;
  await page.getByRole("button", { name: "Kiểm tra lại" }).click();
  await expect(overlay).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Tiếp tục làm bài" })).toBeVisible();
});

test("a newer build shows the update card at the next navigation, never on the test itself", async ({
  page,
}) => {
  await page.route("**/version.json", (route) =>
    route.fulfill({ json: { build: "a-newer-build", version: "9.9.9" } }),
  );
  await stubApi(page, {
    ...student,
    [`POST /app/assignments/${assignmentId}/attempts`]: { body: session() },
    [`GET /app/attempts/${attemptId}`]: { body: session() },
  });
  await page.goto("/app");
  await expect(page.getByRole("button", { name: "Tiếp tục làm bài" })).toBeVisible();

  const asked = page.waitForRequest("**/version.json");
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  await asked;

  await page.getByRole("button", { name: "Tiếp tục làm bài" }).click();
  await expect(page).toHaveURL(new RegExp(`/app/attempts/${attemptId}$`));
  await expect(page.getByText("Bài luyện tập buổi tối").first()).toBeVisible();
  await expect(page.getByText("Quizzivy vừa được cập nhật")).toHaveCount(0);

  await page.goBack();
  await expect(page.getByText("Quizzivy vừa được cập nhật")).toBeVisible();
  await expect(page.getByRole("button", { name: "Tải lại ngay" })).toBeFocused();
});
