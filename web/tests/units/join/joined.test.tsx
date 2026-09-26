import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import JoinPage from "@/features/join/pages/JoinPage";
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

let joins = 0;

function joinAnswers(status: number, body: unknown) {
  server.use(
    http.post(`${BASE}/app/classes/join`, () => {
      joins += 1;
      return contractJson("/app/classes/join", "post", status, body);
    }),
  );
}

function renderReturn() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const routes: RouteObject[] = [
    { path: "/join/:code", element: <JoinPage /> },
    { path: "/app/classes", element: <p>my classes</p> },
    { path: "/change-password", element: <p>change password</p> },
  ];
  const router = createMemoryRouter(routes, { initialEntries: [`/join/${CODE}`] });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

beforeEach(() => {
  joins = 0;
  sessionStorage.clear();
  saveJoinContext({ code: CODE, className: CLASS_NAME, teacherName: TEACHER });
  server.use(
    http.post(`${BASE}/join/preview`, () =>
      contractJson("/join/preview", "post", 200, {
        classId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
        className: CLASS_NAME,
        teacherName: TEACHER,
      }),
    ),
  );
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("coming back from sign-in to join", () => {
  it("finishes the join and says so", async () => {
    joinAnswers(200, { ...sampleClass, name: CLASS_NAME });
    useAuthStore.getState().setSession("token", studentUser);
    const user = userEvent.setup();
    const router = renderReturn();

    expect(
      await screen.findByRole("heading", { name: `Bạn đã vào lớp ${CLASS_NAME}` }),
    ).toBeVisible();
    expect(
      screen.getByText(`${TEACHER} sẽ thấy bạn trong danh sách lớp.`),
    ).toBeVisible();
    expect(joins).toBe(1);
    expect(readJoinContext()).toBeNull();

    await user.click(screen.getByRole("link", { name: "Đến lớp của tôi" }));
    expect(router.state.location.pathname).toBe("/app/classes");
  });

  it("counts a class the student is already in as joined", async () => {
    joinAnswers(404, {
      error: {
        code: "ALREADY_ENROLLED",
        message: "Bạn đã ở trong lớp này.",
        requestId: REQUEST_ID,
      },
    });
    useAuthStore.getState().setSession("token", studentUser);
    renderReturn();
    expect(
      await screen.findByRole("heading", { name: `Bạn đã vào lớp ${CLASS_NAME}` }),
    ).toBeVisible();
  });

  it("says plainly when the class could not take the student", async () => {
    joinAnswers(404, {
      error: {
        code: "JOIN_CODE_EXHAUSTED",
        message: "Mã lớp này đã dùng hết lượt.",
        requestId: REQUEST_ID,
      },
    });
    useAuthStore.getState().setSession("token", studentUser);
    renderReturn();
    expect(
      await screen.findByRole("heading", {
        name: `Chưa thể thêm bạn vào ${CLASS_NAME}`,
      }),
    ).toBeVisible();
    expect(screen.getByRole("alert")).toHaveTextContent("Mã lớp này đã dùng hết lượt.");
    expect(screen.getByRole("link", { name: "Đến lớp của tôi" })).toBeVisible();
    expect(readJoinContext()).toBeNull();
  });

  it("waits for the session before it joins, and joins once", async () => {
    joinAnswers(200, { ...sampleClass, name: CLASS_NAME });
    useAuthStore.setState({ user: null, accessToken: null, isBootstrapping: true });
    renderReturn();
    expect(screen.getByRole("status")).toHaveTextContent(
      `Đang thêm bạn vào ${CLASS_NAME}…`,
    );
    expect(joins).toBe(0);

    useAuthStore.getState().setSession("token", studentUser);
    expect(
      await screen.findByRole("heading", { name: `Bạn đã vào lớp ${CLASS_NAME}` }),
    ).toBeVisible();
    expect(joins).toBe(1);
  });

  it("sends an account with a temporary password to change it first", async () => {
    joinAnswers(200, { ...sampleClass, name: CLASS_NAME });
    useAuthStore
      .getState()
      .setSession("token", { ...studentUser, mustChangePassword: true });
    const router = renderReturn();
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/change-password"),
    );
    expect(joins).toBe(0);
    expect(readJoinContext()).not.toBeNull();
  });

  it("offers the form again when the visitor turns out to be signed out", async () => {
    useAuthStore.setState({ user: null, accessToken: null, isBootstrapping: true });
    renderReturn();
    useAuthStore.getState().finishBootstrap();
    expect(
      await screen.findByRole("button", { name: `Tham gia ${CLASS_NAME}` }),
    ).toBeVisible();
    expect(joins).toBe(0);
  });
});
