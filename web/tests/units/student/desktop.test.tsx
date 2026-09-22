import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { http } from "msw";
import StudentLayout from "@/layouts/StudentLayout";
import StudentHomePage from "@/features/assignments/pages/StudentHomePage";
import AssignmentIntroPage from "@/features/assignments/pages/AssignmentIntroPage";
import StudentClassesPage from "@/features/classes/pages/StudentClassesPage";
import StudentSettingsPage from "@/features/auth/pages/StudentSettingsPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { viewport } from "@tests/support/viewport";
import { useAuthStore } from "@/stores/auth";
import { ASSIGNMENT, BASE, card, detail, renderAt } from "./support";
import "@/lib/i18n";

const CLASS = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "IELTS Foundation",
  description: "Thứ 3 và thứ 5, 19:30–21:00.",
  teacherName: "Cô Thương",
  joinedAt: "2026-06-01T00:00:00Z",
};

function serveStudent(
  sections: { dueNow?: unknown[]; upcoming?: unknown[]; completed?: unknown[] },
  classes: unknown[] = [CLASS],
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
}

const shell = (
  path: string,
  page: React.ReactElement,
  handle?: object,
  route: string = path,
) =>
  renderAt(path, [
    {
      element: <StudentLayout />,
      children: [{ path: route, element: page, ...(handle ? { handle } : {}) }],
    },
  ]);

beforeEach(() => {
  viewport("desktop");
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z"));
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
});

describe("the shell (S-13)", () => {
  it("shows the bar with the student's name on every screen, including a detail one", async () => {
    serveStudent({});
    shell("/app/settings", <StudentSettingsPage />, {
      detail: true,
      titleKey: "nav.settings",
    });
    expect(await screen.findByRole("link", { name: "Bài của tôi" })).toHaveAttribute(
      "href",
      "/app",
    );
    expect(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    ).toHaveTextContent("An");
    expect(screen.queryByRole("button", { name: "Quay lại" })).toBeNull();
  });

  it("keeps the back arrow on a phone's detail screen", async () => {
    viewport("phone");
    serveStudent({});
    shell("/app/settings", <StudentSettingsPage />, {
      detail: true,
      titleKey: "nav.settings",
    });
    expect(await screen.findByRole("button", { name: "Quay lại" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Cài đặt");
    expect(screen.queryByRole("link", { name: "Bài của tôi" })).toBeNull();
  });
});

describe("home (S-13)", () => {
  it("keeps all assignment groups in the main reading flow", async () => {
    serveStudent({
      dueNow: [card({ className: "IELTS Foundation" })],
      upcoming: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d2",
          testTitle: "Listening practice 03",
          status: "scheduled",
          opensAt: "2026-09-01T01:00:00Z",
        }),
      ],
      completed: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d3",
          testTitle: "Unit 4 — Passive voice",
          status: "closed",
          className: "IELTS Foundation",
          lastAttemptId: "018f0000-0000-7000-8000-0000000000e3",
          lastSubmittedAt: "2026-08-26T13:14:00Z",
        }),
      ],
    });
    shell("/app", <StudentHomePage />);
    await screen.findByText("Unit 5 — Present perfect");

    const main = within(screen.getByRole("main"));
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(main.getByRole("heading", { name: "Sắp tới · 1" })).toBeInTheDocument();
    expect(main.getByText("Listening practice 03")).toBeInTheDocument();
    expect(main.getAllByRole("link", { name: "Xem chi tiết" })).toHaveLength(2);
    expect(main.getByText("IELTS Foundation · Nộp 26/08")).toBeInTheDocument();
  });

  it("draws no panel when there is nothing to put in it", async () => {
    serveStudent({ dueNow: [card()] }, []);
    shell("/app", <StudentHomePage />);
    await screen.findByText("Unit 5 — Present perfect");
    expect(screen.queryByRole("complementary")).toBeNull();
  });
});

describe("the intro (S-14)", () => {
  it("keeps the facts and start action together, with a way back", async () => {
    server.use(
      http.get(`${BASE}/app/assignments/${ASSIGNMENT}`, () =>
        contractJson("/app/assignments/{id}", "get", 200, detail()),
      ),
    );
    shell(
      `/app/assignments/${ASSIGNMENT}`,
      <AssignmentIntroPage />,
      { detail: true, titleKey: "student.assignmentDetail" },
      "/app/assignments/:id",
    );
    await screen.findByRole("heading", { level: 1, name: "Unit 5 — Present perfect" });

    const panel = within(screen.getByRole("main"));
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(panel.getByText("Thời lượng").nextElementSibling).toHaveTextContent(
      "45 phút",
    );
    expect(panel.getByRole("button", { name: "Bắt đầu làm bài" })).toBeInTheDocument();
    expect(
      within(screen.getByRole("main")).getByRole("link", { name: "Bài của tôi" }),
    ).toHaveAttribute("href", "/app");
    expect(screen.getByRole("heading", { name: "Khi làm bài" })).toBeInTheDocument();
  });
});

describe("classes (S-17)", () => {
  it("links each class to its assignments and exposes one join action", async () => {
    serveStudent({
      dueNow: [card({ classId: CLASS.id, className: CLASS.name })],
      completed: [
        card({
          id: "018f0000-0000-7000-8000-0000000000d3",
          status: "closed",
          classId: CLASS.id,
          className: CLASS.name,
          lastSubmittedAt: "2026-08-26T13:14:00Z",
        }),
      ],
    });
    shell("/app/classes", <StudentClassesPage />);
    await screen.findByText("IELTS Foundation");
    expect(screen.getByText("Bạn đang ở trong 1 lớp.")).toBeInTheDocument();
    expect(await screen.findByText("1 bài đang mở")).toBeInTheDocument();
    expect(screen.getByText("1 bài đã nộp")).toBeInTheDocument();

    expect(screen.getByRole("link", { name: "Tham gia lớp" })).toHaveAttribute(
      "href",
      "/join",
    );
    expect(screen.getByRole("link", { name: "Xem bài của lớp" })).toHaveAttribute(
      "href",
      `/app?classId=${CLASS.id}`,
    );
    expect(screen.queryByRole("complementary")).toBeNull();
  });
});

describe("settings (S-17)", () => {
  it("keeps the account, editable forms and sign-out in one flow", async () => {
    serveStudent({});
    shell("/app/settings", <StudentSettingsPage />, {
      detail: true,
      titleKey: "nav.settings",
    });
    const panel = within(screen.getByRole("main"));
    expect(panel.getByText("Nguyễn Văn An")).toBeInTheDocument();
    expect(panel.getByText("an@example.com")).toBeInTheDocument();
    expect(panel.getByText(/Học viên ·/)).toBeInTheDocument();
    expect(panel.getByRole("button", { name: "Đăng xuất" })).toBeInTheDocument();
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(
      screen.getByRole("heading", { level: 1, name: "Cài đặt" }),
    ).toBeInTheDocument();
  });
});
