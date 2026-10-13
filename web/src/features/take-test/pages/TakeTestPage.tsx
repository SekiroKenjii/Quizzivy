import { useIntegrityAutoSubmit } from "@/features/integrity/useIntegrityAutoSubmit";
import { AutoSubmitNotice } from "@/features/integrity/components/AutoSubmitNotice";
import {
  useCallback,
  useEffect,
  useEffectEvent,
  useMemo,
  useReducer,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { LoadError } from "@/components/shared/ListState";
import { cn } from "@/lib/utils";
import { EngineHeader, LeaveButton } from "../components/EngineHeader";
import { LeaveDialog } from "../components/LeaveDialog";
import { Navigator, type DotState, type KeysHint } from "../components/Navigator";
import { GroupContext, type DrawerPane } from "../components/GroupContext";
import { DrawerBackdrop, DrawerHandle, DrawerTip } from "../components/StimulusDrawer";
import type { MaterialGap } from "@/components/shared/content/GroupMaterials";
import { hasReading } from "@/components/shared/content/materials";
import { QuestionCard } from "../components/QuestionCard";
import { SaveStrip } from "../components/SaveState";
import { SectionRail } from "../components/SectionRail";
import { SubmitDialog } from "../components/SubmitDialog";
import { SubmittedScreen } from "../components/SubmittedScreen";
import { releaseSession } from "@/features/integrity/release";
import { forgetAllPlaces } from "@/features/media/playback";
import { FullscreenBar } from "@/features/integrity/components/FullscreenBar";
import { exitFullscreen } from "@/features/integrity/fullscreen";
import { StrikeDialog } from "@/features/integrity/components/StrikeDialog";
import { StrikeIndicator } from "@/features/integrity/components/StrikeIndicator";
import { strikeState } from "@/features/integrity/strikes";
import { useClipboardNotice } from "@/features/integrity/useClipboardNotice";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { progress } from "../answered";
import { getAttempt, type Answer, type StudentQuestion } from "../api";
import { groupBySection, sectionAt, sectionHead, type SectionHead } from "../sections";
import {
  clearOpenQuestion,
  readOpenQuestion,
  writeOpenQuestion,
} from "../openQuestion";
import {
  hasSharedContent,
  questionShowing,
  useKeptScroll,
  useSharedAudioPlaying,
  useStimulusDrawer,
} from "../panes";
import { PaperSourceContext, useLivePaperSource, usePaper } from "../paper";
import { questionKind } from "../questionType";
import { useTakeTestStore } from "../store";
import { useLeave } from "../useLeave";
import { useEngineWidthAtLeast } from "../width";

const PICK_KEYS = "abcde";
const WIDE_PX = 768;
const RAIL_PX = 900;
const RAIL_BESIDE_SHARED_PX = 1180;
const SHORT_LABEL_PX = 420;

/**
 * TakeTestPage is the engine, one question at a time, with the navigator as
 * the footer of the question pane and, on a wide enough screen, the sections
 * rail at the paper's left edge, and the deck's Submit dialog over it, which
 * is the only way to hand the paper in. The header is the deck's at every
 * width; below 768px a strip under it says when a save has failed or the
 * device is offline, and is otherwise absent. The keys are the deck's: the
 * arrows move and stop at either end, A to E choose, F flags. None acts in a
 * text field, while a dialog or the question sheet is open, or on a locked
 * paper. Once the attempt is submitted, by the student, the timer or an
 * auto-submit, the page shows the submitted screen and leaves the fullscreen
 * the assignment asked for. From then on no fullscreen change is recorded, so
 * that exit is never noted as the student's.
 */
export default function TakeTestPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { attemptId } = useParams<{ attemptId: string }>();
  const wide = useEngineWidthAtLeast(WIDE_PX);

  const [status, setStatus] = useState<"loading" | "ready" | "failed">("loading");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [index, setIndex] = useState(0);
  const [submitAsked, setSubmitAsked] = useState(false);
  const [jumps, countJump] = useReducer((n: number) => n + 1, 0);

  const questions = useTakeTestStore((s) => s.questions);
  const answers = useTakeTestStore((s) => s.answers);
  const flags = useTakeTestStore((s) => s.flags);
  const sessionId = useTakeTestStore((s) => s.sessionId);
  const beaconToken = useTakeTestStore((s) => s.beaconToken);
  const integrity = useTakeTestStore((s) => s.integrity);
  const focusLossCount = useTakeTestStore((s) => s.focusLossCount);
  const lock = useTakeTestStore((s) => s.lock);
  const submitState = useTakeTestStore((s) => s.submitState);
  const submitReason = useTakeTestStore((s) => s.submitReason);
  const submittedAt = useTakeTestStore((s) => s.submittedAt);
  const hydrate = useTakeTestStore((s) => s.hydrate);
  const setAnswer = useTakeTestStore((s) => s.setAnswer);
  const toggleFlag = useTakeTestStore((s) => s.toggleFlag);
  const reset = useTakeTestStore((s) => s.reset);
  const leave = useLeave();
  const paper = useLivePaperSource();

  const [reloads, reload] = useReducer((n: number) => n + 1, 0);
  const question = questions[Math.min(index, questions.length - 1)];
  const ended = submitState === "done";

  const { strikes, fullscreen } = useIntegrityMonitor({
    attemptId: attemptId ?? null,
    sessionId,
    beaconToken,
    policy:
      ended && integrity !== null
        ? { ...integrity, requireFullscreen: false }
        : integrity,
    questionId: question?.id ?? null,
  });

  const autoSubmitting = useIntegrityAutoSubmit(focusLossCount + strikes);
  useClipboardNotice(
    attemptId !== undefined && sessionId !== null && integrity?.blockCopyPaste === true,
  );

  const sealed = lock === "superseded" || lock === "closed";
  if (sealed && submitAsked) setSubmitAsked(false);

  const asksFullscreen = integrity?.requireFullscreen === true;
  useEffect(() => {
    if (ended && asksFullscreen) void exitFullscreen();
  }, [ended, asksFullscreen]);

  const restoredFor = useRef<string | null>(null);
  useEffect(() => {
    if (attemptId === undefined) return;
    const abort = new AbortController();
    getAttempt(attemptId, abort.signal)
      .then((session) => {
        if (abort.signal.aborted) return;
        hydrate(session);
        if (restoredFor.current !== attemptId) {
          restoredFor.current = attemptId;
          const stored = readOpenQuestion(attemptId);
          const at = session.questions.findIndex((item) => item.id === stored);
          if (at >= 0) setIndex(at);
        }
        setStatus("ready");
      })
      .catch((cause: unknown) => {
        if (abort.signal.aborted) return;
        setLoadError(cause);
        setStatus("failed");
      });
    return () => {
      abort.abort();
    };
  }, [attemptId, reloads, hydrate]);
  useEffect(
    () => () => {
      if (attemptId !== undefined) {
        const state = useTakeTestStore.getState();
        const mine = state.attemptId === attemptId;
        releaseSession({
          attemptId,
          sessionId: mine ? state.sessionId : null,
          beaconToken: mine ? state.beaconToken : "",
          ended: mine && (state.submitState === "done" || state.lock === "closed"),
        });
      }
      reset({ keepDraft: true });
      forgetAllPlaces();
    },
    [attemptId, reset],
  );

  const openId = question?.id;
  const ready = status === "ready";
  useEffect(() => {
    if (attemptId === undefined) return;
    if (ended) clearOpenQuestion(attemptId);
    else if (ready && openId !== undefined) writeOpenQuestion(attemptId, openId);
  }, [attemptId, openId, ready, ended]);

  const handleShortcut = useEffectEvent((event: KeyboardEvent) => {
    if (lock !== null || submitState !== "idle" || question === undefined) return;
    if (
      event.defaultPrevented ||
      event.isComposing ||
      event.metaKey ||
      event.ctrlKey ||
      event.altKey ||
      typingIn(event.target)
    )
      return;
    if (
      document.querySelector(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
      )
    )
      return;
    switch (event.key) {
      case "ArrowRight":
        event.preventDefault();
        setIndex(Math.min(questions.length - 1, index + 1));
        return;
      case "ArrowLeft":
        event.preventDefault();
        setIndex(Math.max(0, index - 1));
        return;
      case "f":
      case "F":
        event.preventDefault();
        if (!event.repeat && questionShowing(question.id)) toggleFlag(question.id);
        return;
    }
    const pick = PICK_KEYS.indexOf(event.key.toLowerCase());
    const option = question.options?.[pick];
    if (
      pick >= 0 &&
      option !== undefined &&
      !event.repeat &&
      questionKind(question) === "choice" &&
      questionShowing(question.id)
    ) {
      event.preventDefault();
      setAnswer(question.id, chooseOption(question, answers[question.id], option.id));
    }
  });
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => handleShortcut(event);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const jump = useCallback((i: number) => {
    setIndex(i);
    setSubmitAsked(false);
    countJump();
  }, []);

  if (status === "loading") {
    return <Notice>{t("takeTest.loading")}</Notice>;
  }
  if (status === "failed") {
    return (
      <main className="mx-auto w-full max-w-[720px] space-y-3 px-4 py-16">
        <LoadError error={loadError} onRetry={reload}>
          {t("takeTest.loadFailed")}
        </LoadError>
        <Button variant="ghost" size="sm" onClick={() => void navigate("/app")}>
          {t("takeTest.backHome")}
        </Button>
      </main>
    );
  }
  if (question === undefined) {
    return <Notice>{t("takeTest.empty")}</Notice>;
  }

  const dots = dotsOf(questions, answers, flags);

  if (submitState === "done" && submittedAt !== null) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        {wide && (
          <PaperSourceContext value={paper}>
            <EngineHeader wide leading={null} live={false} />
          </PaperSourceContext>
        )}
        <SubmittedScreen
          reason={submitReason ?? "manual"}
          submittedAt={submittedAt}
          answered={dots.filter((d) => d.answered).length}
          total={dots.length}
          onHome={() => void navigate("/app", { replace: true })}
          onResult={() =>
            void navigate(`/app/attempts/${attemptId}/result`, { replace: true })
          }
        />
      </div>
    );
  }

  if (autoSubmitting) return <AutoSubmitNotice />;

  // The server's count from before this sitting plus what this tab has seen since.
  const watching = integrity !== null && lock === null;
  const strikeStatus = watching
    ? strikeState(integrity, focusLossCount + strikes)
    : null;
  const strikeDialog = strikeStatus !== null && (
    <StrikeDialog state={strikeStatus} strikes={strikes} />
  );
  const strikeIndicator =
    strikeStatus === null || strikeStatus.limit === null ? null : (
      <StrikeIndicator state={strikeStatus} />
    );

  return (
    <>
      <PaperSourceContext value={paper}>
        <Paper
          attemptId={attemptId}
          index={index}
          jumps={jumps}
          status={strikeIndicator}
          fullscreenBar={watching && integrity.requireFullscreen && !fullscreen}
          onLeave={leave.ask}
          onMove={setIndex}
          onJump={jump}
          onSubmit={() => setSubmitAsked(true)}
          onReload={reload}
        />
      </PaperSourceContext>
      <SubmitDialog
        open={submitAsked}
        dots={dots}
        onClose={() => setSubmitAsked(false)}
        onGo={jump}
      />
      {strikeDialog}
      <LeaveDialog leave={leave} />
    </>
  );
}

function dotsOf(
  questions: StudentQuestion[],
  answers: Readonly<Record<string, Answer>>,
  flags: ReadonlySet<string>,
): DotState[] {
  return questions.map((q) => {
    const done = progress(q, answers[q.id]);
    return {
      id: q.id,
      answered: done === "full",
      partly: done === "partly",
      flagged: flags.has(q.id),
    };
  });
}

/**
 * PaperProps is what the host of a paper decides: the attempt, under which
 * the drawers a student has opened are remembered (none, in memory only),
 * the open question, the strike count, the fullscreen bar, and what each
 * control does.
 */
export type PaperProps = Readonly<{
  attemptId?: string | undefined;
  index: number;
  jumps: number;
  status: ReactNode;
  fullscreenBar: boolean;
  onLeave: () => void;
  onMove: (index: number) => void;
  onJump: (index: number) => void;
  onSubmit: () => void;
  onReload: () => void;
}>;

/**
 * Paper is the engine's paper open at question `index`: the header, the
 * sections rail, the stimulus pane and the question pane with the navigator.
 * The question pane is on the sidebar surface, its column at most 600px,
 * beside shared content from 768px, and on the page's background, its column
 * at most 720px, alone or on a phone. Above every question the column states
 * the section's title and instructions, as sectionHead decides.
 * The rail shows when the engine is at least 900px wide on a question with no
 * shared content and at least 1180px on one with it, and the footer then
 * names the keys; otherwise the footer's grid button opens the question
 * sheet, and its label takes the short form beside shared content from 768px
 * and below 420px. It reads the paper from the nearest PaperSourceContext and
 * the width from useEngineWidthAtLeast, and draws nothing for a paper with no
 * questions.
 */
export function Paper(props: PaperProps) {
  const { questions } = usePaper();
  const question = questions[Math.min(props.index, questions.length - 1)];
  if (question === undefined) return null;
  return <PaperAt {...props} question={question} />;
}

function PaperAt({
  attemptId,
  index,
  jumps,
  question,
  status,
  fullscreenBar,
  onLeave,
  onMove,
  onJump,
  onSubmit,
  onReload,
}: PaperProps & Readonly<{ question: StudentQuestion }>) {
  const { t } = useTranslation();
  const wide = useEngineWidthAtLeast(WIDE_PX);
  const railFits = useEngineWidthAtLeast(RAIL_PX);
  const railFitsBesideShared = useEngineWidthAtLeast(RAIL_BESIDE_SHARED_PX);
  const narrow = !useEngineWidthAtLeast(SHORT_LABEL_PX);
  const {
    questions,
    sections,
    groups: sharedGroups,
    answers,
    flags,
    lock,
  } = usePaper();
  const sealed = lock === "superseded" || lock === "closed";
  const submit = sealed ? undefined : onSubmit;
  const groups = useMemo(
    () => groupBySection(sections, questions),
    [sections, questions],
  );
  const group = sectionAt(groups, index);
  const dots = dotsOf(questions, answers, flags);
  const total = questions.length;
  const answerPanel = useRef<HTMLDivElement>(null);
  const contexts = useMemo(
    () =>
      new Map(
        sharedGroups.flatMap((context) =>
          context.questionIds.map((id) => [id, context] as const),
        ),
      ),
    [sharedGroups],
  );
  const numbers = useMemo(
    () => new Map(questions.map((item, i) => [item.id, i + 1])),
    [questions],
  );
  const context = contexts.get(question.id);
  const passage = hasSharedContent(context) ? context : undefined;
  const visit = `${question.id}:${jumps}`;
  const drawerMode = !wide && passage !== undefined;
  const panes = useRef<HTMLElement>(null);
  const playing = useSharedAudioPlaying(panes);
  const {
    open: drawerOpen,
    out: drawerOut,
    offset: drawerOffset,
    progress: drawerProgress,
    fresh,
    show: showDrawer,
    close: closeDrawer,
    dismiss,
    handlers: swipeHandlers,
    handleRef,
    closeRef,
  } = useStimulusDrawer({
    root: panes,
    enabled: drawerMode,
    visit,
    groupId: passage?.id,
    attemptId,
  });
  const sheet = useRef<HTMLDivElement>(null);
  const onSheetScroll = useKeptScroll(sheet, false, question.id);
  const landOn = useLanding(visit, sheet, answerPanel);

  const jumpToGap = (gap: MaterialGap) => {
    const landing = gapLanding(gap, questions, numbers);
    if (landing === null) return;
    dismiss();
    landOn(landing.target);
    onJump(landing.index);
  };
  const drawer: DrawerPane | undefined = drawerMode
    ? {
        open: drawerOpen,
        offset: drawerOffset,
        closeRef,
        onClose: closeDrawer,
      }
    : undefined;
  const kind = passage?.stimuli.some(hasReading) === false ? "recording" : "passage";
  const head = sectionHead(groups, group);
  const besideShared = wide && passage !== undefined;
  const rail = wide && (passage === undefined ? railFits : railFitsBesideShared);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <EngineHeader
        wide={wide}
        status={status}
        leading={<LeaveButton onClick={onLeave} />}
        onSubmit={submit}
      />
      <SaveStrip wide={wide} indicator={status} />
      {fullscreenBar && <FullscreenBar />}

      <main
        ref={panes}
        tabIndex={-1}
        aria-label={t("takeTest.dotLabel", { n: index + 1 })}
        className="relative flex min-h-0 min-w-0 flex-1 outline-none!"
        style={drawerMode ? { touchAction: "pan-y" } : undefined}
        {...swipeHandlers}
      >
        {rail && (
          <SectionRail dots={dots} current={index} groups={groups} onJump={onJump} />
        )}
        {passage !== undefined && (
          <GroupContext
            key={passage.id}
            group={passage}
            sectionTitle={group?.section.title}
            numbers={numbers}
            onGap={jumpToGap}
            onRetryMedia={onReload}
            wide={wide}
            drawer={drawer}
          />
        )}
        <section
          className={cn(
            "flex min-w-0 flex-[1_1_0] flex-col",
            besideShared ? "bg-sidebar" : "bg-bg",
          )}
        >
          <div
            ref={sheet}
            onScroll={onSheetScroll}
            className={cn(
              "relative min-h-0 flex-1 overflow-y-auto",
              wide ? "px-8 py-7" : "px-4 py-4.5",
              drawerMode && "touch-pan-y",
            )}
          >
            <div
              ref={answerPanel}
              id={`answer-question-${question.id}`}
              tabIndex={-1}
              aria-label={t("takeTest.dotLabel", { n: index + 1 })}
              className={cn(
                "mx-auto flex w-full flex-col gap-4.5 outline-none!",
                passage === undefined ? "max-w-180" : "max-w-150",
              )}
            >
              {head && <SectionHeader head={head} />}
              <QuestionCard
                question={question}
                number={index + 1}
                total={total}
                onAudioExpired={onReload}
              />
            </div>
          </div>
          <Navigator
            wide={wide}
            rail={rail}
            hint={keysHintOf(question, passage !== undefined)}
            short={narrow || (wide && passage !== undefined)}
            dots={dots}
            current={index}
            groups={groups}
            onMove={onMove}
            onJump={onJump}
            onFinish={submit}
          />
        </section>
        {drawerMode && drawerOut && (
          <DrawerBackdrop progress={drawerProgress} onClose={closeDrawer} />
        )}
        {drawerMode && !drawerOut && (
          <DrawerHandle
            ref={handleRef}
            kind={kind}
            playing={playing}
            nudge={fresh}
            onOpen={showDrawer}
          />
        )}
        {drawerMode && !drawerOut && fresh && <DrawerTip kind={kind} />}
      </main>
    </div>
  );
}

function SectionHeader({ head }: Readonly<{ head: SectionHead }>) {
  return (
    <div className="flex flex-col gap-0.5 border-b pb-3">
      {head.title !== "" && (
        <p className="text-muted-fg text-xs leading-normal font-semibold tracking-[0.04em] uppercase">
          {head.title}
        </p>
      )}
      {head.instructions !== "" && (
        <p className="text-muted-fg text-ui leading-normal text-pretty">
          {head.instructions}
        </p>
      )}
    </div>
  );
}

function keysHintOf(question: StudentQuestion, besideShared: boolean): KeysHint {
  const picks =
    questionKind(question) === "choice" && (question.options ?? []).length > 0;
  if (!picks) return "move";
  return besideShared ? "short" : "long";
}

function gapLanding(
  gap: MaterialGap,
  questions: StudentQuestion[],
  numbers: ReadonlyMap<string, number>,
): { index: number; target: string } | null {
  const number = numbers.get(gap.questionId);
  if (number === undefined) return null;
  const blank =
    gap.kind === "blank"
      ? questions[number - 1]?.blanks?.find((item) => item.gapId === gap.blankGapId)
      : undefined;
  return {
    index: number - 1,
    target: blank ? `answer-blank-${blank.id}` : `answer-question-${gap.questionId}`,
  };
}

function useLanding(
  visit: string,
  sheetRef: RefObject<HTMLDivElement | null>,
  panelRef: RefObject<HTMLDivElement | null>,
) {
  const target = useRef<string | null>(null);
  const [request, ask] = useReducer((n: number) => n + 1, 0);
  useEffect(() => {
    const id = target.current;
    target.current = null;
    if (id === null) {
      if (sheetRef.current) sheetRef.current.scrollTop = 0;
      panelRef.current?.focus({ preventScroll: true });
      return;
    }
    const element = document.getElementById(id) ?? panelRef.current;
    element?.focus({ preventScroll: true });
    element?.scrollIntoView?.({ block: "nearest" });
  }, [visit, request, sheetRef, panelRef]);
  return useCallback((id: string) => {
    target.current = id;
    ask();
  }, []);
}

function typingIn(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    (target instanceof HTMLInputElement &&
      target.type !== "radio" &&
      target.type !== "checkbox") ||
    target.tagName === "TEXTAREA" ||
    target.tagName === "SELECT" ||
    target.closest(
      '[role="slider"], [role="menu"], [role="listbox"], [role="combobox"]',
    ) !== null
  );
}

function chooseOption(
  question: StudentQuestion,
  current: Answer | undefined,
  optionId: string,
): Answer {
  if (question.type === "multiple_choice") {
    const chosen = new Set(current?.type === "choice" ? current.optionIds : []);
    if (!chosen.delete(optionId)) chosen.add(optionId);
    return { type: "choice", optionIds: [...chosen] };
  }
  return { type: "choice", optionIds: [optionId] };
}

function Notice({ children }: Readonly<{ children: string }>) {
  return (
    <main
      role="status"
      aria-live="polite"
      className="mx-auto w-full max-w-[720px] px-4 py-16 text-center"
    >
      <p className="text-muted-foreground text-sm leading-relaxed">{children}</p>
    </main>
  );
}
