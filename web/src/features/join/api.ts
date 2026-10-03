import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";

export type JoinPreview = {
  classId: string;
  className: string;
  teacherName: string;
};

export type Class = components["schemas"]["Class"];

/**
 * §6.2's confirm step. Public: the student has no account yet, which is the
 * whole reason this screen exists -- they see WHICH class they are joining
 * before they authenticate.
 */
export function previewJoinCode(
  joinCode: string,
  signal?: AbortSignal,
): Promise<JoinPreview> {
  return api(
    "post",
    "/join/preview",
    signal ? { body: { joinCode }, signal } : { body: { joinCode } },
  );
}

/**
 * lookupJoinCode is previewJoinCode for a caller that remembers the answer:
 * it resolves null when the server refuses the code, whatever its reason, so
 * a refusal can be kept beside a found class and is not asked about again.
 * Anything else that goes wrong still rejects.
 */
export async function lookupJoinCode(
  joinCode: string,
  signal?: AbortSignal,
): Promise<JoinPreview | null> {
  try {
    return await previewJoinCode(joinCode, signal);
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) return null;
    throw cause;
  }
}

/** The already-signed-in half of §6.2. */
export function joinClass(joinCode: string): Promise<Class> {
  return api("post", "/app/classes/join", { body: { joinCode } });
}
