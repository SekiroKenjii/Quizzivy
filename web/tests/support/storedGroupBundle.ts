import type { components } from "@/lib/api/schema";

// storedGroupBundle projects submitted metadata into the strict stored response shape.
export function storedGroupBundle(
  bundle: components["schemas"]["QuestionGroupBundle"],
): components["schemas"]["StoredQuestionGroupBundle"] {
  return {
    ...bundle,
    questions: bundle.questions.map((question) => ({
      ...question,
      input: {
        ...question.input,
        level: question.input.level ?? null,
        skill: question.input.skill ?? null,
      },
    })),
  };
}
