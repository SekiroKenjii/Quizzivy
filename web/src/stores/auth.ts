import { create } from "zustand";
import type { components } from "@/lib/api/schema";

type User = components["schemas"]["User"];

/**
 * Session state. The access token lives in memory only -- never localStorage or
 * sessionStorage (§5.2) -- so a reload drops it and the refresh cookie is what
 * restores the session.
 */
interface AuthState {
  accessToken: string | null;
  user: User | null;
  /** True until the first `/auth/me` settles, so guards can wait rather than bounce to /login. */
  isBootstrapping: boolean;

  setSession: (accessToken: string, user: User) => void;
  setAccessToken: (accessToken: string) => void;
  setUser: (user: User) => void;
  clearSession: () => void;
  finishBootstrap: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  user: null,
  isBootstrapping: true,

  setSession: (accessToken, user) => set({ accessToken, user, isBootstrapping: false }),
  setAccessToken: (accessToken) => set({ accessToken }),
  setUser: (user) => set({ user }),
  clearSession: () => set({ accessToken: null, user: null, isBootstrapping: false }),
  finishBootstrap: () => set({ isBootstrapping: false }),
}));

/** Non-reactive reads, for the API client — it is not a component. */
export const authStore = {
  getAccessToken: () => useAuthStore.getState().accessToken,
  setAccessToken: (t: string) => useAuthStore.getState().setAccessToken(t),
  clear: () => useAuthStore.getState().clearSession(),
};
