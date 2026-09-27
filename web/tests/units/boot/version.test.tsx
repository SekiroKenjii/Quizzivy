import { beforeEach, describe, expect, it } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createMemoryRouter, type DataRouter } from "react-router";
import { http, HttpResponse } from "msw";
import { appBuild, fetchLatestBuild, useVersionWatch } from "@/app/boot/version";
import { useAppState } from "@/stores/appState";
import { server } from "@tests/support/server";

function published(build: string) {
  server.use(
    http.get("*/version.json", () => HttpResponse.json({ build, version: "0.7.0" })),
  );
}

function watched(): DataRouter {
  const router = createMemoryRouter(
    [
      { path: "/app", element: null },
      { path: "/app/classes", element: null },
      { path: "/app/attempts/:id", handle: { focus: true }, element: null },
    ],
    { initialEntries: ["/app"] },
  );
  renderHook(() => useVersionWatch(router));
  return router;
}

function becomeVisible() {
  act(() => {
    document.dispatchEvent(new Event("visibilitychange"));
  });
}

beforeEach(() => {
  useAppState.setState({ overlay: { kind: "none" }, updateReady: false });
});

describe("reading /version.json", () => {
  it("returns the build it names", async () => {
    published("abc123");
    expect(await fetchLatestBuild()).toBe("abc123");
  });

  it("ignores the SPA fallback's HTML where no file was published", async () => {
    server.use(
      http.get(
        "*/version.json",
        () =>
          new HttpResponse("<!doctype html>", {
            headers: { "Content-Type": "text/html" },
          }),
      ),
    );
    expect(await fetchLatestBuild()).toBeNull();
  });

  it("ignores a network failure", async () => {
    server.use(http.get("*/version.json", () => HttpResponse.error()));
    expect(await fetchLatestBuild()).toBeNull();
  });
});

describe("watching for a newer deploy", () => {
  it("offers the update at the next navigation, never on a focus route", async () => {
    published("a-newer-build");
    const router = watched();
    becomeVisible();
    await waitFor(() => expect(useAppState.getState().updateReady).toBe(true));
    expect(useAppState.getState().overlay.kind).toBe("none");

    await act(() => router.navigate("/app/attempts/a1"));
    expect(useAppState.getState().overlay.kind).toBe("none");

    await act(() => router.navigate("/app/classes"));
    expect(useAppState.getState().overlay.kind).toBe("update");
  });

  it("stays quiet when the published build is this one", async () => {
    let asked = false;
    server.use(
      http.get("*/version.json", () => {
        asked = true;
        return HttpResponse.json({ build: appBuild, version: "0.7.0" });
      }),
    );
    const router = watched();
    becomeVisible();
    await waitFor(() => expect(asked).toBe(true));
    await act(() => router.navigate("/app/classes"));

    expect(useAppState.getState().updateReady).toBe(false);
    expect(useAppState.getState().overlay.kind).toBe("none");
  });

  it("offers it at once when a route's code fails to load, even on a focus route", async () => {
    const router = watched();
    await act(() => router.navigate("/app/attempts/a1"));
    act(() => {
      window.dispatchEvent(new Event("vite:preloadError"));
    });
    expect(useAppState.getState().overlay.kind).toBe("update");
  });
});
