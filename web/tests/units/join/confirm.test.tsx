import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import JoinConfirmPage from "@/features/join/pages/JoinConfirmPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import { studentUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CODE = "K7M3P9QR";
const CLASS_NAME = "Tiếng Anh giao tiếp — Lớp A";

/**
 * §6.2: "The confirm step exists so the student sees WHICH class they are
 * joining before authenticating. Never create an account and enrol in one
 * blind tap."
 */

function renderConfirm() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const routes: RouteObject[] = [
    { path: "/join/:code/confirm", element: <JoinConfirmPage /> },
    { path: "/join", element: <p>join entry</p> },
    { path: "/app", element: <p>student home</p> },
  ];
  const router = createMemoryRouter(routes, {
    initialEntries: [`/join/${CODE}/confirm`],
  });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

function previewSucceeds() {
  server.use(
    http.post(`${BASE}/join/preview`, () =>
      contractJson("/join/preview", "post", 200, {
        classId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
        className: CLASS_NAME,
        teacherName: "Thuong",
      }),
    ),
  );
}

afterEach(() => {
  useAuthStore.getState().clearSession();
  vi.restoreAllMocks();
});

describe("the confirm step", () => {
  it("shows the class name before anything authenticates", async () => {
    let sawAuthCall = false;
    server.use(
      http.post(`${BASE}/auth/google`, () => {
        sawAuthCall = true;
        return new Response(null, { status: 500 });
      }),
    );
    previewSucceeds();
    renderConfirm();

    expect(
      await screen.findByRole("heading", { name: CLASS_NAME }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Thuong/)).toBeInTheDocument();
    expect(sawAuthCall).toBe(false);
  });

  it("offers Google to an anonymous visitor and does not enrol on its own", async () => {
    previewSucceeds();
    renderConfirm();
    await screen.findByRole("heading", { name: CLASS_NAME });

    // A button, not an automatic redirect: the tap is the consent.
    const confirm = screen.getByRole("button", { name: "Tiếp tục với Google" });
    expect(confirm).toBeEnabled();
  });

  it("enrols a signed-in student directly instead of offering Google", async () => {
    useAuthStore.setState({
      isBootstrapping: false,
      accessToken: "t",
      user: studentUser,
    });
    previewSucceeds();
    server.use(
      http.post(`${BASE}/app/classes/join`, () =>
        contractJson("/app/classes/join", "post", 200, {
          id: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          name: CLASS_NAME,
          studentCount: 12,
          openAssignmentCount: 0,
          archivedAt: null,
          selfJoinEnabled: true,
          createdAt: "2026-01-01T00:00:00Z",
        }),
      ),
    );

    const user = userEvent.setup();
    const router = renderConfirm();
    await screen.findByRole("heading", { name: CLASS_NAME });

    await user.click(screen.getByRole("button", { name: "Tham gia lớp này" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/app"));
  });

  it("renders the server's refusal and names no class", async () => {
    server.use(
      http.post(`${BASE}/join/preview`, () =>
        contractJson("/join/preview", "post", 404, {
          error: {
            code: "JOIN_CODE_EXPIRED",
            message: "Mã lớp này đã hết hạn. Vui lòng xin giáo viên mã mới.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        }),
      ),
    );
    renderConfirm();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Mã lớp này đã hết hạn.");
    expect(screen.queryByText(CLASS_NAME)).not.toBeInTheDocument();
    // No way to proceed: there is nothing to consent to.
    expect(screen.queryByRole("button", { name: /Google/ })).not.toBeInTheDocument();
  });

  it("does not retry a refused code", async () => {
    let attempts = 0;
    server.use(
      http.post(`${BASE}/join/preview`, () => {
        attempts += 1;
        return contractJson("/join/preview", "post", 404, {
          error: {
            code: "JOIN_CODE_INVALID",
            message: "Mã lớp không đúng. Vui lòng kiểm tra lại.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        });
      }),
    );
    renderConfirm();

    await screen.findByRole("alert");
    expect(attempts).toBe(1);
  });
});
