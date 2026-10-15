import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { delay, http, HttpResponse } from "msw";
import RolesPage from "@/features/roles/pages/admin/RolesPage";
import { Toaster } from "@/components/ui/sonner";
import i18n from "@/lib/i18n";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import {
  ADMIN_ID,
  BASE,
  HEAD_ID,
  MATRIX_KEYS,
  ROLES,
  STUDENT_ID,
  TEACHER_ID,
  admin,
  assistant,
  head,
  listOf,
  serveRoles,
  student,
  teacher,
} from "@tests/support/roles";

type User = components["schemas"]["CurrentUser"];

const AI = "Dùng tính năng AI";
const ATTENDANCE = "Điểm danh";
const SHARE = "Chia sẻ nội dung của mình với giáo viên khác";
const AUDIT = "Xem nhật ký kiểm tra";
const TAKE_TESTS = "Làm bài kiểm tra và xem kết quả của mình";
const WRITE = "Tạo và sửa bài kiểm tra";
const ADMIN_NAME = "Quản trị viên";
const TEACHER_NAME = "Giáo viên";
const HEAD_NAME = "Tổ trưởng bộ môn";
const CHANGED = (n: number) =>
  `${n} quyền đã thay đổi. Mọi người nhận quyền mới ở lần tải trang tiếp theo.`;

const headUser: User = {
  ...adminUser,
  id: "019535d9-3df7-79fb-b466-fa907fa17faa",
  role: { id: HEAD_ID, builtinKey: null, name: HEAD_NAME },
  permissions: [...head.permissions, "people.roles.manage"],
  workspaces: ["admin"],
};

function renderPage(user: User = adminUser, at = "/admin/roles") {
  useAuthStore.getState().setSession("token", user);
  const router = createMemoryRouter(
    [{ path: "/admin/roles", element: <RolesPage /> }],
    {
      initialEntries: [at],
    },
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Toaster />
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, client, user: userEvent.setup() };
}

function cell(permission: string, role: string) {
  return screen.getByRole("checkbox", { name: `${permission} — ${role}` });
}

function savedWith(updated: typeof ROLES) {
  const sent: unknown[] = [];
  server.use(
    http.put(`${BASE}/admin/roles/permissions`, async ({ request }) => {
      sent.push(await request.json());
      return contractJson("/admin/roles/permissions", "put", 200, { roles: updated });
    }),
  );
  return sent;
}

beforeEach(() => {
  Element.prototype.scrollTo = vi.fn();
});

afterEach(async () => {
  await i18n.changeLanguage("vi");
  vi.restoreAllMocks();
});

describe("the page", () => {
  it("says what it is and draws a card for each role, the Assistant's with no one", async () => {
    serveRoles();
    renderPage();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Vai trò & quyền" }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Mỗi vai trò làm được gì. Quản trị viên luôn làm được mọi việc.",
      ),
    ).toBeVisible();
    expect(await screen.findByRole("article", { name: ADMIN_NAME })).toBeVisible();
    for (const name of [TEACHER_NAME, "Trợ giảng", "Học viên", HEAD_NAME])
      expect(screen.getByRole("article", { name })).toBeVisible();
    const assistantCard = screen.getByRole("article", { name: "Trợ giảng" });
    expect(within(assistantCard).getByText("0")).toBeVisible();
    expect(within(assistantCard).getByText("6")).toBeVisible();
    const custom = screen.getByRole("article", { name: HEAD_NAME });
    expect(within(custom).getByText("Tùy chỉnh")).toBeVisible();
    expect(
      within(screen.getByRole("article", { name: TEACHER_NAME })).queryByText(
        "Tùy chỉnh",
      ),
    ).toBeNull();
  });

  it("draws the matrix: a column for each role including the Assistant, and the groups", async () => {
    serveRoles();
    renderPage();
    const grid = await screen.findByRole("grid", { name: "Bảng quyền theo vai trò" });
    expect(within(grid).getAllByRole("checkbox")).toHaveLength(
      MATRIX_KEYS.length * ROLES.length,
    );
    for (const name of [ADMIN_NAME, TEACHER_NAME, "Trợ giảng", "Học viên", HEAD_NAME])
      expect(within(grid).getByRole("button", { name })).toBeVisible();
    for (const group of ["Nội dung", "Giảng dạy", "Mọi người", "Hệ thống"])
      expect(within(grid).getByText(group)).toBeVisible();
    expect(within(grid).getByText("Tạo tài khoản học viên")).toBeVisible();
    expect(within(grid).getByText("Giúp bài kiểm tra có thể được giao")).toBeVisible();
    expect(within(grid).getAllByText("Được ghi vào nhật ký kiểm tra")).toHaveLength(2);
    expect(cell(WRITE, TEACHER_NAME)).toBeChecked();
    expect(cell(AUDIT, TEACHER_NAME)).not.toBeChecked();
    expect(cell(TAKE_TESTS, ADMIN_NAME)).not.toBeChecked();
  });

  it("reads a built-in role in English until an Admin renames it", async () => {
    await i18n.changeLanguage("en");
    serveRoles(
      listOf([admin, { ...teacher, name: "Giảng viên" }, assistant, student, head]),
    );
    renderPage();
    await screen.findByRole("grid", { name: "Permissions by role" });
    for (const name of ["Admin", "Assistant", "Student"])
      expect(screen.getByRole("article", { name })).toBeVisible();
    expect(screen.getByRole("article", { name: "Giảng viên" })).toBeVisible();
    expect(screen.queryByRole("article", { name: "Teacher" })).toBeNull();
    expect(screen.getByRole("article", { name: HEAD_NAME })).toBeVisible();
    expect(
      screen.getByRole("checkbox", { name: "Create student accounts — Admin" }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { level: 1, name: "Roles & permissions" }),
    ).toBeVisible();
  });

  it("shows a skeleton while the roles load", async () => {
    server.use(
      http.get(`${BASE}/admin/roles`, async () => {
        await delay(60);
        return contractJson("/admin/roles", "get", 200, listOf());
      }),
    );
    renderPage();
    expect(screen.getByRole("status", { name: "Đang tải…" })).toBeVisible();
    await screen.findByRole("grid");
    expect(screen.queryByRole("status", { name: "Đang tải…" })).toBeNull();
  });

  it("says it could not load them, and reads again on Retry", async () => {
    let reads = 0;
    server.use(
      http.get(`${BASE}/admin/roles`, () => {
        reads += 1;
        if (reads === 1)
          return HttpResponse.json(
            {
              error: {
                code: "INTERNAL",
                message: "Máy chủ gặp lỗi.",
                requestId: "r-1",
              },
            },
            { status: 500 },
          );
        return contractJson("/admin/roles", "get", 200, listOf());
      }),
    );
    const { user } = renderPage();
    expect(await screen.findByText("Không tải được danh sách vai trò.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("grid")).toBeVisible();
    expect(reads).toBe(2);
  });
});

describe("the cells an Admin cannot change", () => {
  it("locks the Admin column but Take tests, with the reason, and ignores a press", async () => {
    serveRoles();
    const { user } = renderPage();
    await screen.findByRole("grid");
    const locked = cell(WRITE, ADMIN_NAME);
    expect(locked).toBeChecked();
    expect(locked).toHaveAttribute("aria-disabled", "true");
    expect(locked).toHaveAttribute("title", "Quản trị viên luôn có quyền này");
    expect(locked).toHaveAccessibleDescription("Quản trị viên luôn có quyền này");
    await user.click(locked);
    expect(locked).toBeChecked();
    expect(screen.queryByText(CHANGED(1))).toBeNull();
    expect(cell(TAKE_TESTS, ADMIN_NAME)).not.toHaveAttribute("aria-disabled");
  });

  it("lets an Admin turn Take tests on for the Admin role, which is their own", async () => {
    serveRoles();
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(TAKE_TESTS, ADMIN_NAME));
    expect(cell(TAKE_TESTS, ADMIN_NAME)).toBeChecked();
    expect(await screen.findByText(CHANGED(1))).toBeVisible();
  });

  it("keeps Take tests on the Student role", async () => {
    serveRoles();
    renderPage();
    await screen.findByRole("grid");
    const locked = cell(TAKE_TESTS, "Học viên");
    expect(locked).toBeChecked();
    expect(locked).toHaveAttribute("aria-disabled", "true");
    expect(locked).toHaveAttribute("title", "Học viên luôn có quyền này để làm bài");
  });

  it("locks the caller's own role, and a role or key the subset rule keeps from them", async () => {
    serveRoles(
      listOf([
        { ...admin, editable: false },
        { ...teacher, editable: false },
        { ...assistant, editable: false },
        student,
        head,
      ]),
    );
    const { user } = renderPage(headUser);
    await screen.findByRole("grid");
    const own = cell(WRITE, HEAD_NAME);
    expect(own).toHaveAttribute("aria-disabled", "true");
    expect(own).toHaveAttribute(
      "title",
      "Bạn không đổi được quyền của chính vai trò mình đang giữ.",
    );
    const beyond = cell(WRITE, TEACHER_NAME);
    expect(beyond).toHaveAttribute("title", "Bạn chỉ đổi được những quyền bạn có.");
    const unheld = cell(AUDIT, "Học viên");
    expect(unheld).toHaveAttribute("title", "Bạn chỉ đổi được những quyền bạn có.");
    await user.click(own);
    await user.click(beyond);
    await user.click(unheld);
    expect(screen.queryByText(CHANGED(1))).toBeNull();
    const open = cell(WRITE, "Học viên");
    expect(open).not.toHaveAttribute("aria-disabled");
    await user.click(open);
    expect(await screen.findByText(CHANGED(1))).toBeVisible();
  });

  it("fades a paused cell's colours and never its element", async () => {
    serveRoles();
    renderPage();
    await screen.findByRole("grid");
    const locked = cell(WRITE, ADMIN_NAME);
    expect(locked.className).not.toMatch(/(^|\s)(aria-disabled:)?opacity-/);
    expect(locked).toHaveClass("aria-disabled:checked:bg-primary/50");
    locked.focus();
    expect(locked).toHaveFocus();
  });
});

describe("the unsaved changes", () => {
  it("counts the cells that differ, and a cell turned back is not one", async () => {
    serveRoles();
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(AI, TEACHER_NAME));
    expect(cell(AI, TEACHER_NAME)).not.toBeChecked();
    expect(screen.getByText(CHANGED(1))).toBeVisible();
    await user.click(cell(ATTENDANCE, "Trợ giảng"));
    expect(screen.getByText(CHANGED(2))).toBeVisible();
    await user.click(cell(AI, TEACHER_NAME));
    expect(screen.getByText(CHANGED(1))).toBeVisible();
    await user.click(cell(ATTENDANCE, "Trợ giảng"));
    expect(screen.queryByRole("button", { name: "Lưu thay đổi" })).toBeNull();
  });

  it("marks the cells that differ from the server", async () => {
    serveRoles();
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(AI, TEACHER_NAME));
    expect(cell(AI, TEACHER_NAME)).toHaveAttribute("data-changed", "true");
    expect(cell(SHARE, TEACHER_NAME)).toHaveAttribute("data-changed", "false");
  });

  it("reverts everything on Discard", async () => {
    serveRoles();
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(AI, TEACHER_NAME));
    await user.click(cell(AUDIT, TEACHER_NAME));
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));
    expect(cell(AI, TEACHER_NAME)).toBeChecked();
    expect(cell(AUDIT, TEACHER_NAME)).not.toBeChecked();
    expect(screen.queryByText(CHANGED(2))).toBeNull();
  });

  it("warns how many people are mid-test when a role loses Take tests", async () => {
    serveRoles();
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(TAKE_TESTS, HEAD_NAME));
    expect(await screen.findByText("2 người đang làm bài ngay lúc này.")).toBeVisible();
    await user.click(cell(TAKE_TESTS, HEAD_NAME));
    expect(screen.queryByText(/đang làm bài ngay lúc này/)).toBeNull();
  });

  it("says the same in English, in the singular too", async () => {
    await i18n.changeLanguage("en");
    serveRoles(
      listOf([admin, teacher, assistant, student, { ...head, inProgressAttempts: 1 }]),
    );
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(
      screen.getByRole("checkbox", {
        name: /^Take tests and view own results — Tổ trưởng/,
      }),
    );
    expect(
      await screen.findByText(
        "1 permission changed. People get the new permissions on their next page load.",
      ),
    ).toBeVisible();
    expect(screen.getByText("1 person is taking a test right now.")).toBeVisible();
  });
});

describe("saving", () => {
  it("sends one save with the cells that changed and the revisions read, then shows the answer", async () => {
    serveRoles();
    const sent = savedWith([
      {
        ...teacher,
        permissions: teacher.permissions.filter((key) => key !== "ai.use"),
        revision: 4,
      },
    ]);
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(AI, TEACHER_NAME));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    expect(await screen.findByText("Đã lưu quyền")).toBeVisible();
    expect(sent).toEqual([
      {
        changes: [{ roleId: TEACHER_ID, key: "ai.use", granted: false }],
        expectedRevisions: [{ roleId: TEACHER_ID, revision: 3 }],
      },
    ]);
    expect(screen.queryByText(CHANGED(1))).toBeNull();
    expect(cell(AI, TEACHER_NAME)).not.toBeChecked();
    expect(
      within(screen.getByRole("article", { name: TEACHER_NAME })).getByText("13"),
    ).toBeVisible();
  });

  it("keeps the draft and says why when the save is refused", async () => {
    serveRoles();
    server.use(
      http.put(`${BASE}/admin/roles/permissions`, () =>
        HttpResponse.json(
          {
            error: {
              code: "CANNOT_ACT_ON_SELF",
              message: "Bạn không thể đổi quyền của vai trò của mình.",
              requestId: "r-2",
            },
          },
          { status: 422 },
        ),
      ),
    );
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(AI, TEACHER_NAME));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    expect(
      await screen.findByText("Bạn không thể đổi quyền của vai trò của mình."),
    ).toBeVisible();
    expect(screen.getByText(CHANGED(1))).toBeVisible();
    expect(cell(AI, TEACHER_NAME)).not.toBeChecked();
  });

  it("reloads the matrix on a stale save and keeps the unsaved cells marked", async () => {
    const seen = serveRoles();
    const refused: unknown[] = [];
    server.use(
      http.put(`${BASE}/admin/roles/permissions`, async ({ request }) => {
        refused.push(await request.json());
        seen.list = listOf([
          admin,
          {
            ...teacher,
            revision: 4,
            permissions: teacher.permissions.filter((key) => key !== "content.share"),
          },
          assistant,
          student,
          head,
        ]);
        return HttpResponse.json(
          {
            error: {
              code: "STALE_WRITE",
              message: "Vai trò đã thay đổi.",
              requestId: "r-3",
            },
          },
          { status: 409 },
        );
      }),
    );
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(AI, TEACHER_NAME));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    expect(
      await screen.findByText(
        "Có người vừa thay đổi các vai trò này. Bảng đã được tải lại; hãy kiểm tra các ô đã đánh dấu rồi lưu lại.",
      ),
    ).toBeVisible();
    await waitFor(() => expect(seen.reads).toBe(2));
    await waitFor(() => expect(cell(SHARE, TEACHER_NAME)).not.toBeChecked());
    expect(cell(AI, TEACHER_NAME)).not.toBeChecked();
    expect(cell(AI, TEACHER_NAME)).toHaveAttribute("data-changed", "true");
    expect(screen.getByText(CHANGED(1))).toBeVisible();

    const sent = savedWith([{ ...teacher, revision: 5, permissions: [] }]);
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0]).toEqual({
      changes: [{ roleId: TEACHER_ID, key: "ai.use", granted: false }],
      expectedRevisions: [{ roleId: TEACHER_ID, revision: 4 }],
    });
    expect(refused).toHaveLength(1);
  });

  it("holds the bar still while the save is out", async () => {
    serveRoles();
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.put(`${BASE}/admin/roles/permissions`, async () => {
        await gate;
        return contractJson("/admin/roles/permissions", "put", 200, {
          roles: [{ ...teacher, revision: 5, permissions: [] }],
        });
      }),
    );
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(cell(AI, TEACHER_NAME));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    const saving = await screen.findByRole("button", { name: "Đang lưu…" });
    expect(saving).toHaveAttribute("aria-disabled", "true");
    await user.click(cell(AUDIT, TEACHER_NAME));
    expect(cell(AUDIT, TEACHER_NAME)).not.toBeChecked();
    expect(screen.getByText(CHANGED(1))).toBeVisible();
    release();
    await waitFor(() => expect(screen.queryByText(CHANGED(1))).toBeNull());
  });
});

describe("the highlighted column", () => {
  it("frames a role's column when its header is pressed, and drops the frame when pressed again", async () => {
    serveRoles();
    const { router, user } = renderPage();
    const grid = await screen.findByRole("grid");
    const header = within(grid).getByRole("button", { name: TEACHER_NAME });
    expect(header).toHaveAttribute("aria-pressed", "false");
    expect(document.querySelector("[data-slot=column-highlight]")).toBeNull();
    await user.click(header);
    expect(router.state.location.search).toBe(`?role=${TEACHER_ID}`);
    expect(header).toHaveAttribute("aria-pressed", "true");
    expect(document.querySelector("[data-slot=column-highlight]")).not.toBeNull();
    await user.click(within(grid).getByRole("button", { name: "Trợ giảng" }));
    expect(router.state.location.search).toBe(
      "?role=019535d8-0000-7000-8000-000000000003",
    );
    await user.click(within(grid).getByRole("button", { name: "Trợ giảng" }));
    expect(router.state.location.search).toBe("");
    expect(document.querySelector("[data-slot=column-highlight]")).toBeNull();
  });

  it("starts on the role a link names, and ignores one that is not a role", async () => {
    serveRoles();
    renderPage(adminUser, `/admin/roles?role=${STUDENT_ID}`);
    const grid = await screen.findByRole("grid");
    expect(within(grid).getByRole("button", { name: "Học viên" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(document.querySelector("[data-slot=column-highlight]")).not.toBeNull();
  });

  it("ignores an unknown role in the link", async () => {
    serveRoles();
    renderPage(adminUser, "/admin/roles?role=nobody");
    await screen.findByRole("grid");
    expect(document.querySelector("[data-slot=column-highlight]")).toBeNull();
  });

  it("slides to a role's column from its card, scrolls to the matrix and focuses its first cell", async () => {
    serveRoles();
    const scroll = vi.fn();
    Element.prototype.scrollIntoView = scroll;
    const { router, user } = renderPage();
    await screen.findByRole("grid");
    await user.click(
      screen.getByRole("button", { name: `Sửa quyền của ${TEACHER_NAME}` }),
    );
    expect(router.state.location.search).toBe(`?role=${TEACHER_ID}`);
    expect(scroll).toHaveBeenCalledWith({ block: "start", behavior: "smooth" });
    expect(cell(WRITE, TEACHER_NAME)).toHaveFocus();
    await user.click(
      screen.getByRole("button", { name: `Sửa quyền của ${TEACHER_NAME}` }),
    );
    expect(router.state.location.search).toBe(`?role=${TEACHER_ID}`);
  });

  it("moves without animation under reduced motion", async () => {
    serveRoles();
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: query.includes("prefers-reduced-motion"),
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }));
    const scroll = vi.fn();
    Element.prototype.scrollIntoView = scroll;
    const { user } = renderPage();
    await screen.findByRole("grid");
    await user.click(
      screen.getByRole("button", { name: `Sửa quyền của ${ADMIN_NAME}` }),
    );
    expect(scroll).toHaveBeenCalledWith({ block: "start", behavior: "auto" });
    expect(ADMIN_ID).toBeTruthy();
    vi.unstubAllGlobals();
  });
});

describe("the keyboard", () => {
  it("makes the cells one tab stop, and moves between them with the arrows", async () => {
    serveRoles();
    const { user } = renderPage();
    const grid = await screen.findByRole("grid");
    const stops = within(grid)
      .getAllByRole("checkbox")
      .filter((box) => box.tabIndex === 0);
    expect(stops).toHaveLength(1);
    const first = cell(WRITE, ADMIN_NAME);
    expect(first).toBe(stops[0]);
    first.focus();
    await user.keyboard("{ArrowRight}");
    expect(cell(WRITE, TEACHER_NAME)).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(cell("Xuất bản bài kiểm tra", TEACHER_NAME)).toHaveFocus();
    await user.keyboard("{ArrowLeft}{ArrowUp}");
    expect(first).toHaveFocus();
    await user.keyboard("{ArrowUp}{ArrowLeft}");
    expect(first).toHaveFocus();
    await user.keyboard("{End}");
    expect(cell(WRITE, HEAD_NAME)).toHaveFocus();
    await user.keyboard("{Home}");
    expect(first).toHaveFocus();
    await user.keyboard("{Control>}{End}{/Control}");
    expect(cell("Quản lý tín dụng AI", HEAD_NAME)).toHaveFocus();
    await user.keyboard("{Control>}{Home}{/Control}");
    expect(first).toHaveFocus();
    expect(
      within(grid)
        .getAllByRole("checkbox")
        .filter((box) => box.tabIndex === 0),
    ).toEqual([first]);
  });

  it("reaches a locked cell and toggles an open one with the space bar", async () => {
    serveRoles();
    const { user } = renderPage();
    await screen.findByRole("grid");
    cell(WRITE, ADMIN_NAME).focus();
    await user.keyboard(" ");
    expect(cell(WRITE, ADMIN_NAME)).toBeChecked();
    await user.keyboard("{ArrowRight}");
    await user.keyboard(" ");
    expect(cell(WRITE, TEACHER_NAME)).not.toBeChecked();
    expect(screen.getByText(CHANGED(1))).toBeVisible();
  });

  it("gives focus back to the table when the bar goes", async () => {
    serveRoles();
    const { user } = renderPage();
    await screen.findByRole("grid");
    cell(AI, TEACHER_NAME).focus();
    await user.keyboard(" ");
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));
    expect(cell(AI, TEACHER_NAME)).toHaveFocus();
  });
});
