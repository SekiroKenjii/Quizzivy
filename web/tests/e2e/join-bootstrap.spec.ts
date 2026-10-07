import { expect, test, type Request } from "@playwright/test";
import { anonymous, API, stubApi } from "./support/api";

const CODE = "K7M3P9QR";
const CLASS_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const CLASS_NAME = "Tiếng Anh giao tiếp — Lớp A";
const TEACHER = "Hoàng Thương";

function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { open, opened };
}

test("an anonymous class preview survives bootstrap refusal before its response", async ({
  page,
}, testInfo) => {
  const meRelease = gate();
  const previewRelease = gate();
  const calls: string[] = [];
  const events: string[] = [];
  const pageErrors: string[] = [];
  const record = (request: Request) => {
    const url = new URL(request.url());
    if (url.origin === API) calls.push(`${request.method()} ${url.pathname}`);
  };
  const recordError = (error: Error) => pageErrors.push(error.message);
  page.on("request", record);
  page.on("pageerror", recordError);
  let primary: unknown;
  let failed = false;
  const cleanupErrors: string[] = [];

  try {
    await stubApi(page, {
      ...anonymous,
      "GET /auth/me": async (route) => {
        events.push("me-started");
        await meRelease.opened;
        await route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({
            error: {
              code: "UNAUTHORIZED",
              message: "Phiên đăng nhập không hợp lệ.",
              requestId: "e2e",
            },
          }),
        });
        events.push("me-returned");
      },
      "POST /join/preview": async (route) => {
        expect(route.request().postDataJSON()).toEqual({ joinCode: CODE });
        events.push("preview-started");
        await previewRelease.opened;
        events.push("preview-released");
        await route.fulfill({
          contentType: "application/json",
          body: JSON.stringify({
            classId: CLASS_ID,
            className: CLASS_NAME,
            teacherName: TEACHER,
          }),
        });
        events.push("preview-returned");
      },
    });
    await page.goto(`/join/${CODE}`);
    await expect
      .poll(() => events.includes("me-started") && events.includes("preview-started"))
      .toBe(true);
    const splash = page.locator('.qz-boot[role="status"]');
    await expect(splash).toBeVisible();
    await expect(page.getByText(CLASS_NAME, { exact: true })).toHaveCount(0);
    const refreshResponse = page.waitForResponse(
      (response) =>
        response.url() === `${API}/auth/refresh` &&
        response.request().method() === "POST",
    );
    meRelease.open();
    const refresh = await refreshResponse;
    expect(refresh.status()).toBe(401);
    expect(await refresh.finished()).toBeNull();
    events.push("refresh-finished");
    await expect(splash).toHaveCount(0);
    events.push("splash-removed");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(page.getByRole("alert")).toHaveCount(0);
    previewRelease.open();
    await expect(page.getByText(CLASS_NAME, { exact: true })).toBeVisible();
    await expect(page.getByText(TEACHER, { exact: true })).toBeVisible();
    events.push("class-visible");
    expect(calls.filter((call) => call === "POST /join/preview")).toHaveLength(1);
    expect(calls.filter((call) => call === "POST /auth/refresh")).toHaveLength(1);
    expect(calls).not.toContain("POST /auth/login");
    expect(calls).not.toContain("POST /auth/google");
    expect(calls).not.toContain("POST /app/classes/join");
  } catch (error) {
    failed = true;
    primary = error;
  }
  meRelease.open();
  previewRelease.open();
  try {
    await page.unrouteAll({ behavior: "wait" });
  } catch (error) {
    cleanupErrors.push(error instanceof Error ? error.message : String(error));
  }
  page.off("request", record);
  page.off("pageerror", recordError);
  try {
    await testInfo.attach("join-bootstrap-chronology", {
      body: Buffer.from(
        JSON.stringify({ calls, events, pageErrors, cleanupErrors }, null, 2),
      ),
      contentType: "application/json",
    });
  } catch (error) {
    cleanupErrors.push(error instanceof Error ? error.message : String(error));
  }
  if (failed) throw primary;
  expect(cleanupErrors).toEqual([]);
  expect(pageErrors).toEqual([]);
});
