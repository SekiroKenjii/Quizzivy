import { afterEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import {
  chooseAccountPreference,
  savePreferences,
  saveProfilePatch,
  refreshAccount,
  runAccountMutation,
} from "@/features/auth/accountPreferences";
import { useAuthStore } from "@/stores/auth";
import { readThemePreference } from "@/lib/theme";
import { readLargerTestText } from "@/lib/testText";
import { getDisplayTimeZone } from "@/lib/i18n/datetime";
import i18n from "@/lib/i18n";
import { studentUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";

afterEach(() => {
  useAuthStore.getState().clearSession();
  localStorage.clear();
  vi.restoreAllMocks();
});

it("hydrates effective defaults over stale storage without materializing a PATCH", async () => {
  const writes = vi.fn(() => HttpResponse.json({}));
  server.use(
    http.patch("http://localhost:8080/auth/me", writes),
    http.patch("http://localhost:8080/me/preferences", writes),
  );
  localStorage.setItem("quizzivy.theme", "dark");
  localStorage.setItem("quizzivy.locale", "en");
  localStorage.setItem("quizzivy.testText", "large");
  useAuthStore.getState().setSession("a", studentUser);
  expect(readThemePreference()).toBe("light");
  expect(readLargerTestText()).toBe(false);
  expect(i18n.language).toBe("vi");
  expect(getDisplayTimeZone()).toBe("Asia/Ho_Chi_Minh");
  expect(useAuthStore.getState().user?.preferences).toBeUndefined();
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(writes).not.toHaveBeenCalled();
});

it("serializes all five profile fields while preserving null and omission", async () => {
  const bodies: unknown[] = [];
  useAuthStore.getState().setSession("a", studentUser);
  server.use(
    http.patch("http://localhost:8080/auth/me", async ({ request }) => {
      bodies.push(await request.json());
      return HttpResponse.json({
        ...studentUser,
        fullName: "New",
        locale: "en",
        timeZone: "UTC",
      });
    }),
  );
  await saveProfilePatch({
    fullName: "New",
    displayName: null,
    phone: null,
    locale: "en",
    timeZone: "UTC",
  });
  await saveProfilePatch({ fullName: "New" });
  expect(bodies).toEqual([
    { fullName: "New", displayName: null, phone: null, locale: "en", timeZone: "UTC" },
    { fullName: "New" },
  ]);
  expect(getDisplayTimeZone()).toBe("UTC");
});

it("accepts a complete preference response without dropping unrelated user fields or materializing defaults", async () => {
  useAuthStore.getState().setSession("a", {
    ...studentUser,
    preferences: {
      compactTables: true,
      assignmentDefaults: { durationMinutes: 30, showScore: true },
    },
  });
  const bodies: unknown[] = [];
  server.use(
    http.patch("http://localhost:8080/me/preferences", async ({ request }) => {
      bodies.push(await request.json());
      return HttpResponse.json({
        compactTables: true,
        theme: "system",
        largerTestText: false,
        assignmentDefaults: { durationMinutes: 15 },
      });
    }),
  );
  await savePreferences({
    assignmentDefaults: { durationMinutes: 15 },
    largerTestText: false,
  });
  expect(bodies).toEqual([
    { assignmentDefaults: { durationMinutes: 15 }, largerTestText: false },
  ]);
  expect(useAuthStore.getState().user).toMatchObject({
    id: studentUser.id,
    fullName: studentUser.fullName,
    preferences: {
      compactTables: true,
      theme: "system",
      largerTestText: false,
      assignmentDefaults: { durationMinutes: 15 },
    },
  });
  expect(
    useAuthStore.getState().user?.preferences?.assignmentDefaults,
  ).not.toHaveProperty("showScore");
  expect(readThemePreference()).toBe("system");
});

it("orders an authoritative reread before a newer write and cancels a queued old-actor send", async () => {
  useAuthStore.getState().setSession("a", studentUser);
  let release!: () => void;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  let entered!: () => void;
  const started = new Promise<void>((resolve) => {
    entered = resolve;
  });
  const patch = vi.fn(() => HttpResponse.json({ ...studentUser, fullName: "Wrong" }));
  server.use(
    http.get("http://localhost:8080/auth/me", async () => {
      entered();
      await held;
      return HttpResponse.json(studentUser);
    }),
    http.patch("http://localhost:8080/auth/me", patch),
  );
  const read = refreshAccount().catch((error: unknown) => error);
  await started;
  const queued = saveProfilePatch({ fullName: "Wrong" }).catch(
    (error: unknown) => error,
  );
  useAuthStore.getState().setSession("b", studentUser);
  release();
  expect(await read).toMatchObject({ name: "AbortError" });
  expect(await queued).toMatchObject({ name: "AbortError" });
  expect(patch).not.toHaveBeenCalled();
  expect(useAuthStore.getState().accessToken).toBe("b");
});

it("keeps a successful server acknowledgment when local mirror storage throws", async () => {
  useAuthStore.getState().setSession("a", studentUser);
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("No storage");
  });
  server.use(
    http.patch("http://localhost:8080/me/preferences", () =>
      HttpResponse.json({ theme: "system", largerTestText: true }),
    ),
  );
  await expect(chooseAccountPreference({ theme: "system" })).resolves.toBe(true);
  expect(useAuthStore.getState().user?.preferences).toEqual({
    theme: "system",
    largerTestText: true,
  });
  expect(readThemePreference()).toBe("system");
  expect(readLargerTestText()).toBe(true);
});

it("ignores foreign storage while signed in and keeps explicit false instead of a stale larger-text preference", () => {
  useAuthStore.getState().setSession("a", {
    ...studentUser,
    preferences: { theme: "system", largerTestText: false },
  });
  localStorage.setItem("quizzivy.theme", "dark");
  localStorage.setItem("quizzivy.testText", "large");
  window.dispatchEvent(
    new StorageEvent("storage", { key: "quizzivy.theme", newValue: "dark" }),
  );
  window.dispatchEvent(
    new StorageEvent("storage", { key: "quizzivy.testText", newValue: "large" }),
  );
  expect(readThemePreference()).toBe("system");
  expect(readLargerTestText()).toBe(false);
});

it("rejects a foreign authoritative user and cannot apply a departed actor's held acknowledgment", async () => {
  useAuthStore.getState().setSession("a", studentUser);
  server.use(
    http.get("http://localhost:8080/auth/me", () =>
      HttpResponse.json({ ...studentUser, id: "018f0000-0000-7000-8000-000000000099" }),
    ),
  );
  await expect(refreshAccount()).rejects.toMatchObject({ name: "AbortError" });
  expect(useAuthStore.getState().user?.id).toBe(studentUser.id);
  let release!: () => void;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  let entered!: () => void;
  const started = new Promise<void>((resolve) => {
    entered = resolve;
  });
  server.use(
    http.patch("http://localhost:8080/me/preferences", async () => {
      entered();
      await held;
      return HttpResponse.json({ theme: "dark" });
    }),
  );
  const choice = chooseAccountPreference({ theme: "dark" });
  await started;
  useAuthStore
    .getState()
    .setSession("b", { ...studentUser, preferences: { theme: "light" } });
  release();
  await expect(choice).resolves.toBe(false);
  expect(readThemePreference()).toBe("light");
  expect(useAuthStore.getState().accessToken).toBe("b");
});

it("keeps a security mutation and its reread ahead of a newer preference write", async () => {
  useAuthStore.getState().setSession("a", studentUser);
  let release!: () => void;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  let entered!: () => void;
  const started = new Promise<void>((resolve) => {
    entered = resolve;
  });
  const order: string[] = [];
  server.use(
    http.get("http://localhost:8080/auth/me", async () => {
      order.push("read");
      entered();
      await held;
      return HttpResponse.json({
        ...studentUser,
        googleLinked: true,
        preferences: { theme: "light" },
      });
    }),
    http.patch("http://localhost:8080/me/preferences", () => {
      order.push("patch");
      return HttpResponse.json({ theme: "dark" });
    }),
  );
  const security = runAccountMutation(async () => {
    order.push("security");
  });
  await started;
  const choice = savePreferences({ theme: "dark" });
  await Promise.resolve();
  expect(order).toEqual(["security", "read"]);
  release();
  await security;
  await choice;
  expect(order).toEqual(["security", "read", "patch"]);
  expect(useAuthStore.getState().user).toMatchObject({
    googleLinked: true,
    preferences: { theme: "dark" },
  });
});

it("sends an empty preferences object as a no-op and accepts the complete unchanged response", async () => {
  const preferences = {
    theme: "system" as const,
    compactTables: true,
    assignmentDefaults: { durationMinutes: 30, showScore: true },
  };
  useAuthStore.getState().setSession("a", { ...studentUser, preferences });
  const bodies: unknown[] = [];
  server.use(
    http.patch("http://localhost:8080/me/preferences", async ({ request }) => {
      bodies.push(await request.json());
      return HttpResponse.json(preferences);
    }),
  );
  await savePreferences({});
  expect(bodies).toEqual([{}]);
  expect(useAuthStore.getState().user?.preferences).toEqual(preferences);
});
