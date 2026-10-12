import { afterEach, describe, expect, it } from "vitest";
import { screen, within } from "@testing-library/react";
import { useAuthStore } from "@/stores/auth";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import { renderStudents, rowOf, serveStudents, student } from "./support";
import "@/lib/i18n";

const DUNG = student("018f0000-0000-7000-8000-0000000000e2", "Hoàng Tiến Dũng");
const HAN = student("018f0000-0000-7000-8000-0000000000e1", "Phạm Gia Hân", {
  disabledAt: "2026-08-01T00:00:00Z",
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

async function openRecord(
  user: ReturnType<typeof renderStudents>["user"],
  name: string,
) {
  await user.click(within(await rowOf(name)).getByText(name));
  return screen.findByRole("dialog", { name });
}

describe("the Students controls that need people.users.manage", () => {
  it("a Teacher's record has a temporary password but no Suspend; an Admin's has both", async () => {
    serveStudents([DUNG]);
    const teacher = renderStudents(teacherUser);
    const teacherRecord = await openRecord(teacher.user, "Hoàng Tiến Dũng");
    expect(
      within(teacherRecord).getByRole("button", { name: "Đặt mật khẩu tạm" }),
    ).toBeInTheDocument();
    expect(
      within(teacherRecord).queryByRole("button", { name: "Khoá tài khoản" }),
    ).toBeNull();
    teacher.unmount();

    const admin = renderStudents(adminUser);
    const adminRecord = await openRecord(admin.user, "Hoàng Tiến Dũng");
    expect(
      within(adminRecord).getByRole("button", { name: "Khoá tài khoản" }),
    ).toBeInTheDocument();
  });

  it("a Teacher cannot restore or delete a suspended account; an Admin can", async () => {
    serveStudents([HAN]);
    const teacher = renderStudents(teacherUser);
    const teacherRecord = await openRecord(teacher.user, "Phạm Gia Hân");
    expect(within(teacherRecord).queryByRole("button", { name: "Mở khoá" })).toBeNull();
    expect(
      within(teacherRecord).queryByRole("button", { name: /Xoá vĩnh viễn/ }),
    ).toBeNull();
    teacher.unmount();

    const admin = renderStudents(adminUser);
    const adminRecord = await openRecord(admin.user, "Phạm Gia Hân");
    expect(
      within(adminRecord).getByRole("button", { name: "Mở khoá" }),
    ).toBeInTheDocument();
    expect(
      within(adminRecord).getByRole("button", { name: /Xoá vĩnh viễn/ }),
    ).toBeInTheDocument();
  });

  it("a Teacher's bulk bar moves, resets and removes; only an Admin's also suspends and deletes", async () => {
    serveStudents([DUNG]);
    const teacher = renderStudents(teacherUser);
    let row = await rowOf("Hoàng Tiến Dũng");
    await teacher.user.click(
      within(row).getByRole("checkbox", { name: "Chọn Hoàng Tiến Dũng" }),
    );
    expect(screen.getByRole("button", { name: "Chuyển lớp" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đặt lại mật khẩu" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Xoá khỏi lớp" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Vô hiệu hoá các mục đã chọn" }),
    ).toBeNull();
    expect(screen.queryByRole("button", { name: "Xoá vĩnh viễn" })).toBeNull();
    teacher.unmount();

    const admin = renderStudents(adminUser);
    row = await rowOf("Hoàng Tiến Dũng");
    await admin.user.click(
      within(row).getByRole("checkbox", { name: "Chọn Hoàng Tiến Dũng" }),
    );
    expect(
      screen.getByRole("button", { name: "Vô hiệu hoá các mục đã chọn" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Xoá vĩnh viễn" })).toBeInTheDocument();
  });
});
