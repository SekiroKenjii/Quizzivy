import { expect, it } from "vitest";
import { http } from "msw";
import {
  scanGradingCandidates,
  gradingGroupKey,
  mergeQueueOrder,
  nextOpenItem,
  scoreOptions,
} from "@/features/attempts/pages/teacher/gradingRecovery";
import type { AttemptListRow, GradingQueueItem } from "@/features/attempts/api";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { BASE, ASSIGNMENT_ID, STUDENT_ID, ESSAY_ID } from "./fixtures";

function row(
  index: number,
  status: "submitted" | "timed_out" = "submitted",
): AttemptListRow {
  return {
    id: `018f0000-0000-7000-8000-${String(index).padStart(12, "0")}`,
    assignmentId: ASSIGNMENT_ID,
    studentId: STUDENT_ID,
    studentName: "Cùng tên",
    testTitle: "Bài giao",
    status,
    pendingManual: 0,
    flagged: false,
  };
}

it("exhausts both statuses beyond100 with page-size metadata and deduplicates overlapping UUIDs", async () => {
  const calls: string[] = [];
  server.use(
    http.get(`${BASE}/teacher/attempts`, ({ request }) => {
      const url = new URL(request.url);
      const page = Number(url.searchParams.get("page") ?? 1);
      const status =
        url.searchParams.get("status") === "submitted" ? "submitted" : "timed_out";
      expect(url.searchParams.get("limit")).toBe("100");
      expect(url.searchParams.get("pendingGrading")).toBe("false");
      calls.push(`${status}:${page}`);
      const start = page === 1 ? 0 : 99;
      const items = Array.from({ length: page === 1 ? 100 : 2 }, (_, index) =>
        row(start + index + (status === "submitted" ? 1 : 100), status),
      );
      return contractJson("/teacher/attempts", "get", 200, {
        items,
        total: 102,
        page,
        pageSize: 100,
      });
    }),
  );
  const result = await scanGradingCandidates(new AbortController().signal, "", "");
  expect(calls).toEqual(["submitted:1", "submitted:2", "timed_out:1", "timed_out:2"]);
  expect(result).toHaveLength(200);
  expect(result.at(-1)?.id).toBe(row(200).id);
});

it("applies assignment and student filters locally after exhaustive discovery", async () => {
  const calls: URL[] = [];
  const other = { ...row(2), assignmentId: "018f0000-0000-7000-8000-000000000002" };
  const otherStudent = { ...row(3), studentId: "018f0000-0000-7000-8000-000000000003" };
  server.use(
    http.get(`${BASE}/teacher/attempts`, ({ request }) => {
      const url = new URL(request.url);
      calls.push(url);
      return contractJson("/teacher/attempts", "get", 200, {
        items: [row(1), other, otherStudent],
        total: 3,
        page: 1,
        pageSize: 100,
      });
    }),
  );
  expect(
    await scanGradingCandidates(
      new AbortController().signal,
      ASSIGNMENT_ID,
      STUDENT_ID,
    ),
  ).toEqual([row(1)]);
  expect(calls).toHaveLength(2);
  expect(
    calls.every(
      (url) =>
        !url.searchParams.has("assignmentId") && !url.searchParams.has("studentId"),
    ),
  ).toBe(true);
});

it("refuses incomplete empty pages rather than reporting a successful scan", async () => {
  server.use(
    http.get(`${BASE}/teacher/attempts`, () =>
      contractJson("/teacher/attempts", "get", 200, {
        items: [],
        total: 101,
        page: 1,
        pageSize: 100,
      }),
    ),
  );
  await expect(
    scanGradingCandidates(new AbortController().signal, "", ""),
  ).rejects.toThrow("Incomplete recovery page");
});

it("an aborted scan cannot send any discovery request", async () => {
  const controller = new AbortController();
  controller.abort();
  await expect(scanGradingCandidates(controller.signal, "", "")).rejects.toMatchObject({
    name: "AbortError",
  });
});

it("groups by immutable IDs and keeps score zero with a numeric fallback above nine choices", () => {
  const item: GradingQueueItem = {
    attemptId: row(1).id,
    questionId: ESSAY_ID,
    assignmentId: ASSIGNMENT_ID,
    assignmentTitle: "Même titre",
    studentId: STUDENT_ID,
    studentName: "Même nom",
    questionNumber: 1,
    type: "short_answer",
    prompt: "Question",
    answer: { type: "text", value: "Answer" },
    points: 1,
    score: null,
    comment: null,
  };
  expect(gradingGroupKey(item, "student")).toBe(STUDENT_ID);
  expect(gradingGroupKey({ ...item, assignmentId: row(2).id }, "question")).not.toBe(
    gradingGroupKey(item, "question"),
  );
  expect(gradingGroupKey({ ...item, studentId: row(3).id }, "student")).not.toBe(
    gradingGroupKey(item, "student"),
  );
  expect(scoreOptions(1)).toEqual([0, 0.5, 1]);
  expect(scoreOptions(4)).toHaveLength(9);
  expect(scoreOptions(5)).toEqual([]);
});

it("offers nine keyed scores up to four points and a number field from four and a half", () => {
  expect(scoreOptions(4)).toEqual([0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5, 4]);
  expect(scoreOptions(4.5)).toEqual([]);
  expect(scoreOptions(0)).toEqual([0]);
});

it("keeps each answer where the queue first showed it as the server's list changes", () => {
  expect(mergeQueueOrder([], ["a", "b", "c"])).toEqual(["a", "b", "c"]);
  expect(mergeQueueOrder(["a", "b", "c"], ["b", "c"])).toEqual(["a", "b", "c"]);
  expect(mergeQueueOrder(["a", "b", "c"], ["a", "x", "c"])).toEqual([
    "a",
    "x",
    "b",
    "c",
  ]);
  expect(mergeQueueOrder(["a", "b"], ["y", "b", "z"])).toEqual(["y", "a", "b", "z"]);
});

it("nextOpenItem looks after the current item first, wraps round, and answers null when nothing is open", () => {
  const open = new Set(["a", "d"]);
  const items = ["a", "b", "c", "d"];
  expect(nextOpenItem(items, 1, (item) => open.has(item))).toBe("d");
  expect(nextOpenItem(items, 3, (item) => open.has(item))).toBe("a");
  expect(nextOpenItem(items, -1, (item) => open.has(item))).toBe("a");
  expect(nextOpenItem(items, 0, () => false)).toBeNull();
  expect(nextOpenItem([], -1, () => true)).toBeNull();
});
