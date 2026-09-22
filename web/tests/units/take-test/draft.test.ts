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
