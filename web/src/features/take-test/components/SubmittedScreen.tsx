import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { SubmitReason } from "../store";
import { formatTime, shortDate } from "@/lib/i18n/datetime";

/** SubmittedScreen confirms the submission and offers the submitted paper or home. */
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
  const { t } = useTranslation();
  return (
    <main className="mx-auto w-full max-w-[720px] px-4 pt-16 text-center">
      <CircleCheck
        className="text-muted-foreground mx-auto size-8"
        aria-hidden="true"
      />
      <h1 className="mt-4 text-lg font-semibold tracking-tight">
        {t(`takeTest.submittedTitle_${reason}`)}
      </h1>
      <p className="text-muted-foreground mt-2 text-sm leading-relaxed">
        {t(`takeTest.submittedBody_${reason}`)}
      </p>
      <p className="text-muted-foreground mt-3 text-xs tabular-nums">
        {t("takeTest.submittedMeta", {
          time: formatTime(submittedAt),
          date: shortDate(submittedAt),
          answered,
          total,
        })}
      </p>
      <div className="mt-5 flex flex-col items-stretch gap-2 sm:flex-row sm:justify-center">
        <Button size="lg" onClick={onResult}>
          {t("takeTest.viewSubmitted")}
        </Button>
        <Button size="lg" variant="outline" onClick={onHome}>
          {t("takeTest.backHome")}
        </Button>
      </div>
    </main>
  );
}
