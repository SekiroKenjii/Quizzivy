import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { toast } from "@/components/ui/sonner";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { adminUser } from "@tests/support/fixtures";
import {
  BASE,
  classRow,
  renderUsers,
  rowOf,
  serveClasses,
  serveUsers,
  table,
  user,
} from "./support";

vi.mock("@/components/ui/sonner", async (original) => ({
  ...(await original<typeof import("@/components/ui/sonner")>()),
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }),
}));

const ROWS = Array.from({ length: 12 }, (_, index) =>
  user(String(index + 1), `Người dùng ${String(index + 1).padStart(2, "0")}`),
);

type Sent = { userIds: string[]; delivery?: string };

function serveBulk(sent: Sent[], failedId?: string) {
  server.use(
    http.post(`${BASE}/admin/users/reset-passwords`, async ({ request }) => {
      const body = (await request.json()) as Sent;
      sent.push(body);
      const reached = body.userIds.filter((id) => id !== failedId);
      return contractJson("/admin/users/reset-passwords", "post", 200, {
        items: reached.map((id) => {
          const row = ROWS.find((one) => one.id === id)!;
          return {
            userId: id,
            fullName: row.fullName,
            email: row.email,
            ...(body.delivery === "temporary_password"
              ? { temporaryPassword: `Tam-${row.id.slice(-2)}` }
              : {}),
          };
        }),
        failed:
          failedId === undefined ? [] : [{ userId: failedId, code: "LAST_ADMIN" }],
      });
    }),
    http.post(`${BASE}/admin/users/disable`, async ({ request }) => {
      const body = (await request.json()) as Sent;
      sent.push(body);
      return contractJson("/admin/users/disable", "post", 200, {
        disabled: body.userIds,
        failed: [],
      });
    }),
  );
}

async function pick(actor: ReturnType<typeof renderUsers>["user"], name: string) {
  await actor.click(
    within(await rowOf(name)).getByRole("checkbox", { name: `Chọn ${name}` }),
  );
}

beforeEach(() => {
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.error).mockClear();
});

describe("Users: bulk actions (T-R5.22a)", () => {
  it("keeps a selection across pages and resets it with links", async () => {
    const sent: Sent[] = [];
    serveUsers(ROWS);
    serveBulk(sent);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await actor.click(screen.getByRole("link", { name: "Trang sau" }));
    await pick(actor, "Người dùng 11");
    expect(screen.getByText(/Đã chọn 2/)).toBeVisible();
    await actor.click(screen.getByRole("button", { name: "Đặt lại mật khẩu" }));
    const confirm = await screen.findByRole("dialog", { name: "Đặt lại 2 mật khẩu?" });
    await actor.click(
      within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    await waitFor(() =>
      expect(sent).toEqual([
        { userIds: [ROWS[0]!.id, ROWS[10]!.id], delivery: "email_link" },
      ]),
    );
    expect(toast.success).toHaveBeenCalledWith("Đã gửi 2 liên kết đặt mật khẩu");
    await waitFor(() => expect(screen.queryByText(/Đã chọn 2/)).toBeNull());
  });

  it("refuses more than 40 temporary passwords before it calls the server", async () => {
    const many = Array.from({ length: 41 }, (_, index) =>
      user(String(index + 1), `Học viên ${String(index + 1).padStart(2, "0")}`),
    );
    const sent: Sent[] = [];
    serveUsers(many);
    serveBulk(sent);
    const { user: actor } = renderUsers("/admin/users?size=50");
    await table();
    await actor.click(screen.getByRole("checkbox", { name: /Chọn tất cả/ }));
    await actor.click(screen.getByRole("button", { name: "Đặt lại mật khẩu" }));
    const confirm = await screen.findByRole("dialog", { name: "Đặt lại 41 mật khẩu?" });
    await actor.click(
      within(confirm).getByRole("radio", { name: /Hiện mật khẩu tạm thời/ }),
    );
    expect(within(confirm).getByText(/tối đa 40 mật khẩu tạm thời/)).toBeVisible();
    await actor.click(
      within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    expect(sent).toEqual([]);
  });

  it("shows temporary passwords once and keeps an account it did not reach selected", async () => {
    const sent: Sent[] = [];
    serveUsers(ROWS);
    serveBulk(sent, ROWS[1]!.id);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await pick(actor, "Người dùng 02");
    await actor.click(screen.getByRole("button", { name: "Đặt lại mật khẩu" }));
    const confirm = await screen.findByRole("dialog", { name: "Đặt lại 2 mật khẩu?" });
    await actor.click(
      within(confirm).getByRole("radio", { name: /Hiện mật khẩu tạm thời/ }),
    );
    await actor.click(
      within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    const result = await screen.findByRole("dialog", {
      name: "Kết quả đặt lại mật khẩu",
    });
    expect(within(result).getByText("Tam-01")).toBeVisible();
    expect(within(result).getByText("1 người chưa được xử lý")).toBeVisible();
    expect(
      within(result).getByText("Đây là quản trị viên đang hoạt động cuối cùng."),
    ).toBeVisible();
    await actor.keyboard("{Escape}");
    expect(within(result).getByRole("button", { name: "Xong" })).toHaveFocus();
    await actor.click(within(result).getByRole("button", { name: "Xong" }));
    await waitFor(() => expect(screen.queryByText("Tam-01")).toBeNull());
    expect(screen.getByText(/Đã chọn 1/)).toBeVisible();
  });

  it("disables the selection after a danger confirmation", async () => {
    const sent: Sent[] = [];
    serveUsers(ROWS);
    serveBulk(sent);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 03");
    await actor.click(screen.getByRole("button", { name: "Vô hiệu hoá" }));
    const confirm = await screen.findByRole("dialog", {
      name: "Vô hiệu hoá 1 tài khoản?",
    });
    await actor.click(within(confirm).getByRole("button", { name: "Vô hiệu hoá" }));
    await waitFor(() => expect(sent).toEqual([{ userIds: [ROWS[2]!.id] }]));
    expect(toast.success).toHaveBeenCalledWith("Đã vô hiệu hoá 1 tài khoản");
  });

  it("clears the selection when the tab changes", async () => {
    serveUsers(ROWS);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    expect(screen.getByText(/Đã chọn 1/)).toBeVisible();
    await actor.click(screen.getByRole("button", { name: /Học viên/ }));
    await waitFor(() => expect(screen.queryByText(/Đã chọn 1/)).toBeNull());
  });
});

const CLASSES = [
  classRow("1", "IELTS 6.5 Tối", "Lê Thu"),
  classRow("2", "TOEIC 600 Cuối tuần", "Ngô Nam"),
];

function serveMembers(sent: { classId: string; userId: string }[], refusedId?: string) {
  server.use(
    http.post(`${BASE}/teacher/classes/:id/members`, async ({ params, request }) => {
      const { userId } = (await request.json()) as { userId: string };
      sent.push({ classId: String(params.id), userId });
      if (userId === refusedId)
        return HttpResponse.json(
          {
            error: {
              code: "NOT_FOUND",
              message: "not found",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 404 },
        );
      return HttpResponse.json({}, { status: 201 });
    }),
  );
}

describe("Users: Add to class (T-R5.22a)", () => {
  it("adds the selection to the class the Admin searched for", async () => {
    const sent: { classId: string; userId: string }[] = [];
    serveUsers(ROWS);
    serveClasses(CLASSES);
    serveMembers(sent);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await pick(actor, "Người dùng 02");
    await actor.click(screen.getByRole("button", { name: "Thêm vào lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm 2 người vào một lớp",
    });
    expect(
      await within(dialog).findByRole("radio", { name: /IELTS 6.5 Tối/ }),
    ).toBeVisible();
    await actor.type(within(dialog).getByRole("searchbox"), "Ngô");
    await waitFor(() =>
      expect(within(dialog).queryByRole("radio", { name: /IELTS/ })).toBeNull(),
    );
    await actor.click(within(dialog).getByRole("radio", { name: /TOEIC 600/ }));
    await actor.click(within(dialog).getByRole("button", { name: "Thêm vào lớp" }));
    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent).toEqual(
      expect.arrayContaining([
        { classId: CLASSES[1]!.id, userId: ROWS[0]!.id },
        { classId: CLASSES[1]!.id, userId: ROWS[1]!.id },
      ]),
    );
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith(
        "Đã thêm 2 người vào TOEIC 600 Cuối tuần",
      ),
    );
    await waitFor(() => expect(screen.queryByText(/Đã chọn 2/)).toBeNull());
  });

  it("asks for a class before it adds anyone", async () => {
    const sent: { classId: string; userId: string }[] = [];
    serveUsers(ROWS);
    serveClasses(CLASSES);
    serveMembers(sent);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await actor.click(screen.getByRole("button", { name: "Thêm vào lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm 1 người vào một lớp",
    });
    await actor.click(within(dialog).getByRole("button", { name: "Thêm vào lớp" }));
    expect(within(dialog).getByText("Hãy chọn một lớp.")).toBeVisible();
    expect(sent).toEqual([]);
  });

  it("names each account the class did not take and keeps it selected", async () => {
    const sent: { classId: string; userId: string }[] = [];
    serveUsers(ROWS);
    serveClasses(CLASSES);
    serveMembers(sent, ROWS[1]!.id);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await pick(actor, "Người dùng 02");
    await actor.click(screen.getByRole("button", { name: "Thêm vào lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm 2 người vào một lớp",
    });
    await actor.click(
      await within(dialog).findByRole("radio", { name: /IELTS 6.5 Tối/ }),
    );
    await actor.click(within(dialog).getByRole("button", { name: "Thêm vào lớp" }));
    const result = await screen.findByRole("dialog", { name: "Kết quả thêm vào lớp" });
    expect(within(result).getByText("1 người chưa được xử lý")).toBeVisible();
    expect(within(result).getByText(/Người dùng 02/)).toBeVisible();
    expect(
      within(result).getByText("Chỉ thêm được học viên đang hoạt động."),
    ).toBeVisible();
    await actor.click(within(result).getByRole("button", { name: "Xong" }));
    expect(screen.getByText(/Đã chọn 1/)).toBeVisible();
  });

  it("is not offered to a caller without scope.all", async () => {
    serveUsers(ROWS);
    const { user: actor } = renderUsers("/admin/users", 1280, {
      ...adminUser,
      permissions: adminUser.permissions.filter((key) => key !== "scope.all"),
    });
    await pick(actor, "Người dùng 01");
    expect(screen.getByRole("button", { name: "Đặt lại mật khẩu" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Thêm vào lớp" })).toBeNull();
  });
});

describe("Users: bulk actions after QC (#735)", () => {
  it("selects from the card list on a phone, by card and by page (QC-UB-1)", async () => {
    serveUsers(ROWS);
    const { user: actor } = renderUsers("/admin/users", 360);
    const list = await screen.findByRole("list", { name: "Người dùng" });
    await actor.click(
      within(list).getByRole("checkbox", { name: "Chọn Người dùng 01" }),
    );
    expect(screen.getByText(/Đã chọn 1/)).toBeVisible();
    expect(screen.getByText("Chọn trang")).toBeVisible();
    await actor.click(screen.getByRole("checkbox", { name: /Chọn tất cả/ }));
    expect(screen.getByText(/Đã chọn 10/)).toBeVisible();
    expect(screen.getByRole("button", { name: "Thêm vào lớp" })).toBeVisible();
  });

  it("refuses more than 100 before any call (QC-UB-8)", async () => {
    const many = Array.from({ length: 101 }, (_, index) =>
      user(String(index + 1), `Bạn ${String(index + 1).padStart(3, "0")}`),
    );
    const sent: Sent[] = [];
    serveUsers(many);
    serveBulk(sent);
    const { user: actor } = renderUsers("/admin/users?size=50");
    await table();
    await actor.click(screen.getByRole("checkbox", { name: /Chọn tất cả/ }));
    await actor.click(screen.getByRole("link", { name: "Trang sau" }));
    await rowOf("Bạn 051");
    await actor.click(screen.getByRole("checkbox", { name: /Chọn tất cả/ }));
    await actor.click(screen.getByRole("link", { name: "Trang sau" }));
    await rowOf("Bạn 101");
    await actor.click(screen.getByRole("checkbox", { name: /Chọn tất cả/ }));
    expect(screen.getByText(/Đã chọn 101/)).toBeVisible();
    await actor.click(screen.getByRole("button", { name: "Vô hiệu hoá" }));
    expect(toast.error).toHaveBeenCalledWith("Chọn tối đa 100 người cho một thao tác.");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(sent).toEqual([]);
  });

  it("says how many it disabled beside those it did not (QC-UB-2)", async () => {
    serveUsers(ROWS);
    server.use(
      http.post(`${BASE}/admin/users/disable`, async ({ request }) => {
        const body = (await request.json()) as Sent;
        return contractJson("/admin/users/disable", "post", 200, {
          disabled: body.userIds.slice(1),
          failed: [{ userId: body.userIds[0]!, code: "CANNOT_ACT_ON_SELF" }],
        });
      }),
    );
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await pick(actor, "Người dùng 02");
    await pick(actor, "Người dùng 03");
    await actor.click(screen.getByRole("button", { name: "Vô hiệu hoá" }));
    const confirm = await screen.findByRole("dialog", {
      name: "Vô hiệu hoá 3 tài khoản?",
    });
    await actor.click(within(confirm).getByRole("button", { name: "Vô hiệu hoá" }));
    const result = await screen.findByRole("dialog", { name: "Kết quả vô hiệu hoá" });
    expect(within(result).getByText("Đã vô hiệu hoá 2 tài khoản")).toBeVisible();
    expect(within(result).getByText("Người dùng 01")).toBeVisible();
    expect(toast.success).not.toHaveBeenCalled();
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(false);
  });

  it("names the account a link reset did not reach, with the links sent (QC-UB-8)", async () => {
    const sent: Sent[] = [];
    serveUsers(ROWS);
    serveBulk(sent, ROWS[1]!.id);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await pick(actor, "Người dùng 02");
    await actor.click(screen.getByRole("button", { name: "Đặt lại mật khẩu" }));
    const confirm = await screen.findByRole("dialog", { name: "Đặt lại 2 mật khẩu?" });
    await actor.click(
      within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    const result = await screen.findByRole("dialog", {
      name: "Kết quả đặt lại mật khẩu",
    });
    expect(within(result).getByText("Đã gửi 1 liên kết đặt mật khẩu")).toBeVisible();
    expect(within(result).getByText("Người dùng 02")).toBeVisible();
    expect(toast.success).not.toHaveBeenCalled();
  });
});

describe("Users: Add to class after QC (#735)", () => {
  it("runs at most four additions at once and counts them (QC-UB-4, QC-UB-8)", async () => {
    const many = Array.from({ length: 6 }, (_, index) =>
      user(String(index + 1), `Học viên ${String(index + 1)}`),
    );
    serveUsers(many);
    serveClasses(CLASSES);
    let inFlight = 0;
    let most = 0;
    const gates: (() => void)[] = [];
    server.use(
      http.post(`${BASE}/teacher/classes/:id/members`, async () => {
        inFlight += 1;
        most = Math.max(most, inFlight);
        await new Promise<void>((resolve) => gates.push(resolve));
        inFlight -= 1;
        return HttpResponse.json({}, { status: 201 });
      }),
    );
    const { user: actor } = renderUsers();
    await table();
    await actor.click(screen.getByRole("checkbox", { name: /Chọn tất cả/ }));
    await actor.click(screen.getByRole("button", { name: "Thêm vào lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm 6 người vào một lớp",
    });
    await actor.click(
      await within(dialog).findByRole("radio", { name: /IELTS 6.5 Tối/ }),
    );
    await actor.click(within(dialog).getByRole("button", { name: "Thêm vào lớp" }));
    await waitFor(() => expect(gates).toHaveLength(4));
    expect(within(dialog).getByRole("status")).toHaveTextContent(
      "Đang thêm 0/6 người…",
    );
    expect(within(dialog).getByRole("button", { name: "Huỷ" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    await actor.keyboard("{Escape}");
    expect(
      screen.getByRole("dialog", { name: "Thêm 6 người vào một lớp" }),
    ).toBeVisible();
    gates.shift()!();
    await waitFor(() =>
      expect(within(dialog).getByRole("status")).toHaveTextContent(
        "Đang thêm 1/6 người…",
      ),
    );
    while (gates.length > 0 || inFlight > 0) {
      gates.shift()?.();
      await new Promise((resolve) => setTimeout(resolve, 10));
    }
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("Đã thêm 6 người vào IELTS 6.5 Tối"),
    );
    expect(most).toBe(4);
  });

  it("says how many it added and why the others were refused (QC-UB-2, QC-UB-5)", async () => {
    serveUsers(ROWS);
    serveClasses(CLASSES);
    server.use(
      http.post(`${BASE}/teacher/classes/:id/members`, async ({ request }) => {
        const { userId } = (await request.json()) as { userId: string };
        const refuse = (status: number, code: string) =>
          HttpResponse.json(
            {
              error: {
                code,
                message: code,
                requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
              },
            },
            { status },
          );
        if (userId === ROWS[1]!.id) return refuse(429, "RATE_LIMITED");
        if (userId === ROWS[2]!.id) return refuse(403, "FORBIDDEN");
        return HttpResponse.json({}, { status: 201 });
      }),
    );
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await pick(actor, "Người dùng 02");
    await pick(actor, "Người dùng 03");
    await actor.click(screen.getByRole("button", { name: "Thêm vào lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm 3 người vào một lớp",
    });
    await actor.click(
      await within(dialog).findByRole("radio", { name: /IELTS 6.5 Tối/ }),
    );
    await actor.click(within(dialog).getByRole("button", { name: "Thêm vào lớp" }));
    const result = await screen.findByRole("dialog", { name: "Kết quả thêm vào lớp" });
    expect(within(result).getByText("Đã thêm 1 người vào IELTS 6.5 Tối")).toBeVisible();
    expect(
      within(result).getByText("Bạn thao tác quá nhanh. Hãy thử lại sau ít phút."),
    ).toBeVisible();
    expect(
      within(result).getByText("Bạn không có quyền làm việc này với tài khoản này."),
    ).toBeVisible();
  });

  it("fades Add to class until a class is picked (QC-UB-3)", async () => {
    serveUsers(ROWS);
    serveClasses(CLASSES);
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await actor.click(screen.getByRole("button", { name: "Thêm vào lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm 1 người vào một lớp",
    });
    const submit = within(dialog).getByRole("button", { name: "Thêm vào lớp" });
    expect(submit).toHaveAttribute("aria-disabled", "true");
    expect(submit.className).toContain("aria-disabled:bg-primary/50");
    await actor.click(
      await within(dialog).findByRole("radio", { name: /IELTS 6.5 Tối/ }),
    );
    expect(submit).not.toHaveAttribute("aria-disabled");
  });

  it("says the classes are loading, offers Retry when they fail, and that a search is too short (QC-UB-6)", async () => {
    serveUsers(ROWS);
    let fail = true;
    let release: () => void = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${BASE}/admin/classes`, async () => {
        await held;
        if (fail)
          return HttpResponse.json(
            {
              error: {
                code: "INTERNAL",
                message: "x",
                requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
              },
            },
            { status: 500 },
          );
        return contractJson("/admin/classes", "get", 200, {
          items: CLASSES,
          total: 2,
          page: 1,
          pageSize: 10,
          facets: { active: 2, archived: 0 },
          disabledTeacherClasses: 0,
        });
      }),
    );
    const { user: actor } = renderUsers();
    await pick(actor, "Người dùng 01");
    await actor.click(screen.getByRole("button", { name: "Thêm vào lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Thêm 1 người vào một lớp",
    });
    expect(within(dialog).getByText("Đang tải danh sách lớp…")).toBeVisible();
    release();
    expect(
      await within(dialog).findByText("Không tải được danh sách lớp. Hãy thử lại."),
    ).toBeVisible();
    fail = false;
    await actor.click(within(dialog).getByRole("button", { name: "Thử lại" }));
    expect(
      await within(dialog).findByRole("radio", { name: /IELTS 6.5 Tối/ }),
    ).toBeVisible();
    await actor.type(within(dialog).getByRole("searchbox"), "IE");
    expect(within(dialog).getByText("Nhập ít nhất 3 ký tự để tìm kiếm.")).toBeVisible();
  });
});
