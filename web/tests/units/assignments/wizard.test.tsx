import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentWizardPage from "@/features/assignments/pages/teacher/AssignmentWizardPage";
import { draftBody, draftOf, emptyDraft } from "@/features/assignments/draft";
import { Toaster } from "@/components/ui/sonner";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const OTHER_TEST_ID = "018f0000-0000-7000-8000-0000000000a2";
const VERSION_ID = "018f0000-0000-7000-8000-0000000000f1";
const OTHER_VERSION_ID = "018f0000-0000-7000-8000-0000000000f2";
const CLASS_A = "018f0000-0000-7000-8000-0000000000c1";
const CLASS_B = "018f0000-0000-7000-8000-0000000000c2";
const AN = "018f0000-0000-7000-8000-0000000000e1";
const BINH = "018f0000-0000-7000-8000-0000000000e2";
const CHI = "018f0000-0000-7000-8000-0000000000e3";
const CREATED = "018f0000-0000-7000-8000-0000000000d1";
const NOW = "2026-09-01T00:00:00Z";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

type Assignment = components["schemas"]["Assignment"];

function publishedTest(id: string, title: string, skills: string[]) {
  return {
    id,
    title,
    description: null,
    status: "published",
    currentVersion: 3,
    totalPoints: 30,
    questionCount: 99,
    audioCount: 0,
    skills,
    assignments: { live: 0, scheduled: 0, closed: 0 },
    unpublishedChanges: null,
    sections: [],
    createdAt: NOW,
    updatedAt: NOW,
  };
}

function version(id: string, manualCount: number) {
  return {
    id,
    version: 3,
    totalPoints: 30,
    questionCount: 24,
    audioCount: 0,
    manualCount,
    assignmentCount: 0,
    changeNote: null,
    publishedAt: NOW,
    publishedBy: "Thương",
  };
}

function klass(id: string, name: string, studentCount: number) {
  return {
    id,
    name,
    description: null,
    studentCount,
    openAssignmentCount: 0,
    selfJoinEnabled: false,
    archivedAt: null,
    createdAt: NOW,
  };
}

const stats = { submittedCount: 0, flaggedCount: 0, activity: { live: false } };

function member(userId: string, fullName: string) {
  return {
    userId,
    fullName,
    email: `${userId}@example.com`,
    joinedVia: "admin",
    joinedAt: NOW,
    joinCodeHint: null,
    stats,
  };
}

function student(id: string, fullName: string, classIds: string[]) {
  return {
    id,
    email: `${id}@example.com`,
    fullName,
    hasPassword: true,
    linkedProviders: [],
    mustChangePassword: false,
    createdAt: NOW,
    disabledAt: null,
    classes: classIds.map((classId) => ({
      id: classId,
      name: classId === CLASS_A ? "IELTS Foundation" : "TOEIC 600",
      joinedVia: "admin",
      joinedAt: NOW,
    })),
    stats,
  };
}

const TESTS = [
  publishedTest(TEST_ID, "Unit 5 Reading", ["reading"]),
  publishedTest(OTHER_TEST_ID, "Mock B", ["grammar"]),
];
const VERSIONS: Record<string, ReturnType<typeof version>> = {
  [TEST_ID]: version(VERSION_ID, 2),
  [OTHER_TEST_ID]: version(OTHER_VERSION_ID, 0),
};
const CLASSES = [klass(CLASS_A, "IELTS Foundation", 2), klass(CLASS_B, "TOEIC 600", 1)];
const MEMBERS: Record<string, ReturnType<typeof member>[]> = {
  [CLASS_A]: [member(AN, "Nguyễn An"), member(BINH, "Trần Bình")],
  [CLASS_B]: [member(AN, "Nguyễn An")],
};
const STUDENTS = [
  student(AN, "Nguyễn An", [CLASS_A, CLASS_B]),
  student(BINH, "Trần Bình", [CLASS_A]),
  student(CHI, "Lê Chi", []),
];

let posted: Record<string, unknown>[] = [];

beforeEach(() => {
  posted = [];
  server.use(
    http.get(`${BASE}/teacher/tests`, ({ request }) => {
      const q = new URL(request.url).searchParams.get("q") ?? "";
      const items = TESTS.filter((test) =>
        test.title.toLowerCase().includes(q.toLowerCase()),
      );
      return contractJson("/teacher/tests", "get", 200, {
        items,
        page: 1,
        pageSize: 20,
        total: items.length,
        facets: { all: 2, draft: 0, published: 2, archived: 0 },
        tags: [],
      });
    }),
    http.get(`${BASE}/teacher/tests/:id`, ({ params }) =>
      contractJson(
        "/teacher/tests/{id}",
        "get",
        200,
        TESTS.find((test) => test.id === params["id"]),
      ),
    ),
    http.get(`${BASE}/teacher/tests/:id/versions`, ({ params }) =>
      contractJson("/teacher/tests/{id}/versions", "get", 200, {
        items: [VERSIONS[String(params["id"])]],
      }),
    ),
    http.get(`${BASE}/teacher/classes`, () =>
      contractJson("/teacher/classes", "get", 200, {
        items: CLASSES,
        page: 1,
        pageSize: 100,
        total: CLASSES.length,
        facets: { all: 2, joinable: 0, archived: 0, students: 3 },
      }),
    ),
    http.get(`${BASE}/teacher/classes/:id`, ({ params }) =>
      contractJson(
        "/teacher/classes/{id}",
        "get",
        200,
        CLASSES.find((item) => item.id === params["id"]),
      ),
    ),
    http.get(`${BASE}/teacher/classes/:id/members`, ({ params }) => {
      const items = MEMBERS[String(params["id"])] ?? [];
      return contractJson("/teacher/classes/{id}/members", "get", 200, {
        items,
        page: 1,
        pageSize: 100,
        total: items.length,
      });
    }),
    http.get(`${BASE}/teacher/students`, () =>
      contractJson("/teacher/students", "get", 200, {
        items: STUDENTS,
        page: 1,
        pageSize: 100,
        total: STUDENTS.length,
        facets: { total: STUDENTS.length, activeLast7Days: 0 },
      }),
    ),
    http.get(`${BASE}/teacher/students/:id`, ({ params }) =>
      contractJson(
        "/teacher/students/{id}",
        "get",
        200,
        STUDENTS.find((item) => item.id === params["id"]),
      ),
    ),
    http.post(`${BASE}/teacher/assignments`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      posted.push(body);
      return contractJson("/teacher/assignments", "post", 201, stored(body));
    }),
  );
});

function stored(body: Record<string, unknown>): Assignment {
  const input = body as unknown as components["schemas"]["AssignmentInput"];
  return {
    id: CREATED,
    testId: TEST_ID,
    testVersionId: input.testVersionId,
    testVersion: 3,
    testTitle: "Unit 5 Reading",
    targets: {
      classes: CLASSES.filter((item) => input.targets.classIds.includes(item.id)).map(
        (item) => ({ id: item.id, name: item.name, studentCount: item.studentCount }),
      ),
      students: STUDENTS.filter((item) =>
        input.targets.studentIds.includes(item.id),
      ).map((item) => ({ id: item.id, name: item.fullName })),
    },
    publishedAt: null,
    updatedAt: NOW,
    window: { ...input.window, closedAt: null },
    durationMinutes: input.durationMinutes,
    maxAttempts: input.maxAttempts,
    shuffleQuestions: input.shuffleQuestions,
    shuffleOptions: input.shuffleOptions,
    review: { release: "on_submit", showClassAverage: false, ...input.review },
    studentNote: null,
    integrity: input.integrity,
    status: "draft",
  };
}

function renderWizard(entry = "/teacher/assignments/new") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/assignments/new", element: <AssignmentWizardPage /> },
      { path: "/teacher/assignments", element: <p>assignments list</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return router;
}

describe("the new assignment wizard", () => {
  it("opens on the Test step with the deck's header and a disabled Schedule and Rules", async () => {
    const router = renderWizard();
    expect(
      screen.getByRole("heading", { level: 1, name: "Giao bài mới" }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Chưa có gì được gửi đi cho tới khi bạn bấm Giao bài. Mặc định là lựa chọn an toàn.",
      ),
    ).toBeVisible();
    const steps = screen.getByRole("list", { name: "Các bước giao bài" });
    const buttons = within(steps).getAllByRole("button");
    expect(buttons).toHaveLength(4);
    expect(buttons[0]).toHaveAttribute("aria-current", "step");
    expect(buttons[2]).toBeDisabled();
    expect(buttons[3]).toBeDisabled();
    expect(buttons[0]).toHaveTextContent("Chưa chọn đề");
    expect(buttons[3]).toHaveTextContent("Chặn sao chép/dán · Hiện điểm");
    const row = await screen.findByRole("radio", { name: /Unit 5 Reading/ });
    await waitFor(() =>
      expect(row).toHaveTextContent("Đọc · 24 câu · 2 câu cần chấm tay"),
    );
    await userEvent.setup().click(screen.getByRole("button", { name: "Tiếp tục" }));
    expect(router.state.location.search).toBe("?step=2");
    expect(screen.getByRole("button", { name: "Tiếp tục" })).toBeDisabled();
    expect(
      screen.getByText("Lịch và quy định sẽ có trong bản cập nhật tới."),
    ).toBeVisible();
  });

  it("preselects the test and the class named in the URL without counting it as a change", async () => {
    const user = userEvent.setup();
    const router = renderWizard(
      `/teacher/assignments/new?test=${TEST_ID}&class=${CLASS_A}&step=2`,
    );
    expect(
      await screen.findByRole("checkbox", { name: /IELTS Foundation/ }),
    ).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("checkbox", { name: /TOEIC 600/ })).toHaveAttribute(
      "aria-checked",
      "false",
    );
    expect(await screen.findByText("2 học viên sẽ nhận bài này")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Quay lại" }));
    expect(await screen.findByRole("radio", { name: /Unit 5 Reading/ })).toBeChecked();
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/assignments"),
    );
  });

  it("counts a student once across classes and individual picks, and names the class that includes them", async () => {
    const user = userEvent.setup();
    renderWizard("/teacher/assignments/new?step=2");
    await user.click(await screen.findByRole("checkbox", { name: /IELTS Foundation/ }));
    await user.click(screen.getByRole("checkbox", { name: /TOEIC 600/ }));
    expect(await screen.findByText("2 học viên sẽ nhận bài này")).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Thêm từng học viên" }));
    const dialog = await screen.findByRole("dialog");
    const an = await within(dialog).findByRole("checkbox", { name: /Nguyễn An/ });
    expect(an).toHaveTextContent(
      "IELTS Foundation, TOEIC 600 · đã có trong lớp đã chọn",
    );
    expect(within(dialog).getByRole("checkbox", { name: /Lê Chi/ })).toHaveTextContent(
      "Lê Chi",
    );
    await user.click(an);
    await user.click(within(dialog).getByRole("checkbox", { name: /Lê Chi/ }));
    await user.click(within(dialog).getByRole("button", { name: "Thêm 2" }));

    const panel = await screen.findByRole("region", { name: /Học viên thêm lẻ/ });
    expect(within(panel).getByText("Đã có trong lớp IELTS Foundation")).toBeVisible();
    expect(within(panel).getByText("Chưa vào lớp nào")).toBeVisible();
    expect(
      await screen.findByText("3 học viên sẽ nhận bài này · 2 từ các lớp, 1 thêm lẻ"),
    ).toBeVisible();

    await user.click(within(panel).getByRole("button", { name: "Bỏ tất cả" }));
    expect(screen.queryByRole("region", { name: /Học viên thêm lẻ/ })).toBeNull();
    expect(await screen.findByText("2 học viên sẽ nhận bài này")).toBeVisible();
  });

  it("saves a draft with every field and opens the Drafts tab", async () => {
    const user = userEvent.setup();
    const router = renderWizard();
    await user.click(screen.getByRole("button", { name: "Lưu nháp" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Chọn một đề trước khi lưu nháp.",
    );
    expect(posted).toEqual([]);

    await user.click(await screen.findByRole("radio", { name: /Unit 5 Reading/ }));
    await user.click(screen.getByRole("button", { name: "Tiếp tục" }));
    await user.click(await screen.findByRole("checkbox", { name: /TOEIC 600/ }));
    await user.click(screen.getByRole("button", { name: "Lưu nháp" }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/assignments"),
    );
    expect(router.state.location.search).toBe("?status=draft");
    expect(
      await screen.findByText("Đã lưu nháp. Học viên chưa thấy bài này."),
    ).toBeVisible();
    expect(posted).toHaveLength(1);
    expect(posted[0]).toMatchObject({
      draft: true,
      testVersionId: VERSION_ID,
      targets: { classIds: [CLASS_B], studentIds: [] },
      durationMinutes: 45,
      maxAttempts: 1,
      shuffleQuestions: false,
      shuffleOptions: false,
      review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
      integrity: {
        requireFullscreen: false,
        blockCopyPaste: true,
        maxFocusLoss: 0,
        onLimitExceeded: "flag",
        minAwayMs: 3000,
      },
    });
    expect(Object.keys(posted[0] ?? {}).sort((a, b) => a.localeCompare(b))).toEqual([
      "draft",
      "durationMinutes",
      "integrity",
      "maxAttempts",
      "review",
      "shuffleOptions",
      "shuffleQuestions",
      "targets",
      "testVersionId",
      "window",
    ]);
  });

  it("asks before leaving with unsaved choices, and stays when told to", async () => {
    const user = userEvent.setup();
    const router = renderWizard();
    await user.click(await screen.findByRole("radio", { name: /Mock B/ }));
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Rời trang mà không lưu?",
    });
    await user.click(within(dialog).getByRole("button", { name: "Ở lại" }));
    expect(router.state.location.pathname).toBe("/teacher/assignments/new");
    expect(screen.getByRole("radio", { name: /Mock B/ })).toBeChecked();

    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Rời trang",
      }),
    );
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/assignments"),
    );
  });
});

describe("the assignment draft", () => {
  it("round-trips every field through the request body and the stored assignment", () => {
    const draft = {
      ...emptyDraft(new Date("2026-09-01T03:20:00Z")),
      picked: {
        testId: TEST_ID,
        testTitle: "Unit 5 Reading",
        version: { ...version(VERSION_ID, 2), testUpdatedAt: NOW },
      },
      classes: [{ id: CLASS_A, label: "IELTS Foundation", hint: "2" }],
      students: [{ id: CHI, label: "Lê Chi" }],
      durationMinutes: 65,
      maxAttempts: 2,
      shuffleOptions: true,
      integrity: {
        requireFullscreen: true,
        blockCopyPaste: false,
        maxFocusLoss: -1,
        onLimitExceeded: "auto_submit" as const,
        minAwayMs: 5000,
      },
    };
    const back = draftOf(stored({ ...draftBody(draft), draft: true }), [
      draft.picked.version,
    ]);
    expect(back).toEqual({
      ...draft,
      review: { ...draft.review, release: "on_submit", showClassAverage: false },
    });
    expect(draftBody(back)).toEqual({
      ...draftBody(draft),
      review: { ...draft.review, release: "on_submit", showClassAverage: false },
    });
  });
});
