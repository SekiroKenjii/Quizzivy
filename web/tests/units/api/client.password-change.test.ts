import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import {
  api,
  setPasswordChangeHandler,
  __resetRefreshStateForTests,
} from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

function envelope(code: string, status: number) {
  return new Response(
    JSON.stringify({ error: { code, message: "boom", requestId: "req-1" } }),
    {
      status,
      headers: { "Content-Type": "application/json" },
    },
  );
}

let changeRequired: Mock<() => void>;

beforeEach(() => {
  __resetRefreshStateForTests();
  useAuthStore.getState().clearSession();
  useAuthStore.getState().setAccessToken("token");
  changeRequired = vi.fn<() => void>();
  setPasswordChangeHandler(changeRequired);
});

afterEach(() => {
  vi.unstubAllGlobals();
  setPasswordChangeHandler(() => {});
});

describe("a 403 PASSWORD_CHANGE_REQUIRED", () => {
  it("is announced before the error is thrown", async () => {
    vi.stubGlobal("fetch", () =>
      Promise.resolve(envelope("PASSWORD_CHANGE_REQUIRED", 403)),
    );
    let announcedFirst = false;
    changeRequired.mockImplementation(() => {
      announcedFirst = true;
    });

    const failure = await api("get", "/auth/me").catch((e: unknown) => e);

    expect(failure).toBeInstanceOf(ApiError);
    expect((failure as ApiError).code).toBe("PASSWORD_CHANGE_REQUIRED");
    expect(announcedFirst).toBe(true);
    expect(changeRequired).toHaveBeenCalledTimes(1);
  });

  it("is not announced for another 403", async () => {
    vi.stubGlobal("fetch", () => Promise.resolve(envelope("FORBIDDEN", 403)));

    await expect(api("get", "/auth/me")).rejects.toBeInstanceOf(ApiError);

    expect(changeRequired).not.toHaveBeenCalled();
  });
});
