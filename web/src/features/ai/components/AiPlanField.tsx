import { Info, Layers, TriangleAlert } from "lucide-react";
import { ChipsField } from "@/components/shared/form/fields/ChipsField";
import { Segmented } from "@/components/ui/segmented";
import { cn } from "@/lib/utils";
import type { AiField, AiPlanRow } from "../kind";
import { AiStepper } from "./AiStepper";

type PlanField = Extract<AiField, { kind: "plan" }>;

const SUB_LABEL = "text-meta font-medium";
const SEG =
  "self-start max-w-full max-[767px]:w-full max-[767px]:[&>button]:h-10 max-[767px]:[&>button]:flex-1";

function PlanRow({
  row,
  field,
  first,
}: Readonly<{ row: AiPlanRow; field: PlanField; first: boolean }>) {
  const open = row.blocked === undefined && Number(row.count) > 0;
  const name = `${field.head.count} · ${row.title}`;
  return (
    <div
      role="listitem"
      aria-label={row.title}
      className={cn("flex flex-col gap-2.5 px-3.5 py-3", !first && "border-t")}
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <div className="flex min-w-0 flex-[1_1_200px] flex-col">
          <span
            className={cn(
              "text-ui line-clamp-2 leading-[1.4] font-medium",
              row.italic && "italic",
              row.blocked !== undefined && "text-muted-fg",
            )}
          >
            {row.title}
          </span>
          <span className="text-muted-fg text-meta leading-[1.45]">{row.meta}</span>
        </div>
        <div className="flex flex-none items-center gap-2 max-[767px]:w-full">
          <AiStepper
            label={name}
            fewer={field.fewer}
            more={field.more}
            value={row.count}
            min={0}
            max={field.max}
            blank="0"
            disabled={row.blocked !== undefined}
            onChange={row.onCount}
          />
          {row.unit !== undefined && row.blocked === undefined && (
            <span className="text-muted-fg text-meta whitespace-nowrap">
              {row.unit}
            </span>
          )}
        </div>
      </div>
      {row.blocked !== undefined && (
        <p className="text-muted-fg text-meta flex items-start gap-1.5 leading-normal">
          <Info aria-hidden="true" className="mt-0.5 size-3.5 flex-none" />
          <span className="min-w-0 text-pretty">{row.blocked}</span>
        </p>
      )}
      {open && (
        <div className="flex flex-col gap-2.5">
          {row.mode && (
            <div className="flex flex-col gap-1.5">
              <span aria-hidden="true" className={SUB_LABEL}>
                {row.mode.label}
              </span>
              <Segmented
                label={row.mode.label}
                value={row.mode.value}
                options={row.mode.options}
                onChange={row.mode.onChange}
                className={SEG}
              />
              <span className="text-muted-fg text-meta flex items-start gap-1.5 leading-normal">
                <Layers aria-hidden="true" className="mt-0.5 size-3.5 flex-none" />
                <span className="text-pretty">{row.mode.calc}</span>
              </span>
            </div>
          )}
          <div className="flex flex-col gap-1.5 max-[767px]:[&_button]:h-10">
            <span aria-hidden="true" className={SUB_LABEL}>
              {row.types.label}
              <span className="text-muted-fg font-normal">{row.types.hint}</span>
            </span>
            <ChipsField
              id={`${row.key}-types`}
              label={row.types.label}
              value={row.types.value}
              options={row.types.options}
              onChange={row.types.onChange}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <span aria-hidden="true" className={SUB_LABEL}>
              {row.level.label}
              <span className="text-muted-fg font-normal">{row.level.hint}</span>
            </span>
            <Segmented
              label={row.level.label}
              value={row.level.value}
              options={row.level.options}
              onChange={row.level.onChange}
              className={SEG}
            />
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * AiPlanField is the whole-test plan: one row per section in builder order,
 * each with its count from 0 and, once it has one, its style, types and
 * level; the total under it, and a warning naming each limit the plan is
 * over. From 768 a header names the two columns.
 */
export function AiPlanField({ field }: Readonly<{ field: PlanField }>) {
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-col gap-0.5">
        <span className="text-sm font-medium">{field.label}</span>
        <span className="text-muted-fg text-meta leading-normal text-pretty">
          {field.sub}
        </span>
      </div>
      <div
        role="list"
        aria-label={field.label}
        className="bg-card rounded-[0.625rem] border"
      >
        <div
          aria-hidden="true"
          className="bg-sidebar text-muted-fg flex justify-between gap-3 rounded-t-[0.625rem] border-b px-3.5 py-2 text-xs font-medium max-[767px]:hidden"
        >
          <span>{field.head.section}</span>
          <span>{field.head.count}</span>
        </div>
        {field.rows.map((row, index) => (
          <PlanRow key={row.key} row={row} field={field} first={index === 0} />
        ))}
      </div>
      <strong className="text-ui px-0.5 font-semibold tabular-nums">
        {field.total}
      </strong>
      {field.over.length > 0 && (
        <div
          role="alert"
          className="bg-warning-soft flex items-start gap-2 rounded-[0.5rem] px-3 py-2.5 text-sm leading-normal"
        >
          <TriangleAlert
            aria-hidden="true"
            className="text-warning-ink mt-0.5 size-3.75 flex-none"
          />
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            {field.over.map((text) => (
              <span key={text} className="text-pretty">
                {text}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
