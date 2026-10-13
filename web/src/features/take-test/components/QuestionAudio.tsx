import { useTranslation } from "react-i18next";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { usePaper } from "../paper";
import type { StudentQuestion } from "../api";

/**
 * QuestionAudio is the listening half of a question (§11.3, §11.4): the
 * recording card in the question pane. A start counts a play and records it;
 * a resume does neither. A play in progress keeps its place while the
 * student is on another question.
 */
export function QuestionAudio({
  question,
  onExpired,
}: Readonly<{
  question: StudentQuestion;
  /** Refetches the attempt, minting a fresh signed URL (§11.2). */
  onExpired: () => void;
}>) {
  const { t } = useTranslation();
  const {
    questionPlays,
    onAudioPlay,
    onAudioEnded,
    onAudioBlocked,
    onAudioSeekBlocked,
  } = usePaper();
  const played = questionPlays[question.id] ?? 0;

  if (question.media?.kind !== "audio") return null;

  return (
    <AudioPlayer
      layout="card"
      src={question.media.url}
      label={t("takeTest.audioLabel")}
      durationMs={question.media.durationMs}
      allowSeek={question.audio?.allowSeek ?? false}
      allowPause={question.audio?.allowPause}
      preload="metadata"
      plays={{ played, limit: question.audio?.maxPlays ?? null }}
      place={`question:${question.id}`}
      onPlay={() => onAudioPlay(question.id)}
      onEnded={() => onAudioEnded(question.id)}
      onBlocked={() => onAudioBlocked(question.id)}
      onSeekBlocked={() => onAudioSeekBlocked(question.id)}
      onRetry={onExpired}
    />
  );
}
