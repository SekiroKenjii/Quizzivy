import type { LibraryAsset, MediaAsset } from "@/features/media/api";
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
  allowPause: true,
};

/**
 * startingAudioPolicy is the play limit a newly attached recording starts
 * with: the file's own default from Media, else Twice (DG-69).
 */
export function startingAudioPolicy(asset: MediaAsset | LibraryAsset): AudioPolicy {
  const stored = ("defaultMaxPlays" in asset ? asset.defaultMaxPlays : null) ?? null;
  if (stored === null) return DEFAULT_AUDIO_POLICY;
  return { ...DEFAULT_AUDIO_POLICY, maxPlays: stored === 0 ? null : stored };
}
