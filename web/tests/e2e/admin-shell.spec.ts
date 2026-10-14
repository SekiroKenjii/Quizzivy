import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";

const auditor = {
  ...adminUser,
  permissions: ["system.audit.read"],
  workspaces: ["admin"],
};

const credits = {
  requests: [{ id: "a" }, { id: "b" }],
};

async function openConsole(page: Page, user: typeof adminUser, path: string) {
  await stubApi(page, {
    ...sessionAs(user),
    "GET /public/status": { body: { maintenance: null, ai: true } },
    "GET /admin/ai/credits": { body: credits },
    "PATCH /me/preferences": { body: {} },
  });
  await page.goto(path);
}

const nav = (page: Page) => page.getByRole("navigation", { name: "Điều hướng chính" });

for (const width of [1440, 1280, 1024, 768]) {
  test(`the Admin console draws its sidebar, top bar and page at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await openConsole(page, adminUser, "/admin");

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(nav(page).getByRole("link")).toHaveCount(8);
    await expect(nav(page).getByRole("link", { name: "Tổng quan" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    await expect(
      nav(page).getByRole("link", { name: /^Tín dụng AI\s*2 yêu cầu/ }),
    ).toBeVisible();
    await expect(page.getByRole("link", { name: "Chế độ giáo viên" })).toBeVisible();
    expect(
      await page.evaluate(
        () =>
          document.documentElement.scrollWidth <= document.documentElement.clientWidth,
      ),
    ).toBe(true);
  });
}

test("the Admin console is a drawer at 360px and the page does not scroll sideways", async ({
  page,
}) => {
  await page.setViewportSize({ width: 360, height: 780 });
  await openConsole(page, adminUser, "/admin/users");

  await expect(nav(page)).toHaveCount(0);
  const view = page.getByRole("link", { name: "Chế độ giáo viên" });
  await expect(view).toBeVisible();
  expect((await view.boundingBox())?.width).toBeLessThan(40);

  await page.getByRole("button", { name: "Mở menu" }).click();
  const drawer = page.getByRole("dialog", { name: "Điều hướng chính" });
  await expect(drawer).toBeVisible();
  expect((await drawer.boundingBox())?.width).toBe(272);
  await drawer.getByRole("link", { name: "Nhật ký kiểm tra" }).click();
  await expect(page).toHaveURL(/\/admin\/audit$/);
  await expect(drawer).toBeHidden();
  expect(
    await page.evaluate(
      () =>
        document.documentElement.scrollWidth <= document.documentElement.clientWidth,
    ),
  ).toBe(true);
});

test("a custom role holding only the audit log gets the 403 at the users page", async ({
  page,
}) => {
  await openConsole(page, auditor, "/admin/users");

  await expect(
    page.getByRole("heading", { level: 1, name: "Bạn không có quyền mở trang này" }),
  ).toBeVisible();
  await page.goto("/admin/audit");
  await expect(nav(page).getByRole("link")).toHaveCount(2);
  await expect(page.getByRole("heading", { level: 1 })).not.toHaveText(
    "Bạn không có quyền mở trang này",
  );
});

test("an old teacher address under /admin goes to the same path under /teacher", async ({
  page,
}) => {
  await openConsole(page, adminUser, "/admin/settings/profile");

  await expect(page).toHaveURL(/\/teacher\/settings\/profile$/);
});

test("the switcher carries an Admin to the teacher console and back", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await openConsole(page, adminUser, "/admin");

  await page.getByRole("link", { name: "Chế độ giáo viên" }).click();
  await expect(page).toHaveURL(/\/teacher$/);
  await page.getByRole("button", { name: /Quizzivy/ }).click();
  await page.getByRole("menuitem", { name: "Bảng quản trị" }).click();
  await expect(page).toHaveURL(/\/admin$/);
});
