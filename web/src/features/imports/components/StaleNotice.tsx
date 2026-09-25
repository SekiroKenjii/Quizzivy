import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";

/** StaleNotice says the latest refresh failed while older data is still shown, and offers to refresh again. */
export function StaleNotice({ onRetry }: Readonly<{ onRetry: () => void }>) {
  const { t } = useTranslation();
  return (
    <p role="status" className="flex flex-wrap items-center gap-2 text-sm">
      <span>{t("imports.staleData")}</span>
      <Button type="button" variant="outline" size="xs" onClick={onRetry}>
        {t("common.retry")}
      </Button>
    </p>
  );
}
