import { useTranslation } from "react-i18next";
import { Check, CircleAlert } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { AutosaveStatus } from "@/features/tests/useAutosave";
import { formatTime } from "@/lib/i18n/datetime";

/**
 * §8: autosave is reported in words, never as a spinner that leaves the teacher
 * guessing whether it is safe to close the tab.
 */
export function AutosaveStatusLabel({
  status,
  onRetry,
}: {
  status: AutosaveStatus;
  onRetry?: () => void;
}) {
  const { t } = useTranslation();

  if (status.kind === "idle") return null;

  if (status.kind === "dirty") {
    return (
      <span role="status" aria-live="polite" className="text-muted-foreground text-xs">
        {t("builder.dirty")}
      </span>
    );
  }

  if (status.kind === "saving") {
    return (
      <span role="status" aria-live="polite" className="text-muted-foreground text-xs">
        {t("builder.saving")}
      </span>
    );
  }

  if (status.kind === "saved") {
    return (
      <Badge role="status" aria-live="polite">
        <Check aria-hidden="true" />
        {t("builder.saved", { time: formatTime(status.at) })}
      </Badge>
    );
  }

  if (status.kind === "stale") {
    return (
      <Badge variant="danger" role="alert">
        <CircleAlert aria-hidden="true" />
        {t("builder.stale")}
      </Badge>
    );
  }
  return (
    <span role="alert" className="flex items-center gap-2">
      <Badge variant="danger">
        <CircleAlert aria-hidden="true" />
        {t("builder.saveFailed")}
      </Badge>
      {status.message === "" ? null : (
        <span className="text-muted-foreground text-xs">{status.message}</span>
      )}
      {onRetry === undefined ? null : (
        <Button variant="outline" size="xs" onClick={onRetry}>
          {t("common.retry")}
        </Button>
      )}
    </span>
  );
}
