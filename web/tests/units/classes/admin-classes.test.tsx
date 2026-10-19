import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import AdminClassesPage from "@/features/classes/pages/admin/AdminClassesPage";
import type { AdminClass } from "@/features/classes/adminApi";
import i18n from "@/lib/i18n";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contentWidth } from "@tests/support/contentWidth";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";

const BASE = "http://localhost:8080";

const GIAU = "018f0000-0000-7000-8000-0000000000a1";
const DUNG = "018f0000-0000-7000-8000-0000000000a2";
const MINH = "018f0000-0000-7000-8000-0000000000a3";

const MORNING: AdminClass = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "IELTS 7.0 Sáng",
  scheduleLabel: "T2 · T4, 09:00",
  room: null,
  teacher: { id: GIAU, name: "Giàu Bùi", avatarUrl: null, disabled: false },
  studentCount: 1240,
  joinCode: {
    code: "K7QM-2PXA",
    expiresAt: "2099-01-01T00:00:00Z",
    maxUses: 40,
    usesCount: 3,
  },
  createdAt: "2026-06-12T03:00:00Z",
};
const ORPHANED: AdminClass = {
  ...MORNING,
  id: "018f0000-0000-7000-8000-0000000000c2",
  name: "Kids Starters B",
  scheduleLabel: null,
  teacher: { id: DUNG, name: "Vũ Thuỳ Dung", avatarUrl: null, disabled: true },
  studentCount: 12,
  joinCode: null,
};
const SUMMER: AdminClass = {
  ...MORNING,
  id: "018f0000-0000-7000-8000-0000000000c3",
  name: "Hè 2025",
};

type UserRow = components["schemas"]["AdminUserRow"];

function staff(
  id: string,
  fullName: string,
  status: UserRow["status"] = "active",
): UserRow {
  return {
    id,
    fullName,
    email: `${id.slice(-2)}@example.com`,
    avatarUrl: null,
    role: {
      id: "019535d8-0000-7000-8000-000000000002",
      name: "Giáo viên",
      builtinKey: "teacher",
      icon: "graduation-cap",
      color: "lime",
    },
    signIn: { password: true, google: false },
    classes: { count: 1, firstName: null },
    lastActiveAt: null,
    status,
    createdAt: "2026-01-01T00:00:00Z",
  };
}

let lists: URLSearchParams[] = [];
let creates: unknown[] = [];
let changes: { id: string; body: unknown }[] = [];
let patches: { id: string; body: unknown }[] = [];
let rotations: string[] = [];
let rotationBodies: unknown[] = [];

function serveClasses(active: AdminClass[], archived: AdminClass[] = [SUMMER]) {
  server.use(
    http.get(`${BASE}/admin/classes`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      lists.push(query);
      const disabled = query.get("teacher") === "disabled";
      const keep = (rows: AdminClass[]) =>
        disabled ? rows.filter((row) => row.teacher.disabled) : rows;
      const items = keep(query.get("tab") === "archived" ? archived : active);
      return contractJson("/admin/classes", "get", 200, {
        items,
        page: 1,
        pageSize: Number(query.get("limit") ?? 20),
        total: items.length,
        facets: { active: keep(active).length, archived: keep(archived).length },
        disabledTeacherClasses: active.filter((row) => row.teacher.disabled).length,
      });
    }),
  );
}

function serveJoinCodes(joinCodes: boolean) {
  server.use(
    http.get(`${BASE}/public/organization`, () =>
      contractJson("/public/organization", "get", 200, {
        name: "Quizzivy",
        shortName: null,
        website: null,
        contactEmail: null,
        frontDeskPhone: null,
        frontDeskHours: null,
        address: null,
        logo: { lightUrl: null, darkUrl: null },
        signIn: { google: true, password: true },
        joinCodes,
      }),
    ),
  );
}

function classBody(klass: AdminClass) {
  return {
    id: klass.id,
    name: klass.name,
    description: null,
    scheduleLabel: klass.scheduleLabel,
    room: null,
    studentCount: klass.studentCount,
    openAssignmentCount: 0,
    selfJoinEnabled: true,
    archivedAt: null,
    joinCode: null,
    createdAt: klass.createdAt,
  };
}

beforeEach(() => {
  lists = [];
  creates = [];
  changes = [];
  patches = [];
  rotations = [];
  rotationBodies = [];
  serveClasses([MORNING, ORPHANED]);
  serveJoinCodes(true);
  server.use(
    http.get(`${BASE}/admin/users`, ({ request }) => {
      const tab = new URL(request.url).searchParams.get("tab");
      const items =
        tab === "teachers"
          ? [staff(GIAU, "Giàu Bùi"), staff(DUNG, "Vũ Thuỳ Dung", "disabled")]
          : [staff(MINH, "Minh Trần")];
      return contractJson("/admin/users", "get", 200, {
        items,
        page: 1,
        pageSize: 50,
        total: items.length,
        facets: { all: 3, teachers: 2, students: 0, admins: 1, disabled: 1 },
      });
    }),
    http.post(`${BASE}/admin/classes`, async ({ request }) => {
      creates.push(await request.json());
      return contractJson("/admin/classes", "post", 201, classBody(MORNING));
    }),
    http.put(`${BASE}/admin/classes/:id/teacher`, async ({ request, params }) => {
      changes.push({ id: String(params["id"]), body: await request.json() });
      return contractJson(
        "/admin/classes/{id}/teacher",
        "put",
        200,
        classBody(MORNING),
      );
    }),
    http.patch(`${BASE}/teacher/classes/:id`, async ({ request, params }) => {
      patches.push({ id: String(params["id"]), body: await request.json() });
      return contractJson("/teacher/classes/{id}", "patch", 200, classBody(MORNING));
    }),
    http.post(`${BASE}/teacher/classes/:id/join-code`, async ({ request, params }) => {
      rotations.push(String(params["id"]));
      rotationBodies.push(await request.json());
      return contractJson("/teacher/classes/{id}/join-code", "post", 201, {
        code: "Z9Z9-Y8Y8",
        expiresAt: "2099-01-01T00:00:00Z",
        maxUses: 40,
      });
    }),
  );
});

afterEach(async () => {
  useAuthStore.getState().clearSession();
  await act(() => i18n.changeLanguage("vi"));
});

function renderPage(path = "/admin/classes", width = 1280) {
  useAuthStore.getState().setSession("token", adminUser);
  contentWidth(width);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [{ path: "/admin/classes", element: <AdminClassesPage /> }],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup({ delay: null }), router };
}

async function table() {
  return screen.findByRole("table", { name: "Lớp học" });
}

async function rowOf(name: string) {
  return within(await table()).findByRole("row", { name: new RegExp(name) });
}

async function headers() {
  return within(await table())
    .getAllByRole("columnheader")
    .map((cell) => cell.textContent);
}

async function openMenu(user: ReturnType<typeof userEvent.setup>, name: string) {
  const row = await rowOf(name);
  await user.click(
    within(row).getByRole("button", { name: `Thao tác với lớp ${name}` }),
  );
  return screen.findByRole("menu");
}

describe("the Admin Classes page", () => {
  it("says what it is, offers New class, and counts both tabs", async () => {
    renderPage();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Lớp học" }),
    ).toBeVisible();
    expect(screen.getByText("Mọi lớp của mọi giáo viên.")).toBeVisible();
    expect(screen.getByRole("button", { name: "Lớp mới" })).toBeVisible();
    expect(screen.getByPlaceholderText("Tìm lớp hoặc giáo viên")).toBeVisible();
    const tabs = screen.getByRole("group", { name: "Lọc theo trạng thái" });
    expect(
      await within(tabs).findByRole("button", { name: /Đang hoạt động\s*2/ }),
    ).toBeVisible();
    expect(within(tabs).getByRole("button", { name: /Đã lưu trữ\s*1/ })).toBeVisible();
  });

  it("draws every column on a wide page, the code in full and the month it was made", async () => {
    renderPage();
    expect(await headers()).toEqual([
      "Lớp",
      "Giáo viên",
      "Học viên",
      "Mã tham gia",
      "Ngày tạo",
      "Thao tác",
    ]);
    const row = await rowOf(MORNING.name);
    expect(within(row).getByText("K7QM-2PXA")).toBeVisible();
    expect(within(row).getByText("Th6 2026")).toBeVisible();
    expect(within(row).getByText("1.240")).toBeVisible();
    expect(within(row).getByText("T2 · T4, 09:00")).toBeVisible();
    const orphan = await rowOf(ORPHANED.name);
    expect(within(orphan).getByText("Đã tắt")).toBeVisible();
    expect(within(orphan).getByText("Chưa có mã")).toBeVisible();
    expect(within(orphan).getByText("Chưa có lịch học")).toBeVisible();
    expect(within(row).queryByRole("link")).toBeNull();
  });

  it.each([
    [899, ["Lớp", "Giáo viên", "Học viên", "Mã tham gia", "Thao tác"]],
    [900, ["Lớp", "Giáo viên", "Học viên", "Mã tham gia", "Ngày tạo", "Thao tác"]],
    [759, ["Lớp", "Giáo viên", "Học viên", "Thao tác"]],
    [760, ["Lớp", "Giáo viên", "Học viên", "Mã tham gia", "Thao tác"]],
    [599, ["Lớp", "Học viên", "Thao tác"]],
    [600, ["Lớp", "Giáo viên", "Học viên", "Thao tác"]],
  ])("draws the columns a content width of %i takes", async (width, expected) => {
    renderPage("/admin/classes", width);
    expect(await headers()).toEqual(expected);
  });

  it("folds the teacher into the class's sub-line on a narrow page", async () => {
    renderPage("/admin/classes", 560);
    const row = await rowOf(MORNING.name);
    expect(within(row).getByText("Giàu Bùi · T2 · T4, 09:00")).toBeVisible();
    const orphan = await rowOf(ORPHANED.name);
    expect(
      within(orphan).getByText("Vũ Thuỳ Dung (đã tắt) · Chưa có lịch học"),
    ).toBeVisible();
  });

  it("shows a skeleton first, then an error with Retry", async () => {
    let healthy = false;
    server.use(
      http.get(`${BASE}/admin/classes`, () => {
        if (healthy) return undefined;
        return HttpResponse.json(
          { error: { code: "INTERNAL", message: "x" } },
          { status: 500 },
        );
      }),
    );
    const { user } = renderPage();
    expect(screen.getByRole("status", { name: "Đang tải…" })).toBeInTheDocument();
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText("Không tải được danh sách lớp.")).toBeVisible();
    healthy = true;
    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));
    expect(await rowOf(MORNING.name)).toBeVisible();
  });

  it("says each tab is empty in its own words", async () => {
    serveClasses([], []);
    const { user } = renderPage();
    expect(await screen.findByText("Chưa có lớp nào đang hoạt động.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: /Đã lưu trữ/ }));
    expect(await screen.findByText("Không có lớp nào đã lưu trữ.")).toBeVisible();
  });

  it("sends the search and the tab, and the filter the Overview links to as a removable chip", async () => {
    const { user, router } = renderPage("/admin/classes?teacher=disabled");
    expect(await rowOf(ORPHANED.name)).toBeVisible();
    expect(screen.queryByRole("row", { name: new RegExp(MORNING.name) })).toBeNull();
    expect(lists.at(-1)?.get("teacher")).toBe("disabled");
    const chips = screen.getByRole("group", { name: "Bộ lọc đang dùng" });
    await user.click(
      within(chips).getByRole("button", { name: "Bỏ bộ lọc Giáo viên đã tắt" }),
    );
    await waitFor(() => expect(router.state.location.search).toBe(""));
    expect(await rowOf(MORNING.name)).toBeVisible();
    await user.type(screen.getByPlaceholderText("Tìm lớp hoặc giáo viên"), "giau");
    await waitFor(() => expect(lists.at(-1)?.get("q")).toBe("giau"));
    await user.click(screen.getByRole("button", { name: /Đã lưu trữ/ }));
    await waitFor(() => expect(lists.at(-1)?.get("tab")).toBe("archived"));
  });
});

describe("a class's menu", () => {
  it("offers Edit, Change teacher, Rotate and Archive on the Active tab", async () => {
    const { user } = renderPage();
    const menu = await openMenu(user, MORNING.name);
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent),
    ).toEqual(["Sửa lớp", "Đổi giáo viên", "Đổi mã tham gia", "Lưu trữ lớp"]);
  });

  it("offers Create code for a class without one", async () => {
    const { user } = renderPage();
    const menu = await openMenu(user, ORPHANED.name);
    expect(within(menu).getByRole("menuitem", { name: "Tạo mã" })).toBeVisible();
  });

  it("offers Edit, Change teacher and Restore on the Archived tab, without the code's rotation", async () => {
    const { user } = renderPage("/admin/classes?tab=archived");
    const row = await rowOf(SUMMER.name);
    expect(within(row).queryByRole("button", { name: /Đổi mã tham gia/ })).toBeNull();
    expect(within(row).queryByRole("button", { name: /Sao chép/ })).toBeNull();
    expect(within(row).queryByText("K7QM-2PXA")).toBeNull();
    expect(within(row).getByText("—")).toBeInTheDocument();
    const menu = await openMenu(user, SUMMER.name);
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent),
    ).toEqual(["Sửa lớp", "Đổi giáo viên", "Khôi phục lớp"]);
    await user.click(within(menu).getByRole("menuitem", { name: "Khôi phục lớp" }));
    await waitFor(() =>
      expect(patches).toEqual([{ id: SUMMER.id, body: { archived: false } }]),
    );
    expect(await screen.findByText("Đã khôi phục lớp")).toBeVisible();
  });

  it("rotates a code after the deck's confirmation and toasts the new one", async () => {
    const { user } = renderPage();
    const row = await rowOf(MORNING.name);
    await user.click(
      within(row).getByRole("button", { name: `Đổi mã tham gia của ${MORNING.name}` }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: `Đổi mã tham gia của ${MORNING.name}?`,
    });
    expect(
      within(dialog).getByText(
        "Mã và liên kết cũ ngừng hoạt động ngay. Học viên đã ở trong lớp không bị ảnh hưởng.",
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Đổi mã" }));
    await waitFor(() => expect(rotations).toEqual([MORNING.id]));
    expect(rotationBodies).toEqual([{ expiresInDays: 30, maxUses: 40 }]);
    expect(await screen.findByText("Mã tham gia mới: Z9Z9-Y8Y8")).toBeVisible();
  });

  it("archives a class after asking", async () => {
    const { user } = renderPage();
    const menu = await openMenu(user, MORNING.name);
    await user.click(within(menu).getByRole("menuitem", { name: "Lưu trữ lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: `Lưu trữ ${MORNING.name}?`,
    });
    expect(
      within(dialog).getByText(
        "Học viên không thể tham gia hay bắt đầu bài nữa. Kết quả được giữ lại.",
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Lưu trữ" }));
    await waitFor(() =>
      expect(patches).toEqual([{ id: MORNING.id, body: { archived: true } }]),
    );
    await waitFor(() =>
      expect(screen.getByPlaceholderText("Tìm lớp hoặc giáo viên")).toHaveFocus(),
    );
  });

  it("reads the list again when the server refuses an archive", async () => {
    server.use(
      http.patch(`${BASE}/teacher/classes/:id`, () =>
        HttpResponse.json(
          { error: { code: "NOT_FOUND", message: "Không tìm thấy lớp." } },
          { status: 404 },
        ),
      ),
    );
    const { user } = renderPage();
    const menu = await openMenu(user, MORNING.name);
    await user.click(within(menu).getByRole("menuitem", { name: "Lưu trữ lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: `Lưu trữ ${MORNING.name}?`,
    });
    const before = lists.length;
    await user.click(within(dialog).getByRole("button", { name: "Lưu trữ" }));
    expect(await within(dialog).findByText("Không tìm thấy lớp.")).toBeVisible();
    await waitFor(() => expect(lists.length).toBeGreaterThan(before));
  });

  it("keeps the shown page's menu while the next tab loads", async () => {
    let release: () => void = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    const { user } = renderPage();
    await rowOf(MORNING.name);
    server.use(
      http.get(`${BASE}/admin/classes`, async ({ request }) => {
        if (new URL(request.url).searchParams.get("tab") === "archived") await held;
        return undefined;
      }),
    );
    await user.click(screen.getByRole("button", { name: /Đã lưu trữ/ }));
    const menu = await openMenu(user, MORNING.name);
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent),
    ).toEqual(["Sửa lớp", "Đổi giáo viên", "Đổi mã tham gia", "Lưu trữ lớp"]);
    release();
  });
});

describe("the class dialogs", () => {
  it("creates a class for an active teacher, leaving out a disabled one", async () => {
    const { user } = renderPage();
    await user.click(await screen.findByRole("button", { name: "Lớp mới" }));
    const dialog = await screen.findByRole("dialog", { name: "Lớp mới" });
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên lớp/ }),
      "IELTS 8.0",
    );
    await user.click(
      await within(dialog).findByRole("combobox", { name: /Giáo viên/ }),
    );
    const options = await screen.findAllByRole("option");
    expect(options.map((option) => option.textContent)).toEqual(
      expect.arrayContaining([
        expect.stringMatching(/Giàu Bùi/),
        expect.stringMatching(/Minh Trần/),
      ]),
    );
    expect(screen.queryByRole("option", { name: /Vũ Thuỳ Dung/ })).toBeNull();
    await user.click(screen.getByRole("option", { name: /Minh Trần/ }));
    await user.type(
      within(dialog).getByRole("textbox", { name: /Lịch học/ }),
      "T3 · T5",
    );
    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));
    await waitFor(() =>
      expect(creates).toEqual([
        { name: "IELTS 8.0", teacherId: MINH, scheduleLabel: "T3 · T5" },
      ]),
    );
    expect(await screen.findByText("Đã tạo lớp")).toBeVisible();
  });

  it("edits the name and the schedule, the teacher read-only with Change teacher beside it", async () => {
    const { user } = renderPage();
    const menu = await openMenu(user, MORNING.name);
    await user.click(within(menu).getByRole("menuitem", { name: "Sửa lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Sửa lớp" });
    expect(within(dialog).queryByRole("combobox")).toBeNull();
    expect(within(dialog).getByText("Giàu Bùi")).toBeVisible();
    const name = within(dialog).getByRole("textbox", { name: /Tên lớp/ });
    await user.clear(name);
    await user.type(name, "IELTS 7.5 Sáng");
    await user.click(within(dialog).getByRole("button", { name: "Lưu thay đổi" }));
    await waitFor(() =>
      expect(patches).toEqual([
        {
          id: MORNING.id,
          body: { name: "IELTS 7.5 Sáng", scheduleLabel: "T2 · T4, 09:00" },
        },
      ]),
    );
  });

  it("opens Change teacher from Edit class", async () => {
    const { user } = renderPage();
    const menu = await openMenu(user, MORNING.name);
    await user.click(within(menu).getByRole("menuitem", { name: "Sửa lớp" }));
    const dialog = await screen.findByRole("dialog", { name: "Sửa lớp" });
    await user.click(within(dialog).getByRole("button", { name: "Đổi giáo viên" }));
    expect(await screen.findByRole("dialog", { name: "Đổi giáo viên" })).toBeVisible();
    expect(screen.queryByRole("dialog", { name: "Sửa lớp" })).toBeNull();
  });

  it("changes the teacher, telling both by default", async () => {
    const { user } = renderPage();
    const menu = await openMenu(user, MORNING.name);
    await user.click(within(menu).getByRole("menuitem", { name: "Đổi giáo viên" }));
    const dialog = await screen.findByRole("dialog", { name: "Đổi giáo viên" });
    expect(within(dialog).getByText(MORNING.name)).toBeVisible();
    expect(
      within(dialog).getByRole("switch", { name: "Báo cho cả hai giáo viên" }),
    ).toBeChecked();
    await user.click(
      await within(dialog).findByRole("combobox", { name: /Giáo viên mới/ }),
    );
    await user.click(await screen.findByRole("option", { name: /Minh Trần/ }));
    await user.click(within(dialog).getByRole("button", { name: "Đổi" }));
    await waitFor(() =>
      expect(changes).toEqual([
        { id: MORNING.id, body: { teacherId: MINH, notify: true } },
      ]),
    );
    expect(
      await screen.findByText(`${MORNING.name} giờ do Minh Trần dạy`),
    ).toBeVisible();
  });

  it("keeps the dialog open and says why when the server refuses the teacher", async () => {
    server.use(
      http.put(`${BASE}/admin/classes/:id/teacher`, () =>
        HttpResponse.json(
          { error: { code: "INVALID_RECIPIENT", message: "not a teacher" } },
          { status: 422 },
        ),
      ),
    );
    const { user } = renderPage();
    const menu = await openMenu(user, MORNING.name);
    await user.click(within(menu).getByRole("menuitem", { name: "Đổi giáo viên" }));
    const dialog = await screen.findByRole("dialog", { name: "Đổi giáo viên" });
    await user.click(within(dialog).getByRole("button", { name: "Đổi" }));
    expect(
      await within(dialog).findByText(
        "Người này không dạy lớp được: tài khoản đã tắt hoặc vai trò không có khu vực giáo viên.",
      ),
    ).toBeVisible();
  });
});

describe("the teacher picker", () => {
  function many(count: number) {
    return Array.from({ length: count }, (_, index) => {
      const n = String(index + 1).padStart(3, "0");
      return staff(`018f0000-0000-7000-8000-000000001${n}`, `Giáo viên ${n}`);
    });
  }

  async function pick(user: ReturnType<typeof userEvent.setup>) {
    await user.click(await screen.findByRole("button", { name: "Lớp mới" }));
    const dialog = await screen.findByRole("dialog", { name: "Lớp mới" });
    await user.click(
      await within(dialog).findByRole("combobox", { name: /Giáo viên/ }),
    );
    return dialog;
  }

  it("offers the 51st teacher, reading the Users list to its end", async () => {
    const teachers = many(51);
    const asked: string[] = [];
    server.use(
      http.get(`${BASE}/admin/users`, ({ request }) => {
        const query = new URL(request.url).searchParams;
        const tab = query.get("tab") ?? "";
        const page = Number(query.get("page") ?? 1);
        asked.push(`${tab}:${page}`);
        const all = tab === "teachers" ? teachers : [];
        const items = all.slice((page - 1) * 50, page * 50);
        return contractJson("/admin/users", "get", 200, {
          items,
          page,
          pageSize: 50,
          total: all.length,
          facets: { all: 51, teachers: 51, students: 0, admins: 0, disabled: 0 },
        });
      }),
    );
    const { user } = renderPage();
    await pick(user);
    expect(await screen.findByRole("option", { name: "Giáo viên 051" })).toBeVisible();
    expect(screen.getAllByRole("option")).toHaveLength(51);
    expect([...asked].sort((a, b) => a.localeCompare(b))).toEqual([
      "admins:1",
      "teachers:1",
      "teachers:2",
    ]);
  });

  it("tells two teachers of the same name apart by their email", async () => {
    server.use(
      http.get(`${BASE}/admin/users`, ({ request }) => {
        const tab = new URL(request.url).searchParams.get("tab");
        const items =
          tab === "teachers"
            ? [
                staff(GIAU, "Lan Nguyễn"),
                staff(DUNG, "Lan Nguyễn"),
                staff(MINH, "Minh Trần"),
              ]
            : [];
        return contractJson("/admin/users", "get", 200, {
          items,
          page: 1,
          pageSize: 50,
          total: items.length,
          facets: { all: 3, teachers: 3, students: 0, admins: 0, disabled: 0 },
        });
      }),
    );
    const { user } = renderPage();
    await pick(user);
    expect(
      (await screen.findAllByRole("option")).map((option) => option.textContent),
    ).toEqual([
      "Lan Nguyễn (a1@example.com)",
      "Lan Nguyễn (a2@example.com)",
      "Minh Trần",
    ]);
  });

  it("creates one class however fast Create is pressed twice", async () => {
    const { user } = renderPage();
    const dialog = await pick(user);
    await user.click(screen.getByRole("option", { name: /Minh Trần/ }));
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên lớp/ }),
      "IELTS 8.0",
    );
    const create = within(dialog).getByRole("button", { name: "Tạo lớp" });
    act(() => {
      create.click();
      create.click();
    });
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(creates).toHaveLength(1);
  });
});

describe("the page while the centre has turned joining by code off (D-028)", () => {
  it("says so, and pauses Copy, Rotate and the menu's rotation with the centre's sentence", async () => {
    serveJoinCodes(false);
    const { user } = renderPage();
    expect(
      await screen.findByText(/Trung tâm đang tắt tham gia bằng mã\./),
    ).toBeVisible();
    const row = await rowOf(MORNING.name);
    const rotate = await within(row).findByRole("button", {
      name: `Đổi mã tham gia của ${MORNING.name}. Trung tâm đang tắt tham gia bằng mã.`,
    });
    expect(rotate).toHaveAttribute("aria-disabled", "true");
    const copy = within(row).getByRole("button", {
      name: `Sao chép mã tham gia của ${MORNING.name}. Trung tâm đang tắt tham gia bằng mã.`,
    });
    expect(copy).toHaveAttribute("aria-disabled", "true");
    expect(within(row).getByText("K7QM-2PXA")).toBeVisible();
    await user.click(rotate);
    expect(screen.queryByRole("dialog")).toBeNull();
    const menu = await openMenu(user, MORNING.name);
    const item = within(menu).getByRole("menuitem", { name: /Đổi mã tham gia/ });
    expect(item).toHaveAttribute("aria-disabled", "true");
    await user.click(item);
    expect(rotations).toEqual([]);
  });

  it("says a new class has no code yet", async () => {
    serveJoinCodes(false);
    const { user } = renderPage();
    await screen.findByText(/Trung tâm đang tắt tham gia bằng mã\./);
    await user.click(screen.getByRole("button", { name: "Lớp mới" }));
    const dialog = await screen.findByRole("dialog", { name: "Lớp mới" });
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên lớp/ }),
      "IELTS 8.0",
    );
    await user.click(
      await within(dialog).findByRole("combobox", { name: /Giáo viên/ }),
    );
    await user.click(await screen.findByRole("option", { name: /Minh Trần/ }));
    await user.click(within(dialog).getByRole("button", { name: "Tạo lớp" }));
    expect(
      await screen.findByText(
        "Đã tạo lớp. Trung tâm đang tắt tham gia bằng mã nên lớp chưa có mã.",
      ),
    ).toBeVisible();
  });
});

describe("the page in English", () => {
  it("uses the deck's words", async () => {
    await act(() => i18n.changeLanguage("en"));
    renderPage();
    expect(await screen.findByText("Every class across teachers.")).toBeVisible();
    expect(screen.getByPlaceholderText("Search classes or teachers")).toBeVisible();
    expect((await screen.findAllByText("Jun 2026")).length).toBeGreaterThan(0);
  });
});
