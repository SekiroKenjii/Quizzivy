import { forwardRef, useRef, useState, type ComponentPropsWithoutRef } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";
import { Bell } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Sheet } from "@/components/shared/Sheet";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import { NotificationsPanel, type NotificationAudience } from "./NotificationsPanel";

const FRAME: Record<
  NotificationAudience,
  { button: string; icon: string; dot: string; alignOffset: number }
> = {
  teacher: {
    button: "size-8.5 rounded-md",
    icon: "size-[1.0625rem]",
    dot: "top-1.75 right-2",
    alignOffset: 0,
  },
  student: {
    button: "size-9 rounded-md",
    icon: "size-4.5",
    dot: "top-2 right-2.25",
    alignOffset: -44,
  },
};

type BellButtonProps = ComponentPropsWithoutRef<"button"> & {
  audience: NotificationAudience;
  unread: number;
};

const BellButton = forwardRef<HTMLButtonElement, BellButtonProps>(function BellButton(
  { audience, unread, className, ...props },
  ref,
) {
  const { t } = useTranslation();
  const frame = FRAME[audience];
  return (
    <button
      ref={ref}
      type="button"
      aria-label={
        unread > 0
          ? t("notifications.bell.labelUnread", { count: unread })
          : t("notifications.bell.label")
      }
      className={cn(
        "hover:bg-hover relative grid flex-none cursor-pointer place-items-center",
        frame.button,
        className,
      )}
      {...props}
    >
      <Bell aria-hidden="true" className={frame.icon} />
      {unread > 0 ? (
        <span
          data-slot="bell-dot"
          aria-hidden="true"
          className={cn(
            "bg-danger border-bg absolute size-1.75 rounded-full border-[1.5px]",
            frame.dot,
          )}
        />
      ) : null}
    </button>
  );
});

/**
 * NotificationBell is the shell's bell: a 7px dot while `unread` is above
 * zero, and the notifications of `audience`'s kinds in a 340px popover from
 * 768px, which on the student's bar ends at the bar's edge as drawn, or the
 * shared 380px sheet, the whole width of a phone, below it. Both return
 * focus to the bell when they close, except after a row sends the reader to
 * another page, whose shell then takes focus. `canGrade` is the reader's
 * reach of Grading.
 */
export function NotificationBell({
  audience,
  unread,
  canGrade = false,
}: Readonly<{
  audience: NotificationAudience;
  unread: number;
  canGrade?: boolean;
}>) {
  const { t } = useTranslation();
  const wide = useMediaQuery("(min-width: 768px)");
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [open, setOpen] = useState(false);
  const leaving = useRef(false);

  const pick = (to: string | null) => {
    if (to === null) return;
    leaving.current = to !== pathname;
    setOpen(false);
    void navigate(to);
  };
  const panel = (heading: boolean) => (
    <NotificationsPanel
      audience={audience}
      canGrade={canGrade}
      heading={heading}
      onPick={pick}
    />
  );

  if (!wide)
    return (
      <>
        <BellButton
          audience={audience}
          unread={unread}
          aria-haspopup="dialog"
          aria-expanded={open}
          onClick={() => setOpen(true)}
        />
        <Sheet
          open={open}
          onOpenChange={setOpen}
          title={t("notifications.bell.title")}
          width={380}
        >
          <div className="-m-3.5 flex min-h-0 flex-1 flex-col">{panel(false)}</div>
        </Sheet>
      </>
    );

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <BellButton audience={audience} unread={unread} />
      </PopoverTrigger>
      <PopoverContent
        align="end"
        alignOffset={FRAME[audience].alignOffset}
        sideOffset={8}
        aria-label={t("notifications.bell.title")}
        onCloseAutoFocus={(event) => {
          if (!leaving.current) return;
          leaving.current = false;
          event.preventDefault();
        }}
        className="bg-card text-fg z-(--z-popover) flex max-h-[min(32rem,calc(100svh-5rem))] w-[340px] max-w-[calc(100vw-24px)] flex-col overflow-hidden rounded-xl border p-0 data-[scale=deck]:rounded-xl data-[scale=deck]:p-0"
      >
        {panel(true)}
      </PopoverContent>
    </Popover>
  );
}
