import { afterEach, expect, it } from "vitest";
import { clearAnswerDrafts, readDraft, writeDraft } from "@/features/take-test/draft";

afterEach(clearAnswerDrafts);

it("only restores a draft for its student and active session", () => {
  const answers = { q1: { type: "text" as const, value: "pending answer" } };
  writeDraft("attempt", "student", "session", 2000, answers);
  expect(readDraft("attempt", "another-student", "session", 1000)).toEqual({});
  expect(readDraft("attempt", "student", "another-session", 1000)).toEqual({});
  expect(readDraft("attempt", "student", "session", 1000)).toEqual(answers);
});

it("does not resurrect expired or already confirmed answers", () => {
  writeDraft("attempt", "student", "session", 2000, {
    q1: { type: "text", value: "old" },
  });
  expect(readDraft("attempt", "student", "session", 2000)).toEqual({});
  expect(localStorage.getItem("quizzivy.answer-draft.attempt")).toBeNull();
  writeDraft("attempt", "student", "session", 2000, {
    q1: { type: "text", value: "pending" },
  });
  writeDraft("attempt", "student", "session", 2000, {});
  expect(localStorage.getItem("quizzivy.answer-draft.attempt")).toBeNull();
});

it("keeps a matching answer", () => {
  const answers = {
    q1: { type: "matching" as const, pairs: { i1: "o2", i2: "o1" } },
  };
  writeDraft("attempt", "student", "session", 2000, answers);
  expect(readDraft("attempt", "student", "session", 1000)).toEqual(answers);
});

it("drops an entry it cannot read and keeps the rest of the draft", () => {
  localStorage.setItem(
    "quizzivy.answer-draft.attempt",
    JSON.stringify({
      studentId: "student",
      sessionId: "session",
      deadlineAt: 2000,
      answers: {
        q1: { type: "text", value: "kept" },
        q2: { type: "drawing", strokes: [] },
        q3: { type: "choice", optionIds: "o1" },
        q4: { type: "matching", pairs: { i1: 7 } },
        q5: null,
        q6: { type: "choice", optionIds: ["o2", "o1"] },
      },
    }),
  );
  expect(readDraft("attempt", "student", "session", 1000)).toEqual({
    q1: { type: "text", value: "kept" },
    q6: { type: "choice", optionIds: ["o2", "o1"] },
  });
});

it("still clears a draft whose frame does not parse", () => {
  localStorage.setItem(
    "quizzivy.answer-draft.attempt",
    JSON.stringify({ studentId: "student", sessionId: "session", answers: {} }),
  );
  expect(readDraft("attempt", "student", "session", 1000)).toEqual({});
  expect(localStorage.getItem("quizzivy.answer-draft.attempt")).toBeNull();
});
