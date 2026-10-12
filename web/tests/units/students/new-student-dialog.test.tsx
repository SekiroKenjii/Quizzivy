import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { teacherUser } from "@tests/support/fixtures";
import {
  BASE,
  CLASS_A,
  inClass,
  renderStudents,
  serveStudents,
  student,
} from "./support";
import "@/lib/i18n";

const SECRET = "dua-hau-xanh-07";
const TRANG = student("018f0000-0000-7000-8000-0000000000e9", "Lê Thu Trang", {
  email: "trang@example.com",
  mustChangePassword: true,
});

let bodies: unknown[] = [];

beforeEach(() => {
  bodies = [];
  serveStudents([]);
  server.use(
    http.post(`${BASE}/teacher/students`, async ({ request }) => {
      const body = (await request.json()) as { classIds?: string[] };
      bodies.push(body);
      return contractJson("/teacher/students", "post", 201, {
        user: { ...TRANG, classes: body.classIds ? [inClass(CLASS_A)] : [] },
        temporaryPassword: SECRET,
      });
    }),
  );
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

async function openForm(user: ReturnType<typeof renderStudents>["user"]) {
  await screen.findByText("Chưa có học viên nào.");
  await user.click(screen.getAllByRole("button", { name: "Thêm học viên" })[0]!);
  return screen.findByRole("dialog", { name: "Thêm học viên" });
}

describe("adding a student", () => {
  it("asks for the full name, the email and a class, with No class yet first", async () => {
    const { user } = renderStudents(teacherUser);
    const form = await openForm(user);

    expect(within(form).getByRole("textbox", { name: "Họ và tên" })).toBeVisible();
    expect(within(form).getByRole("textbox", { name: "Email" })).toBeVisible();
    const klass = within(form).getByRole("combobox", { name: "Lớp" });
    expect(klass).toHaveTextContent("Chưa vào lớp");
    await user.click(klass);
    expect(
      (await screen.findAllByRole("option")).map((option) => option.textContent),
    ).toEqual(["Chưa vào lớp", CLASS_A.name, "IELTS Nền tảng A"]);
  });

  it("shows the temporary password in a dialog of its own, never in a toast", async () => {
    const { user } = renderStudents(teacherUser);
    const form = await openForm(user);

    await user.type(
      within(form).getByRole("textbox", { name: "Họ và tên" }),
      "Lê Thu Trang",
    );
    await user.type(
      within(form).getByRole("textbox", { name: "Email" }),
      "trang@example.com",
    );
    await user.click(within(form).getByRole("combobox", { name: "Lớp" }));
    await user.click(await screen.findByRole("option", { name: CLASS_A.name }));
    await user.click(within(form).getByRole("button", { name: "Tạo tài khoản" }));

    const sheet = await screen.findByRole("dialog", { name: "Đã tạo tài khoản" });
    expect(within(sheet).getByText(SECRET)).toBeVisible();
    expect(within(sheet).getByText("Lê Thu Trang")).toBeVisible();
    expect(bodies).toEqual([
      { fullName: "Lê Thu Trang", email: "trang@example.com", classIds: [CLASS_A.id] },
    ]);
    const toasts = document.querySelector("[data-sonner-toaster]");
    expect(toasts?.textContent ?? "").not.toContain(SECRET);
  });

  it("ignores Escape while the password is shown, and forgets it on Xong", async () => {
    const { user } = renderStudents(teacherUser);
    const form = await openForm(user);
    await user.type(
      within(form).getByRole("textbox", { name: "Họ và tên" }),
      "Lê Thu Trang",
    );
    await user.type(
      within(form).getByRole("textbox", { name: "Email" }),
      "trang@example.com",
    );
    await user.click(within(form).getByRole("button", { name: "Tạo tài khoản" }));
    const sheet = await screen.findByRole("dialog", { name: "Đã tạo tài khoản" });

    await user.keyboard("{Escape}");
    expect(within(sheet).getByText(SECRET)).toBeVisible();
    await user.click(within(sheet).getByRole("button", { name: "Xong" }));

    await waitFor(() => expect(document.body.innerHTML).not.toContain(SECRET));
  });

  it("gives focus back to Add student after Done (QA-40-1)", async () => {
    const { user } = renderStudents(teacherUser);
    const form = await openForm(user);
    await user.type(
      within(form).getByRole("textbox", { name: "Họ và tên" }),
      "Lê Thu Trang",
    );
    await user.type(
      within(form).getByRole("textbox", { name: "Email" }),
      "trang@example.com",
    );
    await user.click(within(form).getByRole("button", { name: "Tạo tài khoản" }));
    const sheet = await screen.findByRole("dialog", { name: "Đã tạo tài khoản" });
    await user.click(within(sheet).getByRole("button", { name: "Xong" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getAllByRole("button", { name: "Thêm học viên" })[0]).toHaveFocus();
  });

  it("opens empty again after a student was added", async () => {
    const { user } = renderStudents(teacherUser);
    let form = await openForm(user);
    await user.type(
      within(form).getByRole("textbox", { name: "Họ và tên" }),
      "Lê Thu Trang",
    );
    await user.type(
      within(form).getByRole("textbox", { name: "Email" }),
      "trang@example.com",
    );
    await user.click(within(form).getByRole("button", { name: "Tạo tài khoản" }));
    await user.click(
      within(await screen.findByRole("dialog", { name: "Đã tạo tài khoản" })).getByRole(
        "button",
        { name: "Xong" },
      ),
    );

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await user.click(screen.getAllByRole("button", { name: "Thêm học viên" })[0]!);
    form = await screen.findByRole("dialog", { name: "Thêm học viên" });
    expect(within(form).getByRole("textbox", { name: "Họ và tên" })).toHaveValue("");
  });
});
