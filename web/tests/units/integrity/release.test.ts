import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  beginSession,
  clearSession,
  drain,
  pending,
  record,
  type Buffered,
} from "@/features/integrity/buffer";
import { releaseSession } from "@/features/integrity/release";

const ATTEMPT = "att-1";
const KEY = "quizzivy.integrity." + ATTEMPT;

let sent: { url: string; body: Blob }[] = [];
let taken = true;

beforeEach(() => {
  sessionStorage.clear();
  clearSession(ATTEMPT);
  sent = [];
  taken = true;
  Object.defineProperty(navigator, "sendBeacon", {
    configurable: true,
    value: vi.fn((url: string, body: Blob) => {
      sent.push({ url, body });
      return taken;
    }),
  });
});

function stored(): Buffered | null {
  return JSON.parse(sessionStorage.getItem(KEY) ?? "null") as Buffered | null;
}

function bufferTwo() {
  beginSession(ATTEMPT, "ses-1");
  record(ATTEMPT, "window_blur");
  record(ATTEMPT, "window_focus");
}

const numbered = (events: { kind: string; clientSeq: number }[] | undefined) =>
  events?.map((event) => [event.kind, event.clientSeq]);

describe("releasing the buffer when the engine unmounts", () => {
  it("hands what is buffered to the beacon and keeps the sequence", async () => {
    bufferTwo();

    releaseSession({
      attemptId: ATTEMPT,
      sessionId: "ses-1",
      beaconToken: "tok",
      ended: false,
    });

    expect(sent).toHaveLength(1);
    expect(sent[0]?.url).toMatch(/\/app\/attempts\/att-1\/events$/);
    const body = JSON.parse(await sent[0]!.body.text()) as {
      beaconToken: string;
      sessionId: string;
      events: { kind: string; clientSeq: number }[];
    };
    expect(body.beaconToken).toBe("tok");
    expect(body.sessionId).toBe("ses-1");
    expect(numbered(body.events)).toEqual([
      ["window_blur", 0],
      ["window_focus", 1],
    ]);
    expect(stored()).toEqual({ sessionId: "ses-1", nextSeq: 2, events: [] });
    expect(pending()).toEqual([]);
  });

  it("posts nothing when nothing is buffered", () => {
    bufferTwo();
    drain(ATTEMPT);

    releaseSession({
      attemptId: ATTEMPT,
      sessionId: "ses-1",
      beaconToken: "tok",
      ended: false,
    });

    expect(navigator.sendBeacon).not.toHaveBeenCalled();
    expect(stored()).toEqual({ sessionId: "ses-1", nextSeq: 2, events: [] });
  });

  it("keeps the events when the browser does not take the beacon", () => {
    taken = false;
    bufferTwo();

    releaseSession({
      attemptId: ATTEMPT,
      sessionId: "ses-1",
      beaconToken: "tok",
      ended: false,
    });

    expect(navigator.sendBeacon).toHaveBeenCalledTimes(1);
    expect(stored()?.nextSeq).toBe(2);
    expect(numbered(stored()?.events)).toEqual([
      ["window_blur", 0],
      ["window_focus", 1],
    ]);
    expect(pending()).toEqual([]);

    beginSession(ATTEMPT, "ses-1");
    expect(numbered(pending())).toEqual([
      ["window_blur", 0],
      ["window_focus", 1],
    ]);
  });

  it.each<[string, string | null, string]>([
    ["no session", null, "tok"],
    ["no token", "ses-1", ""],
  ])(
    "keeps the events when there is no session or no token (%s)",
    (_missing, sessionId, beaconToken) => {
      bufferTwo();

      releaseSession({ attemptId: ATTEMPT, sessionId, beaconToken, ended: false });

      expect(navigator.sendBeacon).not.toHaveBeenCalled();
      expect(stored()?.nextSeq).toBe(2);
      expect(numbered(stored()?.events)).toEqual([
        ["window_blur", 0],
        ["window_focus", 1],
      ]);
      expect(pending()).toEqual([]);
    },
  );

  it("forgets the attempt once it has ended, and sends nothing", () => {
    bufferTwo();

    releaseSession({
      attemptId: ATTEMPT,
      sessionId: "ses-1",
      beaconToken: "tok",
      ended: true,
    });

    expect(sessionStorage.getItem(KEY)).toBeNull();
    expect(navigator.sendBeacon).not.toHaveBeenCalled();
    expect(pending()).toEqual([]);
  });
});
