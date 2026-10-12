import { create } from "zustand";
import type { components } from "@/lib/api/schema";

type User = components["schemas"]["CurrentUser"];

/**
 * Session state. The access token lives in memory only -- never localStorage or
 * sessionStorage (§5.2) -- so a reload drops it and the refresh cookie is what
 * restores the session.
 */
interface AuthState {
  actorGeneration: number;
  accessToken: string | null;
  user: User | null;
  /** True until the first `/auth/me` settles, so guards can wait rather than bounce to /login. */
  isBootstrapping: boolean;
  /**
   * True once a signed-in user's session was refused. The user stays, so the
   * page under the "sign in again" overlay stays mounted, but the token is gone.
   */
  expired: boolean;
  /**
   * True from the user's own sign-out until the next sign-in. The guard then
   * sends a visitor to the sign-in page without `next`.
   */
  signedOut: boolean;

  setSession: (accessToken: string, user: User) => void;
  setAccessToken: (accessToken: string) => void;
  setUser: (user: User) => void;
  clearSession: () => void;
  signOut: () => void;
  expireSession: () => void;
  finishBootstrap: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  actorGeneration: 0,
  accessToken: null,
  user: null,
  isBootstrapping: true,
  expired: false,
  signedOut: false,

  setSession: (accessToken, user) =>
    set((state) => ({
      actorGeneration: state.actorGeneration + 1,
      accessToken,
      user,
      isBootstrapping: false,
      expired: false,
      signedOut: false,
    })),
  setAccessToken: (accessToken) => set({ accessToken }),
  setUser: (user) =>
    set((state) => (state.user && state.user.id !== user.id ? state : { user })),
  clearSession: () =>
    set((state) => ({
      actorGeneration: state.actorGeneration + 1,
      accessToken: null,
      user: null,
      isBootstrapping: false,
      expired: false,
    })),
  signOut: () =>
    set((state) => ({
      actorGeneration: state.actorGeneration + 1,
      accessToken: null,
      user: null,
      isBootstrapping: false,
      expired: false,
      signedOut: true,
    })),
  expireSession: () => set({ accessToken: null, expired: true }),
  finishBootstrap: () => set({ isBootstrapping: false }),
}));

/** ActorLease identifies one admission independently of access-token rotation. */
export interface ActorLease {
  generation: number;
  userId: string | null;
}

/** authStore provides non-reactive session reads and actor guards. */
export const authStore = {
  getGeneration: () => useAuthStore.getState().actorGeneration,
  captureActor: (): ActorLease => ({
    generation: useAuthStore.getState().actorGeneration,
    userId: useAuthStore.getState().user?.id ?? null,
  }),
  isCurrent: (lease: ActorLease) =>
    useAuthStore.getState().actorGeneration === lease.generation &&
    (useAuthStore.getState().user?.id ?? null) === lease.userId,
  getAccessToken: () => useAuthStore.getState().accessToken,
  setAccessToken: (t: string) => useAuthStore.getState().setAccessToken(t),
  clear: () => useAuthStore.getState().clearSession(),
  refuseAnonymous: () =>
    useAuthStore.setState({
      accessToken: null,
      isBootstrapping: false,
      expired: false,
    }),
  isSignedIn: () => useAuthStore.getState().user !== null,
  isExpired: () => useAuthStore.getState().expired,
  expire: () => useAuthStore.getState().expireSession(),
};
