import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

export type GroupBundle = components["schemas"]["QuestionGroupBundle"];
export type StoredGroup = components["schemas"]["StoredQuestionGroup"];
export type GroupSummary = components["schemas"]["QuestionGroupSummary"];
export type GroupCreateInput = components["schemas"]["GroupCreateInput"];
export type GroupUpdateInput = components["schemas"]["GroupUpdateInput"];
export type GroupCopyInput = components["schemas"]["GroupCopyInput"];

export interface ListGroupsParams {
  q?: string;
  tag?: string;
  status?: "active" | "archived" | "all";
  page?: number;
  limit?: number;
}

export function listGroups(params: ListGroupsParams = {}, signal?: AbortSignal) {
  const query = { ...params };
  return api("get", "/admin/question-groups", signal ? { query, signal } : { query });
}

export function getGroup(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/admin/question-groups/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function createGroup(body: GroupCreateInput) {
  return api("post", "/admin/question-groups", { body });
}

export function updateGroup(id: string, body: GroupUpdateInput) {
  return api("put", "/admin/question-groups/{id}", { path: { id }, body });
}

export function copyGroup(id: string, body: GroupCopyInput) {
  return api("post", "/admin/question-groups/{id}/copy", { path: { id }, body });
}

export function archiveGroup(group: StoredGroup | GroupSummary, archived: boolean) {
  return api("patch", "/admin/question-groups/{id}/archive", {
    path: { id: "bundle" in group ? group.bundle.group.id : group.id },
    body: { expectedRevision: group.revision, archived },
  });
}

export function deleteGroup(
  id: string,
  expectedRevision: number,
  expectedTestUpdatedAt?: string,
) {
  return api("delete", "/admin/question-groups/{id}", {
    path: { id },
    query: {
      expectedRevision,
      ...(expectedTestUpdatedAt ? { expectedTestUpdatedAt } : {}),
    },
  });
}
