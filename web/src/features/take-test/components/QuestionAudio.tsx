import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { useTakeTestStore } from "../store";
import type { StudentQuestion } from "../api";

/**
 * The listening half of a question (§11.3, §11.4).
 *
 * The count on screen is the SERVER's. A client-side counter resets on reload,
 * which is what would make maxPlays mean nothing -- so this renders what the
 * payload carried, moves it optimistically when a play starts, and lets the
 * next fetch settle it.
 *
 * Nothing here can stop the audio. §11.4 is explicit that blocking playback on
 * a round trip punishes bad wifi far more often than it catches anyone, and a
 * student determined to replay can go offline anyway -- which leaves a gap in
 * the event log, which is what the teacher's timeline is for.
 */
export function QuestionAudio({
  question,
  onExpired,
}: {
  question: StudentQuestion;
  /** Refetches the attempt, minting a fresh signed URL (§11.2). */
  onExpired: () => void;
}) {
  const { t } = useTranslation();
  const played = useTakeTestStore((s) => s.audioPlays[question.id] ?? 0);
  const notePlay = useTakeTestStore((s) => s.notePlay);

  if (question.media?.kind !== "audio") return null;
  const maxPlays = question.audio?.maxPlays ?? null;

  return (
    <AudioPlayer
      src={question.media.url}
      label={t("takeTest.audioLabel")}
      durationMs={question.media.durationMs}
      allowSeek={question.audio?.allowSeek ?? false}
      preload="metadata"
      hint={playsHint(t, played, maxPlays)}
      onPlay={() => notePlay(question.id)}
      onRetry={onExpired}
    />
  );
}

/**
 * "Còn 2 lượt nghe", or nothing at all when the teacher set no limit.
 *
 * Never negative. Over-limit plays are real and are reported to the teacher
 * (§11.4), but counting down past zero on the student's screen would read as an
 * accusation in a place that cannot explain itself.
 */
function playsHint(
  t: TFunction,
  played: number,
  maxPlays: number | null,
): string | undefined {
  if (maxPlays === null) return undefined;
  return t("takeTest.playsLeft", { count: Math.max(0, maxPlays - played) });
}
