import type { CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { Lock } from "lucide-react";
import type { Locale } from "@/lib/i18n";
import type { Ring, RingTone } from "../resultView";

const ARC: Record<RingTone, string> = {
  success: "var(--success)",
  accent: "var(--accent-c)",
  warning: "var(--warning)",
};

const VALUE = "text-[1.5rem] leading-[1.1] font-bold tabular-nums";
const CAPTION = "text-muted-fg max-w-20 text-center text-xs leading-[1.1]";

function arcOf(ring: Ring): { colour: string; degrees: number } {
  if (ring.kind === "score")
    return { colour: ARC[ring.tone], degrees: Math.round(ring.share * 360) };
  if (ring.kind === "soFar")
    return { colour: ARC.accent, degrees: Math.round(ring.share * 360) };
  return { colour: ARC.warning, degrees: 0 };
}

/**
 * ScoreRing is the summary's 112px ring, as the design deck draws it: an arc
 * for the share of the points earned, and the number inside. A final score
 * reads "8 of 10" in the tone its share earns; while answers wait for the
 * teacher it reads "3 of 6 so far" in the accent; with nothing marked yet it
 * reads "…" and "grading"; a score the review policy hides is a lock and no
 * number.
 */
export function ScoreRing({ ring }: Readonly<{ ring: Ring }>) {
  const { t, i18n } = useTranslation();
  const n = new Intl.NumberFormat(i18n.language as Locale, {
    maximumFractionDigits: 2,
  });
  const { colour, degrees } = arcOf(ring);
  return (
    <div
      data-slot="score-ring"
      data-ring={ring.kind}
      className="grid size-28 flex-none place-items-center rounded-full bg-[conic-gradient(var(--arc)_var(--turn),var(--muted)_0)]"
      style={{ "--arc": colour, "--turn": `${degrees}deg` } as CSSProperties}
    >
      <div className="bg-card flex size-22.5 flex-none flex-col items-center justify-center rounded-full">
        {ring.kind === "score" && (
          <>
            <span className={VALUE}>{n.format(ring.earned)}</span>
            <span className={CAPTION}>
              {t("result.ring.of", { total: n.format(ring.total) })}
            </span>
          </>
        )}
        {ring.kind === "soFar" && (
          <>
            <span className={VALUE}>{n.format(ring.earned)}</span>
            <span className={CAPTION}>
              {t("result.ring.soFar", { total: n.format(ring.total) })}
            </span>
          </>
        )}
        {ring.kind === "grading" && (
          <>
            <span aria-hidden="true" className={VALUE}>
              …
            </span>
            <span className={CAPTION}>{t("result.ring.grading")}</span>
          </>
        )}
        {ring.kind === "withheld" && (
          <>
            <Lock aria-hidden="true" className="text-muted-fg size-5" />
            <span className="sr-only">{t("result.ring.withheld")}</span>
          </>
        )}
      </div>
    </div>
  );
}
