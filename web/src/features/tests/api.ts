import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type Test = components["schemas"]["Test"];
export type TestSection = components["schemas"]["TestSection"];
export type TestStatus = components["schemas"]["TestStatus"];
export type TestVersion = components["schemas"]["TestVersion"];
export type PublishViolation = components["schemas"]["PublishValidationError"];

/** What autosave sends: the whole outline, guarded by the version it read. */
export interface OutlineDraft {
  expectedUpdatedAt: string;
  title: string;
  description: string | null;
  sections: {
    id: string | null;
    title: string;
    instructions: string | null;
    questionIds: string[];
  }[];
}

export function getTest(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/admin/tests/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function saveOutline(id: string, draft: OutlineDraft) {
  return api("patch", "/admin/tests/{id}", { path: { id }, body: draft });
}

export function publishTest(id: string) {
  return api("post", "/admin/tests/{id}/publish", { path: { id } });
}

export function toOutlineDraft(test: Test): Omit<OutlineDraft, "expectedUpdatedAt"> {
  return {
    title: test.title,
    description: test.description ?? null,
    sections: test.sections.map((section) => ({
      id: section.id,
      title: section.title,
      instructions: section.instructions ?? null,
      questionIds: section.questionIds,
    })),
  };
}
