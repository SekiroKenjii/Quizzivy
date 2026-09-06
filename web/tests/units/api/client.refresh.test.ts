import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  api,
  setSessionLostHandler,
  __resetRefreshStateForTests,
} from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

/**
 * These cover R-06, which is the failure mode this client exists to prevent:
 * "the app signs me out every time I refresh the page".
 */

type Handler = (url: string, init: RequestInit) => Response | Promise<Response>;

let calls: { url: string; method: string }[] = [];
let handler: Handler;

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function envelope(code: string, status: number, message = "boom") {
  return json({ error: { code, message, requestId: "req-1" } }, status);
}

beforeEach(() => {
  calls = [];
  __resetRefreshStateForTests();
  useAuthStore.getState().clearSession();
  useAuthStore.getState().setAccessToken("initial-token");
  setSessionLostHandler(() => {});
  vi.stubGlobal("fetch", (input: string | URL, init: RequestInit = {}) => {
    const url = String(input);
    calls.push({ url, method: init.method ?? "GET" });
    return handler(url, init);
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const refreshCalls = () => calls.filter((c) => c.url.endsWith("/auth/refresh"));

describe("single-flight refresh (R-06)", () => {
  it("issues exactly one POST /auth/refresh for five concurrent 401s, and all five succeed", async () => {
    let refreshed = false;
    handler = (url) => {
      if (url.endsWith("/auth/refresh")) {
        refreshed = true;
        return json({ accessToken: "fresh-token", expiresIn: 900 });
      }
      // Every protected call 401s until the refresh lands.
      return refreshed
        ? json({
            id: "u1",
            email: "a@b.c",
            fullName: "A",
            role: "student",
            hasPassword: true,
            linkedProviders: [],
            mustChangePassword: false,
            createdAt: "2026-01-01T00:00:00Z",
          })
        : envelope("REFRESH_TOKEN_INVALID", 401);
    };

    const results = await Promise.all(
      Array.from({ length: 5 }, () => api("get", "/auth/me")),
    );

    expect(refreshCalls()).toHaveLength(1);
    expect(results).toHaveLength(5);
    for (const r of results) expect(r.id).toBe("u1");
    expect(useAuthStore.getState().accessToken).toBe("fresh-token");
  });

  it("retries each request with the NEW token, not the stale one", async () => {
    const authHeaders: (string | null)[] = [];
    let refreshed = false;
    handler = (url, init) => {
      if (url.endsWith("/auth/refresh")) {
        refreshed = true;
        return json({ accessToken: "fresh-token", expiresIn: 900 });
      }
      const headers = new Headers(init.headers);
      authHeaders.push(headers.get("Authorization"));
      return refreshed
        ? json({ items: [], page: 1, pageSize: 50, total: 0 })
        : envelope("X", 401);
    };

    await Promise.all([api("get", "/admin/classes"), api("get", "/admin/classes")]);

    expect(refreshCalls()).toHaveLength(1);
    // First attempts used the stale token; retries must use the fresh one.
    expect(authHeaders.slice(0, 2)).toEqual([
      "Bearer initial-token",
      "Bearer initial-token",
    ]);
    expect(authHeaders.slice(2)).toEqual(["Bearer fresh-token", "Bearer fresh-token"]);
  });

  it("allows a later, separate refresh once the first has settled", async () => {
    let refreshed = false;
    handler = (url) => {
      if (url.endsWith("/auth/refresh")) {
        refreshed = true;
        return json({ accessToken: "fresh-token", expiresIn: 900 });
      }
      return refreshed
        ? json({ items: [], page: 1, pageSize: 50, total: 0 })
        : envelope("X", 401);
    };
    await api("get", "/admin/classes");
    expect(refreshCalls()).toHaveLength(1);

    // A new 401 much later is a genuinely new situation, not a stampede.
    refreshed = false;
    await api("get", "/admin/classes");
    expect(refreshCalls()).toHaveLength(2);
  });
});

describe("session loss (§5.2)", () => {
  it("clears the store and reports session loss when refresh fails", async () => {
    const lost = vi.fn();
    setSessionLostHandler(lost);
    handler = () => envelope("REFRESH_TOKEN_INVALID", 401);

    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);
    expect(lost).toHaveBeenCalledOnce();
    expect(useAuthStore.getState().accessToken).toBeNull();
    expect(useAuthStore.getState().user).toBeNull();
  });

  it("logs out on a second 401 after a successful refresh", async () => {
    const lost = vi.fn();
    setSessionLostHandler(lost);
    handler = (url) =>
      url.endsWith("/auth/refresh")
        ? json({ accessToken: "fresh-token", expiresIn: 900 })
        : envelope("REFRESH_TOKEN_INVALID", 401); // still 401 even with the new token

    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);
    // Exactly one refresh, and exactly two attempts at the protected call.
    expect(refreshCalls()).toHaveLength(1);
    expect(calls.filter((c) => c.url.endsWith("/auth/me"))).toHaveLength(2);
    expect(lost).toHaveBeenCalledOnce();
    expect(useAuthStore.getState().accessToken).toBeNull();
  });

  it("does not try to refresh when the login endpoint itself 401s", async () => {
    handler = () => envelope("INVALID_CREDENTIALS", 401);
    await expect(
      api("post", "/auth/login", { body: { email: "a@b.c", password: "wrongpass" } }),
    ).rejects.toMatchObject({ code: "INVALID_CREDENTIALS" });
    // A refresh here would be a loop, and would trip reuse detection.
    expect(refreshCalls()).toHaveLength(0);
  });
});

describe("error envelope (00-overview.md §7)", () => {
  it("decodes code, message and requestId", async () => {
    handler = () => envelope("JOIN_CODE_EXPIRED", 404, "Mã lớp đã hết hạn.");
    const err = await api("post", "/join/preview", {
      body: { joinCode: "ABCD1234" },
    }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    const apiErr = err as ApiError;
    expect(apiErr.code).toBe("JOIN_CODE_EXPIRED");
    expect(apiErr.message).toBe("Mã lớp đã hết hạn.");
    expect(apiErr.requestId).toBe("req-1");
    expect(apiErr.status).toBe(404);
  });

  it("survives a non-JSON error body", async () => {
    handler = () => new Response("<html>502</html>", { status: 502 });
    const err = (await api("get", "/auth/me").catch((e: unknown) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("UNKNOWN");
    expect(err.status).toBe(502);
  });

  it("exposes Retry-After on a 429", async () => {
    handler = () =>
      new Response(
        JSON.stringify({
          error: { code: "RATE_LIMITED", message: "slow down", requestId: "r" },
        }),
        {
          status: 429,
          headers: { "Content-Type": "application/json", "Retry-After": "42" },
        },
      );
    const err = (await api("post", "/join/preview", {
      body: { joinCode: "ABCD1234" },
    }).catch((e: unknown) => e)) as ApiError;
    expect(err.isRateLimited).toBe(true);
    expect(err.retryAfterSeconds).toBe(42);
  });
});

describe("§5.2: the access token never touches web storage", () => {
  it("is absent from localStorage and sessionStorage after a refresh", async () => {
    handler = (url) =>
      url.endsWith("/auth/refresh")
        ? json({ accessToken: "super-secret-token", expiresIn: 900 })
        : json({ items: [], page: 1, pageSize: 50, total: 0 });

    useAuthStore.getState().setAccessToken("initial-token");
    await api("get", "/admin/classes");

    const haystack = JSON.stringify({
      local: { ...localStorage },
      session: { ...sessionStorage },
    });
    expect(haystack).not.toContain("super-secret-token");
    expect(haystack).not.toContain("initial-token");
  });
});
