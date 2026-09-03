import { useEffect, useReducer, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import {
  ChevronLeft,
  ChevronRight,
  Check,
  Flag,
  List,
  LoaderCircle,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { Clock } from "../components/Clock";
import { NavigatorRail, NavigatorSheet, type DotState } from "../components/Navigator";
import { QuestionCard } from "../components/QuestionCard";
import { ReviewScreen } from "../components/ReviewScreen";
import { clearSession } from "@/features/integrity/buffer";
import { FullscreenBar } from "@/features/integrity/components/FullscreenBar";
import { StrikeDialog } from "@/features/integrity/components/StrikeDialog";
import { StrikeIndicator } from "@/features/integrity/components/StrikeIndicator";
import { strikeState } from "@/features/integrity/strikes";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { answered } from "../answered";
import { getAttempt, type Answer, type StudentQuestion } from "../api";
import { useTakeTestStore } from "../store";

/**
 * S-05's engine, one question at a time -- and S-06's two other views of the
 * same attempt: the navigator (a sheet in thumb range, a rail from 1024px)
 * and the review before submitting.
 *
 * Both are views, not routes. A route change would unmount this page, and
 * with it the store, the event buffer and the monitor; the student would
 * come back to a paper that had forgotten them. The header keeps its height
 * across question types on purpose, so the body never shifts as a student
 * moves between them.
 */
export default function TakeTestPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { attemptId } = useParams<{ attemptId: string }>();

  const [status, setStatus] = useState<"loading" | "ready" | "failed">("loading");
  const [index, setIndex] = useState(0);
  const [view, setView] = useState<"question" | "review">("question");
  const [navOpen, setNavOpen] = useState(false);

  const questions = useTakeTestStore((s) => s.questions);
  const answers = useTakeTestStore((s) => s.answers);
  const flags = useTakeTestStore((s) => s.flags);
  const sessionId = useTakeTestStore((s) => s.sessionId);
  const beaconToken = useTakeTestStore((s) => s.beaconToken);
  const integrity = useTakeTestStore((s) => s.integrity);
  const focusLossCount = useTakeTestStore((s) => s.focusLossCount);
  const lock = useTakeTestStore((s) => s.lock);
  const submitState = useTakeTestStore((s) => s.submitState);
  const dirty = useTakeTestStore((s) => s.dirty.size);
  const inFlight = useTakeTestStore((s) => s.flushInFlight);
  const hydrate = useTakeTestStore((s) => s.hydrate);
  const setAnswer = useTakeTestStore((s) => s.setAnswer);
  const toggleFlag = useTakeTestStore((s) => s.toggleFlag);
  const reset = useTakeTestStore((s) => s.reset);

  // Also the recovery from an expired signed URL: §11.2 says treat it as
  // expiring and refetch rather than failing, and the payload is where a fresh
  // one comes from.
  const [reloads, reload] = useReducer((n: number) => n + 1, 0);

  // Every §10 listener, in one place. Mounted here because this is the screen
  // being watched, and it stops watching when the screen goes away.
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
      .catch(() => {
        if (!abort.signal.aborted) setStatus("failed");
      });
    return () => {
      abort.abort();
      clearSession(attemptId);
      reset();
    };
  }, [attemptId, reloads, hydrate, reset]);

  // Submitted -- by the button, by the timer, or by the other tab -- and
  // there is nothing left to do here. Home, until Phase 4's result page.
  useEffect(() => {
    if (submitState === "done") void navigate("/app", { replace: true });
  }, [submitState, navigate]);

  // S-08's shortcuts. Never while the student is typing, and never inside a
  // sheet or the review, where the keys mean something else.
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
    return <Notice>{t("takeTest.loadFailed")}</Notice>;
  }
  if (question === undefined) {
    return <Notice>{t("takeTest.empty")}</Notice>;
  }
  const last = index >= questions.length - 1;

  const dots: DotState[] = questions.map((q) => ({
    id: q.id,
    answered: answered(answers[q.id]),
    flagged: flags.has(q.id),
  }));
  const jump = (i: number) => {
    setIndex(i);
    setNavOpen(false);
    setView("question");
  };

  // The server's count from before this sitting plus what this tab has seen
  // since. Nothing integrity-related renders over a locked paper: it is
  // read-only, and a strike against it would be a strike against nothing.
  const watching = integrity !== null && lock === null;
  const strikeStatus = watching
    ? strikeState(integrity, focusLossCount + strikes)
    : null;
  const strikeDialog = strikeStatus !== null && (
    <StrikeDialog state={strikeStatus} strikes={strikes} lastAwayMs={lastAwayMs} />
  );

  if (view === "review") {
    return (
      <>
        <ReviewScreen dots={dots} onBack={() => setView("question")} onJump={jump} />
        {strikeDialog}
      </>
    );
  }

  const flagged = flags.has(question.id);
  const choice =
    question.type === "single_choice" || question.type === "multiple_choice";

  return (
    <div className="flex flex-1 flex-col">
      <Header
        index={index}
        total={questions.length}
        onExit={() => void navigate("/app")}
      />
      <SaveStrip
        dirty={dirty}
        inFlight={inFlight}
        lock={lock}
        indicator={
          strikeStatus === null ? null : <StrikeIndicator state={strikeStatus} />
        }
      />
      {watching && integrity.requireFullscreen && !fullscreen && <FullscreenBar />}
      {strikeDialog}

      <div className="flex flex-1">
        <main className="mx-auto w-full max-w-[720px] min-w-0 flex-1 px-4 py-5">
          <div className="mb-3 flex items-center justify-between gap-3">
            <p className="text-muted-foreground text-xs">
              {t("takeTest.questionCounter", { n: index + 1, total: questions.length })}
            </p>
            <Button
              variant="ghost"
              size="sm"
              className="text-muted-foreground"
              aria-pressed={flagged}
              aria-label={t(flagged ? "takeTest.unflagThis" : "takeTest.flagThis")}
              disabled={lock !== null}
              onClick={() => toggleFlag(question.id)}
            >
              <Flag
                className={flagged ? "fill-current" : undefined}
                aria-hidden="true"
              />
              <span className="hidden lg:inline">{t("takeTest.flag")}</span>
            </Button>
          </div>
          <QuestionCard question={question} onAudioExpired={reload} />
          <p className="text-muted-foreground mt-6 hidden items-center gap-1 text-xs lg:flex">
            {t("takeTest.shortcuts")}
            {choice && (
              <>
                {" "}
                <Kbd>{KEY.a}</Kbd>
                {KEY.dash}
                <Kbd>{KEY.d}</Kbd> {t("takeTest.shortcutPick")} {KEY.dot}
              </>
            )}{" "}
            <Kbd>{KEY.left}</Kbd> <Kbd>{KEY.right}</Kbd> {t("takeTest.shortcutMove")}{" "}
            {KEY.dot} <Kbd>{KEY.f}</Kbd> {t("takeTest.shortcutFlag")}
          </p>
        </main>
        <NavigatorRail
          dots={dots}
          current={index}
          onJump={jump}
          onReview={() => setView("review")}
        />
      </div>

      <footer
        className="bg-background sticky bottom-0 flex items-center gap-2 border-t p-3"
        style={{ paddingBottom: "max(0.75rem, env(safe-area-inset-bottom))" }}
      >
        <Button
          variant="outline"
          size="icon"
          aria-label={t("takeTest.previous")}
          disabled={index === 0}
          onClick={() => setIndex((i) => Math.max(0, i - 1))}
        >
          <ChevronLeft aria-hidden="true" />
        </Button>
        <Button
          variant="outline"
          className="flex-1 lg:hidden"
          onClick={() => setNavOpen(true)}
        >
          <List aria-hidden="true" />
          {t("takeTest.questionList")}
        </Button>
        {last ? (
          <Button className="flex-1" onClick={() => setView("review")}>
            {t("takeTest.reviewAndSubmit")}
          </Button>
        ) : (
          <Button className="flex-1" onClick={() => setIndex((i) => i + 1)}>
            {t("takeTest.next")}
            <ChevronRight aria-hidden="true" />
          </Button>
        )}
      </footer>

      <NavigatorSheet
        open={navOpen}
        onOpenChange={setNavOpen}
        dots={dots}
        current={index}
        onJump={jump}
        onReview={() => {
          setNavOpen(false);
          setView("review");
        }}
      />
    </div>
  );
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

function Notice({ children }: { children: string }) {
  return (
    <main className="mx-auto w-full max-w-[720px] px-4 py-16 text-center">
      <p className="text-muted-foreground text-sm leading-relaxed">{children}</p>
    </main>
  );
}

/**
 * The clock, and the exit that §10.2 forbids ever removing -- deliberately
 * the least prominent control here, and never beside the submit.
 */
function Header({
  index,
  total,
  onExit,
}: {
  index: number;
  total: number;
  onExit: () => void;
}) {
  const { t } = useTranslation();
  return (
    <header className="border-b">
      <div className="mx-auto flex h-12 w-full max-w-[720px] items-center gap-3 px-4">
        <Button
          variant="ghost"
          size="xs"
          className="text-muted-foreground px-1"
          onClick={onExit}
        >
          <X aria-hidden="true" />
          {t("takeTest.exit")}
        </Button>
        <span className="text-muted-foreground ml-auto text-xs tabular-nums">
          {t("takeTest.questionCounter", { n: index + 1, total })}
        </span>
        <Clock />
      </div>
      <div className="bg-secondary h-1">
        <div
          className="bg-primary h-full transition-[width]"
          style={{ width: `${(((index + 1) / total) * 100).toFixed(2)}%` }}
        />
      </div>
    </header>
  );
}

/**
 * "Did my work survive?" -- asked constantly and quietly, so it gets its own
 * strip. In the header it would compete with the timer; in a toast it would
 * disappear exactly when the student wanted it.
 */
function SaveStrip({
  dirty,
  inFlight,
  lock,
  indicator,
}: {
  dirty: number;
  inFlight: boolean;
  lock: string | null;
  /** S-05 puts the strike count at the strip's far end, beside the save state. */
  indicator: ReactNode;
}) {
  const { t } = useTranslation();
  // The moment the SERVER confirmed, not the moment this rendered. Reading the
  // clock here would answer "what time is it" while appearing to answer "when
  // did my work last survive", and would keep looking reassuring long after
  // saves stopped landing.
  const lastSavedAt = useTakeTestStore((s) => s.lastSavedAt);

  if (lock !== null) {
    const message =
      lock === "superseded"
        ? t("takeTest.lockedSuperseded")
        : lock === "deadline"
          ? t("takeTest.lockedDeadline")
          : t("takeTest.lockedClosed");
    return (
      <div className="bg-warning/10 border-b px-4 py-3">
        <p className="mx-auto w-full max-w-[720px] text-xs leading-relaxed">
          {message}
        </p>
      </div>
    );
  }

  return (
    <div className="bg-muted/30 border-b px-4 py-3">
      <div className="text-muted-foreground mx-auto flex w-full max-w-[720px] items-center gap-2 text-xs">
        {inFlight ? (
          <>
            <LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" />
            {t("takeTest.saving")}
          </>
        ) : dirty > 0 ? (
          t("takeTest.unsaved")
        ) : (
          <>
            <Check className="size-3.5" aria-hidden="true" />
            {lastSavedAt === null
              ? t("takeTest.savedNothingYet")
              : t("takeTest.saved", { time: hhmm(lastSavedAt) })}
          </>
        )}
        {indicator !== null && <span className="ml-auto">{indicator}</span>}
      </div>
    </div>
  );
}

function hhmm(iso: string): string {
  // 24-hour, as the deck writes it ("Đã lưu 09:41"). Left to the locale it
  // renders as "04:44 PM" on an English-defaulted machine, which is neither
  // what S-05 shows nor how anyone here reads a clock.
  return new Date(iso).toLocaleTimeString("vi-VN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}
