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
} from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { ChevronLeft, ChevronRight, Flag, List } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { LoadError } from "@/components/shared/ListState";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import { EngineHeader, LeaveButton } from "../components/EngineHeader";
import { LeaveDialog } from "../components/LeaveDialog";
import { NavigatorRail, NavigatorSheet, type DotState } from "../components/Navigator";
import { GroupContext } from "../components/GroupContext";
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
import { useTakeTestStore } from "../store";
import { useLeave } from "../useLeave";
import { worth } from "../worth";

/**
 * TakeTestPage is the engine, one question at a time, with the navigator
 * beside it (a sheet in thumb range, a rail from 768px) and the deck's Submit
 * dialog over it, which is the only way to hand the paper in. The header is
 * the deck's at every width. From 768px there is no sticky footer and the two
 * buttons sit under the answer at their own width; below it a strip under the
 * header says when a save has failed or the device is offline, and is
 * otherwise absent. Once the attempt is submitted, by the student, the timer
 * or an auto-submit, the page shows the submitted screen and leaves the
 * fullscreen the assignment asked for.
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
  const [navOpen, setNavOpen] = useState(false);

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

  // Every §10 listener, in one place.
  const { strikes, fullscreen } = useIntegrityMonitor({
    attemptId: attemptId ?? null,
    sessionId,
    beaconToken,
    policy: integrity,
    questionId: question?.id ?? null,
  });

  const autoSubmitting = useIntegrityAutoSubmit(focusLossCount + strikes);
  useClipboardNotice(
    attemptId !== undefined && sessionId !== null && integrity?.blockCopyPaste === true,
  );

  const sealed = lock === "superseded" || lock === "closed";
  if (sealed && submitAsked) setSubmitAsked(false);

  const ended = submitState === "done";
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
    if (navOpen || lock !== null || submitState !== "idle" || question === undefined)
      return;
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
        if (index === questions.length - 1) setSubmitAsked(true);
        else setIndex(index + 1);
        return;
      case "ArrowLeft":
        event.preventDefault();
        setIndex(Math.max(0, index - 1));
        return;
      case "f":
      case "F":
        event.preventDefault();
        if (!event.repeat) toggleFlag(question.id);
        return;
    }
    const pick = "abcd".indexOf(event.key.toLowerCase());
    const option = question.options?.[pick];
    if (pick >= 0 && option !== undefined && !event.repeat) {
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
    setNavOpen(false);
    setSubmitAsked(false);
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
        question={question}
        total={questions.length}
        group={sectionAt(groups, index)}
        sectioned={groups.length > 1}
        dots={dots}
        groups={groups}
        status={strikeIndicator}
        fullscreenBar={watching && integrity.requireFullscreen && !fullscreen}
        navOpen={navOpen}
        onNavOpen={setNavOpen}
        onLeave={leave.ask}
        onMove={setIndex}
        onJump={jump}
        onReview={() => setSubmitAsked(true)}
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

/** The paper itself: header, strip, one question, the way to the next. */
function Paper({
  wide,
  index,
  question,
  total,
  group,
  sectioned,
  dots,
  groups,
  status,
  fullscreenBar,
  navOpen,
  onNavOpen,
  onLeave,
  onMove,
  onJump,
  onReview,
  onReload,
}: Readonly<{
  wide: boolean;
  index: number;
  question: StudentQuestion;
  total: number;
  group: SectionGroup | null;
  /** More than one part: the meta line names the part (S-08). */
  sectioned: boolean;
  dots: DotState[];
  groups: SectionGroup[];
  status: ReactNode;
  fullscreenBar: boolean;
  navOpen: boolean;
  onNavOpen: (open: boolean) => void;
  onLeave: () => void;
  onMove: (index: number) => void;
  onJump: (index: number) => void;
  onReview: () => void;
  onReload: () => void;
}>) {
  const { t } = useTranslation();
  const flagged = useTakeTestStore((s) => s.flags.has(question.id));
  const lock = useTakeTestStore((s) => s.lock);
  const toggleFlag = useTakeTestStore((s) => s.toggleFlag);
  const last = index >= total - 1;
  const choice = (question.options?.length ?? 0) > 0;
  const paper = useRef<HTMLElement>(null);
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
  const contextClasses = paperColumns(context !== undefined);
  const previousContext = useRef<string | undefined>(undefined);
  const focusTarget = useRef<string | null>(null);
  const [focusRequest, requestFocus] = useReducer((n: number) => n + 1, 0);
  const revealAnswerPanel = useEffectEvent(() => {
    if (!wide) answerPanel.current?.scrollIntoView?.({ block: "start" });
  });
  useEffect(() => {
    const target = focusTarget.current;
    focusTarget.current = null;
    if (target !== null) {
      const element = document.getElementById(target) ?? answerPanel.current;
      element?.focus({ preventScroll: true });
      element?.scrollIntoView?.({ block: "nearest" });
    } else if (context?.id && previousContext.current === context.id) {
      answerPanel.current?.focus({ preventScroll: true });
      revealAnswerPanel();
    } else if (paper.current) {
      paper.current.scrollTop = 0;
      paper.current.focus({ preventScroll: true });
    }
    previousContext.current = context?.id;
  }, [question.id, context?.id, focusRequest]);

  const jumpToGap = useCallback(
    (gap: MaterialGap) => {
      const number = numbers.get(gap.questionId);
      if (number === undefined) return;
      const targetQuestion = questions[number - 1];
      const blank =
        gap.kind === "blank"
          ? targetQuestion?.blanks?.find((item) => item.gapId === gap.blankGapId)
          : undefined;
      focusTarget.current = blank
        ? `answer-blank-${blank.id}`
        : `answer-question-${gap.questionId}`;
      requestFocus();
      onJump(number - 1);
    },
    [numbers, questions, onJump],
  );

  const flag = (
    <Button
      variant="ghost"
      size={wide ? "sm" : "icon-sm"}
      className="text-muted-foreground min-h-11 min-w-11 shrink-0 lg:min-h-0 lg:min-w-0"
      aria-pressed={flagged}
      aria-label={t(flagged ? "takeTest.unflagThis" : "takeTest.flagThis")}
      disabled={lock !== null}
      onClick={() => toggleFlag(question.id)}
    >
      <Flag className={flagged ? "fill-current" : undefined} aria-hidden="true" />
      {wide && t("takeTest.flag")}
    </Button>
  );
  const previous = (
    <Button
      variant="outline"
      size={wide ? "default" : "icon"}
      className={wide ? undefined : "size-11"}
      aria-label={wide ? undefined : t("takeTest.previous")}
      disabled={index === 0}
      onClick={() => onMove(Math.max(0, index - 1))}
    >
      <ChevronLeft aria-hidden="true" />
      {wide && t("takeTest.previous")}
    </Button>
  );
  const next = last ? (
    <Button
      className={wide ? undefined : "h-11 min-w-0 flex-1 px-3 whitespace-normal"}
      onClick={onReview}
    >
      {t("takeTest.reviewAndSubmit")}
    </Button>
  ) : (
    <Button
      className={wide ? undefined : "h-11 min-w-0 flex-1 px-3 whitespace-normal"}
      onClick={() => onMove(index + 1)}
    >
      {t("takeTest.next")}
      <ChevronRight aria-hidden="true" />
    </Button>
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <EngineHeader
        wide={wide}
        status={status}
        leading={<LeaveButton onClick={onLeave} />}
        onSubmit={lock === "superseded" || lock === "closed" ? undefined : onReview}
      />
      <SaveStrip wide={wide} indicator={status} />
      {fullscreenBar && <FullscreenBar />}

      <div data-columns className="flex min-h-0 flex-1">
        <main
          ref={paper}
          tabIndex={-1}
          aria-label={t("takeTest.dotLabel", { n: index + 1 })}
          data-resize-middle
          className={cn(
            "@container/paper min-w-0 flex-1 overflow-y-auto outline-none",
            wide ? "p-8" : "p-4",
          )}
        >
          <div
            className={cn("mx-auto flex w-full flex-col gap-5", contextClasses.width)}
          >
            {wide && (
              <div className="flex items-center justify-between gap-3">
                <p className="text-muted-foreground text-xs">
                  {metaLine(t, question, index, total, sectioned ? group : null)}
                </p>
                {flag}
              </div>
            )}
            {group !== null && opensSection(group, index) && (
              <SectionInstructions
                group={group}
                audio={question.media?.kind === "audio"}
              />
            )}
            <div className={contextClasses.grid}>
              {context && (
                <GroupContext
                  key={context.id}
                  group={context}
                  numbers={numbers}
                  onGap={jumpToGap}
                  onRetryMedia={onReload}
                  wide={wide}
                />
              )}
              <div
                ref={answerPanel}
                id={`answer-question-${question.id}`}
                tabIndex={-1}
                aria-label={t("takeTest.dotLabel", { n: index + 1 })}
                className="flex min-w-0 flex-col gap-5 outline-none"
              >
                <QuestionCard
                  question={question}
                  onAudioExpired={onReload}
                  action={wide ? undefined : flag}
                />
                {wide && (
                  <div className="flex flex-wrap items-center gap-2 pt-2">
                    {previous}
                    {next}
                    <p className="text-muted-foreground ml-3 flex items-center gap-1 text-xs">
                      {t("takeTest.shortcuts")}
                      {choice && (
                        <>
                          {" "}
                          <Kbd>{KEY.a}</Kbd>
                          {KEY.dash}
                          <Kbd>{KEY.d}</Kbd> {t("takeTest.shortcutPick")} {KEY.dot}
                        </>
                      )}{" "}
                      <Kbd>{KEY.left}</Kbd> <Kbd>{KEY.right}</Kbd>{" "}
                      {t("takeTest.shortcutMove")} {KEY.dot} <Kbd>{KEY.f}</Kbd>{" "}
                      {t("takeTest.shortcutFlag")}
                    </p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </main>
        {wide && (
          <NavigatorRail
            dots={dots}
            current={index}
            groups={groups}
            onJump={onJump}
            onReview={onReview}
          />
        )}
      </div>

      {!wide && (
        <footer
          className="flex shrink-0 items-center gap-2 border-t p-3"
          style={{ paddingBottom: "max(0.75rem, env(safe-area-inset-bottom))" }}
        >
          {previous}
          <Button
            variant="outline"
            className="size-11 shrink-0"
            size="icon"
            aria-label={t("takeTest.questionList")}
            onClick={() => onNavOpen(true)}
          >
            <List aria-hidden="true" />
          </Button>
          {next}
        </footer>
      )}

      <NavigatorSheet
        open={navOpen}
        onOpenChange={onNavOpen}
        dots={dots}
        current={index}
        groups={groups}
        onJump={onJump}
        onReview={() => {
          onNavOpen(false);
          onReview();
        }}
      />
    </div>
  );
}

/** S-08's line above the stem: the part when there is more than one, the position, the worth. */
function metaLine(
  t: TFunction,
  question: StudentQuestion,
  index: number,
  total: number,
  group: SectionGroup | null,
): string {
  const points = worth(question, t);
  if (group === null) {
    return t("takeTest.questionMeta", { n: index + 1, total, points });
  }
  return t("takeTest.sectionMeta", {
    section: group.section.title,
    n: index + 1,
    total,
    points,
  });
}

/** The key caps S-08 draws. Not translated: they are the keys. */
const KEY = {
  a: "A",
  d: "D",
  dash: "–",
  dot: "·",
  left: "←",
  right: "→",
  f: "F",
} as const;

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

/** A–D on a choice question: a single picks, a multiple toggles. */
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

function paperColumns(shared: boolean) {
  return shared
    ? {
        width: "max-w-none",
        grid: "grid min-w-0 items-start gap-6 @min-[800px]/paper:grid-cols-2",
      }
    : { width: "max-w-[720px]", grid: undefined };
}
