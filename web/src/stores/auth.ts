import { create } from "zustand";
import type { components } from "@/lib/api/schema";

type User = components["schemas"]["User"];

/**
 * The access token lives **in memory only** — never localStorage, never
 * sessionStorage (§5.2). That is why this is a Zustand store rather than a
 * persisted one: a token in web storage is readable by any script on the page,
 * and the whole point of the short-lived-token-plus-httpOnly-refresh-cookie
 * design is that an XSS cannot walk away with a durable credential.
 *
 * Losing the token on reload is the intended trade-off. `/auth/refresh` mints a
 * new one from the cookie, which JavaScript cannot read.
 *
 * There is a test asserting the token is absent from both storages. Do not
 * "fix" a reload flash by persisting it.
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
