import { useId, useState } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { LoaderCircle } from "lucide-react";
import {
  DeckDialog,
  DeckDialogActions,
  DeckDialogCancel,
} from "@/components/shared/DeckDialog";
import { Button } from "@/components/ui/button";
import { useTick } from "@/hooks/useTick";
import { countdown } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";
import { useTakeTestStore } from "../store";
import { timeLeft } from "../timeLeft";
import type { DotState } from "./Navigator";

const MINUTE_MS = 60_000;
const LOW_MS = 5 * MINUTE_MS;

function bodyOf(unanswered: number, left: number, t: TFunction): string {
  if (unanswered === 0) return t("takeTest.submitDialog.bodyAll");
  if (left <= 0) return t("takeTest.submitDialog.bodyOver");
  const minutes = Math.floor(left / MINUTE_MS);
  if (minutes === 0) return t("takeTest.submitDialog.bodySoon");
  return t("takeTest.submitDialog.bodyUnanswered", { count: minutes });
}

/**
 * SubmitDialog is the deck's Submit dialog, the one place a student hands the
 * paper in. It says how many questions are unanswered, shows Answered,
 * Flagged and Time left, the last ticking with the header's timer, taking
 * the danger ink under five minutes and standing at 00:00 once the server has
 * said the time is up, and offers a "Go to" chip for each unanswered
 * question, which closes the dialog on that question. "Submit
 * test" runs the store's submit, which saves what is unsaved first and does
 * nothing when a timer or an auto-submit is already submitting. While a
 * submission is out the dialog cannot be dismissed and both buttons are
 * inert; one that fails says so in the dialog and leaves it open to try
 * again. The chips scroll inside their own box on a long paper, so the two
 * buttons stay in reach.
 */
export function SubmitDialog({
  open,
  dots,
  onClose,
  onGo,
}: Readonly<{
  open: boolean;
  dots: DotState[];
  onClose: () => void;
  onGo: (index: number) => void;
}>) {
  const { t } = useTranslation();
  const goId = useId();
  const busy = useTakeTestStore((s) => s.submitState === "inFlight");
  const deadlineAt = useTakeTestStore((s) => s.deadlineAt);
  const offsetMs = useTakeTestStore((s) => s.offsetMs);
  const over = useTakeTestStore((s) => s.lock === "deadline");
  useTick(open);

  const left = timeLeft({ deadlineAt, offsetMs, lock: over ? "deadline" : null });
  const unanswered = dots.flatMap((dot, index) => (dot.answered ? [] : [index]));
  const flagged = dots.filter((dot) => dot.flagged).length;
  const facts = [
    {
      label: t("takeTest.submitDialog.answered"),
      value: `${dots.length - unanswered.length} / ${dots.length}`,
      ink: undefined,
    },
    {
      label: t("takeTest.submitDialog.flagged"),
      value: String(flagged),
      ink: flagged > 0 ? "text-warning-ink" : undefined,
    },
    {
      label: t("takeTest.submitDialog.timeLeft"),
      value: countdown(left),
      ink: left < LOW_MS ? "text-danger-ink" : undefined,
    },
  ];

  return (
    <DeckDialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !busy) onClose();
      }}
      title={
        unanswered.length === 0
          ? t("takeTest.submitDialog.titleAll")
          : t("takeTest.submitDialog.titleUnanswered", { n: unanswered.length })
      }
      description={bodyOf(unanswered.length, left, t)}
    >
      <dl className="bg-border grid grid-cols-3 gap-px overflow-hidden rounded-lg border">
        {facts.map((fact) => (
          <div
            key={fact.label}
            className="bg-card flex min-w-0 flex-col justify-between px-3 py-2.5"
          >
            <dt className="text-muted-fg text-xs leading-normal">{fact.label}</dt>
            <dd className={cn("text-stat-sm leading-normal font-semibold", fact.ink)}>
              {fact.value}
            </dd>
          </div>
        ))}
      </dl>
      {unanswered.length > 0 && (
        <div
          role="group"
          aria-labelledby={goId}
          className="-m-1 flex max-h-40 flex-none flex-wrap items-center gap-1.5 overflow-y-auto p-1"
        >
          <span id={goId} className="text-muted-fg text-meta leading-normal">
            {t("takeTest.submitDialog.goTo")}
          </span>
          {unanswered.map((index) => (
            <button
              key={dots[index]?.id ?? index}
              type="button"
              disabled={busy}
              aria-label={t("takeTest.submitDialog.question", { n: index + 1 })}
              className="bg-card hover:bg-muted rounded-seg h-7.5 min-w-8 border px-2 text-sm leading-none font-semibold disabled:opacity-50"
              onClick={() => onGo(index)}
            >
              {index + 1}
            </button>
          ))}
        </div>
      )}
      <Actions busy={busy} onClose={onClose} />
    </DeckDialog>
  );
}

function Actions({ busy, onClose }: Readonly<{ busy: boolean; onClose: () => void }>) {
  const { t } = useTranslation();
  const submit = useTakeTestStore((s) => s.submit);
  const [failed, setFailed] = useState(false);

  const confirm = async () => {
    setFailed(false);
    await submit("manual");
    if (useTakeTestStore.getState().submitState === "idle") setFailed(true);
  };

  return (
    <>
      {failed && !busy && (
        <p role="alert" className="text-danger-ink text-sm leading-normal">
          {t("takeTest.submitFailed")}
        </p>
      )}
      <DeckDialogActions>
        <DeckDialogCancel disabled={busy} onClick={onClose}>
          {t("takeTest.keepWorking")}
        </DeckDialogCancel>
        <Button
          size="lg"
          aria-busy={busy || undefined}
          aria-disabled={busy || undefined}
          onClick={() => {
            if (!busy) void confirm();
          }}
        >
          {busy && (
            <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
          )}
          {t(busy ? "takeTest.submitting" : "takeTest.submitDialog.confirm")}
        </Button>
      </DeckDialogActions>
    </>
  );
}
