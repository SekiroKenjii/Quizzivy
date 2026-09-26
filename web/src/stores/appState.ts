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

  setBootPhase: (phase: BootPhase, error?: Error | null) => void;
  retryBoot: () => void;
  showOverlay: (overlay: Overlay) => void;
  closeOverlay: () => void;
}

/**
 * useAppState holds the app-wide states that are not the session: how boot
 * went, and which overlay, if any, stands over the page. `retryBoot` runs the
 * session restore again.
 */
export const useAppState = create<AppState>((set) => ({
  bootPhase: "booting",
  bootError: null,
  bootAttempt: 0,
  overlay: { kind: "none" },

  setBootPhase: (bootPhase, bootError = null) => set({ bootPhase, bootError }),
  retryBoot: () =>
    set((state) => ({
      bootPhase: "booting",
      bootError: null,
      bootAttempt: state.bootAttempt + 1,
    })),
  showOverlay: (overlay) => set({ overlay }),
  closeOverlay: () => set({ overlay: { kind: "none" } }),
}));
