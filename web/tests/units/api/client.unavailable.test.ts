import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { http } from "msw";
import {
  api,
  setMaintenanceHandler,
  setSessionLostHandler,
  uploadFile,
  __resetRefreshStateForTests,
} from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import i18n from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";

const BASE = "http://localhost:8080";
const WINDOW = { startsAt: "2026-10-01T15:00:00Z", endsAt: "2026-10-01T16:00:00Z" };

type Handler = (url: string, init: RequestInit) => Response | Promise<Response>;

let calls: { url: string; headers: Headers }[] = [];
let handler: Handler;
let lost: Mock<() => void>;
let maintenance: Mock<(window: typeof WINDOW) => void>;

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function envelope(code: string, status: number, details?: Record<string, unknown>) {
  return json(
    { error: { code, message: "boom", requestId: "req-1", details } },
    status,
  );
}

const refreshCalls = () => calls.filter((c) => c.url.endsWith("/auth/refresh"));

beforeEach(() => {
  calls = [];
  __resetRefreshStateForTests();
  useAuthStore.getState().clearSession();
  useAuthStore.getState().setAccessToken("initial-token");
  lost = vi.fn<() => void>();
  maintenance = vi.fn<(window: typeof WINDOW) => void>();
  setSessionLostHandler(lost);
  setMaintenanceHandler(maintenance);
});

afterEach(async () => {
  vi.unstubAllGlobals();
  await i18n.changeLanguage("vi");
});

function stubFetch() {
  vi.stubGlobal("fetch", (input: string | URL, init: RequestInit = {}) => {
    const url = String(input);
    calls.push({ url, headers: new Headers(init.headers) });
    return handler(url, init);
  });
}

describe("a refresh that cannot be answered", () => {
  beforeEach(stubFetch);

  it.each([
    ["the network is down", () => Promise.reject(new TypeError("Failed to fetch")), 0],
    ["the server fails", () => envelope("INTERNAL", 500), 500],
    [
      "the API is down for maintenance",
      () => envelope("MAINTENANCE", 503, WINDOW),
      503,
    ],
  ])("keeps the session when %s", async (_, refresh, status) => {
    handler = (url) =>
      url.endsWith("/auth/refresh") ? refresh() : envelope("UNAUTHORIZED", 401);

    const failure = await api("get", "/auth/me").catch((e: unknown) => e);

    expect(failure).toBeInstanceOf(ApiError);
    expect((failure as ApiError).status).toBe(status);
    expect((failure as ApiError).isRetryable).toBe(true);
    expect(lost).not.toHaveBeenCalled();
    expect(useAuthStore.getState().accessToken).toBe("initial-token");
  });

  it("names the maintenance window before the error is thrown", async () => {
    handler = (url) =>
      url.endsWith("/auth/refresh")
        ? envelope("MAINTENANCE", 503, WINDOW)
        : envelope("UNAUTHORIZED", 401);
    let seenBeforeRejection = false;
    maintenance.mockImplementation(() => {
      seenBeforeRejection = true;
    });

    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);
    expect(seenBeforeRejection).toBe(true);
    expect(maintenance).toHaveBeenCalledWith(WINDOW);
  });

  it("still signs out when the refresh is refused", async () => {
    handler = (url) =>
      url.endsWith("/auth/refresh")
        ? envelope("FORBIDDEN", 403)
        : envelope("UNAUTHORIZED", 401);
    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);
    expect(refreshCalls()).toHaveLength(1);
    expect(lost).toHaveBeenCalledOnce();
    expect(useAuthStore.getState().accessToken).toBeNull();
  });

  it("makes one refresh for five concurrent 401s and rejects all five retryably", async () => {
    handler = (url) =>
      url.endsWith("/auth/refresh")
        ? Promise.reject(new TypeError("Failed to fetch"))
        : envelope("UNAUTHORIZED", 401);

    const results = await Promise.allSettled(
      Array.from({ length: 5 }, () => api("get", "/auth/me")),
    );

    expect(refreshCalls()).toHaveLength(1);
    for (const result of results) {
      expect(result.status).toBe("rejected");
      const reason = (result as PromiseRejectedResult).reason as ApiError;
      expect(reason).toBeInstanceOf(ApiError);
      expect(reason.isRetryable).toBe(true);
    }
    expect(lost).not.toHaveBeenCalled();
  });
});

describe("any request during maintenance", () => {
  beforeEach(stubFetch);

  it("raises the window from an ordinary request too", async () => {
    handler = () => envelope("MAINTENANCE", 503, WINDOW);
    await expect(api("get", "/app/classes")).rejects.toMatchObject({
      code: "MAINTENANCE",
      status: 503,
    });
    expect(maintenance).toHaveBeenCalledWith(WINDOW);
    expect(lost).not.toHaveBeenCalled();
  });

  it("stays quiet for any other 503", async () => {
    handler = () => envelope("INTERNAL", 503);
    await expect(api("get", "/app/classes")).rejects.toBeInstanceOf(ApiError);
    expect(maintenance).not.toHaveBeenCalled();
  });
});

describe("the language of server messages", () => {
  beforeEach(stubFetch);

  it("follows the app's language, not the browser's", async () => {
    handler = () => json({ items: [] });
    await api("get", "/app/classes");
    await i18n.changeLanguage("en");
    await api("get", "/app/classes");
    handler = (url) => {
      if (url.endsWith("/auth/refresh"))
        return json({ accessToken: "fresh-token", expiresIn: 900 });
      if (calls.length > 3) return json({ items: [] });
      return envelope("UNAUTHORIZED", 401);
    };
    await api("get", "/app/classes");

    expect(calls.map((c) => c.headers.get("Accept-Language"))).toEqual([
      "vi",
      "en",
      "en",
      "en",
      "en",
    ]);
  });
});

describe("uploads", () => {
  it("send the app's language and raise a maintenance window", async () => {
    let language: string | null = null;
    server.use(
      http.post(`${BASE}/admin/media`, ({ request }) => {
        language = request.headers.get("Accept-Language");
        return Response.json(
          {
            error: {
              code: "MAINTENANCE",
              message: "boom",
              requestId: "req-1",
              details: WINDOW,
            },
          },
          { status: 503 },
        );
      }),
    );

    await expect(
      uploadFile("/admin/media", new File(["x"], "clip.mp3", { type: "audio/mpeg" })),
    ).rejects.toMatchObject({ code: "MAINTENANCE" });
    expect(language).toBe("vi");
    expect(maintenance).toHaveBeenCalledWith(WINDOW);
  });
});
