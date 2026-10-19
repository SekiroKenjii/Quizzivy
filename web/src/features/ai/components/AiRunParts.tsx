import { useTranslation } from "react-i18next";
import { Check, CircleAlert, Info, LoaderCircle } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AiPart } from "../kind";

/**
 * AiRunParts is the running stage of a run of several calls: how many parts
 * have settled, failed ones included, as a sentence and a bar, then one row
 * per part, writing, done with its count, or failed, and a line saying the
 * panel may be closed.
 */
export function AiRunParts({ parts }: Readonly<{ parts: readonly AiPart[] }>) {
  const { t } = useTranslation();
  const settled = parts.filter((part) => part.state !== "running").length;
  const percent = parts.length === 0 ? 0 : Math.round((settled / parts.length) * 100);
  return (
    <div role="status" aria-live="polite" className="flex flex-col gap-3.5">
      <div className="flex flex-col gap-2">
        <strong className="text-ui font-semibold">
          {t("ai.generate.progress", { done: settled, total: parts.length })}
        </strong>
        <div className="bg-muted h-1 overflow-hidden rounded-full">
          <div
            className="bg-accent-c h-full transition-[width] duration-300 motion-reduce:transition-none"
            style={{ width: `${percent}%` }}
          />
        </div>
      </div>
      <ul className="rounded-[0.625rem] border">
        {parts.map((part, index) => (
          <li
            key={part.key}
            className={cn(
              "flex items-center gap-2.5 px-3.5 py-2.5",
              index > 0 && "border-t",
            )}
          >
            <span
              aria-hidden="true"
              className="grid size-5 flex-none place-items-center"
            >
              {part.state === "running" && (
                <LoaderCircle className="size-3.5 animate-spin motion-reduce:animate-none" />
              )}
              {part.state === "done" && <Check className="text-success size-3.75" />}
              {part.state === "failed" && (
                <CircleAlert className="text-warning-ink size-3.75" />
              )}
            </span>
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="text-ui truncate font-medium">{part.title}</span>
              <span className="text-muted-fg text-meta">{part.meta}</span>
            </span>
            <span
              className={cn(
                "text-meta flex-none font-medium whitespace-nowrap",
                part.state === "done" && "text-success-ink",
                part.state === "failed" && "text-warning-ink",
                part.state === "running" && "text-muted-fg",
              )}
            >
              {part.state === "running" && t("ai.generate.sectionWriting")}
              {part.state === "done" &&
                t("ai.generate.questionCount", { count: part.count })}
              {part.state === "failed" && t("ai.generate.sectionFailedShort")}
            </span>
          </li>
        ))}
      </ul>
      <p className="text-muted-fg text-meta flex items-start gap-1.5 leading-normal">
        <Info aria-hidden="true" className="mt-0.5 size-3.5 flex-none" />
        <span className="text-pretty">{t("ai.generate.progressHint")}</span>
      </p>
    </div>
  );
}
