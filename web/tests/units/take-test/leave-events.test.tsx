import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  saveAnswers,
  submitAttempt,
  type StudentQuestion,
} from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { pending } from "@/features/integrity/buffer";
import { session, viewport } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
}));

const PROMPT = "What should cities prioritise?";
const BUFFER = "quizzivy.integrity.att-1";
const ABSENCE = ["window_blur", "window_focus"];

const questions: StudentQuestion[] = [
  { id: "q1", sectionId: "s1", type: "short_answer", prompt: PROMPT, points: 1 },
];

interface Beacon {
  url: string;
  beaconToken: string;
  sessionId: string;
  events: { kind: string; clientSeq: number }[];
}

let sent: { url: string; body: Blob }[] = [];

const store = () => useTakeTestStore.getState();

function paper(sessionId = "ses-1") {
  const now = new Date();
  return {
    ...session({
      serverTime: now.toISOString(),
      deadlineAt: new Date(now.getTime() + 3_600_000).toISOString(),
    }),
    questions,
    sessionId,
  };
}

function reply() {
  const now = new Date().toISOString();
  return {
    serverTime: now,
    savedAt: now,
    deadlineAt: new Date(store().deadlineAt).toISOString(),
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

function away() {
  act(() => {
    window.dispatchEvent(new Event("blur"));
    window.dispatchEvent(new Event("focus"));
  });
}

async function leaveByTheCross(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Thoát khỏi bài làm" }));
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", { name: "Thoát" }),
  );
}

function beacons(): Promise<Beacon[]> {
  return Promise.all(
    sent.map(async ({ url, body }) => ({
      url,
      ...(JSON.parse(await body.text()) as Omit<Beacon, "url">),
    })),
  );
}

const numbered = (events: { kind: string; clientSeq: number }[]) =>
  events.map((event) => [event.kind, event.clientSeq]);
const path = (router: { state: { location: { pathname: string } } }) =>
  router.state.location.pathname;

beforeEach(() => {
  viewport("desktop");
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  sent = [];
  Object.defineProperty(navigator, "sendBeacon", {
    configurable: true,
    value: vi.fn((url: string, body: Blob) => {
      sent.push({ url, body });
      return true;
    }),
  });
  vi.mocked(getAttempt).mockReset().mockResolvedValue(paper());
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () => reply());
  vi.mocked(submitAttempt)
    .mockReset()
    .mockImplementation(async () => ({
      ...paper().attempt,
      status: "submitted",
      submittedAt: new Date().toISOString(),
    }));
});

afterEach(() => {
  store().reset();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("what the integrity monitor has buffered when the engine is left", () => {
  it("sends a buffered absence when the student leaves with nothing unsaved", async () => {
    const user = userEvent.setup();
    const router = await open();
    away();
    expect(pending().map((event) => event.kind)).toEqual(ABSENCE);

    await leaveByTheCross(user);
    await waitFor(() => expect(path(router)).toBe("/app"));

    expect(saveAnswers).not.toHaveBeenCalled();
    const [beacon, ...others] = await beacons();
    expect(others).toEqual([]);
    expect(beacon?.url).toMatch(/\/app\/attempts\/att-1\/events$/);
    expect(beacon?.sessionId).toBe("ses-1");
    expect(beacon?.beaconToken).toBe("beacon");
    expect(numbered(beacon?.events ?? [])).toEqual([
      ["window_blur", 0],
      ["window_focus", 1],
    ]);
  });

  it("sends it on the browser's Back too", async () => {
    const router = await open();
    away();

    await act(() => router.navigate(-1));
    await waitFor(() => expect(path(router)).toBe("/app/classes"));

    expect(saveAnswers).not.toHaveBeenCalled();
    const [beacon, ...others] = await beacons();
    expect(others).toEqual([]);
    expect(beacon?.sessionId).toBe("ses-1");
    expect(numbered(beacon?.events ?? [])).toEqual([
      ["window_blur", 0],
      ["window_focus", 1],
    ]);
  });

  it("numbers on from the last sent event after a return by Back in the same session", async () => {
    const user = userEvent.setup();
    const router = await open();
    away();
    await leaveByTheCross(user);
    await waitFor(() => expect(path(router)).toBe("/app"));

    await act(() => router.navigate(-1));
    await screen.findByText(PROMPT);
    expect(getAttempt).toHaveBeenCalledTimes(2);
    away();

    expect(numbered(pending()).slice(-2)).toEqual([
      ["window_blur", 2],
      ["window_focus", 3],
    ]);
  });

  it("starts from zero under a new session", async () => {
    const user = userEvent.setup();
    const router = await open();
    away();
    await leaveByTheCross(user);
    await waitFor(() => expect(path(router)).toBe("/app"));

    vi.mocked(getAttempt).mockResolvedValue(paper("ses-2"));
    await act(() => router.navigate(-1));
    await screen.findByText(PROMPT);
    away();

    expect(numbered(pending())).toEqual([
      ["window_blur", 0],
      ["window_focus", 1],
    ]);
  });

  it("forgets the buffer when the attempt was submitted", async () => {
    const user = userEvent.setup();
    const router = await open();
    expect(sessionStorage.getItem(BUFFER)).not.toBeNull();

    await user.click(
      within(screen.getByRole("banner")).getByRole("button", { name: "Nộp bài" }),
    );
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Nộp bài",
      }),
    );
    await screen.findByText("Bài đã được nộp.");
    away();
    expect(pending().map((event) => event.kind)).toEqual(ABSENCE);

    await user.click(await screen.findByRole("button", { name: "Về trang chủ" }));
    await waitFor(() => expect(path(router)).toBe("/app"));

    expect(sessionStorage.getItem(BUFFER)).toBeNull();
    expect(sent).toEqual([]);
  });

  it("sends nothing for a paper that never loaded", async () => {
    const earlier = JSON.stringify({
      sessionId: "ses-1",
      nextSeq: 1,
      events: [
        { kind: "window_blur", occurredAt: "2026-09-01T08:00:00.000Z", clientSeq: 0 },
      ],
    });
    sessionStorage.setItem(BUFFER, earlier);
    vi.mocked(getAttempt).mockRejectedValue(new Error("offline"));
    const user = userEvent.setup();
    const router = mount();

    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Về trang chủ" }));
    await waitFor(() => expect(path(router)).toBe("/app"));

    expect(sent).toEqual([]);
    expect(sessionStorage.getItem(BUFFER)).toBe(earlier);
  });
});
