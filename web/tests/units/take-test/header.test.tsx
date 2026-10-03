import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  saveAnswers,
  submitAttempt,
  type AttemptSession,
  type StudentQuestion,
} from "@/features/take-test/api";
import { FLUSH_DEBOUNCE_MS, useTakeTestStore } from "@/features/take-test/store";
import { ApiError } from "@/lib/api/errors";
import { session, text, viewport } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
}));

const NOW = new Date("2026-09-22T05:00:00.000Z");
const MINUTE = 60_000;
const DECK_LEFT = 38 * MINUTE + 12_000;
const TITLE = "Reading · Why cities need green space";
const SAVED = "Đã lưu tất cả câu trả lời";
const SAVING = "Đang lưu…";
const FAILED = "Chưa lưu được. Đang thử lại…";
const OFFLINE = "Đang mất mạng. Sẽ lưu khi có mạng lại.";

const questions: StudentQuestion[] = [
  {
    id: "q1",
    sectionId: "s1",
    type: "single_choice",
    prompt: "What is the main purpose of the passage?",
    points: 1,
    options: [
      { id: "o1", text: "To compare parks" },
      { id: "o2", text: "To argue for green space" },
    ],
  },
  {
    id: "q2",
    sectionId: "s1",
    type: "short_answer",
    prompt: "What should cities prioritise?",
    points: 1,
  },
];

const at = (ms: number) => new Date(NOW.getTime() + ms).toISOString();
const store = () => useTakeTestStore.getState();

function reply() {
  const now = new Date().toISOString();
  return {
    serverTime: now,
    savedAt: now,
    deadlineAt: new Date(store().deadlineAt).toISOString(),
  };
}

async function open(left = DECK_LEFT, over: Partial<AttemptSession> = {}) {
  vi.mocked(getAttempt).mockResolvedValue({
    ...session({ serverTime: NOW.toISOString(), deadlineAt: at(left) }),
    testTitle: TITLE,
    questions,
    ...over,
  });
  const router = createMemoryRouter(
    [
      { path: "/app/attempts/:attemptId", element: <TakeTestPage /> },
      { path: "/app", element: <p>home</p> },
    ],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  render(<RouterProvider router={router} />);
  await act(() => vi.advanceTimersByTimeAsync(0));
  return router;
}

const pass = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const banner = () => within(screen.getByRole("banner"));
const line = (label: string) =>
  banner().getByText(label, { ignore: '[role="status"]' });
const timer = () => screen.getByRole("timer", { name: "Thời gian còn lại" });
const strip = () => document.querySelector<HTMLElement>('[data-slot="save-strip"]');
const announced = () =>
  screen
    .getAllByRole("status")
    .map((region) => region.textContent)
    .filter((said) => said !== "");
const type = (value = "public green space") =>
  act(() => store().setAnswer("q2", text(value)));

function heldSave() {
  const held: { resolve?: () => void; reject?: (cause: unknown) => void } = {};
  vi.mocked(saveAnswers).mockImplementationOnce(
    () =>
      new Promise((resolve, reject) => {
        held.resolve = () => resolve(reply());
        held.reject = reject;
      }),
  );
  return {
    succeed: () => act(async () => held.resolve?.()),
    fail: (cause: unknown = new Error("offline")) =>
      act(async () => held.reject?.(cause)),
  };
}

function connection(online: boolean) {
  vi.spyOn(window.navigator, "onLine", "get").mockReturnValue(online);
  act(() => {
    window.dispatchEvent(new Event(online ? "online" : "offline"));
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () => reply());
  vi.mocked(submitAttempt)
    .mockReset()
    .mockImplementation(
      async () =>
        session({
          serverTime: new Date().toISOString(),
          deadlineAt: at(DECK_LEFT),
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

describe("the header at 1280", () => {
  beforeEach(() => {
    viewport("desktop");
  });

  it("is the deck's row: leave, the title over the save line, the timer and Submit", async () => {
    await open();

    const header = screen.getByRole("banner");
    expect(header).toHaveClass("h-15", "px-6");
    const controls = within(header).getAllByRole("button");
    expect(controls.map((control) => control.textContent)).toEqual(["", "Nộp bài"]);
    expect(controls[0]).toHaveAccessibleName("Thoát khỏi bài làm");
    expect(controls[0]).toHaveClass("size-9", "min-h-0", "min-w-0");
    expect(controls[1]).toHaveClass("h-9.5", "min-h-0", "bg-primary");

    expect(banner().getByText(TITLE)).toHaveClass("truncate", "font-semibold");
    expect(banner().getByText(SAVED)).toBeInTheDocument();
    expect(timer()).toHaveTextContent("38:12");
    expect(timer()).toHaveClass("mx-auto", "h-9", "rounded-full", "tabular-nums");

    expect(strip()).toBeNull();
    expect(banner().queryByText(/^Câu \d/)).toBeNull();
    expect(screen.queryByRole("progressbar")).toBeNull();
  });

  it("opens the review from Submit, where the header keeps the way back and no second Submit", async () => {
    await open();
    fireEvent.click(banner().getByRole("button", { name: "Nộp bài" }));

    expect(
      screen.getByRole("heading", { name: "Xem lại trước khi nộp" }),
    ).toBeInTheDocument();
    expect(banner().getByRole("button", { name: "Quay lại bài" })).toBeInTheDocument();
    expect(banner().queryByRole("button", { name: "Nộp bài" })).toBeNull();
    expect(banner().queryByRole("button", { name: "Thoát khỏi bài làm" })).toBeNull();
    expect(screen.getAllByRole("button", { name: "Nộp bài" })).toHaveLength(1);
    expect(timer()).toBeInTheDocument();
    expect(banner().getByText(SAVED)).toBeInTheDocument();
  });

  it("says Saving from the keystroke until the server confirms, however long that takes", async () => {
    await open();
    const save = heldSave();

    type();
    expect(banner().getByText(SAVING)).toBeInTheDocument();
    expect(banner().queryByText(SAVED)).toBeNull();
    expect(strip()).toBeNull();
    expect(saveAnswers).not.toHaveBeenCalled();

    await pass(FLUSH_DEBOUNCE_MS);
    expect(saveAnswers).toHaveBeenCalledTimes(1);
    expect(store().flushInFlight).toBe(true);
    expect(banner().getByText(SAVING)).toBeInTheDocument();

    await pass(2 * MINUTE);
    expect(banner().getByText(SAVING)).toBeInTheDocument();
    expect(announced()).toEqual([]);

    await save.succeed();
    expect(banner().getByText(SAVED)).toBeInTheDocument();
    expect(banner().queryByText(SAVING)).toBeNull();
  });

  it("says the save failed, in the danger ink, until a retry lands", async () => {
    await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(new Error("boom"));
    const retry = heldSave();

    type();
    await pass(FLUSH_DEBOUNCE_MS);
    expect(line(FAILED).parentElement).toHaveClass("text-danger-ink");
    expect(line(FAILED).parentElement).toHaveAttribute("aria-hidden", "true");
    expect(announced()).toEqual([FAILED]);
    expect(banner().queryByText(SAVING)).toBeNull();

    await pass(2_000);
    expect(saveAnswers).toHaveBeenCalledTimes(2);
    expect(store().flushInFlight).toBe(true);
    expect(line(FAILED)).toBeInTheDocument();

    await retry.succeed();
    expect(banner().getByText(SAVED)).toBeInTheDocument();
    expect(banner().getByText(SAVED).parentElement).not.toHaveClass("text-danger-ink");
    expect(banner().getByText(SAVED).parentElement).not.toHaveAttribute("aria-hidden");
    expect(announced()).toEqual([]);
  });

  it("says the device is offline while an answer waits, and nothing once it is saved", async () => {
    await open();
    connection(false);
    expect(banner().getByText(SAVED)).toBeInTheDocument();

    vi.mocked(saveAnswers).mockRejectedValue(new Error("offline"));
    type();
    expect(line(OFFLINE).parentElement).toHaveClass("text-danger-ink");
    expect(announced()).toEqual([OFFLINE]);

    await pass(FLUSH_DEBOUNCE_MS);
    expect(line(OFFLINE)).toBeInTheDocument();

    connection(true);
    expect(line(FAILED)).toBeInTheDocument();

    vi.mocked(saveAnswers).mockImplementation(async () => reply());
    await pass(30_000);
    expect(banner().getByText(SAVED)).toBeInTheDocument();
  });

  it("puts the strike count after the save line when the assignment counts departures", async () => {
    await open(DECK_LEFT, {
      integrity: {
        requireFullscreen: false,
        blockCopyPaste: true,
        maxFocusLoss: 2,
        onLimitExceeded: "flag",
        minAwayMs: 3000,
      },
    });
    expect(banner().getByText("Còn 2 lần rời trang")).toBeInTheDocument();
    expect(banner().getByText(SAVED)).toBeInTheDocument();
    expect(strip()).toBeNull();
  });

  it("draws no strike count when departures are not counted", async () => {
    await open();
    expect(screen.queryByText(/lần rời trang/)).toBeNull();
    expect(banner().queryByText("·")).toBeNull();
  });

  it("drops the save line and Submit on a paper another device took over", async () => {
    await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(
      new ApiError({ status: 409, code: "SESSION_SUPERSEDED", message: "elsewhere" }),
    );
    type();
    await pass(FLUSH_DEBOUNCE_MS);

    expect(store().lock).toBe("superseded");
    expect(
      screen.getByText("Bài này đang mở ở thiết bị khác. Bạn không thể sửa ở đây nữa."),
    ).toBeInTheDocument();
    expect(banner().getByText(TITLE)).toBeInTheDocument();
    expect(banner().queryByText(FAILED)).toBeNull();
    expect(banner().queryByText(SAVING)).toBeNull();
    expect(announced()).toEqual([]);
    expect(banner().queryByRole("button", { name: "Nộp bài" })).toBeNull();
    expect(
      banner().getByRole("button", { name: "Thoát khỏi bài làm" }),
    ).toBeInTheDocument();
    expect(timer()).toBeInTheDocument();
  });

  it("drops Submit and the save line on a paper that has already ended", async () => {
    await open(DECK_LEFT, {
      attempt: {
        ...session({
          serverTime: NOW.toISOString(),
          deadlineAt: at(DECK_LEFT),
          status: "submitted",
        }).attempt,
      },
    });
    expect(screen.getByText("Bài làm này đã kết thúc.")).toBeInTheDocument();
    expect(banner().queryByRole("button", { name: "Nộp bài" })).toBeNull();
    expect(banner().queryByText(SAVED)).toBeNull();
    expect(
      banner().getByRole("button", { name: "Thoát khỏi bài làm" }),
    ).toBeInTheDocument();
  });

  it("keeps Submit when the lock is the deadline, which still has a paper to hand in", async () => {
    await open();
    act(() => store().lockNow("deadline"));
    expect(screen.getByText("Đã hết giờ làm bài.")).toBeInTheDocument();
    expect(banner().getByRole("button", { name: "Nộp bài" })).toBeInTheDocument();
    expect(banner().queryByText(SAVED)).toBeNull();
  });

  it("leaves only the title once the paper is submitted", async () => {
    await open();
    await act(() => store().submit("manual"));

    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
    expect(banner().getByText(TITLE)).toBeInTheDocument();
    expect(banner().queryByRole("button")).toBeNull();
    expect(screen.queryByRole("timer")).toBeNull();
    expect(banner().queryByText(SAVED)).toBeNull();
    expect(screen.queryAllByRole("status")).toEqual([]);
  });
});

describe("the timer", () => {
  beforeEach(() => {
    viewport("desktop");
  });

  it("reads the server's clock on a device running five minutes fast", async () => {
    vi.setSystemTime(new Date(NOW.getTime() + 5 * MINUTE));
    await open(30 * MINUTE);
    expect(timer()).toHaveTextContent("30:00");
    await pass(1_000);
    expect(timer()).toHaveTextContent("29:59");
  });

  it("takes the danger tones under five minutes, and not at five", async () => {
    await open(5 * MINUTE + 1_000);
    expect(timer()).toHaveTextContent("05:01");
    expect(timer()).toHaveClass("bg-muted", "text-fg");

    await pass(1_000);
    expect(timer()).toHaveTextContent("05:00");
    expect(timer()).toHaveClass("bg-muted", "text-fg");
    expect(timer()).not.toHaveClass("bg-danger-soft");

    await pass(1_000);
    expect(timer()).toHaveTextContent("04:59");
    expect(timer()).toHaveClass("bg-danger-soft", "text-danger-ink");
    expect(timer()).not.toHaveClass("bg-muted", "text-fg");
  });

  it("announces five minutes and one minute, politely, once each", async () => {
    await open(5 * MINUTE + 2_000);
    const heard: string[] = [];
    const listen = () => {
      const now = [...document.querySelectorAll('[role="status"]')]
        .map((region) => region.textContent)
        .filter((said) => said !== "")
        .join("|");
      if (heard.at(-1) !== now) heard.push(now);
    };

    listen();
    for (let second = 0; second < 5 * MINUTE; second += 1_000) {
      await pass(1_000);
      listen();
    }

    expect(heard).toEqual(["", "Còn 5 phút", "", "Còn 1 phút"]);
    expect(timer()).toHaveTextContent("00:02");
    expect(screen.getByText("Còn 1 phút")).toHaveAttribute("role", "status");
    expect(timer()).not.toHaveAttribute("aria-live");
  });

  it("says five minutes at 5:00 and one minute at 1:00, not a second early", async () => {
    await open(5 * MINUTE + 2_000);
    await pass(1_000);
    expect(timer()).toHaveTextContent("05:01");
    expect(announced()).toEqual([]);
    await pass(1_000);
    expect(timer()).toHaveTextContent("05:00");
    expect(announced()).toEqual(["Còn 5 phút"]);

    await pass(59_000);
    expect(timer()).toHaveTextContent("04:01");
    expect(announced()).toEqual(["Còn 5 phút"]);
    await pass(1_000);
    expect(timer()).toHaveTextContent("04:00");
    expect(announced()).toEqual([]);

    await pass(3 * MINUTE - 1_000);
    expect(timer()).toHaveTextContent("01:01");
    expect(announced()).toEqual([]);
    await pass(1_000);
    expect(timer()).toHaveTextContent("01:00");
    expect(announced()).toEqual(["Còn 1 phút"]);

    await pass(59_000);
    expect(timer()).toHaveTextContent("00:01");
    expect(announced()).toEqual(["Còn 1 phút"]);
  });

  it("says one minute again when a moved deadline lets the paper pass it twice", async () => {
    await open(2 * MINUTE);
    await pass(80_000);
    expect(timer()).toHaveTextContent("00:40");
    expect(announced()).toEqual(["Còn 1 phút"]);

    vi.mocked(saveAnswers).mockImplementationOnce(async () => ({
      ...reply(),
      deadlineAt: at(4 * MINUTE),
    }));
    type();
    await pass(2_000);
    expect(timer()).toHaveTextContent("02:38");
    expect(announced()).toEqual([]);

    await pass(97_000);
    expect(timer()).toHaveTextContent("01:01");
    expect(announced()).toEqual([]);
    await pass(1_000);
    expect(timer()).toHaveTextContent("01:00");
    expect(announced()).toEqual(["Còn 1 phút"]);
  });

  it("does not bring an old line back when a deadline moves by less than a minute", async () => {
    await open(5 * MINUTE + 1_000);
    await pass(63_000);
    expect(timer()).toHaveTextContent("03:58");
    expect(announced()).toEqual([]);

    vi.mocked(saveAnswers).mockImplementationOnce(async () => ({
      ...reply(),
      deadlineAt: at(5 * MINUTE + 31_000),
    }));
    type();
    await pass(2_000);
    expect(timer()).toHaveTextContent("04:26");
    expect(announced()).toEqual([]);
  });

  it("does not announce a time the paper was never above", async () => {
    await open(3 * MINUTE);
    expect(timer()).toHaveClass("bg-danger-soft");
    await pass(30_000);
    expect(announced()).toEqual([]);
    await pass(90_000);
    expect(announced()).toEqual(["Còn 1 phút"]);
  });

  it("follows a deadline the server moves, out of the danger tones and back", async () => {
    await open(5 * MINUTE + 1_000);
    await pass(2_000);
    expect(timer()).toHaveTextContent("04:59");
    expect(timer()).toHaveClass("bg-danger-soft");
    expect(announced()).toEqual(["Còn 5 phút"]);

    vi.mocked(saveAnswers).mockImplementationOnce(async () => ({
      ...reply(),
      deadlineAt: at(15 * MINUTE),
    }));
    type();
    await pass(FLUSH_DEBOUNCE_MS);
    expect(store().deadlineAt).toBe(NOW.getTime() + 15 * MINUTE);
    expect(timer()).toHaveTextContent("14:56");
    expect(timer()).toHaveClass("bg-muted", "text-fg");
    await pass(1_000);
    expect(announced()).toEqual([]);

    await pass(10 * MINUTE);
    expect(timer()).toHaveTextContent("04:55");
    expect(timer()).toHaveClass("bg-danger-soft");
    expect(announced()).toEqual(["Còn 5 phút"]);
  });

  it("says the minutes really left when the time drops past five in one step", async () => {
    await open(20 * MINUTE);
    vi.mocked(saveAnswers).mockImplementationOnce(async () => ({
      ...reply(),
      deadlineAt: at(3 * MINUTE + 30_000),
    }));
    type();
    await pass(FLUSH_DEBOUNCE_MS);
    await pass(1_000);
    expect(timer()).toHaveTextContent("03:27");
    expect(announced()).toEqual(["Còn 4 phút"]);
  });
});

describe("the header on a phone", () => {
  beforeEach(() => {
    viewport("phone");
  });

  it("is leave, the timer and Submit, with no title and no save line", async () => {
    await open();

    const header = screen.getByRole("banner");
    expect(header).toHaveClass("h-15", "px-3.5");
    const controls = within(header).getAllByRole("button");
    expect(controls.map((control) => control.textContent)).toEqual(["", "Nộp bài"]);
    expect(controls[0]).toHaveAccessibleName("Thoát khỏi bài làm");
    expect(timer()).toHaveTextContent("38:12");
    expect(banner().queryByText(TITLE)).toBeNull();
    expect(screen.queryByText(SAVED)).toBeNull();
    expect(strip()).toBeEmptyDOMElement();
  });

  it("shows the save line in the strip only while an answer is unsaved", async () => {
    await open();
    const save = heldSave();

    type();
    expect(strip()).toHaveTextContent(SAVING);
    expect(banner().queryByText(SAVING)).toBeNull();

    await pass(FLUSH_DEBOUNCE_MS);
    expect(strip()).toHaveTextContent(SAVING);

    await save.succeed();
    expect(strip()).toBeEmptyDOMElement();
    expect(screen.queryByText(SAVED)).toBeNull();
  });

  it("keeps the strip's row in place, saved or not, so the paper under it never moves", async () => {
    await open();
    const row = strip();
    const paper = screen.getByRole("main").parentElement;
    expect(row).toHaveClass("min-h-[35px]", "flex-none", "border-b", "py-2");
    expect(row?.previousElementSibling).toBe(screen.getByRole("banner"));
    expect(row?.nextElementSibling).toBe(paper);

    const save = heldSave();
    type();
    expect(strip()).toBe(row);
    expect(row?.nextElementSibling).toBe(paper);

    await pass(FLUSH_DEBOUNCE_MS);
    await save.succeed();
    expect(strip()).toBe(row);
    expect(row).toBeEmptyDOMElement();
    expect(row?.nextElementSibling).toBe(paper);
  });

  it("reserves the same row on the review", async () => {
    await open();
    fireEvent.click(banner().getByRole("button", { name: "Nộp bài" }));
    expect(
      screen.getByRole("heading", { name: "Xem lại trước khi nộp" }),
    ).toBeInTheDocument();
    expect(strip()).toBeEmptyDOMElement();
    expect(strip()?.previousElementSibling).toBe(screen.getByRole("banner"));
  });

  it("says in the strip that a save failed or the device is offline, and announces it", async () => {
    await open();
    vi.mocked(saveAnswers).mockRejectedValue(new Error("boom"));

    type();
    await pass(FLUSH_DEBOUNCE_MS);
    expect(strip()).toHaveTextContent(FAILED);
    expect(within(strip()!).getByText(FAILED).parentElement).toHaveClass(
      "text-danger-ink",
    );
    expect(announced()).toEqual([FAILED]);

    connection(false);
    expect(strip()).toHaveTextContent(OFFLINE);
    expect(announced()).toEqual([OFFLINE]);

    connection(true);
    vi.mocked(saveAnswers).mockImplementation(async () => reply());
    await pass(30_000);
    expect(strip()).toBeEmptyDOMElement();
    expect(announced()).toEqual([]);
  });

  it("keeps the lock message strip as it was", async () => {
    await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(
      new ApiError({ status: 409, code: "SESSION_SUPERSEDED", message: "elsewhere" }),
    );
    type();
    await pass(FLUSH_DEBOUNCE_MS);

    const message = screen.getByText(
      "Bài này đang mở ở thiết bị khác. Bạn không thể sửa ở đây nữa.",
    );
    expect(message.parentElement).toHaveClass("bg-warning/10", "border-b");
    expect(strip()).toBeNull();
    expect(screen.queryByText(FAILED)).toBeNull();
  });

  it("keeps the strike count in the strip, with the save line beside it only while unsaved", async () => {
    await open(DECK_LEFT, {
      integrity: {
        requireFullscreen: false,
        blockCopyPaste: true,
        maxFocusLoss: 2,
        onLimitExceeded: "flag",
        minAwayMs: 3000,
      },
    });
    expect(strip()).toHaveTextContent("Còn 2 lần rời trang");
    expect(strip()).not.toHaveTextContent(SAVED);
    expect(banner().queryByText("Còn 2 lần rời trang")).toBeNull();

    const save = heldSave();
    type();
    expect(strip()).toHaveTextContent(SAVING);
    expect(strip()).toHaveTextContent("Còn 2 lần rời trang");

    await pass(FLUSH_DEBOUNCE_MS);
    await save.succeed();
    expect(strip()).toHaveTextContent("Còn 2 lần rời trang");
    expect(strip()).not.toHaveTextContent(SAVING);
  });

  it("has no header once the paper is submitted", async () => {
    await open();
    await act(() => store().submit("manual"));
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("banner")).toBeNull();
  });
});
