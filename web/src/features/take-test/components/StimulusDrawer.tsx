import type { Ref } from "react";
import { useTranslation } from "react-i18next";
import { AudioLines, ChevronsLeft, ChevronsRight, FileText, X } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * StimulusKind is what a group's drawer holds: something to read, or only a
 * recording.
 */
export type StimulusKind = "passage" | "recording";

/**
 * DrawerHandle is the closed stimulus drawer's tab at the left edge of the
 * panes, 42% down: the passage's or the recording's icon, a dot in the
 * accent while the group's recording plays, and the chevrons. Until the
 * student has opened the group's drawer once (`nudge`) it nudges three times;
 * under reduced motion it stays still.
 */
export function DrawerHandle({
  ref,
  kind,
  playing,
  nudge,
  onOpen,
}: Readonly<{
  ref: Ref<HTMLButtonElement>;
  kind: StimulusKind;
  playing: boolean;
  nudge: boolean;
  onOpen: () => void;
}>) {
  const { t } = useTranslation();
  const Icon = kind === "passage" ? FileText : AudioLines;
  return (
    <button
      ref={ref}
      type="button"
      aria-label={t(
        kind === "passage" ? "takeTest.openPassage" : "takeTest.openRecording",
      )}
      className={cn(
        "bg-card text-fg shadow-float absolute top-[42%] left-0 z-50 flex h-21 min-h-0 w-7.5 min-w-0 flex-col items-center justify-center gap-1.5 rounded-r-[12px] border border-l-0 p-0",
        nudge &&
          "animate-[qz-nudge_1.4s_ease-in-out_0.4s_3] motion-reduce:animate-none",
      )}
      onClick={onOpen}
    >
      <Icon aria-hidden="true" className="size-[15px]" />
      {playing && (
        <span
          aria-hidden="true"
          className="bg-brand size-[7px] flex-none rounded-full"
        />
      )}
      <ChevronsRight aria-hidden="true" className="text-muted-fg size-3.5" />
    </button>
  );
}

/**
 * DrawerTip says once, beside the handle, where the passage or the recording
 * went and how to open it. It is a status, so a screen reader hears it.
 */
export function DrawerTip({ kind }: Readonly<{ kind: StimulusKind }>) {
  const { t } = useTranslation();
  return (
    <span
      role="status"
      className="bg-primary text-primary-fg text-meta shadow-float pointer-events-none absolute top-[calc(42%+42px)] left-9.5 z-50 max-w-55 -translate-y-1/2 animate-[qz-tip_0.3s_ease-out_both] rounded-[9px] px-2.5 py-2 leading-[1.4] motion-reduce:animate-none"
    >
      {t(kind === "passage" ? "takeTest.tipPassage" : "takeTest.tipRecording")}
    </span>
  );
}

/**
 * DrawerBackdrop dims the question under an open or moving drawer, as far as
 * `progress` (0 to 1) says the drawer has come out, and closes it on a tap.
 */
export function DrawerBackdrop({
  progress,
  onClose,
}: Readonly<{ progress: number; onClose: () => void }>) {
  return (
    <div
      aria-hidden="true"
      data-drawer-backdrop=""
      className="bg-overlay absolute inset-0 z-[55]"
      style={{ opacity: progress }}
      onClick={onClose}
    />
  );
}

/**
 * DrawerBar sits at the top of the open drawer and stays there while it
 * scrolls: how to go back to the question, and the close button.
 */
export function DrawerBar({
  closeRef,
  onClose,
}: Readonly<{ closeRef: Ref<HTMLButtonElement>; onClose: () => void }>) {
  const { t } = useTranslation();
  return (
    <div className="bg-bg sticky -top-4.5 z-[2] -mx-4 -mt-4.5 mb-3.5 flex items-center justify-between gap-2.5 border-b py-2.5 pr-3 pl-4">
      <span className="text-muted-fg flex min-w-0 items-center gap-1.5 text-xs leading-normal">
        <ChevronsLeft aria-hidden="true" className="size-3.5 flex-none" />
        {t("takeTest.drawerHint")}
      </span>
      <button
        ref={closeRef}
        type="button"
        aria-label={t("takeTest.drawerClose")}
        className="hover:bg-muted text-fg grid size-8.5 min-h-0 min-w-0 flex-none place-items-center rounded-md"
        onClick={onClose}
      >
        <X aria-hidden="true" className="size-[18px]" />
      </button>
    </div>
  );
}
