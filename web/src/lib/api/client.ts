import type { paths } from "./schema";
import { ApiError, toApiError } from "./errors";
import { authStore } from "@/stores/auth";

/**
 * The API client. Native `fetch`, no axios (§2), typed against the generated
 * contract so a call cannot name a path or method the API does not have.
 *
 * The interesting part is refresh. See `refreshSession` below.
 */

const BASE_URL: string =
  import.meta.env["VITE_API_BASE_URL"] ?? "http://localhost:8080";

// ---------------------------------------------------------------- typing

type HttpMethod = "get" | "post" | "patch" | "put" | "delete";

/**
 * Methods the contract actually declares for a path.
 *
 * openapi-typescript emits `post?: never` for methods a path does not support,
 * so `keyof` alone still yields "post" and `api("post", "/auth/me")` would
 * type-check against an endpoint that does not exist.
 *
 * Note the NonNullable. The OPTIONAL modifier in `post?: never` makes the
 * property type `never | undefined`, which collapses to `undefined` — not
 * `never` — so a naive `extends never` check silently keeps every unsupported
 * method. client.typecheck.ts asserts this with a @ts-expect-error, which is
 * self-verifying: if the constraint ever loosens, TypeScript reports the
 * directive as unused and the build fails.
 */
type MethodsOf<P extends keyof paths> = {
  [M in Extract<keyof paths[P], HttpMethod>]: [NonNullable<paths[P][M]>] extends [never]
    ? never
    : M;
}[Extract<keyof paths[P], HttpMethod>];

type JsonOf<T> = T extends { content: { "application/json": infer R } } ? R : never;

type ResponsesOf<O> = O extends { responses: infer R } ? R : never;
type OkStatusOf<O> = Extract<keyof ResponsesOf<O>, 200 | 201>;
/** 204 responses have no JSON body, so they resolve to `void`. */
type SuccessOf<O> = [OkStatusOf<O>] extends [never]
  ? void
  : JsonOf<ResponsesOf<O>[OkStatusOf<O>]>;

type BodyOf<O> = O extends { requestBody?: infer B }
  ? [B] extends [never]
    ? never
    : JsonOf<NonNullable<B>>
  : never;

type PathParamsOf<O> = O extends { parameters: { path?: infer P } }
  ? [P] extends [never]
    ? never
    : P extends undefined
      ? never
      : NonNullable<P>
  : never;

type QueryParamsOf<O> = O extends { parameters: { query?: infer Q } }
  ? [Q] extends [never]
    ? never
    : NonNullable<Q>
  : never;

type Optional<K extends string, T> = [T] extends [never]
  ? { [key in K]?: undefined }
  : { [key in K]: T };

export type RequestOptions<O> = Optional<"path", PathParamsOf<O>> &
  Optional<"query", QueryParamsOf<O>> &
  Optional<"body", BodyOf<O>> & { signal?: AbortSignal };

// ------------------------------------------------------------ url building

function buildUrl(
  path: string,
  pathParams?: Record<string, unknown>,
  query?: Record<string, unknown>,
): string {
  let resolved = path;
  if (pathParams) {
    for (const [key, value] of Object.entries(pathParams)) {
      resolved = resolved.replace(`{${key}}`, encodeURIComponent(String(value)));
    }
  }
  const url = new URL(BASE_URL + resolved);
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value === undefined || value === null) continue;
      url.searchParams.set(key, String(value));
    }
  }
  return url.toString();
}

// ------------------------------------------------------- single-flight refresh

let inFlightRefresh: Promise<boolean> | null = null;

/** Replaced in tests; in the app it sends the user to /login. */
let onSessionLost: () => void = () => {
  if (typeof window !== "undefined" && window.location.pathname !== "/login") {
    window.location.assign("/login");
  }
};

export function setSessionLostHandler(handler: () => void) {
  onSessionLost = handler;
}

/** Test seam. Never call this from application code. */
export function __resetRefreshStateForTests() {
  inFlightRefresh = null;
}

async function performRefresh(): Promise<boolean> {
  const response = await fetch(buildUrl("/auth/refresh"), {
    method: "POST",
    credentials: "include",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) return false;

  const data = (await response.json()) as { accessToken?: string };
  if (!data.accessToken) return false;

  authStore.setAccessToken(data.accessToken);
  return true;
}

/**
 * **Single-flight.** Concurrent callers share one in-flight request.
 *
 * This is not an optimisation, it is a correctness requirement. §5.2 holds the
 * access token in memory only, so a cold page load has none; TanStack Query
 * mounts several queries at once; every one of them 401s. Without this, each
 * would POST /auth/refresh with the same cookie value. The first rotates it,
 * and the rest present an already-rotated token — which §5.2's reuse detection
 * correctly treats as theft, revoking the whole family and logging the user out.
 *
 * The symptom is "the app signs me out every time I refresh the page", and the
 * cause is invisible in any single request. `docs/plan/30-risks.md` R-06.
 */
function refreshSession(): Promise<boolean> {
  inFlightRefresh ??= performRefresh()
    .catch(() => false)
    .finally(() => {
      inFlightRefresh = null;
    });
  return inFlightRefresh;
}

// ------------------------------------------------------------------ request

/** Endpoints that must never trigger a refresh-and-retry, or we loop. */
function isAuthEntryPoint(path: string): boolean {
  return path === "/auth/refresh" || path === "/auth/login" || path === "/auth/google";
}

export async function api<P extends keyof paths, M extends MethodsOf<P>>(
  method: M,
  path: P,
  options: RequestOptions<paths[P][M]> = {} as RequestOptions<paths[P][M]>,
): Promise<SuccessOf<paths[P][M]>> {
  const opts = options as {
    path?: Record<string, unknown>;
    query?: Record<string, unknown>;
    body?: unknown;
    signal?: AbortSignal;
  };
  const url = buildUrl(path as string, opts.path, opts.query);

  const send = async (): Promise<Response> => {
    const headers: Record<string, string> = { Accept: "application/json" };
    const token = authStore.getAccessToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
    if (opts.body !== undefined) headers["Content-Type"] = "application/json";

    return fetch(url, {
      method: method.toUpperCase(),
      headers,
      // The refresh cookie is host-only with Path=/auth (§5.2), so the browser
      // only actually attaches it to /auth/*. Sending credentials uniformly
      // keeps one code path; the cookie's own scoping does the restricting.
      credentials: "include",
      ...(opts.body !== undefined ? { body: JSON.stringify(opts.body) } : {}),
      ...(opts.signal ? { signal: opts.signal } : {}),
    });
  };

  let response = await send();

  if (response.status === 401 && !isAuthEntryPoint(path as string)) {
    const refreshed = await refreshSession();
    if (!refreshed) {
      authStore.clear();
      onSessionLost();
      throw await toApiError(response);
    }

    // Retry exactly once (§5.2). A second 401 means the fresh token is not
    // being accepted either, so the session is genuinely gone.
    response = await send();
    if (response.status === 401) {
      authStore.clear();
      onSessionLost();
      throw await toApiError(response);
    }
  }

  if (!response.ok) throw await toApiError(response);

  if (response.status === 204 || response.headers.get("Content-Length") === "0") {
    return undefined as SuccessOf<paths[P][M]>;
  }
  return (await response.json()) as SuccessOf<paths[P][M]>;
}

export { ApiError };
