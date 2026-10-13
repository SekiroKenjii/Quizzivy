import { createQuestion } from "@/features/question-bank/api";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import { createTest, saveMixedOutline } from "./api";

const TITLE_MAX = 200;

/**
 * PracticeNames are the words a practice test's writes need: its title, kept
 * to a test title's 200 characters, and the title of its one section.
 */
export interface PracticeNames {
  title: string;
  section: string;
}

/**
 * savePractice makes a new draft test of `questions` through the existing
 * writes and answers its id: each question is created in the bank with
 * `createQuestion`, then `createTest` makes the draft and its outline is saved
 * with one section holding them, in order. It changes no other test.
 */
export async function savePractice(
  questions: readonly QuestionValues[],
  names: PracticeNames,
): Promise<string> {
  const ids: string[] = [];
  for (const values of questions) ids.push((await createQuestion(values)).id);
  const title = names.title.slice(0, TITLE_MAX);
  const test = await createTest(title);
  await saveMixedOutline(test.id, test.updatedAt, title, [
    {
      id: null,
      title: names.section,
      instructions: null,
      units: ids.map((id) => ({ kind: "question" as const, id })),
    },
  ]);
  return test.id;
}
