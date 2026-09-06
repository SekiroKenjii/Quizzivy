import { expect, test } from "@playwright/test";
import { ASSIGNMENT, freshAttempt, signInAsStudent } from "./support/live";

/**
 * E2E 9 (§14, §13.5): the payload a student's browser receives for their own
 * attempt carries no `isCorrect`, `sampleAnswer`, `transcript` or
 * `acceptedAnswers`, at any depth. The Go side pins the same thing on the
 * domain type; this reads the bytes that actually crossed the wire.
 */
const BANNED = new Set(["iscorrect", "sampleanswer", "transcript", "acceptedanswers"]);

function offendingPaths(node: unknown, path = "$"): string[] {
  if (Array.isArray(node)) {
    return node.flatMap((child, i) => offendingPaths(child, `${path}[${i}]`));
  }
  if (node !== null && typeof node === "object") {
    return Object.entries(node as Record<string, unknown>).flatMap(([key, child]) => {
      const here = `${path}.${key}`;
      const own = BANNED.has(key.toLowerCase()) ? [here] : [];
      return [...own, ...offendingPaths(child, here)];
    });
  }
  return [];
}

test("E2E 9: GET /app/attempts/:id contains no part of the grading key", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await signInAsStudent(page);
  const attemptId = await freshAttempt(page, ASSIGNMENT.payload);

  const [response] = await Promise.all([
    // The API's response, not the SPA document that shares the path.
    page.waitForResponse(
      (r) =>
        r.request().method() === "GET" &&
        new URL(r.url()).origin === "http://localhost:8080" &&
        new URL(r.url()).pathname === `/app/attempts/${attemptId}`,
    ),
    page.reload(),
  ]);
  expect(response.status()).toBe(200);
  const body: unknown = await response.json();

  // The paper is really there, so an empty payload cannot pass by accident.
  const questions = (body as { questions?: unknown[] }).questions ?? [];
  expect(questions.length).toBeGreaterThan(0);
  expect(offendingPaths(body)).toEqual([]);
});
