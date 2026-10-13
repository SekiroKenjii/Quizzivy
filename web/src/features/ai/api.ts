import { queryOptions } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";

type Schemas = components["schemas"];

/** AiUsage is what one call cost and what is left, in credits. */
export type AiUsage = Schemas["AiUsage"];
/** AiMonthUsage is the signed-in teacher's month of AI credits. */
export type AiMonthUsage = Schemas["AiMonthUsage"];
/** AiCreditCategory is one row of the Settings usage card. */
export type AiCreditCategory = Schemas["AiCreditCategory"];
/** AiCreditRequest is one request for more credits, as the server records it. */
export type AiCreditRequest = Schemas["AiCreditRequest"];
/** AiCreditRequestInput is the body of `POST /teacher/ai/credit-requests`. */
export type AiCreditRequestInput = Schemas["AiCreditRequestInput"];
/** AiLanguage is the language a suggestion writes for a person in. */
export type AiLanguage = Schemas["AiLanguage"];
/** AiGenerateRequest names one source and the settings of a generation. */
export type AiGenerateRequest = Schemas["AiGenerateRequest"];
/** AiGeneratedQuestion is one draft question the assistant wrote. */
export type AiGeneratedQuestion = Schemas["AiGeneratedQuestion"];
/** AiCheckKind is one thing a test check looks for. */
export type AiCheckKind = Schemas["AiCheckKind"];
/** AiFinding is one thing a test check found worth a look. */
export type AiFinding = Schemas["AiFinding"];

/** AiVariantKind is one kind of change a variant makes. */
export type AiVariantKind = Schemas["AiVariantKind"];
/** AiVariantChange is one proposed change to a question of a variant. */
export type AiVariantChange = Schemas["AiVariantChange"];
/** AiCleanupCheck is one thing a bank clean-up looks for. */
export type AiCleanupCheck = Schemas["AiCleanupCheck"];
/** AiDuplicate is a pair of the caller's questions that look alike. */
export type AiDuplicate = Schemas["AiDuplicate"];
/** AiCleanupResult is what one bank clean-up found. */
export type AiCleanupResult = Schemas["AiCleanupResult"];
/** AiTagSuggestion is the tags proposed for one untagged question. */
export type AiTagSuggestion = Schemas["AiTagSuggestion"];
/** AiScoreSuggestion is a proposed score and comment for one short answer, which saves nothing. */
export type AiScoreSuggestion = Schemas["AiScoreSuggestion"];

/**
 * AI_STATUS_KEY is the query key of `GET /public/status`, the whole answer,
 * which every reader shares and narrows with `select`.
 */
export const AI_STATUS_KEY = ["public-status"] as const;
/** AI_USAGE_KEY is the query key of the signed-in teacher's AI month, shared by the panels and Settings. */
export const AI_USAGE_KEY = ["ai-usage"] as const;

/**
 * publicStatusQuery reads `GET /public/status`. The route lets a browser keep
 * the answer for thirty seconds, and so does the query.
 */
export function publicStatusQuery() {
  return queryOptions({
    queryKey: AI_STATUS_KEY,
    queryFn: ({ signal }) => api("get", "/public/status", { signal }),
    staleTime: 30_000,
  });
}

/** aiUsageQuery reads the signed-in teacher's month of AI credits. */
export function aiUsageQuery() {
  return queryOptions({
    queryKey: AI_USAGE_KEY,
    queryFn: ({ signal }) => api("get", "/teacher/ai/usage", { signal }),
    staleTime: 30_000,
  });
}

/** isAiOff reports whether a failure is the server's `501 AI_OFF`, the assistant not configured. */
export function isAiOff(cause: unknown): boolean {
  return cause instanceof ApiError && (cause.code === "AI_OFF" || cause.status === 501);
}

/** requestAiCredits asks the caller's admin for more credits this month. */
export function requestAiCredits(body: AiCreditRequestInput): Promise<AiCreditRequest> {
  return api("post", "/teacher/ai/credit-requests", { body });
}

/** generateAiQuestions asks for draft questions and writes nothing. */
export function generateAiQuestions(body: AiGenerateRequest, signal?: AbortSignal) {
  return api(
    "post",
    "/teacher/ai/questions/generate",
    signal ? { body, signal } : { body },
  );
}

/** checkTestWithAi asks what is worth a look in a test's draft and writes nothing. */
export function checkTestWithAi(
  id: string,
  body: Schemas["AiCheckRequest"],
  signal?: AbortSignal,
) {
  return api(
    "post",
    "/teacher/ai/tests/{id}/check",
    signal ? { path: { id }, body, signal } : { path: { id }, body },
  );
}

/** makeTestVariant asks for a variant's changes to a test's draft and writes nothing. */
export function makeTestVariant(
  id: string,
  body: Schemas["AiVariantRequest"],
  signal?: AbortSignal,
) {
  return api(
    "post",
    "/teacher/ai/tests/{id}/variant",
    signal ? { path: { id }, body, signal } : { path: { id }, body },
  );
}

/** cleanUpQuestionBank asks for near-duplicates and missing tags in the caller's own questions and writes nothing. */
export function cleanUpQuestionBank(
  body: Schemas["AiCleanupRequest"],
  signal?: AbortSignal,
) {
  return api("post", "/teacher/ai/bank/cleanup", signal ? { body, signal } : { body });
}

/**
 * suggestAiScore asks for a score and a comment on one short answer of an
 * attempt and writes nothing: a teacher saves a mark with the grading write.
 */
export function suggestAiScore(
  attemptId: string,
  questionId: string,
  signal?: AbortSignal,
): Promise<AiScoreSuggestion> {
  const path = { attemptId, questionId };
  return api(
    "post",
    "/teacher/ai/attempts/{attemptId}/answers/{questionId}/score-suggestion",
    signal ? { path, signal } : { path },
  );
}
