import { beforeEach, describe, expect, it } from "vitest";
import { http } from "msw";
import { uploadFile, __resetRefreshStateForTests } from "@/lib/api/client";
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

function unauthorized() {
  return contractJson("/me/avatar", "put", 401, {
    error: { code: "UNAUTHORIZED", message: "Hết phiên.", requestId: REQUEST_ID },
  });
}

beforeEach(() => {
  __resetRefreshStateForTests();
  useAuthStore.getState().setSession("token-a", teacherUser);
});

describe("an upload's method", () => {
  it("is POST unless the caller asks for PUT", async () => {
    const methods: string[] = [];
    server.use(
      http.all(`${BASE}/me/avatar`, ({ request }) => {
        methods.push(request.method);
        return contractJson("/me/avatar", "put", 200, teacherUser);
      }),
    );

    await uploadFile("/me/avatar", photo(), { method: "PUT" });
    await uploadFile("/me/avatar", photo());

    expect(methods).toEqual(["PUT", "POST"]);
  });

  it("sends a PUT that answers 401 again after one shared refresh", async () => {
    const sent: { method: string; token: string | null }[] = [];
    let refreshes = 0;
    server.use(
      http.post(`${BASE}/auth/refresh`, () => {
        refreshes += 1;
        return contractJson("/auth/refresh", "post", 200, {
          accessToken: "token-b",
          expiresIn: 900,
        });
      }),
      http.put(`${BASE}/me/avatar`, ({ request }) => {
        const token = request.headers.get("Authorization");
        sent.push({ method: request.method, token });
        return token === "Bearer token-b"
          ? contractJson("/me/avatar", "put", 200, teacherUser)
          : unauthorized();
      }),
    );

    const [first, second] = await Promise.all([
      uploadFile("/me/avatar", photo(), { method: "PUT" }),
      uploadFile("/me/avatar", photo(), { method: "PUT" }),
    ]);

    expect(first).toEqual(teacherUser);
    expect(second).toEqual(teacherUser);
    expect(refreshes).toBe(1);
    expect(sent.map((request) => request.method)).toEqual(["PUT", "PUT", "PUT", "PUT"]);
    expect(sent.slice(2).map((request) => request.token)).toEqual([
      "Bearer token-b",
      "Bearer token-b",
    ]);
    expect(useAuthStore.getState().accessToken).toBe("token-b");
  });
});
