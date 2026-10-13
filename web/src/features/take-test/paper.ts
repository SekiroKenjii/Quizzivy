import { createContext, useContext } from "react";
import type { Answer, StudentGroup, StudentQuestion, StudentSection } from "./api";
import type { SaveStatus } from "./saveStatus";
import type { LockReason, SubmitState } from "./store";

/**
 * PaperSource is everything the engine's paper reads and writes: the paper
 * itself, the answers and flags, the lock and the submit state, the save
 * line, the clock, the plays of each question's audio and of each shared
 * recording, and the calls that report a play to integrity. The paper and
 * every component under it read it through usePaper and never import a
 * store, so a host other than the live engine can fill it. `timeLeft` reads
 * the time left at the moment it is called, for a tick that must not use a
 * value captured at render.
 */
export interface PaperSource {
  title: string;
  questions: StudentQuestion[];
  sections: StudentSection[];
  groups: StudentGroup[];
  answers: Readonly<Record<string, Answer>>;
  setAnswer: (questionId: string, answer: Answer) => void;
  flags: ReadonlySet<string>;
  toggleFlag: (questionId: string) => void;
  lock: LockReason | null;
  submitState: SubmitState;
  saveStatus: SaveStatus;
  deadlineAt: number;
  offsetMs: number;
  timeLeft: () => number;
  questionPlays: Readonly<Record<string, number>>;
  onAudioPlay: (questionId: string) => void;
  onAudioEnded: (questionId: string) => void;
  onAudioBlocked: (questionId: string) => void;
  onAudioSeekBlocked: (questionId: string) => void;
  recordingPlays: Readonly<Record<string, number>>;
  pendingRecordings: ReadonlySet<string>;
  noteRecordingPlay: (recordingId: string) => void;
  flushRecordingPlays: () => void;
  onRecordingEnded: (recordingId: string, durationMs: number | undefined) => void;
  onRecordingBlocked: (recordingId: string) => void;
}

/**
 * PaperSourceContext carries the paper the engine shows. The take-test page
 * fills it with useLivePaperSource (`livePaper.ts`); it has no default, so a component
 * mounted outside a source fails loudly rather than showing an empty paper.
 */
export const PaperSourceContext = createContext<PaperSource | null>(null);

/** usePaper returns the paper source the nearest PaperSourceContext holds. */
export function usePaper(): PaperSource {
  const source = useContext(PaperSourceContext);
  if (source === null) throw new Error("usePaper needs a PaperSourceContext");
  return source;
}
