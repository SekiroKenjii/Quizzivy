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
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import { EngineHeader, LeaveButton } from "../components/EngineHeader";
import { LeaveDialog } from "../components/LeaveDialog";
import { Navigator, type DotState } from "../components/Navigator";
import { GroupContext, GroupListening } from "../components/GroupContext";
import type { MaterialGap } from "@/components/shared/content/GroupMaterials";
import { QuestionCard } from "../components/QuestionCard";
import { SaveStrip } from "../components/SaveState";
import { SectionInstructions } from "../components/SectionInstructions";
import { SubmitDialog } from "../components/SubmitDialog";
import { SubmittedScreen } from "../components/SubmittedScreen";
import { clearSession } from "@/features/integrity/buffer";
import { FullscreenBar } from "@/features/integrity/components/FullscreenBar";
import { exitFullscreen } from "@/features/integrity/fullscreen";
import { StrikeDialog } from "@/features/integrity/components/StrikeDialog";
import { StrikeIndicator } from "@/features/integrity/components/StrikeIndicator";
import { strikeState } from "@/features/integrity/strikes";
import { useClipboardNotice } from "@/features/integrity/useClipboardNotice";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { answered } from "../answered";
import { getAttempt, type Answer, type StudentQuestion } from "../api";
import {
  groupBySection,
  opensSection,
  sectionAt,
  type SectionGroup,
} from "../sections";
import { hasPassage, questionShowing, useKeptScroll } from "../panes";
import { questionKind } from "../questionType";
import { useTakeTestStore } from "../store";
import { useLeave } from "../useLeave";

const PICK_KEYS = "abcde";

/**
 * TakeTestPage is the engine, one question at a time, with the navigator as
 * the footer of the question pane (a strip of numbered squares from 768px, a
 * button that opens the question sheet below it) and the deck's Submit dialog
 * over it, which is the only way to hand the paper in. The header is the
 * deck's at every width; below 768px a strip under it says when a save has
 * failed or the device is offline, and is otherwise absent. The keys are the
 * deck's: the arrows move and stop at either end, A to E choose, F flags.
 * None acts in a text field, while a dialog or the question sheet is open, or
 * on a locked paper. Once the attempt is submitted, by the student, the timer
 * or an auto-submit, the page shows the submitted screen and leaves the
 * fullscreen the assignment asked for. From then on no fullscreen change is
 * recorded, so that exit is never noted as the student's.
 */
export default function TakeTestPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { attemptId } = useParams<{ attemptId: string }>();
  const wide = useMediaQuery("(min-width: 768px)");

  const [status, setStatus] = useState<"loading" | "ready" | "failed">("loading");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [index, setIndex] = useState(0);
  const [submitAsked, setSubmitAsked] = useState(false);
  const [jumps, countJump] = useReducer((n: number) => n + 1, 0);

  const questions = useTakeTestStore((s) => s.questions);
  const sections = useTakeTestStore((s) => s.sections);
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

  const [reloads, reload] = useReducer((n: number) => n + 1, 0);
  const groups = useMemo(
    () => groupBySection(sections, questions),
    [sections, questions],
  );
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

  useEffect(() => {
    if (attemptId === undefined) return;
    const abort = new AbortController();
    getAttempt(attemptId, abort.signal)
      .then((session) => {
        if (abort.signal.aborted) return;
        hydrate(session);
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
      if (attemptId !== undefined) clearSession(attemptId);
      reset({ keepDraft: true });
    },
    [attemptId, reset],
  );

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

  const dots: DotState[] = questions.map((q) => ({
    id: q.id,
    answered: answered(q, answers[q.id]),
    flagged: flags.has(q.id),
  }));

  if (submitState === "done" && submittedAt !== null) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        {wide && <EngineHeader wide leading={null} live={false} />}
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
      <Paper
        wide={wide}
        index={index}
        jumps={jumps}
        question={question}
        total={questions.length}
        group={sectionAt(groups, index)}
        sectioned={groups.length > 1}
        dots={dots}
        groups={groups}
        status={strikeIndicator}
        fullscreenBar={watching && integrity.requireFullscreen && !fullscreen}
        onLeave={leave.ask}
        onMove={setIndex}
        onJump={jump}
        onSubmit={() => setSubmitAsked(true)}
        onReload={reload}
      />
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

function Paper({
  wide,
  index,
  jumps,
  question,
  total,
  group,
  sectioned,
  dots,
  groups,
  status,
  fullscreenBar,
  onLeave,
  onMove,
  onJump,
  onSubmit,
  onReload,
}: Readonly<{
  wide: boolean;
  index: number;
  jumps: number;
  question: StudentQuestion;
  total: number;
  group: SectionGroup | null;
  sectioned: boolean;
  dots: DotState[];
  groups: SectionGroup[];
  status: ReactNode;
  fullscreenBar: boolean;
  onLeave: () => void;
  onMove: (index: number) => void;
  onJump: (index: number) => void;
  onSubmit: () => void;
  onReload: () => void;
}>) {
  const { t } = useTranslation();
  const sealed = useTakeTestStore(
    (state) => state.lock === "superseded" || state.lock === "closed",
  );
  const submit = sealed ? undefined : onSubmit;
  const answerPanel = useRef<HTMLDivElement>(null);
  const sharedGroups = useTakeTestStore((state) => state.groups);
  const questions = useTakeTestStore((state) => state.questions);
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
  const passage = hasPassage(context) ? context : undefined;
  const visit = `${question.id}:${jumps}`;
  const [readingFor, setReadingFor] = useState<string | null>(null);
  if (readingFor !== null && readingFor !== visit) setReadingFor(null);
  const reading = !wide && passage !== undefined && readingFor === visit;
  const sheet = useRef<HTMLDivElement>(null);
  const onSheetScroll = useKeptScroll(sheet, reading, question.id);
  const landOn = useLanding(visit, sheet, answerPanel);

  const jumpToGap = useCallback(
    (gap: MaterialGap) => {
      const landing = gapLanding(gap, questions, numbers);
      if (landing === null) return;
      setReadingFor(null);
      landOn(landing.target);
      onJump(landing.index);
    },
    [numbers, questions, onJump, landOn],
  );

  const part = sectioned && passage === undefined ? group?.section.title : undefined;

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
      {!wide && passage !== undefined && (
        <PaneSwitch
          number={index + 1}
          reading={reading}
          onRead={(on) => setReadingFor(on ? visit : null)}
        />
      )}

      <main
        tabIndex={-1}
        aria-label={t("takeTest.dotLabel", { n: index + 1 })}
        className="flex min-h-0 min-w-0 flex-1 outline-none!"
      >
        {passage !== undefined && (
          <GroupContext
            key={passage.id}
            group={passage}
            eyebrow={group?.section.title}
            numbers={numbers}
            onGap={jumpToGap}
            onRetryMedia={onReload}
            wide={wide}
            hidden={!wide && !reading}
          />
        )}
        <section
          hidden={reading}
          className="bg-sidebar flex min-w-0 flex-[1_1_0] flex-col"
        >
          <div
            ref={sheet}
            onScroll={onSheetScroll}
            className={cn(
              "min-h-0 flex-1 overflow-y-auto",
              wide ? "px-8 py-7" : "px-4 py-4.5",
            )}
          >
            <div
              ref={answerPanel}
              id={`answer-question-${question.id}`}
              tabIndex={-1}
              aria-label={t("takeTest.dotLabel", { n: index + 1 })}
              className="mx-auto flex w-full max-w-150 flex-col gap-4.5 outline-none!"
            >
              {part ? (
                <p className="text-muted-fg text-meta leading-normal font-semibold tracking-[0.02em] uppercase">
                  {part}
                </p>
              ) : null}
              <QuestionCard
                question={question}
                number={index + 1}
                total={total}
                onAudioExpired={onReload}
                lead={
                  <>
                    {group !== null && opensSection(group, index) && (
                      <SectionInstructions
                        group={group}
                        audio={question.media?.kind === "audio"}
                      />
                    )}
                    {context && (
                      <GroupListening
                        key={context.id}
                        group={context}
                        onRetryMedia={onReload}
                      />
                    )}
                  </>
                }
              />
            </div>
          </div>
          <Navigator
            wide={wide}
            dots={dots}
            current={index}
            groups={groups}
            onMove={onMove}
            onJump={onJump}
            onFinish={submit}
          />
        </section>
      </main>
    </div>
  );
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

function PaneSwitch({
  number,
  reading,
  onRead,
}: Readonly<{
  number: number;
  reading: boolean;
  onRead: (reading: boolean) => void;
}>) {
  const { t } = useTranslation();
  return (
    <div
      role="group"
      aria-label={t("takeTest.paneSwitch")}
      className="flex flex-none gap-1.5 border-b px-3.5 py-2"
    >
      <PaneTab on={reading} onClick={() => onRead(true)}>
        {t("takeTest.passage")}
      </PaneTab>
      <PaneTab on={!reading} onClick={() => onRead(false)}>
        {t("takeTest.dotLabel", { n: number })}
      </PaneTab>
    </div>
  );
}

function PaneTab({
  on,
  onClick,
  children,
}: Readonly<{ on: boolean; onClick: () => void; children: string }>) {
  return (
    <button
      type="button"
      aria-pressed={on}
      className={cn(
        "text-ui h-8.5 min-h-0 min-w-0 flex-1 rounded-md leading-none font-medium",
        on ? "bg-primary text-primary-fg" : "bg-muted text-fg",
      )}
      onClick={onClick}
    >
      {children}
    </button>
  );
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
