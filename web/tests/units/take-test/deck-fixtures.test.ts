import { describe, expect, it } from "vitest";
import { contractJson } from "@tests/support/contractResponse";
import { answered } from "@/features/take-test/answered";
import { DECK_LEFT_MS, deckSaved, deckSession } from "./deckSession";

const NOW = new Date("2026-09-22T05:00:00.000Z");

describe("the deck's take-test fixture", () => {
  const session = deckSession(NOW);

  it("is a session, a save reply and a submitted attempt the contract accepts", () => {
    expect(() => contractJson("/app/attempts/{id}", "get", 200, session)).not.toThrow();
    expect(() =>
      contractJson(
        "/app/attempts/{id}/answers",
        "patch",
        200,
        deckSaved(NOW, session.attempt.deadlineAt),
      ),
    ).not.toThrow();
    expect(() =>
      contractJson("/app/attempts/{id}/submit", "post", 200, {
        ...session.attempt,
        status: "submitted",
        submittedAt: NOW.toISOString(),
      }),
    ).not.toThrow();
  });

  it("stays valid with the variations the dialogs are compared in", () => {
    for (const body of [
      {
        ...session,
        attempt: {
          ...session.attempt,
          integrity: { focusLossCount: 2, flagged: false },
        },
      },
      { ...session, integrity: { ...session.integrity, minAwayMs: 0 } },
      { ...session, integrity: { ...session.integrity, requireFullscreen: true } },
      {
        ...session,
        integrity: {
          ...session.integrity,
          maxFocusLoss: 1,
          onLimitExceeded: "auto_submit",
        },
      },
      { ...session, integrity: { ...session.integrity, maxFocusLoss: -1 } },
    ]) {
      expect(() => contractJson("/app/attempts/{id}", "get", 200, body)).not.toThrow();
    }
    expect(() =>
      contractJson("/app/attempts/{id}", "get", 200, {
        ...session,
        integrity: { ...session.integrity, onLimitExceeded: "expel" },
      }),
    ).toThrow(/does not match/);
  });

  it("has refusals the contract accepts for a paper taken over, out of time or ended", () => {
    for (const [code, message] of [
      ["SESSION_SUPERSEDED", "Bài làm này đã được mở ở nơi khác."],
      ["DEADLINE_PASSED", "Đã hết giờ làm bài."],
      ["ATTEMPT_CLOSED", "Bài làm này đã được nộp."],
    ]) {
      const body = {
        error: { code, message, requestId: "018f0000-0000-7000-8000-0000000000aa" },
      };
      expect(() =>
        contractJson("/app/attempts/{id}/answers", "patch", 409, body),
      ).not.toThrow();
      expect(() =>
        contractJson("/app/attempts/{id}/submit", "post", 409, body),
      ).not.toThrow();
    }
  });

  it("carries the deck's numbers: eight questions, three answered, 38:12, two absences", () => {
    expect(session.questions).toHaveLength(8);
    expect(
      session.questions.filter((question) =>
        answered(question, session.answers[question.id]),
      ),
    ).toHaveLength(3);
    expect(Date.parse(session.attempt.deadlineAt) - NOW.getTime()).toBe(DECK_LEFT_MS);
    expect(DECK_LEFT_MS).toBe(38 * 60_000 + 12_000);
    expect(session.integrity).toMatchObject({
      maxFocusLoss: 2,
      onLimitExceeded: "flag",
      blockCopyPaste: true,
    });
  });
});
