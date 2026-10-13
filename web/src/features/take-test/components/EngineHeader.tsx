import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Eye, Timer, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Clock } from "./Clock";
import { SaveAnnouncement, SaveState, UnsavedLine } from "./SaveState";
import { usePaper } from "../paper";

/**
 * EngineHeader is the engine's 60px header, as the deck draws it: the leading
 * control, from 768 the test's title over the save line, the timer centred in
 * the space left, and Submit. The strike count, which the deck does not draw,
 * follows the save line from 768. A locked paper has no save line: the strip
 * under the header says why it is locked. A paper that has ended has no timer
 * either; one another device took over, or one whose time is up, keeps it.
 * `live` is false once the paper is submitted, when only the title is left.
 * A `preview` header, a teacher's, has no leading control and no Submit but
 * the chip "Student view"; its save line says nothing is saved, and the
 * timer's place holds a pill of the same size with no time, because a time
 * limit is the assignment's and a preview has none. While the paper is live
 * its title is the page's `<h1>`, read but not drawn below 768; a preview's
 * is an `<h2>`, because the teacher's page around it has the `<h1>`. A
 * submitted paper's title is plain text, since the end state has the `<h1>`.
 */
export function EngineHeader({
  wide,
  leading,
  status = null,
  live = true,
  preview = false,
  onSubmit,
}: Readonly<{
  wide: boolean;
  leading: ReactNode;
  status?: ReactNode;
  live?: boolean;
  preview?: boolean;
  onSubmit?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const { title, lock } = usePaper();
  const locked = lock !== null;
  const closed = lock === "closed";
  const clock = preview ? <NoTimeLimit /> : <Clock />;

  return (
    <header
      className={cn(
        "flex h-15 flex-none items-center gap-3 border-b",
        wide ? "px-6" : "px-3.5",
      )}
    >
      {leading}
      {!wide && live && (
        <PaperTitle live preview={preview} className="sr-only">
          {title}
        </PaperTitle>
      )}
      {wide && (
        <div className="flex min-w-0 flex-col">
          <PaperTitle
            live={live}
            preview={preview}
            className="truncate text-base leading-[1.3] font-semibold"
          >
            {title}
          </PaperTitle>
          {live && !locked && (
            <span className="text-muted-fg flex min-w-0 items-center gap-[5px] text-xs leading-[1.3]">
              {preview ? <UnsavedLine /> : <SaveState />}
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
      {live && !closed && clock}
      {live && !locked && !preview && <SaveAnnouncement />}
      {preview && <StudentViewChip />}
      {!preview && onSubmit !== undefined && (
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

function PaperTitle({
  live,
  preview,
  className,
  children,
}: Readonly<{
  live: boolean;
  preview: boolean;
  className: string;
  children: ReactNode;
}>) {
  if (!live) return <span className={className}>{children}</span>;
  if (preview) return <h2 className={className}>{children}</h2>;
  return <h1 className={className}>{children}</h1>;
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

function NoTimeLimit() {
  const { t } = useTranslation();
  const label = t("takeTest.preview.noTimeLimit");
  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className="text-md bg-muted text-muted-fg mx-auto inline-flex h-9 flex-none items-center gap-[7px] rounded-full px-3.5 font-semibold whitespace-nowrap"
    >
      <Timer aria-hidden="true" className="size-4" />
      <span aria-hidden="true">
        {["c0", "c1", ":", "c3", "c4"].map((cell) =>
          cell === ":" ? (
            cell
          ) : (
            <span key={cell} className="inline-block w-[1ch] text-center">
              –
            </span>
          ),
        )}
      </span>
    </span>
  );
}

function StudentViewChip() {
  const { t } = useTranslation();
  return (
    <span className="bg-info-soft text-info-ink text-meta inline-flex h-7.5 flex-none items-center gap-1.5 rounded-full px-2.5 leading-none font-semibold whitespace-nowrap">
      <Eye aria-hidden="true" className="size-3.5" />
      {t("takeTest.preview.studentView")}
    </span>
  );
}
