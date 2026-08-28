import { expect, test } from "@playwright/test";
import { anonymous, stubApi, stubGoogleConsent, studentUser } from "./support/api";

/**
 * §14's E2E 3 and E2E 4 — the self-join flow, which is the only way a student
 * account comes into existence (§6.3) and the first Quizzivy screen anyone new
 * ever sees.
 *
 * Google itself is replaced; everything else is the real thing. The browser
 * really does navigate to the authorization endpoint, the redirect really does
 * come back to our callback, and the `state` is echoed from the request rather
 * than invented — so a broken state check fails here rather than passing.
 */

const CODE = "K7M3P9QR";
const CLASS_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const CLASS_NAME = "Tiếng Anh giao tiếp — Lớp A";

test("E2E 3: an anonymous visitor joins a class from a deep link", async ({ page }) => {
  const calls: string[] = [];
  page.on("request", (r) => {
    if (r.url().includes("localhost:8080"))
      calls.push(`${r.method()} ${new URL(r.url()).pathname}`);
  });

  await stubApi(page, {
    ...anonymous,
    "POST /join/preview": {
      body: { classId: CLASS_ID, className: CLASS_NAME, teacherName: "Thuong" },
    },
    "POST /auth/google": {
      body: {
        accessToken: "e2e-access-token",
        expiresIn: 900,
        user: studentUser,
        enrolledClass: {
          id: CLASS_ID,
          name: CLASS_NAME,
          studentCount: 13,
          selfJoinEnabled: true,
          createdAt: "2026-01-01T00:00:00Z",
        },
      },
    },
  });
  await stubGoogleConsent(page);

  // The deep link a QR code or a message produces. The code is shown, not
  // acted on: a student who scanned the wrong poster can see that.
  await page.goto(`/join/${CODE}`);
  await expect(page.getByLabel("Mã lớp")).toHaveValue("K7M3-P9QR");
  await page.getByRole("button", { name: "Tiếp tục" }).click();

  // §6.2's confirm step: which class, before anything authenticates.
  await expect(page).toHaveURL(new RegExp(`/join/${CODE}/confirm$`));
  await expect(page.getByRole("heading", { name: CLASS_NAME })).toBeVisible();
  await expect(page.getByText(/Thuong/)).toBeVisible();
  expect(calls).not.toContain("POST /auth/google");

  // The tap IS the consent. Everything after it is the §5.3 round trip.
  await page.getByRole("button", { name: "Tiếp tục với Google" }).click();

  await expect(page).toHaveURL(/\/app$/);
  await expect(page.getByRole("heading", { name: "Bài của tôi" })).toBeVisible();

  const exchange = calls.filter((c) => c === "POST /auth/google");
  expect(exchange, "the authorization code is exchanged exactly once").toHaveLength(1);
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

  await page.goto(`/join/${CODE}/confirm`);

  // The server's own message, rendered verbatim. §6.5: the four refusals carry
  // different codes and identical information.
  await expect(page.getByRole("alert")).toHaveText(/đã hết hạn/);

  // Nothing about the class survives a refusal -- not the name, not the
  // teacher, not the id.
  const body = (await page.textContent("body")) ?? "";
  expect(body).not.toContain(CLASS_NAME);
  expect(body).not.toContain("Thuong");
  expect(body).not.toContain(CLASS_ID);

  // And there is no way to proceed: nothing to consent to means no button to
  // create an account with.
  await expect(page.getByRole("button", { name: /Google/ })).toHaveCount(0);
  expect(calls).not.toContain("POST /auth/google");

  // A way out that is not a dead end.
  await page.getByRole("link", { name: "Thử mã khác" }).click();
  await expect(page).toHaveURL(/\/join$/);
});

test("a signed-out visitor never reaches the app by guessing the URL", async ({
  page,
}) => {
  await stubApi(page, anonymous);
  await page.goto("/app");
  await expect(page).toHaveURL(/\/login\?next=/);
});
