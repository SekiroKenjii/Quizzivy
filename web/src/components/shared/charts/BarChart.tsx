import { cn } from "@/lib/utils";
import { barHeights, labelShown, stackHeights } from "./bars";

/** BarChartDatum describes a keyed column with its label, value and tooltip. */
export type BarChartDatum = Readonly<{
  key: string;
  label: string;
  value: number;
  title: string;
  parts?: readonly number[] | undefined;
}>;

const SERIES_FILLS = { accent: "bg-brand", info: "bg-info" } as const;

/** BarChartSeries names one stacked part: its legend and table label and its tone. */
export type BarChartSeries = Readonly<{
  key: string;
  label: string;
  tone: keyof typeof SERIES_FILLS;
}>;

/** BarChartProps supplies accessible table text, ordered columns and label cadence. */
export type BarChartProps = Readonly<{
  caption: string;
  columns: readonly [string, string];
  data: readonly BarChartDatum[];
  series?: readonly BarChartSeries[] | undefined;
  plotClassName?: string | undefined;
  labelEvery?: number | undefined;
  className?: string | undefined;
}>;

function StackedParts({
  series,
  heights,
}: Readonly<{ series: readonly BarChartSeries[]; heights: readonly number[] }>) {
  const drawn = series.flatMap((part, position) => {
    const height = heights[position] ?? 0;
    return height > 0 ? [{ part, height }] : [];
  });
  return (
    <>
      {drawn
        .map(({ part, height }, position) => (
          <div
            key={part.key}
            className={cn(
              "min-h-0.5 transition-[height] duration-200 ease-[ease] motion-reduce:transition-none",
              SERIES_FILLS[part.tone],
              position === drawn.length - 1
                ? "rounded-t-[0.25rem] rounded-b-[0.125rem]"
                : "rounded-[0.125rem]",
            )}
            style={{ height: `${height}%` }}
          />
        ))
        .reverse()}
    </>
  );
}

function Legend({ series }: Readonly<{ series: readonly BarChartSeries[] }>) {
  return (
    <ul className="text-muted-fg text-meta mt-2.5 flex flex-wrap gap-x-4 gap-y-1">
      {series.map((part) => (
        <li key={part.key} className="flex items-center gap-1.5">
          <span
            aria-hidden="true"
            className={cn(
              "size-2.5 flex-none rounded-[0.1875rem]",
              SERIES_FILLS[part.tone],
            )}
          />
          {part.label}
        </li>
      ))}
    </ul>
  );
}

/**
 * BarChart pairs scaled columns with an accessible table and disables height
 * motion under reduced motion. With `series`, each column stacks its `parts`
 * in the series' order from the baseline, 2px apart, under a legend, and the
 * table gains a column per series before the total's.
 */
export function BarChart({
  caption,
  columns,
  data,
  series,
  plotClassName,
  labelEvery = 1,
  className,
}: BarChartProps) {
  const heights = barHeights(data.map((datum) => datum.value));
  const stacks = stackHeights(data.map((datum) => datum.parts ?? []));
  const last = data.length - 1;
  return (
    <div className={cn("relative", className)}>
      <div aria-hidden="true">
        <div className={cn("flex h-45 items-end gap-1.5 border-b pb-1", plotClassName)}>
          {data.map((datum, index) => (
            <div
              key={datum.key}
              title={datum.title}
              className="flex h-full min-w-0 flex-1 flex-col justify-end gap-0.5"
            >
              {series === undefined ? (
                <div
                  className={cn(
                    "rounded-t-[0.25rem] rounded-b-[0.125rem] transition-[height] duration-200 ease-[ease] motion-reduce:transition-none",
                    index === last ? "bg-brand" : "bg-hover",
                  )}
                  style={{ height: `${heights[index] ?? 0}%` }}
                />
              ) : (
                <StackedParts series={series} heights={stacks[index] ?? []} />
              )}
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
      {series === undefined ? null : <Legend series={series} />}
      <div className="sr-only">
        <table>
          <caption>{caption}</caption>
          <thead>
            <tr>
              <th scope="col">{columns[0]}</th>
              {series?.map((part) => (
                <th key={part.key} scope="col">
                  {part.label}
                </th>
              ))}
              <th scope="col">{columns[1]}</th>
            </tr>
          </thead>
          <tbody>
            {data.map((datum) => (
              <tr key={datum.key}>
                <th scope="row">{datum.label}</th>
                {series?.map((part, position) => (
                  <td key={part.key}>{datum.parts?.[position] ?? 0}</td>
                ))}
                <td>{datum.value}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
