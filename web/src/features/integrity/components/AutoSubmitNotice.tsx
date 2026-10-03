import { useTranslation } from "react-i18next";
import { EyeOff, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EndState } from "@/features/take-test/components/EndState";
import { useTakeTestStore } from "@/features/take-test/store";

/**
 * AutoSubmitNotice replaces the paper once an `auto_submit` assignment's
 * allowance is passed, while the submission is on its way: the answers are
 * locked and kept, and the page says so in the focus dialog's tones. A polite
 * status line says whether the submission is being sent or is waiting to be
 * sent again, which the store does by itself; the button sends it at once and
 * is inert while a request is out. It never calls the absence a violation:
 * the teacher judges, the app reports.
 */
export function AutoSubmitNotice() {
  const { t } = useTranslation();
  const pending = useTakeTestStore((s) => s.submitState === "inFlight");
  const submit = useTakeTestStore((s) => s.submit);
  return (
    <EndState icon={EyeOff} tone="warning" title={t("integrity.autoSubmitTitle")}>
      <p className="text-muted-fg text-base leading-[1.55] text-pretty">
        {t("integrity.autoSubmitBody")}
      </p>
      <p role="status" className="text-muted-fg text-meta leading-normal text-pretty">
        {t(pending ? "integrity.autoSubmitSending" : "integrity.autoSubmitRetry")}
      </p>
      <Button
        size="lg"
        className="mt-1"
        disabled={pending}
        onClick={() => void submit("auto_submit")}
      >
        {pending && (
          <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
        )}
        {t("takeTest.retrySave")}
      </Button>
    </EndState>
  );
}
