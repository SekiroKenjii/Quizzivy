import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

type Schemas = components["schemas"];

/** AiUsage is what one call cost and what is left, in credits. */
export type AiUsage = Schemas["AiUsage"];
/** AiMonthUsage is the signed-in teacher's month of AI credits. */
export type AiMonthUsage = Schemas["AiMonthUsage"];
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

/** AI_STATUS_KEY caches `GET /public/status` for the AI entry points. */
export const AI_STATUS_KEY = ["public-status"] as const;
/** AI_USAGE_KEY caches the signed-in teacher's month of AI credits. */
export const AI_USAGE_KEY = ["teacher-ai-usage"] as const;

/** readAiStatus reports whether the server has an AI assistant configured. */
export async function readAiStatus(signal?: AbortSignal): Promise<boolean> {
  const status = await api("get", "/public/status", signal ? { signal } : {});
  return status.ai;
}

/** getAiUsage reads the signed-in teacher's month of AI credits. */
export function getAiUsage(signal?: AbortSignal) {
  return api("get", "/teacher/ai/usage", signal ? { signal } : {});
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
