import { beforeEach, describe, expect, it, vi } from "vitest";
import { http } from "msw";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import type { AttemptSession } from "@/features/take-test/api";
import { DECK_ATTEMPT, deckSession } from "./deckSession";

const BASE = "http://localhost:8080";
const ATTEMPT = DECK_ATTEMPT;
const ASSIGNMENT = "018f0000-0000-7000-8000-0000000000d1";
const A = "018f0000-0000-7000-8000-00000000a001";
const B = "018f0000-0000-7000-8000-00000000b002";
const KEY = `quizzivy.session.${ATTEMPT}`;
const NOW = new Date("2026-09-22T05:00:00.000Z");

let named: (string | null)[] = [];

function paper(over: Partial<AttemptSession> = {}): AttemptSession {
  return { ...deckSession(NOW), sessionId: A, ...over };
}

function answerReads(body: AttemptSession) {
  server.use(
    http.get(`${BASE}/app/attempts/${ATTEMPT}`, ({ request }) => {
      named.push(new URL(request.url).searchParams.get("session"));
      return contractJson("/app/attempts/{id}", "get", 200, body);
    }),
  );
}

function answerResumes(body: AttemptSession) {
  server.use(
    http.post(`${BASE}/app/assignments/${ASSIGNMENT}/attempts`, () =>
      contractJson("/app/assignments/{id}/attempts", "post", 200, body),
    ),
  );
}

async function load() {
  vi.resetModules();
  const [engine, assignments, held] = await Promise.all([
    import("@/features/take-test/api"),
    import("@/features/assignments/api"),
    import("@/features/take-test/heldSession"),
  ]);
  return {
    getAttempt: engine.getAttempt,
    startOrResumeAttempt: engine.startOrResumeAttempt,
    continueAttempt: assignments.continueAttempt,
    heldSession: held.heldSession,
    holdSession: held.holdSession,
  };
}

beforeEach(() => {
  sessionStorage.clear();
  localStorage.clear();
  named = [];
});

describe("the session a tab names when it reads its attempt", () => {
  it("names no session on the first read, and holds the one it is given", async () => {
    const { getAttempt, heldSession } = await load();
    answerReads(paper());

    const session = await getAttempt(ATTEMPT);

    expect(named).toEqual([null]);
    expect(session.sessionId).toBe(A);
    expect(heldSession(ATTEMPT)).toBe(A);
    expect(sessionStorage.getItem(KEY)).toBe(A);
  });

  it("names the session it holds on the next read", async () => {
    const { getAttempt } = await load();
    answerReads(paper());

    await getAttempt(ATTEMPT);
    await getAttempt(ATTEMPT);

    expect(named).toEqual([null, A]);
  });

  it("does not hold a session from a payload that says superseded", async () => {
    const { getAttempt, heldSession, holdSession } = await load();
    holdSession(ATTEMPT, A);
    answerReads(paper({ sessionId: A, beaconToken: "", superseded: true }));

    const session = await getAttempt(ATTEMPT);

    expect(named).toEqual([A]);
    expect(session.superseded).toBe(true);
    expect(heldSession(ATTEMPT)).toBe(A);

    answerReads(paper({ sessionId: B, beaconToken: "", superseded: true }));
    await getAttempt(ATTEMPT);

    expect(named).toEqual([A, A]);
    expect(heldSession(ATTEMPT)).toBe(A);
    expect(sessionStorage.getItem(KEY)).toBe(A);
  });

  it("does not hold a session of an attempt that has ended", async () => {
    const { getAttempt, heldSession } = await load();
    const ended = paper();
    answerReads({
      ...ended,
      attempt: {
        ...ended.attempt,
        status: "submitted",
        submittedAt: NOW.toISOString(),
      },
    });

    await getAttempt(ATTEMPT);

    expect(named).toEqual([null]);
    expect(heldSession(ATTEMPT)).toBeNull();
    expect(sessionStorage.getItem(KEY)).toBeNull();
  });

  it("holds the session a start or a resume returns", async () => {
    const { startOrResumeAttempt, heldSession } = await load();
    answerResumes(paper({ sessionId: B }));

    const session = await startOrResumeAttempt(ASSIGNMENT);

    expect(session.sessionId).toBe(B);
    expect(heldSession(ATTEMPT)).toBe(B);
    expect(sessionStorage.getItem(KEY)).toBe(B);
  });

  it("holds the session Continue returns, so the engine's read names it", async () => {
    const { continueAttempt, getAttempt, heldSession, holdSession } = await load();
    holdSession(ATTEMPT, A);
    answerResumes(paper({ sessionId: B }));
    answerReads(paper({ sessionId: B }));

    const session = await continueAttempt(ASSIGNMENT, ATTEMPT);
    expect(session.sessionId).toBe(B);
    expect(heldSession(ATTEMPT)).toBe(B);

    await getAttempt(ATTEMPT);

    expect(named).toEqual([B]);
  });

  it("names the session it held before a reload", async () => {
    sessionStorage.setItem(KEY, A);
    const { getAttempt } = await load();
    answerReads(paper());

    await getAttempt(ATTEMPT);

    expect(named).toEqual([A]);
  });
});
