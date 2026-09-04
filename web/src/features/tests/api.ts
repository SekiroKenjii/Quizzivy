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

export interface ListTestsParams {
  status?: TestStatus;
  /** Repeatable. Matches the tags of the questions a test contains (A-03). */
  tag?: string[];
  q?: string;
  page?: number;
  limit?: number;
}

export function listTests(params: ListTestsParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.status) query["status"] = params.status;
  if (params.tag?.length) query["tag"] = params.tag;
  if (params.q) query["q"] = params.q;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/admin/tests", signal ? { query, signal } : { query });
}

export function createTest(title: string) {
  return api("post", "/admin/tests", { body: { title } });
}

export function duplicateTest(id: string) {
  return api("post", "/admin/tests/{id}/duplicate", { path: { id } });
}

export function archiveTest(test: Test) {
  return api("patch", "/admin/tests/{id}", {
    path: { id: test.id },
    body: { expectedUpdatedAt: test.updatedAt, status: "archived" },
  });
}

/** A-03a: back as a draft. Publishing again is the publish endpoint's job, not this one's. */
export function restoreTest(test: Test) {
  return api("patch", "/admin/tests/{id}", {
    path: { id: test.id },
    body: { expectedUpdatedAt: test.updatedAt, status: "draft" },
  });
}

export function listVersions(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/admin/tests/{id}/versions",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function previewTest(id: string, version?: number, signal?: AbortSignal) {
  const query = version === undefined ? {} : { version };
  return api(
    "get",
    "/admin/tests/{id}/preview",
    signal ? { path: { id }, query, signal } : { path: { id }, query },
  );
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
