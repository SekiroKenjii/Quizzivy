import type { paths } from "./schema";
import { ApiError, toApiError, type ApiErrorCode } from "./errors";
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
      // Repeated key per element, which is what OpenAPI's `style: form,
      // explode: true` means and what the Go binder reads. String(array) would
      // send "a,b" as one value, and the server would look for a question type
      // literally called "a,b".
      if (Array.isArray(value)) {
        for (const item of value) {
          if (item === undefined || item === null) continue;
          url.searchParams.append(key, String(item));
        }
        continue;
      }
      url.searchParams.set(key, String(value));
    }
  }
  return url.toString();
}

// ------------------------------------------------------- single-flight refresh

let inFlightRefresh: Promise<boolean> | null = null;

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

/** Progress and cancellation for an upload, which fetch cannot report. */
export interface UploadOptions {
  onProgress?: (fraction: number) => void;
  signal?: AbortSignal;
}

/**
 * Uploads one file as multipart/form-data, sharing this module's token and
 * single-flight refresh.
 *
 * XMLHttpRequest rather than fetch, for the one thing fetch cannot do: report
 * how much of the body has been sent. §11.1 allows 10 MB, which is long enough
 * on a phone that a progress bar is the difference between "working" and
 * "broken".
 */
export async function uploadFile<T>(
  path: string,
  file: File,
  options: UploadOptions = {},
): Promise<T> {
  const url = buildUrl(path);

  const send = () =>
    new Promise<{ status: number; body: string }>((resolve, reject) => {
      const request = new XMLHttpRequest();
      request.open("POST", url);
      request.withCredentials = true;
      request.setRequestHeader("Accept", "application/json");
      const token = authStore.getAccessToken();
      if (token) request.setRequestHeader("Authorization", `Bearer ${token}`);

      if (options.onProgress) {
        request.upload.onprogress = (event) => {
          if (event.lengthComputable) options.onProgress?.(event.loaded / event.total);
        };
      }
      request.onload = () =>
        resolve({ status: request.status, body: request.responseText });
      request.onerror = () =>
        reject(
          new ApiError({
            status: 0,
            code: "UNKNOWN",
            message: "Không thể kết nối máy chủ.",
          }),
        );
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

  let response = await send();
  if (response.status === 401) {
    const refreshed = await refreshSession();
    if (!refreshed) {
      authStore.clear();
      onSessionLost();
      throw toUploadError(response);
    }
    response = await send();
    if (response.status === 401) {
      authStore.clear();
      onSessionLost();
      throw toUploadError(response);
    }
  }
  if (response.status < 200 || response.status >= 300) throw toUploadError(response);

  return JSON.parse(response.body) as T;
}

function toUploadError(response: { status: number; body: string }): ApiError {
  try {
    const parsed = JSON.parse(response.body) as {
      error?: { code?: string; message?: string; requestId?: string };
    };
    if (parsed.error?.code && parsed.error.message) {
      return new ApiError({
        status: response.status,
        code: parsed.error.code as ApiErrorCode,
        message: parsed.error.message,
        requestId: parsed.error.requestId,
      });
    }
  } catch {
    // A non-JSON body means a proxy or a crash, not our error envelope.
  }
  return new ApiError({
    status: response.status,
    code: "UNKNOWN",
    message: "Đã xảy ra lỗi. Vui lòng thử lại.",
  });
}

export { ApiError };
