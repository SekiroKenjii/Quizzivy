import { useEffect } from "react";
import type { DataRouter } from "react-router";
import { useAppState } from "@/stores/appState";
import { isActive } from "./activity";

const CHECK_EVERY_MS = 10 * 60_000;
const TICK_MS = 60_000;

/** appBuild is the commit this bundle was built from. */
export const appBuild: string = __APP_BUILD__;

/** appVersion is web/package.json's version, which the splash shows. */
export const appVersion: string = __APP_VERSION__;

/**
 * fetchLatestBuild reads `/version.json` past every cache and returns the build
 * it names, or null when the answer is not that file: a network failure, or
 * the SPA fallback's HTML where no file was published.
 */
export async function fetchLatestBuild(): Promise<string | null> {
  try {
    const response = await fetch("/version.json", { cache: "no-store" });
    const type = response.headers.get("Content-Type") ?? "";
    if (!response.ok || !type.includes("application/json")) return null;
    const body = (await response.json()) as { build?: unknown };
    return typeof body.build === "string" ? body.build : null;
  } catch {
    return null;
  }
}

/** isFocusRoute reports whether a route in matches declares `handle.focus`. */
export function isFocusRoute(
  matches: readonly { route: { handle?: unknown } }[],
): boolean {
  return matches.some((match) => {
    const handle = match.route.handle;
    return (
      typeof handle === "object" &&
      handle !== null &&
      (handle as { focus?: unknown }).focus === true
    );
  });
}

/**
 * useVersionWatch looks for a newer deploy when the tab becomes visible and
 * every ten minutes while the user is active. A newer build shows the update
 * card at the next navigation that does not land on a focus route; a lazy
 * chunk that fails to load shows it at once, since that route cannot load.
 */
export function useVersionWatch(router: DataRouter): void {
  useEffect(() => {
    let lastCheck = Date.now();
    let checking = false;

    const check = async () => {
      if (checking || useAppState.getState().updateReady) return;
      checking = true;
      lastCheck = Date.now();
      const latest = await fetchLatestBuild();
      checking = false;
      if (latest !== null && latest !== appBuild)
        useAppState.getState().setUpdateReady();
    };

    const onVisibility = () => {
      if (document.visibilityState === "visible") void check();
    };
    const tick = window.setInterval(() => {
      if (isActive() && Date.now() - lastCheck >= CHECK_EVERY_MS) void check();
    }, TICK_MS);
    const onPreloadError = () => {
      const state = useAppState.getState();
      state.setUpdateReady();
      state.showOverlay({ kind: "update" });
    };

    let location = router.state.location.key;
    const unsubscribe = router.subscribe((state) => {
      if (state.location.key === location) return;
      location = state.location.key;
      if (useAppState.getState().updateReady && !isFocusRoute(state.matches)) {
        useAppState.getState().showOverlay({ kind: "update" });
      }
    });

    document.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("vite:preloadError", onPreloadError);
    return () => {
      unsubscribe();
      window.clearInterval(tick);
      document.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("vite:preloadError", onPreloadError);
    };
  }, [router]);
}
