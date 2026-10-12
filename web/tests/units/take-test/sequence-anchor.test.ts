import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { http } from "msw";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import type { AttemptSession } from "@/features/take-test/api";
import { DECK_ATTEMPT, deckSession } from "./deckSession";

const BASE = "http://localhost:8080";
const ATTEMPT = DECK_ATTEMPT;
const T0 = Date.parse("2026-09-01T08:00:00.000Z");
const SERVER_NOW = T0 + 4000;
const DEVICE_NOW = SERVER_NOW - 3_600_000;

function paper(over: Partial<AttemptSession> = {}): AttemptSession {
  const deck = deckSession(new Date(SERVER_NOW));
  return {
    ...deck,
    attempt: { ...deck.attempt, startedAt: new Date(T0).toISOString() },
    serverTime: new Date(SERVER_NOW).toISOString(),
    ...over,
  };
}

function answerReads(body: AttemptSession) {
  server.use(
    http.get(`${BASE}/app/attempts/${ATTEMPT}`, () =>
      contractJson("/app/attempts/{id}", "get", 200, body),
    ),
  );
}

async function load() {
  vi.resetModules();
  const [engine, buffer] = await Promise.all([
    import("@/features/take-test/api"),
    import("@/features/integrity/buffer"),
  ]);
  return {
    getAttempt: engine.getAttempt,
    beginSession: buffer.beginSession,
    record: buffer.record,
    pending: buffer.pending,
  };
}

async function numberAfterReading(body: AttemptSession) {
  const { getAttempt, beginSession, record, pending } = await load();
  answerReads(body);

  const session = await getAttempt(ATTEMPT);
  beginSession(ATTEMPT, session.sessionId);
  record(ATTEMPT, "paste");

  return pending().map((event) => event.clientSeq);
}

beforeEach(() => {
  sessionStorage.clear();
  localStorage.clear();
  vi.useFakeTimers({ toFake: ["Date"], now: DEVICE_NOW });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the sequence a read of the attempt anchors", () => {
  it("anchors the sequence on the server's clock when the attempt is read", async () => {
    expect(await numberAfterReading(paper())).toEqual([4000]);
  });

  it("anchors it for a superseded payload too", async () => {
    expect(
      await numberAfterReading(paper({ beaconToken: "", superseded: true })),
    ).toEqual([4000]);
  });
});
