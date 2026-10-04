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

/**
 * EventListItem is one line of an EventList: the tone of its dot, what
 * happened, and when, as the screen formats it.
 */
export type EventListItem = Readonly<{
  key: string;
  tone: EventTone;
  text: ReactNode;
  time: string;
}>;

/**
 * EventListProps is what an EventList takes: the `label` that names the list
 * and its events, in the order they are drawn.
 */
export type EventListProps = Readonly<{
  label: string;
  items: readonly EventListItem[];
  className?: string | undefined;
}>;

/**
 * EventList is the deck's list of things that happened, oldest first: a
 * round dot, a line of text and the time under it, with no line joining the
 * dots. It is an ordered list named by `label`. The dot is decoration and
 * hidden from assistive technology: its tone says nothing the text does not,
 * so a screen never colours a judgement it does not also write. Every item,
 * the last included, keeps the deck's 14px below it. An empty list renders
 * nothing; the screen owns the sentence for that.
 */
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
