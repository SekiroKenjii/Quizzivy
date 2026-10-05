import * as React from "react";
import { ChevronDownIcon, ChevronLeftIcon, ChevronRightIcon } from "lucide-react";
import { DayPicker, getDefaultClassNames, type DayButton } from "react-day-picker";

import { Button, buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useDeckScale } from "@/components/ui/deck-scale";

/** Calendar draws the inherited grid or the deck's six-week date selection surface. */
function Calendar({
  className,
  classNames,
  showOutsideDays = true,
  captionLayout = "label",
  buttonVariant = "ghost",
  formatters,
  components,
  ...props
}: React.ComponentProps<typeof DayPicker> & {
  buttonVariant?: React.ComponentProps<typeof Button>["variant"];
}) {
  const deck = useDeckScale();
  const defaultClassNames = getDefaultClassNames();

  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      className={cn(
        "group/calendar bg-background p-3 [--cell-size:--spacing(8)] [[data-slot=card-content]_&]:bg-transparent [[data-slot=popover-content]_&]:bg-transparent",
        String.raw`rtl:**:[.rdp-button\_next>svg]:rotate-180`,
        String.raw`rtl:**:[.rdp-button\_previous>svg]:rotate-180`,
        deckClass(
          deck,
          "in-data-[scale=deck]:w-67 in-data-[scale=deck]:p-2 in-data-[scale=deck]:[--cell-size:34px]",
        ),
        className,
      )}
      captionLayout={captionLayout}
      formatters={{
        formatMonthDropdown: (date) =>
          date.toLocaleString("default", { month: "short" }),
        ...formatters,
      }}
      classNames={{
        root: cn("w-fit", defaultClassNames.root),
        months: cn(
          "relative flex flex-col gap-4 md:flex-row",
          deckClass(deck, "in-data-[scale=deck]:gap-0"),
          defaultClassNames.months,
        ),
        month: cn(
          "flex w-full flex-col gap-4",
          deckClass(deck, "in-data-[scale=deck]:gap-0.5"),
          defaultClassNames.month,
        ),
        nav: cn(
          "absolute inset-x-0 top-0 flex w-full items-center justify-between gap-1",
          deckClass(deck, "in-data-[scale=deck]:top-0.5 in-data-[scale=deck]:px-0.5"),
          defaultClassNames.nav,
        ),
        button_previous: cn(
          buttonVariants({ variant: buttonVariant }),
          "size-(--cell-size) p-0 select-none aria-disabled:opacity-50",
          deckClass(
            deck,
            "in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:w-7.5 in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:border in-data-[scale=deck]:bg-card in-data-[scale=deck]:hover:bg-muted in-data-[scale=deck]:[&_svg]:size-3.75",
          ),
          defaultClassNames.button_previous,
        ),
        button_next: cn(
          buttonVariants({ variant: buttonVariant }),
          "size-(--cell-size) p-0 select-none aria-disabled:opacity-50",
          deckClass(
            deck,
            "in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:w-7.5 in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:border in-data-[scale=deck]:bg-card in-data-[scale=deck]:hover:bg-muted in-data-[scale=deck]:[&_svg]:size-3.75",
          ),
          defaultClassNames.button_next,
        ),
        month_caption: cn(
          "flex h-(--cell-size) w-full items-center justify-center px-(--cell-size)",
          deckClass(deck, "in-data-[scale=deck]:h-10 in-data-[scale=deck]:text-ui"),
          defaultClassNames.month_caption,
        ),
        dropdowns: cn(
          "flex h-(--cell-size) w-full items-center justify-center gap-1.5 text-sm font-medium",
          defaultClassNames.dropdowns,
        ),
        dropdown_root: cn(
          "border-input has-focus:border-ring has-focus:ring-ring/50 relative rounded-md border shadow-xs has-focus:ring-[3px]",
          defaultClassNames.dropdown_root,
        ),
        dropdown: cn(
          "bg-popover absolute inset-0 opacity-0",
          defaultClassNames.dropdown,
        ),
        caption_label: cn(
          "font-medium select-none",
          captionLayout === "label"
            ? "text-sm"
            : "[&>svg]:text-muted-foreground flex h-8 items-center gap-1 rounded-md pr-1 pl-2 text-sm [&>svg]:size-3.5",
          deckClass(
            deck,
            "in-data-[scale=deck]:text-ui in-data-[scale=deck]:font-semibold",
          ),
          defaultClassNames.caption_label,
        ),
        month_grid: cn("w-full border-collapse", defaultClassNames.month_grid),
        weekdays: cn(
          "flex",
          deckClass(deck, "in-data-[scale=deck]:gap-0.5"),
          defaultClassNames.weekdays,
        ),
        weekday: cn(
          "text-muted-foreground flex-1 rounded-md text-[0.8rem] font-normal select-none",
          deckClass(
            deck,
            "in-data-[scale=deck]:h-6.5 in-data-[scale=deck]:grid in-data-[scale=deck]:place-items-center in-data-[scale=deck]:text-caption in-data-[scale=deck]:font-medium in-data-[scale=deck]:rounded-none",
          ),
          defaultClassNames.weekday,
        ),
        week: cn(
          "mt-2 flex w-full",
          deckClass(deck, "in-data-[scale=deck]:mt-0.5 in-data-[scale=deck]:gap-0.5"),
          defaultClassNames.week,
        ),
        week_number_header: cn(
          "w-(--cell-size) select-none",
          defaultClassNames.week_number_header,
        ),
        week_number: cn(
          "text-muted-foreground text-[0.8rem] select-none",
          defaultClassNames.week_number,
        ),
        day: cn(
          "group/day relative aspect-square h-full w-full p-0 text-center select-none [&:last-child[data-selected=true]_button]:rounded-r-md",
          props.showWeekNumber
            ? "[&:nth-child(2)[data-selected=true]_button]:rounded-l-md"
            : "[&:first-child[data-selected=true]_button]:rounded-l-md",
          deckClass(
            deck,
            "in-data-[scale=deck]:aspect-auto in-data-[scale=deck]:h-8.5",
          ),
          defaultClassNames.day,
        ),
        range_start: cn("bg-accent rounded-l-md", defaultClassNames.range_start),
        range_middle: cn("rounded-none", defaultClassNames.range_middle),
        range_end: cn("bg-accent rounded-r-md", defaultClassNames.range_end),
        today: cn(
          "bg-accent text-accent-foreground rounded-md data-[selected=true]:rounded-none",
          deckClass(
            deck,
            "in-data-[scale=deck]:bg-transparent in-data-[scale=deck]:text-fg in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:inset-shadow-[0_0_0_1px_var(--ring)]",
          ),
          defaultClassNames.today,
        ),
        outside: cn(
          "text-muted-foreground aria-selected:text-muted-foreground",
          deckClass(
            deck,
            "in-data-[scale=deck]:text-muted-fg in-data-[scale=deck]:opacity-55",
          ),
          defaultClassNames.outside,
        ),
        disabled: cn("text-muted-foreground opacity-50", defaultClassNames.disabled),
        hidden: cn("invisible", defaultClassNames.hidden),
        ...classNames,
      }}
      components={{
        Root: ({ className, rootRef, ...props }) => (
          <div
            data-slot="calendar"
            ref={rootRef}
            className={cn(className)}
            {...props}
          />
        ),
        Chevron: ({ className, orientation, ...props }) => {
          if (orientation === "left") {
            return <ChevronLeftIcon className={cn("size-4", className)} {...props} />;
          }
          if (orientation === "right") {
            return <ChevronRightIcon className={cn("size-4", className)} {...props} />;
          }
          return <ChevronDownIcon className={cn("size-4", className)} {...props} />;
        },
        DayButton: CalendarDayButton,
        WeekNumber: ({ children, ...props }) => (
          <td {...props}>
            <div className="flex size-(--cell-size) items-center justify-center text-center">
              {children}
            </div>
          </td>
        ),
        ...components,
      }}
      {...props}
    />
  );
}

/** CalendarDayButton preserves day-picker focus and the scoped day geometry. */
function CalendarDayButton({
  className,
  day,
  modifiers,
  ...props
}: React.ComponentProps<typeof DayButton>) {
  const deck = useDeckScale();
  const defaultClassNames = getDefaultClassNames();

  const ref = React.useRef<HTMLButtonElement>(null);
  React.useEffect(() => {
    if (modifiers.focused) ref.current?.focus();
  }, [modifiers.focused]);

  return (
    <Button
      ref={ref}
      variant="ghost"
      size="icon"
      data-day={day.date.toLocaleDateString()}
      data-selected-single={
        modifiers.selected &&
        !modifiers.range_start &&
        !modifiers.range_end &&
        !modifiers.range_middle
      }
      data-range-start={modifiers.range_start}
      data-range-end={modifiers.range_end}
      data-range-middle={modifiers.range_middle}
      className={cn(
        "data-[selected-single=true]:bg-primary data-[selected-single=true]:text-primary-foreground data-[range-middle=true]:bg-accent data-[range-middle=true]:text-accent-foreground data-[range-start=true]:bg-primary data-[range-start=true]:text-primary-foreground data-[range-end=true]:bg-primary data-[range-end=true]:text-primary-foreground group-data-[focused=true]/day:border-ring group-data-[focused=true]/day:ring-ring/50 dark:hover:text-accent-foreground flex aspect-square size-auto w-full min-w-(--cell-size) flex-col gap-1 leading-none font-normal group-data-[focused=true]/day:relative group-data-[focused=true]/day:z-10 group-data-[focused=true]/day:ring-[3px] data-[range-end=true]:rounded-md data-[range-end=true]:rounded-r-md data-[range-middle=true]:rounded-none data-[range-start=true]:rounded-md data-[range-start=true]:rounded-l-md [&>span]:text-xs [&>span]:opacity-70",
        deckClass(
          deck,
          "in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:aspect-auto in-data-[scale=deck]:h-8.5 in-data-[scale=deck]:min-w-0 in-data-[scale=deck]:text-sm in-data-[scale=deck]:leading-normal in-data-[scale=deck]:tabular-nums in-data-[scale=deck]:data-[selected-single=true]:font-semibold",
        ),
        defaultClassNames.day,
        className,
      )}
      {...props}
    />
  );
}

export { Calendar, CalendarDayButton };

function deckClass(deck: boolean, value: string): string | undefined {
  return deck ? value : undefined;
}
