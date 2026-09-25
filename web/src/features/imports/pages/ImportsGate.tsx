import { useTranslation } from "react-i18next";
import { Outlet } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { EmptyState, LoadError } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import type { ImportCapabilities } from "../api";
import { importCapabilitiesQuery } from "../queries";

const intakeOf = (capabilities: ImportCapabilities) => capabilities.intakeEnabled;

export default function ImportsGate() {
  const { t } = useTranslation();
  const intake = useQuery({ ...importCapabilitiesQuery(), select: intakeOf });
  if (intake.data === true) return <Outlet />;
  if (intake.data === false)
    return (
      <>
        <PageHeader
          title={t("imports.availability.offTitle")}
          backTo="/admin/tests"
          backLabel={t("imports.backToTests")}
        />
        <EmptyState hint={t("imports.availability.offHint")}>
          {t("imports.availability.off")}
        </EmptyState>
      </>
    );
  if (intake.isPending)
    return (
      <div role="status" aria-label={t("common.loading")} className="space-y-3">
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  return (
    <LoadError error={intake.error} onRetry={() => void intake.refetch()}>
      {t("imports.availability.loadFailed")}
    </LoadError>
  );
}
