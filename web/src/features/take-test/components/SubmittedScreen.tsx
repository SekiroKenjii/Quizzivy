import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { SubmitReason } from "../store";
import { formatTime, shortDate } from "@/lib/i18n/datetime";

/**
 * S-06's closed state: the engine chrome is gone, the cause is said once in
 * the title, and "Về trang chủ" is the only control.
 */
export function SubmittedScreen({
  reason,
  submittedAt,
  answered,
  total,
  onHome,
}: {
  reason: SubmitReason;
  submittedAt: string;
  answered: number;
  total: number;
  onHome: () => void;
}) {
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
      <Button className="mt-5" onClick={onHome}>
        {t("takeTest.backHome")}
      </Button>
    </main>
  );
}
