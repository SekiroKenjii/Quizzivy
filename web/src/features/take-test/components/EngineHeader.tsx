import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Clock } from "./Clock";
import { SaveAnnouncement, SaveState } from "./SaveState";
import { useTakeTestStore } from "../store";

/**
 * EngineHeader is the engine's 60px header, as the deck draws it: the leading
 * control, from 768 the test's title over the save line, the timer centred in
 * the space left, and Submit. The strike count, which the deck does not draw,
 * follows the save line from 768. A locked paper has no save line: the strip
 * under the header says why it is locked. `live` is false once the paper is
 * submitted, when only the title is left.
 */
export function EngineHeader({
  wide,
  leading,
  status = null,
  live = true,
  onSubmit,
}: Readonly<{
  wide: boolean;
  leading: ReactNode;
  status?: ReactNode;
  live?: boolean;
  onSubmit?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const title = useTakeTestStore((s) => s.testTitle);
  const locked = useTakeTestStore((s) => s.lock !== null);

  return (
    <header
      className={cn(
        "flex h-15 flex-none items-center gap-3 border-b",
        wide ? "px-6" : "px-3.5",
      )}
    >
      {leading}
      {wide && (
        <div className="flex min-w-0 flex-col">
          <span className="truncate text-base leading-[1.3] font-semibold">
            {title}
          </span>
          {live && !locked && (
            <span className="text-muted-fg flex min-w-0 items-center gap-[5px] text-xs leading-[1.3]">
              <SaveState />
              {status !== null && (
                <>
                  <span aria-hidden="true">·</span>
                  <span className="flex-none">{status}</span>
                </>
              )}
            </span>
          )}
        </div>
      )}
      {live && <Clock />}
      {live && !locked && <SaveAnnouncement />}
      {onSubmit !== undefined && (
        <Button
          size="md"
          className="min-h-0 min-w-0 px-4 text-base font-semibold"
          onClick={onSubmit}
        >
          {t("takeTest.submit")}
        </Button>
      )}
    </header>
  );
}

/**
 * LeaveButton is the header's 36px ✕, named "Leave test". It asks before it
 * leaves; the caller opens the Leave dialog.
 */
export function LeaveButton({ onClick }: Readonly<{ onClick: () => void }>) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      aria-label={t("takeTest.leave")}
      className="hover:bg-hover grid size-9 min-h-0 min-w-0 flex-none place-items-center rounded-md"
      onClick={onClick}
    >
      <X aria-hidden="true" className="size-4.5" />
    </button>
  );
}
