import { expect, test } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./support/api";
import { classes, fits, gradedResult, student } from "./support/student";

for (const width of [320, 360, 768, 1024, 1440, 1920]) {
  test(`student discovery uses available space at ${width}px`, async ({
    page,
  }, info) => {
    await page.setViewportSize({ width, height: 900 });
    await student(page);
    await page.goto("/app");
    await expect(page.getByRole("button", { name: "Tiếp tục làm bài" })).toHaveCount(1);
    const rows = page.getByRole("main").getByRole("link");
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(0)).toContainText("Bài luyện tập 2");
    await expect(rows.nth(0)).toContainText("Đang làm");
    await expect(rows.nth(1)).toContainText("Bài luyện tập 3");
    await expect(rows.nth(1)).toContainText("Đang mở");
    await fits(page);
    const main = await page.getByRole("main").boundingBox();
    expect(main!.width).toBeGreaterThan(width * 0.95);
    await expect(page.getByRole("complementary")).toHaveCount(0);
    if (width < 1024) {
      for (const control of await page
        .getByRole("main")
        .locator("button, [data-slot='button']")
        .all()) {
        const box = await control.boundingBox();
        expect(box!.height).toBeGreaterThanOrEqual(44);
        expect(box!.width).toBeGreaterThanOrEqual(44);
      }
    }
    await page.screenshot({
      path: info.outputPath(`home-${width}.png`),
      fullPage: true,
    });
    await page.getByRole("link", { name: "Lớp", exact: true }).click();
    await expect(
      page.getByRole("heading", { level: 2, name: classes[0]!.name }),
    ).toBeVisible();
    await expect(page.getByText("Bài luyện tập 1 · đang làm")).toBeVisible();
    await expect(page.getByRole("main").getByRole("link")).toHaveCount(0);
    const join = await page.getByRole("button", { name: "Tham gia lớp" }).boundingBox();
    expect(join!.height).toBe(40);
    await fits(page);
  });
}

test("unsaved settings survive crossing the desktop breakpoint and password visibility is reversible", async ({
  page,
}, info) => {
  await student(page);
  await page.setViewportSize({ width: 1024, height: 900 });
  await page.goto("/app/settings");
  const name = page.getByRole("textbox", { name: "Họ và tên" });
  await name.fill("Tên đang chỉnh sửa");
  await page.setViewportSize({ width: 320, height: 900 });
  await expect(name).toHaveValue("Tên đang chỉnh sửa");
  await fits(page);
  const sections = page.getByRole("group", { name: "Mục cài đặt" });
  await sections.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(/settings\/sign-in$/);
  await page.getByRole("button", { name: "Đổi", exact: true }).click();
  const password = page.getByLabel("Mật khẩu mới", { exact: true });
  await password.fill("Test-only-password");
  await page.getByRole("button", { name: "Hiện mật khẩu" }).last().click();
  await expect(password).toHaveAttribute("type", "text");
  await page.getByRole("button", { name: "Ẩn mật khẩu" }).click();
  await expect(password).toHaveAttribute("type", "password");
  await page.setViewportSize({ width: 1440, height: 900 });
  await expect(password).toHaveValue("Test-only-password");
  await sections.getByRole("button", { name: "Hồ sơ" }).click();
  await expect(name).toHaveValue("Tên đang chỉnh sửa");
  await expect(page.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
  await sections.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(password).toHaveValue("Test-only-password");
  await page.screenshot({ path: info.outputPath("settings-1440.png"), fullPage: true });
});

test("result filters survive resizing, explain empty results and retain the full title", async ({
  page,
}) => {
  const result = gradedResult;
  await stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/attempts/result/result": { body: result },
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/app/attempts/result/result");
  const wrong = page.getByRole("button", { name: /^Sai/ });
  await wrong.click();
  await page.setViewportSize({ width: 800, height: 900 });
  await expect(page.getByRole("link", { name: "Quay lại" })).toHaveCount(0);
  await expect(
    page.getByRole("main").getByRole("link", { name: "Trang chủ" }),
  ).toHaveAttribute("href", "/app");
  await page.setViewportSize({ width: 320, height: 900 });
  await expect(wrong).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("banner").getByText("Kết quả", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(result.testTitle);
  await expect(page.getByText("Không có câu sai trong bài này.")).toBeVisible();
  await fits(page);
  await page.getByRole("button", { name: "Xem tất cả câu" }).click();
  await expect(page.getByText("Câu trả lời đã chấm")).toBeVisible();
});

test("English student controls fit a 320px phone", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("quizzivy.locale", "en"));
  await student(page, { ...studentUser, locale: "en" });
  await page.setViewportSize({ width: 320, height: 900 });
  await page.goto("/app");
  await expect(page.getByRole("button", { name: "Continue test" })).toHaveCount(1);
  await expect(page.getByText("In progress", { exact: true })).toBeVisible();
  await fits(page);
  await page.goto("/app/settings");
  await expect(
    page.getByRole("banner").getByText("Settings", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("group", { name: "Settings section" })).toBeVisible();
  await fits(page);
});

for (const width of [767, 768]) {
  test(`the shell is ${width < 768 ? "a tab bar" : "a top bar"} at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await student(page);
    await page.goto("/app");
    const destinations = page.getByRole("navigation", { name: "Điều hướng chính" });
    await expect(destinations).toHaveCount(1);
    const box = (await destinations.boundingBox())!;
    const header = (await page.getByRole("banner").first().boundingBox())!;
    expect(header.height).toBe(60);
    if (width < 768) {
      await expect(destinations.getByRole("link")).toHaveText([
        /Trang chủ$/,
        "Lớp",
        "Tôi",
      ]);
      expect(box.y + box.height).toBe(900);
      expect(box.height).toBe(65);
    } else {
      await expect(destinations.getByRole("link")).toHaveText([/^Trang chủ/, "Lớp"]);
      expect(box.y + box.height).toBeLessThan(60);
    }
    await fits(page);
  });
}

test("a toast clears the tab bar, and sits at the edge where there is none", async ({
  page,
}) => {
  const lift = () =>
    page.evaluate(() =>
      getComputedStyle(document.documentElement)
        .getPropertyValue("--toast-bottom")
        .trim(),
    );
  await page.setViewportSize({ width: 390, height: 844 });
  await student(page);
  await page.goto("/app");
  await expect(page.getByRole("link", { name: "Tôi", exact: true })).toBeVisible();
  expect(await lift()).toBe("84px");

  await page.getByRole("link", { name: "Tôi", exact: true }).click();
  await expect(page.getByRole("link", { name: "Quay lại" })).toBeVisible();
  expect(await lift()).toBe("");

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/app");
  await expect(page.getByRole("link", { name: "Trang chủ Quizzivy" })).toBeVisible();
  expect(await lift()).toBe("");
});

test("the student console follows the dark theme", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("quizzivy.theme", "dark"));
  await page.setViewportSize({ width: 1280, height: 900 });
  await student(page, { ...studentUser, preferences: { theme: "dark" } });
  await page.goto("/app");
  await expect(page.getByRole("link", { name: "Trang chủ Quizzivy" })).toBeVisible();
  await expect(page.locator("html")).toHaveClass(/dark/);
  const colours = await page.evaluate(() => {
    const root = getComputedStyle(document.documentElement);
    const header = getComputedStyle(document.querySelector("header")!);
    return {
      token: root.getPropertyValue("--bg").trim(),
      header: header.backgroundColor,
    };
  });
  expect(colours.token).not.toBe("");
  expect(colours.header).not.toBe("rgb(255, 255, 255)");
  await expect(
    page.getByRole("link", { name: "Trang chủ Quizzivy" }).locator("img"),
  ).toHaveAttribute("src", "/brand/quizzivy-mark-on-dark.svg");
});

test("a destination whose module has not shipped is not a page", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await student(page);
  for (const path of ["/app/learn", "/app/grades", "/app/messages"]) {
    await page.goto(path);
    await expect(
      page.getByRole("heading", { name: "Trang này không tồn tại" }),
    ).toBeVisible();
  }
});
