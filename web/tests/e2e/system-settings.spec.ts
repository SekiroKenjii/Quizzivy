import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, stubApi } from "./support/api";

const HASH = "a".repeat(64);

const settings = {
  organization: {
    name: "Quizzivy English Centre",
    shortName: "Quizzivy",
    website: "quizzivy.com",
    contactEmail: "hello@quizzivy.com",
    frontDeskPhone: "028 7300 1234",
    frontDeskHours: "8:00 – 20:00 hằng ngày",
    address: "12 Nguyễn Huệ, Quận 1, TP.HCM",
    logo: { light: null, dark: null },
  },
  signIn: {
    google: true,
    password: true,
    forcePasswordChange: true,
    joinCodes: true,
    allowedGoogleDomains: ["quizzivy.com", "gmail.com"],
  },
  retention: {
    keepAnswersMonths: 24,
    keepFocusEventsMonths: 13,
    anonymiseDisabledAfterMonths: null,
  },
  limits: {
    seats: 450,
    teachers: null,
    storageBytes: null,
    mediaPerTeacherBytes: null,
    importBytes: null,
    importPerTeacherBytes: null,
    emailsPerDay: null,
    emailsPerMonth: null,
    databaseBytes: null,
    warnAtPercent: null,
  },
  limitDefaults: {
    seats: { value: null, source: "none" },
    teachers: { value: null, source: "none" },
    storageBytes: { value: 10 * 1024 ** 3, source: "r2_free" },
    mediaPerTeacherBytes: { value: 5 * 1024 ** 3, source: "app" },
    importBytes: { value: 1024 ** 3, source: "app" },
    importPerTeacherBytes: { value: 256 * 1024 ** 2, source: "app" },
    emailsPerDay: { value: 100, source: "resend_free" },
    emailsPerMonth: { value: 3000, source: "resend_free" },
    databaseBytes: { value: 10 * 1024 ** 3, source: "neon_launch" },
    warnAtPercent: { value: 80, source: "app" },
  },
  revision: 7,
  updatedAt: "2026-10-14T09:00:00Z",
  updatedBy: adminUser.id,
};

const usage = {
  seats: { used: 412, limit: 450 },
  teachers: { used: 18, limit: null },
  storage: { bytes: 8.6 * 1024 ** 3, limit: 10 * 1024 ** 3 },
  media: {
    bytes: 7.9 * 1024 ** 3,
    largestLibrary: 1.2 * 1024 ** 3,
    perTeacherLimit: 5 * 1024 ** 3,
  },
  imports: {
    bytes: 700 * 1024 ** 2,
    count: 14,
    openBytes: 200 * 1024 ** 2,
    limit: 1024 ** 3,
    perTeacherLimit: 256 * 1024 ** 2,
  },
  email: { today: 12, month: 340, held: 0, dailyLimit: 100, monthlyLimit: 3000 },
  database: { bytes: 600 * 1024 ** 2, limit: 10 * 1024 ** 3 },
  attemptsKept: { count: 9120 },
  aiCredits: { used: 6200, pool: 20000 },
  warnAtPercent: 80,
};

const publicOrganization = {
  name: "Quizzivy English Centre",
  shortName: null,
  website: null,
  contactEmail: null,
  frontDeskPhone: null,
  frontDeskHours: null,
  address: null,
  logo: { lightUrl: null, darkUrl: null },
  signIn: { google: true, password: true },
  joinCodes: true,
};

async function openSettings(page: Page, path = "/admin/settings", theme?: "dark") {
  if (theme)
    await page.addInitScript(() => localStorage.setItem("quizzivy.theme", "dark"));
  const puts: unknown[] = [];
  await stubApi(page, {
    ...sessionAs(adminUser),
    "GET /public/status": { body: { maintenance: null, ai: true } },
    "GET /public/organization": { body: publicOrganization },
    "GET /admin/settings": { body: settings },
    "GET /admin/usage": { body: usage },
    "PUT /admin/settings": async (route) => {
      const body = route.request().postDataJSON() as { expectedRevision: number };
      puts.push(body);
      const { expectedRevision, ...rest } = body;
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          ...rest,
          limitDefaults: settings.limitDefaults,
          revision: expectedRevision + 1,
          updatedAt: "2026-10-15T09:00:00Z",
          updatedBy: adminUser.id,
        }),
      });
    },
    "POST /admin/settings/sign-in-impact": {
      body: { affectedUsers: 3, affectedAdmins: 0, actorLocked: false, lockout: false },
    },
    "POST /admin/settings/logo": {
      body: { sha256: HASH, type: "image/png", width: 512, height: 512 },
    },
    "PATCH /me/preferences": { body: {} },
  });
  await page.goto(path);
  await expect(
    page.getByRole("heading", { level: 1, name: "Cài đặt hệ thống" }),
  ).toBeVisible();
  return puts;
}

const noSidewaysScroll = (page: Page) =>
  page.evaluate(
    () => document.documentElement.scrollWidth <= document.documentElement.clientWidth,
  );

for (const [width, height] of [
  [1280, 900],
  [768, 900],
  [360, 780],
] as const) {
  for (const theme of [undefined, "dark"] as const) {
    test(`System settings draws Organization and Sign-in at ${width}px${theme ? ", dark" : ""} without scrolling sideways`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height });
      await openSettings(page, "/admin/settings", theme);

      await expect(page.getByRole("textbox", { name: "Tên tổ chức" })).toHaveValue(
        "Quizzivy English Centre",
      );
      await expect(page.getByRole("button", { name: "Thay logo" })).toBeVisible();
      expect(await noSidewaysScroll(page)).toBe(true);

      await page
        .getByRole("navigation", { name: "Mục cài đặt hệ thống" })
        .getByRole("link", { name: "Đăng nhập" })
        .click();
      await expect(page).toHaveURL(/\/admin\/settings\/sign-in$/);
      await expect(
        page.getByRole("switch", { name: "Đăng nhập bằng Google" }),
      ).toBeVisible();
      await expect(
        page.getByRole("textbox", { name: "Tên miền Google được phép" }),
      ).toBeVisible();
      expect(await noSidewaysScroll(page)).toBe(true);
    });
  }
}

test("the section column sticks as the page scrolls from 768px of content, and is a row on a phone", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 500 });
  await openSettings(page);
  const column = page.getByRole("navigation", { name: "Mục cài đặt hệ thống" });
  await expect(column).toBeVisible();
  await expect(column).toHaveCSS("position", "sticky");
  await expect(column).toHaveCSS("flex-direction", "column");

  await page.setViewportSize({ width: 360, height: 780 });
  await expect(column).toHaveCSS("position", "static");
  await expect(column).toHaveCSS("flex-direction", "row");
});

test("one unsaved-changes bar follows an edit across sections, and a save sends the whole document", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const puts = await openSettings(page);
  const name = page.getByRole("textbox", { name: "Tên tổ chức" });
  await name.fill("Trung tâm Quizzivy");
  await expect(page.getByText("Bạn có thay đổi chưa lưu.")).toHaveCount(1);

  await page
    .getByRole("navigation", { name: "Mục cài đặt hệ thống" })
    .getByRole("link", { name: "Đăng nhập" })
    .click();
  await page.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }).click();
  await expect(page.getByText("3 người sẽ không đăng nhập được nữa.")).toHaveCount(0);
  await page.getByRole("button", { name: "Bỏ tên miền gmail.com" }).click();
  await expect(page.getByText("3 người sẽ không đăng nhập được nữa.")).toBeVisible();
  await expect(page.getByText("Bạn có thay đổi chưa lưu.")).toHaveCount(1);

  await page.getByRole("button", { name: "Lưu thay đổi" }).click();
  await expect(page.getByText("Đã lưu cài đặt")).toBeVisible();
  await expect(page.getByText("Bạn có thay đổi chưa lưu.")).toHaveCount(0);
  expect(puts).toHaveLength(1);
  expect(puts[0]).toMatchObject({
    expectedRevision: 7,
    organization: { name: "Trung tâm Quizzivy" },
    signIn: { joinCodes: false, allowedGoogleDomains: ["quizzivy.com"] },
    retention: settings.retention,
  });
});

test("a keyboard user can reach, toggle and read the Sign-in switches", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await openSettings(page, "/admin/settings/sign-in");
  const google = page.getByRole("switch", { name: "Đăng nhập bằng Google" });

  await google.focus();
  await expect(google).toBeFocused();
  await page.keyboard.press("Space");
  await expect(google).not.toBeChecked();
  await page.keyboard.press("Space");
  await expect(google).toBeChecked();
});

for (const [width, height] of [
  [1440, 900],
  [1280, 900],
  [1024, 900],
  [768, 900],
  [360, 780],
] as const) {
  for (const theme of [undefined, "dark"] as const) {
    test(`Plan & usage draws its limits at ${width}px${theme ? ", dark" : ""} without scrolling sideways`, async ({
      page,
    }, info) => {
      await page.setViewportSize({ width, height });
      await openSettings(page, "/admin/settings/plan", theme);

      const seats = page.getByRole("textbox", { name: "Tài khoản", exact: true });
      await expect(seats).toHaveValue("450");
      await expect(page.getByText("Đang dùng 412")).toBeVisible();
      await expect(
        page.getByRole("textbox", { name: "Tổng dung lượng tệp" }),
      ).toHaveAttribute("placeholder", "10");
      await expect(
        page.getByRole("link", { name: "Quản lý tín dụng AI" }),
      ).toBeVisible();
      expect(await noSidewaysScroll(page)).toBe(true);
      await page.screenshot({
        path: info.outputPath(`plan-usage-${width}${theme ? "-dark" : ""}.png`),
        fullPage: true,
      });
      await page
        .getByRole("link", { name: "Quản lý tín dụng AI" })
        .scrollIntoViewIfNeeded();
      await page.screenshot({
        path: info.outputPath(`plan-usage-${width}${theme ? "-dark" : ""}-end.png`),
      });
    });
  }
}

test("Plan & usage saves a typed size in bytes and a cleared limit as null", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const puts = await openSettings(page, "/admin/settings/plan");

  await page.getByRole("textbox", { name: "Tổng dung lượng tệp" }).fill("20");
  await page.getByRole("button", { name: "Dùng mặc định cho Tài khoản" }).click();
  await page.getByRole("button", { name: "Lưu thay đổi" }).click();

  await expect.poll(() => puts.length).toBe(1);
  expect(puts[0]).toMatchObject({
    limits: { seats: null, storageBytes: 20 * 1024 ** 3 },
  });
});
