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
  readonly violations: components["schemas"]["PublishValidationError"][];

  constructor(init: {
    status: number;
    code: ApiErrorCode | "UNKNOWN";
    message: string;
    requestId?: string | undefined;
    details?: Record<string, unknown> | undefined;
    retryAfterSeconds?: number | undefined;
    violations?: components["schemas"]["PublishValidationError"][];
  }) {
    super(init.message);
    this.name = "ApiError";
    this.status = init.status;
    this.code = init.code;
    this.requestId = init.requestId;
    this.details = init.details;
    this.retryAfterSeconds = init.retryAfterSeconds;
    this.violations = init.violations ?? [];
  }

  get isRateLimited() {
    return this.status === 429;
  }

  /**
   * isRetryable says the same request may succeed if sent again: it got no
   * answer (status 0), was rate limited, or met a server-side fault.
   */
  get isRetryable() {
    return this.status === 0 || this.status === 429 || this.status >= 500;
  }
}

/**
 * maintenanceWindow returns the period a 503 `MAINTENANCE` names in its
 * details, or null for any other failure.
 */
export function maintenanceWindow(
  cause: unknown,
): { startsAt: string; endsAt: string } | null {
  if (!(cause instanceof ApiError) || cause.code !== "MAINTENANCE") return null;
  const startsAt = cause.details?.["startsAt"];
  const endsAt = cause.details?.["endsAt"];
  if (typeof startsAt !== "string" || typeof endsAt !== "string") return null;
  return { startsAt, endsAt };
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
      violations:
        e.code === "PUBLISH_VALIDATION_FAILED" &&
        "violations" in body &&
        Array.isArray(body.violations)
          ? body.violations.filter(
              (value): value is components["schemas"]["PublishValidationError"] =>
                typeof value === "object" &&
                value !== null &&
                typeof value.rule === "string" &&
                typeof value.message === "string",
            )
          : [],
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
