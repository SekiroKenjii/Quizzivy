import { expect, test } from "@playwright/test";
import {
  anonymous,
  sessionAs,
  stubApi,
  stubGoogleConsent,
  studentUser,
} from "./support/api";

/**
 * §14's E2E 3 and E2E 4 — the self-join flow, which is the only way a student
 * account comes into existence (§6.3) and the first Quizzivy screen anyone new
 * ever sees.
 */

const CODE = "K7M3P9QR";
const CLASS_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const CLASS_NAME = "Tiếng Anh giao tiếp — Lớp A";
const TEACHER = "Hoàng Thương";

test("E2E 3: an anonymous visitor joins a class from a deep link", async ({ page }) => {
  const calls: string[] = [];
  page.on("request", (r) => {
    if (r.url().includes("localhost:8080"))
      calls.push(`${r.method()} ${new URL(r.url()).pathname}`);
  });
  let exchanged: unknown = null;

  await stubApi(page, {
    ...anonymous,
    "POST /join/preview": {
      body: { classId: CLASS_ID, className: CLASS_NAME, teacherName: TEACHER },
    },
    "POST /auth/google": async (route) => {
      exchanged = route.request().postDataJSON();
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          accessToken: "e2e-access-token",
          expiresIn: 900,
          user: studentUser,
          enrolledClass: {
            id: CLASS_ID,
            name: CLASS_NAME,
            studentCount: 13,
            openAssignmentCount: 0,
            archivedAt: null,
            selfJoinEnabled: true,
            createdAt: "2026-01-01T00:00:00Z",
          },
        }),
      });
    },
    "GET /app/classes": {
      body: {
        items: [
          {
            id: CLASS_ID,
            name: CLASS_NAME,
            description: null,
            teacherName: TEACHER,
            joinedAt: "2026-09-26T01:00:00Z",
          },
        ],
      },
    },
    "GET /app/assignments": { body: { dueNow: [], upcoming: [], completed: [] } },
  });
  await stubGoogleConsent(page);
  await page.goto(`/join/${CODE}`);

  // §6.2: which class, before anything authenticates.
  await expect(page.getByLabel("Mã lớp")).toHaveValue("K7M3-P9QR");
  await expect(page.getByText(CLASS_NAME, { exact: true })).toBeVisible();
  await expect(page.getByText(TEACHER, { exact: true })).toBeVisible();
  expect(calls).not.toContain("POST /auth/google");

  await page.getByRole("button", { name: `Tham gia ${CLASS_NAME}` }).click();
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByText(`Đăng nhập để tham gia ${CLASS_NAME}.`)).toBeVisible();

  await page.getByRole("button", { name: "Tiếp tục với Google" }).click();

  await expect(
    page.getByRole("heading", { name: `Bạn đã vào lớp ${CLASS_NAME}` }),
  ).toBeVisible({ timeout: 20_000 });
  await expect(page).toHaveURL(new RegExp(`/join/${CODE}$`));
  await expect(
    page.getByText(`${TEACHER} sẽ thấy bạn trong danh sách lớp.`),
  ).toBeVisible();
  expect(exchanged).toMatchObject({ joinCode: CODE });
  const exchange = calls.filter((c) => c === "POST /auth/google");
  expect(exchange, "the authorization code is exchanged exactly once").toHaveLength(1);

  await page.getByRole("link", { name: "Đến lớp của tôi" }).click();
  await expect(page).toHaveURL(/\/app\/classes$/);
  await expect(page.getByText(CLASS_NAME).first()).toBeVisible();
});

test("E2E 4: an expired code says so plainly, creates nothing, and names no class", async ({
  page,
}) => {
  const calls: string[] = [];
  page.on("request", (r) => {
    if (r.url().includes("localhost:8080"))
      calls.push(`${r.method()} ${new URL(r.url()).pathname}`);
  });

  await stubApi(page, {
    ...anonymous,
    "POST /join/preview": {
      status: 404,
      body: {
        error: {
          code: "JOIN_CODE_EXPIRED",
          message: "Mã lớp này đã hết hạn. Vui lòng xin giáo viên mã mới.",
          requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
        },
      },
    },
  });

  // The confirm step's old address still lands on the page that replaced it.
  await page.goto(`/join/${CODE}/confirm`);
  await expect(page).toHaveURL(new RegExp(`/join/${CODE}$`));
  await expect(page.getByRole("alert")).toHaveText(
    "Không có lớp nào dùng mã này. Hãy kiểm tra lại với giáo viên.",
  );
  const body = (await page.textContent("body")) ?? "";
  expect(body).not.toContain(CLASS_NAME);
  expect(body).not.toContain(TEACHER);
  expect(body).not.toContain(CLASS_ID);
  await expect(page.getByRole("button", { name: /Tham gia|Google/ })).toHaveCount(0);
  expect(calls).not.toContain("POST /auth/google");
  expect(calls).not.toContain("POST /app/classes/join");

  // A way out that is not a dead end: the field is right there.
  await page.getByLabel("Mã lớp").fill("");
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("a signed-out visitor never reaches the app by guessing the URL", async ({
  page,
}) => {
  await stubApi(page, anonymous);
  await page.goto("/app");
  await expect(page).toHaveURL(/\/login\?next=/);
});

test("a signed-in student joins from Classes with one lookup and one join", async ({
  page,
}) => {
  const calls: string[] = [];
  page.on("request", (r) => {
    if (r.url().includes("localhost:8080"))
      calls.push(`${r.method()} ${new URL(r.url()).pathname}`);
  });
  let joined = false;
  const json = (body: unknown) => ({
    contentType: "application/json",
    body: JSON.stringify(body),
  });
  await stubApi(page, {
    ...sessionAs(studentUser),
    "GET /app/classes": (route) =>
      route.fulfill(
        json({
          items: joined
            ? [
                {
                  id: CLASS_ID,
                  name: CLASS_NAME,
                  description: null,
                  teacherName: TEACHER,
                  joinedAt: "2026-09-26T01:00:00Z",
                },
              ]
            : [],
        }),
      ),
    "GET /app/assignments": { body: { dueNow: [], upcoming: [], completed: [] } },
    "POST /join/preview": {
      body: { classId: CLASS_ID, className: CLASS_NAME, teacherName: TEACHER },
    },
    "POST /app/classes/join": (route) => {
      joined = true;
      return route.fulfill(
        json({
          id: CLASS_ID,
          name: CLASS_NAME,
          description: null,
          studentCount: 13,
          openAssignmentCount: 0,
          archivedAt: null,
          selfJoinEnabled: true,
          createdAt: "2026-01-01T00:00:00Z",
        }),
      );
    },
  });
  await page.goto("/app/classes");
  await expect(page.getByText("Bạn chưa tham gia lớp nào.")).toBeVisible();
  await page.getByRole("button", { name: "Tham gia lớp" }).click();
  const dialog = page.getByRole("dialog", { name: "Tham gia lớp" });
  await dialog.getByLabel("Mã lớp").pressSequentially("k7m3p9qr");
  await expect(dialog.getByLabel("Mã lớp")).toHaveValue("K7M3-P9QR");
  await expect(dialog.getByText(CLASS_NAME, { exact: true })).toBeVisible();
  await expect(dialog.getByText(/học viên/)).toHaveCount(0);
  await dialog.getByRole("button", { name: "Tham gia", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText(`Bạn đã vào lớp ${CLASS_NAME}`)).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: CLASS_NAME })).toBeVisible();
  await expect(page.getByRole("button", { name: "Tham gia lớp" })).toBeFocused();
  expect(calls.filter((c) => c === "POST /join/preview")).toHaveLength(1);
  expect(calls.filter((c) => c === "POST /app/classes/join")).toHaveLength(1);
});
