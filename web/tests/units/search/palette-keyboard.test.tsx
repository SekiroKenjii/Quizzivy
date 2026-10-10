import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider, useLocation } from "react-router";
import { http } from "msw";
import { CommandPalette } from "@/features/search/CommandPalette";
import type { PermissionKey } from "@/features/auth/permissions";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const BASE = "http://localhost:8080";
const NOW = new Date("2026-08-29T10:00:00Z");
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";
const ASSIGNMENT_ID = "018f0000-0000-7000-8000-0000000000d1";
const STUDENT_ID = "018f0000-0000-7000-8000-0000000000e1";
const CLASS_ID = "018f0000-0000-7000-8000-0000000000c1";
const LOAD = { timeout: 5000 };

function assignment() {
  return {
    id: ASSIGNMENT_ID,
    testId: TEST_ID,
    testVersionId: "018f0000-0000-7000-8000-0000000000f1",
    testVersion: 3,
    testTitle: "Unit 5 — Present perfect",
    targets: {
      classes: [{ id: CLASS_ID, name: "IELTS Foundation", studentCount: 18 }],
      students: [],
    },
    publishedAt: "2026-08-27T00:00:00Z",
    updatedAt: "2026-08-27T00:00:00Z",
    window: {
      opensAt: "2026-08-28T00:00:00Z",
      closesAt: "2026-08-31T14:00:00Z",
      closedAt: null,
    },
    durationMinutes: 45,
    maxAttempts: 1,
    shuffleQuestions: false,
    shuffleOptions: false,
    review: {
      showScore: true,
      showCorrectAnswers: false,
      showExplanations: false,
      release: "on_submit",
      showClassAverage: false,
    },
    studentNote: null,
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 0,
      onLimitExceeded: "flag" as const,
      minAwayMs: 3000,
    },
    status: "open" as const,
    submittedCount: 12,
    targetCount: 19,
    flaggedCount: 0,
  };
}

function student() {
  return {
    id: STUDENT_ID,
    email: "han@example.com",
    fullName: "Phạm Gia Hân",
    hasPassword: true,
    linkedProviders: [],
    mustChangePassword: false,
    createdAt: "2026-01-01T00:00:00Z",
    disabledAt: null,
    classes: [
      {
        id: CLASS_ID,
        name: "IELTS Foundation",
        joinedVia: "admin" as const,
        joinedAt: "2026-06-01T00:00:00Z",
      },
    ],
    stats: {
      submittedCount: 0,
      flaggedCount: 0,
      activity: { live: false, lastAttemptAt: null },
    },
  };
}

function test() {
  return {
    skills: [],
    assignments: { live: 0, scheduled: 0, closed: 0 },
    unpublishedChanges: null,
    id: TEST_ID,
    title: "Unit 5 — Present perfect",
    description: null,
    status: "published" as const,
    currentVersion: 1,
    nextVersion: 2,
    totalPoints: 2,
    questionCount: 1,
    audioCount: 0,
    sections: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
  };
}

function question() {
  return {
    level: null,
    skill: null,
    id: QUESTION_ID,
    type: "single_choice" as const,
    prompt: "Unit 5 · Người phụ nữ đề nghị làm gì?",
    media: null,
    audio: null,
    transcript: null,
    options: [],
    blanks: [],
    points: 2,
    explanation: null,
    sampleAnswer: null,
    tags: ["unit-5"],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

const QUESTION_FACETS = {
  levels: { pre_a1: 0, a1: 0, a2: 0, b1: 0, b2: 0, c1: 0, c2: 0 },
  skills: {
    grammar: 0,
    vocabulary: 0,
    reading: 0,
    listening: 0,
    writing: 0,
    speaking: 0,
  },
  all: 1,
  single_choice: 1,
  multiple_choice: 0,
  true_false: 0,
  fill_blank: 0,
  short_answer: 0,
};

let asked: { path: string; q: string | null }[] = [];
let matching = true;

function remember(request: Request) {
  const url = new URL(request.url);
  asked.push({ path: url.pathname, q: url.searchParams.get("q") });
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: NOW });
  asked = [];
  matching = true;
  grant(teacherUser.permissions);
  server.use(
    http.get(`${BASE}/teacher/assignments`, ({ request }) => {
      remember(request);
      const items = matching ? [assignment()] : [];
      return contractJson("/teacher/assignments", "get", 200, {
        page: 1,
        pageSize: 4,
        total: items.length,
        items,
        facets: { all: 1, draft: 0, scheduled: 0, open: 1, closed: 0 },
      });
    }),
    http.get(`${BASE}/teacher/students`, ({ request }) => {
      remember(request);
      const items = matching ? [student()] : [];
      return contractJson("/teacher/students", "get", 200, {
        items,
        page: 1,
        pageSize: 3,
        total: items.length,
        facets: { total: items.length, activeLast7Days: 0 },
      });
    }),
    http.get(`${BASE}/teacher/tests`, ({ request }) => {
      remember(request);
      const items = matching ? [test()] : [];
      return contractJson("/teacher/tests", "get", 200, {
        facets: { all: 1, draft: 0, published: 1, archived: 0 },
        tags: [],
        items,
        page: 1,
        pageSize: 4,
        total: items.length,
      });
    }),
    http.get(`${BASE}/teacher/questions`, ({ request }) => {
      remember(request);
      const items = matching ? [question()] : [];
      return contractJson("/teacher/questions", "get", 200, {
        facets: QUESTION_FACETS,
        tags: [],
        bankTotal: 1,
        items,
        page: 1,
        pageSize: 3,
        total: items.length,
      });
    }),
  );
});

afterEach(() => {
  vi.useRealTimers();
  useAuthStore.getState().clearSession();
});

function grant(permissions: readonly PermissionKey[]) {
  useAuthStore.getState().setSession("token", {
    ...teacherUser,
    permissions: [...permissions],
    workspaces: ["teacher"],
  });
}

function Where() {
  const location = useLocation();
  return <p>{`${location.pathname}${location.search}`}</p>;
}

function renderPalette() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher", element: <CommandPalette open onOpenChange={() => {}} /> },
      { path: "*", element: <Where /> },
    ],
    { initialEntries: ["/teacher"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

function list() {
  return within(screen.getByRole("listbox", { name: "Tìm kiếm và lệnh" }));
}

function groupNames() {
  return list()
    .getAllByRole("group")
    .map((group) => group.getAttribute("aria-labelledby"))
    .map((id) => document.getElementById(id ?? "")?.textContent);
}

describe("the command palette as it opens", () => {
  it("offers the pages, then recent assignments and students with their hints", async () => {
    renderPalette();

    expect(screen.getByRole("combobox")).toHaveAttribute(
      "placeholder",
      "Tìm trang, đề thi, học viên…",
    );
    const assignmentOption = await screen.findByRole(
      "option",
      {
        name: /Present perfect/,
      },
      LOAD,
    );
    expect(assignmentOption).toHaveTextContent("Đang mở");
    expect(
      await screen.findByRole("option", { name: /Phạm Gia Hân/ }, LOAD),
    ).toHaveTextContent("IELTS Foundation");
    expect(groupNames()).toEqual(["Trang", "Bài giao", "Học viên"]);
    expect(
      within(list().getByRole("group", { name: "Trang" }))
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual([
      "Tổng quan",
      "Bài giao",
      "Chấm bài",
      "Đề thi",
      "Ngân hàng câu hỏi",
      "Học viên",
      "Lớp học",
      "Thư viện media",
      "Cài đặt",
    ]);
    expect(asked.some(({ path }) => path === "/teacher/tests")).toBe(false);
  });

  it("leaves out a page and a group the user cannot open", async () => {
    grant(["content.tests.write", "teaching.assignments.write"]);
    renderPalette();
    await screen.findByRole("option", { name: /Present perfect/ }, LOAD);

    const pages = within(list().getByRole("group", { name: "Trang" }))
      .getAllByRole("option")
      .map((option) => option.textContent);
    expect(pages).not.toContain("Chấm bài");
    expect(pages).not.toContain("Học viên");
    expect(list().queryByRole("group", { name: "Học viên" })).toBeNull();
    expect(asked.some(({ path }) => path === "/teacher/students")).toBe(false);
  });
});

describe("searching", () => {
  it("matches pages without accents and asks the server for the rest", async () => {
    const user = renderPalette();
    await user.type(screen.getByRole("combobox"), "hoc vien");

    await waitFor(() =>
      expect(
        asked
          .filter(({ q }) => q === "hoc vien")
          .map(({ path }) => path)
          .sort((a, b) => a.localeCompare(b)),
      ).toEqual([
        "/teacher/assignments",
        "/teacher/questions",
        "/teacher/students",
        "/teacher/tests",
      ]),
    );
    expect(
      within(list().getByRole("group", { name: "Trang" }))
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["Học viên"]);
    expect(
      await screen.findByRole("option", { name: /Người phụ nữ/ }, LOAD),
    ).toBeVisible();
    expect(groupNames()).toEqual([
      "Trang",
      "Bài giao",
      "Học viên",
      "Đề thi",
      "Câu hỏi",
    ]);
  });

  it("asks only once the typing pauses", async () => {
    const user = renderPalette();
    await screen.findByRole("option", { name: /Present perfect/ }, LOAD);
    asked = [];
    await user.type(screen.getByRole("combobox"), "unit");

    await waitFor(() => expect(asked.length).toBeGreaterThan(0));
    expect(new Set(asked.map(({ q }) => q))).toEqual(new Set(["unit"]));
  });

  it("cancels an older search and never lets its answer replace a newer one", async () => {
    let aborted = false;
    let releaseFirst: () => void = () => undefined;
    const first = new Promise<void>((resolve) => {
      releaseFirst = resolve;
    });
    server.use(
      http.get(`${BASE}/teacher/tests`, async ({ request }) => {
        const q = new URL(request.url).searchParams.get("q");
        if (q === "unit") {
          request.signal.addEventListener("abort", () => {
            aborted = true;
          });
          await first;
        }
        const item = { ...test(), title: q === "unit" ? "Old answer" : "New answer" };
        return contractJson("/teacher/tests", "get", 200, {
          facets: { all: 1, draft: 0, published: 1, archived: 0 },
          tags: [],
          items: [item],
          page: 1,
          pageSize: 4,
          total: 1,
        });
      }),
    );
    const user = renderPalette();
    const input = screen.getByRole("combobox");
    await user.type(input, "unit");
    await waitFor(() => expect(asked.some(({ q }) => q === "unit")).toBe(true));
    await user.type(input, "s");

    expect(
      await screen.findByRole("option", { name: /New answer/ }, LOAD),
    ).toBeVisible();
    await waitFor(() => expect(aborted).toBe(true));
    releaseFirst();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(screen.queryByRole("option", { name: /Old answer/ })).toBeNull();
    expect(screen.getByRole("option", { name: /New answer/ })).toBeVisible();
  });

  it("says nothing matches when every group comes back empty", async () => {
    matching = false;
    const user = renderPalette();
    await user.type(screen.getByRole("combobox"), "zzzz");

    expect(await screen.findByText("Không tìm thấy gì khớp.")).toBeVisible();
    expect(screen.queryAllByRole("option")).toHaveLength(0);
    expect(screen.getByRole("combobox")).toHaveAttribute("aria-expanded", "false");
  });
});

describe("the command palette, from the keyboard alone", () => {
  it("moves aria-activedescendant with the arrows, wraps, and opens what it names", async () => {
    const user = renderPalette();
    const input = screen.getByRole("combobox");
    await user.type(input, "unit");
    await screen.findByRole("option", { name: /Người phụ nữ/ }, LOAD);

    const options = screen.getAllByRole("option");
    expect(input).toHaveAttribute("aria-activedescendant", options[0]?.id);
    await user.keyboard("{ArrowUp}");
    expect(input).toHaveAttribute("aria-activedescendant", options.at(-1)?.id);
    expect(options.at(-1)).toHaveAttribute("aria-selected", "true");
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(input).toHaveAttribute("aria-activedescendant", options[1]?.id);

    await user.keyboard("{ArrowUp}{Enter}");
    expect(
      await screen.findByText(`/teacher/assignments/${ASSIGNMENT_ID}`),
    ).toBeVisible();
  });

  it("opens the students list searched for a student", async () => {
    const user = renderPalette();
    await user.click(await screen.findByRole("option", { name: /Phạm Gia Hân/ }, LOAD));

    expect(
      await screen.findByText(
        `/teacher/students?q=${encodeURIComponent("Phạm Gia Hân")}`,
      ),
    ).toBeVisible();
  });
});
