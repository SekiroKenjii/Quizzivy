import {
  useEffect,
  useRef,
  useState,
  useLayoutEffect,
  type AriaAttributes,
} from "react";
import { useTranslation } from "react-i18next";
import { format } from "date-fns";
import { vi, enUS } from "date-fns/locale";
import { CalendarIcon, ClockIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import { useDeckScale } from "@/components/ui/deck-scale";
import { DateTimePicker } from "./DateTimePicker";

const LOCALES = { vi, en: enUS } as const;
const HOURS = Array.from({ length: 24 }, (_, h) => h);
const DEFAULT_TIME = "08:00";

/** DateTimeMode chooses the wall-clock components edited by the field. */
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
  readonly disabled?: boolean | undefined;
  readonly "aria-invalid"?: AriaAttributes["aria-invalid"];
  readonly "aria-describedby"?: string | undefined;
}

/** DateTimeField edits wall-clock values with inherited or deck controls that dismiss when disabled. */
export function DateTimeField({
  id,
  label,
  value,
  onChange,
  mode = "datetime",
  minuteStep = 5,
  className,
  disabled = false,
  "aria-invalid": invalid,
  "aria-describedby": describedBy,
}: DateTimeFieldProps) {
  const deck = useDeckScale();
  const acceptsChanges = useRef(!disabled);
  useLayoutEffect(() => {
    acceptsChanges.current = !disabled;
    return () => {
      acceptsChanges.current = false;
    };
  }, [disabled]);
  if (deck)
    return (
      <DateTimePicker
        id={id}
        label={label}
        value={value}
        onChange={(next) => {
          if (acceptsChanges.current) onChange(next);
        }}
        disabled={disabled}
        mode={mode}
        minuteStep={minuteStep}
        className={className}
        aria-invalid={invalid}
        aria-describedby={describedBy}
      />
    );
  const { date, time } = split(value, mode);
  const emit = (nextDate: string, nextTime: string) => {
    if (acceptsChanges.current) onChange(join(nextDate, nextTime, mode));
  };
  const both = mode === "datetime";

  return (
    <div role="group" aria-label={label} className={cn("flex", className)}>
      {mode !== "time" && (
        <DayPart
          id={id}
          disabled={disabled}
          aria-invalid={invalid}
          aria-describedby={describedBy}
          date={date}
          className={both ? "rounded-r-none" : undefined}
          onChange={(next) => emit(next, time || DEFAULT_TIME)}
        />
      )}
      {mode !== "date" && (
        <ClockPart
          id={mode === "time" ? id : undefined}
          disabled={disabled}
          label={label}
          aria-invalid={invalid}
          aria-describedby={describedBy}
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
  disabled,
  "aria-invalid": invalid,
  "aria-describedby": describedBy,
  id,
  date,
  className,
  onChange,
}: {
  readonly disabled: boolean;
  readonly "aria-invalid": AriaAttributes["aria-invalid"];
  readonly "aria-describedby": string | undefined;
  readonly id: string | undefined;
  readonly date: string;
  readonly className: string | undefined;
  readonly onChange: (next: string) => void;
}) {
  const { t } = useTranslation();
  const locale = useLocale();
  const [open, setOpen] = useState(false);
  if (disabled && open) setOpen(false);
  const selected = date === "" ? undefined : fromDayKey(date);
  return (
    <Popover
      open={open && !disabled}
      onOpenChange={(next) => {
        if (!next || !disabled) setOpen(next);
      }}
    >
      <PopoverTrigger asChild>
        <Button
          id={id}
          disabled={disabled}
          aria-invalid={invalid}
          aria-describedby={describedBy}
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
          disabled={disabled}
          disableNavigation={disabled}
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
            if (disabled || day === undefined) return;
            onChange(toDayKey(day));
            setOpen(false);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}

function ClockPart({
  disabled,
  "aria-invalid": invalid,
  "aria-describedby": describedBy,
  id,
  label,
  time,
  minuteStep,
  className,
  onChange,
}: {
  readonly disabled: boolean;
  readonly "aria-invalid": AriaAttributes["aria-invalid"];
  readonly "aria-describedby": string | undefined;
  readonly id: string | undefined;
  readonly label: string;
  readonly time: string;
  readonly minuteStep: number;
  readonly className: string;
  readonly onChange: (next: string) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  if (disabled && open) setOpen(false);
  const [hour, minute] = time === "" ? [null, null] : time.split(":").map(Number);
  const minutes = minuteOptions(minuteStep, minute ?? null);
  const pad = (n: number) => String(n).padStart(2, "0");

  return (
    <Popover
      open={open && !disabled}
      onOpenChange={(next) => {
        if (!next || !disabled) setOpen(next);
      }}
    >
      <PopoverTrigger asChild>
        <Button
          id={id}
          disabled={disabled}
          aria-invalid={invalid}
          aria-describedby={describedBy}
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
            disabled={disabled}
            title={t("common.calendar.hours")}
            options={HOURS}
            selected={hour ?? null}
            open={open}
            onPick={(h) => onChange(`${pad(h)}:${pad(minute ?? 0)}`)}
          />
          <Column
            disabled={disabled}
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
  disabled,
  title,
  options,
  selected,
  open,
  onPick,
}: {
  readonly disabled: boolean;
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
            disabled={disabled}
            aria-selected={option === selected}
            ref={option === selected ? chosen : null}
            className={cn(
              "hover:bg-accent h-8 w-12 rounded-md text-sm tabular-nums outline-none",
              option === selected &&
                "bg-primary text-primary-foreground hover:bg-primary/90",
            )}
            onClick={() => {
              if (!disabled) onPick(option);
            }}
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
