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
} from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { LoadError } from "@/components/shared/ListState";
import { EngineHeader } from "../components/EngineHeader";
import { LeaveDialog } from "../components/LeaveDialog";
import { Paper } from "../components/Paper";
import { SubmitDialog } from "../components/SubmitDialog";
import { SubmittedScreen } from "../components/SubmittedScreen";
import { releaseSession } from "@/features/integrity/release";
import { forgetAllPlaces } from "@/features/media/playback";
import { exitFullscreen } from "@/features/integrity/fullscreen";
import { StrikeDialog } from "@/features/integrity/components/StrikeDialog";
import { StrikeIndicator } from "@/features/integrity/components/StrikeIndicator";
import { strikeState } from "@/features/integrity/strikes";
import { useClipboardNotice } from "@/features/integrity/useClipboardNotice";
import { useIntegrityMonitor } from "@/features/integrity/useIntegrityMonitor";
import { dotsOf } from "../answered";
import { getAttempt } from "../api";
import {
  clearOpenQuestion,
  readOpenQuestion,
  writeOpenQuestion,
} from "../openQuestion";
import { PaperSourceContext } from "../paper";
import { useLivePaperSource } from "../livePaper";
import { paperKey } from "../shortcuts";
import { paperStops } from "../stops";
import { useTakeTestStore } from "../store";
import { useLeave } from "../useLeave";
import { useEngineWidthAtLeast } from "../width";

const WIDE_PX = 768;

/**
 * TakeTestPage is the engine, one question at a time, with the navigator as
 * the footer of the question pane and, on a wide enough screen, the sections
 * rail at the paper's left edge, and the deck's Submit dialog over it, which
 * is the only way to hand the paper in. The header is the deck's at every
 * width; below 768px a strip under it says when a save has failed or the
 * device is offline, and is otherwise absent. The keys are the deck's: the
 * arrows move by stop and stop at either end, A to E choose, F flags; on a
 * run of a cloze they choose and flag for the open gap, and a choice opens
 * the run's next gap with no answer. None acts in a
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
  const groups = useTakeTestStore((s) => s.groups);
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
  const stops = useMemo(() => paperStops(questions, groups), [questions, groups]);
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

  const openAt = (target: number | null) => {
    if (target !== null) setIndex(target);
  };
  const handleShortcut = useEffectEvent((event: KeyboardEvent) => {
    if (lock !== null || submitState !== "idle" || question === undefined) return;
    const key = paperKey(event, { stops, index, question, questions, answers });
    if (key === null) return;
    if (
      document.querySelector(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
      )
    )
      return;
    event.preventDefault();
    switch (key.kind) {
      case "open":
        openAt(key.index);
        return;
      case "flag":
        if (key.questionId !== null) toggleFlag(key.questionId);
        return;
      case "answer":
        setAnswer(key.questionId, key.answer);
        openAt(key.open);
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
