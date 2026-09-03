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

  // The rules are read before the clock starts (S-04), so the card's button
  // goes to the intro, never straight into the paper.
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
