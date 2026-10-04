import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider, type DataRouter } from "react-router";
import { http } from "msw";
import { AppStateLayer } from "@/app/boot/AppStateLayer";
import { queryClient } from "@/app/queryClient";
import { pending as pendingEvents } from "@/features/integrity/buffer";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import { useTakeTestStore } from "@/features/take-test/store";
import FocusLayout from "@/layouts/FocusLayout";
import { setMaintenanceHandler } from "@/lib/api/client";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { studentUser } from "@tests/support/fixtures";
import { viewport } from "@tests/support/viewport";
import {
  DECK_ATTEMPT,
  deckSaved,
  deckSession,
} from "@tests/units/take-test/deckSession";
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

describe.each(["desktop", "phone"] as const)("over the engine on a %s", (width) => {
  const MINUTE = 60_000;
  const DRAFT = `quizzivy.answer-draft.${DECK_ATTEMPT}`;
  const FIRST = "018f0000-0000-7000-8000-00000000b001";
  const FIFTH = "018f0000-0000-7000-8000-00000000b005";
  const store = () => useTakeTestStore.getState();
  const draft = () =>
    (
      JSON.parse(localStorage.getItem(DRAFT) ?? '{"answers":{}}') as {
        answers: unknown;
      }
    ).answers;

  function engine(left: number) {
    const opened = new Date();
    server.use(
      http.get(`${BASE}/app/attempts/:id`, () =>
        contractJson("/app/attempts/{id}", "get", 200, deckSession(opened, left)),
      ),
    );
    const router = createMemoryRouter(
      [
        {
          path: "/app/attempts/:attemptId",
          handle: { focus: true },
          element: <FocusLayout />,
          children: [{ index: true, element: <TakeTestPage /> }],
        },
        { path: "/login", element: <p>login page</p> },
      ],
      { initialEntries: [`/app/attempts/${DECK_ATTEMPT}`] },
    );
    const { container } = render(
      <>
        <RouterProvider router={router} />
        <AppStateLayer router={router} />
      </>,
    );
    return { page: container, deadline: opened.getTime() + left };
  }

  function saves(answer: () => Response) {
    server.use(http.patch(`${BASE}/app/attempts/:id/answers`, answer));
  }

  function leaveAndReturn() {
    act(() => {
      window.dispatchEvent(new Event("blur"));
      window.dispatchEvent(new Event("focus"));
      document.dispatchEvent(new Event("visibilitychange"));
    });
  }

  beforeEach(() => {
    viewport(width);
    localStorage.clear();
    sessionStorage.clear();
    store().reset();
    setMaintenanceHandler((window) => {
      useAppState.getState().showOverlay({ kind: "maintenance", window });
    });
  });

  afterEach(() => {
    store().reset();
    vi.unstubAllGlobals();
  });

  it("keeps the engine mounted under maintenance, drafting every change, and adopts the later deadline after it", async () => {
    saves(() =>
      contractJson("/app/attempts/{id}/answers", "patch", 503, {
        error: {
          code: "MAINTENANCE",
          message: "Hệ thống đang bảo trì.",
          details: WINDOW,
          requestId: "018f0000-0000-7000-8000-00000000ee01",
        },
      }),
    );
    const user = userEvent.setup();
    const { page, deadline } = engine(4 * MINUTE);
    await user.click(
      await screen.findByRole("radio", {
        name: /To compare parks in European and Asian cities/,
      }),
    );
    expect(screen.getByRole("timer")).toHaveClass("bg-danger-soft");

    await act(async () => {
      await store().flush();
    });
    expect(
      await screen.findByRole("dialog", { name: "Quizzivy đang được cập nhật" }),
    ).toBeInTheDocument();
    expect(page).toHaveAttribute("inert");
    expect(
      within(page).getByRole("timer", { hidden: true, name: "Thời gian còn lại" }),
    ).toBeInTheDocument();
    expect(store().dirty.has(FIRST)).toBe(true);
    expect(draft()).toEqual({
      [FIRST]: {
        type: "choice",
        optionIds: ["018f0000-0000-7000-8000-00000000c101"],
      },
    });

    act(() => store().setAnswer(FIFTH, { type: "text", value: "public green space" }));
    expect(draft()).toMatchObject({
      [FIFTH]: { type: "text", value: "public green space" },
    });

    const recorded = pendingEvents().length;
    leaveAndReturn();
    expect(pendingEvents()).toHaveLength(recorded);
    expect(screen.queryByRole("alertdialog")).toBeNull();

    const later = new Date(deadline + 10 * MINUTE).toISOString();
    saves(() =>
      contractJson(
        "/app/attempts/{id}/answers",
        "patch",
        200,
        deckSaved(new Date(), later),
      ),
    );
    status(null);
    screen.getByRole("button", { name: "Kiểm tra lại" }).click();
    await waitFor(() => expect(useAppState.getState().overlay.kind).toBe("none"));
    expect(page).not.toHaveAttribute("inert");

    await act(async () => {
      await store().flush();
    });
    expect(store().deadlineAt).toBe(Date.parse(later));
    expect(store().dirty.size).toBe(0);
    expect(localStorage.getItem(DRAFT)).toBeNull();
    expect(screen.getByRole("timer")).toHaveTextContent(/^13:\d\d$/);
    expect(screen.getByRole("timer")).toHaveClass("bg-muted");
    expect(screen.getByRole("timer")).not.toHaveClass("bg-danger-soft");
  });

  it("keeps the engine mounted under the sign-in overlay, drafting every change and counting no departure", async () => {
    const { page } = engine(38 * MINUTE);
    await screen.findByRole("radio", {
      name: /To compare parks in European and Asian cities/,
    });

    act(() => useAuthStore.getState().expireSession());
    act(() => useAppState.getState().showOverlay({ kind: "expired" }));
    expect(screen.getByText("Vui lòng đăng nhập lại")).toBeInTheDocument();
    expect(page).toHaveAttribute("inert");
    expect(
      within(page).getByRole("timer", { hidden: true, name: "Thời gian còn lại" }),
    ).toBeInTheDocument();
    expect(
      within(page).getByRole("button", { hidden: true, name: "Thoát khỏi bài làm" }),
    ).toBeInTheDocument();

    act(() => store().setAnswer(FIFTH, { type: "text", value: "green space" }));
    expect(draft()).toEqual({ [FIFTH]: { type: "text", value: "green space" } });
    act(() => store().setAnswer(FIFTH, { type: "text", value: "public green space" }));
    expect(draft()).toEqual({ [FIFTH]: { type: "text", value: "public green space" } });

    leaveAndReturn();
    expect(pendingEvents()).toEqual([]);
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("counts a departure again once the overlay is gone", async () => {
    engine(38 * MINUTE);
    await screen.findByRole("radio", {
      name: /To compare parks in European and Asian cities/,
    });
    act(() => useAppState.getState().showOverlay({ kind: "expired" }));
    act(() => useAppState.getState().closeOverlay());

    leaveAndReturn();
    expect(pendingEvents().map((event) => event.kind)).toEqual([
      "window_blur",
      "window_focus",
      "tab_visible",
    ]);
  });
});
