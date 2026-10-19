import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { toast } from "@/components/ui/sonner";
import type { components } from "@/lib/api/schema";
import { contractJson } from "@tests/support/contractResponse";
import { admin, listOf, serveRoles, teacher } from "@tests/support/roles";
import { server } from "@tests/support/server";
import {
  ADMIN_ROLE,
  BASE,
  STUDENT_ROLE,
  TEACHER_ROLE,
  renderUsers,
  rowOf,
  serveUsers,
  user,
  type UserRow,
} from "./support";

vi.mock("@/components/ui/sonner", async (original) => ({
  ...(await original<typeof import("@/components/ui/sonner")>()),
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }),
}));

type Detail = components["schemas"]["AdminUserDetail"];
type Can = components["schemas"]["AdminUserCan"];

const ROLES: components["schemas"]["AssignableRole"][] = [
  { ...ADMIN_ROLE, studentTarget: false },
  { ...TEACHER_ROLE, studentTarget: false },
  { ...STUDENT_ROLE, studentTarget: true },
];
const ALL: Can = {
  changeRole: true,
  resetPassword: true,
  disable: true,
  enable: false,
  signOut: true,
  transfer: true,
};
const NONE: Can = {
  changeRole: false,
  resetPassword: false,
  disable: false,
  enable: false,
  signOut: false,
  transfer: false,
};
const NOTHING = { classes: 0, tests: 0, questions: 0, media: 0, assignments: 0 };

const BINH = user("2", "Trần Thị Bình", { role: TEACHER_ROLE });
const AN = user("1", "Nguyễn Văn An");
const CHI = user("3", "Lê Chi", { role: TEACHER_ROLE });

function detail(row: UserRow, overrides: Partial<Detail> = {}): Detail {
  return {
    user: row,
    createdBy: null,
    classes: [],
    ownedContent: NOTHING,
    inProgressAttempts: 0,
    activity: [],
    can: ALL,
    ...overrides,
  };
}

function refusal(status: number, code: string) {
  return HttpResponse.json(
    {
      error: { code, message: code, requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e" },
    },
    { status },
  );
}

function serveDetail(answer: Detail) {
  server.use(
    http.get(`${BASE}/admin/users/:id`, () =>
      contractJson("/admin/users/{id}", "get", 200, answer),
    ),
    http.get(`${BASE}/admin/roles/assignable`, () =>
      contractJson("/admin/roles/assignable", "get", 200, ROLES),
    ),
  );
}

async function sheet() {
  return screen.findByRole("dialog", { name: /Trần Thị Bình/ });
}

beforeEach(() => {
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.error).mockClear();
});

describe("Users: the user sheet (T-R5.22b)", () => {
  it("opens from a row at ?user= and Esc closes it and clears the address", async () => {
    serveUsers([AN, BINH]);
    serveDetail(
      detail(BINH, {
        classes: [{ id: "019535da-0000-7000-8000-000000000001", name: "IELTS 6.5" }],
        activity: [{ kind: "sign_in", at: "2026-10-18T01:00:00Z", detail: "google" }],
      }),
    );
    const { user: actor, router } = renderUsers();
    await actor.click(within(await rowOf("Trần Thị Bình")).getAllByRole("button")[0]!);
    const panel = await sheet();
    expect(router.state.location.search).toContain(`user=${BINH.id}`);
    expect(await within(panel).findByText("IELTS 6.5")).toBeVisible();
    expect(within(panel).getByText("Đăng nhập bằng Google")).toBeVisible();
    expect(within(panel).getByRole("button", { name: "Giáo viên" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await actor.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(router.state.location.search).not.toContain("user=");
  });

  it("shows only the actions the server allows", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH, { can: { ...NONE, signOut: true } }));
    renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    expect(
      await within(panel).findByRole("button", { name: /Đăng xuất khỏi mọi thiết bị/ }),
    ).toBeVisible();
    for (const name of [/Đặt lại mật khẩu/, /Vô hiệu hoá tài khoản/, /Chuyển nội dung/])
      expect(within(panel).queryByRole("button", { name })).toBeNull();
    expect(within(panel).getByRole("button", { name: "Giáo viên" })).toBeDisabled();
  });

  it("hides the role without people.roles.manage", async () => {
    const asked: string[] = [];
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    server.use(
      http.get(`${BASE}/admin/roles/assignable`, ({ request }) => {
        asked.push(request.url);
        return contractJson("/admin/roles/assignable", "get", 200, ROLES);
      }),
    );
    const { adminUser } = await import("@tests/support/fixtures");
    renderUsers(`/admin/users?user=${BINH.id}`, 1280, {
      ...adminUser,
      permissions: adminUser.permissions.filter((key) => key !== "people.roles.manage"),
    });
    const panel = await sheet();
    await within(panel).findByRole("button", { name: /Đặt lại mật khẩu/ });
    expect(within(panel).queryByRole("group", { name: "Vai trò" })).toBeNull();
    expect(asked).toEqual([]);
  });

  it("changes the role through a confirmation", async () => {
    const sent: unknown[] = [];
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    server.use(
      http.put(`${BASE}/admin/users/:id/role`, async ({ request }) => {
        sent.push(await request.json());
        return contractJson("/admin/users/{id}/role", "put", 200, {
          ...BINH,
          role: ADMIN_ROLE,
        });
      }),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: "Quản trị viên" }),
    );
    const confirm = await screen.findByRole("dialog", {
      name: "Chuyển Trần Thị Bình thành quản trị viên?",
    });
    expect(
      within(confirm).getByText(
        "Quản trị viên có thể thay đổi mọi cài đặt và xem nhật ký kiểm tra.",
      ),
    ).toBeVisible();
    await actor.click(within(confirm).getByRole("button", { name: "Đổi vai trò" }));
    await waitFor(() => expect(sent).toEqual([{ roleId: ADMIN_ROLE.id }]));
    expect(toast.success).toHaveBeenCalledWith("Đã đổi vai trò thành quản trị viên");
  });

  it("says a teacher who owns content must hand it over first", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    server.use(
      http.put(`${BASE}/admin/users/:id/role`, () => refusal(409, "USER_OWNS_CONTENT")),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(await within(panel).findByRole("button", { name: "Học viên" }));
    const confirm = await screen.findByRole("dialog", {
      name: "Chuyển Trần Thị Bình thành học viên?",
    });
    await actor.click(within(confirm).getByRole("button", { name: "Đổi vai trò" }));
    expect(
      await within(confirm).findByText(
        "Tài khoản này còn sở hữu lớp hoặc nội dung. Hãy vô hiệu hoá rồi chuyển nội dung trước.",
      ),
    ).toBeVisible();
  });

  it("warns before disabling a teacher with classes and a student mid-test", async () => {
    const sent: string[] = [];
    serveUsers([BINH]);
    serveDetail(
      detail(BINH, { ownedContent: { ...NOTHING, classes: 2 }, inProgressAttempts: 1 }),
    );
    server.use(
      http.post(`${BASE}/admin/users/:id/disable`, ({ params }) => {
        sent.push(String(params.id));
        return contractJson("/admin/users/{id}/disable", "post", 200, {
          ...BINH,
          status: "disabled",
        });
      }),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Vô hiệu hoá tài khoản/ }),
    );
    const confirm = await screen.findByRole("dialog", {
      name: "Vô hiệu hoá Trần Thị Bình?",
    });
    expect(within(confirm).getByText(/Họ đang dạy 2 lớp/)).toBeVisible();
    expect(
      within(confirm).getByText("Họ đang làm bài kiểm tra ngay lúc này."),
    ).toBeVisible();
    await actor.click(within(confirm).getByRole("button", { name: "Vô hiệu hoá" }));
    await waitFor(() => expect(sent).toEqual([BINH.id]));
    expect(toast.success).toHaveBeenCalledWith("Đã vô hiệu hoá tài khoản");
  });

  it("emails a reset link, or shows a temporary password once", async () => {
    const sent: unknown[] = [];
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    server.use(
      http.post(`${BASE}/admin/users/:id/reset-password`, async ({ request }) => {
        const body = (await request.json()) as { delivery: string };
        sent.push(body);
        return contractJson(
          "/admin/users/{id}/reset-password",
          "post",
          200,
          body.delivery === "temporary_password"
            ? { temporaryPassword: "Ngoc-Lan-42" }
            : {},
        );
      }),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Đặt lại mật khẩu/ }),
    );
    let confirm = await screen.findByRole("dialog", {
      name: "Đặt lại mật khẩu của Trần Thị Bình?",
    });
    await actor.click(
      within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith(`Đã gửi liên kết tới ${BINH.email}`),
    );

    await actor.click(within(panel).getByRole("button", { name: /Đặt lại mật khẩu/ }));
    confirm = await screen.findByRole("dialog", {
      name: "Đặt lại mật khẩu của Trần Thị Bình?",
    });
    await actor.click(
      within(confirm).getByRole("radio", { name: /Hiện mật khẩu tạm thời/ }),
    );
    await actor.click(
      within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    expect(await screen.findByText("Ngoc-Lan-42")).toBeVisible();
    expect(sent).toEqual([
      { delivery: "email_link" },
      { delivery: "temporary_password" },
    ]);
  });

  it("offers Transfer content only for a disabled account that owns something", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH, { ownedContent: { ...NOTHING, tests: 3 } }));
    renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await within(panel).findByRole("button", { name: /Đặt lại mật khẩu/ });
    expect(within(panel).queryByRole("button", { name: /Chuyển nội dung/ })).toBeNull();
  });

  it("moves a disabled teacher's content and says what stayed behind", async () => {
    const off = { ...BINH, status: "disabled" as const };
    const sent: unknown[] = [];
    serveUsers([off, CHI]);
    serveDetail(
      detail(off, {
        can: { ...ALL, disable: false, enable: true },
        ownedContent: { ...NOTHING, classes: 2, tests: 5 },
      }),
    );
    server.use(
      http.post(`${BASE}/admin/users/:id/transfer-content`, async ({ request }) => {
        sent.push(await request.json());
        return contractJson("/admin/users/{id}/transfer-content", "post", 200, {
          moved: {
            classes: 2,
            tests: 5,
            questions: 0,
            questionGroups: 0,
            media: 0,
            assignments: 0,
          },
          leftBehind: { imports: 1, students: 0 },
        });
      }),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Chuyển nội dung/ }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Chuyển nội dung của Trần Thị Bình cho",
    });
    expect(within(dialog).getByText("5 đề")).toBeVisible();
    const submit = within(dialog).getByRole("button", { name: "Chuyển nội dung" });
    expect(submit).toHaveAttribute("aria-disabled", "true");
    await actor.click(await within(dialog).findByRole("radio", { name: /Lê Chi/ }));
    expect(within(dialog).queryByRole("radio", { name: /Trần Thị Bình/ })).toBeNull();
    await actor.click(
      within(dialog).getByRole("switch", { name: "Báo cho giáo viên nhận" }),
    );
    await actor.click(submit);
    expect(await screen.findByText("Đã chuyển cho Lê Chi:")).toBeVisible();
    expect(
      screen.getByText("1 lượt nhập Word chưa xong vẫn ở lại với tài khoản này."),
    ).toBeVisible();
    expect(sent).toEqual([{ toUserId: CHI.id, notify: false }]);
  });

  it("warns when the recipient's role cannot edit a kind it receives (D-052)", async () => {
    const off = { ...BINH, status: "disabled" as const };
    serveUsers([off, CHI]);
    serveDetail(
      detail(off, {
        can: { ...ALL, disable: false, enable: true },
        ownedContent: { ...NOTHING, classes: 1, tests: 5 },
      }),
    );
    serveRoles(
      listOf([
        {
          ...teacher,
          permissions: teacher.permissions.filter(
            (key) => key !== "content.tests.write",
          ),
        },
      ]),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Chuyển nội dung/ }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Chuyển nội dung của Trần Thị Bình cho",
    });
    await actor.click(await within(dialog).findByRole("radio", { name: /Lê Chi/ }));
    expect(
      await within(dialog).findByText(
        "Lê Chi không có quyền sửa đề, nên sau khi chuyển chỉ xem được chúng.",
      ),
    ).toBeVisible();
  });
});

describe("Users: the user sheet after QC (#724)", () => {
  async function refuseRoleChange(actor: ReturnType<typeof renderUsers>["user"]) {
    const panel = await sheet();
    await actor.click(await within(panel).findByRole("button", { name: "Học viên" }));
    const confirm = await screen.findByRole("dialog", {
      name: "Chuyển Trần Thị Bình thành học viên?",
    });
    await actor.click(within(confirm).getByRole("button", { name: "Đổi vai trò" }));
    return confirm;
  }

  it("tells an active owner to disable first, and Disable opens its confirmation (QC-US-2)", async () => {
    serveUsers([BINH]);
    serveDetail(
      detail(BINH, {
        can: { ...ALL, transfer: false },
        ownedContent: { ...NOTHING, classes: 1 },
      }),
    );
    server.use(
      http.put(`${BASE}/admin/users/:id/role`, () => refusal(409, "USER_OWNS_CONTENT")),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const confirm = await refuseRoleChange(actor);
    expect(
      await within(confirm).findByText(
        "Tài khoản này còn sở hữu lớp hoặc nội dung. Hãy vô hiệu hoá rồi chuyển nội dung trước.",
      ),
    ).toBeVisible();
    await actor.click(
      within(confirm).getByRole("button", { name: "Vô hiệu hoá tài khoản" }),
    );
    expect(
      await screen.findByRole("dialog", { name: "Vô hiệu hoá Trần Thị Bình?" }),
    ).toBeVisible();
  });

  it("tells a disabled owner to transfer, and opens Transfer content (QC-US-2)", async () => {
    const off = { ...BINH, status: "disabled" as const };
    serveUsers([off, CHI]);
    serveDetail(
      detail(off, {
        can: { ...ALL, disable: false, enable: true },
        ownedContent: { ...NOTHING, tests: 2 },
      }),
    );
    server.use(
      http.put(`${BASE}/admin/users/:id/role`, () => refusal(409, "USER_OWNS_CONTENT")),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const confirm = await refuseRoleChange(actor);
    expect(
      await within(confirm).findByText(
        "Tài khoản này còn sở hữu lớp hoặc nội dung. Hãy chuyển nội dung của họ trước.",
      ),
    ).toBeVisible();
    await actor.click(within(confirm).getByRole("button", { name: "Chuyển nội dung" }));
    expect(
      await screen.findByRole("dialog", {
        name: "Chuyển nội dung của Trần Thị Bình cho",
      }),
    ).toBeVisible();
  });

  it("says an Admin who sees everything must act when the caller can take no step (QC-US-2)", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH, { can: { ...ALL, disable: false, transfer: false } }));
    server.use(
      http.put(`${BASE}/admin/users/:id/role`, () => refusal(409, "USER_OWNS_CONTENT")),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const confirm = await refuseRoleChange(actor);
    expect(
      await within(confirm).findByText(
        "Tài khoản này còn sở hữu lớp hoặc nội dung. Một quản trị viên xem được mọi dữ liệu phải chuyển nội dung trước.",
      ),
    ).toBeVisible();
    expect(
      within(confirm).queryByRole("button", { name: /Vô hiệu hoá|Chuyển nội dung/ }),
    ).toBeNull();
  });

  it("names a role the caller cannot give instead of a blank control (QC-US-1)", async () => {
    const helper = user("7", "Trần Thị Bình", {
      role: {
        ...TEACHER_ROLE,
        id: "019535d8-0000-7000-8000-000000000003",
        name: "Trợ giảng",
        builtinKey: "assistant",
      },
    });
    serveUsers([helper]);
    serveDetail(detail(helper));
    renderUsers(`/admin/users?user=${helper.id}`);
    const panel = await sheet();
    expect(
      await within(panel).findByText(
        "Bạn không thể giao vai trò này, nên không thể đổi ở đây.",
      ),
    ).toBeVisible();
    expect(within(panel).queryByRole("button", { name: "Giáo viên" })).toBeNull();
  });

  it("draws no Actions heading when the server allows nothing (QC-US-6)", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH, { can: NONE }));
    renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await within(panel).findByText("Ngày tạo");
    expect(within(panel).queryByRole("heading", { name: "Thao tác" })).toBeNull();
  });

  it("sends Sign out everywhere once on a double click (QC-US-5)", async () => {
    let calls = 0;
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    server.use(
      http.post(`${BASE}/admin/users/:id/sign-out`, () => {
        calls += 1;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.dblClick(
      await within(panel).findByRole("button", { name: /Đăng xuất khỏi mọi thiết bị/ }),
    );
    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));
    expect(calls).toBe(1);
  });

  it("says an unknown account was not found, once and without Retry (QC-US-8)", async () => {
    let calls = 0;
    serveUsers([BINH]);
    server.use(
      http.get(`${BASE}/admin/users/:id`, () => {
        calls += 1;
        return refusal(404, "NOT_FOUND");
      }),
    );
    renderUsers(`/admin/users?user=${BINH.id}`);
    expect(await screen.findByText("Không tìm thấy tài khoản này.")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
    expect(calls).toBe(1);
  });

  it("leaves the classes and test lines out of Disable when there are none (QC-US-13)", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Vô hiệu hoá tài khoản/ }),
    );
    const confirm = await screen.findByRole("dialog", {
      name: "Vô hiệu hoá Trần Thị Bình?",
    });
    expect(within(confirm).queryByText(/Họ đang dạy/)).toBeNull();
    expect(
      within(confirm).queryByText("Họ đang làm bài kiểm tra ngay lúc này."),
    ).toBeNull();
  });

  it("offers no Transfer for a disabled account that owns nothing (QC-US-13)", async () => {
    const off = { ...BINH, status: "disabled" as const };
    serveUsers([off]);
    serveDetail(detail(off, { can: { ...ALL, disable: false, enable: true } }));
    renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await within(panel).findByRole("button", { name: /Kích hoạt lại tài khoản/ });
    expect(within(panel).queryByRole("button", { name: /Chuyển nội dung/ })).toBeNull();
  });

  it("keeps a temporary password through a Back that closes the sheet, and drops it on Done (QC-US-4)", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    server.use(
      http.post(`${BASE}/admin/users/:id/reset-password`, () =>
        contractJson("/admin/users/{id}/reset-password", "post", 200, {
          temporaryPassword: "Ngoc-Lan-42",
        }),
      ),
    );
    const { user: actor, router } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Đặt lại mật khẩu/ }),
    );
    const confirm = await screen.findByRole("dialog", {
      name: "Đặt lại mật khẩu của Trần Thị Bình?",
    });
    await actor.click(
      within(confirm).getByRole("radio", { name: /Hiện mật khẩu tạm thời/ }),
    );
    await actor.click(
      within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    expect(await screen.findByText("Ngoc-Lan-42")).toBeVisible();
    expect(
      screen.getByText("Họ phải đổi mật khẩu khi đăng nhập lần tới."),
    ).toBeVisible();
    await router.navigate("/admin/users");
    expect(screen.getByText("Ngoc-Lan-42")).toBeVisible();
    await actor.click(screen.getByRole("button", { name: "Xong" }));
    await waitFor(() => expect(screen.queryByText("Ngoc-Lan-42")).toBeNull());
  });

  it("lists the students the recipient still does not reach (QC-US-13)", async () => {
    const off = { ...BINH, status: "disabled" as const };
    serveUsers([off, CHI]);
    serveDetail(
      detail(off, {
        can: { ...ALL, disable: false, enable: true },
        ownedContent: { ...NOTHING, classes: 1 },
      }),
    );
    server.use(
      http.post(`${BASE}/admin/users/:id/transfer-content`, () =>
        contractJson("/admin/users/{id}/transfer-content", "post", 200, {
          moved: {
            classes: 1,
            tests: 0,
            questions: 0,
            questionGroups: 0,
            media: 0,
            assignments: 0,
          },
          leftBehind: { imports: 0, students: 3 },
        }),
      ),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Chuyển nội dung/ }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Chuyển nội dung của Trần Thị Bình cho",
    });
    await actor.click(await within(dialog).findByRole("radio", { name: /Lê Chi/ }));
    await actor.click(within(dialog).getByRole("button", { name: "Chuyển nội dung" }));
    expect(await screen.findByText(/^3 học viên do tài khoản này tạo/)).toBeVisible();
  });

  it("leaves out recipients whose role does not open the teacher workspace (QC-US-9)", async () => {
    const off = { ...BINH, status: "disabled" as const };
    const CONSOLE_ID = "019535d8-0000-7000-8000-0000000000b1";
    const clerk = user("8", "Phạm Quản Lý", {
      role: {
        ...ADMIN_ROLE,
        id: CONSOLE_ID,
        name: "Quản lý người dùng",
        builtinKey: null,
      },
    });
    serveUsers([off, CHI, clerk]);
    serveDetail(
      detail(off, {
        can: { ...ALL, disable: false, enable: true },
        ownedContent: { ...NOTHING, tests: 1 },
      }),
    );
    serveRoles(
      listOf([
        admin,
        teacher,
        {
          ...admin,
          id: CONSOLE_ID,
          builtinKey: null,
          custom: true,
          name: "Quản lý người dùng",
          permissions: ["people.users.manage"],
        },
      ]),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Chuyển nội dung/ }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Chuyển nội dung của Trần Thị Bình cho",
    });
    expect(await within(dialog).findByRole("radio", { name: /Lê Chi/ })).toBeVisible();
    expect(within(dialog).queryByRole("radio", { name: /Phạm Quản Lý/ })).toBeNull();
  });
});

describe("Users: the user sheet after QC's recheck (#724)", () => {
  async function showPassword(actor: ReturnType<typeof renderUsers>["user"]) {
    const panel = await sheet();
    const reset = await within(panel).findByRole("button", {
      name: /Đặt lại mật khẩu/,
    });
    await actor.click(reset);
    const confirm = await screen.findByRole("dialog", {
      name: "Đặt lại mật khẩu của Trần Thị Bình?",
    });
    await actor.click(
      within(confirm).getByRole("radio", { name: /Hiện mật khẩu tạm thời/ }),
    );
    await actor.click(
      within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    expect(await screen.findByText("Ngoc-Lan-42")).toBeVisible();
  }

  function serveReset() {
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    server.use(
      http.post(`${BASE}/admin/users/:id/reset-password`, () =>
        contractJson("/admin/users/{id}/reset-password", "post", 200, {
          temporaryPassword: "Ngoc-Lan-42",
        }),
      ),
    );
  }

  it("refuses to leave the page while a password is shown, and lets go after Done (QC-US-15)", async () => {
    serveReset();
    const { user: actor, router } = renderUsers(`/admin/users?user=${BINH.id}`);
    await showPassword(actor);
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(true);
    void router.navigate("/admin/roles");
    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith(
        "Hãy sao chép hoặc in mật khẩu rồi bấm Xong trước khi rời trang.",
      ),
    );
    expect(router.state.location.pathname).toBe("/admin/users");
    expect(screen.getByText("Ngoc-Lan-42")).toBeVisible();
    expect(screen.getByRole("button", { name: "Xong" })).toHaveFocus();
    await actor.click(screen.getByRole("button", { name: "Xong" }));
    await waitFor(() => expect(screen.queryByText("Ngoc-Lan-42")).toBeNull());
    const later = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(later);
    expect(later.defaultPrevented).toBe(false);
  });

  it("gives focus back to the role control when the role confirm is cancelled (QC-US-16)", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    const segment = await within(panel).findByRole("button", { name: "Quản trị viên" });
    await actor.click(segment);
    const confirm = await screen.findByRole("dialog", {
      name: "Chuyển Trần Thị Bình thành quản trị viên?",
    });
    await actor.click(within(confirm).getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(segment).toHaveFocus());
  });

  it("gives focus back to the role control after Disable opened from the owns-content answer (QC-US-16)", async () => {
    serveUsers([BINH]);
    serveDetail(
      detail(BINH, {
        can: { ...ALL, transfer: false },
        ownedContent: { ...NOTHING, classes: 1 },
      }),
    );
    server.use(
      http.put(`${BASE}/admin/users/:id/role`, () => refusal(409, "USER_OWNS_CONTENT")),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    const segment = await within(panel).findByRole("button", { name: "Học viên" });
    await actor.click(segment);
    const confirm = await screen.findByRole("dialog", {
      name: "Chuyển Trần Thị Bình thành học viên?",
    });
    await actor.click(within(confirm).getByRole("button", { name: "Đổi vai trò" }));
    await actor.click(
      await within(confirm).findByRole("button", { name: "Vô hiệu hoá tài khoản" }),
    );
    await screen.findByRole("dialog", { name: "Vô hiệu hoá Trần Thị Bình?" });
    await actor.keyboard("{Escape}");
    await waitFor(() => expect(segment).toHaveFocus());
  });

  it("says a malformed id was not found, without Retry (QC-US-18)", async () => {
    serveUsers([BINH]);
    server.use(
      http.get(`${BASE}/admin/users/:id`, () => refusal(400, "VALIDATION_FAILED")),
    );
    renderUsers(`/admin/users?user=${BINH.id}`);
    expect(await screen.findByText("Không tìm thấy tài khoản này.")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
  });

  it("puts the whole email in the header's tooltip (QC-US-18)", async () => {
    serveUsers([BINH]);
    serveDetail(detail(BINH));
    renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await within(panel).findByRole("button", { name: /Đặt lại mật khẩu/ });
    expect(within(panel).getByTitle(BINH.email)).toHaveTextContent(BINH.email);
  });

  it("keeps recipients whose role holds only content or student keys (QC-US-18)", async () => {
    const off = { ...BINH, status: "disabled" as const };
    const AUTHOR_ID = "019535d8-0000-7000-8000-0000000000b2";
    const TUTOR_ID = "019535d8-0000-7000-8000-0000000000b3";
    const custom = (id: string, name: string) =>
      user(id === AUTHOR_ID ? "8" : "9", name, {
        role: { ...ADMIN_ROLE, id, name, builtinKey: null },
      });
    serveUsers([off, custom(AUTHOR_ID, "Soạn đề"), custom(TUTOR_ID, "Trợ giảng")]);
    serveDetail(
      detail(off, {
        can: { ...ALL, disable: false, enable: true },
        ownedContent: { ...NOTHING, tests: 1 },
      }),
    );
    serveRoles(
      listOf([
        admin,
        teacher,
        {
          ...admin,
          id: AUTHOR_ID,
          builtinKey: null,
          custom: true,
          name: "Soạn đề",
          permissions: ["content.tests.write"],
        },
        {
          ...admin,
          id: TUTOR_ID,
          builtinKey: null,
          custom: true,
          name: "Trợ giảng",
          permissions: ["people.students.read"],
        },
      ]),
    );
    const { user: actor } = renderUsers(`/admin/users?user=${BINH.id}`);
    const panel = await sheet();
    await actor.click(
      await within(panel).findByRole("button", { name: /Chuyển nội dung/ }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Chuyển nội dung của Trần Thị Bình cho",
    });
    expect(
      await within(dialog).findAllByRole("radio", { name: /Soạn đề/ }),
    ).not.toHaveLength(0);
    expect(
      within(dialog).getAllByRole("radio", { name: /Trợ giảng/ }),
    ).not.toHaveLength(0);
  });
});
