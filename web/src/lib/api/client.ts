import type { paths, operations } from "./schema";
import { ApiError, maintenanceWindow, toApiError, type ApiErrorCode } from "./errors";
import i18n from "@/lib/i18n";
import { authStore, type ActorLease } from "@/stores/auth";
import {
  cookieOperation,
  transitionStatus,
  type AccountTransition,
  type CookieKind,
} from "./authTransition";

/**
 * The API client. Native `fetch`, no axios (§2), typed against the generated
 * contract so a call cannot name a path or method the API does not have.
 *
 * The interesting part is refresh. See `refreshSession` below.
 */

// Exported for the one caller that cannot go through `api()`: the pagehide
// beacon, which is a navigator.sendBeacon and not a fetch (D-03).
export const BASE_URL: string =
  import.meta.env["VITE_API_BASE_URL"] ?? "http://localhost:8080";

// ---------------------------------------------------------------- typing

type HttpMethod = "get" | "post" | "patch" | "put" | "delete";

/**
 * The single fetch wrapper every request goes through (§2).
 *
 * Typed against the generated schema, so a path, method or body that is not in
 * the contract fails to compile rather than 404ing at runtime.
 */
type MethodsOf<P extends keyof paths> = {
  [M in Extract<keyof paths[P], HttpMethod>]: [NonNullable<paths[P][M]>] extends [never]
    ? never
    : M;
}[Extract<keyof paths[P], HttpMethod>];

type JsonOf<T> = T extends { content: { "application/json": infer R } } ? R : never;

type ResponsesOf<O> = O extends { responses: infer R } ? R : never;
type OkStatusOf<O> = Extract<keyof ResponsesOf<O>, 200 | 201 | 202>;
type SuccessOf<O> = [OkStatusOf<O>] extends [never]
  ? void
  : [JsonOf<ResponsesOf<O>[OkStatusOf<O>]>] extends [never]
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

type BodyOption<O> = O extends { requestBody: unknown }
  ? Optional<"body", BodyOf<O>>
  : Partial<Optional<"body", BodyOf<O>>>;

export type RequestOptions<O> = Optional<"path", PathParamsOf<O>> &
  Optional<"query", QueryParamsOf<O>> &
  BodyOption<O> & {
    signal?: AbortSignal;
    cache?: RequestCache;
    transition?: AccountTransition;
  };

// ------------------------------------------------------------ url building

function fillPath(path: string, params?: Record<string, unknown>): string {
  if (!params) return path;
  let resolved = path;
  for (const [key, value] of Object.entries(params)) {
    resolved = resolved.replace(`{${key}}`, encodeURIComponent(String(value)));
  }
  return resolved;
}

// A repeated key per element is what OpenAPI's `style: form, explode: true`
// means and what the Go binder reads; String(array) would send "a,b" as one.
function appendValue(url: URL, key: string, value: unknown): void {
  if (value === undefined || value === null) return;
  if (Array.isArray(value)) {
    for (const item of value) appendValue(url, key, item);
    return;
  }
  url.searchParams.append(key, String(value));
}

function buildUrl(
  path: string,
  pathParams?: Record<string, unknown>,
  query?: Record<string, unknown>,
): string {
  const url = new URL(BASE_URL + fillPath(path, pathParams));
  for (const [key, value] of Object.entries(query ?? {})) {
    appendValue(url, key, value);
  }
  return url.toString();
}

// ------------------------------------------------------- single-flight refresh

type RefreshPayload =
  operations["refreshSession"]["responses"][200]["content"]["application/json"];
type RefreshResult =
  { kind: "refreshed"; data: RefreshPayload } | { kind: "failed"; error: unknown };

let inFlightRefresh: { actor: ActorLease; completion: Promise<RefreshResult> } | null =
  null;

/** Replaced in tests; in the app it sends the user to /login. */
let onSessionLost: () => void = () => {
  if (typeof window === "undefined") return;
  const path = window.location.pathname;
  const isPublic =
    path === "/login" ||
    path === "/join" ||
    path.startsWith("/join/") ||
    path.startsWith("/auth/");
  if (!isPublic) window.location.assign("/login");
};

let onMaintenance: (window: { startsAt: string; endsAt: string }) => void = () => {};

/**
 * setSessionLostHandler registers what runs when the session is refused. A
 * refusal counts only against the session its request was sent under: when the
 * store's token is no longer the one that request carried, the store is left
 * alone and the handler does not run. Otherwise a signed-in user is kept, with
 * the token dropped and the session marked expired, so the page under the
 * "sign in again" overlay stays mounted; anyone else is cleared.
 */
export function setSessionLostHandler(handler: () => void) {
  onSessionLost = handler;
}

function loseSession(sentWith: string | null, lease: ActorLease) {
  if (!authStore.isCurrent(lease) || authStore.getAccessToken() !== sentWith) return;
  if (authStore.isSignedIn()) {
    authStore.expire();
  } else {
    authStore.refuseAnonymous();
  }
  onSessionLost();
}

/**
 * setMaintenanceHandler registers what runs when any response is a 503
 * `MAINTENANCE`, with the window it names, before the error is thrown.
 */
export function setMaintenanceHandler(
  handler: (window: { startsAt: string; endsAt: string }) => void,
) {
  onMaintenance = handler;
}

/** Test seam. Never call this from application code. */
export function __resetRefreshStateForTests() {
  inFlightRefresh = null;
}

function language(): string {
  return i18n.language?.startsWith("en") ? "en" : "vi";
}

function unavailable(): ApiError {
  return new ApiError({
    status: 0,
    code: "UNKNOWN",
    message: i18n.t("api.unavailable"),
  });
}

function noticed(error: ApiError, lease: ActorLease): ApiError {
  const window = maintenanceWindow(error);
  if (window && authStore.isCurrent(lease)) onMaintenance(window);
  return error;
}

async function performRefresh(lease: ActorLease): Promise<RefreshPayload> {
  return cookieOperation("refresh", lease.generation, async () => {
    assertActor(lease);
    const response = await fetch(buildUrl("/auth/refresh"), {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Accept-Language": language() },
    });
    if (!response.ok) throw noticed(await toApiError(response), lease);
    const data: unknown = await response.json();
    assertActor(lease);
    if (
      typeof data !== "object" ||
      data === null ||
      !("accessToken" in data) ||
      typeof data.accessToken !== "string" ||
      !data.accessToken ||
      !("expiresIn" in data) ||
      typeof data.expiresIn !== "number"
    )
      throw unavailable();
    authStore.setAccessToken(data.accessToken);
    return { accessToken: data.accessToken, expiresIn: data.expiresIn };
  });
}

function refreshSession(lease: ActorLease): Promise<RefreshResult> {
  if (
    inFlightRefresh?.actor.generation === lease.generation &&
    inFlightRefresh.actor.userId === lease.userId
  )
    return inFlightRefresh.completion;
  const completion = performRefresh(lease)
    .then(
      (data): RefreshResult => ({ kind: "refreshed", data }),
      (error: unknown): RefreshResult => ({ kind: "failed", error }),
    )
    .finally(() => {
      if (inFlightRefresh === mine) inFlightRefresh = null;
    });
  const mine = { actor: lease, completion };
  inFlightRefresh = mine;
  return completion;
}

// ------------------------------------------------------------------ request

/** Endpoints that must never trigger a refresh-and-retry, or we loop. */
function isAuthEntryPoint(path: string): boolean {
  return (
    path === "/auth/refresh" ||
    path === "/auth/login" ||
    path === "/auth/google" ||
    path === "/auth/logout"
  );
}

function cookieKind(method: string, path: string): CookieKind | null {
  if (method !== "post") return null;
  if (path === "/auth/refresh") return "refresh";
  if (path === "/auth/login") return "login";
  if (path === "/auth/google") return "google";
  if (path === "/auth/logout") return "logout";
  return null;
}

async function tokenAfterRefresh(
  lease: ActorLease,
  sentWith: string | null,
  originalError: () => Promise<ApiError>,
  notifyAnonymous = true,
) {
  if (!authStore.isCurrent(lease) || authStore.isExpired()) throw await originalError();
  if (!["idle", "refresh"].includes(transitionStatus().kind)) {
    if (notifyAnonymous && !authStore.isSignedIn()) loseSession(sentWith, lease);
    throw await originalError();
  }
  const outcome = await refreshSession(lease);
  if (!authStore.isCurrent(lease)) throw await originalError();
  if (outcome.kind === "failed") {
    if (outcome.error instanceof ApiError && !outcome.error.isRetryable) {
      loseSession(sentWith, lease);
      throw await originalError();
    }
    throw outcome.error instanceof ApiError ? outcome.error : unavailable();
  }
  if (!["idle", "refresh"].includes(transitionStatus().kind))
    throw await originalError();
  assertActor(lease);
  return authStore.getAccessToken();
}

function assertActor(lease: ActorLease) {
  if (!authStore.isCurrent(lease)) throw new DOMException("Superseded", "AbortError");
}

/**
 * api sends one request typed against the contract and returns its JSON body.
 * A 401 outside the auth entry points waits for the shared refresh and is sent
 * once more with the new token. A refused refresh, or a 401 for that second
 * send, loses the session only when the store still holds the token the
 * refused request was sent with; either way the 401 is thrown to the caller.
 */
export function api<P extends keyof paths, M extends MethodsOf<P>>(
  method: M,
  path: P,
  options?: RequestOptions<paths[P][M]>,
): Promise<SuccessOf<paths[P][M]>>;
export async function api<P extends keyof paths, M extends MethodsOf<P>>(
  method: M,
  path: P,
  options: RequestOptions<paths[P][M]> = {} as RequestOptions<paths[P][M]>,
): Promise<unknown> {
  const opts = options as {
    path?: Record<string, unknown>;
    query?: Record<string, unknown>;
    body?: unknown;
    signal?: AbortSignal;
    cache?: RequestCache;
    transition?: AccountTransition;
  };
  const url = buildUrl(path as string, opts.path, opts.query);
  const lease = authStore.captureActor();
  const kind = cookieKind(method, path as string);

  const send = async (token: string | null): Promise<Response> => {
    const headers: Record<string, string> = {
      Accept: "application/json",
      "Accept-Language": language(),
    };
    if (token) headers["Authorization"] = `Bearer ${token}`;
    if (opts.body !== undefined) headers["Content-Type"] = "application/json";

    return fetch(url, {
      method: method.toUpperCase(),
      headers,
      credentials: "include",
      ...(opts.body !== undefined ? { body: JSON.stringify(opts.body) } : {}),
      ...(!kind && opts.signal ? { signal: opts.signal } : {}),
      ...(opts.cache ? { cache: opts.cache } : {}),
    });
  };

  const request = async (): Promise<unknown> => {
    assertActor(lease);
    if (kind === "refresh") {
      const result = await refreshSession(lease);
      if (result.kind === "failed") throw result.error;
      assertActor(lease);
      if (opts.signal?.aborted) throw new DOMException("Aborted", "AbortError");
      return result.data;
    }
    let sentWith = authStore.getAccessToken();
    let response = await send(sentWith);
    if (response.status === 401 && !isAuthEntryPoint(path as string)) {
      sentWith = await tokenAfterRefresh(lease, sentWith, () => toApiError(response));
      response = await send(sentWith);
      if (response.status === 401) {
        loseSession(sentWith, lease);
        throw await toApiError(response);
      }
    }
    if (!response.ok) throw noticed(await toApiError(response), lease);
    if (response.status === 204 || response.headers.get("Content-Length") === "0") {
      assertActor(lease);
      return undefined;
    }
    const data: unknown = await response.json();
    assertActor(lease);
    if (opts.signal?.aborted) throw new DOMException("Aborted", "AbortError");
    return data;
  };
  return kind && kind !== "refresh"
    ? cookieOperation(kind, lease.generation, request, opts.transition)
    : request();
}

/**
 * UploadOptions are an upload's progress and cancellation, which fetch cannot
 * report, and its method, which is POST unless `method` says PUT.
 */
export interface UploadOptions {
  onProgress?: (fraction: number) => void;
  signal?: AbortSignal;
  method?: "POST" | "PUT";
}

/**
 * uploadFile uploads one file as multipart/form-data, sharing this module's
 * token and single-flight refresh. Like `api`, it loses the session over a 401
 * only when the store still holds the token that request was sent with.
 */
export async function uploadFile<T>(
  path: string,
  file: File,
  options: UploadOptions = {},
): Promise<T> {
  const url = buildUrl(path);
  const lease = authStore.captureActor();

  const send = (token: string | null) =>
    new Promise<{ status: number; body: string }>((resolve, reject) => {
      const request = new XMLHttpRequest();
      request.open(options.method ?? "POST", url);
      request.withCredentials = true;
      request.setRequestHeader("Accept", "application/json");
      request.setRequestHeader("Accept-Language", language());
      if (token) request.setRequestHeader("Authorization", `Bearer ${token}`);

      if (options.onProgress) {
        request.upload.onprogress = (event) => {
          if (event.lengthComputable && authStore.isCurrent(lease))
            options.onProgress?.(event.loaded / event.total);
        };
      }
      request.onload = () =>
        resolve({ status: request.status, body: request.responseText });
      request.onerror = () => reject(unavailable());
      request.onabort = () => reject(new DOMException("Aborted", "AbortError"));

      if (options.signal) {
        if (options.signal.aborted) {
          request.abort();
          return;
        }
        options.signal.addEventListener("abort", () => request.abort(), { once: true });
      }

      const form = new FormData();
      form.append("file", file);
      request.send(form);
    });

  let sentWith = authStore.getAccessToken();
  let response = await send(sentWith);
  if (response.status === 401) {
    sentWith = await tokenAfterRefresh(
      lease,
      sentWith,
      () => Promise.resolve(toUploadError(response)),
      false,
    );
    response = await send(sentWith);
    if (response.status === 401) {
      loseSession(sentWith, lease);
      throw toUploadError(response);
    }
  }
  if (response.status < 200 || response.status >= 300)
    throw noticed(toUploadError(response), lease);

  assertActor(lease);
  return JSON.parse(response.body) as T;
}

function toUploadError(response: { status: number; body: string }): ApiError {
  try {
    const parsed = JSON.parse(response.body) as {
      error?: {
        code?: string;
        message?: string;
        requestId?: string;
        details?: Record<string, unknown>;
      };
    };
    if (parsed.error?.code && parsed.error.message) {
      return new ApiError({
        status: response.status,
        code: parsed.error.code as ApiErrorCode,
        message: parsed.error.message,
        requestId: parsed.error.requestId,
        details: parsed.error.details,
      });
    }
  } catch {
    // A non-JSON body means a proxy or a crash, not our error envelope.
  }
  return new ApiError({
    status: response.status,
    code: "UNKNOWN",
    message: i18n.t("api.failed"),
  });
}

export { ApiError };
