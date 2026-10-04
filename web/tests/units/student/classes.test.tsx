import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { delay, http } from "msw";
import { focusManager } from "@tanstack/react-query";
import { Toaster, toast } from "@/components/ui/sonner";
import StudentClassesPage from "@/features/classes/pages/StudentClassesPage";
import i18n from "@/lib/i18n";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { sampleClass } from "@tests/support/fixtures";
import { useAuthStore } from "@/stores/auth";
import { viewport } from "@tests/support/viewport";
import { ATTEMPT, BASE, card, renderAt } from "./support";

const A = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "IELTS Foundation A",
  description: null,
  teacherName: "Hoàng Thương",
  joinedAt: "2026-07-12T01:00:00Z",
};
const B = { ...A, id: "018f0000-0000-7000-8000-0000000000c2", name: "TOEIC 600" };

type Lists = { dueNow?: unknown[]; upcoming?: unknown[]; completed?: unknown[] };

let enrolled: unknown[] = [];
let asked = { classes: 0, assignments: 0 };

function id(n: number) {
  return `018f0000-0000-7000-8000-${String(n).padStart(12, "0")}`;
}

function serveClasses(how: "load" | "fail" | "wait" = "load") {
  server.use(
    http.get(`${BASE}/app/classes`, async () => {
      asked.classes += 1;
      if (how === "wait") await delay("infinite");
      if (how === "fail") return new Response(null, { status: 500 });
      return contractJson("/app/classes", "get", 200, { items: enrolled });
    }),
  );
}

function serveLists(lists: Lists | "fail" | "wait" = {}) {
  server.use(
    http.get(`${BASE}/app/assignments`, async () => {
      asked.assignments += 1;
      if (lists === "wait") await delay("infinite");
      if (lists === "fail" || lists === "wait")
        return new Response(null, { status: 500 });
      return contractJson("/app/assignments", "get", 200, {
        dueNow: lists.dueNow ?? [],
        upcoming: lists.upcoming ?? [],
        completed: lists.completed ?? [],
      });
    }),
  );
}

function show() {
  return renderAt("/app/classes", [
    {
      path: "/app/classes",
      element: (
        <>
          <main>
            <StudentClassesPage />
          </main>
          <Toaster />
        </>
      ),
    },
  ]);
}

function page(classes: unknown[], lists: Lists | "fail" | "wait" = {}) {
  enrolled = classes;
  serveClasses();
  serveLists(lists);
  return show();
}

function cardOf(name: string) {
  return within(screen.getByRole("heading", { level: 2, name }).closest("article")!);
}

beforeEach(() => {
  enrolled = [];
  asked = { classes: 0, assignments: 0 };
  viewport("phone");
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z"));
});
afterEach(async () => {
  act(() => {
    toast.dismiss();
  });
  vi.useRealTimers();
  vi.unstubAllGlobals();
  focusManager.setFocused();
  useAuthStore.getState().clearSession();
  await i18n.changeLanguage("vi");
});

describe("the page", () => {
  it("says what it is and offers the one way into another class", async () => {
    page([A]);
    expect(
      await screen.findByRole("heading", { level: 1, name: "Lớp của tôi" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Giáo viên của bạn và bài tiếp theo ở từng lớp."),
    ).toBeInTheDocument();
    await screen.findByRole("heading", { level: 2, name: A.name });
    expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual([
      "Tham gia lớp",
    ]);
    expect(screen.getByRole("button", { name: "Tham gia lớp" })).toHaveClass(
      "h-10",
      "min-h-0",
    );
  });

  it("draws a class with its teacher, and no way to open, message or leave it", async () => {
    page([A]);
    const card = cardOf((await screen.findByRole("heading", { level: 2 })).textContent);
    expect(card.getByText("Hoàng Thương")).toBeInTheDocument();
    expect(card.getByText("Giáo viên")).toBeInTheDocument();
    expect(card.getByText("TH")).toBeInTheDocument();
    expect(card.queryAllByRole("link")).toEqual([]);
    expect(card.queryAllByRole("button")).toEqual([]);
    expect(screen.queryByText(/Tuần này/)).toBeNull();
    expect(screen.queryByText(/tham gia 12\/07/)).toBeNull();
  });

  it("puts a class's description under its name until the timetable ships", async () => {
    page([{ ...A, description: "Thứ 3 và thứ 5, 19:30–21:00." }, B]);
    await screen.findByRole("heading", { level: 2, name: B.name });
    expect(cardOf(A.name).getByText("Thứ 3 và thứ 5, 19:30–21:00.")).toHaveClass(
      "text-muted-fg",
    );
    expect(cardOf(B.name).queryByText(/Thứ 3/)).toBeNull();
  });

  it("leaves out the teacher row for a class with no teacher name", async () => {
    page([{ ...A, teacherName: null }]);
    await screen.findByRole("heading", { level: 2, name: A.name });
    expect(cardOf(A.name).queryByText("Giáo viên")).toBeNull();
    expect(cardOf(A.name).getByText("Tiếp theo")).toBeInTheDocument();
  });
});

describe("what comes next in a class", () => {
  it.each([
    [
      "an attempt in progress",
      card({
        hasLiveAttempt: true,
        lastAttemptId: ATTEMPT,
        liveDeadlineAt: "2026-08-29T10:38:00Z",
      }),
      "Unit 5 — Present perfect · đang làm",
    ],
    [
      "a paper closing today",
      card({ closesAt: "2026-08-29T14:00:00Z" }),
      "Unit 5 — Present perfect · đóng lúc 21:00 hôm nay",
    ],
    [
      "a paper closing another day",
      card({ closesAt: "2026-09-03T01:00:00Z" }),
      "Unit 5 — Present perfect · hạn Thứ 5, 03/09",
    ],
    [
      "a paper opening later today",
      card({
        status: "scheduled",
        opensAt: "2026-08-29T12:00:00Z",
        closesAt: "2026-09-08T01:00:00Z",
      }),
      "Unit 5 — Present perfect · mở lúc 19:00 hôm nay",
    ],
    [
      "a paper opening another day",
      card({
        status: "scheduled",
        opensAt: "2026-09-01T01:00:00Z",
        closesAt: "2026-09-08T01:00:00Z",
      }),
      "Unit 5 — Present perfect · mở Thứ 3, 01/09",
    ],
  ])("says %s", async (_, paper, line) => {
    const list = paper.status === "scheduled" ? "upcoming" : "dueNow";
    page([A], { [list]: [{ ...paper, classId: A.id, className: A.name }] });
    expect(await screen.findByText(line)).toBeInTheDocument();
    expect(cardOf(A.name).getByText("Tiếp theo")).toBeInTheDocument();
  });

  it("is the earliest paper of that class, and nothing for a class without one", async () => {
    page([A, B], {
      dueNow: [
        card({
          id: id(1),
          testTitle: "Later",
          classId: A.id,
          closesAt: "2026-09-03T01:00:00Z",
        }),
        card({
          id: id(2),
          testTitle: "Sooner",
          classId: A.id,
          closesAt: "2026-08-30T01:00:00Z",
        }),
        card({
          id: id(3),
          testTitle: "Shared",
          classId: null,
          closesAt: "2026-08-29T11:00:00Z",
        }),
      ],
      completed: [
        card({ id: id(4), testTitle: "Done", classId: B.id, status: "closed" }),
      ],
    });
    expect(await screen.findByText("Sooner · hạn CN, 30/08")).toBeInTheDocument();
    expect(cardOf(A.name).queryByText(/Later|Shared/)).toBeNull();
    expect(cardOf(B.name).getByText("Chưa có bài nào được giao")).toBeInTheDocument();
  });

  it("is a grey bar while the papers load", async () => {
    page([A], "wait");
    await screen.findByRole("heading", { level: 2, name: A.name });
    expect(cardOf(A.name).queryByText("Chưa có bài nào được giao")).toBeNull();
    expect(cardOf(A.name).queryByText("Chưa tải được bài của lớp")).toBeNull();
    const footer = cardOf(A.name).getByText("Tiếp theo").parentElement!;
    expect(footer.querySelector("[data-slot='skeleton']")).not.toBeNull();
  });

  it("says the papers could not be loaded, and keeps the cards", async () => {
    page([A], "fail");
    expect(await screen.findByText("Chưa tải được bài của lớp")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: A.name })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps what it said when a later read of the papers fails", async () => {
    page([A], { dueNow: [card({ classId: A.id })] });
    const line = await screen.findByText(/^Unit 5 — Present perfect · /);
    serveLists("fail");
    act(() => focusManager.setFocused(false));
    act(() => focusManager.setFocused(true));
    await waitFor(() => expect(asked.assignments).toBe(2));
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(line).toBeInTheDocument();
    expect(screen.queryByText("Chưa tải được bài của lớp")).toBeNull();
  });

  it("moves on when the paper it named closes, without asking the server", async () => {
    page([A], {
      dueNow: [
        card({
          id: id(1),
          testTitle: "Closing",
          classId: A.id,
          closesAt: "2026-08-29T10:00:40Z",
        }),
        card({
          id: id(2),
          testTitle: "After",
          classId: A.id,
          closesAt: "2026-09-03T01:00:00Z",
        }),
      ],
    });
    expect(await screen.findByText(/^Closing · /)).toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(61_000));
    expect(screen.getByText(/^After · /)).toBeInTheDocument();
    expect(asked.assignments).toBe(1);
  });
});

describe("the states the deck does not draw", () => {
  it("shows card shapes while the classes load", async () => {
    enrolled = [A];
    serveClasses("wait");
    serveLists();
    show();
    expect(
      await screen.findByRole("status", { name: "Đang tải…" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tham gia lớp" })).toBeInTheDocument();
  });

  it("says the classes did not load and loads them on retry", async () => {
    const user = userEvent.setup();
    enrolled = [A];
    serveClasses("fail");
    serveLists();
    show();
    expect(await screen.findByText("Không tải được. Hãy thử lại.")).toBeInTheDocument();
    serveClasses();
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(
      await screen.findByRole("heading", { level: 2, name: A.name }),
    ).toBeInTheDocument();
  });

  it("says there are no classes, with the header's button as the one action", async () => {
    page([]);
    expect(await screen.findByText("Bạn chưa tham gia lớp nào.")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Tham gia lớp" })).toHaveLength(1);
  });

  it("keeps the cards when a later read of the classes fails", async () => {
    page([A]);
    await screen.findByRole("heading", { level: 2, name: A.name });
    serveClasses("fail");
    act(() => focusManager.setFocused(false));
    act(() => focusManager.setFocused(true));
    await waitFor(() => expect(asked.classes).toBe(2));
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(screen.getByRole("heading", { level: 2, name: A.name })).toBeInTheDocument();
    expect(screen.queryByText("Không tải được. Hãy thử lại.")).toBeNull();
  });
});

describe("joining from the page", () => {
  it("opens the dialog, joins, says so and shows the new class", async () => {
    const user = userEvent.setup();
    const previews: string[] = [];
    const joins: string[] = [];
    server.use(
      http.post(`${BASE}/join/preview`, async ({ request }) => {
        previews.push(((await request.json()) as { joinCode: string }).joinCode);
        return contractJson("/join/preview", "post", 200, {
          classId: B.id,
          className: B.name,
          teacherName: "Phan Minh Đức",
        });
      }),
      http.post(`${BASE}/app/classes/join`, async ({ request }) => {
        joins.push(((await request.json()) as { joinCode: string }).joinCode);
        enrolled = [A, B];
        return contractJson("/app/classes/join", "post", 200, {
          ...sampleClass,
          id: B.id,
          name: B.name,
        });
      }),
    );
    page([A]);
    await screen.findByRole("heading", { level: 2, name: A.name });
    await user.click(screen.getByRole("button", { name: "Tham gia lớp" }));
    const dialog = within(await screen.findByRole("dialog", { name: "Tham gia lớp" }));
    await user.type(dialog.getByLabelText("Mã lớp"), "W3RT8KDZ");
    expect(await dialog.findByText(B.name)).toBeInTheDocument();
    await user.click(dialog.getByRole("button", { name: "Tham gia" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(await screen.findByText(`Bạn đã vào lớp ${B.name}`)).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: B.name })).toBeInTheDocument();
    expect(previews).toEqual(["W3RT8KDZ"]);
    expect(joins).toEqual(["W3RT8KDZ"]);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Tham gia lớp" })).toHaveFocus(),
    );
  });
});

describe("in English", () => {
  it("writes the deck's words", async () => {
    await i18n.changeLanguage("en");
    page([A, B], {
      dueNow: [card({ classId: A.id, closesAt: "2026-08-29T14:00:00Z" })],
      upcoming: [
        card({
          id: id(2),
          testTitle: "Listening practice 03",
          classId: B.id,
          status: "scheduled",
          opensAt: "2026-09-01T01:00:00Z",
          closesAt: "2026-09-08T01:00:00Z",
        }),
      ],
    });
    expect(
      await screen.findByRole("heading", { level: 1, name: "Classes" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Your teachers and what's next in each class."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Join a class" })).toBeInTheDocument();
    expect(
      await screen.findByText("Unit 5 — Present perfect closes today at 21:00"),
    ).toBeInTheDocument();
    expect(cardOf(A.name).getByText("Teacher")).toBeInTheDocument();
    expect(cardOf(A.name).getByText("Next")).toBeInTheDocument();
    expect(
      cardOf(B.name).getByText("Listening practice 03 · opens Tue 1 Sep"),
    ).toBeInTheDocument();
  });

  it("says a class has nothing assigned", async () => {
    await i18n.changeLanguage("en");
    page([A]);
    expect(await screen.findByText("Nothing assigned yet")).toBeInTheDocument();
  });
});
