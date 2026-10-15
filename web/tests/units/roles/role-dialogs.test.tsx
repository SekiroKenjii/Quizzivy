import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import i18n from "@/lib/i18n";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser } from "@tests/support/fixtures";
import { renderRolesPage } from "@tests/support/rolesPage";
import { server } from "@tests/support/server";
import {
  BASE,
  ROLES,
  TEACHER_ID,
  admin,
  head,
  listOf,
  serveRoles,
  student,
  teacher,
} from "@tests/support/roles";

const NEW_ROLE = "Vai trò mới";
const EDIT_ROLE = "Sửa vai trò";
const TAKEN = "Vai trò này đã tồn tại";
const OTHER_TAKEN = "Một vai trò khác đã dùng tên này";
const ADMIN_HINT = "Quản trị viên luôn giữ mọi quyền.";
const EVERYWHERE = "Thay đổi hiện ở mọi nơi vai trò này xuất hiện.";

const deputy = {
  ...head,
  id: "019535d8-0000-7000-8000-0000000000a2",
  name: "Tổ phó",
  description: "",
  userCount: 0,
  inProgressAttempts: 0,
  revision: 1,
};

async function openNew(user: ReturnType<typeof renderRolesPage>["user"]) {
  await user.click(await screen.findByRole("button", { name: NEW_ROLE }));
  return screen.findByRole("dialog", { name: NEW_ROLE });
}

async function openEdit(
  user: ReturnType<typeof renderRolesPage>["user"],
  name: string,
) {
  await user.click(await screen.findByRole("button", { name: `${EDIT_ROLE} ${name}` }));
  return screen.findByRole("dialog", { name: EDIT_ROLE });
}

function created(over: Partial<typeof deputy> = {}) {
  return {
    ...deputy,
    permissions: [],
    ...over,
  };
}

function serveCreate(answer: ReturnType<typeof created>) {
  const sent: unknown[] = [];
  server.use(
    http.post(`${BASE}/admin/roles`, async ({ request }) => {
      sent.push(await request.json());
      return contractJson("/admin/roles", "post", 201, answer);
    }),
  );
  return sent;
}

function serveUpdate(answer: (body: Record<string, unknown>) => typeof teacher) {
  const sent: Record<string, unknown>[] = [];
  server.use(
    http.patch(`${BASE}/admin/roles/:id`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      sent.push(body);
      return contractJson("/admin/roles/{id}", "patch", 200, answer(body));
    }),
  );
  return sent;
}

function refuse(path: string, method: "post" | "patch", status: number, code: string) {
  const handler = method === "post" ? http.post : http.patch;
  server.use(
    handler(`${BASE}${path}`, () =>
      HttpResponse.json(
        { error: { code, message: `Máy chủ: ${code}`, requestId: "r-1" } },
        { status },
      ),
    ),
  );
}

beforeEach(() => {
  Element.prototype.scrollTo = vi.fn();
});

afterEach(async () => {
  await i18n.changeLanguage("vi");
  vi.restoreAllMocks();
});

describe("New role", () => {
  it("offers a Vai trò mới button, outline, to whoever manages roles", async () => {
    serveRoles();
    renderRolesPage();
    expect(await screen.findByRole("button", { name: NEW_ROLE })).toBeVisible();
  });

  it("leaves the button out for someone who cannot manage roles", async () => {
    serveRoles();
    renderRolesPage({
      ...adminUser,
      permissions: adminUser.permissions.filter((key) => key !== "people.roles.manage"),
    });
    await screen.findByRole("grid");
    expect(screen.queryByRole("button", { name: NEW_ROLE })).toBeNull();
  });

  it("holds the button back until the roles have loaded", async () => {
    server.use(
      http.get(`${BASE}/admin/roles`, () => HttpResponse.json({}, { status: 500 })),
    );
    renderRolesPage();
    await screen.findByText("Không tải được danh sách vai trò.");
    expect(screen.queryByRole("button", { name: NEW_ROLE })).toBeNull();
  });

  it("asks for a name, who to copy from (none first) and a description", async () => {
    serveRoles();
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    expect(
      within(dialog).getByText(
        "Bắt đầu với quyền của vai trò bạn sao chép. Sau đó đổi trong bảng bên dưới.",
      ),
    ).toBeVisible();
    expect(within(dialog).getByRole("textbox", { name: /Tên vai trò/ })).toHaveFocus();
    const copy = within(dialog).getByRole("group", { name: "Sao chép quyền từ" });
    expect(
      within(copy)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Không sao chép", "Giáo viên", "Học viên", "Tổ trưởng bộ môn"]);
    expect(
      within(copy).getByRole("button", { name: "Không sao chép" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(within(dialog).getByRole("textbox", { name: /Mô tả/ })).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "Tạo vai trò" })).toBeVisible();
  });

  it("never offers the Admin or the Assistant to copy from, nor a role the caller cannot act on", async () => {
    serveRoles(listOf([admin, teacher, student, { ...head, editable: false }]));
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    const copy = within(dialog).getByRole("group", { name: "Sao chép quyền từ" });
    expect(
      within(copy)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Không sao chép", "Giáo viên", "Học viên"]);
  });

  it("turns the choice into a list once there are too many for segments", async () => {
    serveRoles(listOf([...ROLES, deputy]));
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    expect(
      within(dialog).queryByRole("group", { name: "Sao chép quyền từ" }),
    ).toBeNull();
    expect(
      within(dialog).getByRole("combobox", { name: "Sao chép quyền từ" }),
    ).toBeVisible();
  });

  it("creates the role with its name, source and description, then scrolls to its card", async () => {
    serveRoles();
    const sent = serveCreate(created());
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên vai trò/ }),
      "  Tổ phó ",
    );
    await user.click(within(dialog).getByRole("button", { name: "Giáo viên" }));
    await user.type(
      within(dialog).getByRole("textbox", { name: /Mô tả/ }),
      "Hỗ trợ tổ trưởng",
    );
    await user.click(within(dialog).getByRole("button", { name: "Tạo vai trò" }));
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0]).toEqual({
      name: "Tổ phó",
      description: "Hỗ trợ tổ trưởng",
      copyFromRoleId: TEACHER_ID,
    });
    expect(
      await screen.findByText(
        "Đã tạo vai trò Tổ phó. Cột của vai trò nằm trong bảng bên dưới.",
      ),
    ).toBeVisible();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(await screen.findByRole("article", { name: "Tổ phó" })).toBeVisible();
    expect(
      within(screen.getByRole("grid")).getByRole("button", { name: "Tổ phó" }),
    ).toBeVisible();
    await waitFor(() => expect(Element.prototype.scrollTo).toHaveBeenCalled());
    expect(screen.getByRole("button", { name: NEW_ROLE })).toHaveFocus();
  });

  it("sends only the name when nothing else is given", async () => {
    serveRoles();
    const sent = serveCreate(created());
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên vai trò/ }),
      "Tổ phó",
    );
    await user.click(within(dialog).getByRole("button", { name: "Tạo vai trò" }));
    await waitFor(() => expect(sent).toEqual([{ name: "Tổ phó" }]));
  });

  it("asks for a name before sending anything", async () => {
    serveRoles();
    const sent = serveCreate(created());
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    await user.click(within(dialog).getByRole("button", { name: "Tạo vai trò" }));
    expect(await within(dialog).findByText("Bắt buộc")).toBeVisible();
    expect(sent).toHaveLength(0);
  });

  it("refuses a name another role has, in any letter case, before the request", async () => {
    serveRoles();
    const sent = serveCreate(created());
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên vai trò/ }),
      "giáo VIÊN",
    );
    await user.click(within(dialog).getByRole("button", { name: "Tạo vai trò" }));
    expect(await within(dialog).findByText(TAKEN)).toBeVisible();
    expect(sent).toHaveLength(0);
  });

  it("refuses a built-in role's English name while the page reads English", async () => {
    await i18n.changeLanguage("en");
    serveRoles();
    const sent = serveCreate(created());
    const { user } = renderRolesPage();
    await user.click(await screen.findByRole("button", { name: "New role" }));
    const dialog = await screen.findByRole("dialog", { name: "New role" });
    await user.type(
      within(dialog).getByRole("textbox", { name: /Role name/ }),
      "teacher",
    );
    await user.click(within(dialog).getByRole("button", { name: "Create role" }));
    expect(
      await within(dialog).findByText("A role with this name already exists"),
    ).toBeVisible();
    expect(sent).toHaveLength(0);
  });

  it("says so when the server finds the name taken, and keeps what was typed", async () => {
    serveRoles();
    refuse("/admin/roles", "post", 409, "ROLE_NAME_TAKEN");
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên vai trò/ }),
      "Tổ phó",
    );
    await user.click(within(dialog).getByRole("button", { name: "Tạo vai trò" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(TAKEN);
    expect(within(dialog).getByRole("textbox", { name: /Tên vai trò/ })).toHaveValue(
      "Tổ phó",
    );
  });

  it("shows the server's reason and reloads the roles when the source is no longer the caller's to copy", async () => {
    const seen = serveRoles();
    refuse("/admin/roles", "post", 403, "TARGET_ROLE_NOT_ALLOWED");
    const { user } = renderRolesPage();
    const dialog = await openNew(user);
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên vai trò/ }),
      "Tổ phó",
    );
    await user.click(within(dialog).getByRole("button", { name: "Giáo viên" }));
    await user.click(within(dialog).getByRole("button", { name: "Tạo vai trò" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Máy chủ: TARGET_ROLE_NOT_ALLOWED",
    );
    await waitFor(() => expect(seen.reads).toBe(2));
  });

  it("starts empty each time it opens", async () => {
    serveRoles();
    const { user } = renderRolesPage();
    let dialog = await openNew(user);
    await user.type(
      within(dialog).getByRole("textbox", { name: /Tên vai trò/ }),
      "Tổ phó",
    );
    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    dialog = await openNew(user);
    expect(within(dialog).getByRole("textbox", { name: /Tên vai trò/ })).toHaveValue(
      "",
    );
  });
});

describe("Edit role", () => {
  it("puts a pencil on every card, named for its role", async () => {
    serveRoles();
    renderRolesPage();
    for (const name of [
      "Quản trị viên",
      "Giáo viên",
      "Trợ giảng",
      "Học viên",
      "Tổ trưởng bộ môn",
    ])
      expect(
        await screen.findByRole("button", { name: `${EDIT_ROLE} ${name}` }),
      ).toBeVisible();
  });

  it("opens with the role's twelve icons, name, description and seven colours", async () => {
    serveRoles();
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Giáo viên");
    expect(within(dialog).getByText(EVERYWHERE)).toBeVisible();
    const icons = within(dialog).getByRole("radiogroup", { name: "Biểu tượng" });
    expect(within(icons).getAllByRole("radio")).toHaveLength(12);
    expect(within(icons).getByRole("radio", { name: "Mũ cử nhân" })).toBeChecked();
    expect(within(dialog).getByRole("textbox", { name: /Tên/ })).toHaveValue(
      "Giáo viên",
    );
    expect(within(dialog).getByRole("textbox", { name: /Mô tả/ })).toHaveValue(
      "Soạn đề, quản lý lớp, chấm bài.",
    );
    const colours = within(dialog).getByRole("radiogroup", { name: "Màu thẻ" });
    expect(within(colours).getAllByRole("radio")).toHaveLength(7);
    expect(within(colours).getByRole("radio", { name: "Chanh" })).toBeChecked();
    expect(within(dialog).getByRole("button", { name: "Lưu vai trò" })).toBeVisible();
  });

  it("says in the Admin's dialog that the Admin keeps every permission", async () => {
    serveRoles();
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Quản trị viên");
    expect(within(dialog).getByText(ADMIN_HINT)).toBeVisible();
    expect(within(dialog).queryByText(EVERYWHERE)).toBeNull();
  });

  it("offers no way to delete a role", async () => {
    serveRoles();
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Tổ trưởng bộ môn");
    expect(
      within(dialog).queryByRole("button", { name: /Xoá|Xóa|Delete/i }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: /Xoá vai trò|Xóa vai trò/ }),
    ).toBeNull();
  });

  it("sends only what changed, with the revision it read, and redraws the card", async () => {
    serveRoles();
    const sent = serveUpdate((body) => ({
      ...teacher,
      revision: 4,
      icon: body["icon"] as typeof teacher.icon,
      color: body["color"] as typeof teacher.color,
    }));
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Giáo viên");
    await user.click(within(dialog).getByRole("radio", { name: "Vương miện" }));
    await user.click(within(dialog).getByRole("radio", { name: "Hồng" }));
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0]).toEqual({ expectedRevision: 3, icon: "crown", color: "rose" });
    expect(await screen.findByText("Đã lưu vai trò")).toBeVisible();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("article", { name: "Giáo viên" })).toHaveClass(
      "bg-danger-soft",
    );
    expect(
      screen.getByRole("button", { name: `${EDIT_ROLE} Giáo viên` }),
    ).toHaveFocus();
  });

  it("renames a role, shows the new name on its card and in the matrix", async () => {
    serveRoles();
    const sent = serveUpdate((body) => ({
      ...teacher,
      revision: 4,
      name: body["name"] as string,
    }));
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Giáo viên");
    const name = within(dialog).getByRole("textbox", { name: /Tên/ });
    await user.clear(name);
    await user.type(name, "  Giảng viên ");
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    await waitFor(() =>
      expect(sent).toEqual([{ expectedRevision: 3, name: "Giảng viên" }]),
    );
    expect(await screen.findByRole("article", { name: "Giảng viên" })).toBeVisible();
    expect(
      within(screen.getByRole("grid")).getByRole("button", { name: "Giảng viên" }),
    ).toBeVisible();
  });

  it("clears a description by sending an empty one", async () => {
    serveRoles();
    const sent = serveUpdate((body) => ({
      ...head,
      revision: 2,
      description: body["description"] as string,
    }));
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Tổ trưởng bộ môn");
    await user.clear(within(dialog).getByRole("textbox", { name: /Mô tả/ }));
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    await waitFor(() =>
      expect(sent).toEqual([{ expectedRevision: head.revision, description: "" }]),
    );
  });

  it("closes without a request when nothing changed", async () => {
    serveRoles();
    const sent = serveUpdate(() => teacher);
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Giáo viên");
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent).toHaveLength(0);
  });

  it("does not store a built-in role's English name or description when only its look changes", async () => {
    await i18n.changeLanguage("en");
    serveRoles();
    const sent = serveUpdate((body) => ({
      ...teacher,
      revision: 4,
      color: body["color"] as typeof teacher.color,
    }));
    const { user } = renderRolesPage();
    await user.click(await screen.findByRole("button", { name: "Edit role Teacher" }));
    const dialog = await screen.findByRole("dialog", { name: "Edit role" });
    expect(within(dialog).getByRole("textbox", { name: /Name/ })).toHaveValue(
      "Teacher",
    );
    expect(within(dialog).getByRole("textbox", { name: /Description/ })).toHaveValue(
      "Builds tests, runs classes, grades.",
    );
    await user.click(within(dialog).getByRole("radio", { name: "Blue" }));
    await user.click(within(dialog).getByRole("button", { name: "Save role" }));
    await waitFor(() => expect(sent).toEqual([{ expectedRevision: 3, color: "blue" }]));
  });

  it("asks for a name", async () => {
    serveRoles();
    const sent = serveUpdate(() => teacher);
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Giáo viên");
    await user.clear(within(dialog).getByRole("textbox", { name: /Tên/ }));
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    expect(await within(dialog).findByText("Bắt buộc")).toBeVisible();
    expect(sent).toHaveLength(0);
  });

  it("refuses another role's name but not its own in another case", async () => {
    serveRoles();
    const sent = serveUpdate((body) => ({
      ...teacher,
      revision: 4,
      name: body["name"] as string,
    }));
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Giáo viên");
    const name = within(dialog).getByRole("textbox", { name: /Tên/ });
    await user.clear(name);
    await user.type(name, "học VIÊN");
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    expect(await within(dialog).findByText(OTHER_TAKEN)).toBeVisible();
    expect(sent).toHaveLength(0);
    await user.clear(name);
    await user.type(name, "GIÁO VIÊN");
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    await waitFor(() =>
      expect(sent).toEqual([{ expectedRevision: 3, name: "GIÁO VIÊN" }]),
    );
  });

  it("says so when the server finds the name taken", async () => {
    serveRoles();
    refuse(`/admin/roles/${TEACHER_ID}`, "patch", 409, "ROLE_NAME_TAKEN");
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Giáo viên");
    const name = within(dialog).getByRole("textbox", { name: /Tên/ });
    await user.clear(name);
    await user.type(name, "Giảng viên");
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(OTHER_TAKEN);
  });

  it("reloads the roles on a stale save, keeps what was typed and saves again with the new revision", async () => {
    const seen = serveRoles();
    const sent: Record<string, unknown>[] = [];
    server.use(
      http.patch(`${BASE}/admin/roles/:id`, async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        sent.push(body);
        if (sent.length === 1) {
          seen.list = listOf([admin, { ...teacher, revision: 5 }, student, head]);
          return HttpResponse.json(
            { error: { code: "STALE_WRITE", message: "Cũ", requestId: "r-1" } },
            { status: 409 },
          );
        }
        return contractJson("/admin/roles/{id}", "patch", 200, {
          ...teacher,
          revision: 6,
          color: "blue",
        });
      }),
    );
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Giáo viên");
    await user.click(within(dialog).getByRole("radio", { name: "Xanh dương" }));
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Vai trò này vừa được thay đổi ở nơi khác. Đã tải lại; hãy xem rồi lưu lại.",
    );
    await waitFor(() => expect(seen.reads).toBe(2));
    expect(within(dialog).getByRole("radio", { name: "Xanh dương" })).toBeChecked();
    await user.click(within(dialog).getByRole("button", { name: "Lưu vai trò" }));
    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent[1]).toEqual({ expectedRevision: 5, color: "blue" });
  });

  it("keeps the pencil of a role the caller may not act on, disabled with the reason", async () => {
    serveRoles(listOf([{ ...admin, editable: false }, teacher, student, head]));
    const { user } = renderRolesPage();
    const pencil = await screen.findByRole("button", {
      name: `${EDIT_ROLE} Quản trị viên`,
    });
    expect(pencil).toHaveAttribute("aria-disabled", "true");
    expect(pencil).toHaveAttribute("title", "Bạn chỉ đổi được những quyền bạn có.");
    await user.click(pencil);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(
      screen.getByRole("button", { name: `${EDIT_ROLE} Giáo viên` }),
    ).not.toHaveAttribute("aria-disabled");
  });

  it("opens the dialog of the role pressed, not of one beside it", async () => {
    serveRoles();
    const { user } = renderRolesPage();
    const dialog = await openEdit(user, "Tổ trưởng bộ môn");
    expect(within(dialog).getByRole("textbox", { name: /Tên/ })).toHaveValue(
      "Tổ trưởng bộ môn",
    );
  });
});
