import { useCallback, useMemo } from "react";
import {
  recordAudioEvent,
  recordSharedAudioEvent,
} from "@/features/integrity/useIntegrityMonitor";
import { groupPlayCount, useGroupPlaybackStore } from "./groupPlayback";
import type { PaperSource } from "./paper";
import { useSaveStatus } from "./saveStatus";
import { takeTestStore, useTakeTestStore } from "./store";
import { timeLeft } from "./timeLeft";

/**
 * useLivePaperSource is the live engine's paper: the take-test store, the
 * group playback store and the save status, with each audio callback making
 * the calls the engine has always made, counting the play and recording it
 * for integrity while an attempt is loaded.
 */
export function useLivePaperSource(): PaperSource {
  const attemptId = useTakeTestStore((s) => s.attemptId);
  const title = useTakeTestStore((s) => s.testTitle);
  const questions = useTakeTestStore((s) => s.questions);
  const sections = useTakeTestStore((s) => s.sections);
  const groups = useTakeTestStore((s) => s.groups);
  const answers = useTakeTestStore((s) => s.answers);
  const setAnswer = useTakeTestStore((s) => s.setAnswer);
  const flags = useTakeTestStore((s) => s.flags);
  const toggleFlag = useTakeTestStore((s) => s.toggleFlag);
  const lock = useTakeTestStore((s) => s.lock);
  const submitState = useTakeTestStore((s) => s.submitState);
  const deadlineAt = useTakeTestStore((s) => s.deadlineAt);
  const offsetMs = useTakeTestStore((s) => s.offsetMs);
  const questionPlays = useTakeTestStore((s) => s.audioPlays);
  const notePlay = useTakeTestStore((s) => s.notePlay);
  const saveStatus = useSaveStatus();
  const confirmed = useGroupPlaybackStore((s) => s.confirmed);
  const pending = useGroupPlaybackStore((s) => s.pending);
  const noteRecordingPlay = useGroupPlaybackStore((s) => s.notePlay);
  const flush = useGroupPlaybackStore((s) => s.flush);

  const readTimeLeft = useCallback(() => timeLeft(takeTestStore.getState()), []);
  const flushRecordingPlays = useCallback(() => void flush(), [flush]);
  const recordingPlays = useMemo(
    () =>
      Object.fromEntries(
        groups.flatMap((group) =>
          group.recordings.map((recording) => [
            recording.id,
            groupPlayCount({ confirmed, pending }, recording.id),
          ]),
        ),
      ),
    [groups, confirmed, pending],
  );
  const pendingRecordings = useMemo(
    () => new Set(pending.map((play) => play.recordingId)),
    [pending],
  );
  const audio = useMemo(() => {
    const report =
      (kind: "audio_play" | "audio_ended" | "audio_blocked" | "audio_seek") =>
      (questionId: string) => {
        if (attemptId !== null) recordAudioEvent(attemptId, kind, questionId);
      };
    const reportPlay = report("audio_play");
    return {
      onAudioPlay: (questionId: string) => {
        notePlay(questionId);
        reportPlay(questionId);
      },
      onAudioEnded: report("audio_ended"),
      onAudioBlocked: report("audio_blocked"),
      onAudioSeekBlocked: report("audio_seek"),
      onRecordingEnded: (recordingId: string, durationMs: number | undefined) => {
        if (attemptId !== null)
          recordSharedAudioEvent(attemptId, "audio_ended", recordingId, durationMs);
      },
      onRecordingBlocked: (recordingId: string) => {
        if (attemptId !== null)
          recordSharedAudioEvent(attemptId, "audio_blocked", recordingId);
      },
    };
  }, [attemptId, notePlay]);

  return useMemo(
    () => ({
      title,
      questions,
      sections,
      groups,
      answers,
      setAnswer,
      flags,
      toggleFlag,
      lock,
      submitState,
      saveStatus,
      deadlineAt,
      offsetMs,
      timeLeft: readTimeLeft,
      questionPlays,
      recordingPlays,
      pendingRecordings,
      noteRecordingPlay,
      flushRecordingPlays,
      ...audio,
    }),
    [
      title,
      questions,
      sections,
      groups,
      answers,
      setAnswer,
      flags,
      toggleFlag,
      lock,
      submitState,
      saveStatus,
      deadlineAt,
      offsetMs,
      readTimeLeft,
      questionPlays,
      recordingPlays,
      pendingRecordings,
      noteRecordingPlay,
      flushRecordingPlays,
      audio,
    ],
  );
}
