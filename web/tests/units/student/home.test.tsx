import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { delay, http, HttpResponse } from "msw";
import { focusManager } from "@tanstack/react-query";
import StudentHomePage from "@/features/assignments/pages/StudentHomePage";
import i18n from "@/lib/i18n";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import { viewport } from "@tests/support/viewport";
import { Toaster, toast } from "@/components/ui/sonner";
import { ASSIGNMENT, ATTEMPT, BASE, card, mockStart, renderAt } from "./support";

const flags = vi.hoisted(() => ({
  notifications: false,
  messages: false,
  schedule: false,
  grades: false,
  learn: false,
}));
vi.mock("@/app/modules", () => ({ modules: flags }));

const SAMPLE_CLASS = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "IELTS Foundation",
  description: null,
  teacherName: "Cô Thương",
  joinedAt: "2026-06-01T00:00:00Z",
};

type Lists = { dueNow?: unknown[]; upcoming?: unknown[]; completed?: unknown[] };

let asked = 0;

function id(n: number) {
  return `018f0000-0000-7000-8000-${String(n).padStart(12, "0")}`;
}

function serve(lists: Lists, classes: unknown[] = [SAMPLE_CLASS]) {
  server.use(
    http.get(`${BASE}/app/assignments`, () => {
      asked += 1;
      return contractJson("/app/assignments", "get", 200, {
        dueNow: lists.dueNow ?? [],
        upcoming: lists.upcoming ?? [],
        completed: lists.completed ?? [],
      });
    }),
    http.get(`${BASE}/app/classes`, () =>
      contractJson("/app/classes", "get", 200, { items: classes }),
    ),
  );
}

function show() {
  return renderAt("/app", [
    {
      path: "/app",
      element: (
        <>
          <StudentHomePage />
          <Toaster />
        </>
      ),
    },
    { path: "/app/assignments/:id", element: <p>intro page</p> },
    { path: "/app/attempts/:id/result", element: <p>result page</p> },
  ]);
}

function home(lists: Lists, classes?: unknown[]) {
  serve(lists, classes);
  return show();
}

function live(over: Record<string, unknown> = {}) {
  return card({
    hasLiveAttempt: true,
    lastAttemptId: ATTEMPT,
    liveDeadlineAt: "2026-08-29T10:38:12Z",
    ...over,
  });
}

function done(n: number, over: Record<string, unknown> = {}) {
  return card({
    id: id(n),
    testTitle: `Paper ${n}`,
    status: "closed",
    attemptsUsed: 1,
    maxAttempts: 1,
    lastAttemptId: id(900 + n),
    lastSubmittedAt: "2026-08-26T09:30:00Z",
    ...over,
  });
}

function scheduled(over: Record<string, unknown> = {}) {
  return card({
    id: id(2),
    testTitle: "Listening practice 03",
    status: "scheduled",
    opensAt: "2026-09-01T01:00:00Z",
    closesAt: "2026-09-08T01:00:00Z",
    ...over,
  });
}

function row(title: string) {
  return screen.getByRole("link", { name: new RegExp(title) });
}

function titles(pattern: RegExp) {
  return screen
    .getAllByRole("link")
    .map((link) => pattern.exec(link.textContent ?? "")?.[0]);
}

beforeEach(() => {
  asked = 0;
  viewport("phone");
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z"));
});
afterEach(async () => {
  act(() => {
    toast.dismiss();
  });
  flags.grades = false;
  vi.useRealTimers();
  vi.unstubAllGlobals();
  focusManager.setFocused();
  useAuthStore.getState().clearSession();
  await i18n.changeLanguage("vi");
});

describe("the greeting and the line under it", () => {
  it("greets by given name for the time of day in the app's zone", async () => {
    home({ dueNow: [card()] });
    expect(
      await screen.findByRole("heading", { level: 1, name: "Chào buổi chiều, An" }),
    ).toBeInTheDocument();
  });

  it("says when the attempt in progress closes", async () => {
    home({ dueNow: [live({ liveDeadlineAt: "2026-08-29T10:22:14Z" })] });
    expect(
      await screen.findByText("Bạn đang làm dở một bài. Bài đóng lúc 17:22."),
    ).toBeInTheDocument();
  });

  it("counts what closes today when nothing is in progress", async () => {
    home({ dueNow: [card()] });
    expect(
      await screen.findByText("Bạn có 1 bài đến hạn hôm nay."),
    ).toBeInTheDocument();
  });

  it.each([
    ["2026-09-01T01:00:00Z", "Listening practice 03, thứ ba."],
    ["2026-08-29T12:00:00Z", "Listening practice 03, mở lúc 19:00 hôm nay."],
    ["2026-08-29T17:00:00Z", "Listening practice 03, chủ nhật."],
    ["2026-09-04T16:59:00Z", "Listening practice 03, thứ sáu."],
    ["2026-09-04T17:00:00Z", "Listening practice 03, 05/09."],
  ])("names the next paper, opening at %s", async (opensAt, tail) => {
    home({ upcoming: [scheduled({ opensAt })] });
    expect(
      await screen.findByText(`Hôm nay không có bài đến hạn. Tiếp theo: ${tail}`),
    ).toBeInTheDocument();
  });

  it("counts a paper that opens and closes later today", async () => {
    home({
      upcoming: [
        scheduled({
          opensAt: "2026-08-29T12:00:00Z",
          closesAt: "2026-08-29T14:00:00Z",
        }),
      ],
    });
    expect(
      await screen.findByText("Bạn có 1 bài đến hạn hôm nay."),
    ).toBeInTheDocument();
  });

  it("says nothing is due when only results are left", async () => {
    home({ completed: [done(1)] });
    expect(
      await screen.findByText("Hôm nay không có bài đến hạn."),
    ).toBeInTheDocument();
  });
});

describe("the resume card", () => {
  it("says how long is left, how much is answered and when it closes", async () => {
    home({
      dueNow: [
        live({ className: "IELTS Foundation", questionCount: 8, liveAnsweredCount: 3 }),
      ],
    });
    expect(await screen.findByText("Đang làm · còn 38 phút")).toHaveClass(
      "bg-info-soft",
    );
    expect(
      screen.getByRole("heading", { level: 2, name: "Unit 5 — Present perfect" }),
    ).toBeInTheDocument();
    const meta = screen.getByText(
      "IELTS Foundation · 3/8 câu đã trả lời · đóng 17:38 hôm nay",
    );
    expect(meta.nextElementSibling?.firstElementChild).toHaveStyle({ width: "38%" });
  });

  it("resumes straight into the paper", async () => {
    const user = userEvent.setup();
    const calls = mockStart();
    const router = home({ dueNow: [live()] });
    await user.click(await screen.findByRole("button", { name: "Tiếp tục làm bài" }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(calls).toEqual(["start"]);
  });

  it("says why it could not resume, and the reason outlives the card", async () => {
    const user = userEvent.setup();
    mockStart(409);
    home({ dueNow: [live()] });
    const resume = await screen.findByRole("button", { name: "Tiếp tục làm bài" });
    serve({ completed: [done(1, { id: card().id, lastAttemptId: ATTEMPT })] });
    await user.click(resume);
    await waitFor(() => expect(asked).toBe(2));
    await screen.findByRole("heading", { level: 2, name: "Kết quả gần đây" });
    expect(screen.queryByRole("button", { name: "Tiếp tục làm bài" })).toBeNull();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bạn đã dùng hết số lượt làm bài.",
    );
  });

  it("lets the student try again when the server could not be reached", async () => {
    const user = userEvent.setup();
    server.use(
      http.post(`${BASE}/app/assignments/${ASSIGNMENT}/attempts`, () =>
        HttpResponse.error(),
      ),
    );
    home({ dueNow: [live()] });
    const resume = await screen.findByRole("button", { name: "Tiếp tục làm bài" });
    await user.click(resume);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không bắt đầu được. Hãy thử lại.",
    );
    await waitFor(() => expect(asked).toBe(2));
    expect(resume).toBeEnabled();
  });

  it("holds the button while the resume is on its way", async () => {
    const user = userEvent.setup();
    let started = 0;
    server.use(
      http.post(`${BASE}/app/assignments/${ASSIGNMENT}/attempts`, async () => {
        started += 1;
        await delay("infinite");
        return new Response(null, { status: 500 });
      }),
    );
    home({ dueNow: [live()] });
    const resume = await screen.findByRole("button", { name: "Tiếp tục làm bài" });
    await user.click(resume);
    await waitFor(() => expect(started).toBe(1));
    expect(resume).toBeDisabled();
  });

  it("puts the attempt in progress above what is coming up", async () => {
    home({
      dueNow: [
        live({ testTitle: "Taking" }),
        card({ id: id(2), testTitle: "Waiting" }),
      ],
      completed: [done(3)],
    });
    await screen.findByRole("heading", { level: 2, name: "Taking" });
    expect(
      screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent),
    ).toEqual(["Taking", "Sắp tới", "Kết quả gần đây"]);
  });

  it("drops a minute when the time left crosses it, not when the clock's minute turns", async () => {
    home({ dueNow: [live({ liveDeadlineAt: "2026-08-29T10:22:14Z" })] });
    expect(await screen.findByText("Đang làm · còn 22 phút")).toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(20_000));
    expect(screen.getByText("Đang làm · còn 21 phút")).toBeInTheDocument();
  });

  it("says no time on the card, and the window's close, when no deadline came", async () => {
    home({ dueNow: [live({ liveDeadlineAt: null })] });
    expect(await screen.findByText("Đang làm")).toBeInTheDocument();
    expect(screen.getByText(/ · đóng 21:00 hôm nay$/)).toBeInTheDocument();
  });

  it("rounds the bar to the nearest percent", async () => {
    home({ dueNow: [live({ questionCount: 3, liveAnsweredCount: 1 })] });
    const meta = await screen.findByText(/^1\/3 câu đã trả lời · /);
    expect(meta.nextElementSibling?.firstElementChild).toHaveStyle({ width: "33%" });
  });

  it("reads nothing answered when the count did not come", async () => {
    home({ dueNow: [live()] });
    const meta = await screen.findByText(/^0\/24 câu đã trả lời · /);
    expect(meta.nextElementSibling?.firstElementChild).toHaveStyle({ width: "0%" });
  });

  it("starts its line with the answers for a student targeted by name", async () => {
    home({ dueNow: [live({ className: null, liveAnsweredCount: 2 })] });
    expect(
      await screen.findByText("2/24 câu đã trả lời · đóng 17:38 hôm nay"),
    ).toBeInTheDocument();
  });

  it("dates a deadline that falls on another day", async () => {
    home({ dueNow: [live({ liveDeadlineAt: "2026-08-29T17:35:00Z" })] });
    expect(await screen.findByText(/ · đóng 00:35, 30\/08$/)).toBeInTheDocument();
    expect(
      screen.getByText("Bạn đang làm dở một bài. Bài đóng lúc 00:35, 30/08."),
    ).toBeInTheDocument();
  });

  it("counts the minutes down without asking the server again", async () => {
    home({ dueNow: [live({ liveDeadlineAt: "2026-08-29T10:22:14Z" })] });
    expect(await screen.findByText("Đang làm · còn 22 phút")).toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(65_000));
    expect(screen.getByText("Đang làm · còn 21 phút")).toBeInTheDocument();
    expect(asked).toBe(1);
  });

  it("never reads less than a minute, and no time at all without a deadline", async () => {
    home({
      dueNow: [
        live({ liveDeadlineAt: "2026-08-29T09:59:00Z" }),
        live({ id: id(2), testTitle: "No deadline", liveDeadlineAt: null }),
      ],
    });
    expect(await screen.findByText("Đang làm · còn 1 phút")).toBeInTheDocument();
    expect(within(row("No deadline")).getByText("Đang làm")).toBeInTheDocument();
  });

  it("is the attempt closing first; another one in progress is a row", async () => {
    home({
      dueNow: [
        live({ testTitle: "Later", liveDeadlineAt: "2026-08-29T10:45:00Z" }),
        live({
          id: id(2),
          testTitle: "Sooner",
          liveDeadlineAt: "2026-08-29T10:15:00Z",
        }),
        card({ id: id(3), testTitle: "Not started" }),
      ],
    });
    expect(
      await screen.findByRole("heading", { level: 2, name: "Sooner" }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Tiếp tục làm bài" })).toHaveLength(1);
    expect(row("Later")).toHaveAttribute("href", `/app/assignments/${card().id}`);
    expect(within(row("Later")).getByText("Đang làm")).toHaveClass("bg-info-soft");
    expect(row("Later").querySelector("[aria-hidden='true']")).toHaveClass("bg-muted");
    expect(screen.getByText("2 bài")).toBeInTheDocument();
  });
});

describe("a tab left open", () => {
  it("moves a paper to due tomorrow when its last 24 hours begin", async () => {
    home({ dueNow: [card({ closesAt: "2026-08-30T10:00:45Z" })] });
    const link = within(await screen.findByRole("link", { name: /Unit 5/ }));
    expect(link.getByText("Đang mở")).toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(61_000));
    expect(link.getByText("Hạn ngày mai")).toBeInTheDocument();
    expect(asked).toBe(1);
  });

  it("greets for the evening once it is evening", async () => {
    vi.setSystemTime(new Date("2026-08-29T10:59:30Z"));
    home({ completed: [done(1)] });
    const heading = await screen.findByRole("heading", { level: 1 });
    expect(heading).toHaveTextContent("Chào buổi chiều, An");
    await act(() => vi.advanceTimersByTimeAsync(61_000));
    expect(heading).toHaveTextContent("Chào buổi tối, An");
  });
});

describe("coming up", () => {
  it("counts its rows", async () => {
    home({
      dueNow: [card(), card({ id: id(2), testTitle: "Second" })],
      upcoming: [scheduled({ id: id(3) })],
    });
    expect(
      await screen.findByRole("heading", { level: 2, name: "Sắp tới" }),
    ).toBeInTheDocument();
    expect(screen.getByText("3 bài")).toBeInTheDocument();
  });

  it("draws the tile, the title and the class with the length and the questions", async () => {
    home({ dueNow: [card({ className: "IELTS Foundation" })] });
    const link = within(await screen.findByRole("link", { name: /Unit 5/ }));
    expect(link.getByText("T7")).toBeInTheDocument();
    expect(link.getByText("29")).toBeInTheDocument();
    expect(link.getByText("Thứ bảy, 29/08")).toHaveClass("sr-only");
    expect(link.getByText("IELTS Foundation · 45 phút · 24 câu")).toBeInTheDocument();
  });

  it("leaves no stray separator for a student targeted by name", async () => {
    home({ dueNow: [card({ className: null })] });
    const link = within(await screen.findByRole("link", { name: /Unit 5/ }));
    expect(link.getByText("45 phút · 24 câu")).toBeInTheDocument();
  });

  it("opens the intro, where the rules are read before the clock starts", async () => {
    const user = userEvent.setup();
    const router = home({ dueNow: [card()] });
    await user.click(await screen.findByRole("link", { name: /Unit 5/ }));
    expect(router.state.location.pathname).toBe(`/app/assignments/${card().id}`);
  });

  it.each([
    ["2026-08-29T14:00:00Z", "Hạn hôm nay", "warning", "bg-warning-soft"],
    ["2026-08-30T09:59:00Z", "Hạn ngày mai", "warning", "bg-warning-soft"],
    ["2026-08-30T10:01:00Z", "Đang mở", "success", "bg-muted"],
  ])("reads a paper closing at %s as %s", async (closesAt, pill, tone, tile) => {
    home({ dueNow: [card({ closesAt })] });
    const link = await screen.findByRole("link", { name: /Unit 5/ });
    expect(within(link).getByText(pill)).toHaveClass(
      `in-data-[scale=deck]:bg-${tone}-soft`,
    );
    expect(link.querySelector("[aria-hidden='true']")).toHaveClass(tile);
  });

  it.each([
    ["2026-09-01T01:00:00Z", "Mở T3", "T3", "1"],
    ["2026-08-29T12:00:00Z", "Mở 19:00", "T7", "29"],
    ["2026-08-29T17:00:00Z", "Mở CN", "CN", "30"],
    ["2026-09-04T16:59:00Z", "Mở T6", "T6", "4"],
    ["2026-09-04T17:00:00Z", "Mở 05/09", "T7", "5"],
  ])("says a paper opening at %s %s", async (opensAt, pill, weekday, day) => {
    home({ upcoming: [scheduled({ opensAt })] });
    const link = within(await screen.findByRole("link", { name: /Listening/ }));
    expect(link.getByText(pill)).toHaveClass("bg-muted", "text-muted-fg");
    expect(link.getByText(weekday)).toBeInTheDocument();
    expect(link.getByText(day)).toBeInTheDocument();
  });

  it("lists rows by the date on their tiles", async () => {
    home({
      upcoming: [scheduled({ id: id(3), testTitle: "Opens Tuesday" })],
      dueNow: [
        card({
          id: id(2),
          testTitle: "Closes Monday",
          closesAt: "2026-08-31T15:00:00Z",
        }),
        card({
          id: id(1),
          testTitle: "Closes Sunday",
          closesAt: "2026-08-30T01:00:00Z",
        }),
      ],
    });
    await screen.findByRole("link", { name: /Closes Sunday/ });
    expect(titles(/(Closes|Opens) [A-Z][a-z]+day/)).toEqual([
      "Closes Sunday",
      "Closes Monday",
      "Opens Tuesday",
    ]);
  });

  it("is not drawn with nothing to do", async () => {
    home({ completed: [done(1)] });
    await screen.findByText("Paper 1");
    expect(screen.queryByRole("heading", { name: "Sắp tới" })).toBeNull();
  });
});

describe("recent results", () => {
  const FOUR = [
    done(1, { lastSubmittedAt: "2026-08-25T03:00:00Z" }),
    done(2, {
      lastSubmittedAt: "2026-08-28T03:00:00Z",
      score: { earned: 27, total: 30, pendingManual: 0 },
    }),
    done(3, { lastSubmittedAt: "2026-08-26T03:00:00Z" }),
    done(4, { lastSubmittedAt: "2026-08-27T03:00:00Z" }),
  ];

  it("lists every result, newest first, while no other screen links to one", async () => {
    home({ completed: FOUR });
    expect(
      await screen.findByRole("heading", { level: 2, name: "Kết quả gần đây" }),
    ).toBeInTheDocument();
    expect(titles(/Paper \d/)).toEqual(["Paper 2", "Paper 4", "Paper 3", "Paper 1"]);
    expect(within(row("Paper 2")).getByText("27 / 30")).toBeInTheDocument();
  });

  it("shows the three submitted last once Grades lists them all", async () => {
    flags.grades = true;
    home({ completed: FOUR });
    await screen.findByRole("heading", { level: 2, name: "Kết quả gần đây" });
    expect(titles(/Paper \d/)).toEqual(["Paper 2", "Paper 4", "Paper 3"]);
  });

  it("keeps the decimals a score has", async () => {
    home({
      completed: [done(1, { score: { earned: 7.5, total: 10, pendingManual: 0 } })],
    });
    const link = within(await screen.findByRole("link", { name: /Paper 1/ }));
    expect(link.getByText("7,5 / 10")).toBeInTheDocument();
  });

  it("says a paper is being graded and keeps the part score to itself", async () => {
    home({
      completed: [done(1, { score: { earned: 20, total: 30, pendingManual: 2 } })],
    });
    const link = within(await screen.findByRole("link", { name: /Paper 1/ }));
    expect(link.getByText("Chờ chấm")).toBeInTheDocument();
    expect(link.queryByText(/20/)).toBeNull();
  });

  it("says only that a paper was submitted when its score is not shown", async () => {
    home({ completed: [done(1)] });
    const link = within(await screen.findByRole("link", { name: /Paper 1/ }));
    expect(link.getByText("Đã nộp")).toBeInTheDocument();
  });

  it("dates a row by when it was handed in, and not before the server closes it", async () => {
    home({ completed: [done(1), done(2, { lastSubmittedAt: null })] });
    const first = within(await screen.findByRole("link", { name: /Paper 1/ }));
    expect(first.getByText("Thứ 4, 26/08")).toBeInTheDocument();
    expect(row("Paper 2")).toHaveTextContent(/^Paper 2Đã nộp$/);
  });

  it("opens the result", async () => {
    const user = userEvent.setup();
    const router = home({ completed: [done(1)] });
    await user.click(await screen.findByRole("link", { name: /Paper 1/ }));
    expect(router.state.location.pathname).toBe(`/app/attempts/${id(901)}/result`);
  });

  it("reads just now for a minute, then the date, without asking the server", async () => {
    home({ completed: [done(1, { lastSubmittedAt: "2026-08-29T09:59:30Z" })] });
    const link = within(await screen.findByRole("link", { name: /Paper 1/ }));
    expect(link.getByText("Vừa xong")).toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(31_000));
    expect(link.getByText("Thứ 7, 29/08")).toBeInTheDocument();
    expect(asked).toBe(1);
  });

  it("shows a result while a retake remains, and the paper still to do", async () => {
    home({
      dueNow: [
        card({
          closesAt: "2026-09-05T00:00:00Z",
          attemptsUsed: 1,
          lastAttemptId: id(901),
          lastSubmittedAt: "2026-08-29T09:00:00Z",
          score: { earned: 8, total: 10, pendingManual: 0 },
        }),
      ],
    });
    const links = await screen.findAllByRole("link", { name: /Unit 5/ });
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      `/app/assignments/${card().id}`,
      `/app/attempts/${id(901)}/result`,
    ]);
    expect(within(links[0]!).getByText("Đang mở")).toBeInTheDocument();
    expect(within(links[1]!).getByText("8 / 10")).toBeInTheDocument();
  });

  it("is not drawn with no result, and nothing is drawn for practice", async () => {
    home({ dueNow: [card()] });
    await screen.findByRole("link", { name: /Unit 5/ });
    expect(screen.queryByRole("heading", { name: "Kết quả gần đây" })).toBeNull();
    expect(screen.queryByRole("heading", { name: /Luyện tập/ })).toBeNull();
  });
});

describe("the states the deck does not draw", () => {
  it("greets and shows a skeleton while the lists load", async () => {
    server.use(http.get(`${BASE}/app/assignments`, () => delay("infinite")));
    show();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Chào buổi chiều, An" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status", { name: "Đang tải…" })).toBeInTheDocument();
  });

  it("says the lists did not load and loads them on retry", async () => {
    const user = userEvent.setup();
    server.use(
      http.get(`${BASE}/app/assignments`, () => new Response(null, { status: 500 })),
    );
    show();
    expect(await screen.findByText("Không tải được. Hãy thử lại.")).toBeInTheDocument();
    serve({ dueNow: [card()] });
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("link", { name: /Unit 5/ })).toBeInTheDocument();
  });

  it("says nothing is assigned to a student who has a class, and offers no code", async () => {
    home({});
    expect(
      await screen.findByText("Hiện chưa có bài nào được giao."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Tham gia lớp" })).toBeNull();
    expect(screen.queryByText(/Hôm nay không có bài/)).toBeNull();
  });

  it("opens the Join dialog for a student who is in no class", async () => {
    const user = userEvent.setup();
    home({}, []);
    expect(await screen.findByText("Bạn chưa tham gia lớp nào.")).toBeInTheDocument();
    expect(screen.queryByText("Hiện chưa có bài nào được giao.")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Tham gia lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Tham gia lớp" });
    expect(within(dialog).getByLabelText("Mã lớp")).toBeInTheDocument();
  });

  it("waits for the classes before choosing between the two", async () => {
    serve({});
    server.use(http.get(`${BASE}/app/classes`, () => delay("infinite")));
    show();
    await waitFor(() => expect(asked).toBe(1));
    expect(
      await screen.findByRole("status", { name: "Đang tải…" }),
    ).toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(screen.queryByText("Hiện chưa có bài nào được giao.")).toBeNull();
    expect(screen.queryByText("Bạn chưa tham gia lớp nào.")).toBeNull();
  });

  it("says nothing is assigned when the classes could not be read", async () => {
    serve({});
    server.use(
      http.get(`${BASE}/app/classes`, () => new Response(null, { status: 500 })),
    );
    show();
    expect(
      await screen.findByText("Hiện chưa có bài nào được giao."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Tham gia lớp" })).toBeNull();
  });

  it("offers no class code to a student with a paper, whatever the classes say", async () => {
    home({ dueNow: [live()] }, []);
    expect(
      await screen.findByRole("button", { name: "Tiếp tục làm bài" }),
    ).toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(screen.queryByRole("button", { name: "Tham gia lớp" })).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("draws no skeleton beside a paper while the classes are still loading", async () => {
    serve({ dueNow: [card()] });
    server.use(http.get(`${BASE}/app/classes`, () => delay("infinite")));
    show();
    await screen.findByRole("link", { name: /Unit 5/ });
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("draws no skeleton while a later refetch is in flight", async () => {
    home({ dueNow: [card()] });
    await screen.findByRole("link", { name: /Unit 5/ });
    server.use(http.get(`${BASE}/app/assignments`, () => delay("infinite")));
    act(() => focusManager.setFocused(false));
    act(() => focusManager.setFocused(true));
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("keeps the lists on screen when a later refetch fails", async () => {
    home({ dueNow: [card()] });
    await screen.findByRole("link", { name: /Unit 5/ });

    let failed = 0;
    server.use(
      http.get(`${BASE}/app/assignments`, () => {
        failed += 1;
        return new Response(null, { status: 500 });
      }),
    );
    act(() => focusManager.setFocused(false));
    act(() => focusManager.setFocused(true));
    await waitFor(() => expect(failed).toBe(1));
    await act(() => vi.advanceTimersByTimeAsync(50));

    expect(screen.getByRole("link", { name: /Unit 5/ })).toBeInTheDocument();
    expect(screen.queryByText("Không tải được. Hãy thử lại.")).toBeNull();
  });
});

describe("in English", () => {
  it("writes the deck's words", async () => {
    await i18n.changeLanguage("en");
    home({
      dueNow: [
        live({
          questionCount: 8,
          liveAnsweredCount: 3,
          className: "IELTS 6.5 Evening",
        }),
        card({ id: id(2), testTitle: "Tomorrow", closesAt: "2026-08-30T09:59:00Z" }),
        card({ id: id(3), testTitle: "Next week", closesAt: "2026-09-05T00:00:00Z" }),
      ],
      upcoming: [scheduled({ id: id(4) })],
      completed: [
        done(5, { score: { earned: 27, total: 30, pendingManual: 0 } }),
        done(6, {
          lastSubmittedAt: "2026-08-25T03:00:00Z",
          score: { earned: 2, total: 30, pendingManual: 1 },
        }),
      ],
    });
    expect(
      await screen.findByRole("heading", { level: 1, name: "Good afternoon, An" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("You have a test in progress. It closes at 17:38."),
    ).toBeInTheDocument();
    expect(screen.getByText("In progress · 38 min left")).toBeInTheDocument();
    expect(
      screen.getByText("IELTS 6.5 Evening · 3 of 8 answered · closes today 17:38"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Continue test" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Coming up" })).toBeInTheDocument();
    expect(screen.getByText("3 tests")).toBeInTheDocument();
    expect(within(row("Tomorrow")).getByText("Due tomorrow")).toBeInTheDocument();
    expect(within(row("Tomorrow")).getByText("Sun")).toBeInTheDocument();
    expect(
      within(row("Tomorrow")).getByText("45 min · 24 questions"),
    ).toBeInTheDocument();
    expect(within(row("Next week")).getByText("Open now")).toBeInTheDocument();
    expect(within(row("Listening")).getByText("Opens Tue")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Recent results" })).toBeInTheDocument();
    expect(within(row("Paper 5")).getByText("27 / 30")).toBeInTheDocument();
    expect(within(row("Paper 5")).getByText("Wed 26 Aug")).toBeInTheDocument();
    expect(within(row("Paper 6")).getByText("Being graded")).toBeInTheDocument();
  });

  it("writes the weekday and the date of an opening", async () => {
    await i18n.changeLanguage("en");
    home({
      upcoming: [
        scheduled(),
        scheduled({ id: id(5), testTitle: "Far", opensAt: "2026-09-04T17:00:00Z" }),
      ],
    });
    expect(
      await screen.findByText(
        "Nothing due today. Next up: Listening practice 03 on Tuesday.",
      ),
    ).toBeInTheDocument();
    expect(within(row("Far")).getByText("Opens 5 Sep")).toBeInTheDocument();
  });

  it("counts one test as one", async () => {
    await i18n.changeLanguage("en");
    home({ dueNow: [card({ closesAt: "2026-09-05T00:00:00Z" })] });
    expect(await screen.findByText("1 test")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Nothing due today. Next up: Unit 5 — Present perfect on 5 Sep.",
      ),
    ).toBeInTheDocument();
  });
});
