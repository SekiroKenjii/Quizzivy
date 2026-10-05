import { cn } from "@/lib/utils";
import { barHeights, labelShown } from "./bars";

/** BarChartDatum describes a keyed column with its label, value and tooltip. */
export type BarChartDatum = Readonly<{
  key: string;
  label: string;
  value: number;
  title: string;
}>;

/** BarChartProps supplies accessible table text, ordered columns and label cadence. */
export type BarChartProps = Readonly<{
  caption: string;
  columns: readonly [string, string];
  data: readonly BarChartDatum[];
  labelEvery?: number | undefined;
  className?: string | undefined;
}>;

/** BarChart pairs scaled columns with an accessible table and disables height motion under reduced motion. */
export function BarChart({
  caption,
  columns,
  data,
  labelEvery = 1,
  className,
}: BarChartProps) {
  const heights = barHeights(data.map((datum) => datum.value));
  const last = data.length - 1;
  return (
    <div className={cn("relative", className)}>
      <div aria-hidden="true">
        <div className="flex h-45 items-end gap-1.5 border-b pb-1">
          {data.map((datum, index) => (
            <div
              key={datum.key}
              title={datum.title}
              className="flex h-full min-w-0 flex-1 flex-col justify-end gap-0.5"
            >
              <div
                className={cn(
                  "rounded-t-[0.25rem] rounded-b-[0.125rem] transition-[height] duration-200 ease-[ease] motion-reduce:transition-none",
                  index === last ? "bg-brand" : "bg-hover",
                )}
                style={{ height: `${heights[index] ?? 0}%` }}
              />
            </div>
          ))}
        </div>
        <div className="mt-1.5 flex gap-1.5">
          {data.map((datum, index) => (
            <span
              key={datum.key}
              className="text-muted-fg text-2xs min-w-0 flex-1 overflow-hidden text-center leading-normal whitespace-nowrap"
            >
              {labelShown(index, labelEvery) ? datum.label : null}
            </span>
          ))}
        </div>
      </div>
      <div className="sr-only">
        <table>
          <caption>{caption}</caption>
          <thead>
            <tr>
              <th scope="col">{columns[0]}</th>
              <th scope="col">{columns[1]}</th>
            </tr>
          </thead>
          <tbody>
            {data.map((datum) => (
              <tr key={datum.key}>
                <th scope="row">{datum.label}</th>
                <td>{datum.value}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
