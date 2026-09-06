import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { format } from "date-fns";
import { vi, enUS } from "date-fns/locale";
import { CalendarIcon, ClockIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

const LOCALES = { vi, en: enUS } as const;
const HOURS = Array.from({ length: 24 }, (_, h) => h);
const DEFAULT_TIME = "08:00";

export type DateTimeMode = "date" | "time" | "datetime";

interface DateTimeFieldProps {
  readonly id?: string;
  /** Names the halves for assistive tech; the label element names the first half through `id`. */
  readonly label: string;
  /** `yyyy-MM-ddTHH:mm`, `yyyy-MM-dd` or `HH:mm`, by mode; empty when nothing is chosen yet. */
  readonly value: string;
  readonly onChange: (next: string) => void;
  readonly mode?: DateTimeMode;
  /** Minutes offered in the clock; a value off the step is still shown and kept. */
  readonly minuteStep?: number;
  readonly className?: string;
}

/**
 * F-06's date-time field: shadcn's date picker and a clock drawn as one joined
 * pair of outline buttons, never the platform's `datetime-local` popup, which
 * speaks the browser's language and format. Callers pin the value to
 * Asia/Ho_Chi_Minh through `fromDateTimeInput` / `toDateTimeInput`.
 */
export function DateTimeField({
  id,
  label,
  value,
  onChange,
  mode = "datetime",
  minuteStep = 5,
  className,
}: DateTimeFieldProps) {
  const { date, time } = split(value, mode);
  const emit = (nextDate: string, nextTime: string) =>
    onChange(join(nextDate, nextTime, mode));
  const both = mode === "datetime";

  return (
    <div role="group" aria-label={label} className={cn("flex", className)}>
      {mode !== "time" && (
        <DayPart
          id={id}
          date={date}
          className={both ? "rounded-r-none" : undefined}
          onChange={(next) => emit(next, time || DEFAULT_TIME)}
        />
      )}
      {mode !== "date" && (
        <ClockPart
          id={mode === "time" ? id : undefined}
          label={label}
          time={time}
          minuteStep={minuteStep}
          className={both ? "-ml-px w-28 rounded-l-none" : "flex-1"}
          onChange={(next) => emit(date, next)}
        />
      )}
    </div>
  );
}

function DayPart({
  id,
  date,
  className,
  onChange,
}: {
  readonly id: string | undefined;
  readonly date: string;
  readonly className: string | undefined;
  readonly onChange: (next: string) => void;
}) {
  const { t } = useTranslation();
  const locale = useLocale();
  const [open, setOpen] = useState(false);
  const selected = date === "" ? undefined : fromDayKey(date);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          id={id}
          variant="outline"
          className={cn(
            "flex-1 justify-between font-normal",
            selected === undefined && "text-muted-foreground",
            className,
          )}
        >
          {selected === undefined ? t("common.pickDate") : formatDay(selected)}
          <CalendarIcon aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto overflow-hidden p-0" align="start">
        <Calendar
          mode="single"
          locale={LOCALES[locale]}
          labels={{
            labelPrevious: () => t("common.calendar.previousMonth"),
            labelNext: () => t("common.calendar.nextMonth"),
            labelNav: () => t("common.calendar.nav"),
            labelDayButton: (day, modifiers) =>
              [
                format(day, "PPPP", { locale: LOCALES[locale] }),
                modifiers.today ? t("common.calendar.today") : null,
                modifiers.selected ? t("common.calendar.selected") : null,
              ]
                .filter((part) => part !== null)
                .join(", "),
          }}
          selected={selected}
          {...(selected === undefined ? {} : { defaultMonth: selected })}
          onSelect={(day) => {
            if (day === undefined) return;
            onChange(toDayKey(day));
            setOpen(false);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}

function ClockPart({
  id,
  label,
  time,
  minuteStep,
  className,
  onChange,
}: {
  readonly id: string | undefined;
  readonly label: string;
  readonly time: string;
  readonly minuteStep: number;
  readonly className: string;
  readonly onChange: (next: string) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [hour, minute] = time === "" ? [null, null] : time.split(":").map(Number);
  const minutes = minuteOptions(minuteStep, minute ?? null);
  const pad = (n: number) => String(n).padStart(2, "0");

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          id={id}
          variant="outline"
          aria-label={t("common.timeOf", { label })}
          className={cn(
            "justify-between font-normal tabular-nums",
            time === "" && "text-muted-foreground",
            className,
          )}
        >
          {time === "" ? t("common.pickTime") : time}
          <ClockIcon aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-2" align="end">
        <div className="flex gap-2">
          <Column
            title={t("common.calendar.hours")}
            options={HOURS}
            selected={hour ?? null}
            open={open}
            onPick={(h) => onChange(`${pad(h)}:${pad(minute ?? 0)}`)}
          />
          <Column
            title={t("common.calendar.minutes")}
            options={minutes}
            selected={minute ?? null}
            open={open}
            onPick={(m) => {
              onChange(`${pad(hour ?? 0)}:${pad(m)}`);
              setOpen(false);
            }}
          />
        </div>
      </PopoverContent>
    </Popover>
  );
}

/** One scrolling list of the clock; the chosen entry is scrolled into view when the popover opens. */
function Column({
  title,
  options,
  selected,
  open,
  onPick,
}: {
  readonly title: string;
  readonly options: readonly number[];
  readonly selected: number | null;
  readonly open: boolean;
  readonly onPick: (value: number) => void;
}) {
  const chosen = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    if (open) chosen.current?.scrollIntoView({ block: "center" });
  }, [open]);
  return (
    <div role="listbox" aria-label={title} className="flex flex-col">
      <p className="text-muted-foreground px-1 pb-1 text-center text-xs">{title}</p>
      <div className="flex max-h-56 flex-col gap-0.5 overflow-y-auto">
        {options.map((option) => (
          <button
            key={option}
            type="button"
            role="option"
            aria-selected={option === selected}
            ref={option === selected ? chosen : null}
            className={cn(
              "hover:bg-accent focus-visible:ring-ring h-8 w-12 rounded-md text-sm tabular-nums outline-none focus-visible:ring-2",
              option === selected &&
                "bg-primary text-primary-foreground hover:bg-primary/90",
            )}
            onClick={() => onPick(option)}
          >
            {String(option).padStart(2, "0")}
          </button>
        ))}
      </div>
    </div>
  );
}

function minuteOptions(step: number, current: number | null): number[] {
  const steps = Array.from({ length: Math.ceil(60 / step) }, (_, i) => i * step);
  if (current !== null && !steps.includes(current)) steps.push(current);
  return steps.sort((a, b) => a - b);
}

function split(value: string, mode: DateTimeMode): { date: string; time: string } {
  if (mode === "date") return { date: value, time: "" };
  if (mode === "time") return { date: "", time: value };
  const [date = "", time = ""] = value.split("T");
  return { date, time };
}

function join(date: string, time: string, mode: DateTimeMode): string {
  if (mode === "date") return date;
  if (mode === "time") return time;
  return `${date}T${time}`;
}

/** A calendar day as the `yyyy-MM-dd` half of the field, with no timezone in between. */
function toDayKey(day: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${day.getFullYear()}-${pad(day.getMonth() + 1)}-${pad(day.getDate())}`;
}

function fromDayKey(key: string): Date {
  const [y = 0, m = 1, d = 1] = key.split("-").map(Number);
  return new Date(y, m - 1, d);
}

function formatDay(day: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(day.getDate())}/${pad(day.getMonth() + 1)}/${day.getFullYear()}`;
}
