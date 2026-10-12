import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { assistantUser, teacherUser } from "@tests/support/fixtures";
import { viewport } from "@tests/support/viewport";
import {
  BASE,
  CLASS_A,
  CLASS_B,
  inClass,
  menuOf,
  renderStudents,
  rowOf,
  serveStudents,
  student,
  table,
} from "./support";
import "@/lib/i18n";

const AN = student("018f0000-0000-7000-8000-0000000000e1", "Nguyễn Văn An", {
  classes: [inClass(CLASS_A)],
  stats: {
    submittedCount: 3,
    flaggedCount: 0,
    score: { earned: 41, total: 50, pendingManual: 0 },
    activity: { live: false, lastAttemptAt: null },
  },
});
const BINH = student("018f0000-0000-7000-8000-0000000000e2", "Trần Thị Bình", {
  mustChangePassword: true,
});

afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
});

describe("the Students list", () => {
  it("says how many students across how many classes, and draws the deck's columns", async () => {
    serveStudents([AN, BINH]);
    renderStudents(teacherUser);

    expect(await screen.findByText("2 học viên trong 2 lớp.")).toBeVisible();
    const grid = await table();
    expect(
      within(grid)
        .getAllByRole("columnheader")
        .map((cell) => cell.textContent),
    ).toEqual(
      expect.arrayContaining([
        "Học viên",
        "Lớp",
        "Điểm TB",
        "Hoạt động cuối",
        "Tài khoản",
      ]),
    );
    const an = await rowOf("Nguyễn Văn An");
    expect(an).toHaveTextContent("82%");
    expect(an).toHaveTextContent(CLASS_A.name);
    expect(an).toHaveTextContent("Đang hoạt động");
    expect(await rowOf("Trần Thị Bình")).toHaveTextContent("Mật khẩu tạm");
  });

  it("keeps a student's classes on one line in their cell: two chips and +N naming the rest (QA-40-2)", async () => {
    const seven = Array.from({ length: 7 }, (_, index) => ({
      id: `018f0000-0000-7000-8000-0000000000b${index}`,
      name: `Lớp 14.${index + 1}`,
    }));
    serveStudents([
      student("018f0000-0000-7000-8000-0000000000e7", "Học viên Lớp 14", {
        classes: seven.map(inClass),
      }),
    ]);
    renderStudents(teacherUser);

    const row = await rowOf("Học viên Lớp 14");
    expect(within(row).getByText("Lớp 14.1")).toHaveAttribute("title", "Lớp 14.1");
    expect(within(row).getByText("Lớp 14.2")).toBeVisible();
    expect(within(row).queryByText("Lớp 14.3")).toBeNull();
    const more = within(row).getByText("+5").parentElement!;
    expect(more).toHaveAttribute(
      "title",
      "Lớp 14.3, Lớp 14.4, Lớp 14.5, Lớp 14.6, Lớp 14.7",
    );
    expect(more).toHaveTextContent(
      "và 5 lớp khác: Lớp 14.3, Lớp 14.4, Lớp 14.5, Lớp 14.6, Lớp 14.7",
    );
    expect(more.parentElement).toHaveClass("flex-nowrap", "overflow-hidden");
  });

  it("sends the class and status facets as the list's query", async () => {
    const asked = serveStudents([AN, BINH]);
    renderStudents(
      teacherUser,
      `/teacher/students?classId=${CLASS_A.id}&classId=${CLASS_B.id}&account=reset`,
    );

    await table();
    const last = asked.at(-1)!;
    expect(last.getAll("classId")).toEqual([CLASS_A.id, CLASS_B.id]);
    expect(last.get("mustChangePassword")).toBe("true");
    expect(last.get("status")).toBeNull();
  });

  it("lists suspended accounts beside open ones when Suspended is chosen too", async () => {
    const asked = serveStudents([AN]);
    renderStudents(teacherUser, "/teacher/students?account=active&account=suspended");

    await table();
    expect(asked.at(-1)!.get("status")).toBe("all");
    expect(asked.at(-1)!.get("mustChangePassword")).toBe("false");
  });

  it("offers Add student when there are no students yet", async () => {
    serveStudents([]);
    renderStudents(teacherUser);

    expect(await screen.findByText("Chưa có học viên nào.")).toBeVisible();
    expect(screen.getAllByRole("button", { name: "Thêm học viên" })).toHaveLength(2);
  });

  it("says the list could not load, with Retry", async () => {
    server.use(
      http.get(`${BASE}/teacher/students`, () =>
        HttpResponse.json(
          {
            error: {
              code: "INTERNAL",
              message: "Lỗi.",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 500 },
        ),
      ),
    );
    renderStudents(teacherUser);

    expect(await screen.findByText("Không tải được danh sách học viên.")).toBeVisible();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeVisible();
  });

  it("gives a teacher the row menu the deck draws, with no Send message", async () => {
    serveStudents([AN]);
    const { user } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent),
    ).toEqual(["Xem kết quả", "Chuyển lớp", "Đặt lại mật khẩu", "Xoá khỏi lớp"]);
  });

  it("opens the student's record from the row and from View results", async () => {
    serveStudents([AN]);
    const { user, router } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Xem kết quả" }));

    expect(await screen.findByRole("dialog", { name: "Nguyễn Văn An" })).toBeVisible();
    expect(router.state.location.search).toBe(`?studentId=${AN.id}`);
  });

  it("gives the Assistant, who only reads students, none of the controls (#285)", async () => {
    serveStudents([AN]);
    const { user } = renderStudents(assistantUser);

    const row = await rowOf("Nguyễn Văn An");
    expect(screen.queryByRole("button", { name: "Thêm học viên" })).toBeNull();
    expect(within(row).queryByRole("checkbox")).toBeNull();
    const menu = await menuOf(user, "Nguyễn Văn An");
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent),
    ).toEqual(["Xem kết quả"]);
    await user.keyboard("{Escape}");

    await user.click(within(row).getByText("Nguyễn Văn An"));
    const sheet = await screen.findByRole("dialog", { name: "Nguyễn Văn An" });
    expect(within(sheet).queryByRole("button", { name: "Sửa thông tin" })).toBeNull();
    expect(
      within(sheet).queryByRole("button", { name: "Đặt mật khẩu tạm" }),
    ).toBeNull();
    expect(within(sheet).queryByRole("button", { name: "Xoá khỏi lớp" })).toBeNull();
  });

  it("shows cards on a phone, without checkboxes or menus", async () => {
    viewport("phone");
    serveStudents([AN]);
    renderStudents(teacherUser);

    const list = await screen.findByRole("list", { name: "Học viên" });
    expect(within(list).getByText("Nguyễn Văn An")).toBeVisible();
    expect(within(list).getByText(/IELTS 6\.5 Tối · —/)).toBeVisible();
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Thao tác với Nguyễn Văn An" }),
    ).toBeNull();
  });

  it("drops the selection when the filters change", async () => {
    serveStudents([AN, BINH]);
    const { user } = renderStudents(teacherUser);

    const row = await rowOf("Nguyễn Văn An");
    await user.click(within(row).getByRole("checkbox", { name: "Chọn Nguyễn Văn An" }));
    expect(screen.getByText(/Đã chọn 1/)).toBeVisible();
    await user.type(screen.getByRole("searchbox"), "An");

    await waitFor(() => expect(screen.queryByText(/Đã chọn 1/)).toBeNull());
  });
});
