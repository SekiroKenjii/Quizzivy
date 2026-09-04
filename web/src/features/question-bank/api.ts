import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { QuestionValues } from "@/features/question-bank/questionSchema";

export type AdminQuestion = components["schemas"]["AdminQuestion"];
export type QuestionType = components["schemas"]["QuestionType"];

export interface ListQuestionsParams {
  /** Repeatable. Several types widen the results; see A-06's rail. */
  type?: QuestionType[];
  tag?: string[];
  hasAudio?: boolean;
  q?: string;
  page?: number;
  limit?: number;
}

export function listQuestions(params: ListQuestionsParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.type?.length) query["type"] = params.type;
  if (params.tag?.length) query["tag"] = params.tag;
  if (params.hasAudio !== undefined) query["hasAudio"] = params.hasAudio;
  if (params.q) query["q"] = params.q;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/admin/questions", signal ? { query, signal } : { query });
}

export function getQuestion(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/admin/questions/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function createQuestion(body: QuestionValues) {
  return api("post", "/admin/questions", { body });
}

export function updateQuestion(id: string, body: QuestionValues) {
  return api("patch", "/admin/questions/{id}", { path: { id }, body });
}

/**
 * The response shape and the form shape differ on purpose: the response embeds
 * the whole `media` asset, the request sends only its id. This is the one place
 * that knows both.
 */
export function toFormValues(question: AdminQuestion): QuestionValues {
  return {
    type: question.type,
    prompt: question.prompt,
    mediaAssetId: question.media?.id ?? null,
    audio: question.audio ?? null,
    transcript: question.transcript ?? null,
    options: (question.options ?? []).map((option) => ({
      id: option.id,
      text: option.text,
      isCorrect: option.isCorrect,
    })),
    blanks: (question.blanks ?? []).map((blank) => ({
      id: blank.id,
      ordinal: blank.ordinal,
      acceptedAnswers: blank.acceptedAnswers,
      caseSensitive: blank.caseSensitive ?? false,
    })),
    points: question.points,
    explanation: question.explanation ?? null,
    sampleAnswer: question.sampleAnswer ?? null,
    tags: question.tags,
  };
}

/** A-06's bulk "Gắn thẻ". Additive and idempotent; see the contract. */
export function tagQuestions(questionIds: string[], tags: string[]) {
  return api("post", "/admin/questions/tags", { body: { questionIds, tags } });
}

export function deleteQuestion(id: string) {
  return api("delete", "/admin/questions/{id}", { path: { id } });
}
