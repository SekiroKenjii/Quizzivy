import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { teacherUser } from "@tests/support/fixtures";
import { BASE, menuOf, renderStudents, rowOf, serveStudents, student } from "./support";
import "@/lib/i18n";

const AN = student("018f0000-0000-7000-8000-0000000000e1", "Nguyễn Văn An");
const BINH = student("018f0000-0000-7000-8000-0000000000e2", "Trần Thị Bình");
const CUONG = student("018f0000-0000-7000-8000-0000000000e3", "Lê Văn Cường");
const DUNG = student("018f0000-0000-7000-8000-0000000000e4", "Phạm Tiến Dũng");
const EM = student("018f0000-0000-7000-8000-0000000000e5", "Võ Thị Em");
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const SECRET = "song-hoa-mai-42";

afterEach(() => {
  vi.restoreAllMocks();
  useAuthStore.getState().clearSession();
  localStorage.clear();
  sessionStorage.clear();
});

function resetsOne(answer: () => Response) {
  const asked: string[] = [];
  server.use(
    http.post(`${BASE}/teacher/students/:id/reset-password`, ({ params }) => {
      asked.push(String(params["id"]));
      return answer();
    }),
  );
  return asked;
}

const nowhereElse = (
  secret: string,
  client: ReturnType<typeof renderStudents>["client"],
) => {
  expect(
    JSON.stringify(
      client
        .getQueryCache()
        .getAll()
        .map((q) => q.state.data),
    ),
  ).not.toContain(secret);
  expect(
    JSON.stringify(
      client
        .getMutationCache()
        .getAll()
        .map((m) => m.state.data),
    ),
  ).not.toContain(secret);
  expect(JSON.stringify({ ...localStorage })).not.toContain(secret);
  expect(JSON.stringify({ ...sessionStorage })).not.toContain(secret);
};

describe("resetting one student's password", () => {
  it("asks first, then shows the password once with Copy and Print, and keeps it nowhere else", async () => {
    serveStudents([AN]);
    const asked = resetsOne(() =>
      contractJson("/teacher/students/{id}/reset-password", "post", 200, {
        temporaryPassword: SECRET,
      }),
    );
    const { user, router, client } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Đặt lại mật khẩu" }));
    const confirm = await screen.findByRole("dialog", {
      name: "Đặt lại mật khẩu cho Nguyễn Văn An?",
    });
    expect(confirm).toHaveTextContent("Việc này được ghi vào nhật ký kiểm tra.");
    expect(asked).toEqual([]);
    await user.click(within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }));

    const sheet = await screen.findByRole("dialog", {
      name: "Mật khẩu tạm của Nguyễn Văn An",
    });
    expect(asked).toEqual([AN.id]);
    expect(within(sheet).getByText(SECRET)).toBeVisible();
    expect(within(sheet).getByText(/chỉ hiện một lần/)).toBeVisible();
    expect(within(sheet).getByRole("button", { name: "Sao chép" })).toBeVisible();
    expect(within(sheet).getByRole("button", { name: "In" })).toBeVisible();
    const { pathname, search, hash } = router.state.location;
    expect(`${pathname}${search}${hash}`).not.toContain(SECRET);
    expect(screen.queryByRole("status", { name: /song-hoa/ })).toBeNull();
    nowhereElse(SECRET, client);
  });

  it("does not close on Escape, and forgets the password when Done closes it", async () => {
    serveStudents([AN]);
    resetsOne(() =>
      contractJson("/teacher/students/{id}/reset-password", "post", 200, {
        temporaryPassword: SECRET,
      }),
    );
    const { user } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Đặt lại mật khẩu" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Đặt lại mật khẩu",
      }),
    );
    const sheet = await screen.findByRole("dialog", { name: /Mật khẩu tạm/ });
    await user.keyboard("{Escape}");
    expect(within(sheet).getByText(SECRET)).toBeVisible();

    await user.click(within(sheet).getByRole("button", { name: "Xong" }));
    await waitFor(() => expect(screen.queryByText(SECRET)).toBeNull());
    expect(document.body.innerHTML).not.toContain(SECRET);
  });

  it("returns focus to the row's menu button after Done", async () => {
    serveStudents([AN]);
    resetsOne(() =>
      contractJson("/teacher/students/{id}/reset-password", "post", 200, {
        temporaryPassword: SECRET,
      }),
    );
    const { user } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Đặt lại mật khẩu" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Đặt lại mật khẩu",
      }),
    );
    const sheet = await screen.findByRole("dialog", { name: /Mật khẩu tạm/ });
    await user.click(within(sheet).getByRole("button", { name: "Xong" }));

    const row = await rowOf("Nguyễn Văn An");
    await waitFor(() =>
      expect(
        within(row).getByRole("button", { name: "Thao tác với Nguyễn Văn An" }),
      ).toHaveFocus(),
    );
  });

  it("says another teacher also teaches the student when the server refuses with STUDENT_SHARED", async () => {
    serveStudents([AN]);
    resetsOne(() =>
      HttpResponse.json(
        {
          error: {
            code: "STUDENT_SHARED",
            message: "Shared.",
            requestId: REQUEST_ID,
          },
        },
        { status: 403 },
      ),
    );
    const { user } = renderStudents(teacherUser);

    const menu = await menuOf(user, "Nguyễn Văn An");
    await user.click(within(menu).getByRole("menuitem", { name: "Đặt lại mật khẩu" }));
    const confirm = await screen.findByRole("dialog");
    await user.click(within(confirm).getByRole("button", { name: "Đặt lại mật khẩu" }));

    expect(await within(confirm).findByRole("alert")).toHaveTextContent(
      "Một giáo viên khác cũng dạy học viên này, nên chỉ quản trị viên đặt lại được mật khẩu",
    );
    expect(screen.queryByRole("dialog", { name: /Mật khẩu tạm/ })).toBeNull();
  });
});

describe("resetting several passwords at once", () => {
  async function selectAll(user: ReturnType<typeof renderStudents>["user"]) {
    await rowOf("Nguyễn Văn An");
    await user.click(screen.getByRole("checkbox", { name: /Chọn tất cả/ }));
  }

  it("sends one request, then lists each password and each student it did not reach, with why", async () => {
    serveStudents([AN, BINH, CUONG, DUNG, EM]);
    const bodies: unknown[] = [];
    server.use(
      http.post(`${BASE}/teacher/students/reset-passwords`, async ({ request }) => {
        bodies.push(await request.json());
        return contractJson("/teacher/students/reset-passwords", "post", 200, {
          items: [
            {
              studentId: AN.id,
              fullName: AN.fullName,
              email: AN.email,
              temporaryPassword: "mot-hai-ba-11",
            },
          ],
          failed: [
            { studentId: BINH.id, code: "NOT_FOUND" },
            { studentId: CUONG.id, code: "FORBIDDEN" },
            { studentId: DUNG.id, code: "STUDENT_SHARED" },
            { studentId: EM.id, code: "INTERNAL" },
          ],
        });
      }),
    );
    const print = vi.spyOn(window, "print").mockImplementation(() => undefined);
    const { user, client } = renderStudents(teacherUser);

    await selectAll(user);
    await user.click(screen.getByRole("button", { name: "Đặt lại mật khẩu" }));
    const confirm = await screen.findByRole("dialog", { name: "Đặt lại 5 mật khẩu?" });
    await user.click(within(confirm).getByRole("button", { name: "Đặt lại 5" }));

    const sheet = await screen.findByRole("dialog", { name: "1 mật khẩu tạm" });
    expect(bodies).toEqual([
      { studentIds: [AN.id, BINH.id, CUONG.id, DUNG.id, EM.id] },
    ]);
    expect(within(sheet).getByText("mot-hai-ba-11")).toBeVisible();
    expect(within(sheet).getByText("4 học viên chưa được đặt lại")).toBeVisible();
    expect(sheet).toHaveTextContent(
      "Trần Thị Bình · Không tìm thấy học viên trong các lớp của bạn",
    );
    expect(sheet).toHaveTextContent("Lê Văn Cường · Học viên có quyền mà bạn không có");
    expect(sheet).toHaveTextContent(
      "Phạm Tiến Dũng · Một giáo viên khác cũng dạy học viên này, nên chỉ quản trị viên đặt lại được mật khẩu",
    );
    expect(sheet).toHaveTextContent("Võ Thị Em · Có lỗi ở phía Quizzivy. Hãy thử lại");
    expect(sheet).toHaveAttribute("data-print", "credentials");
    await user.click(within(sheet).getByRole("button", { name: "In" }));
    expect(print).toHaveBeenCalledTimes(1);
    nowhereElse("mot-hai-ba-11", client);
  });

  it("refuses more than 40 students before asking the server", async () => {
    const many = Array.from({ length: 41 }, (_, index) =>
      student(
        `018f0000-0000-7000-8000-0000000001${String(index).padStart(2, "0")}`,
        `Học viên ${index}`,
      ),
    );
    serveStudents(many);
    const asked = vi.fn();
    server.use(http.post(`${BASE}/teacher/students/reset-passwords`, asked));
    const { user } = renderStudents(teacherUser);

    await rowOf("Học viên 0");
    await user.click(screen.getByRole("checkbox", { name: /Chọn tất cả/ }));
    await user.click(screen.getByRole("button", { name: "Đặt lại mật khẩu" }));

    expect(
      await screen.findByText(/Mỗi lần chỉ đặt lại được tối đa 40 mật khẩu/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(asked).not.toHaveBeenCalled();
  });
});
