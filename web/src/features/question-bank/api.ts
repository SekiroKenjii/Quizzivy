import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { QuestionValues } from "@/features/question-bank/questionSchema";

export type AdminQuestion = components["schemas"]["AdminQuestion"];
export type QuestionType = components["schemas"]["QuestionType"];

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
