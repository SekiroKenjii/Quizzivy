import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { LoadError } from "@/components/shared/ListState";
import { Skeleton } from "@/components/ui/skeleton";
import { aiUsageQuery, isAiOff, useAiConfigured } from "@/features/settings/aiApi";
import { AiPreferencesSection } from "@/features/settings/sections/AiPreferences";
import { AiUsageCard } from "@/features/settings/sections/AiUsageCard";

/**
 * AiAssistantSection is Settings › AI assistant (`/teacher/settings/ai`,
 * `ai.use`): the month's usage with the request for more credits, and the
 * assistant's preferences. It reads nothing until it is first `shown`, so
 * the other sections do not wait on it. With no assistant on the server
 * (`PublicStatus.ai` false, or the usage read answering `501 AI_OFF`) it
 * shows one sentence instead (DG-201).
 */
export function AiAssistantSection({ shown }: Readonly<{ shown: boolean }>) {
  const { t } = useTranslation();
  const configured = useAiConfigured(shown);
  const usage = useQuery({
    ...aiUsageQuery(),
    enabled: shown && configured.data === true,
  });

  if (configured.data === false || isAiOff(usage.error))
    return (
      <p className="bg-card shadow-card text-ui rounded-xl border px-4.5 py-4 leading-normal">
        {t("settings.ai.off")}
      </p>
    );
  if (configured.isError)
    return (
      <LoadError error={configured.error} onRetry={() => void configured.refetch()}>
        {t("settings.ai.loadFailed")}
      </LoadError>
    );
  if (configured.data === undefined)
    return (
      <div
        role="status"
        aria-label={t("common.loading")}
        className="flex flex-col gap-3.5"
      >
        <Skeleton className="h-72 w-full rounded-xl" />
        <Skeleton className="h-60 w-full rounded-xl" />
      </div>
    );
  return (
    <>
      <AiUsageCard query={usage} />
      <AiPreferencesSection />
    </>
  );
}
