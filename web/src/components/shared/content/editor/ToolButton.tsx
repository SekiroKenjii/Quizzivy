import type { ComponentProps, PointerEvent, MouseEvent } from "react";
import { ChevronDown, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

function keepSelection(event: PointerEvent | MouseEvent) {
  event.preventDefault();
}

/**
 * ToolButton is a 32px icon button of the editor's toolbar. It names itself
 * with `label`, shows `title` as its tooltip, and keeps the editor's
 * selection when pressed with a mouse, a pen or a finger. A menu's trigger
 * passes `menu`: it draws the deck's caret and leaves the press to the menu,
 * which keeps the selection itself.
 */
export function ToolButton({
  icon: Icon,
  label,
  title,
  className,
  menu = false,
  onPointerDown,
  onMouseDown,
  ...props
}: Readonly<
  ComponentProps<"button"> & {
    icon: LucideIcon;
    label: string;
    title: string;
    menu?: boolean;
  }
>) {
  return (
    <button
      type="button"
      aria-label={label}
      title={title}
      onPointerDown={(event) => {
        if (!menu) keepSelection(event);
        onPointerDown?.(event);
      }}
      onMouseDown={(event) => {
        if (!menu) keepSelection(event);
        onMouseDown?.(event);
      }}
      className={cn(
        menu && "gap-0.5 px-1.5",
        "text-fg hover:bg-hover aria-pressed:border-border aria-pressed:bg-muted data-[active=true]:border-border data-[active=true]:bg-muted aria-expanded:bg-muted inline-flex h-8 min-w-8 flex-none items-center justify-center rounded-[7px] border border-transparent transition-colors disabled:opacity-40 disabled:hover:bg-transparent motion-reduce:transition-none",
        className,
      )}
      {...props}
    >
      <Icon size={16} aria-hidden="true" />
      {menu && <ChevronDown size={12} aria-hidden="true" className="text-muted-fg" />}
    </button>
  );
}
