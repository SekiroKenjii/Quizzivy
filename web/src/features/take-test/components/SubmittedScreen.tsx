import { useTranslation } from "react-i18next";
import { CircleCheck, EyeOff, Timer, type LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { formatTime, shortDate, useDisplayTimeZone } from "@/lib/i18n/datetime";
import type { SubmitReason } from "../store";
import { EndState } from "./EndState";

const LOOK: Record<
  SubmitReason,
  { icon: LucideIcon; tone: "success" | "muted" | "warning" }
> = {
  manual: { icon: CircleCheck, tone: "success" },
  timer_expired: { icon: Timer, tone: "muted" },
  auto_submit: { icon: EyeOff, tone: "warning" },
};

/**
 * SubmittedScreen says the paper is in, and why when the student did not hand
 * it in: the timer ran out, or an `auto_submit` assignment's allowance was
 * passed. Under the heading it says when it went in and how many questions
 * were answered, and offers the submitted paper or home. The tile takes the
 * success tone for the student's own submit, the timer for a timeout and the
 * focus dialog's warning tile for an auto-submit.
 */
export function SubmittedScreen({
  reason,
  submittedAt,
  answered,
  total,
  onHome,
  onResult,
}: Readonly<{
  reason: SubmitReason;
  submittedAt: string;
  answered: number;
  total: number;
  onHome: () => void;
  onResult: () => void;
}>) {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const { icon, tone } = LOOK[reason];
  return (
    <EndState icon={icon} tone={tone} title={t(`takeTest.submittedTitle_${reason}`)}>
      <p className="text-muted-fg text-base leading-[1.55] text-pretty">
        {t(`takeTest.submittedBody_${reason}`)}
      </p>
      <p className="text-muted-fg text-meta leading-normal tabular-nums">
        {t("takeTest.submittedMeta", {
          time: formatTime(submittedAt),
          date: shortDate(submittedAt),
          answered,
          total,
        })}
      </p>
      <div className="mt-1 flex flex-wrap justify-center gap-2">
        <Button
          variant="outline"
          size="lg"
          className="shadow-none in-data-[scale=deck]:font-medium"
          onClick={onHome}
        >
          {t("takeTest.backHome")}
        </Button>
        <Button size="lg" onClick={onResult}>
          {t("takeTest.viewSubmitted")}
        </Button>
      </div>
    </EndState>
  );
}
