import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  within,
  type RenderResult,
} from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  saveAnswers,
  submitAttempt,
  type AttemptSession,
  type StudentQuestion,
} from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { session, viewport } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
}));

const NOW = new Date("2026-09-22T05:00:00.000Z");
const MINUTE = 60_000;
const KEY = "quizzivy.open-question.att-1";
const OTHER_KEY = "quizzivy.open-question.att-2";
const FIRST = "Câu 1 trên 3 · Chọn một đáp án";
const SECOND = "Câu 2 trên 3 · Trả lời ngắn";
const THIRD = "Câu 3 trên 3 · Đúng hay sai";

const questions: StudentQuestion[] = [
  {
    id: "q1",
    sectionId: "s1",
    type: "single_choice",
    prompt: "Pick one",
    points: 1,
    options: [
      { id: "o1", text: "Alpha" },
      { id: "o2", text: "Beta" },
    ],
  },
  {
    id: "q2",
    sectionId: "s1",
    type: "short_answer",
    prompt: "Describe the map",
    points: 1,
    media: {
      id: "018f0000-0000-7000-8000-00000000f002",
      kind: "image",
      url: "https://assets.example/map.png",
      mimeType: "image/png",
      bytes: 4096,
      originalFilename: "map.png",
      createdAt: "2026-09-20T00:00:00Z",
    },
  },
  { id: "q3", sectionId: "s1", type: "true_false", prompt: "True?", points: 1 },
];

const at = (ms: number) => new Date(NOW.getTime() + ms).toISOString();
const store = () => useTakeTestStore.getState();
const pass = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const line = () => screen.getByText(/^Câu \d trên 3 · /).textContent;
const next = () => fireEvent.click(screen.getByRole("button", { name: "Câu sau" }));

function paper(order: StudentQuestion[] = questions): AttemptSession {
  return {
    ...session({ serverTime: NOW.toISOString(), deadlineAt: at(40 * MINUTE) }),
    questions: order,
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

async function open() {
  const router = createMemoryRouter(
    [
      { path: "/app/attempts/:attemptId", element: <TakeTestPage /> },
      { path: "/app", element: <p>home</p> },
    ],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  const view = render(<RouterProvider router={router} />);
  await pass(0);
  return view;
}

function reload(view: RenderResult) {
  view.unmount();
  store().reset();
  return open();
}

async function readAgain() {
  fireEvent.error(screen.getByAltText("Hình ảnh của câu hỏi"));
  fireEvent.click(screen.getByRole("button", { name: "Thử lại" }));
  await pass(0);
}

function refuseStorage() {
  const getItem = Storage.prototype.getItem;
  const setItem = Storage.prototype.setItem;
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(function (
    this: Storage,
    key: string,
  ) {
    if (key === KEY) throw new DOMException("denied", "SecurityError");
    return getItem.call(this, key);
  });
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(function (
    this: Storage,
    key: string,
    value: string,
  ) {
    if (key === KEY) throw new DOMException("denied", "SecurityError");
    setItem.call(this, key, value);
  });
}

beforeEach(() => {
  viewport("desktop");
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  vi.mocked(getAttempt).mockReset().mockResolvedValue(paper());
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () => reply());
  vi.mocked(submitAttempt)
    .mockReset()
    .mockImplementation(
      async () =>
        session({
          serverTime: new Date().toISOString(),
          deadlineAt: at(40 * MINUTE),
          status: "submitted",
        }).attempt,
    );
});

afterEach(() => {
  store().reset();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the open question across a reload", () => {
  it("opens the question that was open before a reload", async () => {
    const view = await open();
    next();
    next();
    expect(line()).toBe(THIRD);

    await reload(view);

    expect(line()).toBe(THIRD);
    expect(sessionStorage.getItem(KEY)).toBe("q3");
  });

  it("opens question 1 on a first visit", async () => {
    await open();
    expect(line()).toBe(FIRST);
  });

  it("opens question 1 when the stored id is not on the paper", async () => {
    sessionStorage.setItem(KEY, "gone");
    await open();
    expect(line()).toBe(FIRST);
  });

  it("opens question 1 when storage throws, and the page still works", async () => {
    refuseStorage();
    await open();
    expect(line()).toBe(FIRST);

    next();
    expect(line()).toBe(SECOND);
  });

  it("does not read another attempt's question", async () => {
    sessionStorage.setItem(OTHER_KEY, "q3");
    await open();
    expect(line()).toBe(FIRST);
  });

  it("forgets the question once the attempt is submitted", async () => {
    await open();
    next();
    expect(sessionStorage.getItem(KEY)).toBe("q2");

    fireEvent.click(
      within(screen.getByRole("banner")).getByRole("button", { name: "Nộp bài" }),
    );
    await pass(0);
    fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Nộp bài" }),
    );
    await pass(0);

    expect(
      screen.getByRole("heading", { level: 1, name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
    expect(sessionStorage.getItem(KEY)).toBeNull();

    await pass(5 * MINUTE);
    expect(sessionStorage.getItem(KEY)).toBeNull();
  });

  it("stays on the same question when the paper is read again", async () => {
    await open();
    next();
    expect(line()).toBe(SECOND);

    await readAgain();

    expect(getAttempt).toHaveBeenCalledTimes(2);
    expect(line()).toBe(SECOND);
  });

  it("keeps the question on a paper that is locked without a submit", async () => {
    await open();
    next();
    act(() => store().lockNow("superseded"));
    await pass(0);

    expect(sessionStorage.getItem(KEY)).toBe("q2");
  });

  it("stays on the same question after a refetch when storage throws", async () => {
    refuseStorage();
    await open();
    next();
    expect(line()).toBe(SECOND);

    await readAgain();

    expect(getAttempt).toHaveBeenCalledTimes(2);
    expect(line()).toBe(SECOND);
  });

  it("restores after a first load that failed", async () => {
    sessionStorage.setItem(KEY, "q3");
    vi.mocked(getAttempt).mockRejectedValueOnce(new Error("offline"));
    await open();
    expect(screen.getByRole("alert")).toHaveTextContent("Không mở được bài làm.");

    fireEvent.click(screen.getByRole("button", { name: "Thử lại" }));
    await pass(0);

    expect(getAttempt).toHaveBeenCalledTimes(2);
    expect(line()).toBe(THIRD);
  });

  it("follows the question, not its position, when the paper comes back in another order", async () => {
    const view = await open();
    next();
    expect(line()).toBe(SECOND);

    vi.mocked(getAttempt).mockResolvedValue(
      paper([questions[0]!, questions[2]!, questions[1]!]),
    );
    await reload(view);

    expect(line()).toBe("Câu 3 trên 3 · Trả lời ngắn");
    expect(screen.getByText("Describe the map")).toBeInTheDocument();
  });

  it("stays on the same question after a refetch when storage holds another question's id", async () => {
    await open();
    next();
    sessionStorage.setItem(KEY, "q3");

    await readAgain();

    expect(getAttempt).toHaveBeenCalledTimes(2);
    expect(line()).toBe(SECOND);
  });
});
