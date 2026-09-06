import type { components } from "./schema";

type ErrorEnvelope = components["schemas"]["ErrorResponse"];
export type ApiErrorCode = components["schemas"]["ErrorCode"];

/**
 * Every non-2xx response carries the envelope from docs/plan/00-overview.md §7.
 * `code` is the only thing callers branch on; `message` is already localised
 * server-side and is what the UI displays.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: ApiErrorCode | "UNKNOWN";
  readonly requestId: string | undefined;
  readonly details: Record<string, unknown> | undefined;
  readonly retryAfterSeconds: number | undefined;

  constructor(init: {
    status: number;
    code: ApiErrorCode | "UNKNOWN";
    message: string;
    requestId?: string | undefined;
    details?: Record<string, unknown> | undefined;
    retryAfterSeconds?: number | undefined;
  }) {
    super(init.message);
    this.name = "ApiError";
    this.status = init.status;
    this.code = init.code;
    this.requestId = init.requestId;
    this.details = init.details;
    this.retryAfterSeconds = init.retryAfterSeconds;
  }

  get isRateLimited() {
    return this.status === 429;
  }
}

/**
 * The per-field reasons behind a validation failure, if the response carried
 * any.
 */
/** What to show for a failed call: the server's own message, else the caller's fallback. */
export function failureMessage(cause: unknown, fallback: string): string {
  return cause instanceof ApiError ? cause.message : fallback;
}

export function fieldMessages(cause: unknown): string[] {
  if (!(cause instanceof ApiError) || !cause.details) return [];
  return Object.values(cause.details).filter(
    (value): value is string => typeof value === "string",
  );
}

export type ReferencingTest = components["schemas"]["ReferencingTest"];

/** The tests a QUESTION_REFERENCED or MEDIA_REFERENCED refusal names, if any. */
export function referencingTests(cause: unknown): ReferencingTest[] {
  if (!(cause instanceof ApiError)) return [];
  const tests = cause.details?.["tests"];
  if (!Array.isArray(tests)) return [];
  return tests.filter(
    (value): value is ReferencingTest =>
      typeof value === "object" &&
      value !== null &&
      typeof (value as ReferencingTest).id === "string" &&
      typeof (value as ReferencingTest).title === "string",
  );
}

function isEnvelope(value: unknown): value is ErrorEnvelope {
  return (
    typeof value === "object" &&
    value !== null &&
    "error" in value &&
    typeof (value as { error: unknown }).error === "object" &&
    (value as { error: unknown }).error !== null
  );
}

/**
 * Decode an error response. Never throws: a gateway returning HTML, or a
 * network-level failure, still has to produce something the UI can render.
 */
export async function toApiError(response: Response): Promise<ApiError> {
  const retryAfter = response.headers.get("Retry-After");
  const retryAfterSeconds = retryAfter ? Number(retryAfter) : undefined;

  let body: unknown;
  try {
    body = await response.json();
  } catch {
    body = undefined;
  }

  if (isEnvelope(body)) {
    const e = body.error;
    return new ApiError({
      status: response.status,
      code: e.code,
      message: e.message,
      requestId: e.requestId,
      details: e.details as Record<string, unknown> | undefined,
      retryAfterSeconds: Number.isFinite(retryAfterSeconds)
        ? retryAfterSeconds
        : undefined,
    });
  }

  return new ApiError({
    status: response.status,
    code: "UNKNOWN",
    message: `HTTP ${response.status}`,
    retryAfterSeconds: Number.isFinite(retryAfterSeconds)
      ? retryAfterSeconds
      : undefined,
  });
}
