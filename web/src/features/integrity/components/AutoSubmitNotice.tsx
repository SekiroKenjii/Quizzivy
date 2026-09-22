import { useTranslation } from "react-i18next";
import { useTakeTestStore } from "@/features/take-test/store";
import { Button } from "@/components/ui/button";

export function AutoSubmitNotice() {
  const { t } = useTranslation();
  const pending = useTakeTestStore((s) => s.submitState === "inFlight");
  const submit = useTakeTestStore((s) => s.submit);
  return (
    <main className="student-page mx-auto w-full max-w-xl space-y-4 p-6">
      <h1 className="text-xl font-semibold">{t("integrity.autoSubmitTitle")}</h1>
      <p>{t("integrity.autoSubmitBody")}</p>
      <p role="status" className="text-muted-foreground text-sm">
        {t(pending ? "integrity.autoSubmitSending" : "integrity.autoSubmitRetry")}
      </p>
      <Button disabled={pending} onClick={() => void submit("auto_submit")}>
        {t("takeTest.retrySave")}
      </Button>
    </main>
  );
}
