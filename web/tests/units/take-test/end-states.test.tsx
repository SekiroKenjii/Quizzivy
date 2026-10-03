import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { toast } from "sonner";
import { Toaster } from "@/components/ui/sonner";
import FocusLayout from "@/layouts/FocusLayout";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import { pending as bufferedEvents } from "@/features/integrity/buffer";
import {
  getAttempt,
  saveAnswers,
  submitAttempt,
  type AttemptSession,
  type IntegrityPolicy,
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
const SUBMIT = "Nộp bài";
const COPY_OFF = "Sao chép và dán đã bị tắt trong bài này";
const AUTO_BODY =
  "Bạn đã rời trang làm bài quá số lần được phép nên bài đã được nộp. Câu trả lời của bạn được giữ lại để chấm. Giáo viên đã được báo.";

const q = (n: number) => `018f0000-0000-7000-8000-00000000b00${n}`;
const store = () => useTakeTestStore.getState();
const pass = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const deadline = () => new Date(store().deadlineAt).toISOString();
const base = deckSession(NOW);

function submitted() {
  return {
    ...base.attempt,
    status: "submitted" as const,
    submittedAt: new Date().toISOString(),
  };
}

type Over = Omit<Partial<AttemptSession>, "attempt"> & {
  attempt?: Partial<AttemptSession["attempt"]>;
};

function policy(over: Partial<IntegrityPolicy>): Over {
  return { integrity: { ...base.integrity, ...over } };
}

function counted(focusLossCount: number): Over {
  return { attempt: { integrity: { focusLossCount, flagged: false } } };
}

async function open(over: Over = {}, left = DECK_LEFT_MS) {
  const session = deckSession(NOW, left);
  vi.mocked(getAttempt).mockResolvedValue({
    ...session,
    ...over,
    attempt: { ...session.attempt, ...over.attempt },
  });
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
  render(
    <>
      <RouterProvider router={router} />
      <Toaster />
    </>,
  );
  await pass(0);
  return router;
}

const header = () => within(screen.getByRole("banner", { hidden: true }));
const tile = () => screen.getByRole("main").querySelector("svg")?.parentElement;
const bar = () => document.querySelector<HTMLElement>('[data-slot="notice-bar"]');

async function handIn() {
  fireEvent.click(header().getByRole("button", { name: SUBMIT }));
  await pass(0);
  fireEvent.click(
    within(screen.getByRole("dialog")).getByRole("button", { name: SUBMIT }),
  );
  await pass(0);
}

async function leaveFor(ms: number) {
  act(() => {
    window.dispatchEvent(new Event("blur"));
  });
  await pass(ms);
  act(() => {
    window.dispatchEvent(new Event("focus"));
  });
  await pass(0);
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

function fullscreen(over: { enabled: boolean; element?: Element | null }) {
  Object.defineProperty(document, "fullscreenEnabled", {
    configurable: true,
    get: () => over.enabled,
  });
  Object.defineProperty(document, "fullscreenElement", {
    configurable: true,
    get: () => over.element ?? null,
  });
}

const request = vi.fn<() => Promise<void>>();
const exit = vi.fn<() => Promise<void>>();

beforeEach(() => {
  viewport("desktop");
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  request.mockReset().mockResolvedValue(undefined);
  exit.mockReset().mockResolvedValue(undefined);
  document.documentElement.requestFullscreen = request;
  document.exitFullscreen = exit;
  fullscreen({ enabled: true });
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () => deckSaved(new Date(), deadline()));
  vi.mocked(submitAttempt)
    .mockReset()
    .mockImplementation(async () => submitted());
});

afterEach(async () => {
  act(() => {
    toast.dismiss();
  });
  store().reset();
  fullscreen({ enabled: false });
  await act(() => i18n.changeLanguage("vi"));
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the submitted screen", () => {
  it("says the paper is in, when and with how many answers, on a success tile", async () => {
    await open();
    await handIn();

    expect(
      screen.getByRole("heading", { level: 1, name: "Bài đã được nộp." }),
    ).toHaveClass("text-xl", "font-semibold");
    expect(
      screen.getByText(
        "Bài làm của bạn đã được nộp với các câu bạn đã trả lời. Mọi câu đã lưu đều được giữ nguyên.",
      ),
    ).toHaveClass("text-muted-fg", "text-base");
    expect(screen.getByText("Nộp lúc 12:00 · 22/09 · 3/8 câu đã trả lời")).toHaveClass(
      "text-meta",
      "tabular-nums",
    );
    expect(tile()).toHaveClass("size-12", "rounded-xl", "bg-success-soft");
    expect(tile()).toHaveClass("text-success-ink");
    expect(tile()?.querySelector("svg")).toHaveClass("lucide-circle-check");
    expect(screen.getByRole("main").firstElementChild).toHaveClass(
      "max-w-120",
      "items-center",
      "text-center",
    );

    expect(
      within(screen.getByRole("main"))
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Về trang chủ", "Xem bài đã nộp"]);
    expect(screen.getByRole("button", { name: "Xem bài đã nộp" })).toHaveClass(
      "bg-primary",
    );
    expect(screen.getByRole("button", { name: "Về trang chủ" })).toHaveClass(
      "shadow-none",
      "border",
    );
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByRole("timer")).toBeNull();
  });

  it.each([
    { button: "Về trang chủ", path: "/app" },
    { button: "Xem bài đã nộp", path: `/app/attempts/${DECK_ATTEMPT}/result` },
  ])("follows $button", async ({ button, path }) => {
    const router = await open();
    await handIn();
    expect(router.state.location.pathname).toBe(`/app/attempts/${DECK_ATTEMPT}`);

    fireEvent.click(screen.getByRole("button", { name: button }));
    await pass(0);
    expect(router.state.location.pathname).toBe(path);
    expect(router.state.historyAction).toBe("REPLACE");
  });

  it("is the same screen on a phone, with no header over it", async () => {
    viewport("phone");
    await open();
    await handIn();
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("banner")).toBeNull();
    expect(tile()).toHaveClass("bg-success-soft");
  });
});

describe("the timer at zero", () => {
  it("submits without the student and says the time ran out, on the timer's tile", async () => {
    await open({}, 2_000);
    await pass(2_000);

    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith(DECK_ATTEMPT, {
      reason: "timer_expired",
    });
    expect(
      screen.getByRole("heading", { name: "Bài đã hết giờ và được nộp tự động." }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Hết thời gian làm bài, nên bài đã được nộp với các câu bạn đã trả lời. Mọi câu đã lưu đều được giữ nguyên.",
      ),
    ).toBeInTheDocument();
    expect(tile()).toHaveClass("bg-muted", "text-fg");
    expect(tile()?.querySelector("svg")).toHaveClass("lucide-timer");
  });

  it("reads 00:00 in the danger tones and takes no more answers while the submission is out", async () => {
    await open({}, 2_000);
    const sending = heldSubmit();
    await pass(2_000);

    expect(screen.getByRole("timer")).toHaveTextContent("00:00");
    expect(screen.getByRole("timer")).toHaveClass("bg-danger-soft", "text-danger-ink");
    expect(screen.queryByRole("dialog")).toBeNull();
    act(() => store().setAnswer(q(5), text("too late")));
    expect(store().answers[q(5)]).toBeUndefined();

    await sending.land();
    expect(
      screen.getByRole("heading", { name: "Bài đã hết giờ và được nộp tự động." }),
    ).toBeInTheDocument();
  });

  it("says the time is up in the bar when the server refuses a late save, and keeps Submit", async () => {
    await open({}, 60_000);
    vi.mocked(saveAnswers).mockRejectedValueOnce(
      new ApiError({ status: 409, code: "DEADLINE_PASSED", message: "late" }),
    );
    act(() => store().setAnswer(q(5), text("typed at the bell")));
    await pass(FLUSH_DEBOUNCE_MS);

    expect(store().lock).toBe("deadline");
    expect(bar()).toHaveTextContent("Đã hết giờ làm bài.");
    expect(bar()).toHaveClass("bg-warning-soft", "text-warning-ink", "border-b");
    expect(bar()?.querySelector("svg")).toHaveClass("lucide-timer-off");
    expect(bar()?.previousElementSibling).toBe(screen.getByRole("banner"));
    expect(header().getByRole("button", { name: SUBMIT })).toBeInTheDocument();
  });
});

describe("an auto-submit", () => {
  const auto = policy({ maxFocusLoss: 1, onLimitExceeded: "auto_submit" });

  it("replaces the paper with the notice, then says why the paper went in", async () => {
    await open({ ...auto, ...counted(1) });
    const sending = heldSubmit();
    await leaveFor(4_000);

    expect(
      screen.getByRole("heading", { level: 1, name: "Bài làm đang được nộp" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Đang gửi câu trả lời và nộp bài…",
    );
    expect(screen.queryByRole("banner")).toBeNull();
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith(DECK_ATTEMPT, {
      reason: "auto_submit",
    });

    await sending.land();
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp vì rời trang lần nữa." }),
    ).toBeInTheDocument();
    expect(screen.getByText(AUTO_BODY)).toBeInTheDocument();
    expect(screen.getByRole("main")).not.toHaveTextContent(/vi phạm|gian lận/i);
    expect(tile()).toHaveClass("bg-warning-soft", "text-warning-ink");
    expect(tile()?.querySelector("svg")).toHaveClass("lucide-eye-off");
  });

  it("warns inside the allowance with the focus dialog, and submits nothing", async () => {
    await open(auto);
    await leaveFor(4_000);

    expect(screen.getByRole("alertdialog")).toHaveTextContent(
      "Lần này được tính là lần 1 trong 1 lần được phép. Nếu bạn rời trang lần nữa, bài sẽ được nộp ngay.",
    );
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("says so in English without the word violation", async () => {
    await act(() => i18n.changeLanguage("en"));
    await open({ ...auto, ...counted(1) });
    await leaveFor(4_000);

    expect(
      screen.getByRole("heading", {
        name: "The test was submitted because you left the page again.",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveTextContent(
      "You left the test more times than allowed, so it was submitted. Your answers are kept for grading. Your teacher has been told.",
    );
    expect(screen.getByRole("main")).not.toHaveTextContent(/violat|cheat/i);
  });
});

describe("a paper this tab can no longer write", () => {
  it("says another device has it, in the bar, with no Submit and no focus dialog after", async () => {
    await open();
    vi.mocked(saveAnswers).mockRejectedValueOnce(
      new ApiError({ status: 409, code: "SESSION_SUPERSEDED", message: "elsewhere" }),
    );
    act(() => store().setAnswer(q(5), text("typed here")));
    await pass(FLUSH_DEBOUNCE_MS);

    expect(bar()).toHaveTextContent(
      "Bài này đang mở ở thiết bị khác. Bạn không thể sửa ở đây nữa.",
    );
    expect(bar()).toHaveClass("bg-warning-soft", "text-warning-ink");
    expect(bar()?.querySelector("svg")).toHaveClass("lucide-monitor-smartphone");
    expect(bar()?.previousElementSibling).toBe(screen.getByRole("banner"));
    expect(header().queryByRole("button", { name: SUBMIT })).toBeNull();
    expect(screen.getByRole("main", { name: "Câu 1" })).toBeInTheDocument();

    await leaveFor(4_000);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("says a paper that had already ended has ended, with a lock", async () => {
    await open({ attempt: { status: "submitted" } });
    expect(bar()).toHaveTextContent("Bài làm này đã kết thúc.");
    expect(bar()?.querySelector("svg")).toHaveClass("lucide-lock");
    expect(header().queryByRole("button", { name: SUBMIT })).toBeNull();
    expect(exit).not.toHaveBeenCalled();
  });
});

describe("leaving fullscreen when the attempt ends", () => {
  const asked = policy({ requireFullscreen: true });

  it("leaves it when the student submits, and not before", async () => {
    fullscreen({ enabled: true, element: document.body });
    await open(asked);
    fireEvent.click(header().getByRole("button", { name: SUBMIT }));
    await pass(0);
    expect(exit).not.toHaveBeenCalled();

    fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: SUBMIT }),
    );
    await pass(0);
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
    expect(exit).toHaveBeenCalledTimes(1);
  });

  it("leaves it when the timer submits", async () => {
    fullscreen({ enabled: true, element: document.body });
    await open(asked, 2_000);
    expect(exit).not.toHaveBeenCalled();
    await pass(2_000);
    expect(
      screen.getByRole("heading", { name: "Bài đã hết giờ và được nộp tự động." }),
    ).toBeInTheDocument();
    expect(exit).toHaveBeenCalledTimes(1);
  });

  it("leaves it when an auto-submit goes in, and not while it is still on its way", async () => {
    fullscreen({ enabled: true, element: document.body });
    await open({
      ...policy({
        requireFullscreen: true,
        maxFocusLoss: 1,
        onLimitExceeded: "auto_submit",
      }),
      ...counted(1),
    });
    const sending = heldSubmit();
    await leaveFor(4_000);
    expect(
      screen.getByRole("heading", { name: "Bài làm đang được nộp" }),
    ).toBeInTheDocument();
    expect(exit).not.toHaveBeenCalled();

    await sending.land();
    expect(exit).toHaveBeenCalledTimes(1);
  });

  it("records the student's own change during the sitting, and nothing of its own exit", async () => {
    await open(asked);
    fullscreen({ enabled: true, element: document.body });
    act(() => {
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    expect(bufferedEvents().map((recorded) => recorded.kind)).toEqual([
      "fullscreen_enter",
    ]);

    exit.mockImplementation(async () => {
      fullscreen({ enabled: true, element: null });
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    await handIn();
    expect(exit).toHaveBeenCalledTimes(1);
    expect(bufferedEvents()).toEqual([]);
  });

  it("stays in fullscreen on a paper that is locked without a submit", async () => {
    fullscreen({ enabled: true, element: document.body });
    await open(asked);
    vi.mocked(saveAnswers).mockRejectedValueOnce(
      new ApiError({ status: 409, code: "ATTEMPT_CLOSED", message: "ended" }),
    );
    act(() => store().setAnswer(q(5), text("typed after the end")));
    await pass(FLUSH_DEBOUNCE_MS);

    expect(store().lock).toBe("closed");
    expect(bar()).toHaveTextContent("Bài làm này đã kết thúc.");
    expect(exit).not.toHaveBeenCalled();
  });

  it("leaves a fullscreen the assignment did not ask for alone", async () => {
    fullscreen({ enabled: true, element: document.body });
    await open();
    await handIn();
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
    expect(exit).not.toHaveBeenCalled();
  });

  it("asks nothing of a document that is not in fullscreen", async () => {
    await open(asked);
    await handIn();
    expect(exit).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
  });

  it("shows the submitted screen even when the browser refuses to leave", async () => {
    fullscreen({ enabled: true, element: document.body });
    exit.mockRejectedValue(new TypeError("Not in fullscreen mode"));
    await open(asked);
    await handIn();
    expect(exit).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("heading", { name: "Bài đã được nộp." }),
    ).toBeInTheDocument();
  });
});

describe("the return-to-fullscreen bar on the page", () => {
  const asked = policy({ requireFullscreen: true });

  it("sits under the header while the paper stays writable, and goes once fullscreen is back", async () => {
    await open(asked);

    expect(bar()).toHaveTextContent("Bạn đã thoát chế độ toàn màn hình.");
    expect(bar()?.previousElementSibling).toBe(screen.getByRole("banner"));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByRole("alertdialog")).toBeNull();

    fireEvent.click(screen.getByRole("radio", { name: /To compare parks/ }));
    expect(store().answers[q(1)]).toEqual({
      type: "choice",
      optionIds: ["018f0000-0000-7000-8000-00000000c101"],
    });

    fireEvent.click(screen.getByRole("button", { name: "Quay lại toàn màn hình" }));
    expect(request).toHaveBeenCalledTimes(1);

    fullscreen({ enabled: true, element: document.body });
    act(() => {
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    expect(bar()).toBeNull();
  });

  it("is absent when the assignment does not ask for fullscreen", async () => {
    await open();
    expect(bar()).toBeNull();
  });

  it("gives way to the lock's bar on a paper another device took", async () => {
    await open(asked);
    act(() => store().lockNow("superseded"));
    expect(bar()).toHaveTextContent("Bài này đang mở ở thiết bị khác.");
    expect(document.querySelectorAll('[data-slot="notice-bar"]')).toHaveLength(1);
  });
});

describe("the focus dialog on the page", () => {
  it("counts an absence of the minimum length against the allowance, and the header follows", async () => {
    await open();
    expect(header().getByText("Còn 2 lần rời trang")).toBeInTheDocument();

    await leaveFor(2_999);
    expect(screen.queryByRole("alertdialog")).toBeNull();

    await leaveFor(3_000);
    const alert = screen.getByRole("alertdialog");
    expect(alert).toHaveAccessibleName("Bạn vừa rời trang làm bài");
    expect(alert).toHaveTextContent(
      "Lần này được tính là lần 1 trong 2 lần được phép. Hãy ở lại trang này cho đến khi nộp bài.",
    );
    expect(alert).toHaveAttribute("data-scale", "deck");

    fireEvent.click(within(alert).getByRole("button", { name: "Quay lại bài làm" }));
    await pass(0);
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(header().getByText("Còn 1 lần rời trang")).toBeInTheDocument();
  });

  it("says the teacher has been told once the server's count and this sitting's pass the allowance", async () => {
    await open(counted(2));
    expect(header().getByText("Hết lần rời trang")).toBeInTheDocument();

    await leaveFor(4_000);
    expect(screen.getByRole("alertdialog")).toHaveTextContent(
      "Bạn đã rời trang làm bài 3 lần, nhiều hơn 2 lần được phép. Giáo viên đã được báo. Câu trả lời của bạn vẫn an toàn.",
    );
    expect(submitAttempt).not.toHaveBeenCalled();
    expect(header().getByText("Giáo viên đã được báo")).toHaveClass("text-warning-ink");
  });
});

describe("copy and paste under blockCopyPaste", () => {
  const clipboard = (kind: "copy" | "cut" | "paste") => {
    const event = new Event(kind, { bubbles: true, cancelable: true });
    act(() => {
      document.dispatchEvent(event);
    });
    return event;
  };

  it.each(["copy", "cut", "paste"] as const)(
    "tells the student on %s with the danger toast and a ban icon, and still records and stops it",
    async (kind) => {
      await open();
      const event = clipboard(kind);
      await pass(0);

      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(COPY_OFF);
      const card = alert.closest("[data-sonner-toast]");
      expect(card?.querySelector("[data-icon] svg")).toHaveClass(
        "lucide-ban",
        "text-danger",
        "size-4",
      );
      expect(event.defaultPrevented).toBe(true);
      expect(bufferedEvents().map((recorded) => recorded.kind)).toEqual([kind]);
    },
  );

  it("still tells the student on the submitted screen, where the monitor still stops it", async () => {
    await open();
    await handIn();
    const event = clipboard("copy");
    await pass(0);

    expect(screen.getByRole("alert")).toHaveTextContent(COPY_OFF);
    expect(event.defaultPrevented).toBe(true);
  });

  it("shows one toast however many times the student tries", async () => {
    await open();
    clipboard("copy");
    clipboard("paste");
    clipboard("copy");
    await pass(0);

    expect(screen.getAllByText(COPY_OFF)).toHaveLength(1);
    expect(bufferedEvents().map((recorded) => recorded.kind)).toEqual([
      "copy",
      "paste",
      "copy",
    ]);
  });

  it("says nothing and stops nothing when the assignment allows it, and still records it", async () => {
    await open(policy({ blockCopyPaste: false }));
    const event = clipboard("copy");
    await pass(0);

    expect(screen.queryByText(COPY_OFF)).toBeNull();
    expect(event.defaultPrevented).toBe(false);
    expect(bufferedEvents().map((recorded) => recorded.kind)).toEqual(["copy"]);
  });

  it("uses the deck's English", async () => {
    await act(() => i18n.changeLanguage("en"));
    await open();
    clipboard("copy");
    await pass(0);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Copy and paste are turned off in this test",
    );
  });
});
