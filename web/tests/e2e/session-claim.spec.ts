import { expect, test, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";
import { paper, type Session } from "./support/engine";
import type { components } from "../../src/lib/api/schema";

const S1 = "018f0000-0000-7000-8000-00000000a001";
const S2 = "018f0000-0000-7000-8000-00000000b002";
const DRAFT = "quizzivy.answer-draft.paper";
const HELD = "quizzivy.session.paper";
const BAR = "Bài này đang mở ở thiết bị khác. Bạn không thể sửa ở đây nữa.";

interface Backend {
  data: Session;
  taken: boolean;
  answers: Session["answers"];
  named: (string | null)[];
  saves: { sessionId: string; answers?: Session["answers"] }[];
}

function backend(): Backend {
  return { data: paper(), taken: false, answers: {}, named: [], saves: [] };
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

function read(server: Backend, named: string | null): Session {
  const data = { ...server.data, answers: server.answers };
  if (!server.taken) return { ...data, sessionId: S1 };
  if (named === S1)
    return { ...data, sessionId: S1, beaconToken: "", superseded: true };
  return { ...data, sessionId: S2 };
}

function serve(page: Page, server: Backend) {
  const { data } = server;
  return stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/classes": { body: { items: [] } },
    "GET /app/assignments": {
      body: { dueNow: [card(data)], upcoming: [], completed: [] },
    },
    "GET /app/attempts/paper": async (route) => {
      const named = new URL(route.request().url()).searchParams.get("session");
      server.named.push(named);
      await route.fulfill({ json: read(server, named) });
    },
    "PATCH /app/attempts/paper/answers": async (route) => {
      const body = route.request().postDataJSON() as Backend["saves"][number];
      server.saves.push(body);
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
      server.taken = true;
      await route.fulfill({ json: { ...data, sessionId: S2 } });
    },
    "POST /app/attempts/paper/events": { body: {} },
  });
}

const choiceA = (page: Page) => page.getByRole("radio", { name: "Đáp án A" });
const storedDraft = (page: Page) =>
  page.evaluate((key) => localStorage.getItem(key), DRAFT);

async function staysReadOnly(page: Page, server: Backend) {
  await expect(page.getByText(BAR)).toBeVisible();
  await expect(choiceA(page)).toBeDisabled();
  await page.locator("label").filter({ hasText: "Đáp án A" }).click({ force: true });
  await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 2500)));
  expect(server.saves).toEqual([]);
  await expect(choiceA(page)).not.toBeChecked();
}

test("a tab another device took over stays read-only after a reload", async ({
  page,
}) => {
  const server = backend();
  await serve(page, server);
  await page.goto("/app/attempts/paper");
  await expect(choiceA(page)).toBeEnabled();
  expect(server.named).toEqual([null]);

  server.taken = true;
  const stored = await page.evaluate(
    ([key, studentId, sessionId]) => {
      const draft = JSON.stringify({
        studentId,
        sessionId,
        deadlineAt: Date.now() + 3600000,
        answers: { q1: { type: "choice", optionIds: ["b"] } },
      });
      localStorage.setItem(key, draft);
      return draft;
    },
    [DRAFT, studentUser.id, S2] as const,
  );

  await page.reload();
  await staysReadOnly(page, server);
  expect(server.named).toEqual([null, S1]);
  expect(await storedDraft(page)).toBe(stored);

  await page.reload();
  await staysReadOnly(page, server);
  expect(server.named).toEqual([null, S1, S1]);
  expect(await storedDraft(page)).toBe(stored);
});

test("Continue gives the tab the new session, and its reload keeps it", async ({
  page,
}) => {
  const server = backend();
  await serve(page, server);
  await page.addInitScript(
    ([key, sessionId]) => {
      if (sessionStorage.getItem(key) === null) sessionStorage.setItem(key, sessionId);
    },
    [HELD, S1] as const,
  );

  await page.goto("/app");
  await page.getByRole("button", { name: "Tiếp tục làm bài" }).click();
  await expect(page).toHaveURL(/\/app\/attempts\/paper$/);
  await expect(choiceA(page)).toBeEnabled();
  expect(server.named).toEqual([S2]);
  await expect(page.getByText(BAR)).toHaveCount(0);

  await page.locator("label").filter({ hasText: "Đáp án A" }).click();
  await expect.poll(() => server.saves.length).toBe(1);
  expect(server.saves[0]).toMatchObject({
    sessionId: S2,
    answers: { q1: { type: "choice", optionIds: ["a"] } },
  });

  await page.reload();
  await expect(choiceA(page)).toBeChecked();
  await expect(choiceA(page)).toBeEnabled();
  expect(server.named).toEqual([S2, S2]);
});
