import { afterEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import i18n from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import {
  HEAD_ROLE,
  TEACHER_ROLE,
  renderUsers,
  rowOf,
  serveUsers,
  table,
  user,
} from "./support";

const AN = user("4", "Nguyễn Văn An");
const THUONG = user("1", "Thường Võ");
const GIAU = user("2", "Giàu Bùi", { role: TEACHER_ROLE });
const BINH = user("5", "Trần Thị Bình", { status: "must_change_password" });
const ROWS = [THUONG, GIAU, AN, BINH];

afterEach(async () => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  await act(() => i18n.changeLanguage("vi"));
});

describe("the count a screen reader hears", () => {
  it("says how many people the list holds", async () => {
    serveUsers(ROWS);
    renderUsers();
    const note = await screen.findByText("Tìm thấy 4 người dùng.");
    expect(note).toHaveAttribute("role", "status");
    expect(note).toHaveClass("sr-only");
  });

  it("says when no one is found", async () => {
    serveUsers([]);
    renderUsers();
    expect(await screen.findByText("Không tìm thấy người dùng nào.")).toHaveClass(
      "sr-only",
    );
  });

  it("is in the language of the page, one user and many", async () => {
    await act(() => i18n.changeLanguage("en"));
    serveUsers([THUONG]);
    renderUsers();
    expect(await screen.findByText("1 user found.")).toBeInTheDocument();
  });

  it("counts many users in English as well", async () => {
    await act(() => i18n.changeLanguage("en"));
    serveUsers(ROWS);
    renderUsers();
    expect(await screen.findByText("4 users found.")).toBeInTheDocument();
  });

  it("is the same element after a search, so the change is announced and not a new region", async () => {
    serveUsers(ROWS);
    const { user: person } = renderUsers();
    const note = await screen.findByText("Tìm thấy 4 người dùng.");

    await person.type(screen.getByRole("searchbox"), "ngu");
    await waitFor(() => expect(note).toHaveTextContent("Tìm thấy 1 người dùng."));
    expect(note).toBeInTheDocument();
  });

  it("follows the tab: the total is the tab's, not the list's", async () => {
    serveUsers(ROWS);
    const { user: person } = renderUsers();
    const note = await screen.findByText("Tìm thấy 4 người dùng.");

    await person.click(screen.getByRole("button", { name: /^Giáo viên/ }));
    await waitFor(() => expect(note).toHaveTextContent("Tìm thấy 1 người dùng."));
  });
});

describe("focus", () => {
  it("returns to the search box when the filters are cleared, with the box emptied", async () => {
    serveUsers([]);
    const { user: person } = renderUsers("/admin/users?tab=admins&q=zzz");
    const box = screen.getByRole("searchbox");
    expect(box).toHaveValue("zzz");
    await person.click(await screen.findByRole("button", { name: "Bỏ lọc" }));

    expect(box).toHaveFocus();
    await waitFor(() => expect(box).toHaveValue(""));
    expect(box).toHaveFocus();
  });

  it("returns to the search box when the state chip is removed", async () => {
    serveUsers(ROWS);
    const { user: person } = renderUsers("/admin/users?status=must_change_password");
    expect(await rowOf("Trần Thị Bình")).toBeVisible();
    await person.click(
      screen.getByRole("button", { name: "Bỏ lọc Phải đổi mật khẩu" }),
    );

    expect(screen.getByRole("searchbox")).toHaveFocus();
    expect(screen.queryByRole("group", { name: "Bộ lọc đang áp dụng" })).toBeNull();
  });
});

describe("the role's badge", () => {
  it("is written in the foreground colour, not the muted one", async () => {
    serveUsers([GIAU, user("9", "Hà Thu", { role: HEAD_ROLE })]);
    renderUsers("/admin/users", 1280);
    await table();
    for (const [name, label] of [
      ["Giàu Bùi", "Giáo viên"],
      ["Hà Thu", "Tổ trưởng bộ môn"],
    ] as const) {
      const badge = within(await rowOf(name))
        .getByText(label)
        .closest("[title]");
      expect(badge).toHaveClass("text-fg");
      expect(badge).not.toHaveClass("text-muted-foreground");
      expect(badge).not.toHaveClass("text-muted-fg");
    }
  });

  it("truncates a long name inside the badge, which never grows past its cell", async () => {
    const long = "Trưởng bộ phận khảo thí và kiểm định chất lượng đào tạo";
    serveUsers([user("9", "Hà Thu", { role: { ...HEAD_ROLE, name: long } })]);
    renderUsers("/admin/users", 1280);
    const label = within(await rowOf("Hà Thu")).getByText(long);
    expect(label).toHaveClass("min-w-0", "truncate");
    expect(label.closest("[title]")).toHaveClass("max-w-full");
    expect(label.closest("[title]")).toHaveAttribute("title", long);
  });
});
