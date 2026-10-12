import { memo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  CircleAlert,
  CircleCheck,
  CircleHelp,
  FileSearch,
  Info,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { ImportFinding, ImportSourceRef } from "../../api";
import { findingKey, keyPapers, objectReason, titledReason } from "../../findings";

type Tone = "blocking" | "review" | "acknowledged" | "info";

const ICON: Record<Tone, LucideIcon> = {
  blocking: CircleAlert,
  review: CircleHelp,
  acknowledged: CircleCheck,
  info: Info,
};

const FRAME: Record<Tone, string> = {
  blocking: "border-transparent bg-danger-soft",
  review: "border-transparent bg-warning-soft",
  acknowledged: "bg-card",
  info: "border-transparent bg-info-soft",
};

const INK: Record<Tone, string> = {
  blocking: "text-danger-ink",
  review: "text-warning-ink",
  acknowledged: "text-success-ink",
  info: "text-info-ink",
};

const RING: Record<Tone, string> = {
  blocking: "ring-danger",
  review: "ring-warning",
  acknowledged: "ring-border",
  info: "ring-info",
};

/** ACTION is the class of a finding's action buttons, the deck's 32px outline button. */
export const FINDING_ACTION =
  "bg-card text-fg h-8 rounded-lg px-3 text-meta font-medium shadow-none in-data-[scale=deck]:h-8 in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-meta";

function toneOf(finding: ImportFinding): Tone {
  if (finding.severity === "blocking") return "blocking";
  if (finding.severity === "informational") return "info";
  return finding.acknowledged === true ? "acknowledged" : "review";
}

function helpText(
  t: TFunction,
  finding: ImportFinding,
  key: string,
  reprocess: boolean,
): string {
  if (finding.code === "AMBIGUOUS_KEY_PAPER" && !reprocess)
    return t("imports.findings.codes.AMBIGUOUS_KEY_PAPER.helpProcessingOff");
  return t(`imports.findings.codes.${key}.help`, { count: finding.count });
}

function titleOf(t: TFunction, finding: ImportFinding): string {
  const titled = titledReason(finding);
  if (titled !== null)
    return t(`imports.findings.objectTitles.${titled}`, { count: finding.count });
  return t(`imports.findings.codes.${findingKey(finding.code)}.title`, {
    count: finding.count,
    code: finding.code,
  });
}

/**
 * FindingNotice explains one finding in the deck's tinted box and offers only
 * the actions its severity allows: a blocker has no dismissal, a review item
 * can be confirmed and changed again, and an informational note only points
 * at its evidence. `children` hold its resolution (the candidate values of a
 * key conflict) and `actions` the host's buttons beside "Show in source".
 * While `readOnly`, only locating the evidence remains.
 */
export const FindingNotice = memo(function FindingNotice({
  finding,
  current,
  readOnly = false,
  onAcknowledge,
  onLocate,
  onReprocess,
  actions,
  children,
}: Readonly<{
  finding: ImportFinding;
  current: boolean;
  readOnly?: boolean;
  onAcknowledge: (findingId: string, on: boolean) => void;
  onLocate: (refs: readonly ImportSourceRef[]) => void;
  onReprocess?: ((paper: number) => void) | undefined;
  actions?: ReactNode;
  children?: ReactNode;
}>) {
  const { t } = useTranslation();
  const tone = toneOf(finding);
  const Icon = ICON[tone];
  const key = findingKey(finding.code);
  const reason = objectReason(finding);
  const severity = t(`imports.severity.${tone}`);
  return (
    <div
      id={`finding-${finding.id}`}
      role="group"
      aria-label={severity}
      tabIndex={-1}
      data-current={current || undefined}
      data-tone={tone}
      className={cn(
        "flex flex-col gap-2.5 rounded-[10px] border px-3.5 py-3 outline-none",
        FRAME[tone],
        current && tone !== "acknowledged" && `ring-2 ${RING[tone]}`,
      )}
    >
      <div className="flex items-start gap-2.5">
        <Icon aria-hidden="true" className={cn("mt-0.5 size-4 flex-none", INK[tone])} />
        <div className="min-w-0 flex-1">
          <p className="m-0 flex flex-wrap items-center gap-x-2 gap-y-0.5">
            <span className="text-ui text-fg font-semibold">{titleOf(t, finding)}</span>
            <span className={cn("text-caption font-semibold", INK[tone])}>
              {severity}
            </span>
          </p>
          <p className="text-muted-fg mt-0.5 mb-0 text-sm leading-[1.55] text-pretty">
            {reason === null
              ? helpText(t, finding, key, onReprocess !== undefined)
              : t(`imports.findings.objectReasons.${reason}`)}
          </p>
        </div>
      </div>
      {children}
      {finding.code === "AMBIGUOUS_KEY_PAPER" && onReprocess ? (
        <PaperChoice
          papers={keyPapers(finding)}
          disabled={readOnly}
          onReprocess={onReprocess}
        />
      ) : null}
      {tone === "acknowledged" ? (
        <p className="text-fg text-meta m-0 flex flex-wrap items-center gap-2">
          <CircleCheck aria-hidden="true" className="text-success size-3.5 flex-none" />
          <span className="min-w-0 flex-[1_1_200px]">
            {t("imports.review.confirmedByYou")}
          </span>
          {readOnly ? null : (
            <button
              type="button"
              onClick={() => onAcknowledge(finding.id, false)}
              className="text-fg cursor-pointer rounded-sm font-medium underline underline-offset-2"
            >
              {t("imports.review.unacknowledge")}
            </button>
          )}
        </p>
      ) : null}
      <FindingActions
        finding={finding}
        tone={tone}
        readOnly={readOnly}
        onAcknowledge={onAcknowledge}
        onLocate={onLocate}
        actions={actions}
      />
    </div>
  );
});

function FindingActions({
  finding,
  tone,
  readOnly,
  onAcknowledge,
  onLocate,
  actions,
}: Readonly<{
  finding: ImportFinding;
  tone: Tone;
  readOnly: boolean;
  onAcknowledge: (findingId: string, on: boolean) => void;
  onLocate: (refs: readonly ImportSourceRef[]) => void;
  actions: ReactNode;
}>) {
  const { t } = useTranslation();
  const acknowledge = tone === "review" && !readOnly;
  const locate = finding.evidence.length > 0;
  if (!acknowledge && !locate && !actions) return null;
  return (
    <div className="flex flex-wrap gap-1.5">
      {acknowledge ? (
        <Button
          type="button"
          className={cn(FINDING_ACTION, "bg-primary text-primary-fg border-primary")}
          onClick={() => onAcknowledge(finding.id, true)}
        >
          {t("imports.review.acknowledge")}
        </Button>
      ) : null}
      {actions}
      {locate ? (
        <Button
          type="button"
          variant="outline"
          className={FINDING_ACTION}
          onClick={() => onLocate(finding.evidence)}
        >
          <FileSearch aria-hidden="true" className="size-3.25" />
          {t("imports.review.showEvidence")}
        </Button>
      ) : null}
    </div>
  );
}

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
    <fieldset disabled={disabled} className="m-0 flex flex-col gap-2 border-0 p-0">
      <legend className="text-fg mb-2 p-0 text-xs font-medium">
        {t("imports.review.choosePaper")}
      </legend>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,160px),1fr))] gap-2">
        {papers.map((value) => (
          <label
            key={value}
            className={cn(
              "bg-card text-ui flex cursor-pointer items-center gap-2.5 rounded-[9px] border-[1.5px] px-3 py-2.5 font-semibold",
              paper === value ? "border-primary" : "border-border",
            )}
          >
            <input
              type="radio"
              name="key-paper"
              checked={paper === value}
              onChange={() => setPaper(value)}
              className="accent-primary size-4 flex-none"
            />
            {t("imports.review.paperNumber", { n: value })}
          </label>
        ))}
      </div>
      <div>
        <Button
          type="button"
          className={cn(FINDING_ACTION, "bg-primary text-primary-fg")}
          disabled={paper === null}
          onClick={() => paper !== null && onReprocess(paper)}
        >
          {t("imports.review.reprocessWithPaper")}
        </Button>
      </div>
      <p className="text-muted-fg m-0 text-xs">{t("imports.review.reprocessHint")}</p>
    </fieldset>
  );
}
