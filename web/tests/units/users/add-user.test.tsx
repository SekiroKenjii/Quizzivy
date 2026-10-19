import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { toast } from "@/components/ui/sonner";
import type { components } from "@/lib/api/schema";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import {
  ADMIN_ROLE,
  BASE,
  STUDENT_ROLE,
  TEACHER_ROLE,
  classRow,
  renderUsers,
  serveClasses,
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
  serveClasses([]);
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

describe("Users: Add user after QC (#724)", () => {
  it("sends nothing when Create is pressed while the form is invalid", async () => {
    const sent: unknown[] = [];
    serveUsers([TAKEN]);
    serveCreate(sent);
    const { user: actor, dialog } = await openAdd();
    await actor.type(within(dialog).getByLabelText("Email"), "ha@example");
    await actor.click(within(dialog).getByRole("button", { name: "Tạo tài khoản" }));
    expect(await within(dialog).findByText("Nhập đầy đủ địa chỉ email")).toBeVisible();
    expect(sent).toEqual([]);
  });

  it("marks the field, not an alert, and focuses it when the email is taken on submit", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    const { user: actor, dialog } = await openAdd();
    await actor.type(within(dialog).getByLabelText("Họ và tên"), "Đỗ Hà");
    const email = within(dialog).getByLabelText("Email");
    await actor.type(email, "late@example.com");
    const create = within(dialog).getByRole("button", { name: "Tạo tài khoản" });
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await actor.click(create);
    await waitFor(() => expect(email).toHaveAttribute("aria-invalid", "true"));
    expect(email).toHaveFocus();
    expect(within(dialog).queryByRole("alert")).toBeNull();
  });

  it("keeps an email taken on submit taken in another case", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    const { user: actor, dialog } = await openAdd();
    await actor.type(within(dialog).getByLabelText("Họ và tên"), "Đỗ Hà");
    const email = within(dialog).getByLabelText("Email");
    await actor.type(email, "late@example.com");
    const create = within(dialog).getByRole("button", { name: "Tạo tài khoản" });
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await actor.click(create);
    expect(await within(dialog).findByText("Email này đã có người dùng")).toBeVisible();
    await actor.clear(email);
    await actor.type(email, "LATE@example.com");
    expect(within(dialog).getByText("Email này đã có người dùng")).toBeVisible();
    expect(create).toHaveAttribute("aria-disabled", "true");
  });

  it("opens the new account once a temporary password's Done is pressed", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    const { user: actor, dialog, router } = await openAdd();
    await actor.type(within(dialog).getByLabelText("Họ và tên"), "Đỗ Hà");
    await actor.type(within(dialog).getByLabelText("Email"), "ha@example.com");
    await actor.click(
      within(dialog).getByRole("radio", { name: /Hiện mật khẩu tạm thời/ }),
    );
    const create = within(dialog).getByRole("button", { name: "Tạo tài khoản" });
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await actor.click(create);
    expect(await screen.findByText("Mai-Hoa-17")).toBeVisible();
    expect(router.state.location.search).not.toContain("user=");
    await actor.click(screen.getByRole("button", { name: "Xong" }));
    await waitFor(() =>
      expect(router.state.location.search).toContain(
        "user=018f0000-0000-7000-8000-000000000005",
      ),
    );
  });
});

describe("Users: Add user's class (T-R5.22a)", () => {
  const CLASS = classRow("1", "IELTS 6.5 Tối", "Lê Thu");

  async function fill(
    actor: ReturnType<typeof renderUsers>["user"],
    dialog: HTMLElement,
  ) {
    await actor.type(within(dialog).getByLabelText("Họ và tên"), "Đỗ Hà");
    await actor.type(within(dialog).getByLabelText("Email"), "ha@example.com");
  }

  it("adds a new student to the class picked, and to none by default", async () => {
    const sent: components["schemas"]["UserCreate"][] = [];
    serveUsers([TAKEN]);
    serveCreate(sent);
    serveClasses([CLASS]);
    const { user: actor, dialog } = await openAdd();
    await fill(actor, dialog);
    const field = await within(dialog).findByRole("radiogroup", {
      name: "Thêm vào lớp (không bắt buộc)",
    });
    expect(
      within(field).getByRole("radio", { name: "Chưa vào lớp nào" }),
    ).toBeChecked();
    await actor.click(
      await within(field).findByRole("radio", { name: /IELTS 6.5 Tối/ }),
    );
    const create = within(dialog).getByRole("button", { name: "Tạo tài khoản" });
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await actor.click(create);
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0]).toMatchObject({ roleId: STUDENT_ROLE.id, classId: CLASS.id });
  });

  it("sends no class when none is picked", async () => {
    const sent: components["schemas"]["UserCreate"][] = [];
    serveUsers([TAKEN]);
    serveCreate(sent);
    serveClasses([CLASS]);
    const { user: actor, dialog } = await openAdd();
    await fill(actor, dialog);
    const create = within(dialog).getByRole("button", { name: "Tạo tài khoản" });
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await actor.click(create);
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0]).not.toHaveProperty("classId");
  });

  it("says a class that stopped taking students is gone, and drops the choice (QC A5)", async () => {
    const sent: components["schemas"]["UserCreate"][] = [];
    serveUsers([TAKEN]);
    serveCreate(sent);
    serveClasses([CLASS]);
    server.use(
      http.post(`${BASE}/admin/users`, async ({ request }) => {
        sent.push((await request.json()) as components["schemas"]["UserCreate"]);
        return HttpResponse.json(
          {
            error: {
              code: "NOT_FOUND",
              message: "class",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 404 },
        );
      }),
    );
    const { user: actor, dialog } = await openAdd();
    await fill(actor, dialog);
    const field = await within(dialog).findByRole("radiogroup", {
      name: "Thêm vào lớp (không bắt buộc)",
    });
    await actor.click(
      await within(field).findByRole("radio", { name: /IELTS 6.5 Tối/ }),
    );
    await actor.click(within(dialog).getByRole("button", { name: "Tạo tài khoản" }));
    expect(
      await within(dialog).findByText(
        "Lớp này không còn nhận học viên. Hãy chọn lớp khác.",
      ),
    ).toBeInTheDocument();
    expect(sent[0]).toMatchObject({ classId: CLASS.id });
    expect(
      within(field).getByRole("radio", { name: "Chưa vào lớp nào" }),
    ).toBeChecked();
    expect(screen.getByRole("dialog", { name: "Thêm người dùng" })).toBeInTheDocument();
  });

  it("offers no class for a teacher role", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    serveClasses([CLASS]);
    const { user: actor, dialog } = await openAdd();
    await within(dialog).findByRole("radiogroup", {
      name: "Thêm vào lớp (không bắt buộc)",
    });
    await actor.click(within(dialog).getByRole("button", { name: "Giáo viên" }));
    expect(
      within(dialog).queryByRole("radiogroup", {
        name: "Thêm vào lớp (không bắt buộc)",
      }),
    ).toBeNull();
  });

  it("offers no class to a caller without scope.all", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    serveClasses([CLASS]);
    const view = renderUsers("/admin/users", 1280, {
      ...adminUser,
      permissions: adminUser.permissions.filter((key) => key !== "scope.all"),
    });
    await table();
    await view.user.click(screen.getByRole("button", { name: "Thêm người dùng" }));
    const dialog = await screen.findByRole("dialog", { name: "Thêm người dùng" });
    expect(
      await within(dialog).findByRole("button", { name: "Học viên" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(
      within(dialog).queryByRole("radiogroup", {
        name: "Thêm vào lớp (không bắt buộc)",
      }),
    ).toBeNull();
  });
});

describe("Users: Add user's password dialog after QC's recheck (#724)", () => {
  async function created() {
    serveUsers([TAKEN]);
    serveCreate([]);
    const view = await openAdd();
    await view.user.type(within(view.dialog).getByLabelText("Họ và tên"), "Đỗ Hà");
    await view.user.type(within(view.dialog).getByLabelText("Email"), "ha@example.com");
    await view.user.click(
      within(view.dialog).getByRole("radio", { name: /Hiện mật khẩu tạm thời/ }),
    );
    const create = within(view.dialog).getByRole("button", { name: "Tạo tài khoản" });
    await waitFor(() => expect(create).not.toHaveAttribute("aria-disabled"));
    await view.user.click(create);
    expect(await screen.findByText("Mai-Hoa-17")).toBeVisible();
    return view;
  }

  it("keeps the password on Esc and on a click outside, and drops it on Done (QC-US-18)", async () => {
    const { user: actor } = await created();
    await actor.keyboard("{Escape}");
    expect(screen.getByText("Mai-Hoa-17")).toBeVisible();
    expect(screen.getByRole("button", { name: "Xong" })).toHaveFocus();
    const overlays = document.querySelectorAll('[data-slot="dialog-overlay"]');
    const overlay = overlays[overlays.length - 1]!;
    fireEvent.pointerDown(overlay);
    fireEvent.click(overlay);
    expect(screen.getByText("Mai-Hoa-17")).toBeVisible();
    await actor.click(screen.getByRole("button", { name: "Xong" }));
    await waitFor(() => expect(screen.queryByText("Mai-Hoa-17")).toBeNull());
  });

  it("announces the email's error line politely (QC-US-18)", async () => {
    serveUsers([TAKEN]);
    serveCreate([]);
    const { user: actor, dialog } = await openAdd();
    const email = within(dialog).getByLabelText("Email");
    await actor.type(email, "ha@example");
    await actor.tab();
    const line = within(dialog).getByText("Nhập đầy đủ địa chỉ email");
    expect(line).toHaveAttribute("aria-live", "polite");
    expect(email).toHaveAttribute("aria-describedby", line.id);
  });
});
