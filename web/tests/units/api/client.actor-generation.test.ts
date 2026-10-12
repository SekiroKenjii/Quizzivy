import { QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  api,
  __resetRefreshStateForTests,
  setMaintenanceHandler,
  setSessionLostHandler,
  uploadFile,
} from "@/lib/api/client";
import { useAuthStore } from "@/stores/auth";
import { signInAccount } from "@/features/auth/accountPreferences";
import { transitionStatus } from "@/lib/api/authTransition";
import { studentUser } from "@tests/support/fixtures";
import { http, HttpResponse } from "msw";
import { server } from "@tests/support/server";
vi.mock("@/lib/drafts/store", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/drafts/store")>()),
  clearAuthoringDrafts: async () => undefined,
}));

function gate() {
  let open!: () => void;
  const promise = new Promise<void>((done) => {
    open = done;
  });
  return { promise, open };
}

beforeEach(() => {
  __resetRefreshStateForTests();
  useAuthStore.getState().setSession("a", studentUser);
});
afterEach(() => {
  useAuthStore.getState().clearSession();
  setSessionLostHandler(() => undefined);
  setMaintenanceHandler(() => undefined);
  vi.restoreAllMocks();
});

describe.each(["fetch", "upload"] as const)("%s actor fencing", (kind) => {
  it("does not rotate or replay A's delayed first 401 using B's token", async () => {
    const asked = gate();
    const response = gate();
    const lost = vi.fn();
    const refresh = vi.fn(() =>
      HttpResponse.json({ accessToken: "wrong", expiresIn: 900 }),
    );
    setSessionLostHandler(lost);
    const sent: (string | null)[] = [];
    const handle = async ({ request }: { request: Request }) => {
      sent.push(request.headers.get("Authorization"));
      asked.open();
      await response.promise;
      return HttpResponse.json(
        { error: { code: "UNAUTHORIZED", message: "Denied" } },
        { status: 401 },
      );
    };
    server.use(
      http.get("http://localhost:8080/app/classes", handle),
      http.post("http://localhost:8080/teacher/media", handle),
      http.post("http://localhost:8080/auth/refresh", refresh),
    );
    const failure = (
      kind === "fetch"
        ? api("get", "/app/classes")
        : uploadFile("/teacher/media", new File(["x"], "a.mp3"))
    ).catch((error: unknown) => error);
    await asked.promise;
    useAuthStore.getState().setSession("b", studentUser);
    response.open();
    expect(await failure).toMatchObject({ status: 401 });
    expect(refresh).not.toHaveBeenCalled();
    expect(sent).toEqual(["Bearer a"]);
    expect(lost).not.toHaveBeenCalled();
    expect(useAuthStore.getState().accessToken).toBe("b");
    expect(useAuthStore.getState().expired).toBe(false);
  });
});

it("discards stale successful bodies and maintenance effects", async () => {
  const asked = gate();
  const response = gate();
  const maintenance = vi.fn();
  setMaintenanceHandler(maintenance);
  server.use(
    http.get("http://localhost:8080/app/classes", async () => {
      asked.open();
      await response.promise;
      return HttpResponse.json({ items: [] });
    }),
  );
  const result = api("get", "/app/classes").catch((error: unknown) => error);
  await asked.promise;
  useAuthStore.getState().setSession("b", studentUser);
  response.open();
  expect(await result).toMatchObject({ name: "AbortError" });
  expect(maintenance).not.toHaveBeenCalled();
});

for (const kind of ["fetch", "upload"] as const) {
  it(`${kind} cannot install a held refresh token or replay after same-ID relogin`, async () => {
    const started = gate();
    const response = gate();
    const sends: (string | null)[] = [];
    const refuse = ({ request }: { request: Request }) => {
      sends.push(request.headers.get("Authorization"));
      return HttpResponse.json(
        { error: { code: "UNAUTHORIZED", message: "Denied" } },
        { status: 401 },
      );
    };
    server.use(
      http.get("http://localhost:8080/app/classes", refuse),
      http.post("http://localhost:8080/teacher/media", refuse),
      http.post("http://localhost:8080/auth/refresh", async () => {
        started.open();
        await response.promise;
        return HttpResponse.json({ accessToken: "late-a", expiresIn: 900 });
      }),
    );
    const result = (
      kind === "fetch"
        ? api("get", "/app/classes")
        : uploadFile("/teacher/media", new File(["x"], "a.mp3"))
    ).catch((error: unknown) => error);
    await started.promise;
    useAuthStore.getState().setSession("b", studentUser);
    response.open();
    expect(await result).toMatchObject({ status: 401 });
    expect(useAuthStore.getState().accessToken).toBe("b");
    expect(sends).toEqual(["Bearer a"]);
    expect(transitionStatus().cookieKind).toBeNull();
  });

  it(`${kind} discards stale maintenance effects`, async () => {
    const asked = gate();
    const response = gate();
    const maintenance = vi.fn();
    setMaintenanceHandler(maintenance);
    const fail = async () => {
      asked.open();
      await response.promise;
      return HttpResponse.json(
        {
          error: {
            code: "MAINTENANCE",
            message: "Maintenance",
            details: {
              startsAt: "2026-10-06T08:00:00Z",
              endsAt: "2026-10-06T09:00:00Z",
            },
          },
        },
        { status: 503 },
      );
    };
    server.use(
      http.get("http://localhost:8080/app/classes", fail),
      http.post("http://localhost:8080/teacher/media", fail),
    );
    const result = (
      kind === "fetch"
        ? api("get", "/app/classes")
        : uploadFile("/teacher/media", new File(["x"], "a.mp3"))
    ).catch((error: unknown) => error);
    await asked.promise;
    useAuthStore.getState().setSession("b", studentUser);
    response.open();
    expect(await result).toMatchObject({ status: 503 });
    expect(maintenance).not.toHaveBeenCalled();
    expect(useAuthStore.getState().accessToken).toBe("b");
  });
}

it("keeps a direct cookie response body owned after caller abort until the next explicit login can send", async () => {
  let finish!: () => void;
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      finish = () => {
        controller.enqueue(
          new TextEncoder().encode(
            JSON.stringify({ accessToken: "late-a", expiresIn: 900 }),
          ),
        );
        controller.close();
      };
    },
  });
  const asked = gate();
  let loginCalls = 0;
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    if (String(input).endsWith("/auth/refresh")) {
      asked.open();
      return new Response(stream, { headers: { "Content-Type": "application/json" } });
    }
    loginCalls += 1;
    return Response.json({ accessToken: "b", expiresIn: 900, user: studentUser });
  });
  const controller = new AbortController();
  const old = api("post", "/auth/refresh", { signal: controller.signal }).catch(
    (error: unknown) => error,
  );
  await asked.promise;
  controller.abort();
  const next = admitAccount("login", (ticket) =>
    api("post", "/auth/login", {
      body: { email: studentUser.email, password: "password" },
      transition: ticket,
    }),
  );
  await Promise.resolve();
  expect(loginCalls).toBe(0);
  expect(transitionStatus().cookieKind).toBe("refresh");
  finish();
  expect(await old).toMatchObject({ name: "AbortError" });
  await next;
  expect(loginCalls).toBe(1);
  expect(useAuthStore.getState().accessToken).toBe("b");
  expect(transitionStatus().kind).toBe("idle");
});

it.each(["malformed", "http", "network"] as const)(
  "naturally releases a cookie operation after %s failure",
  async (kind) => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async () => {
      if (kind === "network") throw new TypeError("Network failed");
      if (kind === "http")
        return Response.json(
          { error: { code: "INVALID_CREDENTIALS", message: "Denied" } },
          { status: 401 },
        );
      return new Response("{", { headers: { "Content-Type": "application/json" } });
    });
    await expect(
      api("post", "/auth/login", {
        body: { email: studentUser.email, password: "password" },
      }),
    ).rejects.toBeInstanceOf(Error);
    expect(transitionStatus().kind).toBe("idle");
    expect(transitionStatus().cookieKind).toBeNull();
  },
);

it.each(["direct-first", "automatic-first"] as const)(
  "shares one complete refresh payload and new-token replay with %s",
  async (order) => {
    const body = gate();
    const asked = gate();
    const protectedAsked = gate();
    const payload = { accessToken: "rotated", expiresIn: 900 };
    const tokens: (string | null)[] = [];
    let refreshes = 0;
    const lost = vi.fn();
    setSessionLostHandler(lost);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, options) => {
      if (String(input).endsWith("/auth/refresh")) {
        refreshes += 1;
        asked.open();
        const stream = new ReadableStream<Uint8Array>({
          async start(controller) {
            await body.promise;
            controller.enqueue(new TextEncoder().encode(JSON.stringify(payload)));
            controller.close();
          },
        });
        return new Response(stream, {
          headers: { "Content-Type": "application/json" },
        });
      }
      const token = new Headers(options?.headers).get("Authorization");
      tokens.push(token);
      protectedAsked.open();
      return token === "Bearer rotated"
        ? Response.json({ items: [] })
        : Response.json(
            { error: { code: "UNAUTHORIZED", message: "Denied" } },
            { status: 401 },
          );
    });
    const first =
      order === "direct-first"
        ? api("post", "/auth/refresh")
        : api("get", "/app/classes");
    await asked.promise;
    const second =
      order === "direct-first"
        ? api("get", "/app/classes")
        : api("post", "/auth/refresh");
    await protectedAsked.promise;
    await Promise.resolve();
    expect(transitionStatus().cookieKind).toBe("refresh");
    expect(refreshes).toBe(1);
    body.open();
    const results = await Promise.all([first, second]);
    expect(results[order === "direct-first" ? 0 : 1]).toEqual(payload);
    expect(results[order === "direct-first" ? 1 : 0]).toEqual({ items: [] });
    expect(tokens).toEqual(["Bearer a", "Bearer rotated"]);
    expect(refreshes).toBe(1);
    expect(lost).not.toHaveBeenCalled();
    expect(transitionStatus().kind).toBe("idle");
  },
);

function admitAccount(
  kind: Parameters<typeof signInAccount>[0],
  work: Parameters<typeof signInAccount>[1],
) {
  return signInAccount(kind, work, new QueryClient());
}

it.each(["refused", "malformed", "stale-actor"] as const)(
  "drains a joined %s refresh before the next cookie owner can send",
  async (failure) => {
    const body = gate();
    const asked = gate();
    const protectedAsked = gate();
    let refreshes = 0;
    let logins = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (String(input).endsWith("/auth/refresh")) {
        refreshes += 1;
        asked.open();
        const stream = new ReadableStream<Uint8Array>({
          async start(controller) {
            await body.promise;
            const payload =
              failure === "refused"
                ? { error: { code: "UNAUTHORIZED", message: "Denied" } }
                : { accessToken: "late-a", expiresIn: 900 };
            const text = failure === "malformed" ? "{" : JSON.stringify(payload);
            controller.enqueue(new TextEncoder().encode(text));
            controller.close();
          },
        });
        return new Response(stream, { status: failure === "refused" ? 401 : 200 });
      }
      if (String(input).endsWith("/auth/login")) {
        logins += 1;
        return Response.json({ accessToken: "b", expiresIn: 900, user: studentUser });
      }
      protectedAsked.open();
      return Response.json(
        { error: { code: "UNAUTHORIZED", message: "Denied" } },
        { status: 401 },
      );
    });
    const direct = api("post", "/auth/refresh").catch((error: unknown) => error);
    await asked.promise;
    const automatic = api("get", "/app/classes").catch((error: unknown) => error);
    await protectedAsked.promise;
    await Promise.resolve();
    if (failure === "stale-actor")
      useAuthStore.getState().setSession("new-actor", studentUser);
    const next = admitAccount("login", (ticket) =>
      api("post", "/auth/login", {
        transition: ticket,
        body: { email: studentUser.email, password: "password" },
      }),
    );
    await Promise.resolve();
    expect(logins).toBe(0);
    expect(transitionStatus().cookieKind).toBe("refresh");
    body.open();
    const results = await Promise.all([direct, automatic]);
    const errorNames = {
      "stale-actor": "AbortError",
      malformed: "SyntaxError",
      refused: "ApiError",
    };
    expect(results[0]).toMatchObject({ name: errorNames[failure] });
    expect(results[1]).toMatchObject({ status: 401 });
    await next;
    expect(refreshes).toBe(1);
    expect(logins).toBe(1);
    expect(useAuthStore.getState().accessToken).toBe("b");
    expect(transitionStatus().kind).toBe("idle");
  },
);
