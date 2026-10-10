import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { ProfileSection } from "@/features/settings/sections/Profile";
import {
  StudentAppearanceSection,
  StudentProfileSection,
} from "@/features/auth/components/StudentSettingsSections";
import { Toaster } from "@/components/ui/sonner";
import { authStore, useAuthStore } from "@/stores/auth";
import { studentUser, teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import i18n from "@/lib/i18n";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { useLogout, checkLogoutCleanup } from "@/features/auth/useSession";
import { signInAccount } from "@/features/auth/accountPreferences";
import LoginPage from "@/features/auth/pages/LoginPage";
import { openDraftScope } from "@/lib/drafts/store";
import { login } from "@/features/auth/api";
import { AppStateLayer } from "@/app/boot/AppStateLayer";
import { useAppState } from "@/stores/appState";
import { queryClient } from "@/app/queryClient";
import { transitionStatus } from "@/lib/api/authTransition";

const drafts = vi.hoisted(() => ({ clear: vi.fn<() => Promise<void>>(), real: false }));
vi.mock("@/lib/drafts/store", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/drafts/store")>();
  return {
    ...original,
    clearAuthoringDrafts: () =>
      drafts.real ? original.clearAuthoringDrafts() : drafts.clear(),
  };
});

function gate() {
  let open!: () => void;
  const promise = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { promise, open };
}

function logoutFixture() {
  let departure!: () => Promise<void>;
  function Departure() {
    departure = useLogout();
    return <p>account A</p>;
  }
  const router = createMemoryRouter(
    [
      { path: "/app", element: <Departure /> },
      { path: "/login", element: <p>login</p> },
    ],
    { initialEntries: ["/app"] },
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { logout: () => departure(), router };
}

beforeEach(async () => {
  drafts.real = false;
  drafts.clear.mockReset().mockResolvedValue(undefined);
  await i18n.changeLanguage("vi");
  useAuthStore.getState().setSession("a", studentUser);
});
afterEach(() => {
  vi.useRealTimers();
  cleanup();
  useAuthStore.getState().clearSession();
});

it("persists the larger-text control's changed key while a dirty legal name and focus survive", async () => {
  const bodies: unknown[] = [];
  server.use(
    http.patch("http://localhost:8080/me/preferences", async ({ request }) => {
      bodies.push(await request.json());
      return HttpResponse.json({ largerTestText: true });
    }),
  );
  render(
    <>
      <StudentProfileSection />
      <StudentAppearanceSection />
      <Toaster />
    </>,
  );
  const user = userEvent.setup();
  const name = screen.getByLabelText("Họ và tên");
  await user.clear(name);
  await user.type(name, "Draft legal name");
  await user.click(screen.getByRole("switch"));
  await waitFor(() => expect(bodies).toEqual([{ largerTestText: true }]));
  expect(name).toHaveValue("Draft legal name");
  expect(screen.getByRole("button", { name: "Lưu thay đổi" })).toBeEnabled();
  expect(useAuthStore.getState().user?.fullName).toBe(studentUser.fullName);
});

it("rolls back a failed theme preview and keeps explicit keyboard Retry reachable", async () => {
  let calls = 0;
  server.use(
    http.patch("http://localhost:8080/me/preferences", async () => {
      calls += 1;
      return calls === 1
        ? HttpResponse.json(
            { error: { code: "INTERNAL", message: "Not saved" } },
            { status: 500 },
          )
        : HttpResponse.json({ theme: "dark" });
    }),
  );
  render(
    <>
      <StudentAppearanceSection />
      <Toaster />
    </>,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Tối" }));
  await waitFor(() => expect(calls).toBe(1));
  await waitFor(() => expect(document.documentElement).not.toHaveClass("dark"));
  const retry = await screen.findAllByRole("button", { name: "Thử lại" });
  retry[0]!.focus();
  await user.keyboard("{Enter}");
  await waitFor(() => expect(calls).toBe(2));
  await waitFor(() => expect(document.documentElement).toHaveClass("dark"));
  expect(useAuthStore.getState().user?.preferences?.theme).toBe("dark");
});

it("holds one departure through real logout and cleanup, rejects B admission, and explicitly retries only a failed cleanup", async () => {
  const logout = gate();
  const clear = gate();
  let logoutCalls = 0;
  let loginCalls = 0;
  drafts.clear
    .mockImplementationOnce(() =>
      clear.promise.then(() => {
        throw new Error("IDB failed");
      }),
    )
    .mockResolvedValue(undefined);
  server.use(
    http.post("http://localhost:8080/auth/logout", async () => {
      logoutCalls += 1;
      await logout.promise;
      return new HttpResponse(null, { status: 204 });
    }),
    http.post("http://localhost:8080/auth/login", () => {
      loginCalls += 1;
      return HttpResponse.json({ accessToken: "b", expiresIn: 900, user: studentUser });
    }),
  );
  const fixture = logoutFixture();
  let completion!: Promise<void>;
  act(() => {
    completion = fixture.logout();
  });
  expect(useAuthStore.getState().user).toBeNull();
  await waitFor(() => expect(logoutCalls).toBe(1));
  await expect(
    admitAccount("login", (ticket) => login(studentUser.email, "password", ticket)),
  ).rejects.toMatchObject({ phase: "account" });
  expect(drafts.clear).not.toHaveBeenCalled();
  logout.open();
  await waitFor(() => expect(drafts.clear).toHaveBeenCalledOnce());
  expect(transitionStatus().phase).toBe("cleanup");
  await checkLogoutCleanup();
  expect(drafts.clear).toHaveBeenCalledOnce();
  await expect(
    admitAccount("login", (ticket) => login(studentUser.email, "password", ticket)),
  ).rejects.toMatchObject({ phase: "cleanup" });
  expect(loginCalls).toBe(0);
  clear.open();
  await completion;
  expect(
    await screen.findByText("Không thể dọn dữ liệu tài khoản. Thử lại."),
  ).toBeInTheDocument();
  expect(transitionStatus().phase).toBe("cleanup");
  await act(async () => {
    await checkLogoutCleanup();
  });
  expect(drafts.clear).toHaveBeenCalledTimes(2);
  expect(logoutCalls).toBe(1);
  expect(transitionStatus().kind).toBe("idle");
  await admitAccount("login", (ticket) => login(studentUser.email, "password", ticket));
  expect(loginCalls).toBe(1);
  expect(useAuthStore.getState().accessToken).toBe("b");
});

it("settles the ten-second cleanup status while keeping B blocked until natural cleanup finishes", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const clear = gate();
  drafts.clear.mockImplementation(() => clear.promise);
  server.use(
    http.post(
      "http://localhost:8080/auth/logout",
      () => new HttpResponse(null, { status: 204 }),
    ),
  );
  const fixture = logoutFixture();
  let completion!: Promise<void>;
  act(() => {
    completion = fixture.logout();
  });
  await waitFor(() => expect(drafts.clear).toHaveBeenCalledOnce());
  await act(async () => {
    await vi.advanceTimersByTimeAsync(10_000);
    await completion;
  });
  expect(
    screen.getByText("Việc dọn dữ liệu tài khoản chưa hoàn tất."),
  ).toBeInTheDocument();
  await checkLogoutCleanup();
  expect(drafts.clear).toHaveBeenCalledOnce();
  expect(transitionStatus().kind).toBe("logout");
  await expect(
    admitAccount("login", () =>
      Promise.resolve({ accessToken: "b", user: studentUser }),
    ),
  ).rejects.toMatchObject({ phase: "cleanup" });
  await act(async () => {
    clear.open();
    await clear.promise;
  });
  await waitFor(() => expect(transitionStatus().kind).toBe("idle"));
  expect(fixture.router.state.location.pathname).toBe("/app");
});

it("never admits a late password result after the twenty-second status deadline", async () => {
  vi.useFakeTimers();
  const body = gate();
  const work = vi.fn(async () => {
    await body.promise;
    return { accessToken: "late", user: studentUser };
  });
  const visible = admitAccount("login", work).catch((error: unknown) => error);
  await vi.advanceTimersByTimeAsync(20_000);
  expect(await visible).toMatchObject({ phase: "account" });
  expect(useAuthStore.getState().user).toBeNull();
  await expect(admitAccount("google", work)).rejects.toMatchObject({
    phase: "account",
  });
  expect(work).toHaveBeenCalledOnce();
  body.open();
  await vi.waitFor(() => expect(transitionStatus().kind).toBe("idle"));
  expect(useAuthStore.getState().accessToken).toBeNull();
});

it("reports actual cleanup rejection even after the raw logout UI deadline", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const logout = gate();
  drafts.clear
    .mockRejectedValueOnce(new Error("IDB rejected after deadline"))
    .mockResolvedValue(undefined);
  server.use(
    http.post("http://localhost:8080/auth/logout", async () => {
      await logout.promise;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  const fixture = logoutFixture();
  let completion!: Promise<void>;
  act(() => {
    completion = fixture.logout();
  });
  await waitFor(() => expect(transitionStatus().cookieKind).toBe("logout"));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(20_000);
    await completion;
  });
  expect(
    screen.getByText("Yêu cầu tài khoản vẫn đang được xử lý."),
  ).toBeInTheDocument();
  logout.open();
  await waitFor(() => expect(drafts.clear).toHaveBeenCalledOnce());
  try {
    expect(
      await screen.findByText("Không thể dọn dữ liệu tài khoản. Thử lại."),
    ).toBeInTheDocument();
  } finally {
    await act(async () => {
      await checkLogoutCleanup();
    });
  }
  expect(drafts.clear).toHaveBeenCalledTimes(2);
  expect(transitionStatus().kind).toBe("idle");
});

it("saves only the changed language, applies it once the server keeps it, and stores it for the next boot", async () => {
  useAuthStore.getState().setSession("a", { ...teacherUser, locale: "vi" });
  const reply = gate();
  const bodies: unknown[] = [];
  server.use(
    http.patch("http://localhost:8080/auth/me", async ({ request }) => {
      bodies.push(await request.json());
      await reply.promise;
      return HttpResponse.json({ ...teacherUser, locale: "en" });
    }),
  );
  render(
    <>
      <ProfileSection />
      <Toaster />
    </>,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole("combobox", { name: "Ngôn ngữ" }));
  await user.click(screen.getByRole("option", { name: "English" }));
  expect(i18n.language).toBe("vi");
  await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
  await waitFor(() => expect(bodies).toEqual([{ locale: "en" }]));
  expect(i18n.language).toBe("vi");
  reply.open();
  await waitFor(() => expect(useAuthStore.getState().user?.locale).toBe("en"));
  await waitFor(() => expect(i18n.language).toBe("en"));
  expect(localStorage.getItem("quizzivy.locale")).toBe("en");
  expect(document.documentElement.lang).toBe("en");
  expect(await screen.findByText("Settings saved")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();
  expect(useAuthStore.getState().user?.fullName).toBe(teacherUser.fullName);
});

it("retains an unsupported server zone and persistent Check recovery without patching a fallback", async () => {
  let reads = 0;
  const patches = vi.fn(() => HttpResponse.json({ theme: "light" }));
  server.use(
    http.get("http://localhost:8080/auth/me", () => {
      reads += 1;
      return HttpResponse.json({ ...studentUser, timeZone: "UTC" });
    }),
    http.patch("http://localhost:8080/auth/me", patches),
    http.patch("http://localhost:8080/me/preferences", patches),
  );
  useAuthStore
    .getState()
    .setSession("a", { ...studentUser, timeZone: "Zone/NotSupported" });
  render(
    <>
      <StudentAppearanceSection />
      <Toaster />
    </>,
  );
  const warning =
    "Trình duyệt này chưa hỗ trợ múi giờ của tài khoản. Thời gian tạm hiển thị theo giờ Việt Nam.";
  expect(screen.getAllByText(warning).length).toBeGreaterThan(0);
  expect(useAuthStore.getState().user?.timeZone).toBe("Zone/NotSupported");
  expect(patches).not.toHaveBeenCalled();
  const user = userEvent.setup();
  await user.click(screen.getAllByRole("button", { name: "Kiểm tra lại" })[0]!);
  await waitFor(() => expect(reads).toBe(1));
  await waitFor(() => expect(useAuthStore.getState().user?.timeZone).toBe("UTC"));
  await waitFor(() => expect(screen.queryByText(warning)).not.toBeInTheDocument());
  expect(patches).not.toHaveBeenCalled();
});

it.each(["unmount", "new-window"] as const)(
  "ignores a late maintenance check after %s without invalidating current queries",
  async (change) => {
    const response = gate();
    let entered = false;
    server.use(
      http.get("http://localhost:8080/public/status", async () => {
        entered = true;
        await response.promise;
        return HttpResponse.json({ maintenance: null });
      }),
    );
    const original = {
      startsAt: "2026-10-01T15:00:00Z",
      endsAt: "2026-10-01T16:30:00Z",
    };
    const next = { ...original, endsAt: "2026-10-01T17:00:00Z" };
    useAppState.setState({
      bootPhase: "ready",
      overlay: { kind: "maintenance", window: original },
    });
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const router = createMemoryRouter([{ path: "/", element: <p>page</p> }]);
    const view = render(<AppStateLayer router={router} />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Kiểm tra lại" }));
    await waitFor(() => expect(entered).toBe(true));
    if (change === "unmount") view.unmount();
    if (change === "new-window")
      act(() =>
        useAppState.getState().showOverlay({ kind: "maintenance", window: next }),
      );
    await act(async () => {
      response.open();
      await response.promise;
      await new Promise((resolve) => setTimeout(resolve, 30));
    });
    try {
      expect(useAppState.getState().overlay).toEqual({
        kind: "maintenance",
        window: change === "unmount" ? original : next,
      });
      expect(invalidate).not.toHaveBeenCalled();
    } finally {
      invalidate.mockRestore();
      act(() => useAppState.getState().closeOverlay());
    }
  },
);

function loginFixture(
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } }),
) {
  function Destination() {
    const query = useQuery({
      queryKey: ["my-assignments"],
      queryFn: async (): Promise<string> => {
        throw new Error("B unavailable");
      },
    });
    return (
      <p>
        {typeof query.data === "string" ? query.data : "B has no cached assignments"}
      </p>
    );
  }
  const router = createMemoryRouter(
    [
      { path: "/login", element: <LoginPage /> },
      { path: "/app", element: <Destination /> },
    ],
    { initialEntries: ["/login"] },
  );
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { client, router, view };
}

async function submitLogin() {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText("Email"), studentUser.email);
  await user.type(screen.getByLabelText("Mật khẩu", { exact: true }), "password");
  await user.click(screen.getByRole("button", { name: /^Đăng nhập$/ }));
  return user;
}

it.each(["different-ID", "same-ID"] as const)(
  "clears the actual provider cache before %s replacement renders a failed B destination",
  async (kind) => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
    });
    client.setQueryData(["my-assignments"], "A private assignment");
    const next =
      kind === "same-ID"
        ? studentUser
        : { ...studentUser, id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" };
    server.use(
      http.post("http://localhost:8080/auth/login", () =>
        HttpResponse.json({ accessToken: "b", expiresIn: 900, user: next }),
      ),
    );
    const before = useAuthStore.getState().actorGeneration;
    const fixture = loginFixture(client);
    await submitLogin();
    await waitFor(() => expect(fixture.router.state.location.pathname).toBe("/app"));
    expect(screen.queryByText("A private assignment")).not.toBeInTheDocument();
    expect(screen.getByText("B has no cached assignments")).toBeInTheDocument();
    expect(client.getQueryData(["my-assignments"])).toBeUndefined();
    expect(useAuthStore.getState().actorGeneration).toBeGreaterThan(before);
    expect(drafts.clear).toHaveBeenCalledOnce();
  },
);

it("reports a failed Login cleanup Check and keeps credentials unsent until explicit same-owner recovery", async () => {
  drafts.clear
    .mockRejectedValueOnce(new Error("initial cleanup failure"))
    .mockRejectedValueOnce(new Error("retry cleanup failure"))
    .mockResolvedValue(undefined);
  server.use(
    http.post(
      "http://localhost:8080/auth/logout",
      () => new HttpResponse(null, { status: 204 }),
    ),
  );
  const departure = logoutFixture();
  await act(async () => {
    await departure.logout();
  });
  cleanup();
  loginFixture();
  const sends = vi.fn(() =>
    HttpResponse.json({ accessToken: "b", expiresIn: 900, user: studentUser }),
  );
  server.use(http.post("http://localhost:8080/auth/login", sends));
  const user = await submitLogin();
  try {
    await user.click(await screen.findByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(drafts.clear).toHaveBeenCalledTimes(2));
    expect(
      screen.getByText("Không thể dọn dữ liệu tài khoản. Thử lại."),
    ).toBeInTheDocument();
    expect(sends).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user).toBeNull();
  } finally {
    await act(async () => {
      await checkLogoutCleanup();
    });
  }
  expect(transitionStatus().kind).toBe("idle");
  expect(sends).not.toHaveBeenCalled();
});

function admitAccount(
  kind: Parameters<typeof signInAccount>[0],
  work: Parameters<typeof signInAccount>[1],
) {
  return signInAccount(kind, work, new QueryClient());
}

it("keeps replacement cleanup owned through rejection and retries without replaying credentials", async () => {
  const held = gate();
  drafts.clear
    .mockImplementationOnce(() =>
      held.promise.then(() => {
        throw new Error("cleanup refused");
      }),
    )
    .mockResolvedValue(undefined);
  const client = new QueryClient();
  client.setQueryData(["my-assignments"], "A data");
  const old = authStore.captureActor();
  const work = vi.fn(() => Promise.resolve({ accessToken: "b", user: studentUser }));
  const result = signInAccount("login", work, client).catch((error: unknown) => error);
  expect(authStore.isCurrent(old)).toBe(false);
  expect(client.getQueryData(["my-assignments"])).toBeUndefined();
  expect(work).not.toHaveBeenCalled();
  await expect(signInAccount("google", work, client)).rejects.toMatchObject({
    phase: "cleanup",
  });
  held.open();
  expect(await result).toMatchObject({ message: "cleanup refused" });
  expect(transitionStatus().kind).toBe("login");
  expect(work).not.toHaveBeenCalled();
  await checkLogoutCleanup();
  expect(drafts.clear).toHaveBeenCalledTimes(2);
  expect(transitionStatus().kind).toBe("idle");
  expect(work).not.toHaveBeenCalled();
  expect(useAuthStore.getState().user).toBeNull();
  await signInAccount("login", work, client);
  expect(work).toHaveBeenCalledOnce();
  expect(useAuthStore.getState().accessToken).toBe("b");
});

it("never replays a replacement's credentials after cleanup exceeds ten seconds", async () => {
  vi.useFakeTimers();
  const held = gate();
  drafts.clear.mockImplementationOnce(() => held.promise);
  const work = vi.fn(() => Promise.resolve({ accessToken: "b", user: studentUser }));
  const result = signInAccount("login", work, new QueryClient()).catch(
    (error: unknown) => error,
  );
  await vi.advanceTimersByTimeAsync(10_000);
  expect(await result).toMatchObject({ phase: "cleanup" });
  expect(work).not.toHaveBeenCalled();
  expect(transitionStatus().kind).toBe("login");
  held.open();
  await vi.waitFor(() => expect(transitionStatus().kind).toBe("idle"));
  expect(work).not.toHaveBeenCalled();
  expect(useAuthStore.getState().user).toBeNull();
});

it("does not delete account data for genuinely anonymous admission or ordinary rotation", async () => {
  const client = new QueryClient();
  client.setQueryData(["public-cache"], "public data");
  useAuthStore.getState().setAccessToken("rotated");
  expect(client.getQueryData(["public-cache"])).toBe("public data");
  expect(drafts.clear).not.toHaveBeenCalled();
  useAuthStore.getState().clearSession();
  await signInAccount(
    "login",
    () => Promise.resolve({ accessToken: "b", user: studentUser }),
    client,
  );
  expect(client.getQueryData(["public-cache"])).toBe("public data");
  expect(drafts.clear).not.toHaveBeenCalled();
});

it("handles Login cleanup retry timeout without overlapping cleanup or admitting B", async () => {
  const held = gate();
  drafts.clear
    .mockRejectedValueOnce(new Error("failed cleanup"))
    .mockImplementationOnce(() => held.promise);
  server.use(
    http.post(
      "http://localhost:8080/auth/logout",
      () => new HttpResponse(null, { status: 204 }),
    ),
  );
  const departure = logoutFixture();
  await act(async () => {
    await departure.logout();
  });
  cleanup();
  loginFixture();
  const sends = vi.fn(() =>
    HttpResponse.json({ accessToken: "b", expiresIn: 900, user: studentUser }),
  );
  server.use(http.post("http://localhost:8080/auth/login", sends));
  const user = await submitLogin();
  vi.useFakeTimers({ shouldAdvanceTime: true });
  await user.click(await screen.findByRole("button", { name: "Thử lại" }));
  expect(screen.getByRole("button", { name: "Kiểm tra lại" })).toBeEnabled();
  await act(async () => {
    await vi.advanceTimersByTimeAsync(10_000);
  });
  expect(
    screen.getByText("Việc dọn dữ liệu tài khoản chưa hoàn tất."),
  ).toBeInTheDocument();
  expect(drafts.clear).toHaveBeenCalledTimes(2);
  await user.click(screen.getByRole("button", { name: "Kiểm tra lại" }));
  expect(drafts.clear).toHaveBeenCalledTimes(2);
  expect(sends).not.toHaveBeenCalled();
  held.open();
  await waitFor(() => expect(transitionStatus().kind).toBe("idle"));
  expect(sends).not.toHaveBeenCalled();
  expect(useAuthStore.getState().user).toBeNull();
});

interface MemoryRequest<T> {
  result: T | undefined;
  onsuccess: (() => void) | null;
}
interface MemoryCursor {
  value: unknown;
  delete: () => void;
  continue: () => void;
}

class MemoryDraftTransaction {
  oncomplete: (() => void) | null = null;
  onabort: (() => void) | null = null;
  onerror: (() => void) | null = null;
  error = null;
  private pending = 0;
  private aborted = false;
  private completed = false;
  private stores: Map<string, Map<string, unknown>>;
  constructor(stores: Map<string, Map<string, unknown>>) {
    this.stores = stores;
    queueMicrotask(() => this.finish());
  }
  abort() {
    this.aborted = true;
    queueMicrotask(() => this.onabort?.());
  }
  objectStore(name: string) {
    return new MemoryDraftStore(this, this.stores.get(name)!);
  }
  enqueue<T>(read: () => T, request: MemoryRequest<T>) {
    this.pending += 1;
    queueMicrotask(() => {
      if (!this.aborted) {
        request.result = read();
        request.onsuccess?.();
      }
      this.pending -= 1;
      this.finish();
    });
  }
  private finish() {
    if (!this.pending && !this.aborted && !this.completed) {
      this.completed = true;
      this.oncomplete?.();
    }
  }
}

function memoryRequest<T>(transaction: MemoryDraftTransaction, read: () => T) {
  const request: MemoryRequest<T> = { result: undefined, onsuccess: null };
  transaction.enqueue(read, request);
  return request;
}

function memoryCursor(
  entry: [string, unknown],
  values: Map<string, unknown>,
  next: () => void,
): MemoryCursor {
  return {
    value: entry[1],
    delete: () => {
      values.delete(entry[0]);
    },
    continue: next,
  };
}

class MemoryDraftStore {
  private transaction: MemoryDraftTransaction;
  private values: Map<string, unknown>;
  constructor(transaction: MemoryDraftTransaction, values: Map<string, unknown>) {
    this.transaction = transaction;
    this.values = values;
  }
  get(key: unknown) {
    return memoryRequest(this.transaction, () => this.values.get(JSON.stringify(key)));
  }
  put(value: unknown, key?: unknown) {
    const record = value as { owner?: string; item?: string };
    return memoryRequest(this.transaction, () =>
      this.values.set(JSON.stringify(key ?? [record.owner, record.item]), value),
    );
  }
  clear() {
    return memoryRequest(this.transaction, () => this.values.clear());
  }
  delete(key: unknown) {
    return memoryRequest(this.transaction, () =>
      this.values.delete(JSON.stringify(key)),
    );
  }
  openCursor() {
    const entries = [...this.values.entries()];
    let index = 0;
    const cursor: MemoryRequest<MemoryCursor | null> = {
      result: undefined,
      onsuccess: null,
    };
    const next = () =>
      this.transaction.enqueue(() => {
        const entry = entries[index++];
        return entry ? memoryCursor(entry, this.values, next) : null;
      }, cursor);
    next();
    return cursor;
  }
}

function memoryDraftDatabase() {
  const stores = new Map<string, Map<string, unknown>>();
  const database = {
    createObjectStore(name: string) {
      stores.set(name, new Map());
    },
    transaction() {
      return new MemoryDraftTransaction(stores);
    },
  };
  return {
    open() {
      const request = {
        result: database,
        onupgradeneeded: null as (() => void) | null,
        onsuccess: null as (() => void) | null,
      };
      queueMicrotask(() => {
        request.onupgradeneeded?.();
        request.onsuccess?.();
      });
      return request;
    },
  };
}

it("invalidates an actual public DraftScope and clears same-ID recovery before replacement admission", async () => {
  vi.stubGlobal("indexedDB", memoryDraftDatabase());
  drafts.real = true;
  try {
    const old = await openDraftScope(studentUser.id, "group");
    await old.write("token", Date.now(), { private: "A authoring draft" });
    expect((await old.read())?.payload).toEqual({ private: "A authoring draft" });
    const foreign = await openDraftScope("foreign", "group");
    expect(await foreign.read()).toBeNull();
    await signInAccount(
      "login",
      () => Promise.resolve({ accessToken: "b", user: studentUser }),
      new QueryClient(),
    );
    await expect(old.write("late", Date.now(), { private: "late A" })).rejects.toThrow(
      "Draft transaction aborted",
    );
    const current = await openDraftScope(studentUser.id, "group");
    expect(await current.read()).toBeNull();
    await current.write("b", Date.now(), { current: "B draft" });
    expect((await current.read())?.payload).toEqual({ current: "B draft" });
  } finally {
    drafts.real = false;
    vi.unstubAllGlobals();
  }
});

it("guards late Login cleanup feedback by both actor generation and user identity", async () => {
  const held = gate();
  drafts.clear
    .mockRejectedValueOnce(new Error("failed cleanup"))
    .mockImplementationOnce(() =>
      held.promise.then(() => {
        throw new Error("late retry failure");
      }),
    )
    .mockResolvedValue(undefined);
  server.use(
    http.post(
      "http://localhost:8080/auth/logout",
      () => new HttpResponse(null, { status: 204 }),
    ),
  );
  const departure = logoutFixture();
  await act(async () => {
    await departure.logout();
  });
  cleanup();
  loginFixture();
  const user = await submitLogin();
  await user.click(await screen.findByRole("button", { name: "Thử lại" }));
  const generation = authStore.getGeneration();
  act(() => {
    useAuthStore.getState().setUser(studentUser);
  });
  expect(authStore.getGeneration()).toBe(generation);
  await act(async () => {
    held.open();
    await held.promise;
  });
  try {
    expect(
      screen.getByText("Việc dọn dữ liệu tài khoản chưa hoàn tất."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Không thể dọn dữ liệu tài khoản. Thử lại."),
    ).not.toBeInTheDocument();
  } finally {
    await act(async () => {
      held.open();
      await held.promise;
    });
    useAuthStore.setState({ user: null });
    await act(async () => {
      await checkLogoutCleanup();
    });
  }
  expect(transitionStatus().kind).toBe("idle");
});

it("names known failed departure recovery Retry", async () => {
  drafts.clear
    .mockRejectedValueOnce(new Error("known cleanup failure"))
    .mockResolvedValue(undefined);
  server.use(
    http.post(
      "http://localhost:8080/auth/logout",
      () => new HttpResponse(null, { status: 204 }),
    ),
  );
  const departure = logoutFixture();
  await act(async () => {
    await departure.logout();
  });
  cleanup();
  loginFixture();
  await submitLogin();
  try {
    expect(
      screen.getByText("Không thể dọn dữ liệu tài khoản. Thử lại."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "Kiểm tra lại" }),
    ).not.toBeInTheDocument();
  } finally {
    await act(async () => {
      await checkLogoutCleanup();
    });
  }
});
