import { isOptionContent } from "../optionContent";
import { isQuestionContent, isQuestionPromptContent } from "../questionContent";

export type EditorProfile = "document" | "option" | "question" | "prompt";

/** validEditorProfile checks a validated document against its field's narrower vocabulary. */
export function validEditorProfile(value: unknown, profile: EditorProfile): boolean {
  if (profile === "document") return true;
  if (profile === "option") return isOptionContent(value);
  if (profile === "prompt") return isQuestionPromptContent(value);
  return isQuestionContent(value);
}
