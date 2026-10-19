import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { ADMIN, setPasswordToken, signIn } from "./support/live";

test.use({ actionTimeout: 10_000 });

const AUDIO = fileURLToPath(new URL("./fixtures/unit5-listening.mp3", import.meta.url));
const MEDIA_WRITE_FOR_TEACHER = "Tải lên và xóa tệp phương tiện — Giáo viên";

async function grantTeacherMediaWrite(owner: Page, granted: boolean) {
  await owner.goto("/admin/roles");
  const cell = owner.getByRole("checkbox", {
    name: MEDIA_WRITE_FOR_TEACHER,
    exact: true,
  });
  await expect(cell).toBeVisible();
  if ((await cell.isChecked()) === granted) return;
  await cell.click();
  await expect(cell).toBeChecked({ checked: granted });
  const [saved] = await Promise.all([
    owner.waitForResponse(
      (response) =>
        response.request().method() === "PUT" &&
        new URL(response.url()).pathname.endsWith("/admin/roles/permissions"),
    ),
    owner.getByRole("button", { name: "Lưu thay đổi", exact: true }).click(),
  ]);
  expect(saved.ok(), "the matrix is saved").toBe(true);
  await expect(owner.getByText("Đã lưu quyền", { exact: true })).toBeVisible();
}

async function uploadAudio(teacher: Page): Promise<number> {
  await teacher.getByRole("button", { name: "Tải lên", exact: true }).first().click();
  const dialog = teacher.getByRole("dialog", { name: "Tải lên media" });
  const [chooser] = await Promise.all([
    teacher.waitForEvent("filechooser"),
    dialog.getByRole("button", { name: "Tệp", exact: true }).click(),
  ]);
  await chooser.setFiles(AUDIO);
  const [answer] = await Promise.all([
    teacher.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname.endsWith("/teacher/media"),
      { timeout: 60_000 },
    ),
    dialog.getByRole("button", { name: "Tải lên", exact: true }).click(),
  ]);
  return answer.status();
}

test("the owner adds a teacher by email, narrows Teacher's rights and disables the account", async ({
  browser,
  page: owner,
}) => {
  test.setTimeout(240_000);
  const stamp = Date.now();
  const teacherName = `Giáo viên live ${stamp}`;
  const teacherEmail = `gv.live.${stamp}@example.com`;
  const teacherPassword = `Live-pass-${stamp}`;

  await signIn(owner, ADMIN, /\/admin$/);

  await owner.goto("/admin/users");
  await owner.getByRole("button", { name: "Thêm người dùng", exact: true }).click();
  const add = owner.getByRole("dialog", { name: "Thêm người dùng" });
  await add.getByLabel("Họ và tên", { exact: true }).fill(teacherName);
  await add.getByLabel("Email", { exact: true }).fill(teacherEmail);
  await add
    .getByRole("group", { name: "Vai trò", exact: true })
    .getByRole("button", { name: "Giáo viên", exact: true })
    .click();
  await expect(
    add.getByRole("radio", { name: /^Gửi liên kết đặt mật khẩu qua email/ }),
  ).toBeChecked();
  await add.getByRole("button", { name: "Tạo tài khoản", exact: true }).click();
  await expect(
    owner.getByText(`Đã tạo tài khoản và gửi liên kết tới ${teacherEmail}`),
  ).toBeVisible();
  await expect(owner).toHaveURL(/[?&]user=[0-9a-f-]+/);
  const teacherId = new URL(owner.url()).searchParams.get("user");

  const token = await setPasswordToken(teacherEmail);
  const context = await browser.newContext();
  try {
    const teacher = await context.newPage();
    await teacher.goto(`/set-password#token=${token}`);
    await expect(
      teacher.getByRole("heading", { name: `Đặt mật khẩu cho ${teacherEmail}` }),
    ).toBeVisible();
    await teacher.getByLabel("Mật khẩu mới", { exact: true }).fill(teacherPassword);
    await teacher
      .getByLabel("Nhập lại mật khẩu", { exact: true })
      .fill(teacherPassword);
    await teacher
      .getByRole("button", { name: "Đặt mật khẩu và đăng nhập", exact: true })
      .click();
    await expect(teacher).toHaveURL(/\/teacher$/);
    await expect(
      teacher.getByText("Chưa có bài giao nào.", { exact: true }),
    ).toBeVisible();
    await expect(
      teacher.getByText("Chưa có hoạt động nào.", { exact: true }),
    ).toBeVisible();

    const nav = teacher.getByRole("navigation", { name: "Điều hướng chính" });
    await nav.getByRole("link", { name: "Đề thi", exact: true }).click();
    await expect(
      teacher.getByText("Chưa có đề thi nào.", { exact: true }),
    ).toBeVisible();
    await nav.getByRole("link", { name: "Thư viện media", exact: true }).click();
    await expect(teacher.getByText("Chưa có media", { exact: true })).toBeVisible();

    try {
      await grantTeacherMediaWrite(owner, false);
      expect(await uploadAudio(teacher), "an upload without the right").toBe(403);
      await expect(
        teacher
          .getByRole("dialog", { name: "Tải lên media" })
          .getByText("Bạn không có quyền truy cập chức năng này."),
      ).toBeVisible();
    } finally {
      await grantTeacherMediaWrite(owner, true);
    }
    await teacher.keyboard.press("Escape");
    await expect(teacher.getByRole("dialog", { name: "Tải lên media" })).toBeHidden();
    expect(await uploadAudio(teacher), "an upload once the right is back").toBe(201);

    await owner.goto(`/admin/users?user=${teacherId}`);
    const sheet = owner.getByRole("dialog", { name: teacherName });
    await sheet.getByRole("button", { name: /^Vô hiệu hoá tài khoản/ }).click();
    await owner
      .getByRole("dialog", { name: `Vô hiệu hoá ${teacherName}?` })
      .getByRole("button", { name: "Vô hiệu hoá", exact: true })
      .click();
    await expect(
      owner.getByText("Đã vô hiệu hoá tài khoản", { exact: true }),
    ).toBeVisible();

    const signInAgain = teacher.getByRole("dialog", { name: "Vui lòng đăng nhập lại" });
    const navigated = nav
      .getByRole("link", { name: "Lớp học", exact: true })
      .click()
      .catch(() => undefined);
    await expect(signInAgain).toBeVisible();
    await expect(signInAgain.getByRole("button", { name: "Đăng nhập" })).toBeFocused();
    await navigated;
  } finally {
    await context.close();
  }
});
