import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  onTestFinished,
  vi,
} from "vitest";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient } from "@tanstack/react-query";
import { delay, http, HttpResponse } from "msw";
import AssignmentIntroPage from "@/features/assignments/pages/AssignmentIntroPage";
import i18n from "@/lib/i18n";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { viewport } from "@tests/support/viewport";
import { useAuthStore } from "@/stores/auth";
import {
  ASSIGNMENT,
  ATTEMPT,
  BASE,
  POLICY,
  STUDENT,
  attemptSession,
  detail,
  mockStart,
  renderAt,
} from "./support";

const DETAIL = `${BASE}/app/assignments/${ASSIGNMENT}`;
const START = `${DETAIL}/attempts`;
const ROUTES = [
  { path: "/app/assignments/:id", element: <AssignmentIntroPage /> },
  { path: "/app", element: <p>home page</p> },
];

let asked = 0;

function serve(over: Record<string, unknown> = {}) {
  server.use(
    http.get(DETAIL, () => {
      asked += 1;
      return contractJson("/app/assignments/{id}", "get", 200, detail(over));
    }),
  );
}

function show(over: Record<string, unknown> = {}, client?: QueryClient) {
  serve(over);
  return renderAt(`/app/assignments/${ASSIGNMENT}`, ROUTES, client);
}

function refuse(code: string, message: string, status: 403 | 409 = 409) {
  const calls: string[] = [];
  server.use(
    http.post(START, () => {
      calls.push("start");
      return contractJson("/app/assignments/{id}/attempts", "post", status, {
        error: { code, message, requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e" },
      });
    }),
  );
  return calls;
}

function listen(status: 200 | 409 = 200) {
  const sent: unknown[] = [];
  server.use(
    http.post(START, async ({ request }) => {
      const text = await request.text();
      sent.push(text === "" ? null : JSON.parse(text));
      if (status === 409) {
        return contractJson("/app/assignments/{id}/attempts", "post", 409, {
          error: {
            code: "ATTEMPT_CLOSED",
            message: "Bài làm này đã kết thúc.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        });
      }
      return contractJson(
        "/app/assignments/{id}/attempts",
        "post",
        200,
        attemptSession(),
      );
    }),
  );
  return sent;
}

function takeAway(status: 403 | 404 = 403) {
  server.use(
    http.get(DETAIL, () =>
      contractJson("/app/assignments/{id}", "get", status, {
        error: {
          code: status === 403 ? "FORBIDDEN" : "NOT_FOUND",
          message: "x",
          requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
        },
      }),
    ),
  );
}

async function rules() {
  const heading = await screen.findByRole("heading", {
    level: 2,
    name: "Trước khi bắt đầu",
  });
  return within(heading.parentElement!)
    .getAllByRole("listitem")
    .map((item) => item.textContent);
}

async function facts() {
  await screen.findByRole("heading", { level: 1 });
  return screen
    .getAllByRole("term")
    .map((term) => [term.textContent, term.nextElementSibling?.textContent]);
}

function startButton() {
  return screen.findByRole("button", { name: "Bắt đầu làm bài" });
}

async function ask(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await startButton());
  return screen.findByRole("dialog", { name: "Bắt đầu ngay?" });
}

function confirm(dialog: HTMLElement) {
  return within(dialog).getByRole("button", { name: "Bắt đầu" });
}

const request = vi.fn<() => Promise<void>>();
let supported = true;

beforeEach(() => {
  asked = 0;
  supported = true;
  viewport("desktop");
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z"));
  request.mockReset().mockResolvedValue(undefined);
  document.documentElement.requestFullscreen = request;
  Object.defineProperty(document, "fullscreenEnabled", {
    configurable: true,
    get: () => supported,
  });
  Object.defineProperty(document, "fullscreenElement", {
    configurable: true,
    get: () => null,
  });
});
afterEach(async () => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  await i18n.changeLanguage("vi");
});

describe("before you start, per policy", () => {
  it("lists the default paper's five sentences in order", async () => {
    show();
    expect(await rules()).toEqual([
      "Bài mở đến 21:00 hôm nay.",
      "Đồng hồ chạy từ lúc bạn bấm Bắt đầu và không dừng lại, kể cả khi bạn đóng trang.",
      "Sao chép và dán bị tắt.",
      "Mỗi lần rời trang làm bài đều được ghi lại.",
      "Bạn sẽ xem được điểm và giải thích sau khi nộp bài.",
    ]);
  });

  it("states fullscreen and a limit that tells the teacher", async () => {
    show({ integrity: { ...POLICY, requireFullscreen: true, maxFocusLoss: 2 } });
    const list = await rules();
    expect(list).toContain("Bài chạy ở chế độ toàn màn hình.");
    expect(list).toContain(
      "Bạn được rời trang làm bài 2 lần. Sau đó, giáo viên sẽ được báo.",
    );
  });

  it("says a warning is a warning, and nothing of copying when it is allowed", async () => {
    show({
      integrity: {
        ...POLICY,
        blockCopyPaste: false,
        maxFocusLoss: 3,
        onLimitExceeded: "warn",
      },
    });
    const list = await rules();
    expect(list).toContain(
      "Bạn được rời trang làm bài 3 lần. Sau đó, bạn sẽ được nhắc.",
    );
    expect(list.join(" ")).not.toMatch(/Sao chép|giáo viên/);
  });

  it("says the paper is submitted when the policy submits it", async () => {
    show({
      integrity: { ...POLICY, maxFocusLoss: -1, onLimitExceeded: "auto_submit" },
    });
    expect(await rules()).toContain(
      "Không được rời trang làm bài. Nếu bạn rời trang, bài sẽ được nộp ngay.",
    );
  });

  it("does not promise fullscreen on a browser that has none", async () => {
    supported = false;
    show({ integrity: { ...POLICY, requireFullscreen: true } });
    const list = await rules();
    expect(list).toContain(
      "Bài yêu cầu toàn màn hình, nhưng trình duyệt này không hỗ trợ. Bạn vẫn làm bài bình thường.",
    );
    expect(list).not.toContain("Bài chạy ở chế độ toàn màn hình.");
  });

  it("states the listening limit and the shared allowance", async () => {
    show({ hasAudio: true, hasSharedAudio: true, audioMaxPlays: 2 });
    const list = await rules();
    expect(list).toContain(
      "Mỗi bài nghe có giới hạn riêng, từ 2 lượt. Lượt nghe thêm được ghi lại.",
    );
    expect(list).toContain(
      "Các câu dùng chung bài nghe cũng dùng chung lượt nghe. Đổi câu hoặc tải lại trang không đặt lại lượt nghe.",
    );
  });

  it("says nothing of a shared allowance when no recording is shared", async () => {
    show({ hasAudio: true, audioMaxPlays: 2 });
    expect((await rules()).join(" ")).not.toMatch(/dùng chung/);
  });

  it("puts each sentence's own icon beside it", async () => {
    show({
      integrity: { ...POLICY, requireFullscreen: true },
      hasAudio: true,
      audioMaxPlays: 2,
    });
    const heading = await screen.findByRole("heading", { level: 2 });
    const icons = within(heading.parentElement!)
      .getAllByRole("listitem")
      .map((item) => /lucide-[a-z-]+/.exec(item.innerHTML)?.[0]);
    expect(icons).toEqual([
      "lucide-calendar-clock",
      "lucide-timer",
      "lucide-maximize",
      "lucide-clipboard-x",
      "lucide-eye",
      "lucide-headphones",
      "lucide-circle-check",
    ]);
  });

  it("says replays are unlimited when they are, and nothing without audio", async () => {
    show({ hasAudio: true, audioMaxPlays: null });
    expect(await rules()).toContain(
      "Bài có câu nghe; bạn nghe lại được không giới hạn.",
    );
  });

  it("says nothing of listening on a paper with no recording", async () => {
    show({ audioMaxPlays: 2 });
    expect((await rules()).join(" ")).not.toMatch(/nghe/);
  });

  it("says nothing of what is seen after submitting when the review hides the score", async () => {
    show({
      review: {
        showScore: false,
        showCorrectAnswers: true,
        showExplanations: true,
        release: "on_submit",
        showClassAverage: false,
      },
    });
    expect((await rules()).join(" ")).not.toMatch(/điểm|đáp án|giải thích/);
  });

  it("says the timer is already running for an attempt in progress", async () => {
    show({ hasLiveAttempt: true, lastAttemptId: ATTEMPT });
    const list = await rules();
    expect(list).toContain(
      "Đồng hồ đang chạy và không dừng lại, kể cả khi bạn đóng trang.",
    );
    expect(list.join(" ")).not.toMatch(/bấm Bắt đầu/);
  });

  it("gives the opening and the close of a paper that has not opened", async () => {
    show({
      status: "scheduled",
      opensAt: "2026-09-01T01:00:00Z",
      closesAt: "2026-09-03T14:00:00Z",
    });
    expect((await rules())[0]).toBe(
      "Bài mở từ 08:00, Thứ 3, 01/09 đến 21:00, Thứ 5, 03/09.",
    );
  });

  it("gives no date for a closed paper", async () => {
    show({ status: "closed", closesAt: "2026-08-28T14:00:00Z" });
    expect((await rules())[0]).toBe(
      "Đồng hồ chạy từ lúc bạn bấm Bắt đầu và không dừng lại, kể cả khi bạn đóng trang.",
    );
  });

  it("reads in English", async () => {
    await i18n.changeLanguage("en");
    show();
    const heading = await screen.findByRole("heading", { name: "Before you start" });
    const list = within(heading.parentElement!)
      .getAllByRole("listitem")
      .map((item) => item.textContent);
    expect(list).toEqual([
      "Available until today 21:00.",
      "The timer starts when you press Start and does not pause, even if you close the page.",
      "Copy and paste are turned off.",
      "Leaving the test is recorded.",
      "You will see your score and explanations after submitting.",
    ]);
    expect(screen.getByRole("button", { name: "Start test" })).toBeInTheDocument();
    expect(screen.getAllByRole("term").map((term) => term.textContent)).toEqual([
      "Time limit",
      "Questions",
      "Attempts",
    ]);
    expect(screen.getByText("0 of 2")).toBeInTheDocument();
  });
});

describe("the teacher's note", () => {
  const NOTE = "Mang theo máy tính cầm tay.\nKhông dùng điện thoại.";

  async function noteBlock(name: string) {
    const heading = await screen.findByRole("heading", { level: 2, name });
    return heading.closest("section")!;
  }

  it("is drawn between 'Before you start' and the action, under the teacher's name", async () => {
    show({ studentNote: NOTE, teacherName: "Hoàng Thương" });
    const note = await noteBlock("Ghi chú từ Hoàng Thương");
    const before = screen
      .getByRole("heading", { level: 2, name: "Trước khi bắt đầu" })
      .closest("section")!;
    const action = await startButton();
    expect(before.compareDocumentPosition(note)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(note.compareDocumentPosition(action)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(within(note).getByText("TH")).toBeInTheDocument();
    expect(note.querySelector("p")!.textContent).toBe(NOTE);
  });

  it("is left out when the teacher wrote none", async () => {
    show({ studentNote: null, teacherName: "Hoàng Thương" });
    await startButton();
    expect(screen.queryByText(/Ghi chú từ/)).not.toBeInTheDocument();
    expect(document.querySelector('[data-slot="teacher-note"]')).toBeNull();
  });

  it("is left out when the server sends no note at all", async () => {
    show({ teacherName: "Hoàng Thương" });
    await startButton();
    expect(document.querySelector('[data-slot="teacher-note"]')).toBeNull();
  });

  it("says 'your teacher' when the teacher is not known", async () => {
    show({ studentNote: NOTE, teacherName: null });
    const note = await noteBlock("Ghi chú từ giáo viên");
    expect(note.querySelector("p")!.textContent).toBe(NOTE);
  });

  it("is plain text: markup in it is shown as typed and runs nothing", async () => {
    const typed =
      '<b>Đọc kỹ</b> <img src=x onerror="alert(1)"> **đề** [link](https://example.test)';
    show({ studentNote: typed, teacherName: "Hoàng Thương" });
    const note = await noteBlock("Ghi chú từ Hoàng Thương");
    expect(note.querySelector("p")!.textContent).toBe(typed);
    expect(note.querySelector("b, img, a, strong")).toBeNull();
  });
});

describe("the score sentence of a result released after the close", () => {
  it("says the score comes once the test closes", async () => {
    show({
      review: {
        showScore: true,
        showCorrectAnswers: true,
        showExplanations: false,
        release: "after_close",
        showClassAverage: false,
      },
    });
    const list = await rules();
    expect(list.at(-1)).toBe("Bạn sẽ xem được điểm và đáp án đúng sau khi bài đóng.");
    expect(list.join(" ")).not.toMatch(/sau khi nộp/);
  });
});

describe("the header and the three facts", () => {
  it("names the time limit, the questions and the attempts used", async () => {
    show();
    expect(await facts()).toEqual([
      ["Thời lượng", "45 phút"],
      ["Số câu", "24"],
      ["Lượt làm", "0/2"],
    ]);
  });

  it("counts a submitted attempt", async () => {
    show({ attemptsUsed: 1 });
    expect((await facts())[2]).toEqual(["Lượt làm", "1/2"]);
  });

  it("counts the attempt in progress, and never more than the paper allows", async () => {
    show({
      attemptsUsed: 0,
      maxAttempts: 1,
      hasLiveAttempt: true,
      lastAttemptId: ATTEMPT,
    });
    expect((await facts())[2]).toEqual(["Lượt làm", "1/1"]);
  });

  it("does not count a live attempt twice once the server counts it", async () => {
    show({
      attemptsUsed: 1,
      maxAttempts: 1,
      hasLiveAttempt: true,
      lastAttemptId: ATTEMPT,
    });
    expect((await facts())[2]).toEqual(["Lượt làm", "1/1"]);
  });

  it("counts an attempt in progress as one more, below the maximum", async () => {
    show({
      attemptsUsed: 1,
      maxAttempts: 3,
      hasLiveAttempt: true,
      lastAttemptId: ATTEMPT,
    });
    expect((await facts())[2]).toEqual(["Lượt làm", "2/3"]);
  });

  it("puts the class above the title, without the teacher", async () => {
    show({ className: "IELTS Foundation", teacherName: "Cô Thương" });
    const title = await screen.findByRole("heading", {
      level: 1,
      name: "Unit 5 — Present perfect",
    });
    expect(title.previousElementSibling).toHaveTextContent(/^IELTS Foundation$/);
    expect(screen.queryByText(/Cô Thương/)).toBeNull();
  });

  it("leaves no empty line when the server names no class", async () => {
    show({ className: null });
    const title = await screen.findByRole("heading", { level: 1 });
    expect(title.previousElementSibling).toBeNull();
  });

  it("links back to Home from 768", async () => {
    const user = userEvent.setup();
    const router = show();
    await user.click(await screen.findByRole("link", { name: "Trang chủ" }));
    expect(router.state.location.pathname).toBe("/app");
  });

  it("asks the browser for the shell's 768, not another width", async () => {
    const queries: string[] = [];
    vi.stubGlobal("matchMedia", (query: string) => {
      queries.push(query);
      return {
        matches: true,
        media: query,
        addEventListener: () => {},
        removeEventListener: () => {},
      } as unknown as MediaQueryList;
    });
    show();
    await screen.findByRole("heading", { level: 1 });
    expect(new Set(queries)).toEqual(new Set(["(min-width: 768px)"]));
  });

  it("leaves the way back to the shell's header on a phone", async () => {
    viewport("phone");
    show();
    await screen.findByRole("heading", { level: 1 });
    expect(screen.queryByRole("link", { name: "Trang chủ" })).toBeNull();
  });
});

describe("the cover under the action", () => {
  const STATES = [
    {
      state: "can be started",
      over: {},
      action: () => screen.findByRole("button", { name: "Bắt đầu làm bài" }),
    },
    {
      state: "is in progress",
      over: { hasLiveAttempt: true, lastAttemptId: ATTEMPT },
      action: () => screen.findByRole("button", { name: "Tiếp tục làm bài" }),
    },
    {
      state: "has not opened",
      over: {
        status: "scheduled",
        opensAt: "2026-09-01T01:00:00Z",
        closesAt: "2026-09-20T14:00:00Z",
      },
      action: () => screen.findByRole("button", { name: "Mở thứ ba" }),
    },
    {
      state: "has no attempt left",
      over: { attemptsUsed: 2, maxAttempts: 2 },
      action: () => screen.findByText("Bạn đã dùng hết số lượt làm bài."),
    },
  ];

  it.each(STATES)(
    "is the shell's 48px from 768 when the paper $state",
    async ({ over, action }) => {
      show(over);
      const bar = (await action()).parentElement;
      expect(bar).toHaveClass("sticky", "after:top-full", "after:bg-bg", "after:h-12");
      expect(bar).not.toHaveClass("after:h-7");
    },
  );

  it.each(STATES)(
    "is the shell's 28px on a phone when the paper $state",
    async ({ over, action }) => {
      viewport("phone");
      show(over);
      const bar = (await action()).parentElement;
      expect(bar).toHaveClass("sticky", "after:top-full", "after:bg-bg", "after:h-7");
      expect(bar).not.toHaveClass("after:h-12");
    },
  );
});

describe("a paper that has not opened", () => {
  async function waiting(opensAt: string) {
    show({ status: "scheduled", opensAt, closesAt: "2026-09-20T14:00:00Z" });
    await screen.findByRole("heading", { level: 1 });
    return screen.getByRole("button");
  }

  it("names the weekday inside the coming week", async () => {
    expect(await waiting("2026-09-01T01:00:00Z")).toHaveAccessibleName("Mở thứ ba");
  });

  it("names the hour when it opens today", async () => {
    expect(await waiting("2026-08-29T12:00:00Z")).toHaveAccessibleName(
      "Mở lúc 19:00 hôm nay",
    );
  });

  it("names the date from a week away, when the weekday would be today's", async () => {
    expect(await waiting("2026-09-05T01:00:00Z")).toHaveAccessibleName(
      "Mở Thứ 7, 05/09",
    );
  });

  it("still names the weekday six days away", async () => {
    expect(await waiting("2026-09-04T01:00:00Z")).toHaveAccessibleName("Mở thứ sáu");
  });

  it("cannot be started, and says when it can", async () => {
    const user = userEvent.setup();
    const calls = mockStart();
    const button = await waiting("2026-09-01T01:00:00Z");
    expect(button).toHaveAttribute("aria-disabled", "true");
    await user.click(button);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(calls).toEqual([]);
    expect(request).not.toHaveBeenCalled();
    expect(screen.getByText("Bạn bắt đầu được khi bài mở.")).toBeInTheDocument();
  });

  it("takes 'not open yet' from the server, not from this device's clock", async () => {
    show({
      status: "scheduled",
      opensAt: "2026-08-29T09:59:00Z",
      closesAt: "2026-08-29T14:00:00Z",
    });
    expect((await rules())[0]).toBe("Bài mở hôm nay, từ 16:59 đến 21:00.");
  });

  it("asks the server again each minute, and offers Start once it opens", async () => {
    show({
      status: "scheduled",
      opensAt: "2026-08-29T10:01:00Z",
      closesAt: "2026-09-20T14:00:00Z",
    });
    await screen.findByRole("button", { name: "Mở lúc 17:01 hôm nay" });
    expect(asked).toBe(1);
    serve();
    await act(() => vi.advanceTimersByTimeAsync(61_000));
    expect(await startButton()).toBeInTheDocument();
    expect(asked).toBe(2);
  });

  it("does not keep asking about a paper that is open", async () => {
    show();
    await startButton();
    await act(() => vi.advanceTimersByTimeAsync(125_000));
    expect(asked).toBe(1);
  });

  it("asks at the opening, not a minute after the last answer", async () => {
    show({
      status: "scheduled",
      opensAt: "2026-08-29T10:00:10Z",
      closesAt: "2026-09-20T14:00:00Z",
    });
    await screen.findByRole("button", { name: "Mở lúc 17:00 hôm nay" });
    serve();
    await act(() => vi.advanceTimersByTimeAsync(11_000));
    expect(await startButton()).toBeInTheDocument();
    expect(asked).toBe(2);
  });

  it("asks again when the tab is returned to", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
    });
    show(
      {
        status: "scheduled",
        opensAt: "2026-09-01T01:00:00Z",
        closesAt: "2026-09-20T14:00:00Z",
      },
      client,
    );
    await screen.findByRole("button", { name: "Mở thứ ba" });
    serve();
    act(() => {
      window.dispatchEvent(new Event("visibilitychange"));
    });
    expect(await startButton()).toBeInTheDocument();
    expect(asked).toBe(2);
  });
});

describe("when there is nothing to start", () => {
  it("reads an open paper once more when its close passes, and says it closed", async () => {
    const user = userEvent.setup();
    vi.setSystemTime(new Date("2026-08-29T13:58:30.250Z"));
    show();
    await ask(user);
    serve({ status: "closed" });
    await act(() => vi.advanceTimersByTimeAsync(95_000));
    expect(await screen.findByText("Bài này đã đóng.")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
    expect(asked).toBe(2);
    await act(() => vi.advanceTimersByTimeAsync(180_000));
    expect(asked).toBe(2);
  });

  it("asks once, not each minute, when the server still calls it open", async () => {
    vi.setSystemTime(new Date("2026-08-29T13:58:30.250Z"));
    show();
    await startButton();
    await act(() => vi.advanceTimersByTimeAsync(95_000));
    await waitFor(() => expect(asked).toBe(2));
    await act(() => vi.advanceTimersByTimeAsync(180_000));
    expect(asked).toBe(2);
    expect(await startButton()).toBeInTheDocument();
  });

  it("says the attempts are spent", async () => {
    show({ attemptsUsed: 2, maxAttempts: 2 });
    expect(
      await screen.findByText("Bạn đã dùng hết số lượt làm bài."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("says a closed one is closed", async () => {
    show({ status: "closed" });
    expect(await screen.findByText("Bài này đã đóng.")).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("says the attempts are spent of a closed paper with none left", async () => {
    show({ status: "closed", attemptsUsed: 2, maxAttempts: 2 });
    expect(
      await screen.findByText("Bạn đã dùng hết số lượt làm bài."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Bài này đã đóng.")).toBeNull();
  });

  it("gives a spent paper that has not reopened its own sentence", async () => {
    show({
      status: "scheduled",
      opensAt: "2026-09-01T01:00:00Z",
      closesAt: "2026-09-20T14:00:00Z",
      attemptsUsed: 2,
      maxAttempts: 2,
    });
    expect(
      await screen.findByText("Bạn đã dùng hết số lượt làm bài."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("offers Continue for an attempt in progress on a closed paper", async () => {
    show({ status: "closed", hasLiveAttempt: true, lastAttemptId: ATTEMPT });
    expect(
      await screen.findByRole("button", { name: "Tiếp tục làm bài" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Bài này đã đóng.")).toBeNull();
  });

  it("offers Continue for an attempt in progress with no attempt left", async () => {
    show({
      attemptsUsed: 1,
      maxAttempts: 1,
      hasLiveAttempt: true,
      lastAttemptId: ATTEMPT,
    });
    expect(
      await screen.findByRole("button", { name: "Tiếp tục làm bài" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Bạn đã dùng hết số lượt làm bài.")).toBeNull();
  });
});

describe("loading, failing and missing", () => {
  it("holds the page's shape while the paper loads", async () => {
    server.use(
      http.get(DETAIL, async () => {
        await delay("infinite");
        return HttpResponse.error();
      }),
    );
    renderAt(`/app/assignments/${ASSIGNMENT}`, ROUTES);
    expect(
      await screen.findByRole("status", { name: "Đang tải…" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Trang chủ" })).toBeInTheDocument();
    expect(screen.queryByRole("heading")).toBeNull();
  });

  it("offers a retry when the paper cannot be read, and reads it again", async () => {
    const user = userEvent.setup();
    server.use(http.get(DETAIL, () => HttpResponse.error()));
    renderAt(`/app/assignments/${ASSIGNMENT}`, ROUTES);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không tải được. Hãy thử lại.",
    );
    serve();
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await startButton()).toBeInTheDocument();
  });

  it.each([403, 404] as const)(
    "says a paper is not the student's on a %d, with the way Home",
    async (status) => {
      const user = userEvent.setup();
      takeAway(status);
      viewport("phone");
      const router = renderAt(`/app/assignments/${ASSIGNMENT}`, ROUTES);
      expect(
        await screen.findByText(
          "Không tìm thấy bài này, hoặc bài không được giao cho bạn.",
        ),
      ).toBeInTheDocument();
      expect(screen.queryByRole("alert")).toBeNull();
      expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
      await user.click(screen.getByRole("link", { name: "Trang chủ" }));
      expect(router.state.location.pathname).toBe("/app");
    },
  );

  it("says so when the paper stops being the student's while the page waits", async () => {
    show({
      status: "scheduled",
      opensAt: "2026-09-01T01:00:00Z",
      closesAt: "2026-09-20T14:00:00Z",
    });
    await screen.findByRole("button", { name: "Mở thứ ba" });
    takeAway();
    await act(() => vi.advanceTimersByTimeAsync(61_000));
    expect(
      await screen.findByText(
        "Không tìm thấy bài này, hoặc bài không được giao cho bạn.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("heading")).toBeNull();
  });

  it("says so after a start refused for a paper no longer the student's", async () => {
    const user = userEvent.setup();
    refuse("FORBIDDEN", "Bạn không có quyền làm bài này.", 403);
    show();
    const dialog = await ask(user);
    takeAway();
    await user.click(confirm(dialog));
    expect(
      await screen.findByText(
        "Không tìm thấy bài này, hoặc bài không được giao cho bạn.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByText("Bạn không có quyền làm bài này.")).toBeNull();
  });
});

describe("Start now?", () => {
  const RUNS = "Đồng hồ chạy ngay khi bạn bấm Bắt đầu và không dừng lại.";

  it("asks before anything starts", async () => {
    const user = userEvent.setup();
    const calls = mockStart();
    show({ integrity: { ...POLICY, requireFullscreen: true } });
    const dialog = await ask(user);
    expect(dialog).toHaveAccessibleDescription(
      "Đồng hồ 45 phút chạy ngay khi bạn bấm Bắt đầu và không dừng lại.",
    );
    expect(
      within(dialog)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Để sau", "Bắt đầu"]);
    expect(request).not.toHaveBeenCalled();
    expect(calls).toEqual([]);
  });

  it.each([
    [
      "13:14:30",
      "14:00:00",
      "Đồng hồ 45 phút chạy ngay khi bạn bấm Bắt đầu và không dừng lại.",
    ],
    ["13:39:30", "14:00:00", `Bài đóng lúc 21:00, nên bạn có 20 phút. ${RUNS}`],
    ["13:59:30", "14:00:00", `Bài đóng lúc 21:00, nên bạn có 1 phút. ${RUNS}`],
    ["16:39:30", "17:10:00", `Bài đóng lúc 00:10, 30/08, nên bạn có 30 phút. ${RUNS}`],
  ])("asked at %s UTC of a paper closing at %s, says: %s", async (at, closes, body) => {
    const user = userEvent.setup();
    vi.setSystemTime(new Date(`2026-08-29T${at}Z`));
    show({ closesAt: `2026-08-29T${closes}Z` });
    expect(await ask(user)).toHaveAccessibleDescription(body);
  });

  it("counts the minutes left from the click, not from the last repaint", async () => {
    const user = userEvent.setup();
    vi.setSystemTime(new Date("2026-08-29T13:40:00Z"));
    show({ closesAt: "2026-08-29T14:00:30Z" });
    await startButton();
    await act(() => vi.advanceTimersByTimeAsync(45_000));
    expect(await ask(user)).toHaveAccessibleDescription(
      `Bài đóng lúc 21:00, nên bạn có 19 phút. ${RUNS}`,
    );
  });

  it("counts the minutes again while the question stays open", async () => {
    const user = userEvent.setup();
    vi.setSystemTime(new Date("2026-08-29T13:14:40Z"));
    show({ closesAt: "2026-08-29T14:00:30Z" });
    const dialog = await ask(user);
    expect(dialog).toHaveAccessibleDescription(
      "Đồng hồ 45 phút chạy ngay khi bạn bấm Bắt đầu và không dừng lại.",
    );
    await act(() => vi.advanceTimersByTimeAsync(150_000));
    expect(dialog).toHaveAccessibleDescription(
      `Bài đóng lúc 21:00, nên bạn có 43 phút. ${RUNS}`,
    );
  });

  it("keeps its sentence while it fades out", async () => {
    const user = userEvent.setup();
    const names: Record<string, string> = { open: "enter", closed: "exit" };
    const style = globalThis.getComputedStyle;
    vi.stubGlobal(
      "getComputedStyle",
      (element: HTMLElement, pseudo?: string | null) => {
        const real = style(element, pseudo);
        return new Proxy(real, {
          get: (target, key) => {
            if (key === "animationName")
              return names[element.dataset["state"] ?? ""] ?? "none";
            const value: unknown = Reflect.get(target, key, target);
            return typeof value === "function" ? value.bind(target) : value;
          },
        });
      },
    );
    show();
    const dialog = await ask(user);
    await user.click(within(dialog).getByRole("button", { name: "Để sau" }));
    expect(dialog).toHaveAttribute("data-state", "closed");
    expect(dialog).toBeInTheDocument();
    expect(dialog).toHaveAccessibleDescription(
      "Đồng hồ 45 phút chạy ngay khi bạn bấm Bắt đầu và không dừng lại.",
    );
  });

  it("enters fullscreen in the same tick as the click, then opens the paper", async () => {
    const user = userEvent.setup();
    const calls = mockStart();
    const router = show({ integrity: { ...POLICY, requireFullscreen: true } });
    const dialog = await ask(user);
    fireEvent.click(confirm(dialog));
    expect(request).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(calls).toEqual(["start"]);
  });

  it("Start sends no attempt to resume", async () => {
    const user = userEvent.setup();
    const sent = listen();
    const router = show({
      attemptsUsed: 1,
      lastAttemptId: ATTEMPT,
      lastSubmittedAt: "2026-08-29T09:00:00Z",
    });
    await user.click(confirm(await ask(user)));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(sent).toEqual([null]);
  });

  it("does not touch fullscreen when the policy does not ask for it", async () => {
    const user = userEvent.setup();
    mockStart();
    const router = show();
    await user.click(confirm(await ask(user)));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(request).not.toHaveBeenCalled();
  });

  it("starts without fullscreen on a browser that has none", async () => {
    const user = userEvent.setup();
    supported = false;
    mockStart();
    const router = show({ integrity: { ...POLICY, requireFullscreen: true } });
    await user.click(confirm(await ask(user)));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(request).not.toHaveBeenCalled();
  });

  it("starts even when the browser refuses fullscreen", async () => {
    const user = userEvent.setup();
    request.mockRejectedValue(new TypeError("refused"));
    mockStart();
    const router = show({ integrity: { ...POLICY, requireFullscreen: true } });
    await user.click(confirm(await ask(user)));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
  });

  it.each(["Để sau", "Escape"])("starts nothing on %s", async (way) => {
    const user = userEvent.setup();
    const calls = mockStart();
    const router = show({ integrity: { ...POLICY, requireFullscreen: true } });
    const dialog = await ask(user);
    if (way === "Escape") await user.keyboard("{Escape}");
    else await user.click(within(dialog).getByRole("button", { name: way }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(calls).toEqual([]);
    expect(request).not.toHaveBeenCalled();
    expect(router.state.location.pathname).toBe(`/app/assignments/${ASSIGNMENT}`);
    expect(await startButton()).toHaveFocus();
  });

  it("sends one request for two clicks, and cannot be put off meanwhile", async () => {
    const user = userEvent.setup();
    const calls: string[] = [];
    let release = () => {};
    server.use(
      http.post(START, async () => {
        calls.push("start");
        await new Promise<void>((resolve) => {
          release = resolve;
        });
        return contractJson(
          "/app/assignments/{id}/attempts",
          "post",
          200,
          attemptSession(),
        );
      }),
    );
    const router = show({ integrity: { ...POLICY, requireFullscreen: true } });
    const dialog = await ask(user);
    fireEvent.click(confirm(dialog));
    fireEvent.click(confirm(dialog));
    await waitFor(() => expect(calls).toEqual(["start"]));
    expect(request).toHaveBeenCalledTimes(1);
    expect(confirm(dialog)).toHaveAttribute("aria-busy", "true");
    await user.click(within(dialog).getByRole("button", { name: "Để sau" }));
    await user.keyboard("{Escape}");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    release();
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(calls).toEqual(["start"]);
  });

  it("tells a busy Not yet apart", async () => {
    const user = userEvent.setup();
    server.use(
      http.post(START, async () => {
        await delay("infinite");
        return HttpResponse.error();
      }),
    );
    show();
    const dialog = await ask(user);
    await user.click(within(dialog).getByRole("button", { name: "Bắt đầu" }));
    await waitFor(() =>
      expect(within(dialog).getByRole("button", { name: "Để sau" })).toHaveAttribute(
        "aria-disabled",
        "true",
      ),
    );
  });

  it("keeps the busy dialog and asks nothing again while the engine loads", async () => {
    const user = userEvent.setup();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const calls = mockStart();
    let open = () => {};
    const held = new Promise<void>((resolve) => {
      open = resolve;
    });
    serve();
    const router = renderAt(
      `/app/assignments/${ASSIGNMENT}`,
      [
        ...ROUTES,
        {
          path: "/app/attempts/:id",
          lazy: async () => {
            await held;
            return { Component: () => <p>engine</p> };
          },
        },
      ],
      client,
    );
    await user.click(confirm(await ask(user)));
    await waitFor(() => expect(router.state.navigation.state).toBe("loading"));
    await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
    expect(confirm(screen.getByRole("dialog"))).toHaveAttribute("aria-busy", "true");
    expect(asked).toBe(1);
    open();
    expect(await screen.findByText("engine")).toBeInTheDocument();
    expect(calls).toEqual(["start"]);
    expect(asked).toBe(1);
    await waitFor(() =>
      expect(client.getQueryData(["my-assignment", ASSIGNMENT])).toBeUndefined(),
    );
  });

  it("forgets what it read once the attempt starts", async () => {
    const user = userEvent.setup();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    mockStart();
    const router = show({}, client);
    await user.click(confirm(await ask(user)));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    await waitFor(() =>
      expect(client.getQueryData(["my-assignment", ASSIGNMENT])).toBeUndefined(),
    );
  });

  it("closes on a refusal, says why in the page, and reads the paper again", async () => {
    const user = userEvent.setup();
    const calls = mockStart(409);
    const router = show();
    await user.click(confirm(await ask(user)));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bạn đã dùng hết số lượt làm bài.",
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(calls).toEqual(["start"]);
    expect(router.state.location.pathname).toBe(`/app/assignments/${ASSIGNMENT}`);
    await waitFor(() => expect(asked).toBe(2));
  });

  it("says it once when the paper it reads again has no attempt left", async () => {
    const user = userEvent.setup();
    mockStart(409);
    show();
    const dialog = await ask(user);
    serve({ attemptsUsed: 2, maxAttempts: 2 });
    await user.click(confirm(dialog));
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Bắt đầu làm bài" })).toBeNull(),
    );
    expect(screen.getAllByText("Bạn đã dùng hết số lượt làm bài.")).toHaveLength(1);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bạn đã dùng hết số lượt làm bài.",
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("announces the new state and moves focus to the page when the button goes", async () => {
    const user = userEvent.setup();
    mockStart(409);
    serve();
    renderAt(`/app/assignments/${ASSIGNMENT}`, [
      {
        path: "/app/assignments/:id",
        element: (
          <main tabIndex={-1}>
            <AssignmentIntroPage />
          </main>
        ),
      },
    ]);
    const dialog = await ask(user);
    server.use(
      http.get(DETAIL, async () => {
        await delay(300);
        return contractJson(
          "/app/assignments/{id}",
          "get",
          200,
          detail({ attemptsUsed: 2, maxAttempts: 2 }),
        );
      }),
    );
    await user.click(confirm(dialog));
    const refusal = await screen.findByRole("alert");
    await waitFor(async () => expect(await startButton()).toHaveFocus());
    await waitFor(() => expect(screen.queryByRole("button")).toBeNull());
    expect(screen.getByRole("main")).toHaveFocus();
    expect(screen.getByRole("alert")).not.toBe(refusal);
  });

  it("says a refusal in the reader's language, by its code", async () => {
    await i18n.changeLanguage("en");
    const user = userEvent.setup();
    refuse("ATTEMPT_LIMIT_REACHED", "Bạn đã dùng hết số lượt làm bài.");
    show();
    await user.click(await screen.findByRole("button", { name: "Start test" }));
    const dialog = await screen.findByRole("dialog", { name: "Start now?" });
    expect(dialog).toHaveAccessibleDescription(
      "The 45-minute timer starts as soon as you press Start and does not pause.",
    );
    await user.click(within(dialog).getByRole("button", { name: "Start" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "You have used all your attempts.",
    );
  });

  it("says a start refused with a 403 in the reader's language", async () => {
    await i18n.changeLanguage("en");
    const user = userEvent.setup();
    refuse("FORBIDDEN", "Bạn không có quyền làm bài này.", 403);
    show();
    await user.click(await screen.findByRole("button", { name: "Start test" }));
    const dialog = await screen.findByRole("dialog", { name: "Start now?" });
    await user.click(within(dialog).getByRole("button", { name: "Start" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This test was not found, or it was not assigned to you.",
    );
  });

  it("says a paper that closed meanwhile is not open", async () => {
    const user = userEvent.setup();
    refuse("ASSIGNMENT_NOT_OPEN", "assignment is not open");
    show();
    await user.click(confirm(await ask(user)));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bài này hiện không mở.",
    );
  });

  it("passes on a reason it has no words for", async () => {
    const user = userEvent.setup();
    refuse("ATTEMPT_VOIDED", "Bài này không còn dùng được.");
    show();
    await user.click(confirm(await ask(user)));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bài này không còn dùng được.",
    );
  });

  it("says it could not start when the network fails, and can be tried again", async () => {
    const user = userEvent.setup();
    server.use(http.post(START, () => HttpResponse.error()));
    const router = show();
    await user.click(confirm(await ask(user)));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Không bắt đầu được. Hãy thử lại.");
    expect(alert).toHaveClass("bg-bg");
    expect(await startButton()).toHaveAccessibleDescription(
      "Không bắt đầu được. Hãy thử lại.",
    );
    mockStart();
    await user.click(confirm(await ask(user)));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
  });
});

describe("Continue test", () => {
  function live(over: Record<string, unknown> = {}) {
    return show({
      hasLiveAttempt: true,
      lastAttemptId: ATTEMPT,
      liveDeadlineAt: "2026-08-29T10:38:12Z",
      integrity: { ...POLICY, requireFullscreen: true },
      ...over,
    });
  }

  it("enters fullscreen in the click and resumes, with no question", async () => {
    const calls = mockStart();
    const router = live();
    fireEvent.click(await screen.findByRole("button", { name: "Tiếp tục làm bài" }));
    expect(request).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).toBeNull();
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(calls).toEqual(["start"]);
  });

  it("Continue names the live attempt", async () => {
    const sent = listen();
    const router = live();
    fireEvent.click(await screen.findByRole("button", { name: "Tiếp tục làm bài" }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(sent).toEqual([{ resume: ATTEMPT }]);
  });

  it("saves what a closed tab left before it continues", async () => {
    const key = `quizzivy.answer-draft.${ATTEMPT}`;
    const draft = {
      sessionId: "018f0000-0000-7000-8000-0000000000b7",
      answers: {
        "018f0000-0000-7000-8000-000000000701": {
          type: "text",
          value: "typed offline",
        },
      },
    };
    localStorage.setItem(
      key,
      JSON.stringify({
        studentId: STUDENT.id,
        sessionId: draft.sessionId,
        deadlineAt: Date.parse("2026-08-29T10:45:00Z"),
        answers: draft.answers,
      }),
    );
    onTestFinished(() => localStorage.removeItem(key));
    const calls: { call: string; body: unknown }[] = [];
    server.use(
      http.patch(`${BASE}/app/attempts/${ATTEMPT}/answers`, async ({ request }) => {
        calls.push({ call: "save", body: await request.json() });
        return contractJson("/app/attempts/{id}/answers", "patch", 200, {
          serverTime: "2026-08-29T10:00:00Z",
          savedAt: "2026-08-29T10:00:00Z",
          deadlineAt: "2026-08-29T10:45:00Z",
        });
      }),
      http.post(START, async ({ request }) => {
        calls.push({ call: "resume", body: await request.json() });
        return contractJson(
          "/app/assignments/{id}/attempts",
          "post",
          200,
          attemptSession(),
        );
      }),
    );
    const router = live();
    fireEvent.click(await screen.findByRole("button", { name: "Tiếp tục làm bài" }));
    expect(request).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(calls).toEqual([
      { call: "save", body: draft },
      { call: "resume", body: { resume: ATTEMPT } },
    ]);
    expect(localStorage.getItem(key)).toBeNull();
  });

  it("an ended attempt turns Continue into Start", async () => {
    const user = userEvent.setup();
    const sent = listen(409);
    const router = live();
    const resume = await screen.findByRole("button", { name: "Tiếp tục làm bài" });
    serve({
      attemptsUsed: 1,
      lastAttemptId: ATTEMPT,
      lastSubmittedAt: "2026-08-29T09:59:00Z",
    });
    await user.click(resume);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bài làm này đã kết thúc.",
    );
    expect(await startButton()).toHaveAccessibleDescription("Bài làm này đã kết thúc.");
    expect(screen.queryByRole("button", { name: "Tiếp tục làm bài" })).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect((await facts())[2]).toEqual(["Lượt làm", "1/2"]);
    expect(router.state.location.pathname).toBe(`/app/assignments/${ASSIGNMENT}`);
    expect(sent).toEqual([{ resume: ATTEMPT }]);
  });

  it("says there is no attempt left when the ended one was the last", async () => {
    const user = userEvent.setup();
    listen(409);
    live({ maxAttempts: 1 });
    const resume = await screen.findByRole("button", { name: "Tiếp tục làm bài" });
    serve({
      attemptsUsed: 1,
      maxAttempts: 1,
      lastAttemptId: ATTEMPT,
      lastSubmittedAt: "2026-08-29T09:59:00Z",
    });
    await user.click(resume);
    await waitFor(() => expect(screen.queryByRole("button")).toBeNull());
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bạn đã dùng hết số lượt làm bài.",
    );
  });

  it("has Home read its lists again once the attempt has ended", async () => {
    const user = userEvent.setup();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(["my-assignments"], {
      dueNow: [],
      upcoming: [],
      completed: [],
    });
    listen(409);
    show({ hasLiveAttempt: true, lastAttemptId: ATTEMPT }, client);
    await user.click(await screen.findByRole("button", { name: "Tiếp tục làm bài" }));
    await screen.findByRole("alert");
    expect(client.getQueryState(["my-assignments"])?.isInvalidated).toBe(true);
  });

  it("says when the attempt ends", async () => {
    live();
    expect(await screen.findByText("Lượt này kết thúc lúc 17:38.")).toBeInTheDocument();
  });

  it("names the day when the attempt ends on another one", async () => {
    live({ liveDeadlineAt: "2026-08-29T17:30:00Z" });
    expect(
      await screen.findByText("Lượt này kết thúc lúc 00:30, 30/08."),
    ).toBeInTheDocument();
  });

  it("falls back to the autosave line when the server gives no deadline", async () => {
    live({ liveDeadlineAt: null });
    await screen.findByRole("button", { name: "Tiếp tục làm bài" });
    expect(screen.getByText("Câu trả lời được lưu tự động.")).toBeInTheDocument();
  });

  it("sends one request for two clicks", async () => {
    const calls = mockStart();
    const router = live();
    const button = await screen.findByRole("button", { name: "Tiếp tục làm bài" });
    fireEvent.click(button);
    fireEvent.click(button);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(calls).toEqual(["start"]);
    expect(request).toHaveBeenCalledTimes(1);
  });

  it("takes the last refusal down while it tries again", async () => {
    const user = userEvent.setup();
    server.use(http.post(START, () => HttpResponse.error()));
    show({ hasLiveAttempt: true, lastAttemptId: ATTEMPT });
    await user.click(await screen.findByRole("button", { name: "Tiếp tục làm bài" }));
    await screen.findByRole("alert");
    server.use(
      http.post(START, async () => {
        await delay("infinite");
        return HttpResponse.error();
      }),
    );
    await user.click(await screen.findByRole("button", { name: "Tiếp tục làm bài" }));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });

  it("marks Continue busy while the request is out", async () => {
    const user = userEvent.setup();
    server.use(
      http.post(START, async () => {
        await delay("infinite");
        return HttpResponse.error();
      }),
    );
    show({ hasLiveAttempt: true, lastAttemptId: ATTEMPT });
    const button = await screen.findByRole("button", { name: "Tiếp tục làm bài" });
    await user.click(button);
    await waitFor(() => expect(button).toHaveAttribute("aria-busy", "true"));
  });

  it("says why it could not resume, and reads the paper again", async () => {
    const user = userEvent.setup();
    refuse("ASSIGNMENT_NOT_OPEN", "assignment is not open");
    live();
    await user.click(await screen.findByRole("button", { name: "Tiếp tục làm bài" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bài này hiện không mở.",
    );
    await waitFor(() => expect(asked).toBe(2));
  });
});

describe("what the page trusts", () => {
  it("reads the paper again although a fresh copy is in the cache", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
    });
    client.setQueryData(["my-assignment", ASSIGNMENT], detail());
    show({ attemptsUsed: 2, maxAttempts: 2 }, client);
    expect(
      await screen.findByText("Bạn đã dùng hết số lượt làm bài."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Bắt đầu làm bài" })).toBeNull();
    expect(asked).toBe(1);
  });
});

describe("the open paper's hint", () => {
  it("says answers save by themselves, under the button", async () => {
    show();
    await startButton();
    expect(screen.getByText("Câu trả lời được lưu tự động.")).toBeInTheDocument();
  });
});

describe("a tab left open", () => {
  it("repaints when the day turns, with nothing new from the server", async () => {
    vi.setSystemTime(new Date("2026-08-29T16:59:30Z"));
    show({ closesAt: "2026-08-30T14:00:00Z" });
    expect((await rules())[0]).toBe("Bài mở đến 21:00, CN, 30/08.");
    await act(() => vi.advanceTimersByTimeAsync(31_000));
    expect((await rules())[0]).toBe("Bài mở đến 21:00 hôm nay.");
    expect(asked).toBe(1);
  });

  it("repaints the opening label when the day turns", async () => {
    vi.setSystemTime(new Date("2026-08-29T16:59:30Z"));
    show({
      status: "scheduled",
      opensAt: "2026-08-30T01:00:00Z",
      closesAt: "2026-09-20T14:00:00Z",
    });
    const button = await screen.findByRole("button", { name: "Mở chủ nhật" });
    await act(() => vi.advanceTimersByTimeAsync(31_000));
    expect(button).toHaveAccessibleName("Mở lúc 08:00 hôm nay");
    expect(asked).toBe(1);
  });
});

describe("in English", () => {
  it("reads the teacher's note and the after-close score sentence in English", async () => {
    await i18n.changeLanguage("en");
    show({
      studentNote: "Bring a calculator.",
      teacherName: "Hoàng Thương",
      review: {
        showScore: true,
        showCorrectAnswers: false,
        showExplanations: false,
        release: "after_close",
        showClassAverage: false,
      },
    });
    const note = (
      await screen.findByRole("heading", { level: 2, name: "Note from Hoàng Thương" })
    ).closest("section")!;
    expect(note).toHaveTextContent("Bring a calculator.");
    expect(
      screen.getByText("You will see your score after the test closes."),
    ).toBeInTheDocument();
  });

  it("reads the short window in English, singular and plural", async () => {
    await i18n.changeLanguage("en");
    const user = userEvent.setup();
    vi.setSystemTime(new Date("2026-08-29T13:39:30Z"));
    show();
    await user.click(await screen.findByRole("button", { name: "Start test" }));
    const dialog = await screen.findByRole("dialog", { name: "Start now?" });
    expect(dialog).toHaveAccessibleDescription(
      "The test closes at 21:00, so you have 20 minutes. The timer starts as soon as you press Start and does not pause.",
    );
    expect(
      within(dialog)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Not yet", "Start"]);
  });

  it("reads one minute in English", async () => {
    await i18n.changeLanguage("en");
    const user = userEvent.setup();
    vi.setSystemTime(new Date("2026-08-29T13:58:30Z"));
    show();
    await user.click(await screen.findByRole("button", { name: "Start test" }));
    expect(
      await screen.findByRole("dialog", { name: "Start now?" }),
    ).toHaveAccessibleDescription(
      "The test closes at 21:00, so you have 1 minute. The timer starts as soon as you press Start and does not pause.",
    );
  });

  it("reads the deck's words for a test that has not opened, in English", async () => {
    await i18n.changeLanguage("en");
    show({
      status: "scheduled",
      opensAt: "2026-08-31T01:00:00Z",
      closesAt: "2026-09-20T14:00:00Z",
    });
    expect(await screen.findByRole("button")).toHaveAccessibleName("Opens Monday");
    expect(screen.getByText("You can start once it opens.")).toBeInTheDocument();
  });

  it("reads today's opening in English", async () => {
    await i18n.changeLanguage("en");
    show({
      status: "scheduled",
      opensAt: "2026-08-29T12:00:00Z",
      closesAt: "2026-09-20T14:00:00Z",
    });
    expect(await screen.findByRole("button")).toHaveAccessibleName(
      "Opens today at 19:00",
    );
  });

  it("names the date in English from a week away", async () => {
    await i18n.changeLanguage("en");
    show({
      status: "scheduled",
      opensAt: "2026-09-06T01:00:00Z",
      closesAt: "2026-09-20T14:00:00Z",
    });
    expect(await screen.findByRole("button")).toHaveAccessibleName("Opens Sun 6 Sep");
  });

  it("reads the open test's line in English", async () => {
    await i18n.changeLanguage("en");
    show();
    await screen.findByRole("button", { name: "Start test" });
    expect(screen.getByText("Your answers save automatically.")).toBeInTheDocument();
  });

  it("reads Continue's line in English", async () => {
    await i18n.changeLanguage("en");
    show({
      hasLiveAttempt: true,
      lastAttemptId: ATTEMPT,
      liveDeadlineAt: "2026-08-29T10:38:12Z",
    });
    await screen.findByRole("button", { name: "Continue test" });
    expect(screen.getByText("This attempt ends at 17:38.")).toBeInTheDocument();
  });

  it("reads a closed test in English", async () => {
    await i18n.changeLanguage("en");
    show({ status: "closed" });
    expect(await screen.findByText("This test is closed.")).toBeInTheDocument();
  });

  it("writes the dates in the reader's language", async () => {
    await i18n.changeLanguage("en");
    show({ closesAt: "2026-09-03T14:00:00Z" });
    const heading = await screen.findByRole("heading", { name: "Before you start" });
    expect(
      within(heading.parentElement!).getAllByRole("listitem")[0],
    ).toHaveTextContent("Available until Thu 3 Sep, 21:00.");
  });
});
