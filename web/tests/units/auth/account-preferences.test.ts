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
import { adminUser, studentUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";

const toastSpy = vi.hoisted(() => vi.fn());
vi.mock("@/components/ui/sonner", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/components/ui/sonner")>();
  const toast = Object.assign((...args: Parameters<typeof actual.toast>) => {
    toastSpy(...args);
    return actual.toast(...args);
  }, actual.toast);
  return { ...actual, toast };
});

afterEach(() => {
  useAuthStore.getState().clearSession();
  localStorage.clear();
  vi.restoreAllMocks();
  toastSpy.mockClear();
});

it("applies a device's v0.9.0 choices the account lacks, and saves them once", async () => {
  const preferenceBodies: unknown[] = [];
  const profileBodies: unknown[] = [];
  server.use(
    http.patch("http://localhost:8080/me/preferences", async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      preferenceBodies.push(body);
      return HttpResponse.json(body);
    }),
    http.patch("http://localhost:8080/auth/me", async ({ request }) => {
      const body = (await request.json()) as { locale: "vi" | "en" };
      profileBodies.push(body);
      return HttpResponse.json({
        ...studentUser,
        preferences: preferenceBodies.at(-1),
        ...body,
      });
    }),
  );
  localStorage.setItem("quizzivy.theme", "dark");
  localStorage.setItem("quizzivy.locale", "en");
  localStorage.setItem("quizzivy.testText", "large");
  useAuthStore.getState().setSession("a", studentUser);
  expect(readThemePreference()).toBe("dark");
  expect(readLargerTestText()).toBe(true);
  expect(getDisplayTimeZone()).toBe("Asia/Ho_Chi_Minh");
  await vi.waitFor(() => expect(profileBodies).toHaveLength(1));
  expect(preferenceBodies).toEqual([{ theme: "dark", largerTestText: true }]);
  expect(profileBodies).toEqual([{ locale: "en" }]);
  await vi.waitFor(() => expect(i18n.language).toBe("en"));
  const user = useAuthStore.getState().user;
  expect(user?.preferences).toEqual({ theme: "dark", largerTestText: true });
  expect(user?.locale).toBe("en");
  useAuthStore.getState().setUser({ ...user!, fullName: "Nguyễn Văn Bình" });
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(preferenceBodies).toHaveLength(1);
  expect(profileBodies).toHaveLength(1);
  await i18n.changeLanguage("vi");
});

it("keeps the account's value over the device's and saves nothing", async () => {
  const writes = vi.fn(() => HttpResponse.json({}));
  server.use(
    http.patch("http://localhost:8080/auth/me", writes),
    http.patch("http://localhost:8080/me/preferences", writes),
  );
  localStorage.setItem("quizzivy.theme", "dark");
  localStorage.setItem("quizzivy.testText", "large");
  useAuthStore.getState().setSession("a", {
    ...studentUser,
    locale: "vi",
    preferences: { theme: "light", largerTestText: false },
  });
  expect(readThemePreference()).toBe("light");
  expect(readLargerTestText()).toBe(false);
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(writes).not.toHaveBeenCalled();
});

it("hydrates the defaults and saves nothing when the device stores no choice", async () => {
  const writes = vi.fn(() => HttpResponse.json({}));
  server.use(
    http.patch("http://localhost:8080/auth/me", writes),
    http.patch("http://localhost:8080/me/preferences", writes),
  );
  useAuthStore.getState().setSession("a", studentUser);
  expect(readThemePreference()).toBe("light");
  expect(readLargerTestText()).toBe(false);
  expect(i18n.language).toBe("vi");
  expect(getDisplayTimeZone()).toBe("Asia/Ho_Chi_Minh");
  expect(useAuthStore.getState().user?.preferences).toBeUndefined();
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(writes).not.toHaveBeenCalled();
});

it("treats an invalid stored value as no choice", async () => {
  const writes = vi.fn(() => HttpResponse.json({}));
  server.use(
    http.patch("http://localhost:8080/auth/me", writes),
    http.patch("http://localhost:8080/me/preferences", writes),
  );
  localStorage.setItem("quizzivy.theme", "sepia");
  localStorage.setItem("quizzivy.locale", "fr");
  localStorage.setItem("quizzivy.testText", "huge");
  useAuthStore.getState().setSession("a", studentUser);
  expect(readThemePreference()).toBe("light");
  expect(readLargerTestText()).toBe(false);
  expect(i18n.language).toBe("vi");
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(writes).not.toHaveBeenCalled();
});

it("keeps the device's choice when the save fails, says nothing, and tries again at the next sign-in", async () => {
  const attempts = vi.fn(() => HttpResponse.json({}, { status: 503 }));
  server.use(http.patch("http://localhost:8080/me/preferences", attempts));
  localStorage.setItem("quizzivy.theme", "dark");
  useAuthStore.getState().setSession("a", studentUser);
  await vi.waitFor(() => expect(attempts).toHaveBeenCalledTimes(1));
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(readThemePreference()).toBe("dark");
  expect(toastSpy).not.toHaveBeenCalled();
  useAuthStore.getState().setUser({ ...studentUser, fullName: "Nguyễn Văn Bình" });
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(attempts).toHaveBeenCalledTimes(1);
  useAuthStore.getState().clearSession();
  useAuthStore.getState().setSession("b", studentUser);
  expect(readThemePreference()).toBe("dark");
  await vi.waitFor(() => expect(attempts).toHaveBeenCalledTimes(2));
});

it("sends nothing for an actor that left before its turn, and nothing more after a departure mid-flight", async () => {
  let release!: () => void;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const preferenceWrites = vi.fn(async () => {
    await held;
    return HttpResponse.json({ theme: "dark" });
  });
  const profileWrites = vi.fn(() => HttpResponse.json(studentUser));
  server.use(
    http.patch("http://localhost:8080/me/preferences", preferenceWrites),
    http.patch("http://localhost:8080/auth/me", profileWrites),
  );
  localStorage.setItem("quizzivy.theme", "dark");
  useAuthStore.getState().setSession("a", studentUser);
  useAuthStore.getState().clearSession();
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(preferenceWrites).not.toHaveBeenCalled();

  localStorage.setItem("quizzivy.locale", "en");
  localStorage.removeItem("quizzivy.preferenceMirror");
  localStorage.setItem("quizzivy.theme", "dark");
  useAuthStore.getState().setSession("b", studentUser);
  await vi.waitFor(() => expect(preferenceWrites).toHaveBeenCalledTimes(1));
  useAuthStore.getState().setSession("c", {
    ...adminUser,
    locale: "vi",
    preferences: { theme: "light", largerTestText: false },
  });
  release();
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(profileWrites).not.toHaveBeenCalled();
  expect(useAuthStore.getState().user?.id).toBe(adminUser.id);
  expect(useAuthStore.getState().user?.preferences).toEqual({
    theme: "light",
    largerTestText: false,
  });
  expect(readThemePreference()).toBe("light");
  await i18n.changeLanguage("vi");
});

it("does not take another account's mirrored values for this device's choice", async () => {
  const writes = vi.fn(() => HttpResponse.json({}));
  server.use(
    http.patch("http://localhost:8080/auth/me", writes),
    http.patch("http://localhost:8080/me/preferences", writes),
  );
  useAuthStore.getState().setSession("a", {
    ...adminUser,
    locale: "vi",
    preferences: { theme: "dark", largerTestText: true },
  });
  expect(localStorage.getItem("quizzivy.theme")).toBe("dark");
  useAuthStore.getState().clearSession();
  useAuthStore.getState().setSession("b", studentUser);
  expect(readThemePreference()).toBe("light");
  expect(readLargerTestText()).toBe(false);
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(writes).not.toHaveBeenCalled();
});

it("saves a choice made with no account signed in at the next sign-in", async () => {
  const bodies: unknown[] = [];
  server.use(
    http.patch("http://localhost:8080/me/preferences", async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      bodies.push(body);
      return HttpResponse.json(body);
    }),
  );
  useAuthStore.getState().setSession("a", {
    ...adminUser,
    preferences: { theme: "light" },
  });
  useAuthStore.getState().clearSession();
  await chooseAccountPreference({ theme: "dark" });
  useAuthStore.getState().setSession("b", studentUser);
  await vi.waitFor(() => expect(bodies).toEqual([{ theme: "dark" }]));
  expect(readThemePreference()).toBe("dark");
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
