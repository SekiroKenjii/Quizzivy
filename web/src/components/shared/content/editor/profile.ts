import { questionGaps } from "../gaps";
import type { ContentValidation } from "../validation";
import { validateContent } from "../validation";
import { isOptionContent } from "../optionContent";
import {
  isGapProse,
  isQuestionContent,
  validateQuestionProse,
  type ProseField,
  type QuestionProseValidation,
} from "../questionContent";

/**
 * EditorProfile is a field's vocabulary. "document" is a group material,
 * "option" an option's one paragraph, "question" prose without gaps and
 * "prompt" prose with gaps. The three media profiles add DG-109's media: a
 * prompt without gaps ("questionPrompt"), a fill-in-the-blank prompt
 * ("blankPrompt") and an explanation.
 */
export type EditorProfile =
  | "document"
  | "option"
  | "question"
  | "prompt"
  | "questionPrompt"
  | "blankPrompt"
  | "explanation";

const MEDIA: Partial<Record<EditorProfile, ProseField>> = {
  questionPrompt: "prompt",
  blankPrompt: "prompt",
  explanation: "explanation",
};

/** mediaField names the question prose rules a profile follows, or null for a profile that holds no question media. */
export function mediaField(profile: EditorProfile): ProseField | null {
  return MEDIA[profile] ?? null;
}

/** holdsGaps reports whether a profile's documents may hold gaps. */
export function holdsGaps(profile: EditorProfile): boolean {
  return profile === "prompt" || profile === "document" || profile === "blankPrompt";
}

/** validateForProfile validates a candidate document by the rules its profile's field follows. */
export function validateForProfile(
  input: unknown,
  profile: EditorProfile,
): ContentValidation | QuestionProseValidation {
  const field = mediaField(profile);
  return field === null ? validateContent(input) : validateQuestionProse(input, field);
}

/** validEditorProfile checks a validated document against its field's narrower vocabulary. */
export function validEditorProfile(value: unknown, profile: EditorProfile): boolean {
  if (profile === "document") return true;
  if (profile === "option") return isOptionContent(value);
  if (profile === "prompt") return isGapProse(value);
  if (profile === "question") return isQuestionContent(value);
  const parsed = validateForProfile(value, profile);
  return (
    parsed.ok &&
    parsed.value.format === "semantic_v1" &&
    (holdsGaps(profile) || questionGaps(parsed.value).length === 0)
  );
}
