import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  CloudAlert,
  CloudCheck,
  CloudOff,
  Eye,
  Loader,
  Lock,
  MonitorSmartphone,
  TimerOff,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { usePaper, type PaperSource } from "../paper";
import { NoticeBar } from "./NoticeBar";

const LINE: Record<PaperSource["saveStatus"], { icon: LucideIcon; label: string }> = {
  saved: { icon: CloudCheck, label: "takeTest.allSaved" },
  saving: { icon: Loader, label: "takeTest.saving" },
  offline: { icon: CloudOff, label: "takeTest.saveOffline" },
  failed: { icon: CloudAlert, label: "takeTest.saveFailed" },
};

const LOCKED: Record<
  NonNullable<PaperSource["lock"]>,
  { icon: LucideIcon; label: string }
> = {
  superseded: { icon: MonitorSmartphone, label: "takeTest.lockedSuperseded" },
  deadline: { icon: TimerOff, label: "takeTest.lockedDeadline" },
  closed: { icon: Lock, label: "takeTest.lockedClosed" },
};

function troubled(status: PaperSource["saveStatus"]): boolean {
  return status === "offline" || status === "failed";
}

/**
 * SaveState is the engine's save line: the deck's "All answers saved" and
 * "Saving…", and the two the deck does not draw, offline and failed, in the
 * danger ink. Those two are hidden from a screen reader here because
 * SaveAnnouncement says them. With `wrap` the line breaks onto as many rows
 * as it needs, the icon beside the first, instead of ending in an ellipsis.
 */
export function SaveState({ wrap = false }: Readonly<{ wrap?: boolean }>) {
  const { t } = useTranslation();
  const status = usePaper().saveStatus;
  const { icon: Icon, label } = LINE[status];
  const trouble = troubled(status);
  return (
    <span
      aria-hidden={trouble || undefined}
      className={cn(
        "flex min-w-0 gap-[5px]",
        wrap ? "items-start" : "items-center",
        trouble && "text-danger-ink",
      )}
    >
      <Icon
        aria-hidden="true"
        className={cn(
          "size-3 flex-none",
          wrap && "mt-[3px]",
          status === "saving" && "animate-spin",
        )}
      />
      <span className={wrap ? "min-w-0 text-balance" : "truncate"}>{t(label)}</span>
    </span>
  );
}

/**
 * UnsavedLine stands where a teacher's preview of the paper would have the
 * save line: answers chosen there are kept only while it is open, so it says
 * they are not saved, beside the eye, and never claims a save.
 */
export function UnsavedLine() {
  const { t } = useTranslation();
  return (
    <span className="flex min-w-0 items-center gap-[5px]">
      <Eye aria-hidden="true" className="size-3 flex-none" />
      <span className="truncate">{t("takeTest.preview.notSaved")}</span>
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
  const status = usePaper().saveStatus;
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
 * neither it is absent, as the deck draws it. The save line wraps rather than
 * truncates, so a narrow phone shows the whole message. From 768 the header holds both
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
  const { lock, saveStatus: status } = usePaper();

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
      {trouble && <SaveState wrap />}
      {indicator !== null && <span className="ml-auto flex-none">{indicator}</span>}
    </div>
  );
}
