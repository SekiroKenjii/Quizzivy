import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import {
  act,
  cleanup,
  render,
  renderHook,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import type { ReactNode } from "react";
import {
  createMemoryRouter,
  MemoryRouter,
  RouterProvider,
  type DataRouter,
} from "react-router";
import { RequireSession } from "@/app/guards/RequireSession";
import GoogleCallbackPage from "@/features/auth/pages/GoogleCallbackPage";
import LoginPage from "@/features/auth/pages/LoginPage";
import { rememberPending } from "@/features/auth/google/pkce";
import { useBootstrapSession, useLogout } from "@/features/auth/useSession";
import {
  api,
  setSessionLostHandler,
  uploadFile,
  __resetRefreshStateForTests,
} from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { myClass, studentUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

let lost: Mock<() => void>;

function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { open, opened };
}

function unauthorizedBody() {
  return {
    error: {
      code: "UNAUTHORIZED",
      message: "Phiên đăng nhập không hợp lệ.",
      requestId: REQUEST_ID,
    },
  };
}

function unauthorized() {
  return contractJson("/auth/me", "get", 401, unauthorizedBody());
}

function refreshRefused() {
  return contractJson("/auth/refresh", "post", 401, {
    error: {
      code: "REFRESH_TOKEN_INVALID",
      message: "Phiên đăng nhập đã hết hạn.",
      requestId: REQUEST_ID,
    },
  });
}

function refreshedTo(accessToken: string) {
  return () =>
    contractJson("/auth/refresh", "post", 200, { accessToken, expiresIn: 900 });
}

function refreshAnswers(...answers: (() => Response)[]) {
  let asked = 0;
  return http.post(`${BASE}/auth/refresh`, () => {
    const answer = answers[asked] ?? refreshRefused;
    asked += 1;
    return answer();
  });
}

function meAnswersOnlyTo(token: string) {
  return http.get(`${BASE}/auth/me`, ({ request }) =>
    sentWith(request) === token
      ? contractJson("/auth/me", "get", 200, studentUser)
      : unauthorized(),
  );
}

function sentWith(request: Request) {
  return request.headers.get("Authorization")?.replace("Bearer ", "") ?? null;
}

function heldBody(body: unknown) {
  let finish: () => void = () => undefined;
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      finish = () => {
        controller.enqueue(new TextEncoder().encode(JSON.stringify(body)));
        controller.close();
      };
    },
  });
  return { stream, finish };
}

function Boot({ router }: Readonly<{ router: DataRouter }>) {
  useBootstrapSession();
  return <RouterProvider router={router} />;
}

function open(entry: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/auth/google/callback", element: <GoogleCallbackPage /> },
      { path: "/login", element: <LoginPage /> },
      { path: "/app", element: <p>student home</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <Boot router={router} />
    </QueryClientProvider>,
  );
}

function arriveFromGoogle() {
  const state = "s".repeat(43);
  rememberPending({ verifier: "v".repeat(43), state, mode: "signin" });
  open(`/auth/google/callback?code=abc&state=${state}`);
}

function googleAnswersAfter(opened: Promise<void>) {
  return http.post(`${BASE}/auth/google`, async () => {
    await opened;
    return contractJson("/auth/google", "post", 200, {
      accessToken: "token",
      expiresIn: 900,
      user: studentUser,
    });
  });
}

function expectTheNewSessionIntact() {
  expect(useAuthStore.getState().user).not.toBeNull();
  expect(useAuthStore.getState().user).toEqual(studentUser);
  expect(useAuthStore.getState().accessToken).toBe("token");
  expect(useAuthStore.getState().expired).toBe(false);
  expect(useAppState.getState().overlay.kind).toBe("none");
}

beforeEach(() => {
  sessionStorage.clear();
  __resetRefreshStateForTests();
  useAuthStore.setState({
    accessToken: null,
    user: null,
    isBootstrapping: true,
    expired: false,
  });
  useAppState.setState({
    bootPhase: "booting",
    bootError: null,
    bootAttempt: 0,
    overlay: { kind: "none" },
  });
  lost = vi.fn<() => void>(() => {
    if (useAuthStore.getState().expired) {
      useAppState.getState().showOverlay({ kind: "expired" });
    }
  });
  setSessionLostHandler(lost);
});

afterEach(() => {
  setSessionLostHandler(() => {});
  useAuthStore.getState().clearSession();
  useAppState.getState().closeOverlay();
});

describe("a 401 from before signing in", () => {
  it("keeps the session Google gave when the boot's refresh is refused after it", async () => {
    const refreshAsked = gate();
    const refresh = gate();
    const exchange = gate();
    server.use(
      http.get(`${BASE}/auth/me`, () => unauthorized()),
      http.post(`${BASE}/auth/refresh`, async () => {
        refreshAsked.open();
        await refresh.opened;
        return refreshRefused();
      }),
      googleAnswersAfter(exchange.opened),
    );

    arriveFromGoogle();
    await refreshAsked.opened;
    exchange.open();
    expect(await screen.findByText("student home")).toBeInTheDocument();
    refresh.open();
    await waitFor(() => expect(useAppState.getState().bootPhase).toBe("ready"));

    expectTheNewSessionIntact();
  });

  it("keeps the session Google gave while the boot's first 401 was still being read", async () => {
    const body = heldBody(unauthorizedBody());
    const exchange = gate();
    server.use(
      http.get(
        `${BASE}/auth/me`,
        () =>
          new Response(body.stream, {
            status: 401,
            headers: { "Content-Type": "application/json" },
          }),
      ),
      http.post(`${BASE}/auth/refresh`, () => refreshRefused()),
      googleAnswersAfter(exchange.opened),
    );

    arriveFromGoogle();
    await waitFor(() => expect(lost).toHaveBeenCalled());
    expect(useAppState.getState().bootPhase).toBe("booting");
    exchange.open();
    expect(await screen.findByText("student home")).toBeInTheDocument();
    body.finish();
    await waitFor(() => expect(useAppState.getState().bootPhase).toBe("ready"));

    expectTheNewSessionIntact();
  });

  it("keeps a password sign-in made while the boot's refresh was out", async () => {
    const refreshAsked = gate();
    const refresh = gate();
    server.use(
      http.get(`${BASE}/auth/me`, () => unauthorized()),
      http.post(`${BASE}/auth/refresh`, async () => {
        refreshAsked.open();
        await refresh.opened;
        return refreshRefused();
      }),
      http.post(`${BASE}/auth/login`, () =>
        contractJson("/auth/login", "post", 200, {
          accessToken: "token",
          expiresIn: 900,
          user: studentUser,
        }),
      ),
    );
    const user = userEvent.setup();

    open("/login");
    await refreshAsked.opened;
    await user.type(screen.getByLabelText("Email"), studentUser.email);
    await user.type(screen.getByLabelText("Mật khẩu"), "quizzivy-dev");
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));
    expect(await screen.findByText("student home")).toBeInTheDocument();
    refresh.open();
    await waitFor(() => expect(useAppState.getState().bootPhase).toBe("ready"));

    expectTheNewSessionIntact();
  });
});

describe.each([
  {
    name: "a request",
    method: "get" as const,
    path: "/app/classes",
    send: (): Promise<unknown> => api("get", "/app/classes"),
  },
  {
    name: "an upload",
    method: "post" as const,
    path: "/teacher/media",
    send: (): Promise<unknown> =>
      uploadFile("/teacher/media", new File(["x"], "clip.mp3", { type: "audio/mpeg" })),
  },
])("$name answered 401", ({ method, path, send }) => {
  beforeEach(() => {
    useAuthStore.getState().setSession("token-a", studentUser);
  });

  it("loses the session it was sent under when the refresh is refused", async () => {
    server.use(
      http[method](`${BASE}${path}`, () => unauthorized()),
      refreshAnswers(refreshRefused),
    );

    await expect(send()).rejects.toMatchObject({ status: 401 });

    expect(lost).toHaveBeenCalledOnce();
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      user: studentUser,
      expired: true,
    });
    expect(useAppState.getState().overlay.kind).toBe("expired");
  });

  it("loses the session it was sent under when the retry is refused too", async () => {
    const carried: (string | null)[] = [];
    server.use(
      http[method](`${BASE}${path}`, ({ request }) => {
        carried.push(sentWith(request));
        return unauthorized();
      }),
      refreshAnswers(refreshedTo("token-b")),
    );

    await expect(send()).rejects.toMatchObject({ status: 401 });

    expect(carried).toEqual(["token-a", "token-b"]);
    expect(lost).toHaveBeenCalledOnce();
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      user: studentUser,
      expired: true,
    });
    expect(useAppState.getState().overlay.kind).toBe("expired");
  });

  it("leaves a refreshed session alone when it carried the token before it", async () => {
    const carried: (string | null)[] = [];
    const late = gate();
    server.use(
      http[method](`${BASE}${path}`, async ({ request }) => {
        carried.push(sentWith(request));
        await late.opened;
        return unauthorized();
      }),
      meAnswersOnlyTo("token-b"),
      refreshAnswers(refreshedTo("token-b"), refreshRefused),
    );

    const failure = send().catch((cause: unknown) => cause);
    await api("get", "/auth/me");
    expect(useAuthStore.getState().accessToken).toBe("token-b");
    late.open();

    expect(await failure).toMatchObject({ status: 401 });
    expect(carried).toEqual(["token-a"]);
    expect(lost).not.toHaveBeenCalled();
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: "token-b",
      user: studentUser,
      expired: false,
    });
    expect(useAppState.getState().overlay.kind).toBe("none");
  });

  it("leaves a newer session alone when its retry carried the token before it", async () => {
    const retried = gate();
    const late = gate();
    server.use(
      http[method](`${BASE}${path}`, async ({ request }) => {
        if (sentWith(request) !== "token-b") return unauthorized();
        retried.open();
        await late.opened;
        return unauthorized();
      }),
      meAnswersOnlyTo("token-c"),
      refreshAnswers(refreshedTo("token-b"), refreshedTo("token-c")),
    );

    const failure = send().catch((cause: unknown) => cause);
    await retried.opened;
    await api("get", "/auth/me");
    expect(useAuthStore.getState().accessToken).toBe("token-c");
    late.open();

    expect(await failure).toMatchObject({ status: 401 });
    expect(lost).not.toHaveBeenCalled();
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: "token-c",
      user: studentUser,
      expired: false,
    });
    expect(useAppState.getState().overlay.kind).toBe("none");
  });
});

describe("a late 401 whose own refresh succeeds", () => {
  it("is sent again with the newest token, as before", async () => {
    useAuthStore.getState().setSession("token-a", studentUser);
    const late = gate();
    server.use(
      http.get(`${BASE}/app/classes`, async ({ request }) => {
        if (sentWith(request) === "token-c") {
          return contractJson("/app/classes", "get", 200, { items: [myClass] });
        }
        await late.opened;
        return unauthorized();
      }),
      meAnswersOnlyTo("token-b"),
      refreshAnswers(refreshedTo("token-b"), refreshedTo("token-c")),
    );

    const slow = api("get", "/app/classes");
    await api("get", "/auth/me");
    late.open();

    await expect(slow).resolves.toEqual({ items: [myClass] });
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: "token-c",
      user: studentUser,
      expired: false,
    });
    expect(lost).not.toHaveBeenCalled();
  });
});

describe("a 401 that arrives after signing out", () => {
  it("is not a lost session and raises no sign-in-again overlay", async () => {
    useAuthStore.getState().setSession("token-a", studentUser);
    const late = gate();
    server.use(
      http.get(`${BASE}/app/classes`, async () => {
        await late.opened;
        return unauthorized();
      }),
      http.post(`${BASE}/auth/logout`, () => new HttpResponse(null, { status: 204 })),
      http.post(`${BASE}/auth/refresh`, () => refreshRefused()),
    );
    const client = new QueryClient();
    const { result } = renderHook(() => useLogout(), {
      wrapper: ({ children }: Readonly<{ children: ReactNode }>) => (
        <QueryClientProvider client={client}>
          <MemoryRouter>{children}</MemoryRouter>
        </QueryClientProvider>
      ),
    });

    const failure = api("get", "/app/classes").catch((cause: unknown) => cause);
    await act(() => result.current());
    expect(useAuthStore.getState().user).toBeNull();
    late.open();

    expect(await failure).toBeInstanceOf(ApiError);
    expect(lost).not.toHaveBeenCalled();
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      user: null,
      expired: false,
    });
    expect(useAppState.getState().overlay.kind).toBe("none");
  });
});

const MAINTENANCE = {
  startsAt: "2026-10-04T10:00:00Z",
  endsAt: "2026-10-04T11:00:00Z",
};

function SignOut({ onSignedOut }: Readonly<{ onSignedOut: () => void }>) {
  const logout = useLogout();
  return (
    <button type="button" onClick={() => void logout().then(onSignedOut)}>
      sign out
    </button>
  );
}

function openSignedIn() {
  const page = gate();
  const signedOut = vi.fn<() => void>();
  const router = createMemoryRouter(
    [
      {
        element: <RequireSession />,
        children: [{ path: "/app", element: <SignOut onSignedOut={signedOut} /> }],
      },
      {
        path: "/login",
        lazy: async () => {
          await page.opened;
          return { element: <p>login page</p> };
        },
      },
    ],
    { initialEntries: ["/app"] },
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, page, signedOut };
}

function classesRefusedAfter(opened: Promise<void>) {
  return http.get(`${BASE}/app/classes`, async () => {
    await opened;
    return unauthorized();
  });
}

function expectSignedOutOnTheLoginPage(router: DataRouter) {
  expect(useAppState.getState().overlay.kind).toBe("none");
  expect(router.state.location.pathname).toBe("/login");
  expect(useAuthStore.getState()).toMatchObject({
    accessToken: null,
    user: null,
    expired: false,
  });
}

describe("a 401 handled while signing out", () => {
  beforeEach(() => {
    useAuthStore.getState().setSession("token-a", studentUser);
  });

  afterEach(() => {
    cleanup();
  });

  it("leaves no sign-in-again overlay on the login page when the refusal lands before the session is cleared", async () => {
    const late = gate();
    server.use(
      classesRefusedAfter(late.opened),
      http.post(`${BASE}/auth/logout`, () => new HttpResponse(null, { status: 204 })),
      http.post(`${BASE}/auth/refresh`, () => refreshRefused()),
    );
    const user = userEvent.setup();
    const { router, page, signedOut } = openSignedIn();

    const failure = api("get", "/app/classes").catch((cause: unknown) => cause);
    await user.click(await screen.findByRole("button", { name: "sign out" }));
    await waitFor(() => expect(router.state.navigation.state).not.toBe("idle"));
    late.open();
    await waitFor(() => expect(lost).toHaveBeenCalled());
    expect(useAppState.getState().overlay.kind).toBe("expired");
    page.open();
    expect(await screen.findByText("login page")).toBeInTheDocument();
    await waitFor(() => expect(signedOut).toHaveBeenCalled());

    expectSignedOutOnTheLoginPage(router);
    expect(await failure).toBeInstanceOf(ApiError);
  });

  it("leaves none when the refusal lands while the logout request is still out", async () => {
    const late = gate();
    const logoutAsked = gate();
    const logout = gate();
    server.use(
      classesRefusedAfter(late.opened),
      http.post(`${BASE}/auth/logout`, async () => {
        logoutAsked.open();
        await logout.opened;
        return new HttpResponse(null, { status: 204 });
      }),
      http.post(`${BASE}/auth/refresh`, () => refreshRefused()),
    );
    const user = userEvent.setup();
    const { router, page, signedOut } = openSignedIn();

    const failure = api("get", "/app/classes").catch((cause: unknown) => cause);
    await user.click(await screen.findByRole("button", { name: "sign out" }));
    await logoutAsked.opened;
    late.open();
    await waitFor(() => expect(lost).toHaveBeenCalled());
    expect(useAppState.getState().overlay.kind).toBe("expired");
    expect(router.state.navigation.state).toBe("idle");
    logout.open();
    await waitFor(() => expect(router.state.navigation.state).not.toBe("idle"));
    page.open();
    expect(await screen.findByText("login page")).toBeInTheDocument();
    await waitFor(() => expect(signedOut).toHaveBeenCalled());

    expectSignedOutOnTheLoginPage(router);
    expect(await failure).toBeInstanceOf(ApiError);
  });

  it("leaves a maintenance overlay alone", async () => {
    server.use(
      http.post(`${BASE}/auth/logout`, () => new HttpResponse(null, { status: 204 })),
    );
    const user = userEvent.setup();
    const { router, page, signedOut } = openSignedIn();
    useAppState.getState().showOverlay({ kind: "maintenance", window: MAINTENANCE });

    await user.click(await screen.findByRole("button", { name: "sign out" }));
    await waitFor(() => expect(router.state.navigation.state).not.toBe("idle"));
    page.open();
    expect(await screen.findByText("login page")).toBeInTheDocument();
    await waitFor(() => expect(signedOut).toHaveBeenCalled());

    expect(useAuthStore.getState().user).toBeNull();
    expect(useAppState.getState().overlay).toEqual({
      kind: "maintenance",
      window: MAINTENANCE,
    });
  });

  it("still raises the overlay for a refusal when nobody is signing out", async () => {
    const late = gate();
    server.use(
      classesRefusedAfter(late.opened),
      http.post(`${BASE}/auth/refresh`, () => refreshRefused()),
    );
    const { router, signedOut } = openSignedIn();
    expect(await screen.findByRole("button", { name: "sign out" })).toBeInTheDocument();

    const failure = api("get", "/app/classes").catch((cause: unknown) => cause);
    late.open();

    expect(await failure).toBeInstanceOf(ApiError);
    expect(lost).toHaveBeenCalledOnce();
    expect(signedOut).not.toHaveBeenCalled();
    expect(router.state.location.pathname).toBe("/app");
    expect(useAuthStore.getState()).toMatchObject({
      accessToken: null,
      user: studentUser,
      expired: true,
    });
    expect(useAppState.getState().overlay.kind).toBe("expired");
  });
});
