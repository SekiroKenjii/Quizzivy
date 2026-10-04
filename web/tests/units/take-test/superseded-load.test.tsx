import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  recordAudioPlay,
  saveAnswers,
  submitAttempt,
  type AttemptSession,
  type StudentQuestion,
} from "@/features/take-test/api";
import { writeDraft } from "@/features/take-test/draft";
import { useGroupPlaybackStore } from "@/features/take-test/groupPlayback";
import { FLUSH_DEBOUNCE_MS, useTakeTestStore } from "@/features/take-test/store";
import { ApiError } from "@/lib/api/errors";
import { session, text, viewport } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
}));

const BAR = "Bài này đang mở ở thiết bị khác. Bạn không thể sửa ở đây nữa.";
const PROMPT = "What should cities prioritise?";
const DRAFT = "quizzivy.answer-draft.att-1";

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";

const store = () => useTakeTestStore.getState();
const playback = () => useGroupPlaybackStore.getState();
const elsewhere = () =>
  new ApiError({ status: 409, code: "SESSION_SUPERSEDED", message: "elsewhere" });

function ordinary(answers: AttemptSession["answers"] = {}): AttemptSession {
  return session({ serverTime: now, deadlineAt: deadline, answers });
}

function ended(): AttemptSession {
  return session({ serverTime: now, deadlineAt: deadline, status: "submitted" });
}

function takenOver(paper: AttemptSession): AttemptSession {
  return { ...paper, beaconToken: "", superseded: true };
}

function storeDraft(sessionId: string, value: string): string {
  writeDraft("att-1", "stu-1", sessionId, Date.parse(deadline), {
    q1: text(value),
  });
  const stored = localStorage.getItem(DRAFT);
  if (stored === null) throw new Error("the draft was not stored");
  return stored;
}

function draftSession(): string | null {
  const stored = localStorage.getItem(DRAFT);
  return stored === null
    ? null
    : (JSON.parse(stored) as { sessionId: string }).sessionId;
}

function reply() {
  const at = new Date().toISOString();
  return {
    serverTime: at,
    savedAt: at,
    deadlineAt: new Date(store().deadlineAt).toISOString(),
  };
}

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  vi.mocked(getAttempt).mockReset();
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () => reply());
  vi.mocked(submitAttempt).mockReset().mockResolvedValue(ended().attempt);
  vi.mocked(recordAudioPlay).mockReset().mockResolvedValue({ plays: 1, maxPlays: 2 });
});

afterEach(() => {
  store().reset();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("a payload that says this tab's session was superseded", () => {
  it("locks a paper whose payload says it was superseded", async () => {
    store().hydrate(takenOver(ordinary()));
    expect(store().lock).toBe("superseded");

    store().setAnswer("q1", text("typed"));
    expect(store().answers).toEqual({});
    expect(store().dirty.size).toBe(0);

    await expect(store().flush()).resolves.toBe(false);
    expect(saveAnswers).not.toHaveBeenCalled();
  });

  it("reads no draft into a superseded paper, and forgets it", () => {
    storeDraft("ses-1", "typed before the takeover");

    store().hydrate(takenOver(ordinary({ q2: text("saved") })));

    expect(store().answers).toEqual({ q2: text("saved") });
    expect(store().dirty.size).toBe(0);
    expect(localStorage.getItem(DRAFT)).toBeNull();
  });

  it("locks a live tab that refetches and is told it was superseded, and keeps what it shows", () => {
    store().hydrate(ordinary());
    store().setAnswer("q1", text("typed here"));
    expect(draftSession()).toBe("ses-1");

    store().hydrate(takenOver(ordinary({ q2: text("saved elsewhere") })));

    expect(store().lock).toBe("superseded");
    expect(store().sessionId).toBe("ses-1");
    expect(store().answers).toEqual({
      q1: text("typed here"),
      q2: text("saved elsewhere"),
    });
    expect(localStorage.getItem(DRAFT)).toBeNull();
  });

  it("keeps a superseded tab locked through any number of refetches", () => {
    for (const load of [1, 2, 3]) {
      store().hydrate(takenOver(ordinary()));

      expect(store().lock, `load ${load}`).toBe("superseded");
      expect(playback().lock, `load ${load}`).toBe("superseded");
    }
  });

  it("leaves a draft another session of this browser wrote, through a superseded load and every refetch", () => {
    const stored = storeDraft("ses-2", "typed in the tab that took over");

    for (const load of [1, 2, 3]) {
      store().hydrate(takenOver(ordinary()));

      expect(localStorage.getItem(DRAFT), `load ${load}`).toBe(stored);
    }
    expect(store().sessionId).toBe("ses-1");
  });

  it("leaves the other tab's draft when a refused save locks this one", async () => {
    vi.useFakeTimers();
    vi.mocked(saveAnswers).mockRejectedValue(elsewhere());
    store().hydrate(ordinary());
    store().setAnswer("q1", text("typed here"));
    expect(draftSession()).toBe("ses-1");

    const stored = storeDraft("ses-2", "typed in the tab that took over");
    await vi.advanceTimersByTimeAsync(FLUSH_DEBOUNCE_MS);

    expect(saveAnswers).toHaveBeenCalledTimes(1);
    expect(store().lock).toBe("superseded");
    expect(localStorage.getItem(DRAFT)).toBe(stored);
  });
});

const questions: StudentQuestion[] = [
  { id: "q1", sectionId: "s1", type: "short_answer", prompt: PROMPT, points: 1 },
];

const listening: StudentQuestion[] = [
  {
    id: "q1",
    sectionId: "s1",
    type: "short_answer",
    prompt: PROMPT,
    points: 1,
    media: {
      id: "m1",
      kind: "audio",
      url: "https://assets.example/q1.mp3",
      mimeType: "audio/mpeg",
      bytes: 2048,
      durationMs: 10_004,
      originalFilename: "q1.mp3",
      createdAt: "2026-09-01T00:00:00.000Z",
    },
    audio: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: false },
  },
];

function paper(over: Partial<AttemptSession> = {}): AttemptSession {
  const at = new Date();
  return {
    ...session({
      serverTime: at.toISOString(),
      deadlineAt: new Date(at.getTime() + 3_600_000).toISOString(),
    }),
    questions,
    ...over,
  };
}

function mount() {
  const router = createMemoryRouter(
    [
      { path: "/app", element: <p>home</p> },
      { path: "/app/classes", element: <p>classes</p> },
      { path: "/app/attempts/:attemptId", element: <TakeTestPage /> },
    ],
    { initialEntries: ["/app/classes", "/app/attempts/att-1"], initialIndex: 1 },
  );
  render(<RouterProvider router={router} />);
  return router;
}

async function open() {
  const router = mount();
  await screen.findByText(PROMPT);
  return router;
}

const pass = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const submitButton = () =>
  within(screen.getByRole("banner")).queryByRole("button", { name: "Nộp bài" });

describe("the engine on a tab another device took over", () => {
  beforeEach(() => {
    viewport("desktop");
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  it("shows the read-only paper after a reload of a tab another device took over", async () => {
    vi.mocked(getAttempt).mockResolvedValue(takenOver(paper()));
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    await open();

    expect(screen.getByText(BAR)).toBeInTheDocument();
    expect(submitButton()).toBeNull();

    await user.type(screen.getByRole("textbox"), "parks");
    expect(screen.getByRole("textbox")).toHaveValue("");
    expect(store().answers).toEqual({});

    await pass(FLUSH_DEBOUNCE_MS + 100);
    expect(saveAnswers).not.toHaveBeenCalled();
  });

  it("stays read-only when the audio's retry refetches the attempt", async () => {
    Object.defineProperty(HTMLMediaElement.prototype, "play", {
      configurable: true,
      value: vi.fn(() => Promise.reject(new DOMException("gone", "NotSupportedError"))),
    });
    Object.defineProperty(HTMLMediaElement.prototype, "pause", {
      configurable: true,
      value: vi.fn(),
    });
    Object.defineProperty(HTMLMediaElement.prototype, "paused", {
      configurable: true,
      get: () => true,
    });
    vi.mocked(getAttempt).mockResolvedValue(paper({ questions: listening }));
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    await open();

    vi.mocked(saveAnswers).mockRejectedValueOnce(elsewhere());
    await user.type(screen.getByRole("textbox"), "parks");
    await act(() => store().flush());
    expect(store().lock).toBe("superseded");
    expect(store().beaconToken).toBe("beacon");
    expect(screen.getByText(BAR)).toBeInTheDocument();

    vi.mocked(getAttempt).mockResolvedValue(takenOver(paper({ questions: listening })));
    await user.click(screen.getByRole("button", { name: "Phát" }));
    await user.click(await screen.findByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(store().beaconToken).toBe(""));

    expect(getAttempt).toHaveBeenCalledTimes(2);
    expect(store().lock).toBe("superseded");
    expect(screen.getByText(BAR)).toBeInTheDocument();
    expect(submitButton()).toBeNull();
    expect(screen.getByRole("textbox")).toHaveValue("parks");

    await pass(FLUSH_DEBOUNCE_MS + 100);
    expect(saveAnswers).toHaveBeenCalledTimes(1);
  });

  it("still opens an ordinary payload unlocked", async () => {
    vi.mocked(getAttempt).mockResolvedValue(paper());
    await open();

    expect(store().lock).toBeNull();
    expect(screen.queryByText(BAR)).toBeNull();
    expect(submitButton()).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBeEnabled();
  });
});
