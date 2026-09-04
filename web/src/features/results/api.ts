import { api } from "@/lib/api/client";
import type { components, paths } from "@/lib/api/schema";

export type AttemptResult =
  paths["/app/attempts/{id}/result"]["get"]["responses"][200]["content"]["application/json"];
export type ResultQuestion = components["schemas"]["ResultQuestion"];

export function getAttemptResult(attemptId: string, signal?: AbortSignal) {
  return api("get", "/app/attempts/{id}/result", {
    path: { id: attemptId },
    ...(signal ? { signal } : {}),
  });
}
