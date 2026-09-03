import { afterEach, describe, expect, it } from "vitest";
import { useAuthStore } from "@/stores/auth";
import { studentUser } from "@tests/support/fixtures";

/**
 * §5.2: the access token lives in memory and nowhere else. A token in web
 * storage is readable by any script on the page, which defeats the entire
 * short-token-plus-httpOnly-cookie design.
 */
describe("the auth store", () => {
  afterEach(() => {
    useAuthStore.getState().clearSession();
    localStorage.clear();
    sessionStorage.clear();
  });

  it("keeps the access token out of localStorage and sessionStorage", () => {
    const token = "a-very-recognisable-access-token";
    useAuthStore.getState().setSession(token, studentUser);

    expect(useAuthStore.getState().accessToken).toBe(token);

    // Scan every key rather than guessing the one a future refactor might use.
    for (const storage of [localStorage, sessionStorage]) {
      const dump = Object.keys(storage)
        .map((key) => `${key}=${storage.getItem(key) ?? ""}`)
        .join("|");
      expect(dump).not.toContain(token);
    }
  });

  it("finishes bootstrapping once a session is set", () => {
    useAuthStore.setState({ isBootstrapping: true });

    useAuthStore.getState().setSession("t", studentUser);
    expect(useAuthStore.getState().isBootstrapping).toBe(false);
  });

  it("forgets the user and the token together", () => {
    useAuthStore.getState().setSession("t", studentUser);
    useAuthStore.getState().clearSession();

    const state = useAuthStore.getState();
    expect(state.accessToken).toBeNull();
    expect(state.user).toBeNull();
    expect(state.isBootstrapping).toBe(false);
  });
});
