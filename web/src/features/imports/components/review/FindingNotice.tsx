import { memo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  CircleAlert,
  CircleCheck,
  Info,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { ImportFinding, ImportSourceRef } from "../../api";
import { findingKey, keyPapers, objectReason } from "../../findings";

type Tone = "blocking" | "review" | "acknowledged" | "info";

const ICON: Record<Tone, LucideIcon> = {
  blocking: CircleAlert,
  review: TriangleAlert,
  acknowledged: CircleCheck,
  info: Info,
};

function toneOf(finding: ImportFinding): Tone {
  if (finding.severity === "blocking") return "blocking";
  if (finding.severity === "informational") return "info";
  return finding.acknowledged === true ? "acknowledged" : "review";
}

/**
 * FindingNotice explains one finding and offers only the actions its severity
 * allows: a blocker has no dismissal, a review item can be acknowledged and
 * un-acknowledged, and an informational note only points at its evidence.
 * While `readOnly`, only locating the evidence remains.
 */
export const FindingNotice = memo(function FindingNotice({
  finding,
  current,
  readOnly = false,
  onAcknowledge,
  onLocate,
  onReprocess,
  children,
}: Readonly<{
  finding: ImportFinding;
  current: boolean;
  readOnly?: boolean;
  onAcknowledge: (findingId: string, on: boolean) => void;
  onLocate: (refs: readonly ImportSourceRef[]) => void;
  onReprocess?: ((paper: number) => void) | undefined;
  children?: ReactNode;
}>) {
  const { t } = useTranslation();
  const tone = toneOf(finding);
  const Icon = ICON[tone];
  const key = findingKey(finding.code);
  const reason = objectReason(finding);
  const decidable = tone === "review" || tone === "acknowledged";
  return (
    <div
      id={`finding-${finding.id}`}
      tabIndex={-1}
      data-current={current || undefined}
      className={cn(
        "focus-visible:ring-ring rounded-md border px-3 py-2.5 text-sm outline-none focus-visible:ring-2",
        current && "ring-ring/40 ring-2",
      )}
    >
      <div className="flex items-start gap-2">
        <Icon
          className={cn(
            "mt-0.5 size-4 shrink-0",
            tone === "blocking" && "text-destructive",
            tone === "review" && "text-warning",
            tone !== "blocking" && tone !== "review" && "text-muted-foreground",
          )}
          aria-hidden="true"
        />
        <div className="min-w-0 flex-1 space-y-1">
          <p>
            <span className="text-muted-foreground mr-1.5 text-xs font-medium">
              {t(`imports.severity.${tone}`)}
            </span>
            <span className="font-medium">
              {t(`imports.findings.codes.${key}.title`, {
                count: finding.count,
                code: finding.code,
              })}
            </span>
          </p>
          <p className="text-muted-foreground text-xs leading-relaxed">
            {reason === null
              ? t(`imports.findings.codes.${key}.help`, { count: finding.count })
              : t(`imports.findings.objectReasons.${reason}`)}
          </p>
          {children}
          {finding.code === "AMBIGUOUS_KEY_PAPER" && onReprocess ? (
            <PaperChoice
              papers={keyPapers(finding)}
              disabled={readOnly}
              onReprocess={onReprocess}
            />
          ) : null}
          <div className="flex flex-wrap items-center gap-2 pt-1">
            {finding.evidence.length > 0 ? (
              <Button
                type="button"
                variant="ghost"
                size="xs"
                onClick={() => onLocate(finding.evidence)}
              >
                {t("imports.review.showEvidence")}
              </Button>
            ) : null}
            {decidable && !readOnly ? (
              <Button
                type="button"
                variant={tone === "review" ? "outline" : "ghost"}
                size="xs"
                onClick={() => onAcknowledge(finding.id, tone === "review")}
              >
                {tone === "review"
                  ? t("imports.review.acknowledge")
                  : t("imports.review.unacknowledge")}
              </Button>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
});

function PaperChoice({
  papers,
  disabled,
  onReprocess,
}: Readonly<{
  papers: number[];
  disabled: boolean;
  onReprocess: (paper: number) => void;
}>) {
  const { t } = useTranslation();
  const [paper, setPaper] = useState<number | null>(null);
  if (papers.length === 0) return null;
  return (
    <fieldset disabled={disabled} className="space-y-2 pt-1">
      <legend className="text-xs font-medium">{t("imports.review.choosePaper")}</legend>
      <div className="flex flex-wrap gap-2">
        {papers.map((value) => (
          <Button
            key={value}
            type="button"
            size="xs"
            variant={paper === value ? "default" : "outline"}
            aria-pressed={paper === value}
            onClick={() => setPaper(value)}
          >
            {t("imports.review.paperNumber", { n: value })}
          </Button>
        ))}
      </div>
      <Button
        type="button"
        size="xs"
        disabled={paper === null}
        onClick={() => paper !== null && onReprocess(paper)}
      >
        {t("imports.review.reprocessWithPaper")}
      </Button>
      <p className="text-muted-foreground text-xs">
        {t("imports.review.reprocessHint")}
      </p>
    </fieldset>
  );
}
