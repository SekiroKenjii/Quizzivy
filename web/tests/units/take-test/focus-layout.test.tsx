import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, renderHook, screen } from "@testing-library/react";
import { createMemoryRouter, matchRoutes, RouterProvider } from "react-router";
import { isFocusRoute, useVersionWatch } from "@/app/boot/version";
import { router as appRouter } from "@/app/router";
import FocusLayout from "@/layouts/FocusLayout";
import { useResolvedTheme, writeThemePreference } from "@/lib/theme";
import { useAppState } from "@/stores/appState";
import "@/lib/i18n";

function mount() {
  const router = createMemoryRouter(
    [
      {
        path: "/app/attempts/:attemptId",
        element: <FocusLayout />,
        children: [{ index: true, element: <button type="button">inside</button> }],
      },
    ],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  localStorage.clear();
  act(() => writeThemePreference("light"));
  useAppState.setState({ overlay: { kind: "none" }, updateReady: false });
});

afterEach(() => {
  act(() => writeThemePreference("light"));
  localStorage.clear();
  useAppState.setState({ overlay: { kind: "none" }, updateReady: false });
});

describe("FocusLayout", () => {
  it("is a deck surface that fills the viewport and keeps the 44px floor for what is not rebuilt", () => {
    const { container } = mount();
    const root = container.firstElementChild;
    expect(root).toHaveAttribute("data-scale", "deck");
    expect(root).toHaveClass(
      "student-surface",
      "@container/student",
      "bg-bg",
      "text-fg",
      "h-svh",
      "flex-col",
    );
    expect(root).toContainElement(screen.getByRole("button", { name: "inside" }));
  });

  it("follows a dark preference instead of forcing the page light", () => {
    const theme = renderHook(() => useResolvedTheme());
    act(() => writeThemePreference("dark"));
    mount();
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(theme.result.current).toBe("dark");
  });

  it("follows a light preference", () => {
    const theme = renderHook(() => useResolvedTheme());
    mount();
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    expect(theme.result.current).toBe("light");
  });
});

describe("the take-test route", () => {
  const matchesOf = (path: string) => matchRoutes(appRouter.routes, path) ?? [];

  it("declares handle.focus on the layout route above the engine", () => {
    const matches = matchesOf("/app/attempts/018f0000-0000-7000-8000-0000000000e1");
    expect(matches.at(-1)?.route.index).toBe(true);
    expect(matches.at(-2)?.route.path).toBe("app/attempts/:attemptId");
    expect(matches.at(-2)?.route.handle).toEqual({ focus: true });
    expect(isFocusRoute(matches)).toBe(true);
  });

  it("is the only student route that does", () => {
    for (const path of [
      "/app",
      "/app/classes",
      "/app/settings",
      "/app/assignments/018f0000-0000-7000-8000-0000000000d1",
      "/app/attempts/018f0000-0000-7000-8000-0000000000e1/result",
    ]) {
      expect(matchesOf(path).length, path).toBeGreaterThan(0);
      expect(isFocusRoute(matchesOf(path)), path).toBe(false);
    }
  });

  it("keeps the update prompt out of the engine and shows it on the way out", async () => {
    const focus = matchesOf("/app/attempts/a1").at(-2)?.route;
    const router = createMemoryRouter(
      [
        { path: "/app", element: null },
        { path: "/app/attempts/:attemptId", handle: focus?.handle, element: null },
      ],
      { initialEntries: ["/app"] },
    );
    renderHook(() => useVersionWatch(router));
    act(() => useAppState.getState().setUpdateReady());

    await act(() => router.navigate("/app/attempts/a1"));
    expect(useAppState.getState().overlay.kind).toBe("none");

    await act(() => router.navigate("/app"));
    expect(useAppState.getState().overlay.kind).toBe("update");
  });
});
