import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Check, LoaderCircle } from "lucide-react";
import { useTakeTestStore } from "../store";
import { formatTime } from "@/lib/i18n/datetime";

/** S-05's save state: in flight, unsaved edits, or the last save's time. */
export function SaveState() {
  const { t } = useTranslation();
  const inFlight = useTakeTestStore((s) => s.flushInFlight);
  const dirty = useTakeTestStore((s) => s.dirty.size);
  // The moment the SERVER confirmed, not the moment this rendered.
  const lastSavedAt = useTakeTestStore((s) => s.lastSavedAt);

  if (inFlight) {
    return (
      <>
        <LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" />
        {t("takeTest.saving")}
      </>
    );
  }
  if (dirty > 0) return <>{t("takeTest.unsaved")}</>;
  return (
    <>
      <Check className="size-3.5" aria-hidden="true" />
      {lastSavedAt === null
        ? t("takeTest.savedNothingYet")
        : t("takeTest.saved", { time: formatTime(lastSavedAt) })}
    </>
  );
}

/**
 * "Did my work survive?" -- asked constantly and quietly, so on a phone it
 * gets its own strip under the header (S-05). From 1024px the header row has
 * room for the save state (S-08), and the strip stays only to carry a lock
 * message, which is the one thing here a student must not miss.
 */
export function SaveStrip({
  wide,
  indicator,
}: Readonly<{
  wide: boolean;
  /** S-05 puts the strike count at the strip's far end, beside the save state. */
  indicator: ReactNode;
}>) {
  const { t } = useTranslation();
  const lock = useTakeTestStore((s) => s.lock);

  if (lock !== null) {
    return (
      <div className="bg-warning/10 border-b px-4 py-3">
        <p className="mx-auto w-full max-w-[720px] text-xs leading-relaxed">
          {t(lockMessageKey(lock))}
        </p>
      </div>
    );
  }
  if (wide) return null;

  return (
    <div className="bg-muted/30 border-b px-4 py-3">
      <div className="text-muted-foreground mx-auto flex w-full max-w-[720px] items-center gap-2 text-xs">
        <SaveState />
        {indicator !== null && <span className="ml-auto">{indicator}</span>}
      </div>
    </div>
  );
}

function lockMessageKey(lock: string): string {
  if (lock === "superseded") return "takeTest.lockedSuperseded";
  if (lock === "deadline") return "takeTest.lockedDeadline";
  return "takeTest.lockedClosed";
}
