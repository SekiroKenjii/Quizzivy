import { useEffect, useReducer, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { ChevronLeft, ChevronRight, Check, List, LoaderCircle, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { QuestionCard } from "../components/QuestionCard";
import { clearSession } from "@/features/integrity/buffer";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { getAttempt } from "../api";
import { remainingMs, useTakeTestStore } from "../store";

/**
 * S-05's engine, one question at a time.
 *
 * What is here: the paper, the answer being written into it, the clock, and the
 * answer to "did my work survive?". What is not, yet: the strike counter
 * (T-3.13/T-3.14, which owns the integrity signals) and the question navigator
 * sheet (S-06). Both attach to this chrome without moving it -- the header
 * keeps its height across question types on purpose, so the body never shifts
 * as a student moves between them.
 */
export default function TakeTestPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { attemptId } = useParams<{ attemptId: string }>();

  const [status, setStatus] = useState<"loading" | "ready" | "failed">("loading");
  const [index, setIndex] = useState(0);

  const questions = useTakeTestStore((s) => s.questions);
  const sessionId = useTakeTestStore((s) => s.sessionId);
  const beaconToken = useTakeTestStore((s) => s.beaconToken);
  const integrity = useTakeTestStore((s) => s.integrity);
  const lock = useTakeTestStore((s) => s.lock);
  const dirty = useTakeTestStore((s) => s.dirty.size);
  const inFlight = useTakeTestStore((s) => s.flushInFlight);
  const hydrate = useTakeTestStore((s) => s.hydrate);
  const reset = useTakeTestStore((s) => s.reset);

  // Also the recovery from an expired signed URL: §11.2 says treat it as
  // expiring and refetch rather than failing, and the payload is where a fresh
  // one comes from.
  const [reloads, reload] = useReducer((n: number) => n + 1, 0);

  // Every §10 listener, in one place. It returns the strike count, which T-3.14
  // renders; mounted here because this is the screen being watched, and it
  // stops watching when the screen goes away.
  useIntegrityMonitor({
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

  if (status === "loading") {
    return <Notice>{t("takeTest.loading")}</Notice>;
  }
  if (status === "failed") {
    return <Notice>{t("takeTest.loadFailed")}</Notice>;
  }
  if (questions.length === 0) {
    return <Notice>{t("takeTest.empty")}</Notice>;
  }

  const question = questions[Math.min(index, questions.length - 1)];
  if (question === undefined) return <Notice>{t("takeTest.empty")}</Notice>;
  const last = index >= questions.length - 1;

  return (
    <div className="flex flex-1 flex-col">
      <Header
        index={index}
        total={questions.length}
        onExit={() => void navigate("/app")}
      />
      <SaveStrip dirty={dirty} inFlight={inFlight} lock={lock} />

      <main className="mx-auto w-full max-w-[720px] flex-1 px-4 py-5">
        <QuestionCard question={question} onAudioExpired={reload} />
      </main>

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
        <Button variant="outline" className="flex-1" disabled>
          <List aria-hidden="true" />
          {t("takeTest.questionList")}
        </Button>
        {last ? (
          <Button className="flex-1" disabled>
            {t("takeTest.reviewAndSubmit")}
          </Button>
        ) : (
          <Button className="flex-1" onClick={() => setIndex((i) => i + 1)}>
            {t("takeTest.next")}
            <ChevronRight aria-hidden="true" />
          </Button>
        )}
      </footer>
    </div>
  );
}

function Notice({ children }: { children: string }) {
  return (
    <main className="mx-auto w-full max-w-[720px] px-4 py-16 text-center">
      <p className="text-muted-foreground text-sm leading-relaxed">{children}</p>
    </main>
  );
}

/**
 * The clock, read from the server's time rather than the device's, and the
 * exit that §10.2 forbids ever removing -- deliberately the least prominent
 * control here, and never beside the submit.
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
  const deadlineAt = useTakeTestStore((s) => s.deadlineAt);
  const offsetMs = useTakeTestStore((s) => s.offsetMs);

  // Derived during render rather than held in state. The interval's only job is
  // to ask for a repaint; keeping the number in state as well would be a second
  // copy of the truth, synchronised by an effect that sets state on mount --
  // which is the cascade react-hooks/set-state-in-effect exists to stop.
  const [, repaint] = useReducer((n: number) => n + 1, 0);
  useEffect(() => {
    const tick = setInterval(repaint, 1000);
    return () => clearInterval(tick);
  }, []);
  const left = remainingMs({ deadlineAt, offsetMs });

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
        <span className="text-sm font-semibold tabular-nums">{clock(left)}</span>
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
}: {
  dirty: number;
  inFlight: boolean;
  lock: string | null;
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

function clock(ms: number): string {
  const total = Math.floor(ms / 1000);
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}
