import { expect, test, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";
import { paper, type Session } from "./support/engine";
import type { components } from "../../src/lib/api/schema";

const KEY = "quizzivy.answer-draft.paper";
const PICKED = { q1: { type: "choice", optionIds: ["a"] } };

interface Backend {
  data: Session;
  offline: boolean;
  sessionId: string;
  answers: Session["answers"];
  calls: { call: "save" | "resume"; body: unknown }[];
}

function backend(): Backend {
  return {
    data: paper(),
    offline: false,
    sessionId: "session-1",
    answers: {},
    calls: [],
  };
}

function card(data: Session): components["schemas"]["StudentAssignmentCard"] {
  return {
    id: data.attempt.assignmentId,
    testTitle: data.testTitle,
    status: "open",
    opensAt: data.attempt.startedAt,
    closesAt: data.attempt.deadlineAt,
    durationMinutes: 60,
    questionCount: data.questions.length,
    totalPoints: 3,
    attemptsUsed: 1,
    maxAttempts: 2,
    hasLiveAttempt: true,
    lastAttemptId: data.attempt.id,
    liveDeadlineAt: data.attempt.deadlineAt,
  };
}

function serve(page: Page, server: Backend) {
  const { data } = server;
  const session = (): Session => ({
    ...data,
    sessionId: server.sessionId,
    answers: server.answers,
  });
  return stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/classes": { body: { items: [] } },
    "GET /app/assignments": {
      body: { dueNow: [card(data)], upcoming: [], completed: [] },
    },
    "GET /app/attempts/paper": (route) => route.fulfill({ json: session() }),
    "PATCH /app/attempts/paper/answers": async (route) => {
      if (server.offline) {
        await route.abort("failed");
        return;
      }
      const body = route.request().postDataJSON() as {
        sessionId: string;
        answers?: Session["answers"];
      };
      server.calls.push({ call: "save", body });
      if (body.sessionId !== server.sessionId) {
        await route.fulfill({
          status: 409,
          json: {
            error: {
              code: "SESSION_SUPERSEDED",
              message: "Bài làm này đã được mở ở nơi khác.",
              requestId: "req-1",
            },
          },
        });
        return;
      }
      server.answers = { ...server.answers, ...body.answers };
      await route.fulfill({
        json: {
          serverTime: data.serverTime,
          savedAt: data.serverTime,
          deadlineAt: data.attempt.deadlineAt,
        },
      });
    },
    "POST /app/assignments/assignment/attempts": async (route) => {
      server.calls.push({ call: "resume", body: route.request().postDataJSON() });
      server.sessionId = "session-2";
      await route.fulfill({ json: session() });
    },
    "POST /app/attempts/paper/events": { body: {} },
  });
}

function draftAnswers(page: Page) {
  return page.evaluate((key) => {
    const raw = localStorage.getItem(key);
    return raw === null ? null : (JSON.parse(raw) as { answers: unknown }).answers;
  }, KEY);
}

async function strand(page: Page, server: Backend) {
  await serve(page, server);
  await page.goto("/app/attempts/paper");
  await expect(page.getByRole("radio", { name: "Đáp án A" })).toBeVisible();
  server.offline = true;
  await page.locator("label").filter({ hasText: "Đáp án A" }).click();
  await expect.poll(() => draftAnswers(page)).toEqual(PICKED);
  await page.close();
  server.offline = false;
  server.calls = [];
}

async function continueFromHome(page: Page, server: Backend) {
  await serve(page, server);
  await page.goto("/app");
  await page.getByRole("button", { name: "Tiếp tục làm bài" }).click();
  await expect(page).toHaveURL(/\/app\/attempts\/paper$/);
  await expect(page.getByRole("radio", { name: "Đáp án A" })).toBeVisible();
}

test("answers a closed tab left are saved when the student continues from Home", async ({
  page,
  context,
}) => {
  const server = backend();
  await strand(page, server);

  const reopened = await context.newPage();
  await continueFromHome(reopened, server);

  expect(server.calls.slice(0, 2)).toEqual([
    { call: "save", body: { sessionId: "session-1", answers: PICKED } },
    { call: "resume", body: { resume: "paper" } },
  ]);
  await expect(reopened.getByRole("radio", { name: "Đáp án A" })).toBeChecked();
  await expect.poll(() => draftAnswers(reopened)).toBeNull();
});

test("a draft is forgotten when another device has taken the attempt", async ({
  page,
  context,
}) => {
  const server = backend();
  await strand(page, server);
  server.sessionId = "session-elsewhere";

  const reopened = await context.newPage();
  await continueFromHome(reopened, server);

  expect(server.calls.slice(0, 2)).toEqual([
    { call: "save", body: { sessionId: "session-1", answers: PICKED } },
    { call: "resume", body: { resume: "paper" } },
  ]);
  await expect(reopened.getByRole("radio", { name: "Đáp án A" })).not.toBeChecked();
  await expect.poll(() => draftAnswers(reopened)).toBeNull();
});
