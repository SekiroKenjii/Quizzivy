import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type EventTone = "info" | "danger" | "success" | "warning" | "neutral";

const DOTS: Record<EventTone, string> = {
  info: "bg-info",
  danger: "bg-danger",
  success: "bg-success",
  warning: "bg-warning",
  neutral: "bg-border",
};

/** EventListItem describes an event with its displayed time and decorative tone. */
export type EventListItem = Readonly<{
  key: string;
  tone: EventTone;
  text: ReactNode;
  time: string;
}>;

/** EventListProps supplies the list label and events in display order. */
export type EventListProps = Readonly<{
  label: string;
  items: readonly EventListItem[];
  className?: string | undefined;
}>;

/** EventList preserves caller order with decorative dots and renders nothing when empty. */
export function EventList({ label, items, className }: EventListProps) {
  if (items.length === 0) return null;
  return (
    <ol aria-label={label} className={className}>
      {items.map((item) => (
        <li key={item.key} className="flex gap-3 pb-3.5">
          <span
            aria-hidden="true"
            className={cn(
              "ring-card mt-1.25 size-2.5 shrink-0 rounded-full ring-3",
              DOTS[item.tone],
            )}
          />
          <div className="min-w-0 flex-1 text-sm">
            <div>{item.text}</div>
            <div className="text-muted-fg text-xs leading-normal">{item.time}</div>
          </div>
        </li>
      ))}
    </ol>
  );
}
