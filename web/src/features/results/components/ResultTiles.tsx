import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import type { Locale } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { Tile } from "../resultView";

type Face = Readonly<{ key: string; label: string; value: string; bar: string }>;

function face(tile: Tile, n: Intl.NumberFormat, t: TFunction): Face {
  const score = (earned: number, total: number) =>
    t("result.tiles.score", { earned: n.format(earned), total: n.format(total) });
  switch (tile.kind) {
    case "auto":
      return {
        key: "auto",
        label: t("result.tiles.auto"),
        value: score(tile.earned, tile.total),
        bar: "bg-brand",
      };
    case "waiting":
      return {
        key: "waiting",
        label: t("result.tiles.waiting"),
        value: t("result.tiles.answers", { count: tile.count }),
        bar: "bg-warning",
      };
    case "time":
      return {
        key: "time",
        label: t("result.tiles.time"),
        value: t("result.tiles.minutes", { count: tile.minutes }),
        bar: "bg-info",
      };
    case "part":
      return {
        key: tile.id,
        label: tile.title,
        value: score(tile.earned, tile.total),
        bar: tile.tone === "success" ? "bg-success" : "bg-warning",
      };
  }
}

function fill(tile: Tile): number {
  return tile.kind === "waiting" ? 0 : Math.round(tile.share * 100);
}

/**
 * ResultTiles is the row of boxes under the summary, as the design deck draws
 * it: a label, a value and a 6px bar each. They sit two to a row below 768px
 * and up to four to a row from there. With no tile it renders nothing.
 */
export function ResultTiles({
  tiles,
  wide,
}: Readonly<{ tiles: readonly Tile[]; wide: boolean }>) {
  const { t, i18n } = useTranslation();
  if (tiles.length === 0) return null;
  const n = new Intl.NumberFormat(i18n.language as Locale, {
    maximumFractionDigits: 2,
  });
  const columns = wide ? Math.min(4, tiles.length) : 2;
  return (
    <dl
      data-slot="result-tiles"
      className="grid gap-3"
      style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
    >
      {tiles.map((tile) => {
        const { key, label, value, bar } = face(tile, n, t);
        return (
          <div
            key={key}
            className="bg-card flex flex-col gap-2 rounded-xl border px-4 py-3.5"
          >
            <dt className="text-muted-fg text-sm break-words">{label}</dt>
            <dd className="flex flex-col gap-2">
              <span className="text-stat-sm leading-normal font-semibold tabular-nums">
                {value}
              </span>
              <span
                aria-hidden="true"
                className="bg-muted block h-1.5 overflow-hidden rounded-full"
              >
                <span
                  data-slot="tile-bar"
                  className={cn("block h-full", bar)}
                  style={{ width: `${fill(tile)}%` }}
                />
              </span>
            </dd>
          </div>
        );
      })}
    </dl>
  );
}
