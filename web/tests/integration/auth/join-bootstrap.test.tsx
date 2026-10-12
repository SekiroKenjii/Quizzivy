import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, Outlet, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { useBootstrapSession } from "@/features/auth/useSession";
import JoinPage from "@/features/join/pages/JoinPage";
import { authStore, useAuthStore } from "@/stores/auth";
import { useAppState } from "@/stores/appState";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { studentUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CODE = "K7QM2PXA";
const CLASS_NAME = "TOEIC 600 Weekend";
const TEACHER = "Hoàng Thương";
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { open, opened };
}

function Bootstrap() {
  useBootstrapSession();
  return <Outlet />;
}

function mountJoin() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      {
        element: <Bootstrap />,
        children: [{ path: "/join/:code", element: <JoinPage /> }],
      },
    ],
    { initialEntries: [`/join/${CODE}`] },
  );
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return () => {
    view.unmount();
    router.dispose();
    client.clear();
  };
}

function controlledReplies(status: 401 | 403) {
  const previewStarted = gate();
  const bootstrapStarted = gate();
  const previewRelease = gate();
  const bootstrapRelease = gate();
  const previewReturned = gate();
  const bootstrapReturned = gate();
  let previewInvoked = false;
  let bootstrapInvoked = false;
  const calls: string[] = [];
  const record = ({ request }: { request: Request }) => {
    const url = new URL(request.url);
    if (url.origin === BASE) calls.push(`${request.method} ${url.pathname}`);
  };
  server.events.on("request:start", record);
  server.use(
    http.get(`${BASE}/auth/me`, async () => {
      bootstrapInvoked = true;
      bootstrapStarted.open();
      try {
        await bootstrapRelease.opened;
        const body = {
          error: {
            code: status === 401 ? "UNAUTHORIZED" : "FORBIDDEN",
            message: "Refused",
            requestId: REQUEST_ID,
          },
        };
        return status === 401
          ? contractJson("/auth/me", "get", 401, body)
          : HttpResponse.json(body, { status: 403 });
      } finally {
        bootstrapReturned.open();
      }
    }),
    http.post(`${BASE}/auth/refresh`, () => {
      return contractJson("/auth/refresh", "post", 401, {
        error: {
          code: "REFRESH_TOKEN_INVALID",
          message: "Refused",
          requestId: REQUEST_ID,
        },
      });
    }),
    http.post(`${BASE}/join/preview`, async ({ request }) => {
      previewInvoked = true;
      try {
        expect(await request.json()).toEqual({ joinCode: CODE });
        previewStarted.open();
        await previewRelease.opened;
        return contractJson("/join/preview", "post", 200, {
          classId: REQUEST_ID,
          className: CLASS_NAME,
          teacherName: TEACHER,
        });
      } finally {
        previewReturned.open();
      }
    }),
  );
  return {
    previewStarted,
    bootstrapStarted,
    previewRelease,
    bootstrapRelease,
    calls,
    async dispose(unmount: () => void) {
      bootstrapRelease.open();
      previewRelease.open();
      unmount();
      await Promise.all([
        ...(previewInvoked ? [previewReturned.opened] : []),
        ...(bootstrapInvoked ? [bootstrapReturned.opened] : []),
      ]);
      server.events.removeListener("request:start", record);
    },
  };
}

beforeEach(() => {
  sessionStorage.clear();
  useAuthStore.getState().clearSession();
  useAuthStore.setState({ isBootstrapping: true });
  useAppState.setState({
    bootPhase: "booting",
    bootError: null,
    bootAttempt: 0,
    overlay: { kind: "none" },
  });
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

async function bootstrapSettled() {
  await waitFor(() => expect(useAppState.getState().bootPhase).toBe("ready"));
  expect(useAuthStore.getState().isBootstrapping).toBe(false);
}

async function foundClass() {
  expect(await screen.findByText(CLASS_NAME, { exact: true })).toBeVisible();
  expect(screen.getByText(TEACHER, { exact: true })).toBeVisible();
  expect(screen.queryByText("Chưa kiểm tra được mã. Vui lòng thử lại.")).toBeNull();
}

describe("anonymous bootstrap and the public Join preview", () => {
  it.each([401, 403] as const)(
    "keeps a held class preview after bootstrap refusal %s",
    async (status) => {
      if (status === 403)
        useAuthStore.getState().setAccessToken("anonymous-memory-token");
      const replies = controlledReplies(status);
      const lease = authStore.captureActor();
      const unmount = mountJoin();
      try {
        await Promise.all([
          replies.previewStarted.opened,
          replies.bootstrapStarted.opened,
        ]);
        replies.bootstrapRelease.open();
        await bootstrapSettled();
        expect(useAuthStore.getState().user).toBeNull();
        replies.previewRelease.open();
        await foundClass();
        expect(authStore.isCurrent(lease)).toBe(true);
        expect(useAuthStore.getState().accessToken).toBeNull();
        expect(replies.calls).toEqual(
          expect.arrayContaining(["GET /auth/me", "POST /join/preview"]),
        );
        expect(
          replies.calls.filter((call) => call === "POST /join/preview"),
        ).toHaveLength(1);
        expect(
          replies.calls.filter((call) => call === "POST /auth/refresh"),
        ).toHaveLength(status === 401 ? 1 : 0);
        expect(replies.calls).not.toContain("POST /auth/login");
        expect(replies.calls).not.toContain("POST /auth/google");
      } finally {
        await replies.dispose(unmount);
      }
    },
  );

  it("keeps the class when its preview arrives before bootstrap settles", async () => {
    const replies = controlledReplies(401);
    const unmount = mountJoin();
    try {
      await Promise.all([
        replies.previewStarted.opened,
        replies.bootstrapStarted.opened,
      ]);
      replies.previewRelease.open();
      await foundClass();
      replies.bootstrapRelease.open();
      await bootstrapSettled();
      await foundClass();
      expect(
        replies.calls.filter((call) => call === "POST /join/preview"),
      ).toHaveLength(1);
    } finally {
      await replies.dispose(unmount);
    }
  });

  it("still discards a held old preview after a genuine new admission", async () => {
    const replies = controlledReplies(401);
    const unmount = mountJoin();
    try {
      await Promise.all([
        replies.previewStarted.opened,
        replies.bootstrapStarted.opened,
      ]);
      replies.bootstrapRelease.open();
      await bootstrapSettled();
      act(() => useAuthStore.getState().setSession("new-admission", studentUser));
      replies.previewRelease.open();
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "Chưa kiểm tra được mã. Vui lòng thử lại.",
      );
      expect(screen.queryByText(CLASS_NAME, { exact: true })).toBeNull();
      expect(useAuthStore.getState().user?.id).toBe(studentUser.id);
      expect(useAuthStore.getState().accessToken).toBe("new-admission");
      expect(
        replies.calls.filter((call) => call === "POST /join/preview"),
      ).toHaveLength(1);
    } finally {
      await replies.dispose(unmount);
    }
  });
});
