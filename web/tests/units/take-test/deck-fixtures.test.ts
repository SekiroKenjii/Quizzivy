import { describe, expect, it } from "vitest";
import { contractJson } from "@tests/support/contractResponse";
import { answered } from "@/features/take-test/answered";
import type { AttemptSession, StudentGroup } from "@/features/take-test/api";
import {
  DECK_LEFT_MS,
  DECK_MOCK_PROMPT,
  deckListeningSession,
  deckMockTestSession,
  deckPassageSession,
  deckSaved,
  deckSession,
} from "./deckSession";

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

type Node = { type?: string; assetId?: string; id?: string } & Record<string, unknown>;

function nodesOf(value: unknown): Node[] {
  if (Array.isArray(value)) return value.flatMap(nodesOf);
  if (value === null || typeof value !== "object") return [];
  const node = value as Node;
  return [node, ...Object.values(node).flatMap(nodesOf)];
}

function audioNodes(group: StudentGroup): string[] {
  return group.stimuli.flatMap((material) =>
    nodesOf(material.content)
      .filter((node) => node.type === "audio")
      .map((node) => node.assetId ?? ""),
  );
}

function questionsOf(paper: AttemptSession, group: StudentGroup) {
  return group.questionIds.map((id) => paper.questions.find((item) => item.id === id));
}

/** The server's group rule, which the contract cannot state (group_validation.go). */
function expectAcceptedGroup(paper: AttemptSession, group: StudentGroup) {
  const audio = audioNodes(group);
  const recorded = group.recordings.map((recording) => recording.assetId);
  expect(new Set(recorded).size).toBe(recorded.length);
  for (const assetId of recorded) expect(audio).toContain(assetId);
  for (const assetId of audio) expect(recorded).toContain(assetId);
  for (const assetId of audio)
    expect(group.assets.map((asset) => asset.id)).toContain(assetId);

  const members = questionsOf(paper, group);
  for (const member of members) expect(member?.sectionId).toBe(group.sectionId);
  for (const material of group.stimuli) {
    const gaps = nodesOf(material.content)
      .filter((node) => node.type === "gap")
      .map((node) => node.id);
    expect(material.gaps).toHaveLength(gaps.length);
    expect(new Set(material.gaps.map((gap) => gap.gapId))).toEqual(new Set(gaps));
    for (const gap of material.gaps) {
      expect(group.questionIds).toContain(gap.questionId);
      if (gap.kind === "question") {
        const target = paper.questions.find((item) => item.id === gap.questionId);
        expect(["single_choice", "multiple_choice", "true_false"]).toContain(
          target?.type,
        );
      }
    }
    const targets = material.gaps.map((gap) => gap.questionId);
    expect(new Set(targets).size).toBe(targets.length);
  }
}

const LEAKS = [
  "isCorrect",
  "sampleAnswer",
  "acceptedAnswers",
  "transcript",
  "ans",
  "answer",
];

describe("the deck's three papers", () => {
  const papers = {
    s1: deckPassageSession(NOW),
    s9: deckMockTestSession(NOW),
    s2: deckListeningSession(NOW),
  };

  it("are sessions the contract accepts", () => {
    for (const paper of Object.values(papers)) {
      expect(() => contractJson("/app/attempts/{id}", "get", 200, paper)).not.toThrow();
    }
  });

  it("hold groups the server would accept", () => {
    for (const paper of Object.values(papers)) {
      for (const group of paper.groups ?? []) expectAcceptedGroup(paper, group);
    }
  });

  it("would refuse a recording with no audio node, and an audio node with no recording", () => {
    const paper = papers.s2;
    const group = paper.groups?.[0];
    if (group === undefined) throw new Error("s2 has no group");
    expect(() => expectAcceptedGroup(paper, { ...group, stimuli: [] })).toThrow();
    expect(() => expectAcceptedGroup(paper, { ...group, recordings: [] })).toThrow();
  });

  it("carry no answer key and no answers", () => {
    for (const paper of [papers.s9, papers.s2]) {
      const keys = nodesOf(paper.questions).flatMap((node) => Object.keys(node));
      for (const leak of LEAKS) expect(keys).not.toContain(leak);
      expect(paper.answers).toEqual({});
    }
  });

  it("give s9 34 questions in four sections, the cloze as five gaps bound to 23 to 27", () => {
    const paper = papers.s9;
    expect(paper.questions).toHaveLength(34);
    expect(paper.sections).toHaveLength(4);
    expect(
      paper.sections.map(
        (section) =>
          paper.questions.filter((item) => item.sectionId === section.id).length,
      ),
    ).toEqual([4, 16, 10, 4]);
    const [reading] = paper.groups ?? [];
    expect(reading?.questionIds).toEqual(
      paper.questions.slice(22, 30).map((q) => q.id),
    );
    expect(reading?.stimuli).toHaveLength(1);
    expect(reading?.stimuli[0]?.gaps.map((gap) => gap.questionId)).toEqual(
      paper.questions.slice(22, 27).map((q) => q.id),
    );
    expect(paper.questions.slice(22, 27).map((q) => q.prompt)).toEqual(
      Array(5).fill(DECK_MOCK_PROMPT),
    );
    expect(
      paper.questions.slice(27, 30).map((q) => q.options?.map((o) => o.text)),
    ).toEqual(Array(3).fill(["True", "False", "Not given"]));
    expect(paper.questions.slice(30).map((q) => q.type)).toEqual(
      Array(4).fill("fill_blank"),
    );
  });

  it("give s2 two listening parts, each a material that is only its recording, of two plays, the second not to be paused", () => {
    const paper = papers.s2;
    expect(paper.questions).toHaveLength(10);
    const groups = paper.groups ?? [];
    expect(groups.map((group) => group.questionIds.length)).toEqual([5, 5]);
    for (const group of groups) {
      expect(group.stimuli).toHaveLength(1);
      expect(nodesOf(group.stimuli[0]?.content).filter((node) => node.type)).toEqual([
        expect.objectContaining({ type: "audio" }),
      ]);
      expect(group.recordings.map((recording) => recording.policy.maxPlays)).toEqual([
        2,
      ]);
    }
    expect(
      groups.flatMap((group) => group.recordings.map((r) => r.policy.allowPause)),
    ).toEqual([true, false]);
    expect(paper.questions.slice(5).map((q) => q.type)).toEqual([
      ...Array(4).fill("single_choice"),
      "short_answer",
    ]);
  });
});
