import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, type DataRouter } from "react-router";
import { http } from "msw";
import { AppStateLayer } from "@/app/boot/AppStateLayer";
import { queryClient } from "@/app/queryClient";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { studentUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const WINDOW = { startsAt: "2026-10-01T15:00:00Z", endsAt: "2026-10-01T16:30:00Z" };

function renderAt(path = "/app/attempts/a1"): {
  router: DataRouter;
  page: HTMLElement;
} {
  const router = createMemoryRouter(
    [
      { path: "/app/attempts/:id", element: <p>engine</p> },
      { path: "/login", element: <p>login page</p> },
    ],
    { initialEntries: [path] },
  );
  const { container } = render(
    <>
      <button type="button">on the page</button>
      <AppStateLayer router={router} />
    </>,
  );
  return { router, page: container };
}

function status(maintenance: unknown) {
  server.use(
    http.get(`${BASE}/public/status`, () =>
      contractJson("/public/status", "get", 200, { maintenance }),
    ),
  );
}

beforeEach(() => {
  useAuthStore.setState({
    accessToken: "token",
    user: studentUser,
    isBootstrapping: false,
    expired: false,
  });
  useAppState.setState({
    bootPhase: "ready",
    bootError: null,
    bootAttempt: 0,
    overlay: { kind: "none" },
    updateReady: false,
  });
});

afterEach(() => {
  act(() => useAppState.setState({ overlay: { kind: "none" } }));
  vi.restoreAllMocks();
});

describe("the maintenance overlay", () => {
  it("stands over the page, which stays mounted but inert, and takes focus", () => {
    const { page } = renderAt();
    act(() =>
      useAppState.getState().showOverlay({ kind: "maintenance", window: WINDOW }),
    );

    const dialog = screen.getByRole("dialog", { name: "Quizzivy đang được cập nhật" });
    expect(dialog).toBeInTheDocument();
    expect(page).toHaveAttribute("inert");
    expect(page).toHaveTextContent("on the page");
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Kiểm tra lại" }),
    );
  });

  it("closes when no window is under way, refetches, and gives the page back", async () => {
    status(null);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { page } = renderAt();
    act(() =>
      useAppState.getState().showOverlay({ kind: "maintenance", window: WINDOW }),
    );

    screen.getByRole("button", { name: "Kiểm tra lại" }).click();

    await waitFor(() => expect(useAppState.getState().overlay.kind).toBe("none"));
    expect(invalidate).toHaveBeenCalled();
    expect(page).not.toHaveAttribute("inert");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("treats a window that has not started as no window", async () => {
    status({ ...WINDOW, active: false });
    renderAt();
    act(() =>
      useAppState.getState().showOverlay({ kind: "maintenance", window: WINDOW }),
    );
    screen.getByRole("button", { name: "Kiểm tra lại" }).click();
    await waitFor(() => expect(useAppState.getState().overlay.kind).toBe("none"));
  });

  it("stays while the window is under way, with the window the server names", async () => {
    const later = { startsAt: WINDOW.startsAt, endsAt: "2026-10-01T17:00:00Z" };
    status({ ...later, active: true });
    renderAt();
    act(() =>
      useAppState.getState().showOverlay({ kind: "maintenance", window: WINDOW }),
    );
    screen.getByRole("button", { name: "Kiểm tra lại" }).click();

    await waitFor(() =>
      expect(useAppState.getState().overlay).toEqual({
        kind: "maintenance",
        window: later,
      }),
    );
  });

  it("restores the session when maintenance began before it was restored", async () => {
    status(null);
    useAppState.setState({ bootPhase: "booting" });
    renderAt();
    act(() =>
      useAppState.getState().showOverlay({ kind: "maintenance", window: WINDOW }),
    );
    screen.getByRole("button", { name: "Kiểm tra lại" }).click();

    await waitFor(() => expect(useAppState.getState().bootAttempt).toBe(1));
  });

  it("hands over to the sign-in overlay when the session ran out underneath", async () => {
    status(null);
    renderAt();
    act(() =>
      useAppState.getState().showOverlay({ kind: "maintenance", window: WINDOW }),
    );
    act(() => useAuthStore.getState().expireSession());
    screen.getByRole("button", { name: "Kiểm tra lại" }).click();

    await waitFor(() => expect(useAppState.getState().overlay.kind).toBe("expired"));
  });
});

describe("the sign-in-again overlay", () => {
  it("keeps the page mounted until the user chooses to sign in", () => {
    renderAt();
    act(() => useAuthStore.getState().expireSession());
    act(() => useAppState.getState().showOverlay({ kind: "expired" }));

    expect(screen.getByText("Vui lòng đăng nhập lại")).toBeInTheDocument();
    expect(
      screen.getByText("Bạn đã được đăng xuất để bảo mật tài khoản."),
    ).toBeInTheDocument();
    expect(useAuthStore.getState().user).toEqual(studentUser);
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Đăng nhập" }),
    );
  });

  it("drops the session and goes to sign in, coming back here after", async () => {
    const clear = vi.spyOn(queryClient, "clear");
    const { router } = renderAt("/app/attempts/a1");
    act(() => useAuthStore.getState().expireSession());
    act(() => useAppState.getState().showOverlay({ kind: "expired" }));

    act(() => screen.getByRole("button", { name: "Đăng nhập" }).click());

    expect(useAuthStore.getState().user).toBeNull();
    expect(useAuthStore.getState().expired).toBe(false);
    expect(clear).toHaveBeenCalled();
    await waitFor(() => expect(router.state.location.pathname).toBe("/login"));
    expect(router.state.location.search).toBe(
      `?next=${encodeURIComponent("/app/attempts/a1")}`,
    );
  });
});

describe("the update card", () => {
  it("offers the new version", () => {
    renderAt();
    act(() => useAppState.getState().showOverlay({ kind: "update" }));
    expect(screen.getByText("Phiên bản mới")).toBeInTheDocument();
    expect(screen.getByText("Quizzivy vừa được cập nhật")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tải lại ngay" })).toBeInTheDocument();
  });
});

describe("which overlay wins", () => {
  it("never lets a lesser overlay replace a greater one", () => {
    const { showOverlay } = useAppState.getState();
    act(() => showOverlay({ kind: "expired" }));
    act(() => showOverlay({ kind: "update" }));
    expect(useAppState.getState().overlay.kind).toBe("expired");

    act(() => showOverlay({ kind: "maintenance", window: WINDOW }));
    act(() => showOverlay({ kind: "expired" }));
    expect(useAppState.getState().overlay.kind).toBe("maintenance");
  });

  it("keeps the same overlay when a retry names the same window again", () => {
    const { showOverlay } = useAppState.getState();
    act(() => showOverlay({ kind: "maintenance", window: WINDOW }));
    const shown = useAppState.getState().overlay;
    act(() => showOverlay({ kind: "maintenance", window: { ...WINDOW } }));
    expect(useAppState.getState().overlay).toBe(shown);
  });
});
