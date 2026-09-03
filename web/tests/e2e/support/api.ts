import type { Page, Route } from "@playwright/test";

/**
 * The E2E suite runs a production build against `vite preview` with no backend.
 * Everything the app asks the API for is stubbed here.
 */

export const API = "http://localhost:8080";

export interface StubbedResponse {
  status?: number;
  body?: unknown;
}

type Stubs = Record<string, StubbedResponse | ((route: Route) => Promise<void> | void)>;

/**
 * Answers API calls by `METHOD /path`. Anything not listed gets a 404 with the
 * error envelope, so an unexpected call fails loudly rather than hanging until
 * the test times out.
 */
export async function stubApi(page: Page, stubs: Stubs) {
  await page.route(`${API}/**`, async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const key = `${request.method()} ${path}`;

    const stub = stubs[key];
    if (typeof stub === "function") {
      await stub(route);
      return;
    }
    if (!stub) {
      await route.fulfill({
        status: 404,
        contentType: "application/json",
        body: JSON.stringify({
          error: { code: "NOT_FOUND", message: `no stub for ${key}`, requestId: "e2e" },
        }),
      });
      return;
    }
    await route.fulfill({
      status: stub.status ?? 200,
      contentType: "application/json",
      body: JSON.stringify(stub.body ?? {}),
    });
  });
}

export const adminUser = {
  id: "019535d9-3df7-79fb-b466-fa907fa17f9f",
  email: "thuong@example.com",
  fullName: "Thuong",
  role: "admin" as const,
  hasPassword: true,
  linkedProviders: [] as string[],
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
};

export const studentUser = {
  ...adminUser,
  id: "019535d9-3df7-79fb-b466-fa907fa17f9e",
  email: "hocvien@example.com",
  fullName: "Nguyễn Văn An",
  role: "student" as const,
};

/**
 * Establishes a session by answering the bootstrap call.
 *
 * The access token stays in memory and is never needed here: the guards read
 * `user`, and every subsequent call is stubbed anyway.
 */
export function sessionAs(user: typeof adminUser | typeof studentUser): Stubs {
  return { "GET /auth/me": { body: user } };
}

/** No session: what an anonymous visitor's bootstrap gets. */
export const anonymous: Stubs = {
  "GET /auth/me": {
    status: 401,
    body: {
      error: {
        code: "UNAUTHORIZED",
        message: "Phiên đăng nhập không hợp lệ.",
        requestId: "e2e",
      },
    },
  },
  "POST /auth/refresh": {
    status: 401,
    body: {
      error: {
        code: "REFRESH_TOKEN_INVALID",
        message: "Phiên đã hết hạn.",
        requestId: "e2e",
      },
    },
  },
};

/** Stands in for Google's authorization endpoint. */
export async function stubGoogleConsent(page: Page, code = "fake-authorization-code") {
  await page.route("https://accounts.google.com/**", async (route) => {
    const url = new URL(route.request().url());
    const redirectUri = url.searchParams.get("redirect_uri") ?? "";
    const state = url.searchParams.get("state") ?? "";
    await route.fulfill({
      status: 302,
      headers: {
        location: `${redirectUri}?code=${encodeURIComponent(code)}&state=${encodeURIComponent(state)}`,
      },
      body: "",
    });
  });
}
