import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import FocusLayout from "@/layouts/FocusLayout";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import { pending as bufferedEvents } from "@/features/integrity/buffer";
import {
  getAttempt,
  saveAnswers,
  submitAttempt,
  type AttemptSession,
} from "@/features/take-test/api";
import { FLUSH_DEBOUNCE_MS, useTakeTestStore } from "@/features/take-test/store";
import { ApiError } from "@/lib/api/errors";
import i18n from "@/lib/i18n";
import { DECK_ATTEMPT, DECK_LEFT_MS, deckSaved, deckSession } from "./deckSession";
import { text, viewport } from "./support";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
}));

const NOW = new Date("2026-09-22T05:00:00.000Z");
const MINUTE = 60_000;
const UNANSWERED = "Nộp bài khi còn 5 câu chưa trả lời?";
const ALL = "Nộp bài?";
const KEEP = "Quay lại làm tiếp";
const SUBMIT = "Nộp bài";
const FAILED = "Không nộp được. Hãy thử lại.";

const q = (n: number) => `018f0000-0000-7000-8000-00000000b00${n}`;
const store = () => useTakeTestStore.getState();
const pass = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const deadline = () => new Date(store().deadlineAt).toISOString();

function submitted() {
  return {
    ...deckSession(NOW).attempt,
    status: "submitted" as const,
    submittedAt: new Date().toISOString(),
  };
}

async function open(left = DECK_LEFT_MS, over: Partial<AttemptSession> = {}) {
  vi.mocked(getAttempt).mockResolvedValue({ ...deckSession(NOW, left), ...over });
  const router = createMemoryRouter(
    [
      {
        path: "/app/attempts/:attemptId",
        element: <FocusLayout />,
        children: [{ index: true, element: <TakeTestPage /> }],
      },
      { path: "/app", element: <p>home</p> },
      { path: "/app/attempts/:attemptId/result", element: <p>result page</p> },
    ],
    { initialEntries: [`/app/attempts/${DECK_ATTEMPT}`] },
  );
  render(<RouterProvider router={router} />);
  await pass(0);
  return router;
}

const header = () => within(screen.getByRole("banner", { hidden: true }));
const headerSubmit = () => header().getByRole("button", { name: SUBMIT, hidden: true });
const dialog = () => screen.getByRole("dialog");
const noDialog = () => expect(screen.queryByRole("dialog")).toBeNull();
const inDialog = () => within(dialog());
const chips = () =>
  within(inDialog().getByRole("group", { name: "Đến câu" })).getAllByRole("button");
const facts = () =>
  Object.fromEntries(
    inDialog()
      .getAllByRole("term")
      .map((term) => [term.textContent, term.nextElementSibling]),
  );
const onQuestion = (n: number) =>
  screen.queryByRole("main", { name: `Câu ${n}`, hidden: true });

async function ask() {
  const button = headerSubmit();
  button.focus();
  fireEvent.click(button);
  await pass(0);
  return dialog();
}

function heldSubmit() {
  const held: { resolve?: () => void } = {};
  vi.mocked(submitAttempt).mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        held.resolve = () => resolve(submitted());
      }),
  );
  return { land: () => act(async () => held.resolve?.()) };
}

beforeEach(() => {
  viewport("desktop");
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () => deckSaved(new Date(), deadline()));
  vi.mocked(submitAttempt)
    .mockReset()
    .mockImplementation(async () => submitted());
});

afterEach(async () => {
  store().reset();
  await act(() => i18n.changeLanguage("vi"));
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the Submit dialog", () => {
  it("opens from the header's Submit, over the paper, as the deck draws it", async () => {
    await open();
    noDialog();
    const frame = await ask();

    expect(frame).toHaveAccessibleName(UNANSWERED);
    expect(frame).toHaveAccessibleDescription(
      "Câu chưa trả lời sẽ không có điểm. Bạn vẫn còn 38 phút.",
    );
    expect(frame).toHaveClass("w-[min(27.5rem,calc(100%-1.5rem))]", "bg-card");
    expect(frame).toHaveClass("gap-3.5", "p-5.5", "rounded-2xl", "top-[50%]");
    expect(frame).toHaveAttribute("data-scale", "deck");

    expect(
      inDialog()
        .getAllByRole("term")
        .map((term) => term.textContent),
    ).toEqual(["Đã trả lời", "Đánh dấu", "Còn lại"]);
    expect(facts()["Đã trả lời"]).toHaveTextContent(/^3 \/ 8$/);
    expect(facts()["Đánh dấu"]).toHaveTextContent(/^0$/);
    expect(facts()["Còn lại"]).toHaveTextContent(/^38:12$/);
    for (const value of Object.values(facts())) {
      expect(value).toHaveClass("text-stat-sm", "font-semibold");
      expect(value).not.toHaveClass("text-danger-ink");
      expect(value).not.toHaveClass("text-warning-ink");
    }
    const tiles = inDialog().getAllByRole("term")[0]?.parentElement?.parentElement;
    expect(tiles).toHaveClass("grid-cols-3", "gap-px", "bg-border", "rounded-lg");

    expect(chips().map((chip) => chip.textContent)).toEqual(["4", "5", "6", "7", "8"]);
    expect(chips()[0]).toHaveAccessibleName("Câu 4");
    expect(chips()[0]).toHaveClass("h-7.5", "min-w-8", "rounded-seg", "bg-card");

    expect(
      inDialog()
        .getAllByRole("button")
        .slice(-2)
        .map((button) => button.textContent),
    ).toEqual([KEEP, SUBMIT]);
    expect(inDialog().getByRole("button", { name: SUBMIT })).toHaveClass(
      "in-data-[scale=deck]:h-10.5",
      "bg-primary",
    );
    expect(inDialog().getByRole("button", { name: KEEP })).toHaveClass("shadow-none");

    expect(onQuestion(1)).toBeInTheDocument();
    expect(screen.queryByText("Xem lại trước khi nộp")).toBeNull();
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("does not start with the focus on Submit test", async () => {
    await open();
    await ask();
    expect(dialog()).toContainElement(document.activeElement as HTMLElement);
    expect(inDialog().getByRole("button", { name: SUBMIT })).not.toHaveFocus();
  });

  it("ticks Time left with the header's timer and turns it red under five minutes, not at five", async () => {
    await open(5 * MINUTE + 1_000);
    await ask();
    expect(facts()["Còn lại"]).toHaveTextContent(/^05:01$/);
    expect(dialog()).toHaveAccessibleDescription(/Bạn vẫn còn 5 phút\.$/);

    await pass(1_000);
    expect(facts()["Còn lại"]).toHaveTextContent(/^05:00$/);
    expect(facts()["Còn lại"]).not.toHaveClass("text-danger-ink");
    expect(screen.getByRole("timer", { hidden: true })).not.toHaveClass(
      "text-danger-ink",
    );

    await pass(1_000);
    expect(facts()["Còn lại"]).toHaveTextContent(/^04:59$/);
    expect(facts()["Còn lại"]).toHaveClass("text-danger-ink");
    expect(screen.getByRole("timer", { hidden: true })).toHaveClass("text-danger-ink");
    expect(screen.getByRole("timer", { hidden: true })).toHaveTextContent("04:59");
    expect(dialog()).toHaveAccessibleDescription(/Bạn vẫn còn 4 phút\.$/);
  });

  it("says less than a minute is left instead of zero minutes", async () => {
    await open(MINUTE + 1_000);
    await ask();
    expect(dialog()).toHaveAccessibleDescription(
      "Câu chưa trả lời sẽ không có điểm. Bạn vẫn còn 1 phút.",
    );
    await pass(2_000);
    expect(dialog()).toHaveAccessibleDescription(
      "Câu chưa trả lời sẽ không có điểm. Bạn còn chưa đầy một phút.",
    );
  });

  it("puts Flagged in the warning ink once a question is flagged", async () => {
    await open();
    act(() => {
      store().toggleFlag(q(2));
      store().toggleFlag(q(7));
    });
    await ask();
    expect(facts()["Đánh dấu"]).toHaveTextContent(/^2$/);
    expect(facts()["Đánh dấu"]).toHaveClass("text-warning-ink");
  });

  it("goes to an unanswered question from its chip, and closes", async () => {
    await open();
    await ask();
    fireEvent.click(inDialog().getByRole("button", { name: "Câu 6" }));
    await pass(0);

    noDialog();
    expect(onQuestion(6)).toBeInTheDocument();
    expect(onQuestion(1)).toBeNull();
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("follows the answers: a question answered since has no chip and the count moves", async () => {
    await open();
    act(() => store().setAnswer(q(5), text("parks and trees")));
    await ask();

    expect(dialog()).toHaveAccessibleName("Nộp bài khi còn 4 câu chưa trả lời?");
    expect(facts()["Đã trả lời"]).toHaveTextContent(/^4 \/ 8$/);
    expect(chips().map((chip) => chip.textContent)).toEqual(["4", "6", "7", "8"]);
  });

  it("asks one plain question when every question is answered", async () => {
    await open();
    act(() => {
      store().setAnswer(q(4), {
        type: "choice",
        optionIds: ["018f0000-0000-7000-8000-00000000c401"],
      });
      store().setAnswer(q(5), text("green space"));
      store().setAnswer(q(6), {
        type: "choice",
        optionIds: ["018f0000-0000-7000-8000-00000000c602"],
      });
      store().setAnswer(q(7), text("play streets"));
      store().setAnswer(q(8), {
        type: "choice",
        optionIds: ["018f0000-0000-7000-8000-00000000c801"],
      });
    });
    await ask();

    expect(dialog()).toHaveAccessibleName(ALL);
    expect(dialog()).toHaveAccessibleDescription(
      "Sau khi nộp, bạn không thể sửa câu trả lời.",
    );
    expect(facts()["Đã trả lời"]).toHaveTextContent(/^8 \/ 8$/);
    expect(inDialog().queryByRole("group")).toBeNull();
    expect(dialog()).not.toHaveTextContent("chưa trả lời");
    expect(inDialog().getAllByRole("button")).toHaveLength(2);
  });

  it("lets the student back out with Keep working, Esc or the backdrop, and hands in nothing", async () => {
    await open();

    await ask();
    fireEvent.click(inDialog().getByRole("button", { name: KEEP }));
    await pass(0);
    noDialog();
    expect(headerSubmit()).toHaveFocus();

    await ask();
    fireEvent.keyDown(dialog(), { key: "Escape" });
    await pass(0);
    noDialog();

    await ask();
    const scrim = document.querySelector('[data-slot="dialog-overlay"]');
    if (scrim === null) throw new Error("no scrim");
    fireEvent.pointerDown(scrim);
    fireEvent.click(scrim);
    await pass(0);
    noDialog();

    expect(onQuestion(1)).toBeInTheDocument();
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("saves what is unsaved, then submits, and shows the submitted screen", async () => {
    const router = await open();
    act(() => store().setAnswer(q(5), text("the last thing I typed")));
    await ask();

    fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    await pass(0);

    expect(saveAnswers).toHaveBeenCalledTimes(1);
    expect(vi.mocked(saveAnswers).mock.calls[0]?.[1].answers).toEqual({
      [q(5)]: text("the last thing I typed"),
    });
    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith(DECK_ATTEMPT, {
      reason: "manual",
    });
    expect(vi.mocked(saveAnswers).mock.invocationCallOrder[0]).toBeLessThan(
      vi.mocked(submitAttempt).mock.invocationCallOrder[0] ?? 0,
    );

    noDialog();
    expect(
      screen.getByRole("heading", { level: 1, name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Thoát khỏi bài làm" })).toBeNull();
    expect(router.state.location.pathname).toBe(`/app/attempts/${DECK_ATTEMPT}`);
  });

  it("holds the dialog while the submission is out, and sends it once", async () => {
    await open();
    await ask();
    const request = heldSubmit();

    fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    await pass(0);

    const sending = inDialog().getByRole("button", { name: "Đang nộp…" });
    expect(sending).toHaveAttribute("aria-busy", "true");
    expect(sending).toHaveAttribute("aria-disabled", "true");
    expect(sending.querySelector("svg")).toHaveClass("animate-spin");
    expect(inDialog().getByRole("button", { name: KEEP })).toBeDisabled();
    for (const chip of chips()) expect(chip).toBeDisabled();

    fireEvent.click(sending);
    fireEvent.keyDown(dialog(), { key: "Escape" });
    await pass(0);
    expect(dialog()).toBeInTheDocument();
    expect(submitAttempt).toHaveBeenCalledTimes(1);

    await request.land();
    noDialog();
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
    expect(submitAttempt).toHaveBeenCalledTimes(1);
  });

  it("says it could not submit, inside the dialog, and lets the student try again", async () => {
    await open();
    await ask();
    vi.mocked(submitAttempt).mockRejectedValueOnce(new Error("offline"));

    fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    await pass(0);

    expect(inDialog().getByRole("alert")).toHaveTextContent(FAILED);
    expect(inDialog().getByRole("alert")).toHaveClass("text-danger-ink");
    expect(store().submitState).toBe("idle");
    expect(inDialog().getByRole("button", { name: KEEP })).toBeEnabled();

    fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    await pass(0);
    expect(submitAttempt).toHaveBeenCalledTimes(2);
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
  });

  it("does not hand in a paper whose last answer could not be saved", async () => {
    await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(new Error("offline"));
    act(() => store().setAnswer(q(5), text("must not be lost")));
    await ask();

    fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    await pass(0);

    expect(submitAttempt).not.toHaveBeenCalled();
    expect(inDialog().getByRole("alert")).toHaveTextContent(FAILED);
    expect(store().dirty.has(q(5))).toBe(true);
  });

  it("forgets a failure when it is opened again", async () => {
    await open();
    await ask();
    vi.mocked(submitAttempt).mockRejectedValueOnce(new Error("offline"));
    fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    await pass(0);
    expect(inDialog().getByRole("alert")).toBeInTheDocument();

    fireEvent.click(inDialog().getByRole("button", { name: KEEP }));
    await pass(0);
    await ask();
    expect(inDialog().queryByRole("alert")).toBeNull();
  });

  it("sends nothing of its own when the timer's submission is already out, and reports no failure", async () => {
    await open();
    await ask();
    const request = heldSubmit();

    act(() => {
      void store().submit("timer_expired");
      fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    });
    await pass(0);

    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith(DECK_ATTEMPT, {
      reason: "timer_expired",
    });
    expect(inDialog().queryByRole("alert")).toBeNull();
    expect(inDialog().getByRole("button", { name: "Đang nộp…" })).toBeInTheDocument();

    await request.land();
    expect(submitAttempt).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("heading", { name: "Bài đã hết giờ và được nộp tự động." }),
    ).toBeInTheDocument();
  });

  it("gives way to an auto-submit that starts in the same moment as its own click", async () => {
    await open();
    await ask();
    const request = heldSubmit();

    act(() => {
      void store().submit("auto_submit");
      fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    });
    await pass(0);

    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith(DECK_ATTEMPT, {
      reason: "auto_submit",
    });
    noDialog();
    expect(
      screen.getByRole("heading", { name: "Bài làm đang được nộp" }),
    ).toBeInTheDocument();

    await request.land();
    expect(submitAttempt).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp vì rời trang lần nữa." }),
    ).toBeInTheDocument();
  });

  it("closes for good when another device takes the paper while it is open", async () => {
    await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(
      new ApiError({ status: 409, code: "SESSION_SUPERSEDED", message: "elsewhere" }),
    );
    act(() => store().setAnswer(q(5), text("typed here")));
    await ask();

    await pass(FLUSH_DEBOUNCE_MS);
    expect(store().lock).toBe("superseded");
    noDialog();
    expect(
      screen.getByText("Bài này đang mở ở thiết bị khác. Bạn không thể sửa ở đây nữa."),
    ).toBeInTheDocument();
    expect(header().queryByRole("button", { name: SUBMIT })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Câu 8" }));
    fireEvent.click(
      within(screen.getByRole("main")).getByRole("button", { name: "Xem lại & nộp" }),
    );
    await pass(0);
    noDialog();
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("still hands in a paper whose time is up, and says so instead of counting minutes", async () => {
    await open(2_000);
    vi.mocked(submitAttempt).mockRejectedValueOnce(new Error("offline"));
    await pass(2_000);
    act(() => store().lockNow("deadline"));
    expect(store().submitState).toBe("idle");

    await ask();
    expect(dialog()).toHaveAccessibleDescription(
      "Câu chưa trả lời sẽ không có điểm. Đã hết giờ làm bài.",
    );
    expect(facts()["Còn lại"]).toHaveTextContent(/^00:00$/);

    fireEvent.click(inDialog().getByRole("button", { name: SUBMIT }));
    await pass(0);
    expect(vi.mocked(submitAttempt).mock.lastCall?.[1]).toEqual({ reason: "manual" });
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
  });

  it("opens from the last question's button and from the right arrow there, and keys rest while it is open", async () => {
    await open();
    fireEvent.click(screen.getByRole("button", { name: "Câu 8" }));
    fireEvent.click(
      within(screen.getByRole("main")).getByRole("button", { name: "Xem lại & nộp" }),
    );
    await pass(0);
    expect(dialog()).toHaveAccessibleName(UNANSWERED);

    fireEvent.keyDown(window, { key: "ArrowLeft" });
    fireEvent.keyDown(window, { key: "a" });
    fireEvent.keyDown(window, { key: "f" });
    expect(onQuestion(8)).toBeInTheDocument();
    expect(store().answers[q(8)]).toBeUndefined();
    expect(store().flags.has(q(8))).toBe(false);

    fireEvent.click(inDialog().getByRole("button", { name: KEEP }));
    await pass(0);
    noDialog();

    fireEvent.keyDown(window, { key: "ArrowRight" });
    await pass(0);
    expect(dialog()).toHaveAccessibleName(UNANSWERED);
    expect(onQuestion(8)).toBeInTheDocument();
  });

  it("opens from the rail's button, with the rail still beside the paper", async () => {
    await open();
    const rail = () =>
      within(
        screen.getByRole("complementary", { name: "Danh sách câu", hidden: true }),
      );
    fireEvent.click(rail().getByRole("button", { name: "Xem lại & nộp" }));
    await pass(0);

    expect(dialog()).toHaveAccessibleName(UNANSWERED);
    expect(
      rail().getByRole("button", { name: "Xem lại & nộp", hidden: true }),
    ).toBeInTheDocument();
    expect(
      header().getByRole("button", { name: "Thoát khỏi bài làm", hidden: true }),
    ).toBeInTheDocument();
  });

  it("names the question on screen in what is recorded while it is open", async () => {
    await open();
    fireEvent.click(screen.getByRole("button", { name: "Câu 4" }));
    await ask();

    act(() => {
      window.dispatchEvent(new Event("offline"));
    });
    expect(bufferedEvents().at(-1)).toMatchObject({
      kind: "network_offline",
      questionId: q(4),
    });
  });

  it("stays under the focus dialog when the student leaves with it open, and is there after", async () => {
    await open();
    await ask();

    act(() => {
      window.dispatchEvent(new Event("blur"));
    });
    await pass(4_000);
    act(() => {
      window.dispatchEvent(new Event("focus"));
    });
    await pass(0);

    const alert = screen.getByRole("alertdialog");
    expect(alert).toHaveTextContent(
      "Lần này được tính là lần 1 trong 2 lần được phép.",
    );
    expect(screen.queryByRole("dialog")).toBeNull();

    fireEvent.click(within(alert).getByRole("button", { name: "Quay lại bài làm" }));
    await pass(0);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(dialog()).toHaveAccessibleName(UNANSWERED);
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("keeps the chips in a box of their own that scrolls on a long paper", async () => {
    const base = deckSession(NOW);
    const first = base.questions[0];
    if (first === undefined) throw new Error("no question");
    await open(DECK_LEFT_MS, {
      questions: Array.from({ length: 80 }, (_, index) => ({
        ...first,
        id: `question-${index}`,
      })),
      answers: {},
    });
    await ask();

    expect(dialog()).toHaveAccessibleName("Nộp bài khi còn 80 câu chưa trả lời?");
    expect(chips()).toHaveLength(80);
    expect(inDialog().getByRole("group", { name: "Đến câu" })).toHaveClass(
      "max-h-40",
      "overflow-y-auto",
      "flex-none",
    );
  });
});

describe("the Submit dialog on a phone", () => {
  beforeEach(() => {
    viewport("phone");
  });

  it("opens from the header and from the question sheet's button, which closes first", async () => {
    await open();
    await ask();
    expect(dialog()).toHaveAccessibleName(UNANSWERED);
    expect(dialog()).toHaveClass("w-[min(27.5rem,calc(100%-1.5rem))]", "top-[50%]");
    fireEvent.click(inDialog().getByRole("button", { name: KEEP }));
    await pass(0);
    noDialog();

    fireEvent.click(screen.getByRole("button", { name: "Danh sách câu" }));
    await pass(0);
    fireEvent.click(inDialog().getByRole("button", { name: "Xem lại & nộp" }));
    await pass(0);

    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(dialog()).toHaveAccessibleName(UNANSWERED);
  });
});

describe("the Submit dialog in English", () => {
  beforeEach(async () => {
    await act(() => i18n.changeLanguage("en"));
  });

  it("is the deck's wording", async () => {
    await open();
    fireEvent.click(header().getByRole("button", { name: "Submit" }));
    await pass(0);

    expect(dialog()).toHaveAccessibleName("Submit with 5 unanswered?");
    expect(dialog()).toHaveAccessibleDescription(
      "Unanswered questions score zero. You still have 38 minutes.",
    );
    expect(
      inDialog()
        .getAllByRole("term")
        .map((term) => term.textContent),
    ).toEqual(["Answered", "Flagged", "Time left"]);
    expect(inDialog().getByRole("group", { name: "Go to" })).toBeInTheDocument();
    expect(inDialog().getByRole("button", { name: "Question 4" })).toHaveTextContent(
      /^4$/,
    );
    expect(
      inDialog().getByRole("button", { name: "Keep working" }),
    ).toBeInTheDocument();
    expect(inDialog().getByRole("button", { name: "Submit test" })).toBeInTheDocument();
  });

  it("counts one minute in the singular and asks the plain question when all are answered", async () => {
    await open(MINUTE + 30_000);
    fireEvent.click(header().getByRole("button", { name: "Submit" }));
    await pass(0);
    expect(dialog()).toHaveAccessibleDescription(
      "Unanswered questions score zero. You still have 1 minute.",
    );
    fireEvent.click(inDialog().getByRole("button", { name: "Keep working" }));
    await pass(0);

    act(() => {
      for (const n of [4, 5, 6, 7, 8]) store().setAnswer(q(n), text("answered"));
    });
    fireEvent.click(header().getByRole("button", { name: "Submit" }));
    await pass(0);
    expect(dialog()).toHaveAccessibleName("Submit your test?");
    expect(dialog()).toHaveAccessibleDescription(
      "You cannot change your answers after submitting.",
    );
  });
});
