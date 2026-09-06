import { useEffect, useMemo, useReducer, useState, type ReactNode } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { ChevronLeft, ChevronRight, Flag, List, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { LoadError } from "@/components/shared/ListState";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import { EngineHeader } from "../components/EngineHeader";
import { NavigatorRail, NavigatorSheet, type DotState } from "../components/Navigator";
import { QuestionCard } from "../components/QuestionCard";
import { ReviewScreen } from "../components/ReviewScreen";
import { SaveStrip } from "../components/SaveState";
import { SectionInstructions } from "../components/SectionInstructions";
import { SubmittedScreen } from "../components/SubmittedScreen";
import { clearSession } from "@/features/integrity/buffer";
import { FullscreenBar } from "@/features/integrity/components/FullscreenBar";
import { StrikeDialog } from "@/features/integrity/components/StrikeDialog";
import { StrikeIndicator } from "@/features/integrity/components/StrikeIndicator";
import { strikeState } from "@/features/integrity/strikes";
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
import { worth } from "../worth";

/**
 * S-05's engine, one question at a time -- and S-06's two other views of the
 * same attempt: the navigator (a sheet in thumb range, a rail from 1024px)
 * and the review before submitting. From 1024px the chrome is S-08's: one
 * header row, no save strip, no sticky footer, the two buttons under the
 * answer at their own width.
 */
export default function TakeTestPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { attemptId } = useParams<{ attemptId: string }>();
  const wide = useMediaQuery("(min-width: 1024px)");

  const [status, setStatus] = useState<"loading" | "ready" | "failed">("loading");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [index, setIndex] = useState(0);
  const [view, setView] = useState<"question" | "review">("question");
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

  const [reloads, reload] = useReducer((n: number) => n + 1, 0);
  const groups = useMemo(
    () => groupBySection(sections, questions),
    [sections, questions],
  );

  // Every §10 listener, in one place.
  const { strikes, lastAwayMs, fullscreen } = useIntegrityMonitor({
    attemptId: attemptId ?? null,
    sessionId,
    beaconToken,
    policy: integrity,
  });

  useEffect(() => {
    if (attemptId === undefined) return;
    const abort = new AbortController();
    getAttempt(attemptId, abort.signal)
      .then((session) => {
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
      clearSession(attemptId);
      reset();
    };
  }, [attemptId, reloads, hydrate, reset]);

  // S-08's shortcuts.
  const question = questions[Math.min(index, questions.length - 1)];
  useEffect(() => {
    if (view !== "question" || navOpen || lock !== null || question === undefined)
      return;
    const onKey = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey || typingIn(event.target))
        return;
      switch (event.key) {
        case "ArrowRight":
          setIndex((i) => Math.min(questions.length - 1, i + 1));
          return;
        case "ArrowLeft":
          setIndex((i) => Math.max(0, i - 1));
          return;
        case "f":
        case "F":
          toggleFlag(question.id);
          return;
      }
      const pick = "abcd".indexOf(event.key.toLowerCase());
      const option = question.options?.[pick];
      if (pick >= 0 && option !== undefined) {
        setAnswer(question.id, chooseOption(question, answers[question.id], option.id));
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [view, navOpen, lock, question, questions.length, answers, toggleFlag, setAnswer]);

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
  const jump = (i: number) => {
    setIndex(i);
    setNavOpen(false);
    setView("question");
  };

  if (submitState === "done" && submittedAt !== null) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        {wide && <EngineHeader wide leading={null} progress={null} live={false} />}
        <SubmittedScreen
          reason={submitReason ?? "manual"}
          submittedAt={submittedAt}
          answered={dots.filter((d) => d.answered).length}
          total={dots.length}
          onHome={() => void navigate("/app", { replace: true })}
        />
      </div>
    );
  }

  // The server's count from before this sitting plus what this tab has seen since.
  const watching = integrity !== null && lock === null;
  const strikeStatus = watching
    ? strikeState(integrity, focusLossCount + strikes)
    : null;
  const strikeDialog = strikeStatus !== null && (
    <StrikeDialog state={strikeStatus} strikes={strikes} lastAwayMs={lastAwayMs} />
  );
  const strikeIndicator =
    strikeStatus === null ? null : <StrikeIndicator state={strikeStatus} />;

  if (view === "review") {
    return (
      <>
        <ReviewScreen
          wide={wide}
          dots={dots}
          groups={groups}
          status={strikeIndicator}
          onBack={() => setView("question")}
          onJump={jump}
        />
        {strikeDialog}
      </>
    );
  }

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
        onExit={() => void navigate("/app")}
        onMove={setIndex}
        onJump={jump}
        onReview={() => setView("review")}
        onReload={reload}
      />
      {strikeDialog}
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
  onExit,
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
  onExit: () => void;
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
  const choice =
    question.type === "single_choice" || question.type === "multiple_choice";

  const previous = (
    <Button
      variant="outline"
      size={wide ? "default" : "icon"}
      aria-label={wide ? undefined : t("takeTest.previous")}
      disabled={index === 0}
      onClick={() => onMove(Math.max(0, index - 1))}
    >
      <ChevronLeft aria-hidden="true" />
      {wide && t("takeTest.previous")}
    </Button>
  );
  const next = last ? (
    <Button className={wide ? undefined : "flex-1"} onClick={onReview}>
      {t("takeTest.reviewAndSubmit")}
    </Button>
  ) : (
    <Button className={wide ? undefined : "flex-1"} onClick={() => onMove(index + 1)}>
      {t("takeTest.next")}
      <ChevronRight aria-hidden="true" />
    </Button>
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <EngineHeader
        wide={wide}
        counter={{ n: index + 1, total }}
        progress={(index + 1) / total}
        status={status}
        leading={
          <Button
            variant="ghost"
            size="xs"
            className="text-muted-foreground px-1"
            onClick={onExit}
          >
            <X aria-hidden="true" />
            {t("takeTest.exit")}
          </Button>
        }
      />
      <SaveStrip wide={wide} indicator={status} />
      {fullscreenBar && <FullscreenBar />}

      <div data-columns className="flex min-h-0 flex-1">
        <main
          data-resize-middle
          className={cn("min-w-0 flex-1 overflow-y-auto", wide ? "p-8" : "px-4 py-5")}
        >
          <div className="mx-auto w-full max-w-[720px] space-y-5">
            <div className="flex items-center justify-between gap-3">
              {wide ? (
                <p className="text-muted-foreground text-xs">
                  {metaLine(t, question, index, total, sectioned ? group : null)}
                </p>
              ) : (
                <span />
              )}
              <Button
                variant="ghost"
                size="sm"
                className="text-muted-foreground shrink-0"
                aria-pressed={flagged}
                aria-label={t(flagged ? "takeTest.unflagThis" : "takeTest.flagThis")}
                disabled={lock !== null}
                onClick={() => toggleFlag(question.id)}
              >
                <Flag
                  className={flagged ? "fill-current" : undefined}
                  aria-hidden="true"
                />
                {wide && t("takeTest.flag")}
              </Button>
            </div>
            {group !== null && opensSection(group, index) && (
              <SectionInstructions
                group={group}
                audio={question.media?.kind === "audio"}
              />
            )}
            <QuestionCard question={question} onAudioExpired={onReload} />
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
          <Button variant="outline" className="flex-1" onClick={() => onNavOpen(true)}>
            <List aria-hidden="true" />
            {t("takeTest.questionList")}
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
    section: t("takeTest.sectionLabel", {
      n: group.ordinal,
      title: group.section.title,
    }),
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
    target.tagName === "INPUT" ||
    target.tagName === "TEXTAREA" ||
    target.tagName === "SELECT"
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
