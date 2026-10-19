import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { toast } from "@/components/ui/sonner";
import type { components } from "@/lib/api/schema";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import {
  ADMIN_ROLE,
  BASE,
  STUDENT_ROLE,
  TEACHER_ROLE,
  renderUsers,
  serveUsers,
  table,
  user,
} from "./support";

vi.mock("@/components/ui/sonner", async (original) => ({
  ...(await original<typeof import("@/components/ui/sonner")>()),
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }),
}));

const ROLES: components["schemas"]["AssignableRole"][] = [
  { ...ADMIN_ROLE, studentTarget: false },
  { ...TEACHER_ROLE, studentTarget: false },
  { ...STUDENT_ROLE, studentTarget: true },
];
const TAKEN = user("9", "Phạm Minh", { email: "minh@example.com" });

function serveCreate(sent: unknown[]) {
  server.use(
    http.get(`${BASE}/admin/roles/assignable`, () =>
      contractJson("/admin/roles/assignable", "get", 200, ROLES),
    ),
    http.get(`${BASE}/admin/users/:id`, ({ params }) =>
      contractJson("/admin/users/{id}", "get", 200, {
        user: user("5", "Đỗ Hà", { id: String(params.id) }),
        createdBy: null,
        classes: [],
        ownedContent: { classes: 0, tests: 0, questions: 0, media: 0, assignments: 0 },
        inProgressAttempts: 0,
        activity: [],
        can: {
          changeRole: true,
          resetPassword: true,
          disable: true,
          enable: false,
          signOut: true,
          transfer: true,
        },
      }),
    ),
    http.post(`${BASE}/admin/users`, async ({ request }) => {
      const body = (await request.json()) as components["schemas"]["UserCreate"];
      sent.push(body);
      if (body.email.toLowerCase() === "late@example.com")
        return HttpResponse.json(
          {
            error: {
              code: "EMAIL_TAKEN",
              message: "taken",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 409 },
        );
      const row = user("5", body.fullName, { email: body.email });
      return contractJson("/admin/users", "post", 201, {
        user: row,
        ...(body.delivery === "temporary_password"
          ? { temporaryPassword: "Mai-Hoa-17" }
          : {}),
      });
    }),
  );
}

async function openAdd() {
  const view = renderUsers();
  await table();
  await view.user.click(screen.getByRole("button", { name: "Thêm người dùng" }));
  const dialog = await screen.findByRole("dialog", { name: "Thêm người dùng" });
  return { ...view, dialog };
}

beforeEach(() => {
  vi.mocked(toast.success).mockClear();
});

describe("Users: Add user (T-R5.22b)", () => {
  it("creates a student by default and emails a link, then opens the account", async () => {
    const sent: unknown[] = [];
    serveUsers([TAKEN]);
    serveCreate(sent);
    const { user: actor, dialog, router } = await openAdd();
    const create = within(dialog).getByRole("button", { name: "Tạo tài khoản" });
    expect(create).toHaveAttribute("aria-disabled", "true");
    await actor.type(within(dialog).getByLabelText("Họ và tên"), "Đỗ Hà");
    await actor.type(within(dialog).getByLabelText("Email"), "ha@example.com");
    expect(
      await within(dialog).findByRole("button", { name: "Học viên" }),
    ).toHaveAttribute("aria-pressed", "true");
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await actor.click(create);
    await waitFor(() =>
      expect(sent).toEqual([
        {
          fullName: "Đỗ Hà",
          email: "ha@example.com",
          roleId: STUDENT_ROLE.id,
          delivery: "email_link",
        },
      ]),
    );
    expect(toast.success).toHaveBeenCalledWith(
      "Đã tạo tài khoản và gửi liên kết tới ha@example.com",
    );
    await waitFor(() => expect(router.state.location.search).toContain("user="));
  });

  it("says a partial email is partial, once the field is left", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    const { user: actor, dialog } = await openAdd();
    await actor.type(within(dialog).getByLabelText("Email"), "ha@example");
    expect(within(dialog).queryByText("Nhập đầy đủ địa chỉ email")).toBeNull();
    await actor.tab();
    expect(within(dialog).getByText("Nhập đầy đủ địa chỉ email")).toBeVisible();
    expect(within(dialog).getByLabelText("Email")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });

  it("says an email already in use is taken, before and on submit", async () => {
    const sent: unknown[] = [];
    serveUsers([TAKEN]);
    serveCreate(sent);
    const { user: actor, dialog } = await openAdd();
    await actor.type(within(dialog).getByLabelText("Họ và tên"), "Đỗ Hà");
    const email = within(dialog).getByLabelText("Email");
    await actor.type(email, "MINH@example.com");
    expect(await within(dialog).findByText("Email này đã có người dùng")).toBeVisible();
    await actor.clear(email);
    await actor.type(email, "late@example.com");
    const create = within(dialog).getByRole("button", { name: "Tạo tài khoản" });
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await actor.click(create);
    expect(await within(dialog).findByText("Email này đã có người dùng")).toBeVisible();
    expect(sent).toHaveLength(1);
  });

  it("shows a temporary password once for a teacher", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    const { user: actor, dialog } = await openAdd();
    await actor.type(within(dialog).getByLabelText("Họ và tên"), "Đỗ Hà");
    await actor.type(within(dialog).getByLabelText("Email"), "ha@example.com");
    await actor.click(await within(dialog).findByRole("button", { name: "Giáo viên" }));
    await actor.click(
      within(dialog).getByRole("radio", { name: /Hiện mật khẩu tạm thời/ }),
    );
    const create = within(dialog).getByRole("button", { name: "Tạo tài khoản" });
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await actor.click(create);
    expect(await screen.findByText("Mai-Hoa-17")).toBeVisible();
  });

  it("closes on Esc and gives focus back to its button", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    const { user: actor } = await openAdd();
    await actor.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: "Thêm người dùng" })).toHaveFocus();
  });
});
