import { api } from "@/lib/api/client";
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

/** The already-signed-in half of §6.2. */
export function joinClass(joinCode: string): Promise<Class> {
  return api("post", "/app/classes/join", { body: { joinCode } });
}
