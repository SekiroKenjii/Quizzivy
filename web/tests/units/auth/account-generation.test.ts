import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import "@/lib/i18n";
import { useGoogleSignIn } from "@/features/auth/google/useGoogleSignIn";
import { authStore, useAuthStore } from "@/stores/auth";
import { api } from "@/lib/api/client";
import { studentUser } from "@tests/support/fixtures";

afterEach(() => useAuthStore.getState().clearSession());

describe("actor leases", () => {
  it("rejects the same ID's previous admission while retaining ordinary rotation and expiration", () => {
    useAuthStore.getState().setSession("a", studentUser);
    const lease = authStore.captureActor();
    useAuthStore.getState().setAccessToken("rotated");
    useAuthStore.getState().setUser({ ...studentUser, fullName: "Updated" });
    expect(authStore.isCurrent(lease)).toBe(true);
    useAuthStore.getState().expireSession();
    expect(authStore.isCurrent(lease)).toBe(true);
    expect(useAuthStore.getState().user?.fullName).toBe("Updated");
    useAuthStore.getState().setSession("b", studentUser);
    expect(authStore.isCurrent(lease)).toBe(false);
    expect(authStore.getGeneration()).toBeGreaterThan(lease.generation);
  });

  it("invalidates explicit departures and rejects another actor's reread", () => {
    useAuthStore.getState().setSession("a", studentUser);
    const lease = authStore.captureActor();
    useAuthStore.getState().setUser({ ...studentUser, id: "different" });
    expect(useAuthStore.getState().user?.id).toBe(studentUser.id);
    useAuthStore.getState().signOut();
    expect(authStore.isCurrent(lease)).toBe(false);
    const departed = authStore.captureActor();
    useAuthStore.getState().clearSession();
    expect(authStore.isCurrent(departed)).toBe(false);
  });
});

it.each(["new-session", "bootstrap-user"] as const)(
  "abandons pending PKCE UI after %s and never remembers its late authorization",
  async (change) => {
    if (change === "new-session") useAuthStore.getState().setSession("a", studentUser);
    else useAuthStore.getState().clearSession();
    let release!: (value: ArrayBuffer) => void;
    const held = new Promise<ArrayBuffer>((resolve) => {
      release = resolve;
    });
    const digest = vi.spyOn(crypto.subtle, "digest").mockImplementation(() => held);
    const remember = vi.spyOn(Storage.prototype, "setItem");
    const hook = renderHook(() => useGoogleSignIn());
    let completion!: Promise<void>;
    act(() => {
      completion = hook.result.current.start();
    });
    expect(hook.result.current.pending).toBe(true);
    act(() => {
      if (change === "new-session")
        useAuthStore.getState().setSession("b", studentUser);
      else useAuthStore.getState().setUser(studentUser);
    });
    try {
      expect(hook.result.current.pending).toBe(false);
      expect(hook.result.current.error).toBeNull();
    } finally {
      await act(async () => {
        release(new ArrayBuffer(32));
        await completion;
      });
      hook.unmount();
      digest.mockRestore();
    }
    expect(remember.mock.calls.some(([key]) => key === "quizzivy.oauth.pending")).toBe(
      false,
    );
    remember.mockRestore();
  },
);

it("settles its own PKCE attempt when a same-actor refresh starts during the digest", async () => {
  useAuthStore.getState().setSession("a", studentUser);
  let resolveDigest!: (value: ArrayBuffer) => void;
  const heldDigest = new Promise<ArrayBuffer>((resolve) => {
    resolveDigest = resolve;
  });
  let resolveRefresh!: () => void;
  const heldRefresh = new Promise<void>((resolve) => {
    resolveRefresh = resolve;
  });
  const digest = vi
    .spyOn(crypto.subtle, "digest")
    .mockImplementationOnce(() => heldDigest);
  const remember = vi.spyOn(Storage.prototype, "setItem");
  const assign = vi.fn();
  vi.stubGlobal("window", { ...window, location: { ...window.location, assign } });
  const hook = renderHook(() => useGoogleSignIn());
  let attempt!: Promise<void>;
  act(() => {
    attempt = hook.result.current.start();
  });
  expect(hook.result.current.pending).toBe(true);
  const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async () => {
    await heldRefresh;
    return Response.json({ accessToken: "rotated", expiresIn: 900 });
  });
  const refresh = api("post", "/auth/refresh");
  await act(async () => {
    resolveDigest(new ArrayBuffer(32));
    await attempt;
  });
  try {
    expect(hook.result.current.pending).toBe(false);
    expect(hook.result.current.error).toBe("Yêu cầu tài khoản vẫn đang được xử lý.");
    expect(remember.mock.calls.some(([key]) => key === "quizzivy.oauth.pending")).toBe(
      false,
    );
    expect(assign).not.toHaveBeenCalled();
    resolveRefresh();
    await refresh;
    await act(async () => {
      await hook.result.current.start();
    });
    expect(assign).toHaveBeenCalledOnce();
    expect(
      remember.mock.calls.filter(([key]) => key === "quizzivy.oauth.pending"),
    ).toHaveLength(1);
  } finally {
    resolveRefresh();
    await refresh;
    hook.unmount();
    digest.mockRestore();
    remember.mockRestore();
    fetch.mockRestore();
    vi.unstubAllGlobals();
  }
});

it("does not let an older local PKCE digest clear a newer start and permits an explicit fresh attempt after drain", async () => {
  useAuthStore.getState().setSession("a", studentUser);
  let oldDigest!: (value: ArrayBuffer) => void;
  let newDigest!: (value: ArrayBuffer) => void;
  const digest = vi
    .spyOn(crypto.subtle, "digest")
    .mockImplementationOnce(
      () =>
        new Promise<ArrayBuffer>((resolve) => {
          oldDigest = resolve;
        }),
    )
    .mockImplementationOnce(
      () =>
        new Promise<ArrayBuffer>((resolve) => {
          newDigest = resolve;
        }),
    )
    .mockResolvedValue(new ArrayBuffer(32));
  const remember = vi.spyOn(Storage.prototype, "setItem");
  const assign = vi.fn();
  vi.stubGlobal("window", { ...window, location: { ...window.location, assign } });
  const hook = renderHook(() => useGoogleSignIn());
  let old!: Promise<void>;
  let current!: Promise<void>;
  act(() => {
    old = hook.result.current.start();
    current = hook.result.current.start();
  });
  try {
    await act(async () => {
      oldDigest(new ArrayBuffer(32));
      await old;
    });
    expect(hook.result.current.pending).toBe(true);
    expect(assign).not.toHaveBeenCalled();
    await act(async () => {
      newDigest(new ArrayBuffer(32));
      await current;
    });
    expect(assign).toHaveBeenCalledOnce();
    expect(
      remember.mock.calls.filter(([key]) => key === "quizzivy.oauth.pending"),
    ).toHaveLength(1);
    await act(async () => {
      await hook.result.current.start();
    });
    expect(assign).toHaveBeenCalledTimes(2);
  } finally {
    hook.unmount();
    digest.mockRestore();
    remember.mockRestore();
    vi.unstubAllGlobals();
  }
});
