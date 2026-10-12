import {
  useRef,
  type ComponentProps,
  type CSSProperties,
  type ReactNode,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import { X, type LucideIcon } from "lucide-react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { preventLocalEscapeDismissal } from "@/lib/localEscape";
import { cn } from "@/lib/utils";

/** DialogShell is the teacher's controlled, instant dialog frame with captured return focus. */
export function DialogShell({
  open,
  onOpenChange,
  width = 480,
  className,
  returnFocus,
  children,
  onOpenAutoFocus,
  onCloseAutoFocus,
  onEscapeKeyDown,
  ...props
}: Readonly<
  Omit<ComponentProps<typeof DialogContent>, "width" | "children"> & {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    width?: number | undefined;
    returnFocus?: RefObject<HTMLElement | null> | undefined;
    children: ReactNode;
  }
>) {
  const opener = useRef<HTMLElement | null>(null);
  const page = useRef<HTMLElement | null>(null);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        {...props}
        showCloseButton={false}
        className={cn(
          "bg-card shadow-float flex max-h-[86dvh] w-[min(var(--dialog-width),calc(100%-24px))] max-w-none flex-col gap-0 overflow-hidden rounded-xl p-0 transition-none duration-0 data-[state=closed]:animate-none data-[state=open]:animate-none sm:max-w-none",
          className,
        )}
        style={{ "--dialog-width": `${width}px`, ...props.style } as CSSProperties}
        overlayClassName="data-[state=open]:animate-none data-[state=closed]:animate-none"
        onOpenAutoFocus={(event) => {
          const active = document.activeElement;
          opener.current =
            active instanceof HTMLElement && active !== document.body ? active : null;
          page.current =
            (returnFocus?.current?.isConnected
              ? returnFocus.current.closest<HTMLElement>("main")
              : null) ??
            opener.current?.closest("main") ??
            null;
          onOpenAutoFocus?.(event);
        }}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          const active = document.activeElement;
          const moved =
            active instanceof HTMLElement &&
            active !== document.body &&
            !(
              event.currentTarget instanceof Node &&
              event.currentTarget.contains(active)
            );
          let target = page.current;
          if (opener.current?.isConnected) target = opener.current;
          if (returnFocus?.current?.isConnected) target = returnFocus.current;
          opener.current = null;
          page.current = null;
          if (!moved && target?.isConnected) target.focus();
          onCloseAutoFocus?.(event);
        }}
        onEscapeKeyDown={(event) => {
          preventLocalEscapeDismissal(event);
          onEscapeKeyDown?.(event);
        }}
      >
        {children}
      </DialogContent>
    </Dialog>
  );
}

/** DialogShellHeader names the dialog and draws its optional icon, description and close control. */
export function DialogShellHeader({
  title,
  description,
  icon: Icon,
  tone = "neutral",
}: Readonly<{
  title: string;
  description?: ReactNode;
  icon?: LucideIcon | undefined;
  tone?: "neutral" | "danger";
}>) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-none items-center gap-3 pt-4.5 pr-3.5 pb-1 pl-4.5">
      {Icon && (
        <span
          className={cn(
            "grid size-9 flex-none place-items-center rounded-lg",
            tone === "danger" ? "bg-danger-soft text-danger-ink" : "bg-muted text-fg",
          )}
        >
          <Icon aria-hidden="true" className="size-4.5" />
        </span>
      )}
      <div className="min-w-0 flex-1">
        <DialogTitle className="text-title leading-[1.35] font-semibold">
          {title}
        </DialogTitle>
        {description !== undefined && description !== null && (
          <DialogDescription className="text-muted-fg mt-0.75 text-sm leading-normal text-pretty">
            {description}
          </DialogDescription>
        )}
      </div>
      <DialogClose
        type="button"
        className="text-muted-fg hover:bg-hover hover:text-fg rounded-seg grid size-7.5 flex-none place-items-center"
        aria-label={t("common.close")}
      >
        <X aria-hidden="true" className="size-4" />
      </DialogClose>
    </div>
  );
}

/** DialogShellBody is the frame's only scroller, with the deck's field spacing. */
export function DialogShellBody({
  className,
  ...props
}: Readonly<ComponentProps<"div">>) {
  return (
    <div
      className={cn(
        "flex min-h-0 flex-1 flex-col gap-3.5 overflow-x-hidden overflow-y-auto px-4.5 pt-3.5 pb-4",
        className,
      )}
      {...props}
    />
  );
}

/** DialogShellFooter keeps wrapping actions reachable below the scrolling body. */
export function DialogShellFooter({
  className,
  ...props
}: Readonly<ComponentProps<"div">>) {
  return (
    <div
      className={cn(
        "flex flex-none flex-wrap justify-end gap-2 border-t px-4.5 py-3",
        className,
      )}
      {...props}
    />
  );
}
