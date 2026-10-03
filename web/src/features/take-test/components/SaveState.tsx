import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  CloudAlert,
  CloudCheck,
  CloudOff,
  Loader,
  Lock,
  MonitorSmartphone,
  TimerOff,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useSaveStatus, type SaveStatus } from "../saveStatus";
import { useTakeTestStore, type LockReason } from "../store";
import { NoticeBar } from "./NoticeBar";

const LINE: Record<SaveStatus, { icon: LucideIcon; label: string }> = {
  saved: { icon: CloudCheck, label: "takeTest.allSaved" },
  saving: { icon: Loader, label: "takeTest.saving" },
  offline: { icon: CloudOff, label: "takeTest.saveOffline" },
  failed: { icon: CloudAlert, label: "takeTest.saveFailed" },
};

const LOCKED: Record<LockReason, { icon: LucideIcon; label: string }> = {
  superseded: { icon: MonitorSmartphone, label: "takeTest.lockedSuperseded" },
  deadline: { icon: TimerOff, label: "takeTest.lockedDeadline" },
  closed: { icon: Lock, label: "takeTest.lockedClosed" },
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
 * why at every width, in the notice bar's warning tone: it was taken over by
 * another device, its time is up, or it has ended. Below 768 the strip
 * carries the save line only when a save has failed or the device is offline
 * while an answer is unsaved, never for a save that is merely on its way, and
 * the strike count at its far end when the assignment counts departures; with
 * neither it is absent, as the deck draws it. From 768 the header holds both
 * and there is no strip.
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
    return <NoticeBar icon={LOCKED[lock].icon}>{t(LOCKED[lock].label)}</NoticeBar>;
  }
  const trouble = troubled(status);
  if (wide || (!trouble && indicator === null)) return null;

  return (
    <div
      data-slot="save-strip"
      className="text-muted-fg flex flex-none items-center gap-2 border-b px-3.5 py-2 text-xs leading-normal"
    >
      {trouble && <SaveState />}
      {indicator !== null && <span className="ml-auto flex-none">{indicator}</span>}
    </div>
  );
}
