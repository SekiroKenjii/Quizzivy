import type { QuestionValues } from "@/features/question-bank/questionSchema";

export type AudioPolicy = NonNullable<QuestionValues["audio"]>;

/**
 * §11.1's authoring defaults, which the deck's A-05 requires to be visible
 * rather than implicit: a teacher who never opens the panel still ships a sane
 * listening question.
 */
export const DEFAULT_AUDIO_POLICY: AudioPolicy = {
  maxPlays: 2,
  allowSeek: false,
  showTranscriptAfterSubmit: true,
};
