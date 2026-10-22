import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import { formProse } from "@/components/shared/content/questionContent";

export type AdminQuestion = components["schemas"]["AdminQuestion"];
export type QuestionType = components["schemas"]["QuestionType"];

export interface ListQuestionsParams {
  /** Repeatable. Several types widen the results; see A-06's rail. */
  type?: QuestionType[];
  tag?: string[];
  level?: components["schemas"]["QuestionLevel"][];
  skill?: components["schemas"]["QuestionSkill"][];
  tagMatch?: "any" | "all";
  hasAudio?: boolean;
  q?: string;
  page?: number;
  limit?: number;
}

export function listQuestions(params: ListQuestionsParams = {}, signal?: AbortSignal) {
  const query: Record<string, unknown> = {};
  if (params.type?.length) query["type"] = params.type;
  if (params.tag?.length) query["tag"] = params.tag;
  if (params.level?.length) query["level"] = params.level;
  if (params.skill?.length) query["skill"] = params.skill;
  if (params.tagMatch !== undefined) query["tagMatch"] = params.tagMatch;
  if (params.hasAudio !== undefined) query["hasAudio"] = params.hasAudio;
  if (params.q) query["q"] = params.q;
  if (params.page && params.page > 1) query["page"] = params.page;
  if (params.limit) query["limit"] = params.limit;
  return api("get", "/teacher/questions", signal ? { query, signal } : { query });
}

export function getQuestion(id: string, signal?: AbortSignal) {
  return api(
    "get",
    "/teacher/questions/{id}",
    signal ? { path: { id }, signal } : { path: { id } },
  );
}

export function createQuestion(body: QuestionValues) {
  return api("post", "/teacher/questions", {
    body: { ...body, level: body.level ?? null, skill: body.skill ?? null },
  });
}

export function updateQuestion(id: string, body: QuestionValues) {
  return api("patch", "/teacher/questions/{id}", {
    path: { id },
    body: { ...body, level: body.level ?? null, skill: body.skill ?? null },
  });
}

/**
 * The response shape and the form shape differ on purpose: the response embeds
 * the whole `media` asset, the request sends only its id. This is the one place
 * that knows both.
 */
export function toFormValues(question: AdminQuestion): QuestionValues {
  return {
    type: question.type,
    level: question.level,
    skill: question.skill,
    prompt: question.prompt,
    promptContent: formProse(question.promptContent),
    explanationContent: formProse(question.explanationContent),
    mediaAssetId: question.media?.id ?? null,
    mediaAlt: question.mediaAlt ?? null,
    audio: question.audio ?? null,
    transcript: question.transcript ?? null,
    options: (question.options ?? []).map((option) => ({
      id: option.id,
      text: option.text,
      content: option.content ?? null,
      isCorrect: option.isCorrect,
    })),
    blanks: (question.blanks ?? []).map((blank) => ({
      id: blank.id,
      ordinal: blank.ordinal,
      gapId: blank.gapId ?? null,
      acceptedAnswers: blank.acceptedAnswers,
      caseSensitive: blank.caseSensitive ?? false,
    })),
    points: question.points,
    explanation: question.explanation ?? null,
    sampleAnswer: question.sampleAnswer ?? null,
    ...typeFields(question),
    tags: question.tags,
  };
}

/**
 * typeFields are the R6 settings a stored question carries, in the form's
 * shape: a matching item names its option by index, as the request does,
 * where the response names it by id. Each appears only on its own type.
 */
function typeFields(
  question: AdminQuestion,
): Pick<
  QuestionValues,
  "selectCount" | "maxWords" | "matchAllowReuse" | "matchItems" | "rubric"
> {
  switch (question.type) {
    case "multiple_choice":
      return question.selectCount == null ? {} : { selectCount: question.selectCount };
    case "short_answer":
      return question.maxWords == null ? {} : { maxWords: question.maxWords };
    case "essay":
      return {
        rubric: (question.rubric ?? []).map(({ id, label }) => ({ id, label })),
      };
    case "matching": {
      const ids = (question.options ?? []).map((option) => option.id);
      return {
        matchAllowReuse: question.matchAllowReuse ?? false,
        matchItems: (question.matchItems ?? []).map((item) => ({
          id: item.id,
          text: item.text,
          correctOption: ids.indexOf(item.correctOptionId),
        })),
      };
    }
    default:
      return {};
  }
}

/** A-06's bulk "Gắn thẻ". Additive and idempotent; see the contract. */
export function tagQuestions(questionIds: string[], tags: string[]) {
  return api("post", "/teacher/questions/tags", { body: { questionIds, tags } });
}

export function duplicateQuestion(id: string) {
  return api("post", "/teacher/questions/{id}/duplicate", { path: { id } });
}

export function deleteQuestion(id: string) {
  return api("delete", "/teacher/questions/{id}", { path: { id } });
}
