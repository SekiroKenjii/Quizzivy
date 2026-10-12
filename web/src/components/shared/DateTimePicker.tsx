import { useState, useRef, useLayoutEffect, type AriaAttributes } from "react";
import { useTranslation } from "react-i18next";
import { format } from "date-fns";
import { vi, enUS } from "date-fns/locale";
import { CalendarIcon, ChevronDown, ClockIcon, Minus, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import {
  bumpHour,
  bumpMinute,
  chooseDay,
  dateTimeLabel,
  minuteStepValue,
  parseDateTime,
  pickerStart,
  serializeDateTime,
} from "./dateTimeValue";
import type { DateTimeMode } from "./DateTimeField";
const LOCALES = { vi, en: enUS };
/** DateTimePicker edits deck wall-clock drafts and dismisses without committing when disabled. */
export function DateTimePicker({
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
}: Readonly<{
  id?: string | undefined;
  label: string;
  value: string;
  onChange: (value: string) => void;
  mode?: DateTimeMode;
  minuteStep?: number;
  className?: string | undefined;
  disabled?: boolean | undefined;
  "aria-invalid"?: AriaAttributes["aria-invalid"];
  "aria-describedby"?: string | undefined;
}>) {
  const acceptsChanges = useRef(!disabled);
  useLayoutEffect(() => {
    acceptsChanges.current = !disabled;
    return () => {
      acceptsChanges.current = false;
    };
  }, [disabled]);
  const { t } = useTranslation();
  const locale = useLocale();
  const [open, setOpen] = useState(false);
  if (disabled && open) setOpen(false);
  const [draft, setDraft] = useState(() => pickerStart(value, mode));
  const [month, setMonth] = useState(() => pickerStart(value, mode));
  const selected = parseDateTime(value, mode);
  const Icon = mode === "time" ? ClockIcon : CalendarIcon;
  const emptyText = t(mode === "time" ? "common.pickTime" : "common.pickDate");
  const text = selected ? dateTimeLabel(selected, mode, locale) : emptyText;
  function changeOpen(next: boolean) {
    if (next && !acceptsChanges.current) return;
    if (next) {
      const start = pickerStart(value, mode);
      setDraft(start);
      setMonth(start);
    }
    setOpen(next);
  }
  function commit(date: Date) {
    if (!acceptsChanges.current) return;
    onChange(serializeDateTime(date, mode));
    setOpen(false);
  }
  return (
    <Popover open={open && !disabled} onOpenChange={changeOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          id={id}
          disabled={disabled}
          aria-invalid={invalid}
          aria-describedby={describedBy}
          type="button"
          aria-label={`${label} ${selected ? text : ""}`.trim()}
          aria-haspopup="dialog"
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") {
              event.preventDefault();
              changeOpen(true);
            }
          }}
          className={cn(
            "bg-bg border-border hover:border-ring text-ui flex h-9.5 w-full items-center gap-2 rounded-md border px-2.5 text-left leading-normal font-normal shadow-none in-data-[scale=deck]:h-9.5 in-data-[scale=deck]:gap-2 in-data-[scale=deck]:px-2.5",
            className,
          )}
        >
          <Icon aria-hidden="true" className="text-muted-fg size-3.75 flex-none" />
          <span className={cn("min-w-0 flex-1 truncate", !selected && "text-muted-fg")}>
            {text}
          </span>
          <ChevronDown
            aria-hidden="true"
            className="text-muted-fg size-3.5 flex-none"
          />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        collisionPadding={8}
        aria-label={t(
          mode === "time" ? "datePicker.chooseTime" : "datePicker.chooseDate",
        )}
        className="data-[scale=deck]:w-auto"
      >
        <div className="w-67 p-2">
          {mode !== "time" && (
            <Calendar
              mode="single"
              required
              disabled={disabled}
              disableNavigation={disabled}
              locale={LOCALES[locale]}
              selected={draft}
              month={month}
              onMonthChange={(next) => {
                if (acceptsChanges.current) setMonth(next);
              }}
              weekStartsOn={1}
              fixedWeeks
              showOutsideDays
              labels={{
                labelPrevious: () => t("common.calendar.previousMonth"),
                labelNext: () => t("common.calendar.nextMonth"),
                labelNav: () => t("common.calendar.nav"),
                labelDayButton: (day, modifiers) =>
                  [
                    format(day, "PPPP", { locale: LOCALES[locale] }),
                    modifiers.selected ? t("common.calendar.selected") : null,
                  ]
                    .filter(Boolean)
                    .join(", "),
              }}
              onSelect={(day) => {
                if (!acceptsChanges.current || !day) return;
                const next = chooseDay(draft, day);
                if (mode === "date") commit(next);
                else {
                  setDraft(next);
                  setMonth(next);
                }
              }}
              className="in-data-[scale=deck]:w-full in-data-[scale=deck]:p-0"
            />
          )}
          {mode !== "date" && (
            <>
              <div
                className={cn(
                  "flex items-center justify-between gap-2.5",
                  mode !== "time" && "mt-2 border-t pt-2.5",
                )}
              >
                <span className="text-muted-fg text-meta font-medium">
                  {t("datePicker.time")}
                </span>
                <span className="flex items-center gap-1">
                  <TimeUnit
                    disabled={disabled}
                    value={draft.getHours()}
                    earlier={t("datePicker.earlierHour")}
                    later={t("datePicker.laterHour")}
                    onStep={(delta) => {
                      if (acceptsChanges.current)
                        setDraft((current) => bumpHour(current, delta));
                    }}
                  />
                  <span className="px-0.5 font-semibold">:</span>
                  <TimeUnit
                    disabled={disabled}
                    value={draft.getMinutes()}
                    earlier={t("datePicker.earlierMinute")}
                    later={t("datePicker.laterMinute")}
                    onStep={(delta) => {
                      if (acceptsChanges.current)
                        setDraft((current) => bumpMinute(current, delta, minuteStep));
                    }}
                  />
                </span>
              </div>
              <div className="mt-2.5 flex items-center justify-between gap-2">
                <button
                  type="button"
                  className="text-muted-fg hover:bg-muted rounded-seg text-meta h-7.5 px-3 font-medium"
                  disabled={disabled}
                  onClick={() => {
                    if (!acceptsChanges.current) return;
                    const now = new Date();
                    if (mode === "time") {
                      now.setMinutes(
                        (Math.round(now.getMinutes() / minuteStepValue(minuteStep)) *
                          minuteStepValue(minuteStep)) %
                          60,
                      );
                      setDraft(chooseDay(now, draft));
                    } else {
                      setDraft(chooseDay(draft, now));
                      setMonth(now);
                    }
                  }}
                >
                  {t(mode === "time" ? "datePicker.now" : "datePicker.today")}
                </button>
                <button
                  type="button"
                  className="bg-primary text-primary-fg rounded-seg border-primary text-meta h-7.5 border px-3 font-medium hover:opacity-90"
                  disabled={disabled}
                  onClick={() => commit(draft)}
                >
                  {t("datePicker.done")}
                </button>
              </div>
            </>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
function TimeUnit({
  disabled,
  value,
  earlier,
  later,
  onStep,
}: Readonly<{
  disabled: boolean;
  value: number;
  earlier: string;
  later: string;
  onStep: (delta: number) => void;
}>) {
  return (
    <>
      <button
        type="button"
        disabled={disabled}
        aria-label={earlier}
        className="bg-card hover:bg-muted grid h-7.5 w-6.5 place-items-center rounded-sm border"
        onClick={() => onStep(-1)}
      >
        <Minus aria-hidden="true" className="size-3.5" />
      </button>
      <span className="min-w-7.5 text-center font-semibold tabular-nums">
        {String(value).padStart(2, "0")}
      </span>
      <button
        type="button"
        disabled={disabled}
        aria-label={later}
        className="bg-card hover:bg-muted grid h-7.5 w-6.5 place-items-center rounded-sm border"
        onClick={() => onStep(1)}
      >
        <Plus aria-hidden="true" className="size-3.5" />
      </button>
    </>
  );
}
