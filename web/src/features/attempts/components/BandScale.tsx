import { useId, useRef } from "react";
import { useTranslation } from "react-i18next";
import { ArrowBigUp } from "lucide-react";
import { Button } from "@/components/ui/button";
import { BANDS, formatBand } from "@/lib/bands";
import type { components } from "@/lib/api/schema";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import { bandKeyEvent, focusAfter } from "../bandKeys";
import { markBand, type EssayMark } from "../essayMark";
import "../gradingMessages";

type Criterion = components["schemas"]["RubricCriterion"];

/**
 * BandScale is one row of the thirteen bands, 3 to 9 in half steps (DG-86),
 * each with its key: the digit for a whole band, Shift and the digit for the
 * half above. A key pressed while focus is inside the row sets that row's band
 * and goes no further, so the page's own keys never set a second scale.
 * `scale` names the row in each button's `data-score`, which the grading card
 * uses to put focus back after a save.
 */
export function BandScale({
  scale,
  value,
  labelledBy,
  onChange,
  onFocus,
}: Readonly<{
  scale: string;
  value: number | null;
  labelledBy: string;
  onChange: (band: number, byKey: boolean) => void;
  onFocus?: () => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  return (
    <div role="group" aria-labelledby={labelledBy} className="flex flex-wrap gap-1">
      {BANDS.map((band) => {
        const whole = Number.isInteger(band);
        const digit = Math.floor(band);
        return (
          <Button
            key={band}
            type="button"
            variant={value === band ? "default" : "outline"}
            aria-pressed={value === band}
            aria-label={t("grading.band", { band: formatBand(band, locale) })}
            data-score={`${scale}:${band}`}
            className="h-11 min-w-11 flex-col gap-0 rounded-[9px] px-1.5 text-[14px] leading-[18px] font-semibold tabular-nums"
            onClick={() => onChange(band, false)}
            onFocus={onFocus}
            onKeyDown={(event) => {
              const keyed = bandKeyEvent(event);
              if (keyed === null) return;
              event.preventDefault();
              event.stopPropagation();
              onChange(keyed, true);
            }}
          >
            {formatBand(band, locale)}
            <kbd
              aria-hidden="true"
              className="inline-flex items-center gap-px font-sans text-[10px] leading-3 font-medium opacity-60"
            >
              {!whole && <ArrowBigUp className="size-2.5!" strokeWidth={2.25} />}
              {digit}
            </kbd>
          </Button>
        );
      })}
    </div>
  );
}

/**
 * EssayBands marks an essay on its band: one scale when the question has no
 * rubric, otherwise one scale per criterion in authored order and the overall
 * band derived from them as they change (DG-61). `onChange` names the
 * button's `data-score` beside the new mark. `error` is the refused
 * save's field message. `onActive` reports the criterion whose scale took
 * focus, so the page's band keys can follow it.
 */
export function EssayBands({
  rubric,
  mark,
  error,
  onChange,
  onActive,
}: Readonly<{
  rubric: readonly Criterion[];
  mark: EssayMark;
  error: string | null;
  onChange: (mark: EssayMark, score: string) => void;
  onActive?: (criterionId: string | null) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const id = useId();
  const root = useRef<HTMLDivElement>(null);
  const derived = markBand(rubric, mark);
  const setCriterion = (criterionId: string, band: number, byKey: boolean) => {
    onChange(
      { ...mark, criteria: { ...mark.criteria, [criterionId]: band } },
      `${criterionId}:${band}`,
    );
    if (byKey) focusAfter(root.current, rubric, criterionId);
  };
  return (
    <div ref={root} className="space-y-3">
      {rubric.length === 0 ? (
        <div className="space-y-2">
          <p
            id={`${id}-band`}
            className="text-muted-foreground text-xs leading-[18px] font-medium"
          >
            {t("grading.bandLegend")}
          </p>
          <BandScale
            scale="band"
            value={mark.band}
            labelledBy={`${id}-band`}
            onChange={(band) => onChange({ ...mark, band }, `band:${band}`)}
            onFocus={() => onActive?.(null)}
          />
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-muted-foreground text-xs leading-[18px] font-medium">
            {t("grading.criteriaLegend")}
          </p>
          {rubric.map((criterion) => (
            <div key={criterion.id} className="space-y-1.5">
              <p id={`${id}-${criterion.id}`} className="text-[13.5px] font-medium">
                {criterion.label}
              </p>
              <BandScale
                scale={criterion.id}
                value={mark.criteria[criterion.id] ?? null}
                labelledBy={`${id}-${criterion.id}`}
                onChange={(band, byKey) => setCriterion(criterion.id, band, byKey)}
                onFocus={() => onActive?.(criterion.id)}
              />
            </div>
          ))}
          <p
            aria-live="polite"
            className={cn(
              "text-[14px] font-semibold tabular-nums",
              derived === null && "text-muted-foreground font-normal",
            )}
          >
            {derived === null
              ? t("grading.derivedPending")
              : t("grading.derivedBand", { band: formatBand(derived, locale) })}
          </p>
        </div>
      )}
      {error !== null && (
        <p role="alert" className="text-destructive text-sm">
          {error}
        </p>
      )}
      <p className="text-muted-foreground hidden text-xs min-[768px]:block">
        {t("grading.bandKeys")}
      </p>
    </div>
  );
}
