import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { saveAnswers } from "@/features/take-test/api";
import { saveStrandedDraft } from "@/features/take-test/strandedDraft";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";
import { studentUser } from "@tests/support/fixtures";

vi.mock("@/features/take-test/api", () => ({ saveAnswers: vi.fn() }));

const save = vi.mocked(saveAnswers);

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";
const KEY = "quizzivy.answer-draft.att-1";
const OTHER_KEY = "quizzivy.answer-draft.att-2";

const V080_DRAFT =
  '{"studentId":"stu-1","sessionId":"ses-1","deadlineAt":1788253200000,"answers":{' +
  '"q1":{"type":"choice","optionIds":["o2","o3"]},' +
  '"q2":{"type":"true_false","value":false},' +
  '"q3":{"type":"text","value":"public green space"},' +
  '"q4":{"type":"fill_blank","values":{"b1":"went","b2":"gone"}}}}';

const V080_ANSWERS = {
  q1: { type: "choice", optionIds: ["o2", "o3"] },
  q2: { type: "true_false", value: false },
  q3: { type: "text", value: "public green space" },
  q4: { type: "fill_blank", values: { b1: "went", b2: "gone" } },
};

function draft(over: Record<string, unknown> = {}) {
  return JSON.stringify({
    studentId: "stu-1",
    sessionId: "ses-1",
    deadlineAt: Date.parse(deadline),
    answers: { q1: { type: "text", value: "typed offline" } },
    ...over,
  });
}

function signIn(id: string) {
  useAuthStore.setState({ user: { ...studentUser, id } });
}

beforeEach(() => {
  localStorage.clear();
  save
    .mockReset()
    .mockResolvedValue({ serverTime: now, savedAt: now, deadlineAt: deadline });
  signIn("stu-1");
});

afterEach(() => {
  useAuthStore.getState().clearSession();
  localStorage.clear();
});

describe("answers a closed tab left", () => {
  it("sends a draft an earlier session left, under that session, and removes it once the server has it", async () => {
    localStorage.setItem(KEY, draft());

    await saveStrandedDraft("att-1");

    expect(save.mock.calls).toStrictEqual([
      [
        "att-1",
        {
          sessionId: "ses-1",
          answers: { q1: { type: "text", value: "typed offline" } },
        },
      ],
    ]);
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it("sends what a v0.8.0 tab left, every answer type", async () => {
    localStorage.setItem(KEY, V080_DRAFT);

    await saveStrandedDraft("att-1");

    expect(save.mock.calls).toStrictEqual([
      ["att-1", { sessionId: "ses-1", answers: V080_ANSWERS }],
    ]);
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it.each([
    ["SESSION_SUPERSEDED", 409],
    ["DEADLINE_PASSED", 409],
    ["ATTEMPT_CLOSED", 409],
    ["VALIDATION_FAILED", 400],
    ["VALIDATION_FAILED", 413],
  ] as const)(
    "drops the draft and goes on when the server refuses it with %s (%i)",
    async (code, status) => {
      localStorage.setItem(KEY, draft());
      save.mockReset().mockRejectedValue(new ApiError({ status, code, message: "x" }));

      await expect(saveStrandedDraft("att-1")).resolves.toBeUndefined();

      expect(save).toHaveBeenCalledTimes(1);
      expect(localStorage.getItem(KEY)).toBeNull();
    },
  );

  it.each([
    ["a request that got no answer", new TypeError("Failed to fetch")],
    ["0 UNKNOWN", new ApiError({ status: 0, code: "UNKNOWN", message: "x" })],
    ["500 INTERNAL", new ApiError({ status: 500, code: "INTERNAL", message: "x" })],
    [
      "429 RATE_LIMITED",
      new ApiError({ status: 429, code: "RATE_LIMITED", message: "x" }),
    ],
    [
      "401 UNAUTHORIZED",
      new ApiError({ status: 401, code: "UNAUTHORIZED", message: "x" }),
    ],
    ["403 FORBIDDEN", new ApiError({ status: 403, code: "FORBIDDEN", message: "x" })],
    ["403 UNKNOWN", new ApiError({ status: 403, code: "UNKNOWN", message: "x" })],
    ["404 NOT_FOUND", new ApiError({ status: 404, code: "NOT_FOUND", message: "x" })],
    ["409 UNKNOWN", new ApiError({ status: 409, code: "UNKNOWN", message: "x" })],
    [
      "503 MAINTENANCE",
      new ApiError({ status: 503, code: "MAINTENANCE", message: "x" }),
    ],
  ])(
    "keeps the draft and fails when the server gave no verdict: %s",
    async (_, failure) => {
      const stored = draft();
      localStorage.setItem(KEY, stored);
      save.mockReset().mockRejectedValue(failure);

      await expect(saveStrandedDraft("att-1")).rejects.toBe(failure);

      expect(save).toHaveBeenCalledTimes(1);
      expect(localStorage.getItem(KEY)).toBe(stored);
    },
  );

  it("neither sends nor removes another student's draft", async () => {
    const stored = draft({ studentId: "stu-a" });
    localStorage.setItem(KEY, stored);
    signIn("stu-b");

    await saveStrandedDraft("att-1");

    expect(save).not.toHaveBeenCalled();
    expect(localStorage.getItem(KEY)).toBe(stored);
  });

  it("leaves a draft of another attempt alone", async () => {
    const stored = draft();
    localStorage.setItem(OTHER_KEY, stored);

    await saveStrandedDraft("att-1");

    expect(save).not.toHaveBeenCalled();
    expect(localStorage.getItem(OTHER_KEY)).toBe(stored);
  });

  it.each([
    ["nobody signed in", draft(), null],
    ["no draft", null, "stu-1"],
    ["a draft that does not parse", '{"studentId":"stu-1","answers":', "stu-1"],
    [
      "a draft of another shape",
      '{"studentId":"stu-1","sessionId":"ses-1","answers":{"q1":{"type":"essay"}}}',
      "stu-1",
    ],
    ["a draft with no answers", draft({ answers: {} }), "stu-1"],
  ])("sends nothing with %s", async (_, stored, studentId) => {
    if (stored !== null) localStorage.setItem(KEY, stored);
    if (studentId === null) useAuthStore.getState().clearSession();
    else signIn(studentId);

    await saveStrandedDraft("att-1");

    expect(save).not.toHaveBeenCalled();
    expect(localStorage.getItem(KEY)).toBe(stored);
  });

  it("does not remove a draft a live tab rewrote while the save was out", async () => {
    localStorage.setItem(KEY, draft());
    let release: () => void = () => {};
    save.mockReset().mockReturnValue(
      new Promise((resolve) => {
        release = () =>
          resolve({ serverTime: now, savedAt: now, deadlineAt: deadline });
      }),
    );

    const sending = saveStrandedDraft("att-1");
    expect(save).toHaveBeenCalledTimes(1);
    const rewritten = draft({
      sessionId: "ses-2",
      answers: { q2: { type: "text", value: "typed since" } },
    });
    localStorage.setItem(KEY, rewritten);
    release();
    await sending;

    expect(localStorage.getItem(KEY)).toBe(rewritten);
  });

  it("still sends a draft whose own deadline has passed, because the server may have extended it", async () => {
    localStorage.setItem(KEY, draft({ deadlineAt: 1 }));

    await saveStrandedDraft("att-1");

    expect(save).toHaveBeenCalledTimes(1);
    expect(localStorage.getItem(KEY)).toBeNull();
  });
});
