import { expect, test, type Page } from "@playwright/test";
import {
  ASSIGNMENT,
  freshAttempt,
  goToQuestion,
  gradeAttempt,
  signInAsAdmin,
  signInAsStudent,
  submitAttempt,
} from "./support/live";

/**
 * E2E 9 (§14, §13.5): the payload a student's browser receives for their own
 * attempt carries no `isCorrect`, `sampleAnswer`, `transcript` or
 * `acceptedAnswers`, at any depth. The Go side pins the same thing on the
 * domain type; this reads the bytes that actually crossed the wire.
 *
 * Since R6 (T-R6.14) it reads the paper with a question of every type, and
 * the session payload also carries none of the keys that only the result may
 * show: the correct answers, matches and options, an essay's band and its
 * criteria, and the explanation with its media. A second case reads a graded
 * result whose assignment hides the score, the answers and the explanations,
 * and finds none of them there either.
 */
const BANNED = new Set(["iscorrect", "sampleanswer", "transcript", "acceptedanswers"]);
const RESULT_ONLY = new Set([
  "correctoptionids",
  "correctanswers",
  "correctmatches",
  "correctoptionid",
  "band",
  "criteria",
  "explanation",
  "explanationcontent",
  "explanationassets",
]);

function offendingPaths(node: unknown, banned: Set<string>, path = "$"): string[] {
  if (Array.isArray(node)) {
    return node.flatMap((child, i) => offendingPaths(child, banned, `${path}[${i}]`));
  }
  if (node !== null && typeof node === "object") {
    return Object.entries(node as Record<string, unknown>).flatMap(([key, child]) => {
      const here = `${path}.${key}`;
      const own = banned.has(key.toLowerCase()) ? [here] : [];
      return [...own, ...offendingPaths(child, banned, here)];
    });
  }
  return [];
}

const both = new Set([...BANNED, ...RESULT_ONLY]);

/** The API's response to GET `path`, not the SPA document that shares it. */
function apiRead(page: Page, path: string) {
  return page.waitForResponse(
    (r) =>
      r.request().method() === "GET" &&
      new URL(r.url()).origin === "http://localhost:8080" &&
      new URL(r.url()).pathname === path,
  );
}

test("E2E 9: GET /app/attempts/:id contains no part of the grading key", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signInAsStudent(page);
  const attemptId = await freshAttempt(page, ASSIGNMENT.payload);

  const read = apiRead(page, `/app/attempts/${attemptId}`);
  await page.reload({ waitUntil: "commit" });
  const response = await read;
  expect(response.status()).toBe(200);
  const body: unknown = await response.json();

  // The paper is really there, so an empty payload cannot pass by accident.
  const questions = (body as { questions?: unknown[] }).questions ?? [];
  expect(questions.length).toBeGreaterThan(0);
  expect(offendingPaths(body, BANNED)).toEqual([]);
});

test("E2E 9: the every-type paper's session carries no result-only key", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signInAsStudent(page);
  const attemptId = await freshAttempt(page, ASSIGNMENT.everyType.payload);
  // The engine's own first read has landed, so the one awaited below is the
  // reload's and its body is still there to read.
  await expect(page.getByRole("group", { name: "Câu 1", exact: true })).toBeVisible();

  const read = apiRead(page, `/app/attempts/${attemptId}`);
  await page.reload({ waitUntil: "commit" });
  const response = await read;
  expect(response.status()).toBe(200);
  const body = (await response.json()) as {
    questions?: { type: string; matchItems?: unknown[]; promptAssets?: unknown[] }[];
  };

  // Every type is on the paper, with the parts that carry a key: matching
  // items and the prompt's image. So a key left out cannot pass for absent.
  const questions = body.questions ?? [];
  expect(new Set(questions.map((question) => question.type))).toEqual(
    new Set([
      "tfng",
      "matching",
      "true_false",
      "single_choice",
      "multiple_choice",
      "fill_blank",
      "short_answer",
      "essay",
    ]),
  );
  expect(questions.find((q) => q.type === "matching")?.matchItems).toHaveLength(3);
  expect(questions.find((q) => q.type === "single_choice")?.promptAssets).toHaveLength(
    1,
  );
  expect(offendingPaths(body, both)).toEqual([]);
});

test("E2E 9: a graded result with the score, answers and explanations hidden carries none of them", async ({
  browser,
}) => {
  test.setTimeout(180_000);
  const student = await browser.newPage();
  await signInAsStudent(student);
  const attemptId = await freshAttempt(student, ASSIGNMENT.everyType.hidden);
  await goToQuestion(student, 7);
  await student.getByRole("textbox", { name: "Bài làm của bạn" }).fill("Rice");
  await goToQuestion(student, 8);
  await student
    .getByRole("textbox", { name: "Bài luận của bạn" })
    .fill("Rivers feed the delta, so they matter more than new roads.");
  await expect(student.getByText("Đã lưu tất cả câu trả lời")).toBeVisible();
  await submitAttempt(student);

  // A graded essay is what would put a band and criteria on the result.
  const teacher = await browser.newPage();
  await signInAsAdmin(teacher);
  await gradeAttempt(teacher, attemptId, {
    shortAnswer: "1",
    bands: ["6,0", "6,5", "7,0", "6,0"],
    comment: "Clear view; give an example for each side.",
  });
  await teacher.close();

  const read = apiRead(student, `/app/attempts/${attemptId}/result`);
  await student.goto(`/app/attempts/${attemptId}/result`);
  const response = await read;
  expect(response.status()).toBe(200);
  const body = (await response.json()) as {
    review?: Record<string, unknown>;
    questions?: { type: string; answer?: unknown }[];
  };
  expect(body.review).toMatchObject({
    showScore: false,
    showCorrectAnswers: false,
    showExplanations: false,
  });
  // The essay was answered and graded, so its band was there to leak.
  const essay = body.questions?.find((question) => question.type === "essay");
  expect(essay?.answer).toBeTruthy();
  expect(offendingPaths(body, both)).toEqual([]);
});
