import { useTranslation } from "react-i18next";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { cn } from "@/lib/utils";
import type { AuditEntry } from "../api";

const WHEN_FROM = 700;
const FIELD_FROM = 560;

function shown(value: unknown, empty: string): string {
  if (value === null || value === undefined || value === "") return empty;
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return JSON.stringify(value);
}

const MONO = "font-mono break-words [overflow-wrap:anywhere]";

/**
 * AuditDiff is the panel under an open entry: each field it changed with the
 * value it had struck through and the value it has, whatever else it
 * recorded, and the device it came from. It lines up under the Who column
 * while the When column is drawn, and its field column is dropped to a row
 * of its own below 560px of content. A join code arrives hinted, and a
 * secret never arrives, so values are drawn as returned.
 */
export function AuditDiff({ entry }: Readonly<{ entry: AuditEntry }>) {
  const { t } = useTranslation();
  const whenShown = useContentWidthAtLeast(WHEN_FROM);
  const fieldShown = useContentWidthAtLeast(FIELD_FROM);
  const empty = t("audit.diff.empty");
  const details = Object.entries(entry.details ?? {});
  const grid = fieldShown
    ? "grid-cols-[8.75rem_minmax(0,1fr)_minmax(0,1fr)]"
    : "grid-cols-[minmax(0,1fr)_minmax(0,1fr)]";
  return (
    <div
      className={cn(
        "flex flex-col gap-1.5",
        whenShown ? "pt-1 pr-4 pb-3.5 pl-39.5" : "pt-1.5 pr-4 pb-3.5 pl-4",
      )}
    >
      {entry.changes.length === 0 && details.length === 0 ? (
        <p className="text-muted-fg text-meta">{t("audit.diff.none")}</p>
      ) : null}
      {entry.changes.length === 0 ? null : (
        <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
          {entry.changes.map((change) => (
            <li
              key={change.field}
              className={cn("text-meta grid items-baseline gap-x-3 gap-y-1", grid)}
            >
              <span className="text-muted-fg min-w-0 break-words">{change.field}</span>
              <del className={cn(MONO, "text-danger-ink")}>
                {shown(change.from, empty)}
              </del>
              <ins className={cn(MONO, "text-success-ink no-underline")}>
                {shown(change.to, empty)}
              </ins>
            </li>
          ))}
        </ul>
      )}
      {details.length === 0 ? null : (
        <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
          {details.map(([key, value]) => (
            <li
              key={key}
              className={cn(
                "text-meta grid items-baseline gap-x-3 gap-y-1",
                fieldShown
                  ? "grid-cols-[8.75rem_minmax(0,1fr)]"
                  : "grid-cols-[minmax(0,1fr)]",
              )}
            >
              <span className="text-muted-fg min-w-0 break-words">{key}</span>
              <span className={MONO}>{shown(value, empty)}</span>
            </li>
          ))}
        </ul>
      )}
      {entry.device === null ? null : (
        <span className="text-muted-fg text-xs">{entry.device}</span>
      )}
    </div>
  );
}
