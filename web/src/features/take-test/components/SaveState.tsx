import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  CloudAlert,
  CloudCheck,
  CloudOff,
  Loader,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useSaveStatus, type SaveStatus } from "../saveStatus";
import { useTakeTestStore } from "../store";

const LINE: Record<SaveStatus, { icon: LucideIcon; label: string }> = {
  saved: { icon: CloudCheck, label: "takeTest.allSaved" },
  saving: { icon: Loader, label: "takeTest.saving" },
  offline: { icon: CloudOff, label: "takeTest.saveOffline" },
  failed: { icon: CloudAlert, label: "takeTest.saveFailed" },
};

function troubled(status: SaveStatus): boolean {
  return status === "offline" || status === "failed";
}

/**
 * SaveState is the engine's save line: the deck's "All answers saved" and
 * "Saving…", and the two the deck does not draw, offline and failed, in the
 * danger ink. Those two are hidden from a screen reader here because
 * SaveAnnouncement says them.
 */
export function SaveState() {
  const { t } = useTranslation();
  const status = useSaveStatus();
  const { icon: Icon, label } = LINE[status];
  const trouble = troubled(status);
  return (
    <span
      aria-hidden={trouble || undefined}
      className={cn(
        "flex min-w-0 items-center gap-[5px]",
        trouble && "text-danger-ink",
      )}
    >
      <Icon
        aria-hidden="true"
        className={cn("size-3 flex-none", status === "saving" && "animate-spin")}
      />
      <span className="truncate">{t(label)}</span>
    </span>
  );
}

/**
 * SaveAnnouncement is a polite live region that says the offline or failed
 * line once when saves stop going through, and is empty otherwise, so a
 * screen reader is not told about every routine save.
 */
export function SaveAnnouncement() {
  const { t } = useTranslation();
  const status = useSaveStatus();
  return (
    <span role="status" className="sr-only">
      {troubled(status) ? t(LINE[status].label) : ""}
    </span>
  );
}

/**
 * SaveStrip is the one line under the engine's header. A locked paper says
 * why at every width. Below 768 the strip carries the save line only while
 * something is unsaved, and the strike count at its far end when the
 * assignment counts departures; with neither it is absent. From 768 the
 * header holds both.
 */
export function SaveStrip({
  wide,
  indicator,
}: Readonly<{
  wide: boolean;
  indicator: ReactNode;
}>) {
  const { t } = useTranslation();
  const lock = useTakeTestStore((s) => s.lock);
  const status = useSaveStatus();

  if (lock !== null) {
    return (
      <div className="bg-warning/10 border-b px-4 py-3">
        <p className="mx-auto w-full max-w-[720px] text-xs leading-relaxed">
          {t(lockMessageKey(lock))}
        </p>
      </div>
    );
  }
  if (wide || (status === "saved" && indicator === null)) return null;

  return (
    <div
      data-slot="save-strip"
      className="text-muted-fg flex flex-none items-center gap-2 border-b px-3.5 py-2 text-xs leading-normal"
    >
      {status !== "saved" && <SaveState />}
      {indicator !== null && <span className="ml-auto flex-none">{indicator}</span>}
    </div>
  );
}

function lockMessageKey(lock: string): string {
  if (lock === "superseded") return "takeTest.lockedSuperseded";
  if (lock === "deadline") return "takeTest.lockedDeadline";
  return "takeTest.lockedClosed";
}
