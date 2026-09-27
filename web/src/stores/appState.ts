import { create } from "zustand";

/**
 * BootPhase is where restoring the session stands. `offline` means the API
 * could not be reached, which says nothing about the session; `failed` means
 * it answered with something unexpected, carried in `bootError`.
 */
export type BootPhase = "booting" | "ready" | "offline" | "failed";

/** MaintenanceWindow is the period a 503 `MAINTENANCE` names, as ISO timestamps. */
export type MaintenanceWindow = { startsAt: string; endsAt: string };

/** Overlay is what covers the mounted route, if anything. */
export type Overlay =
  | { kind: "none" }
  | { kind: "maintenance"; window: MaintenanceWindow }
  | { kind: "expired" }
  | { kind: "update" };

interface AppState {
  bootPhase: BootPhase;
  bootError: Error | null;
  bootAttempt: number;
  overlay: Overlay;
  updateReady: boolean;

  setBootPhase: (phase: BootPhase, error?: Error | null) => void;
  retryBoot: () => void;
  showOverlay: (overlay: Overlay) => void;
  closeOverlay: () => void;
  setUpdateReady: () => void;
}

const rank: Record<Overlay["kind"], number> = {
  none: 0,
  update: 1,
  expired: 2,
  maintenance: 3,
};

function sameOverlay(a: Overlay, b: Overlay): boolean {
  if (a.kind === "maintenance" && b.kind === "maintenance") {
    return (
      a.window.startsAt === b.window.startsAt && a.window.endsAt === b.window.endsAt
    );
  }
  return a.kind === b.kind;
}

/**
 * useAppState holds the app-wide states that are not the session: how boot
 * went, which overlay, if any, stands over the page, and whether a newer build
 * is waiting. `retryBoot` runs the session restore again. `showOverlay` never
 * lets a lesser overlay replace a greater one -- maintenance, then an expired
 * session, then an update -- and ignores the overlay already showing.
 */
export const useAppState = create<AppState>((set) => ({
  bootPhase: "booting",
  bootError: null,
  bootAttempt: 0,
  overlay: { kind: "none" },
  updateReady: false,

  setBootPhase: (bootPhase, bootError = null) => set({ bootPhase, bootError }),
  retryBoot: () =>
    set((state) => ({
      bootPhase: "booting",
      bootError: null,
      bootAttempt: state.bootAttempt + 1,
    })),
  showOverlay: (overlay) =>
    set((state) =>
      rank[overlay.kind] < rank[state.overlay.kind] ||
      sameOverlay(overlay, state.overlay)
        ? state
        : { overlay },
    ),
  closeOverlay: () => set({ overlay: { kind: "none" } }),
  setUpdateReady: () => set({ updateReady: true }),
}));
