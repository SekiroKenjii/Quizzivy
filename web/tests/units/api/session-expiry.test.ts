import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  api,
  setSessionLostHandler,
  __resetRefreshStateForTests,
} from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";
import { studentUser } from "@tests/support/fixtures";

let calls: string[] = [];

function refused(url: string) {
  calls.push(url);
  return new Response(
    JSON.stringify({
      error: { code: "REFRESH_TOKEN_REUSED", message: "", requestId: "r" },
    }),
    { status: 401, headers: { "Content-Type": "application/json" } },
  );
}

beforeEach(() => {
  calls = [];
  __resetRefreshStateForTests();
  useAuthStore.getState().setSession("initial-token", studentUser);
  vi.stubGlobal("fetch", (input: string | URL) =>
    Promise.resolve(refused(String(input))),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  setSessionLostHandler(() => {});
});

const refreshes = () => calls.filter((url) => url.endsWith("/auth/refresh"));

describe("a signed-in user whose session is refused", () => {
  it("keeps the user for the sign-in-again overlay but drops the token", async () => {
    const lost = vi.fn();
    setSessionLostHandler(lost);

    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);

    expect(lost).toHaveBeenCalledOnce();
    expect(useAuthStore.getState().user).toEqual(studentUser);
    expect(useAuthStore.getState().accessToken).toBeNull();
    expect(useAuthStore.getState().expired).toBe(true);
  });

  it("sends no more refreshes while the overlay is up", async () => {
    const lost = vi.fn();
    setSessionLostHandler(lost);
    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);
    expect(refreshes()).toHaveLength(1);

    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);
    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);

    expect(refreshes()).toHaveLength(1);
    expect(lost).toHaveBeenCalledOnce();
  });

  it("is a fresh session again after signing in", async () => {
    setSessionLostHandler(() => {});
    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);

    useAuthStore.getState().setSession("new-token", studentUser);
    expect(useAuthStore.getState().expired).toBe(false);
  });
});
