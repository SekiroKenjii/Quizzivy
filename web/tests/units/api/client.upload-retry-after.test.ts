import { beforeEach, describe, expect, it } from "vitest";
import { http } from "msw";
import { ApiError, uploadFile, __resetRefreshStateForTests } from "@/lib/api/client";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

function photo() {
  return new File(["png"], "me.png", { type: "image/png" });
}

function limited(retryAfter: string | null) {
  const response = contractJson("/teacher/media/{id}/replace", "post", 429, {
    error: { code: "RATE_LIMITED", message: "Quá nhanh.", requestId: REQUEST_ID },
  });
  if (retryAfter === null) response.headers.delete("Retry-After");
  else response.headers.set("Retry-After", retryAfter);
  return response;
}

async function refusal(): Promise<ApiError> {
  try {
    await uploadFile("/teacher/media/abc/replace", photo());
  } catch (cause) {
    if (cause instanceof ApiError) return cause;
    throw cause;
  }
  throw new Error("the upload was accepted");
}

beforeEach(() => {
  __resetRefreshStateForTests();
  useAuthStore.getState().setSession("token-a", teacherUser);
});

describe("an upload refused for going too fast", () => {
  it("carries the seconds the server asks the caller to wait", async () => {
    server.use(http.post(`${BASE}/teacher/media/abc/replace`, () => limited("45")));

    const error = await refusal();

    expect(error.status).toBe(429);
    expect(error.code).toBe("RATE_LIMITED");
    expect(error.retryAfterSeconds).toBe(45);
  });

  it("carries none when the server names none", async () => {
    server.use(http.post(`${BASE}/teacher/media/abc/replace`, () => limited(null)));

    expect((await refusal()).retryAfterSeconds).toBeUndefined();
  });

  it("carries the wait of a refusal that is not the server's envelope", async () => {
    server.use(
      http.post(
        `${BASE}/teacher/media/abc/replace`,
        () =>
          new Response("<html>slow down</html>", {
            status: 429,
            headers: { "Retry-After": "7" },
          }),
      ),
    );

    const error = await refusal();

    expect(error.code).toBe("UNKNOWN");
    expect(error.retryAfterSeconds).toBe(7);
  });
});
