import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import GoogleCallbackPage from "@/features/auth/pages/GoogleCallbackPage";
import JoinPage from "@/features/join/pages/JoinPage";
import { rememberPending } from "@/features/auth/google/pkce";
import { readJoinContext, saveJoinContext } from "@/features/join/context";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { sampleClass, studentUser } from "@tests/support/fixtures";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CODE = "K7QM2PXA";
const CLASS_NAME = "TOEIC 600 Weekend";
const TEACHER = "Hoàng Thương";
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

function exchangeAnswers(status: number, body: unknown) {
  server.use(
    http.post(`${BASE}/auth/google`, () =>
      contractJson("/auth/google", "post", status, body),
    ),
  );
}

function arrive(pending: { joinCode?: string } = {}) {
  const state = "s".repeat(43);
  rememberPending({ verifier: "v".repeat(43), state, mode: "signin", ...pending });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/auth/google/callback", element: <GoogleCallbackPage /> },
      { path: "/join/:code", element: <JoinPage /> },
      { path: "/app", element: <p>student home</p> },
      { path: "/login", element: <p>login page</p> },
    ],
    { initialEntries: [`/auth/google/callback?code=abc&state=${state}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

beforeEach(() => {
  sessionStorage.clear();
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("back from Google", () => {
  it("says it is signing in while the code is exchanged", () => {
    exchangeAnswers(200, { accessToken: "token", expiresIn: 900, user: studentUser });
    arrive();
    expect(screen.getByRole("status")).toHaveTextContent("Đang hoàn tất đăng nhập…");
  });

  it("shows a new student the class the code enrolled them in", async () => {
    saveJoinContext({ code: CODE, className: CLASS_NAME, teacherName: TEACHER });
    exchangeAnswers(200, {
      accessToken: "token",
      expiresIn: 900,
      user: studentUser,
      enrolledClass: { ...sampleClass, name: CLASS_NAME },
    });
    const router = arrive({ joinCode: CODE });
    expect(
      await screen.findByRole("heading", { name: `Bạn đã vào lớp ${CLASS_NAME}` }),
    ).toBeVisible();
    expect(
      screen.getByText(`${TEACHER} sẽ thấy bạn trong danh sách lớp.`),
    ).toBeVisible();
    expect(router.state.location.pathname).toBe(`/join/${CODE}`);
    expect(readJoinContext()).toBeNull();
  });

  it("finishes the join for an account that already existed", async () => {
    saveJoinContext({ code: CODE, className: CLASS_NAME, teacherName: TEACHER });
    exchangeAnswers(200, { accessToken: "token", expiresIn: 900, user: studentUser });
    let joins = 0;
    server.use(
      http.post(`${BASE}/app/classes/join`, () => {
        joins += 1;
        return contractJson("/app/classes/join", "post", 200, {
          ...sampleClass,
          name: CLASS_NAME,
        });
      }),
    );
    arrive({ joinCode: CODE });
    expect(
      await screen.findByRole("heading", { name: `Bạn đã vào lớp ${CLASS_NAME}` }),
    ).toBeVisible();
    expect(joins).toBe(1);
  });

  it("keeps the class to join when the sign-in fails, with a way back", async () => {
    saveJoinContext({ code: CODE, className: CLASS_NAME, teacherName: TEACHER });
    exchangeAnswers(401, {
      error: {
        code: "UNAUTHORIZED",
        message: "Google không xác nhận được tài khoản này.",
        requestId: REQUEST_ID,
      },
    });
    arrive({ joinCode: CODE });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Google không xác nhận được tài khoản này.",
    );
    expect(screen.getByRole("link", { name: "Quay lại đăng nhập" })).toHaveAttribute(
      "href",
      "/login",
    );
    expect(readJoinContext()).not.toBeNull();
  });
});
