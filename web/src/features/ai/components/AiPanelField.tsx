import { useId } from "react";
import { Info } from "lucide-react";
import { ChipsField } from "@/components/shared/form/fields/ChipsField";
import { SelectField } from "@/components/shared/form/fields/SelectField";
import { Segmented } from "@/components/ui/segmented";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import type { AiField } from "../kind";

const LABEL = "text-sm font-medium";

/**
 * AiPanelField draws one setup field of an AI panel as the deck does: a source
 * (its ready passages as pills above a textarea), toggle chips, a segmented
 * control, a select, a switch on a bordered card, or an info note.
 */
export function AiPanelField({ field }: Readonly<{ field: AiField }>) {
  const id = useId();
  switch (field.kind) {
    case "source":
      return (
        <div className="flex flex-col gap-2">
          <label htmlFor={id} className={LABEL}>
            {field.label}
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
                      "text-meta inline-flex h-7 max-w-full items-center gap-1.5 rounded-full border px-2.5 font-medium",
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
            className="bg-bg border-border focus-visible:border-ring text-ui in-data-[scale=deck]:lg:text-ui rounded-[0.5rem] px-3 py-2.5 leading-[1.6]"
          />
        </div>
      );
    case "chips":
      return (
        <div className="flex flex-col gap-2">
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
      return (
        <div className="flex flex-col gap-2">
          <span aria-hidden="true" className={LABEL}>
            {field.label}
          </span>
          <Segmented
            label={field.label}
            value={field.value}
            options={field.options}
            onChange={field.onChange}
            scroll
            className="self-start [&>button]:px-3.5"
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
