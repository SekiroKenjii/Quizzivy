import { createContext, useContext, type ReactNode } from "react";
import type { AssetNode } from "./ContentView";
import { placesAudio, validateQuestionProse, type ProseField } from "./questionContent";
import { contentPlainText } from "./plainText";

/**
 * ProseMediaRenderer draws a media block of a question's prompt or
 * explanation (DG-109) from the files its reader was signed: the engine's,
 * the result's or a staff reader's.
 */
export type ProseMediaRenderer = (node: AssetNode, field: ProseField) => ReactNode;

/** ProseMediaRendererContext hands a question's media renderer to QuestionProse and RichBlankPrompt. */
export const ProseMediaRendererContext = createContext<ProseMediaRenderer | null>(null);

/** useProseMediaRenderer is the nearest question's media renderer, or null where none is offered. */
export function useProseMediaRenderer(): ProseMediaRenderer | null {
  return useContext(ProseMediaRendererContext);
}

/**
 * marksAudio reports whether a prompt that will render places the question's
 * own audio itself (D-064), so the reader draws the recording at the marker
 * and not above the prompt.
 */
export function marksAudio(
  question: Readonly<{ prompt: string; promptContent?: unknown }>,
): boolean {
  if (question.promptContent == null) return false;
  const parsed = validateQuestionProse(question.promptContent, "prompt");
  return (
    parsed.ok &&
    placesAudio(parsed.value) &&
    contentPlainText(parsed.value) === question.prompt
  );
}
