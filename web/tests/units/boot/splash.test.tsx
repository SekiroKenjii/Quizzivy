import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, type DataRouter } from "react-router";
import { BootSplash } from "@/app/boot/BootSplash";
import { appVersion } from "@/app/boot/version";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { studentUser } from "@tests/support/fixtures";
import "@/lib/i18n";

let releaseRoute: () => void = () => {};

function lazyRouter(): DataRouter {
  const loaded = new Promise<void>((resolve) => {
    releaseRoute = resolve;
  });
  return createMemoryRouter(
    [
      {
        path: "/app",
        lazy: async () => {
          await loaded;
          return { Component: () => <p>student home</p> };
        },
      },
    ],
    { initialEntries: ["/app"] },
  );
}

function readyRouter(): DataRouter {
  return createMemoryRouter([{ path: "/", element: <p>home</p> }]);
}

function fill(): string {
  return (document.querySelector(".qz-boot-fill") as HTMLElement).style.width;
}

beforeEach(() => {
  useAuthStore.setState({ accessToken: null, user: null, isBootstrapping: true });
  useAppState.setState({
    bootPhase: "booting",
    bootError: null,
    bootAttempt: 0,
    overlay: { kind: "none" },
  });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the boot splash", () => {
  it("checks the session first, at 12%", () => {
    render(<BootSplash router={readyRouter()} />);
    expect(screen.getByRole("status")).toHaveTextContent(
      "Đang kiểm tra phiên đăng nhập…",
    );
    expect(fill()).toBe("12%");
  });

  it("then loads the route's code, at 46%", () => {
    useAuthStore.setState({ user: studentUser, isBootstrapping: false });
    render(<BootSplash router={lazyRouter()} />);
    expect(screen.getByRole("status")).toHaveTextContent("Đang tải lớp học của bạn…");
    expect(fill()).toBe("46%");
  });

  it("hands over and leaves once the route has loaded", async () => {
    render(<BootSplash router={lazyRouter()} />);
    act(() => useAuthStore.setState({ user: studentUser, isBootstrapping: false }));
    await act(async () => releaseRoute());

    await waitFor(() => expect(fill()).toBe("100%"));
    expect(screen.getByText("Sắp xong rồi…")).toBeInTheDocument();
    await waitFor(() => expect(document.querySelector(".qz-boot")).toBeNull());
  });

  it("does not move the bar back when a step repeats", () => {
    useAuthStore.setState({ user: studentUser, isBootstrapping: false });
    render(<BootSplash router={lazyRouter()} />);
    act(() => useAuthStore.setState({ isBootstrapping: true }));
    expect(fill()).toBe("46%");
  });

  it("turns slow when no step is done for eight seconds", () => {
    vi.useFakeTimers();
    render(<BootSplash router={readyRouter()} />);
    act(() => vi.advanceTimersByTime(7_999));
    expect(screen.queryByText("Tải lâu hơn bình thường")).not.toBeInTheDocument();

    act(() => vi.advanceTimersByTime(1));
    expect(screen.getByText("Tải lâu hơn bình thường")).toBeInTheDocument();
    expect(
      screen.getByText("Kết nối có thể đang chậm. Hãy giữ trang này hoặc tải lại."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tải lại" })).toBeInTheDocument();
    expect(document.querySelector(".qz-indet")).not.toBeNull();
  });

  it("starts the slow clock again when a step is done", () => {
    vi.useFakeTimers();
    render(<BootSplash router={lazyRouter()} />);
    act(() => vi.advanceTimersByTime(6_000));
    act(() => useAuthStore.setState({ user: studentUser, isBootstrapping: false }));
    act(() => vi.advanceTimersByTime(6_000));
    expect(screen.queryByText("Tải lâu hơn bình thường")).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(2_000));
    expect(screen.getByText("Tải lâu hơn bình thường")).toBeInTheDocument();
  });

  it("counts down from ten when offline and retries at zero", () => {
    vi.useFakeTimers();
    useAppState.setState({ bootPhase: "offline" });
    render(<BootSplash router={readyRouter()} />);

    expect(screen.getByText("Mất kết nối")).toBeInTheDocument();
    expect(screen.getByText("Không kết nối được Quizzivy")).toBeInTheDocument();
    expect(screen.getByText("Tự thử lại sau 10s")).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(1_000));
    expect(screen.getByText("Tự thử lại sau 9s")).toBeInTheDocument();

    act(() => vi.advanceTimersByTime(9_000));
    expect(useAppState.getState().bootAttempt).toBe(1);
    expect(useAppState.getState().bootPhase).toBe("booting");
    expect(screen.getByText("Đang kiểm tra phiên đăng nhập…")).toBeInTheDocument();
  });

  it("retries at once when the browser comes back online, or when asked", () => {
    useAppState.setState({ bootPhase: "offline" });
    render(<BootSplash router={readyRouter()} />);
    act(() => window.dispatchEvent(new Event("online")));
    expect(useAppState.getState().bootAttempt).toBe(1);

    act(() => useAppState.setState({ bootPhase: "offline" }));
    act(() => screen.getByRole("button", { name: "Thử lại" }).click());
    expect(useAppState.getState().bootAttempt).toBe(2);
  });

  it("gives way to an overlay", () => {
    useAppState.setState({
      overlay: {
        kind: "maintenance",
        window: { startsAt: "2026-10-01T15:00:00Z", endsAt: "2026-10-01T16:00:00Z" },
      },
    });
    render(<BootSplash router={readyRouter()} />);
    expect(document.querySelector(".qz-boot")).toBeNull();
  });

  it("leaves when boot fails, so the error page shows", async () => {
    render(<BootSplash router={readyRouter()} />);
    act(() =>
      useAppState.setState({ bootPhase: "failed", bootError: new Error("boom") }),
    );
    await waitFor(() => expect(document.querySelector(".qz-boot")).toBeNull());
  });

  it("shows the app version and none of the prototype's chrome", () => {
    render(<BootSplash router={readyRouter()} />);
    expect(screen.getByText(`Quizzivy · v${appVersion}`)).toBeInTheDocument();
    expect(screen.queryByText(/v2\.8\.1/)).not.toBeInTheDocument();
    for (const name of [
      "Loading",
      "Hand-off",
      "Slow",
      "Offline",
      "Update",
      "Session expired",
      "Toggle theme",
    ]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
    expect(screen.queryAllByRole("button")).toEqual([]);
  });
});
