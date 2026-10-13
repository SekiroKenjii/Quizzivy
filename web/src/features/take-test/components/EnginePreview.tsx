import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useState,
  type KeyboardEvent,
} from "react";
import { useTranslation } from "react-i18next";
import { Eye } from "lucide-react";
import { DeckScale } from "@/components/ui/deck-scale";
import { forgetAllPlaces } from "@/features/media/playback";
import { notify } from "@/lib/toast";
import { Paper } from "./Paper";
import type { Answer, StudentGroup, StudentQuestion, StudentSection } from "../api";
import { EngineLayerContext } from "../frame";
import { PaperSourceContext, type PaperSource } from "../paper";
import { paperKey } from "../shortcuts";
import { paperStops } from "../stops";

const NO_PLAYS: Readonly<Record<string, number>> = {};
const NO_RECORDINGS: ReadonlySet<string> = new Set();

function noTime() {
  return 0;
}

function ignore() {
  return undefined;
}

/**
 * EnginePreview is the take-test screen as a teacher previews it, inside the
 * frame of a preview: the student's paper, answerable, with the header's
 * preview chip and no Submit. It holds the paper and what is chosen on it in
 * its own state and keeps them only while it is mounted; it reads and writes
 * no store, saves nothing, records nothing for integrity, asks for no
 * fullscreen and counts no play. Finish on the last stop says that students
 * submit there. Its width is the frame's, through EngineWidthContext, and the
 * question sheet opens over it. The engine's keys act while the focus is
 * inside it. `onReload` refetches the paper when a recording's link has run
 * out; a host whose media cannot expire leaves it out.
 */
export function EnginePreview({
  title,
  questions,
  sections,
  groups,
  onReload = ignore,
}: Readonly<{
  title: string;
  questions: StudentQuestion[];
  sections: StudentSection[];
  groups: StudentGroup[];
  onReload?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  const [jumps, countJump] = useReducer((n: number) => n + 1, 0);
  const [answers, setAnswers] = useState<Readonly<Record<string, Answer>>>({});
  const [flags, setFlags] = useState<ReadonlySet<string>>(() => new Set());
  const [layer, setLayer] = useState<HTMLDivElement | null>(null);
  const stops = useMemo(() => paperStops(questions, groups), [questions, groups]);
  const open = Math.min(index, Math.max(0, questions.length - 1));
  const question = questions[open];

  const setAnswer = useCallback((questionId: string, answer: Answer) => {
    setAnswers((current) => ({ ...current, [questionId]: answer }));
  }, []);
  const toggleFlag = useCallback((questionId: string) => {
    setFlags((current) => {
      const next = new Set(current);
      if (!next.delete(questionId)) next.add(questionId);
      return next;
    });
  }, []);
  const jump = useCallback((at: number) => {
    setIndex(at);
    countJump();
  }, []);
  const finish = useCallback(() => {
    notify.info(t("takeTest.preview.finish"), {
      icon: <Eye aria-hidden="true" className="text-info-ink size-4" />,
    });
  }, [t]);

  useEffect(() => () => forgetAllPlaces(), []);

  const source = useMemo<PaperSource>(
    () => ({
      title,
      questions,
      sections,
      groups,
      answers,
      setAnswer,
      flags,
      toggleFlag,
      lock: null,
      submitState: "idle",
      saveStatus: "saved",
      deadlineAt: 0,
      offsetMs: 0,
      timeLeft: noTime,
      questionPlays: NO_PLAYS,
      onAudioPlay: ignore,
      onAudioEnded: ignore,
      onAudioBlocked: ignore,
      onAudioSeekBlocked: ignore,
      recordingPlays: NO_PLAYS,
      pendingRecordings: NO_RECORDINGS,
      noteRecordingPlay: ignore,
      flushRecordingPlays: ignore,
      onRecordingEnded: ignore,
      onRecordingBlocked: ignore,
    }),
    [title, questions, sections, groups, answers, setAnswer, flags, toggleFlag],
  );

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (question === undefined) return;
    if (
      event.currentTarget.querySelector(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
      )
    )
      return;
    const key = paperKey(event.nativeEvent, {
      stops,
      index: open,
      question,
      questions,
      answers,
    });
    if (key === null) return;
    event.preventDefault();
    if (key.kind === "flag") {
      if (key.questionId !== null) toggleFlag(key.questionId);
      return;
    }
    if (key.kind === "answer") setAnswer(key.questionId, key.answer);
    const next = key.kind === "open" ? key.index : key.open;
    if (next !== null) setIndex(next);
  };

  return (
    <DeckScale
      ref={setLayer}
      onKeyDown={onKeyDown}
      className="student-surface bg-bg text-fg @container/student relative flex h-full min-h-0 flex-col leading-relaxed [contain:layout]"
    >
      <EngineLayerContext value={layer}>
        <PaperSourceContext value={source}>
          <Paper
            preview
            index={open}
            jumps={jumps}
            status={null}
            fullscreenBar={false}
            onMove={setIndex}
            onJump={jump}
            onSubmit={finish}
            onReload={onReload}
          />
        </PaperSourceContext>
      </EngineLayerContext>
    </DeckScale>
  );
}
