import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import StudentHomePage from "@/features/assignments/pages/StudentHomePage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import { ATTEMPT, BASE, card, mockStart, renderAt } from "./support";
import "@/lib/i18n";

const SAMPLE_CLASS = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "IELTS Foundation",
  description: null,
  studentCount: 12,
  openAssignmentCount: 0,
  archivedAt: null,
  selfJoinEnabled: true,
  createdAt: "2026-06-01T00:00:00Z",
};

function home(
  sections: { dueNow?: unknown[]; upcoming?: unknown[]; completed?: unknown[] },
  classes: unknown[] = [SAMPLE_CLASS],
) {
  server.use(
    http.get(`${BASE}/app/assignments`, () =>
      contractJson("/app/assignments", "get", 200, {
        dueNow: sections.dueNow ?? [],
        upcoming: sections.upcoming ?? [],
        completed: sections.completed ?? [],
      }),
    ),
    http.get(`${BASE}/app/classes`, () =>
      contractJson("/app/classes", "get", 200, { items: classes }),
    ),
  );
  return renderAt("/app", [
    { path: "/app", element: <StudentHomePage /> },
    { path: "/app/assignments/:id", element: <p>intro page</p> },
  ]);
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z")); // 17:00 in Asia/Ho_Chi_Minh
});
afterEach(() => {
  vi.useRealTimers();
  useAuthStore.getState().clearSession();
});

describe("the three sections", () => {
  it("greets by given name and counts what is due today", async () => {
    home({ dueNow: [card()] });
    expect(
      await screen.findByRole("heading", { name: "Chào An 👋" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Bạn có 1 bài đến hạn hôm nay.")).toBeInTheDocument();
  });

  it("draws the due card with time left, the clock, the attempt and the close", async () => {
    home({ dueNow: [card({ closesAt: "2026-08-29T14:00:00Z" })] });
    await screen.findByText("Unit 5 — Present perfect");
    expect(screen.getByText("Còn 4 giờ")).toBeInTheDocument();
    expect(screen.getByText("45 phút")).toBeInTheDocument();
    expect(screen.getByText("Lượt 1/2")).toBeInTheDocument();
    expect(screen.getByText("Đóng lúc 21:00 hôm nay")).toBeInTheDocument();
  });

  it("sends the start button to the intro", async () => {
    const user = userEvent.setup();
    const router = home({ dueNow: [card()] });
    await user.click(await screen.findByRole("link", { name: "Bắt đầu làm bài" }));
    expect(router.state.location.pathname).toBe("/app/assignments/" + card().id);
  });

  it("lists upcoming with when they open, and completed with a score or 'chờ chấm'", async () => {
    home({
      upcoming: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d2",
          testTitle: "Listening practice 03",
          status: "scheduled",
          opensAt: "2026-09-01T01:00:00Z",
          closesAt: "2026-09-01T14:00:00Z",
        }),
      ],
      completed: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d3",
          testTitle: "Unit 4 — Passive voice",
          status: "closed",
          attemptsUsed: 1,
          score: { earned: 27, total: 30, pendingManual: 0 },
        }),
        card({
          id: "018f0000-0000-7000-8000-0000000000d4",
          testTitle: "Mid-term mock",
          status: "closed",
          attemptsUsed: 1,
          score: { earned: 20, total: 30, pendingManual: 2 },
        }),
        card({
          id: "018f0000-0000-7000-8000-0000000000d5",
          testTitle: "Hidden score",
          status: "closed",
          attemptsUsed: 1,
        }),
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "Sắp tới · 1" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Mở 08:00 · Thứ Ba, 01/09")).toBeInTheDocument();
    expect(screen.getByText("Đã lên lịch")).toBeInTheDocument();

    const done = screen.getByRole("heading", {
      name: "Đã hoàn thành · 3",
    }).parentElement!;
    expect(within(done).getByText("27/30")).toBeInTheDocument();
    expect(within(done).getByText("Chờ chấm")).toBeInTheDocument();
    expect(within(done).queryByText(/20\/30/)).toBeNull();
  });

  it("does not draw a section with nothing in it", async () => {
    home({ dueNow: [card()] });
    await screen.findByText("Unit 5 — Present perfect");
    expect(screen.queryByRole("heading", { name: /Sắp tới/ })).toBeNull();
    expect(screen.queryByRole("heading", { name: /Đã hoàn thành/ })).toBeNull();
  });
});

/**
 * An in-progress attempt is burning a server-side clock the student cannot
 * see from anywhere else, so it outranks everything, including a nearer
 * deadline.
 */
describe("a live attempt", () => {
  it("is offered first, above a due card, and resumes straight into the paper", async () => {
    const user = userEvent.setup();
    const calls = mockStart();
    const router = home({
      dueNow: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d9",
          testTitle: "Sooner",
          closesAt: "2026-08-29T11:00:00Z",
        }),
        card({ hasLiveAttempt: true, lastAttemptId: ATTEMPT }),
      ],
    });

    const resume = await screen.findByText("Bạn đang làm dở một bài");
    const sooner = screen.getByText("Sooner");
    expect(
      resume.compareDocumentPosition(sooner) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      screen.getByText("Unit 5 — Present perfect. Đồng hồ vẫn đang chạy."),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Tiếp tục làm bài" }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(calls).toEqual(["start"]);
  });
});

// Two different empties, two different truths. A student with a class and no
// homework is not offered a join code -- that reads as an error.
describe("nothing assigned", () => {
  it("says so, and offers nothing else to a student who has a class", async () => {
    home({});
    expect(
      await screen.findByText("Hiện chưa có bài nào được giao."),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Chào An" })).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole("link", { name: "Tham gia lớp" })).toBeNull(),
    );
  });

  it("offers the way into a class to a student who has none", async () => {
    home({}, []);
    expect(await screen.findByText("Bạn chưa tham gia lớp nào.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Tham gia lớp" })).toHaveAttribute(
      "href",
      "/join",
    );
  });
});

/** The four things S-03 draws that the card could not carry before. */
describe("what the deck's card says", () => {
  it("names the class the assignment came through, beside the badge", async () => {
    home({ dueNow: [card({ className: "IELTS Foundation" })] });
    expect(await screen.findByText("IELTS Foundation")).toBeInTheDocument();
  });

  it("says nothing about the class when the server names none", async () => {
    home({ dueNow: [card({ className: null })] }, []);
    await screen.findByText("Unit 5 — Present perfect");
    expect(screen.queryByText("IELTS Foundation")).toBeNull();
  });

  it("states how many questions the paper has", async () => {
    home({ dueNow: [card({ questionCount: 24 })] });
    expect(await screen.findByText("24 câu")).toBeInTheDocument();
  });

  it("dates a completed row by when it was handed in", async () => {
    home({
      completed: [
        card({
          status: "closed",
          attemptsUsed: 1,
          lastSubmittedAt: "2026-08-26T09:30:00Z",
        }),
      ],
    });
    expect(await screen.findByText("Nộp 26/08")).toBeInTheDocument();
    expect(screen.queryByText("Lượt 1/2")).toBeNull();
  });

  it("falls back to the attempt count when there is no submission time", async () => {
    home({
      completed: [card({ status: "closed", attemptsUsed: 1, lastSubmittedAt: null })],
    });
    expect(await screen.findByText("Lượt 1/2")).toBeInTheDocument();
  });
});

describe("the resume card's clock", () => {
  it("shows how long is left, not merely that the clock is running", async () => {
    home({
      dueNow: [
        card({
          hasLiveAttempt: true,
          liveDeadlineAt: "2026-08-29T10:22:14Z",
        }),
      ],
    });
    // shouldAdvanceTime moves the seconds, so only the minute is asserted.
    expect(
      await screen.findByText(/Đồng hồ vẫn đang chạy, còn 22:\d{2}\./),
    ).toBeInTheDocument();
  });

  it("counts down rather than freezing at what it was on load", async () => {
    home({
      dueNow: [card({ hasLiveAttempt: true, liveDeadlineAt: "2026-08-29T10:22:14Z" })],
    });
    await screen.findByText(/còn 22:\d{2}\./);
    await vi.advanceTimersByTimeAsync(65_000);
    expect(await screen.findByText(/còn 21:\d{2}\./)).toBeInTheDocument();
  });

  it("stops at zero", async () => {
    home({
      dueNow: [card({ hasLiveAttempt: true, liveDeadlineAt: "2026-08-29T09:59:00Z" })],
    });
    expect(await screen.findByText(/còn 00:00\./)).toBeInTheDocument();
  });

  it("says only that the clock is running when no deadline came with it", async () => {
    home({ dueNow: [card({ hasLiveAttempt: true, liveDeadlineAt: null })] });
    expect(await screen.findByText(/Đồng hồ vẫn đang chạy\.$/)).toBeInTheDocument();
  });
});

describe("the completed card", () => {
  it("links its title to the result when there is a paper to show", async () => {
    home({
      completed: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d3",
          testTitle: "Unit 4 — Passive voice",
          status: "closed",
          attemptsUsed: 1,
          lastAttemptId: "018f0000-0000-7000-8000-0000000000a7",
          score: { earned: 27, total: 30, pendingManual: 0 },
        }),
        card({
          id: "018f0000-0000-7000-8000-0000000000d5",
          testTitle: "Never started",
          status: "closed",
          attemptsUsed: 0,
          lastAttemptId: null,
        }),
      ],
    });
    expect(
      await screen.findByRole("link", { name: "Unit 4 — Passive voice" }),
    ).toHaveAttribute(
      "href",
      "/app/attempts/018f0000-0000-7000-8000-0000000000a7/result",
    );
    expect(
      screen.queryByRole("link", { name: "Never started" }),
    ).not.toBeInTheDocument();
  });
});
