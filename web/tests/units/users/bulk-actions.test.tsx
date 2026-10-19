import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { toast } from "@/components/ui/sonner";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { BASE, renderUsers, rowOf, serveUsers, table, user } from "./support";

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
