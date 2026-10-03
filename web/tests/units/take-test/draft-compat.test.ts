import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { saveAnswers } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { session, text } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";
const DRAFT_KEY = "quizzivy.answer-draft.att-1";
const FLAGS_KEY = "quizzivy.flags.att-1";

const V080_DRAFT =
  '{"studentId":"stu-1","sessionId":"ses-1","deadlineAt":1788253200000,"answers":{' +
  '"q1":{"type":"choice","optionIds":["o2","o3"]},' +
  '"q2":{"type":"true_false","value":false},' +
  '"q3":{"type":"text","value":"public green space"},' +
  '"q4":{"type":"fill_blank","values":{"b1":"went","b2":"gone"}}}}';
const V080_FLAGS = '["q2","q4"]';

const V080_ANSWERS = {
  q1: { type: "choice", optionIds: ["o2", "o3"] },
  q2: { type: "true_false", value: false },
  q3: { type: "text", value: "public green space" },
  q4: { type: "fill_blank", values: { b1: "went", b2: "gone" } },
};

const store = () => useTakeTestStore.getState();
const start = () => store().hydrate(session({ serverTime: now, deadlineAt: deadline }));

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  vi.mocked(saveAnswers)
    .mockReset()
    .mockResolvedValue({ serverTime: now, savedAt: now, deadlineAt: deadline });
  store().reset();
});

afterEach(() => {
  store().reset();
  localStorage.clear();
  sessionStorage.clear();
});

describe("what a v0.8.0 tab left behind", () => {
  it("is written for the deadline this test hydrates with", () => {
    expect(Date.parse(deadline)).toBe(1788253200000);
  });

  it("restores the answers of its local draft, every answer type, and sends them", async () => {
    localStorage.setItem(DRAFT_KEY, V080_DRAFT);
    start();

    expect(store().answers).toEqual(V080_ANSWERS);
    expect([...store().dirty].sort((a, b) => a.localeCompare(b))).toEqual([
      "q1",
      "q2",
      "q3",
      "q4",
    ]);

    await store().flush();
    expect(saveAnswers).toHaveBeenCalledWith("att-1", {
      sessionId: "ses-1",
      answers: V080_ANSWERS,
    });
    expect(localStorage.getItem(DRAFT_KEY)).toBeNull();
  });

  it("lets the draft win over an older answer the server holds", () => {
    localStorage.setItem(DRAFT_KEY, V080_DRAFT);
    store().hydrate(
      session({
        serverTime: now,
        deadlineAt: deadline,
        answers: { q3: text("an older answer"), q9: text("server only") },
      }),
    );
    expect(store().answers).toEqual({ ...V080_ANSWERS, q9: text("server only") });
  });

  it("restores its flags", () => {
    sessionStorage.setItem(FLAGS_KEY, V080_FLAGS);
    start();
    expect([...store().flags]).toEqual(["q2", "q4"]);
  });
});

describe("what this build writes", () => {
  it("is the same draft entry, byte for byte, under the same key", () => {
    start();
    store().setAnswer("q1", { type: "choice", optionIds: ["o2", "o3"] });
    store().setAnswer("q2", { type: "true_false", value: false });
    store().setAnswer("q3", text("public green space"));
    store().setAnswer("q4", { type: "fill_blank", values: { b1: "went", b2: "gone" } });

    expect(Object.keys(localStorage)).toEqual([DRAFT_KEY]);
    expect(localStorage.getItem(DRAFT_KEY)).toBe(V080_DRAFT);
  });

  it("is the same flags entry, byte for byte, under the same key", () => {
    start();
    store().toggleFlag("q2");
    store().toggleFlag("q4");

    expect(Object.keys(sessionStorage)).toContain(FLAGS_KEY);
    expect(sessionStorage.getItem(FLAGS_KEY)).toBe(V080_FLAGS);
  });
});
