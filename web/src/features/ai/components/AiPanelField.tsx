import { useId, type KeyboardEvent } from "react";
import { Select as SelectPrimitive } from "radix-ui";
import { ChevronDown, Info, List, TriangleAlert } from "lucide-react";
import { ChipsField } from "@/components/shared/form/fields/ChipsField";
import { SelectField } from "@/components/shared/form/fields/SelectField";
import { SelectContent, SelectItem } from "@/components/ui/select";
import { Segmented } from "@/components/ui/segmented";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import type { AiField } from "../kind";
import { AiPlanField } from "./AiPlanField";
import { AiStepper } from "./AiStepper";

const LABEL = "text-sm font-medium";
const TOUCH_SEG =
  "max-[767px]:w-full max-[767px]:self-stretch max-[767px]:[&>button]:h-10 max-[767px]:[&>button]:flex-1";
const TOUCH_CHIPS = "max-[767px]:[&_button]:h-10";

type Radios = Extract<AiField, { kind: "radios" }>;
type Destination = Extract<AiField, { kind: "destination" }>;

function RadioCards({ field }: Readonly<{ field: Radios }>) {
  const id = useId();
  function move(event: KeyboardEvent<HTMLButtonElement>) {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key];
    if (step === undefined) return;
    event.preventDefault();
    const index = field.options.findIndex((option) => option.value === field.value);
    const next =
      field.options[(index + step + field.options.length) % field.options.length];
    if (!next) return;
    field.onChange(next.value);
    const group = event.currentTarget.parentElement;
    window.requestAnimationFrame(() =>
      group?.querySelector<HTMLButtonElement>('[aria-checked="true"]')?.focus(),
    );
  }
  return (
    <div className="flex flex-col gap-2">
      <span id={id} className={LABEL}>
        {field.label}
      </span>
      <div role="radiogroup" aria-labelledby={id} className="flex flex-wrap gap-2">
        {field.options.map((option) => {
          const on = option.value === field.value;
          return (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={on}
              tabIndex={on ? 0 : -1}
              onClick={() => field.onChange(option.value)}
              onKeyDown={move}
              className={cn(
                "bg-card flex min-w-0 flex-[1_1_200px] items-start gap-2.5 rounded-[0.625rem] border px-3.5 py-3 text-left",
                on ? "border-primary inset-ring-primary inset-ring" : "border-border",
              )}
            >
              <span
                aria-hidden="true"
                className={cn(
                  "bg-card mt-0.5 size-4 flex-none rounded-full",
                  on ? "border-primary border-5" : "border-ring border",
                )}
              />
              <span className="flex min-w-0 flex-col gap-0.5">
                <span className="text-ui leading-[1.45] font-medium">
                  {option.label}
                </span>
                {option.sub !== undefined && (
                  <span className="text-muted-fg text-meta leading-[1.45] text-pretty">
                    {option.sub}
                  </span>
                )}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function DestinationCard({ field }: Readonly<{ field: Destination }>) {
  const id = useId();
  const chosen = field.options.find((option) => option.value === field.value);
  return (
    <div className="flex flex-col gap-2">
      <span className={LABEL}>{field.label}</span>
      <div className="bg-card rounded-[0.625rem] border">
        <SelectPrimitive.Root value={field.value} onValueChange={field.onChange}>
          <SelectPrimitive.Trigger
            aria-label={field.changeLabel}
            className="hover:bg-hover flex min-h-13 w-full items-center gap-2.5 rounded-t-[0.625rem] py-2 pr-2.5 pl-3.5 text-left max-[767px]:min-h-15"
          >
            <List aria-hidden="true" className="text-muted-fg size-4 flex-none" />
            <span className="flex min-w-0 flex-1 flex-col">
              <span
                className={cn(
                  "text-ui truncate leading-[1.4] font-medium",
                  chosen?.italic && "italic",
                )}
              >
                {chosen?.label}
              </span>
              {chosen?.sub !== undefined && (
                <span className="text-muted-fg text-meta leading-[1.45]">
                  {chosen.sub}
                </span>
              )}
            </span>
            <span
              aria-hidden="true"
              className="bg-card text-meta inline-flex h-7 flex-none items-center gap-1 rounded-[0.4375rem] border px-2 font-medium max-[767px]:h-10"
            >
              {field.change}
              <ChevronDown className="size-3.5" />
            </span>
          </SelectPrimitive.Trigger>
          <SelectContent position="popper" align="start">
            {field.options.map((option) => (
              <SelectItem
                key={option.value}
                value={option.value}
                className="max-[767px]:min-h-13"
              >
                <span
                  className={cn(
                    "block truncate font-medium",
                    option.italic && "italic",
                  )}
                >
                  {option.label}
                </span>
                {option.sub !== undefined && (
                  <span className="text-muted-fg block truncate text-xs">
                    {option.sub}
                  </span>
                )}
                {option.badge !== undefined && (
                  <span className="bg-muted text-muted-fg ml-1 rounded-full px-1.75 text-[0.71875rem] font-medium">
                    {option.badge}
                  </span>
                )}
              </SelectItem>
            ))}
          </SelectContent>
        </SelectPrimitive.Root>
        <div className="flex items-center gap-3 border-t px-3.5 py-3">
          <label htmlFor={id} className="min-w-0 flex-1">
            <span className="text-ui block leading-normal font-medium">
              {field.toggle.text}
            </span>
            <span className="text-muted-fg text-meta block leading-normal text-pretty">
              {field.toggle.sub}
            </span>
          </label>
          <Switch
            id={id}
            checked={field.toggle.value}
            onCheckedChange={field.toggle.onChange}
          />
        </div>
      </div>
    </div>
  );
}

/**
 * AiPanelField draws one field of an AI panel as the deck does: a source
 * (its ready passages as pills above a textarea), toggle chips, a segmented
 * control, a select, a switch on a bordered card, radio cards, the section
 * card new questions go to with its switch, a count stepper, a whole-test
 * plan, an alert with an optional choice, or an info note. Below 768 its
 * controls are 40 to 44px high.
 */
export function AiPanelField({ field }: Readonly<{ field: AiField }>) {
  const id = useId();
  switch (field.kind) {
    case "source":
      return (
        <div className="flex flex-col gap-2">
          <label htmlFor={id} className={LABEL}>
            {field.label}
            {field.optional !== undefined && (
              <span className="text-muted-fg font-normal"> · {field.optional}</span>
            )}
          </label>
          {field.sources.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {field.sources.map((source) => {
                const Icon = source.icon;
                return (
                  <button
                    key={source.key}
                    type="button"
                    aria-pressed={source.active}
                    onClick={source.pick}
                    className={cn(
                      "text-meta inline-flex h-7 max-w-full items-center gap-1.5 rounded-full border px-2.5 font-medium max-[767px]:h-10",
                      source.active
                        ? "border-primary bg-primary text-primary-fg"
                        : "border-border bg-card text-fg",
                    )}
                  >
                    <Icon aria-hidden="true" className="size-3.25 flex-none" />
                    <span className="truncate">{source.label}</span>
                  </button>
                );
              })}
            </div>
          )}
          <Textarea
            id={id}
            value={field.value}
            rows={field.rows}
            placeholder={field.placeholder}
            onChange={(event) => field.onChange(event.target.value)}
            className="bg-bg border-border focus-visible:border-ring text-ui lg:text-ui rounded-[0.5rem] px-3 py-2.5 leading-[1.6] max-[767px]:text-base"
          />
        </div>
      );
    case "chips":
      return (
        <div className={cn("flex flex-col gap-2", TOUCH_CHIPS)}>
          <span aria-hidden="true" className={LABEL}>
            {field.label}
          </span>
          <ChipsField
            id={id}
            label={field.label}
            value={field.value}
            options={field.options}
            onChange={field.onChange}
          />
        </div>
      );
    case "seg":
      if (field.hideLabel)
        return (
          <Segmented
            label={field.label}
            value={field.value}
            options={field.options}
            onChange={field.onChange}
            className={cn("w-full [&>button]:flex-1", TOUCH_SEG)}
          />
        );
      return (
        <div className="flex flex-col gap-2">
          <span aria-hidden="true" className={LABEL}>
            {field.label}
          </span>
          {field.sub !== undefined && (
            <span className="text-muted-fg text-meta -mt-1 leading-normal">
              {field.sub}
            </span>
          )}
          <Segmented
            label={field.label}
            value={field.value}
            options={field.options}
            onChange={field.onChange}
            scroll
            className={cn("self-start [&>button]:px-3.5", TOUCH_SEG)}
          />
        </div>
      );
    case "select":
      return (
        <div className="flex flex-col gap-2">
          <label htmlFor={id} className={LABEL}>
            {field.label}
          </label>
          <SelectField
            id={id}
            label={field.label}
            value={field.value}
            options={field.options}
            onChange={field.onChange}
          />
        </div>
      );
    case "toggle":
      return (
        <div className="flex items-center gap-3 rounded-[0.625rem] border px-3.5 py-3">
          <label htmlFor={id} className="min-w-0 flex-1">
            <span className="text-ui block leading-normal font-medium">
              {field.text}
            </span>
            {field.sub !== undefined && (
              <span className="text-muted-fg text-meta block leading-normal">
                {field.sub}
              </span>
            )}
          </label>
          <Switch id={id} checked={field.value} onCheckedChange={field.onChange} />
        </div>
      );
    case "radios":
      return <RadioCards field={field} />;
    case "destination":
      return <DestinationCard field={field} />;
    case "stepper":
      return (
        <div className="flex flex-col gap-2">
          <label htmlFor={id} className={LABEL}>
            {field.label}
          </label>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
            <AiStepper
              id={id}
              label={field.label}
              fewer={field.fewer}
              more={field.more}
              value={field.value}
              min={field.min}
              max={field.max}
              describedBy={`${id}-hint`}
              onChange={field.onChange}
            />
            <span id={`${id}-hint`} className="text-muted-fg text-meta">
              {field.hint}
            </span>
          </div>
        </div>
      );
    case "plan":
      return <AiPlanField field={field} />;
    case "alert":
      return (
        <div
          role="alert"
          className="bg-danger-soft flex flex-col gap-2.5 rounded-[0.5rem] px-3 py-2.5 text-sm leading-normal"
        >
          <div className="flex items-start gap-2">
            <TriangleAlert
              aria-hidden="true"
              className="text-danger-ink mt-0.5 size-3.75 flex-none"
            />
            <span className="min-w-0 flex-1 text-pretty">{field.text}</span>
          </div>
          {field.choose && (
            <div className="pl-5.75">
              <SelectField
                id={id}
                label={field.choose.label}
                placeholder={field.choose.label}
                value={field.choose.value}
                options={field.choose.options}
                onChange={field.choose.onChange}
              />
            </div>
          )}
        </div>
      );
    case "info":
      return (
        <div className="bg-muted rounded-ctl flex items-start gap-2.5 px-3 py-2.5 text-sm leading-[1.55] text-pretty">
          <Info
            aria-hidden="true"
            className="text-muted-fg mt-0.5 size-3.75 flex-none"
          />
          <span>{field.text}</span>
        </div>
      );
  }
}
